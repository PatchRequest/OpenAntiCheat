// dllmain.cpp
#include <windows.h>
#include "MinHook.h"
#pragma comment(lib, "user32.lib")

typedef HANDLE(WINAPI* CreateRemoteThreadEx_t)(
    HANDLE, LPSECURITY_ATTRIBUTES, SIZE_T, LPTHREAD_START_ROUTINE,
    LPVOID, DWORD, LPPROC_THREAD_ATTRIBUTE_LIST, LPDWORD);

static CreateRemoteThreadEx_t RealCreateRemoteThreadEx = nullptr;
static volatile LONG g_inPopup = 0; // reentrancy guard (CreateThread may call CRTEx internally)

static DWORD WINAPI MsgThread(LPVOID) {
    MessageBoxW(NULL, L"CreateRemoteThreadEx allowed", L"AntiCheat", MB_OK | MB_TOPMOST);
    return 0;
}

static void SpawnPopupAsync() {
    if (InterlockedCompareExchange(&g_inPopup, 1, 0) == 0) {
        HANDLE th = CreateThread(NULL, 0, MsgThread, NULL, 0, NULL);
        if (th) CloseHandle(th);
        InterlockedExchange(&g_inPopup, 0);
    }
}

static HANDLE WINAPI HookCreateRemoteThreadEx(
    HANDLE hProcess,
    LPSECURITY_ATTRIBUTES sa,
    SIZE_T stackSize,
    LPTHREAD_START_ROUTINE start,
    LPVOID param,
    DWORD flags,
    LPPROC_THREAD_ATTRIBUTE_LIST attrList,
    LPDWORD tid)
{
    // forward (do not block)
    HANDLE h = RealCreateRemoteThreadEx
        ? RealCreateRemoteThreadEx(hProcess, sa, stackSize, start, param, flags, attrList, tid)
        : NULL;

    // async popup (guarded to avoid recursion)
    SpawnPopupAsync();
    return h;
}

static void HookExport(HMODULE mod, LPCSTR name, void** pReal, void* hook) {
    if (!mod) return;
    if (void* p = (void*)GetProcAddress(mod, name)) {
        if (MH_CreateHook(p, hook, pReal) == MH_OK) MH_EnableHook(p);
    }
}

static DWORD WINAPI InitThread(LPVOID) {
    if (MH_Initialize() != MH_OK) return 0;
    HMODULE hKernelBase = GetModuleHandleW(L"KernelBase.dll");
    HMODULE hKernel32 = GetModuleHandleW(L"kernel32.dll");

    HookExport(hKernelBase, "CreateRemoteThreadEx",
        (void**)&RealCreateRemoteThreadEx, (void*)HookCreateRemoteThreadEx);
    if (!RealCreateRemoteThreadEx) {
        HookExport(hKernel32, "CreateRemoteThreadEx",
            (void**)&RealCreateRemoteThreadEx, (void*)HookCreateRemoteThreadEx);
    }
    return 0;
}

BOOL APIENTRY DllMain(HMODULE hModule, DWORD reason, LPVOID) {
    if (reason == DLL_PROCESS_ATTACH) {
        DisableThreadLibraryCalls(hModule);
        HANDLE th = CreateThread(NULL, 0, InitThread, NULL, 0, NULL);
        if (th) CloseHandle(th);
    }
    else if (reason == DLL_PROCESS_DETACH) {
        MH_Uninitialize();
    }
    return TRUE;
}
