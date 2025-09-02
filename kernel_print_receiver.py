#!/usr/bin/env python3
import ctypes
import ctypes.wintypes as wt
import sys

S_OK = 0x00000000
HRESULT = ctypes.c_long
if not hasattr(wt, "ULONG_PTR"):
    wt.ULONG_PTR = ctypes.c_size_t

# FILTER_MESSAGE_HEADER (FltUser.h)
class FILTER_MESSAGE_HEADER(ctypes.Structure):
    _fields_ = [("ReplyLength", wt.DWORD), ("MessageId", ctypes.c_ulonglong)]

# matches your C struct
class PROCESS_EVENT(ctypes.Structure):
    _fields_ = [
        ("isCreate", ctypes.c_int),
        ("ProcessId", ctypes.c_int),
        ("ImageFileName", wt.WCHAR * 260),
    ]

FMT_FROM_SYSTEM = 0x00001000
FMT_IGNORE_INSERTS = 0x00000200
def hresult_to_text(hr: int) -> str:
    code = hr & 0xFFFF if (hr & 0xFFFF0000) == 0x80070000 else (hr & 0xFFFFFFFF)
    buf = ctypes.create_unicode_buffer(512)
    n = ctypes.windll.kernel32.FormatMessageW(
        FMT_FROM_SYSTEM | FMT_IGNORE_INSERTS, None, code, 0, buf, len(buf), None
    )
    return buf.value.strip() if n else ""

class Receiver:
    def __init__(self, port_name=r"\MedusaComPort"):
        self.port_name = port_name
        self.fltlib = ctypes.windll.fltlib
        self.k32 = ctypes.windll.kernel32
        self.hPort = wt.HANDLE()
        # prototypes
        self.fltlib.FilterConnectCommunicationPort.argtypes = [
            wt.LPCWSTR, wt.DWORD, wt.LPCVOID, wt.WORD,
            ctypes.c_void_p, ctypes.POINTER(wt.HANDLE)
        ]
        self.fltlib.FilterConnectCommunicationPort.restype = HRESULT
        self.fltlib.FilterGetMessage.argtypes = [wt.HANDLE, wt.LPVOID, wt.DWORD, ctypes.c_void_p]
        self.fltlib.FilterGetMessage.restype = HRESULT
        self.k32.CloseHandle.argtypes = [wt.HANDLE]
        self.k32.CloseHandle.restype = wt.BOOL

    def connect(self):
        hr = self.fltlib.FilterConnectCommunicationPort(self.port_name, 0, None, 0, None, ctypes.byref(self.hPort))
        if hr != S_OK:
            print(f"[ERR] Connect 0x{hr & 0xFFFFFFFF:08X} ({hresult_to_text(hr)})")
            return False
        print(f"[OK] Connected {self.port_name}")
        return True

    def loop(self):
        hdr_sz = ctypes.sizeof(FILTER_MESSAGE_HEADER)
        ev_sz  = ctypes.sizeof(PROCESS_EVENT)
        buf    = ctypes.create_string_buffer(hdr_sz + ev_sz)

        while True:
            # BLOCKING call (no OVERLAPPED) → no 0x800703E5 spam
            hr = self.fltlib.FilterGetMessage(self.hPort, buf, len(buf), None)
            if hr != S_OK:
                print(f"[ERR] FilterGetMessage 0x{hr & 0xFFFFFFFF:08X} ({hresult_to_text(hr)})")
                continue

            hdr = FILTER_MESSAGE_HEADER.from_buffer_copy(buf.raw[:hdr_sz])
            evt = PROCESS_EVENT.from_buffer_copy(buf.raw[hdr_sz:hdr_sz+ev_sz])

            kind  = "CREATE" if evt.isCreate else "EXIT"
            image = evt.ImageFileName if evt.isCreate and evt.ImageFileName else "Unknown"
            print(f"[PROC] {kind} pid={evt.ProcessId} image={image} msgId={hdr.MessageId} replyLen={hdr.ReplyLength}")

    def close(self):
        if self.hPort:
            self.k32.CloseHandle(self.hPort)
            self.hPort = wt.HANDLE()

def main():
    r = Receiver()
    if not r.connect():
        return 1
    try:
        r.loop()
    except KeyboardInterrupt:
        pass
    finally:
        r.close()
    return 0

if __name__ == "__main__":
    sys.exit(main())
