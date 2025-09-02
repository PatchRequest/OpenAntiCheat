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

# ----- kernel payloads with reserved as discriminator -----
# reserved == 0
class CreateProcessNotifyRoutineEvent(ctypes.Structure):
    _fields_ = [
        ("reserved", ctypes.c_int),      # 0
        ("isCreate", ctypes.c_int),      # 1 create 0 exit
        ("ProcessId", ctypes.c_int),
        ("ImageFileName", wt.WCHAR * 260),
        ("CommandLine",  wt.WCHAR * 1024),
    ]

# reserved == 1
class FLT_PREOP_CALLBACK_Event(ctypes.Structure):
    _fields_ = [
        ("reserved", ctypes.c_int),      # 1
        ("operation", ctypes.c_int),     # IRP_MJ_*
        ("ProcessId", ctypes.c_int),
        ("FileName",  wt.WCHAR * 260),
    ]

# reserved == 2
class OB_OPERATION_HANDLE_Event(ctypes.Structure):
    _fields_ = [
        ("reserved", ctypes.c_int),      # 2
        ("operation", ctypes.c_int),     # 0 create 1 duplicate
        ("ProcessId", ctypes.c_int),
    ]

IRP_MAJOR = {
    0x00:"CREATE",0x01:"CREATE_NAMED_PIPE",0x02:"CLOSE",0x03:"READ",0x04:"WRITE",
    0x05:"QUERY_INFORMATION",0x06:"SET_INFORMATION",0x07:"QUERY_EA",0x08:"SET_EA",
    0x09:"FLUSH_BUFFERS",0x0A:"QUERY_VOLUME_INFORMATION",0x0B:"SET_VOLUME_INFORMATION",
    0x0C:"DIRECTORY_CONTROL",0x0D:"FILE_SYSTEM_CONTROL",0x0E:"DEVICE_CONTROL",
    0x0F:"INTERNAL_DEVICE_CONTROL",0x10:"SHUTDOWN",0x11:"LOCK_CONTROL",
    0x12:"CLEANUP",0x13:"CREATE_MAILSLOT",0x14:"QUERY_SECURITY",0x15:"SET_SECURITY",
    0x16:"POWER",0x17:"SYSTEM_CONTROL",0x18:"DEVICE_CHANGE",0x19:"QUERY_QUOTA",
    0x1A:"SET_QUOTA",0x1B:"PNP",
}
OB_OP = {0:"CREATE", 1:"DUPLICATE"}

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
        self.fltlib.FilterConnectCommunicationPort.argtypes = [
            wt.LPCWSTR, wt.DWORD, wt.LPCVOID, wt.WORD, ctypes.c_void_p, ctypes.POINTER(wt.HANDLE)
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
        sz_proc = ctypes.sizeof(CreateProcessNotifyRoutineEvent)
        sz_flt  = ctypes.sizeof(FLT_PREOP_CALLBACK_Event)
        sz_ob   = ctypes.sizeof(OB_OPERATION_HANDLE_Event)
        max_sz  = max(sz_proc, sz_flt, sz_ob)
        buf     = ctypes.create_string_buffer(hdr_sz + max_sz)

        while True:
            hr = self.fltlib.FilterGetMessage(self.hPort, buf, len(buf), None)  # blocking
            if hr != S_OK:
                print(f"[ERR] FilterGetMessage 0x{hr & 0xFFFFFFFF:08X} ({hresult_to_text(hr)})")
                continue

            hdr = FILTER_MESSAGE_HEADER.from_buffer_copy(buf.raw[:hdr_sz])
            payload = buf.raw[hdr_sz:hdr_sz+max_sz]
            tag = ctypes.c_int.from_buffer_copy(payload, 0).value

            if tag == 0:
                ev = CreateProcessNotifyRoutineEvent.from_buffer_copy(payload)
                kind = "CREATE" if ev.isCreate else "EXIT"
                img  = ev.ImageFileName if (ev.ImageFileName and ev.ImageFileName[0]) else "Unknown"
                print(f"[PROC] {kind} pid={ev.ProcessId} image={img} msgId={hdr.MessageId} CommandLine={ev.CommandLine}")

            elif tag == 1:
                ev = FLT_PREOP_CALLBACK_Event.from_buffer_copy(payload)
                op = IRP_MAJOR.get(ev.operation, f"0x{ev.operation:02X}")
                fn = ev.FileName if (ev.FileName and ev.FileName[0]) else ""
                print(f"[FILE] {op} pid={ev.ProcessId} file={fn} msgId={hdr.MessageId}")

            elif tag == 2:
                ev = OB_OPERATION_HANDLE_Event.from_buffer_copy(payload)
                op = OB_OP.get(ev.operation, f"OP_{ev.operation}")
                print(f"[OB] {op} pid={ev.ProcessId} msgId={hdr.MessageId}")

            else:
                print(f"[WARN] unknown tag={tag} msgId={hdr.MessageId}")

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
