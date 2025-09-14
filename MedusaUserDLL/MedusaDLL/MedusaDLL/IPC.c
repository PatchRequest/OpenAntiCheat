#define UNICODE
#include <windows.h>
#include <stdio.h>

// default pipe name (must match Go server)
static const wchar_t* PIPE_NAME = L"\\\\.\\pipe\\MedusaPipe";

// Sends a null-terminated string to the named pipe server.
// Returns 1 on success, 0 on failure.
int SendIPCMessage(const char* msg) {
    HANDLE h = CreateFileW(
        PIPE_NAME,
        GENERIC_WRITE,
        0, NULL, OPEN_EXISTING, 0, NULL);

    if (h == INVALID_HANDLE_VALUE) {
        wprintf(L"CreateFileW failed: %lu\n", GetLastError());
        return 0;
    }

    DWORD written = 0;
    BOOL ok = WriteFile(h, msg, (DWORD)strlen(msg), &written, NULL);
    CloseHandle(h);

    if (!ok) {
        wprintf(L"WriteFile failed: %lu\n", GetLastError());
        return 0;
    }
    return 1;
}

int SendIPCMessageRaw(const void* data, size_t len) {
    HANDLE h = CreateFileW(
        PIPE_NAME,
        GENERIC_WRITE,
        0, NULL, OPEN_EXISTING, 0, NULL);

    if (h == INVALID_HANDLE_VALUE) {
        wprintf(L"CreateFileW failed: %lu\n", GetLastError());
        return 0;
    }

    DWORD written = 0;
    BOOL ok = WriteFile(h, data, (DWORD)len, &written, NULL);
    CloseHandle(h);

    if (!ok || written != len) {
        wprintf(L"WriteFile failed: %lu\n", GetLastError());
        return 0;
    }
    return 1;
}