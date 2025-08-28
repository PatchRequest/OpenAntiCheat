#pragma once
// ComsWrap.h
#pragma once
#include "Coms.h"

// fire-and-forget UM notification (kernel -> user), optional timeout (ms)
__forceinline NTSTATUS FpNotifyUser(_In_z_ const wchar_t* text, _In_ ULONG timeout_ms) {
    USER_TO_FLT msg = { 0 };
    if (!text) text = L"";
    wcsncpy_s(msg.msg, RTL_NUMBER_OF(msg.msg), text, _TRUNCATE);

    // fire-and-forget => recv_buffer == NULL
    ULONG ignored = 0;
    UNREFERENCED_PARAMETER(ignored);
    return MinifltPortSendMessage(&msg, sizeof(msg), NULL, 0, &ignored);
}

// request-reply (kernel -> user -> kernel), copies reply text into outBuf
__forceinline NTSTATUS FpQueryUser(
    _In_z_ const wchar_t* question,
    _Out_writes_(outCch) wchar_t* outBuf,
    _In_ size_t outCch
) {
    if (!outBuf || outCch == 0) return STATUS_INVALID_PARAMETER;

    USER_TO_FLT req = { 0 };
    USER_TO_FLT_REPLY rep = { 0 };
    ULONG repBytes = sizeof(rep);

    if (!question) question = L"";
    wcsncpy_s(req.msg, RTL_NUMBER_OF(req.msg), question, _TRUNCATE);

    NTSTATUS st = MinifltPortSendMessage(&req, sizeof(req), &rep, sizeof(rep), &repBytes);
    if (NT_SUCCESS(st)) {
        wcsncpy_s(outBuf, outCch, rep.msg, _TRUNCATE);
    }
    return st;
}

// typed push used by your filter path decision (kernel -> user expects decision back)
__forceinline NTSTATUS FpAskBlockDecision(
    _In_z_ const wchar_t* path,
    _Out_ BOOLEAN* block
) {
    if (!block) return STATUS_INVALID_PARAMETER;

    FLT_TO_USER req = { 0 };
    FLT_TO_USER_REPLY rep = { 0 };
    ULONG repBytes = sizeof(rep);

    if (!path) path = L"";
    wcsncpy_s(req.path, RTL_NUMBER_OF(req.path), path, _TRUNCATE);

    NTSTATUS st = MinifltPortSendMessage(&req, sizeof(req), &rep, sizeof(rep), &repBytes);
    if (NT_SUCCESS(st)) {
        *block = (rep.block != 0);
    }
    return st;
}

// low-level convenience: raw bytes with optional reply buffer
__forceinline NTSTATUS FpSendRaw(
    _In_reads_bytes_(cbSend) const void* pSend, _In_ ULONG cbSend,
    _Out_writes_bytes_opt_(cbReply) void* pReply, _In_ ULONG cbReply, _Out_opt_ ULONG* pcbReplyWritten
) {
    ULONG written = 0;
    NTSTATUS st = MinifltPortSendMessage((PVOID)pSend, cbSend, pReply, cbReply, &written);
    if (pcbReplyWritten) *pcbReplyWritten = written;
    return st;
}
