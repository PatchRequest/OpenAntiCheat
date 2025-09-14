# Medusa AntiCheat

<p align="center">
  <img src="Medusa.png" alt="Medusa" width="400"/>
</p>

Medusa AntiCheat is a proof-of-concept anti-cheat system built for **learning Windows kernel development** and experimenting with advanced process monitoring and injection detection techniques.

## System Architecture

The system consists of three main components:

### 1. **Kernel Driver** (`MedusaKernelDriver`)
- **ObCallbacks**: Monitor handle operations and object access
- **Notify Routines**: Track process/thread creation and image loading
- **Minifilter**: File system operation monitoring
- **Communication Port**: Bi-directional IPC with userland via `\MedusaComPort`

### 2. **Userland Agent** (`MedusaUserlandAgent`)
- **Event Processing**: Receives and processes kernel events in real-time
- **DLL Injection**: Automatically injects monitoring DLL into new processes
- **WebSocket Client**: Streams events to backend for analysis and storage
- **Process Scoring**: Evaluates suspicious behavior patterns

### 3. **Monitoring DLL** (`MedusaUserDLL`)
- **API Hooking**: Uses MinHook to intercept critical Windows APIs
- **IPC Communication**: Reports hooked function calls back to userland agent
- **Injection Detection**: Monitors `CreateRemoteThreadEx` and other injection APIs

## Current Detection Capabilities

### Kernel-Level Monitoring
- **Process Creation/Termination**: Track all process lifecycle events
- **Thread Creation**: Monitor thread creation across all processes
- **Handle Operations**: Detect suspicious handle access patterns
- **Image Loading**: Monitor DLL/module loads via LoadImage notifications
- **File System Activity**: Track file operations through minifilter

### Userland Hooking
- **CreateRemoteThreadEx**: Detect DLL injection attempts
- **LoadLibraryW**: Monitor dynamic library loading
- **Process Memory Operations**: Track `VirtualAllocEx`, `WriteProcessMemory`

### Event Streaming & Backend
- **Real-time Event Stream**: WebSocket connection to backend (`MedusaBackend`)
- **Image Load Detection**: Specialized detector for monitoring protected processes
- **Event Enrichment**: Add process metadata and context to events

## How It Works

1. **Initialization**: Userland agent takes target PID and DLL path as arguments
2. **Driver Communication**: Connects to kernel driver via minifilter port `\MedusaComPort`
3. **Process Injection**: Automatically injects monitoring DLL into all new processes
4. **Event Collection**: Kernel driver sends events (process, thread, handle, file operations)
5. **API Monitoring**: Injected DLL hooks critical APIs and reports calls via IPC
6. **Backend Streaming**: All events forwarded to WebSocket backend for analysis
7. **Real-time Detection**: Events processed and suspicious patterns flagged

## Event Types Monitored

- **`PROC_TAG`**: Process creation/termination events
- **`THREAD_TAG`**: Thread creation events
- **`OB_TAG`**: Object handle operation events
- **`FLT_TAG`**: File system minifilter events
- **`LOADIMG_TAG`**: Image/DLL load events

## Usage
```bash
# Compile and load kernel driver
# Run userland agent with target PID, WebSocket URL, and DLL path
./MedusaUserlandAgent.exe <target_pid> <websocket_url> <dll_path>
```  

## PoC Disclaimer
This project is not production-ready. A real anti-cheat would also need:
- Obfuscation and encryption
- Secure backend persistence
- Hardened communication

These are skipped here since the focus is on prototyping and experimenting with kernel ↔ userland interaction.

## Why?
Anti-cheat development combines kernel security, Windows internals, and attack surface analysis.  
This project is my playground to explore these concepts while building something functional.

---

⚠️ **Disclaimer**: This is a learning project. Do not expect production-grade security.  
