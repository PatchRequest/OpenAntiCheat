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




static HANDLE WINAPI HookCreateRemoteThreadEx(
    HANDLE hProcess, LPSECURITY_ATTRIBUTES sa, SIZE_T stackSize,
    LPTHREAD_START_ROUTINE start, LPVOID param, DWORD flags,
    LPPROC_THREAD_ATTRIBUTE_LIST attrList, LPDWORD tid)
{
    HANDLE h = RealCreateRemoteThreadEx
        ? RealCreateRemoteThreadEx(hProcess, sa, stackSize, start, param, flags, attrList, tid)
        : NULL;
    SendIPCMessage("CreateRemoteThreadEx called");
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

    if (MH_Initialize() != MH_OK) { SendIPCMessage("MH_Initialize failed\n"); return 0; }

    HMODULE kb = GetModuleHandleW(L"KernelBase.dll");
    HMODULE k32 = GetModuleHandleW(L"kernel32.dll");

    HookExport(kb, "CreateRemoteThreadEx", (void**)&RealCreateRemoteThreadEx, (void*)HookCreateRemoteThreadEx);
    if (!RealCreateRemoteThreadEx)
        HookExport(k32, "CreateRemoteThreadEx", (void**)&RealCreateRemoteThreadEx, (void*)HookCreateRemoteThreadEx);

    if (RealCreateRemoteThreadEx)
        SendIPCMessage("Hooked CreateRemoteThreadEx\n");
    else
        SendIPCMessage("Failed to resolve CreateRemoteThreadEx\n");

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