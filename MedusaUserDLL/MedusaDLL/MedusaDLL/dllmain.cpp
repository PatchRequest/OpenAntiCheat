// dllmain.cpp
#include <windows.h>
#include "pch.h"
#pragma comment(lib, "user32.lib")

static DWORD WINAPI Worker(LPVOID mod) {
    MessageBoxW(NULL, L"Injected", L"Injection works", MB_OK | MB_TOPMOST);
    FreeLibraryAndExitThread((HMODULE)mod, 0);
    return 0; // never reached
}

BOOL APIENTRY DllMain(HMODULE hModule, DWORD reason, LPVOID) {
    if (reason == DLL_PROCESS_ATTACH) {
        DisableThreadLibraryCalls(hModule);
        HANDLE th = CreateThread(NULL, 0, Worker, hModule, 0, NULL);
        if (th) CloseHandle(th);
    }
    return TRUE;
}
