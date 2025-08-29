#!/usr/bin/env python3
"""
Kernel Print Receiver for OpenAntiCheat Minifilter

This script connects to the minifilter's communication port (\MedusaComPort)
and receives kernel prints/notifications sent via raw struct data.
Replicates DbgPrint functionality in userland.
"""

import ctypes
import ctypes.wintypes
from ctypes import wintypes, Structure, c_wchar, c_uint32, POINTER, byref
import sys
import threading
import time

# Windows API constants
GENERIC_READ = 0x80000000
GENERIC_WRITE = 0x40000000
OPEN_EXISTING = 3
FILE_FLAG_OVERLAPPED = 0x40000000

# Buffer size from the minifilter (4096 bytes / page size)
BUFFER_SIZE = 4096

class FLT_TO_USER(Structure):
    """Structure for filter-to-user messages"""
    _fields_ = [
        ("path", c_wchar * (BUFFER_SIZE // 2))  # wchar_t path[BUFFER_SIZE / sizeof(wchar_t)]
    ]

class FLT_TO_USER_REPLY(Structure):
    """Structure for replying to filter messages"""
    _fields_ = [
        ("block", c_uint32)  # unsigned __int32 block; if 1, file access will be denied
    ]

class USER_TO_FLT(Structure):
    """Structure for user-to-filter messages"""
    _fields_ = [
        ("msg", c_wchar * (BUFFER_SIZE // 2))  # wchar_t msg[BUFFER_SIZE / sizeof(wchar_t)]
    ]

class USER_TO_FLT_REPLY(Structure):
    """Structure for filter replies to user messages"""
    _fields_ = [
        ("msg", c_wchar * (BUFFER_SIZE // 2))  # wchar_t msg[BUFFER_SIZE / sizeof(wchar_t)]
    ]

class OVERLAPPED(Structure):
    """Windows OVERLAPPED structure for async I/O"""
    _fields_ = [
        ("Internal", ctypes.POINTER(ctypes.wintypes.ULONG)),
        ("InternalHigh", ctypes.POINTER(ctypes.wintypes.ULONG)),
        ("Offset", ctypes.wintypes.DWORD),
        ("OffsetHigh", ctypes.wintypes.DWORD),
        ("hEvent", ctypes.wintypes.HANDLE)
    ]

class MinifiterClient:
    def __init__(self, port_name=r"\\.\MedusaComPort"):
        self.port_name = port_name
        self.handle = None
        self.running = False
        
        # Load required Windows APIs
        self.kernel32 = ctypes.windll.kernel32
        self.fltlib = ctypes.windll.fltlib
        
        # Define function prototypes
        self._setup_api_prototypes()
    
    def _setup_api_prototypes(self):
        """Set up Windows API function prototypes"""
        # FilterConnectCommunicationPort
        self.fltlib.FilterConnectCommunicationPort.argtypes = [
            ctypes.wintypes.LPCWSTR,  # lpPortName
            ctypes.wintypes.DWORD,    # dwOptions
            ctypes.wintypes.LPCVOID,  # lpContext
            ctypes.wintypes.WORD,     # wSizeOfContext
            ctypes.wintypes.LPSECURITY_ATTRIBUTES,  # lpSecurityAttributes
            ctypes.POINTER(ctypes.wintypes.HANDLE)  # hPort
        ]
        self.fltlib.FilterConnectCommunicationPort.restype = ctypes.wintypes.HRESULT
        
        # FilterGetMessage
        self.fltlib.FilterGetMessage.argtypes = [
            ctypes.wintypes.HANDLE,   # hPort
            ctypes.wintypes.LPVOID,   # lpMessageBuffer
            ctypes.wintypes.DWORD,    # dwMessageBufferSize
            ctypes.POINTER(OVERLAPPED)  # lpOverlapped
        ]
        self.fltlib.FilterGetMessage.restype = ctypes.wintypes.HRESULT
        
        # FilterReplyMessage
        self.fltlib.FilterReplyMessage.argtypes = [
            ctypes.wintypes.HANDLE,   # hPort
            ctypes.wintypes.LPVOID,   # lpReplyBuffer
            ctypes.wintypes.DWORD     # dwReplyBufferSize
        ]
        self.fltlib.FilterReplyMessage.restype = ctypes.wintypes.HRESULT
        
        # CreateEvent
        self.kernel32.CreateEventW.argtypes = [
            ctypes.wintypes.LPSECURITY_ATTRIBUTES,
            ctypes.wintypes.BOOL,
            ctypes.wintypes.BOOL,
            ctypes.wintypes.LPCWSTR
        ]
        self.kernel32.CreateEventW.restype = ctypes.wintypes.HANDLE
        
        # WaitForSingleObject
        self.kernel32.WaitForSingleObject.argtypes = [
            ctypes.wintypes.HANDLE,
            ctypes.wintypes.DWORD
        ]
        self.kernel32.WaitForSingleObject.restype = ctypes.wintypes.DWORD
    
    def connect(self):
        """Connect to the minifilter communication port"""
        try:
            port_handle = ctypes.wintypes.HANDLE()
            
            # Connect to the filter communication port
            hr = self.fltlib.FilterConnectCommunicationPort(
                self.port_name,     # Port name
                0,                  # Options
                None,               # Context
                0,                  # Context size
                None,               # Security attributes
                byref(port_handle)  # Output handle
            )
            
            if hr != 0:  # S_OK
                print(f"[ERROR] Failed to connect to port {self.port_name}. HRESULT: 0x{hr:08X}")
                print("[INFO] Make sure the minifilter driver is loaded and the port exists.")
                return False
            
            self.handle = port_handle
            print(f"[INFO] Successfully connected to {self.port_name}")
            return True
            
        except Exception as e:
            print(f"[ERROR] Exception during connection: {e}")
            return False
    
    def receive_messages(self):
        """Main message receiving loop - replicates kernel DbgPrint in userland"""
        if not self.handle:
            print("[ERROR] Not connected to filter port")
            return
        
        self.running = True
        print("[INFO] Starting to receive kernel messages...")
        print("[INFO] Replicating kernel DbgPrint output in userland:")
        print("=" * 60)
        
        # Create event for overlapped I/O
        event_handle = self.kernel32.CreateEventW(None, True, False, None)
        
        while self.running:
            try:
                # Set up overlapped structure
                overlapped = OVERLAPPED()
                overlapped.hEvent = event_handle
                
                # Buffer to receive message
                message_buffer = ctypes.create_string_buffer(8192)  # Larger buffer for message header + data
                
                # Attempt to get message from filter
                hr = self.fltlib.FilterGetMessage(
                    self.handle,
                    message_buffer,
                    len(message_buffer),
                    byref(overlapped)
                )
                
                if hr == 0:  # S_OK - message received immediately
                    self._process_received_data(message_buffer)
                elif hr == 0x800703E5:  # ERROR_IO_PENDING - async operation
                    # Wait for the async operation to complete
                    wait_result = self.kernel32.WaitForSingleObject(event_handle, 1000)  # 1 second timeout
                    
                    if wait_result == 0:  # WAIT_OBJECT_0 - success
                        self._process_received_data(message_buffer)
                    elif wait_result == 0x102:  # WAIT_TIMEOUT
                        continue  # Try again
                    else:
                        print(f"[ERROR] Wait failed: {wait_result}")
                        break
                else:
                    print(f"[ERROR] FilterGetMessage failed with HRESULT: 0x{hr:08X}")
                    time.sleep(0.1)  # Brief pause before retrying
                    
            except KeyboardInterrupt:
                print("\n[INFO] Received interrupt signal, stopping...")
                break
            except Exception as e:
                print(f"[ERROR] Exception in receive loop: {e}")
                time.sleep(0.1)
        
        # Clean up
        if event_handle:
            self.kernel32.CloseHandle(event_handle)
        
        print("[INFO] Message receiving stopped")
    
    def _process_received_data(self, message_buffer):
        """Process received raw struct data and replicate kernel prints"""
        try:
            # The message buffer contains a filter message header followed by the actual data
            # For simplicity, we'll try to parse it as different struct types
            
            # Try to parse as USER_TO_FLT (kernel notification message)
            try:
                # Skip the filter message header (typically 16-24 bytes)
                data_offset = 24  # Approximate filter message header size
                if len(message_buffer) > data_offset + ctypes.sizeof(USER_TO_FLT):
                    user_msg = USER_TO_FLT.from_buffer_copy(message_buffer.raw[data_offset:])
                    if user_msg.msg and user_msg.msg.strip():
                        # This replicates the DbgPrint output from the kernel
                        self._classify_and_print_kernel_message(user_msg.msg)
                        return
            except:
                pass
            
            # Try to parse as FLT_TO_USER (file access notification)
            try:
                data_offset = 24
                if len(message_buffer) > data_offset + ctypes.sizeof(FLT_TO_USER):
                    file_msg = FLT_TO_USER.from_buffer_copy(message_buffer.raw[data_offset:])
                    if file_msg.path and file_msg.path.strip():
                        # This replicates file access prints
                        print(f"[KERNEL] [MINIFILTER] File access: {file_msg.path}")
                        
                        # Send reply (allow access by default)
                        reply = FLT_TO_USER_REPLY()
                        reply.block = 0  # Don't block
                        
                        reply_buffer = ctypes.string_at(ctypes.byref(reply), ctypes.sizeof(reply))
                        self.fltlib.FilterReplyMessage(
                            self.handle,
                            reply_buffer,
                            len(reply_buffer)
                        )
                        return
            except:
                pass
            
            # If we can't parse as known structs, show raw data
            raw_data = message_buffer.raw[:min(100, len(message_buffer))]
            if any(b != 0 for b in raw_data):
                print(f"[KERNEL] [RAW] Raw message: {raw_data.hex()}")
                
        except Exception as e:
            print(f"[ERROR] Error processing received data: {e}")
    
    def _classify_and_print_kernel_message(self, message):
        """Classify and format kernel messages based on their content"""
        msg = message.strip()
        
        # Classify different types of kernel messages
        if "CreateCallback called" in msg:
            print(f"[KERNEL] [CALLBACK] {msg}")
        elif "Process created:" in msg or "Process terminated" in msg:
            print(f"[KERNEL] [PROCESS] {msg}")
        elif "FltRegisterFilter" in msg or "FltStartFiltering" in msg:
            print(f"[KERNEL] [MINIFILTER] {msg}")
        elif "ObRegisterCallbacks" in msg:
            print(f"[KERNEL] [CALLBACK] {msg}")
        elif "HelloWorld" in msg:
            print(f"[KERNEL] [MAIN] {msg}")
        elif "filterport" in msg:
            print(f"[KERNEL] [COMPORT] {msg}")
        elif msg.startswith("C:\\") or msg.endswith(".exe"):
            # Likely a process path sent via FpNotifyUser
            print(f"[KERNEL] [NOTIFY] Process path: {msg}")
        else:
            print(f"[KERNEL] {msg}")
    
    def send_test_message(self, message_text):
        """Send a test message to the filter"""
        if not self.handle:
            print("[ERROR] Not connected to filter port")
            return False
        
        try:
            # Create message structure
            user_msg = USER_TO_FLT()
            user_msg.msg = message_text[:len(user_msg.msg)-1]  # Ensure null termination
            
            # Prepare reply buffer
            reply = USER_TO_FLT_REPLY()
            reply_size = ctypes.wintypes.DWORD(ctypes.sizeof(reply))
            
            # Send message (using FilterSendMessage equivalent)
            # Note: This is a simplified version - full implementation would need FilterSendMessage
            print(f"[INFO] Would send test message: '{message_text}'")
            return True
            
        except Exception as e:
            print(f"[ERROR] Failed to send message: {e}")
            return False
    
    def disconnect(self):
        """Disconnect from the filter port"""
        self.running = False
        if self.handle:
            self.kernel32.CloseHandle(self.handle)
            self.handle = None
            print("[INFO] Disconnected from filter port")

def main():
    if len(sys.argv) > 1 and sys.argv[1] == "--test":
        print("Test mode - simulating kernel messages")
        # Simulate some kernel messages for testing
        messages = [
            # Main driver messages
            "HelloWorld from the Kernel Land!",
            "Driver Object:\t\t0xFFFFFA8012345678",
            "Registry Path:\t\t0xFFFFFA8012345679",
            
            # Process notifications (from NotifyRoutine.c)
            "Process created: C:\\Windows\\System32\\notepad.exe (PID: 1234)",
            "C:\\Windows\\System32\\notepad.exe",  # FpNotifyUser message
            "Process created with unknown image name (PID: 5678)",
            "Process terminated (PID: 1234)",
            
            # Callback notifications (from Callback.c)
            "CreateCallback called with operation 1 on process at address 0xFFFFFA8012345680 with PID 1234",
            "CreateCallback called with operation 2 on process at address 0xFFFFFA8012345681 with PID 5678",
            "ObRegisterCallbacks failed with status 0xC0000001",
            "Registered callback successfully",
            
            # Minifilter messages (from Minifilter.c)
            "Matched: C:\\test\\file.txt",
            "FltRegisterFilter returned 0x00000000",
            "FltStartFiltering returned 0x00000000",
            
            # Communication port messages (from Coms.c)
            "[filterport] MinifltPortNotifyRoutine User-mode application(1234) connect to this filter",
            "[filterport] MinifltPortMessageRoutine User-mode application(1234) send data to this filter",
            "[filterport] MinifltPortDisconnectRoutine User-mode application(1234) disconnect with this filter",
            
            # Registration status messages
            "createRegistration() returned 0x00000000",
            "createRegistrationMiniFilter() returned 0x00000000",
            "registerProcessNotifyRoutine() returned 0x00000000",
            "createRegistrationComFilter() returned 0x00000000",
        ]
        
        print("=" * 80)
        print("Simulating all types of kernel messages that would be sent to userland:")
        print("=" * 80)
        
        # Create a dummy client for message classification
        client = MinifiterClient()
        
        for msg in messages:
            if msg.startswith("[filterport]") or "CreateCallback" in msg or "Process" in msg or "ObRegisterCallbacks" in msg:
                client._classify_and_print_kernel_message(msg)
            elif msg.startswith("C:\\") and msg.endswith(".exe"):
                client._classify_and_print_kernel_message(msg)
            else:
                client._classify_and_print_kernel_message(msg)
            time.sleep(0.3)
        
        print("=" * 80)
        print("Test simulation complete. These messages would be received via COM port.")
        return
    
    client = MinifiterClient()
    
    try:
        if not client.connect():
            print("[ERROR] Failed to connect to minifilter. Ensure the driver is loaded.")
            return 1
        
        # Start receiving messages in a separate thread
        receive_thread = threading.Thread(target=client.receive_messages)
        receive_thread.daemon = True
        receive_thread.start()
        
        print("\n[INFO] Listening for kernel messages. Press Ctrl+C to stop.")
        print("[INFO] This replicates DbgPrint output from the minifilter in userland.")
        
        # Keep main thread alive
        try:
            while receive_thread.is_alive():
                time.sleep(0.1)
        except KeyboardInterrupt:
            print("\n[INFO] Shutting down...")
    
    finally:
        client.disconnect()
    
    return 0

if __name__ == "__main__":
    sys.exit(main())