#include "NotifyRoutine.h"

VOID CreateProcessNotifyRoutineEx(
	_Inout_ PEPROCESS Process,
	_In_ HANDLE ProcessId,
	_In_opt_ PPS_CREATE_NOTIFY_INFO CreateInfo
) {
	UNREFERENCED_PARAMETER(Process);
	UNREFERENCED_PARAMETER(ProcessId);
		


	ACEvent event = { 0 };
	event.src = 0;
	wcscpy_s(event.EventType, 260, L"CreateProcess");
	event.CallerPID = (int)(ULONG_PTR)PsGetCurrentProcessId();
	event.TargetPID = (int)(ULONG_PTR)ProcessId;
	event.ThreadID = PsGetCurrentThreadId();
	event.IsCreate = (CreateInfo != NULL) ? 1 : 0;
	event.ImageBase = (PVOID)0xffffffffffff;
	event.ImageSize = 0xffff;

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
	

	ACEvent event = { 0 };
	event.src = 0;
	wcscpy_s(event.EventType, 260, L"CreateThread");
	event.CallerPID = (int)(ULONG_PTR)PsGetCurrentProcessId();
	event.TargetPID = (int)(ULONG_PTR)ProcessId;
	event.ThreadID = ThreadId;
	wcscpy_s(event.ImageFileName, 260, L"");
	wcscpy_s(event.CommandLine, 1024, L"");
	event.IsCreate = Create ? 1 : 0;
	event.ImageBase = (PVOID)0xffffffffffff;
	event.ImageSize = 0xffff;


	ULONG sentBytes = 0;
	NTSTATUS status = FpSendRaw(&event, sizeof(event), NULL, 0, &sentBytes);
	if (!NT_SUCCESS(status)) {
		//DbgPrintEx(DPFLTR_IHVDRIVER_ID, DPFLTR_ERROR_LEVEL,"FpSendRaw failed with status 0x%08X\n", status);
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

	ACEvent event = { 0 };
	event.src = 0;
	wcscpy_s(event.EventType, 260, L"LoadImage");
	event.CallerPID = (int)(ULONG_PTR)PsGetCurrentProcessId();
	event.TargetPID = (int)(ULONG_PTR)ProcessId;
	event.ThreadID = PsGetCurrentThreadId();
	event.IsCreate = 1;
	wcscpy_s(event.CommandLine, 1024, L"");
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