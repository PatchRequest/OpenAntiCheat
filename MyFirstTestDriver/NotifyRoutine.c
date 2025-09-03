#include "NotifyRoutine.h"

VOID CreateProcessNotifyRoutineEx(
	_Inout_ PEPROCESS Process,
	_In_ HANDLE ProcessId,
	_In_opt_ PPS_CREATE_NOTIFY_INFO CreateInfo
) {
	UNREFERENCED_PARAMETER(Process);
	UNREFERENCED_PARAMETER(ProcessId);
		
	CreateProcessNotifyRoutineEvent event = { 0 };
	TAG_INIT(event, PROC_TAG);
	event.isCreate = (CreateInfo != NULL) ? 1 : 0;
	event.ProcessId = (int)(ULONG_PTR)ProcessId;
	


	
	if (CreateInfo) {
		// Process is being created
		if (CreateInfo->ImageFileName) {
			event.CommandLine[0] = L'\0';
			event.ImageFileName[0] = L'\0';
			wcsncpy_s(event.ImageFileName, 260, CreateInfo->ImageFileName->Buffer, _TRUNCATE);
			wcsncpy_s(event.CommandLine, 1024, CreateInfo->CommandLine->Buffer, _TRUNCATE);
		}
		else {
			wcscpy_s(event.ImageFileName, 260, L"Unknown");
			wcscpy_s(event.CommandLine, 1024, L"Unknown");
		}
	}
	else {
		wcscpy_s(event.ImageFileName, 260, L"Terminated");
	}
	// Send the event to user-mode via the communication port with FpSendRaw
	ULONG sentBytes = 0;
	NTSTATUS status = FpSendRaw(&event, sizeof(event), NULL, 0, &sentBytes);
	if (!NT_SUCCESS(status)) {
		DbgPrintEx(DPFLTR_IHVDRIVER_ID, DPFLTR_ERROR_LEVEL,
			"FpSendRaw failed with status 0x%08X\n", status);
	}
}

VOID CreateThreadNotifyRoutine(
	_In_ HANDLE ProcessId,
	_In_ HANDLE ThreadId,
	_In_ BOOLEAN Create
) {
	UNREFERENCED_PARAMETER(ProcessId);
	UNREFERENCED_PARAMETER(ThreadId);
	UNREFERENCED_PARAMETER(Create);

	CreateThreadNotifyRoutineEvent event = { 0 };
	TAG_INIT(event, THREAD_TAG);
	event.isCreate = Create ? 1 : 0;
	event.ProcessId = (int)(ULONG_PTR)ProcessId;
	event.ThreadId = (int)(ULONG_PTR)ThreadId;
	// Send the event to user-mode via the communication port with FpSendRaw
	ULONG sentBytes = 0;
	NTSTATUS status = FpSendRaw(&event, sizeof(event), NULL, 0, &sentBytes);
	if (!NT_SUCCESS(status)) {
		DbgPrintEx(DPFLTR_IHVDRIVER_ID, DPFLTR_ERROR_LEVEL,
			"FpSendRaw failed with status 0x%08X\n", status);
	}
}


NTSTATUS registerNotifyRoutine() {
	NTSTATUS status = PsSetCreateProcessNotifyRoutineEx(CreateProcessNotifyRoutineEx, FALSE);
	if (!NT_SUCCESS(status)) {
		DbgPrintEx(DPFLTR_IHVDRIVER_ID, DPFLTR_ERROR_LEVEL,
			"PsSetCreateProcessNotifyRoutineEx failed with status 0x%08X\n", status);
	}
	status = PsSetCreateThreadNotifyRoutine(CreateThreadNotifyRoutine);
	if (!NT_SUCCESS(status)) {
 		DbgPrintEx(DPFLTR_IHVDRIVER_ID, DPFLTR_ERROR_LEVEL,
			"PsSetCreateThreadNotifyRoutine failed with status 0x%08X\n", status);
	}

	return status;
}

void unregisterNotifyRoutine() {
	PsSetCreateProcessNotifyRoutineEx(CreateProcessNotifyRoutineEx, TRUE);
	PsRemoveCreateThreadNotifyRoutine(CreateThreadNotifyRoutine);
}

