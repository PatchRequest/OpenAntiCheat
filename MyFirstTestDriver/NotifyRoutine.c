#include "NotifyRoutine.h"

VOID CreateProcessNotifyRoutineEx(
	_Inout_ PEPROCESS Process,
	_In_ HANDLE ProcessId,
	_In_opt_ PPS_CREATE_NOTIFY_INFO CreateInfo
) {
	UNREFERENCED_PARAMETER(Process);
	UNREFERENCED_PARAMETER(ProcessId);


	if (CreateInfo) {

		// Process is being created
		if (CreateInfo->ImageFileName) {
			DbgPrintEx(DPFLTR_IHVDRIVER_ID, DPFLTR_ERROR_LEVEL,
				"Process created: %wZ (PID: %d)\n",
				CreateInfo->ImageFileName,
				(ULONG)(ULONG_PTR)ProcessId
			);
		}
		else {
			DbgPrintEx(DPFLTR_IHVDRIVER_ID, DPFLTR_ERROR_LEVEL,
				"Process created with unknown image name (PID: %d)\n",
				(ULONG)(ULONG_PTR)ProcessId
			);
		}
	}
	else {
		// Process is being terminated
		DbgPrintEx(DPFLTR_IHVDRIVER_ID, DPFLTR_ERROR_LEVEL,
			"Process terminated (PID: %d)\n",
			(ULONG)(ULONG_PTR)ProcessId
		);
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
	// You can implement thread creation/termination handling here if needed

	// print  the thread creation/termination event
	if (Create) {
		DbgPrintEx(DPFLTR_IHVDRIVER_ID, DPFLTR_ERROR_LEVEL,
			"Thread created (TID: %d) in process (PID: %d)\n",
			(ULONG)(ULONG_PTR)ThreadId,
			(ULONG)(ULONG_PTR)ProcessId
		);
	}
	else {
		DbgPrintEx(DPFLTR_IHVDRIVER_ID, DPFLTR_ERROR_LEVEL,
			"Thread terminated (TID: %d) in process (PID: %d)\n",
			(ULONG)(ULONG_PTR)ThreadId,
			(ULONG)(ULONG_PTR)ProcessId
		);
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

