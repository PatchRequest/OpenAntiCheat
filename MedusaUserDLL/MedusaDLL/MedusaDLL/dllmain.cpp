// dllmain.cpp
#include <windows.h>
#include "MinHook.h"
#pragma comment(lib, "user32.lib")

typedef LONG NTSTATUS;
#define STATUS_ACCESS_DENIED ((NTSTATUS)0xC0000022L)

typedef struct _UNICODE_STRING { USHORT Length, MaximumLength; PWSTR Buffer; } UNICODE_STRING, * PUNICODE_STRING;
typedef struct _OBJECT_ATTRIBUTES {
    ULONG Length; HANDLE RootDirectory; PUNICODE_STRING ObjectName;
    ULONG Attributes; PVOID SecurityDescriptor; PVOID SecurityQualityOfService;
} OBJECT_ATTRIBUTES, * POBJECT_ATTRIBUTES;

typedef NTSTATUS(NTAPI* NtCreateThreadEx_t)(
    PHANDLE, ACCESS_MASK, POBJECT_ATTRIBUTES, HANDLE, PVOID, PVOID,
    ULONG, SIZE_T, SIZE_T, SIZE_T, PVOID);

static NtCreateThreadEx_t RealNtCreateThreadEx = nullptr;

static DWORD WINAPI MsgThread(LPVOID) {
    MessageBoxW(NULL, L"Blocked NtCreateThreadEx", L"AntiCheat", MB_OK | MB_TOPMOST);
    return 0;
}

static NTSTATUS NTAPI HookNtCreateThreadEx(
    PHANDLE ThreadHandle, ACCESS_MASK DesiredAccess, POBJECT_ATTRIBUTES ObjectAttributes,
    HANDLE ProcessHandle, PVOID StartRoutine, PVOID Argument, ULONG CreateFlags,
    SIZE_T ZeroBits, SIZE_T StackSize, SIZE_T MaxStackSize, PVOID AttributeList)
{
    // async popup (don’t call MessageBox on the hooking thread)
    HANDLE th = CreateThread(NULL, 0, MsgThread, NULL, 0, NULL);
    if (th) CloseHandle(th);

    if (ThreadHandle) *ThreadHandle = NULL;  // behave like failure
    return STATUS_ACCESS_DENIED;             // cancel the syscall
    // (Remove the return above and call RealNtCreateThreadEx(...) if you want to allow it)
}

static void HookOne(LPCSTR name, void** pReal, void* hook) {
    void* p = GetProcAddress(GetModuleHandleW(L"ntdll.dll"), name);
    if (!p) return;
    if (MH_CreateHook(p, hook, pReal) == MH_OK) {
        MH_EnableHook(p);
    }
}

static DWORD WINAPI InitThread(LPVOID) {
    if (MH_Initialize() != MH_OK) return 0;
    HookOne("NtCreateThreadEx", (void**)&RealNtCreateThreadEx, (void*)HookNtCreateThreadEx);
    return 0; // keep DLL loaded
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
