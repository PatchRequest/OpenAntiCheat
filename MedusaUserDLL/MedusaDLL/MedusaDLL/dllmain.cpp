// dllmain.cpp
#include <windows.h>
#include "MinHook.h"
#include <stdio.h>
#pragma comment(lib, "user32.lib")

typedef HANDLE(WINAPI* CreateRemoteThreadEx_t)(
    HANDLE, LPSECURITY_ATTRIBUTES, SIZE_T, LPTHREAD_START_ROUTINE,
    LPVOID, DWORD, LPPROC_THREAD_ATTRIBUTE_LIST, LPDWORD);

static CreateRemoteThreadEx_t RealCreateRemoteThreadEx = nullptr;
static volatile LONG g_inited = 0;



extern "C" int SendIPCMessage(const char* msg); // from IPC.c (compile separately)
extern "C" int SendIPCMessageRaw(const void* data, size_t len);

static void EnsureConsole(void) {
    static BOOL inited = FALSE; if (inited) return; inited = TRUE;
    if (!AttachConsole(ATTACH_PARENT_PROCESS)) AllocConsole();
    FILE* f;
    freopen_s(&f, "CONOUT$", "w", stdout);
    freopen_s(&f, "CONOUT$", "w", stderr);
    setvbuf(stdout, NULL, _IONBF, 0);
    setvbuf(stderr, NULL, _IONBF, 0);
}



enum EventSource {
    KM = 0,
    UM = 1,
    DLL = 2
};


typedef struct _ACEvent {
    enum EventSource src;
    wchar_t EventType[260]; // String Like CreateProcess
    int CallerPID;
    int TargetPID;
    int ThreadID;
    int ImageFileName[260];
    wchar_t CommandLine[1024];
    int IsCreate;
    PVOID ImageBase;
    ULONG ImageSize;
} ACEvent, * PACEvent;

void SendTestEvent(void) {
    ACEvent ev = { 0 };

    ev.src = DLL;
    wcsncpy_s(ev.EventType, _countof(ev.EventType), L"TestEvent", _TRUNCATE);
    ev.CallerPID = (int)GetCurrentProcessId();
    ev.TargetPID = ev.CallerPID;
    ev.ThreadID = GetCurrentThreadId();
    ev.IsCreate = 1;
    ev.ImageBase = (PVOID)0xDEADBEEF;   // dummy
    ev.ImageSize = 1234;                // dummy

    // optional: set some command line text
    wcsncpy_s(ev.CommandLine, _countof(ev.CommandLine), L"echo hello world", _TRUNCATE);

    SendIPCMessageRaw(&ev, sizeof(ev));
}


static HANDLE WINAPI HookCreateRemoteThreadEx(
    HANDLE hProcess, LPSECURITY_ATTRIBUTES sa, SIZE_T stackSize,
    LPTHREAD_START_ROUTINE start, LPVOID param, DWORD flags,
    LPPROC_THREAD_ATTRIBUTE_LIST attrList, LPDWORD tid)
{
    HANDLE h = RealCreateRemoteThreadEx
        ? RealCreateRemoteThreadEx(hProcess, sa, stackSize, start, param, flags, attrList, tid)
        : NULL;

    ACEvent ev = { 0 };
    ev.src = DLL;
    wcsncpy_s(ev.EventType, _countof(ev.EventType), L"CreateRemoteThreadEx", _TRUNCATE);
    ev.CallerPID = (int)GetCurrentProcessId();
    ev.TargetPID = (int)GetProcessId(hProcess);
    ev.ThreadID = (tid && *tid) ? (int)(*tid) : (h ? (int)GetThreadId(h) : 0);
    // Optional: capture our own cmdline (caller context)
    // wcsncpy_s(ev.CommandLine, _countof(ev.CommandLine), GetCommandLineW(), _TRUNCATE);
    ev.IsCreate = (h != NULL);
    ev.ImageBase = (PVOID)start;   // remote start routine
    ev.ImageSize = 0;

    // ImageFileName left zeroed (int[260] per your typedef)

    SendIPCMessageRaw(&ev, sizeof(ev));
    return h;
}

static void HookExport(HMODULE mod, LPCSTR name, void** pReal, void* hook) {
    if (!mod) return;
    if (void* p = (void*)GetProcAddress(mod, name)) {
        if (MH_CreateHook(p, hook, pReal) == MH_OK) MH_EnableHook(p);
    }
}


static DWORD WINAPI InitThread(LPVOID) {
    // init once
    if (InterlockedCompareExchange(&g_inited, 1, 0) != 0) return 0;

    if (MH_Initialize() != MH_OK) { return 0; }
    EnsureConsole();
    printf("[medusa] DLL loaded in PID %lu\n", GetCurrentProcessId());
    SendTestEvent();

    HMODULE kb = GetModuleHandleW(L"KernelBase.dll");
    HMODULE k32 = GetModuleHandleW(L"kernel32.dll");

    HookExport(kb, "CreateRemoteThreadEx", (void**)&RealCreateRemoteThreadEx, (void*)HookCreateRemoteThreadEx);
    if (!RealCreateRemoteThreadEx)
        HookExport(k32, "CreateRemoteThreadEx", (void**)&RealCreateRemoteThreadEx, (void*)HookCreateRemoteThreadEx);


    return 0;
}

extern "C" __declspec(dllexport)
void CALLBACK Start(HWND, HINSTANCE, LPSTR, int) {
    // spin up the hook installer
    HANDLE th = CreateThread(NULL, 0, InitThread, NULL, 0, NULL);
    if (th) CloseHandle(th);

    // keep DLL resident for testing with rundll32
    // you can replace with WaitForSingleObject on a global event for clean shutdown
    Sleep(INFINITE);
}

BOOL APIENTRY DllMain(HMODULE h, DWORD reason, LPVOID) {
    if (reason == DLL_PROCESS_ATTACH) {
        DisableThreadLibraryCalls(h);
        // ensure init when injected via LoadLibrary
        HANDLE th = CreateThread(NULL, 0, InitThread, NULL, 0, NULL);
        if (th) CloseHandle(th);
    }
    else if (reason == DLL_PROCESS_DETACH) {
        MH_Uninitialize();
        InterlockedExchange(&g_inited, 0);
    }
    return TRUE;
}