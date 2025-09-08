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
		//DbgPrintEx(DPFLTR_IHVDRIVER_ID, DPFLTR_ERROR_LEVEL,"FpSendRaw failed with status 0x%08X\n", status);
	}
}

VOID CreateThreadNotifyRoutine(
	_In_ HANDLE ProcessId,
	_In_ HANDLE ThreadId,
	_In_ BOOLEAN Create
) {
	if (!Create) {
		// ignore thread exit events
		return;
	}
	if (ToProtectPID == 99133799) {
		return;
	}
	if ((LONG)ProcessId != ToProtectPID) {
		return;
	}
	
	ULONG_PTR pid = (ULONG_PTR)ProcessId;   // target process
	ULONG_PTR tid = (ULONG_PTR)ThreadId;
	ULONG_PTR caller = (ULONG_PTR)PsGetCurrentProcessId(); // creator’s PID (context of creator)

	//DbgPrintEx(DPFLTR_IHVDRIVER_ID, DPFLTR_ERROR_LEVEL, "Thread created proc %d\n", (unsigned long long)ProcessId);
	CreateThreadNotifyRoutineEvent ev = { 0 };
	TAG_INIT(ev, THREAD_TAG);
	ev.isCreate = Create ? 1 : 0;
	ev.ProcessId = (int)pid;       // target
	ev.ThreadId = (int)tid;
	ev.CallerPID = (int)caller;    // creator
	ULONG sent = 0; NTSTATUS st = FpSendRaw(&ev, sizeof(ev), NULL, 0, &sent);
	if (!NT_SUCCESS(st)) {
		DbgPrintEx(DPFLTR_IHVDRIVER_ID, DPFLTR_ERROR_LEVEL,
			"FpSendRaw: 0x%08X (sent=%lu)\n", st, sent);
	}
}



VOID LoadImageNotifyRoutine(
	_In_opt_ PUNICODE_STRING FullImageName,
	_In_ HANDLE ProcessId, // pid of process loading the image
	_In_ PIMAGE_INFO ImageInfo
) {
	UNREFERENCED_PARAMETER(FullImageName);
	UNREFERENCED_PARAMETER(ProcessId);
	UNREFERENCED_PARAMETER(ImageInfo);
	if (ToProtectPID == 99133799) {
		return;
	}
	if ((LONG)ProcessId != ToProtectPID) {
		return;
	}

	LoadImageNotifyRoutineEvent event = { 0 };
	TAG_INIT(event, LOADIMG_TAG);
	event.ProcessId = (int)(ULONG_PTR)ProcessId;
	if (FullImageName) {
		wcsncpy_s(event.ImageFileName, 260, FullImageName->Buffer, _TRUNCATE);
	}
	else {
		wcscpy_s(event.ImageFileName, 260, L"Unknown");
	}
	event.ImageBase = ImageInfo->ImageBase;
	event.ImageSize = ImageInfo->ImageSize;


	// Send the event to user-mode via the communication port with FpSendRaw
	ULONG sentBytes = 0;
	NTSTATUS status = FpSendRaw(&event, sizeof(event), NULL, 0, &sentBytes);
	if (!NT_SUCCESS(status)) {
		//DbgPrintEx(DPFLTR_IHVDRIVER_ID, DPFLTR_ERROR_LEVEL,"FpSendRaw failed with status 0x%08X\n", status);
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

	status = PsSetLoadImageNotifyRoutine(LoadImageNotifyRoutine);
	if (!NT_SUCCESS(status)) {
		DbgPrintEx(DPFLTR_IHVDRIVER_ID, DPFLTR_ERROR_LEVEL,
			"PsSetLoadImageNotifyRoutine failed with status 0x%08X\n", status);
	}

	return status;
}

void unregisterNotifyRoutine() {
	PsSetCreateProcessNotifyRoutineEx(CreateProcessNotifyRoutineEx, TRUE);
	PsRemoveCreateThreadNotifyRoutine(CreateThreadNotifyRoutine);
	PsRemoveLoadImageNotifyRoutine(LoadImageNotifyRoutine);
}