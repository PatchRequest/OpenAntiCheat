#include "NotifyRoutine.h"

VOID CreateProcessNotifyRoutineEx(
	_Inout_ PEPROCESS Process,
	_In_ HANDLE ProcessId,
	_In_opt_ PPS_CREATE_NOTIFY_INFO CreateInfo
) {
	UNREFERENCED_PARAMETER(Process);
	UNREFERENCED_PARAMETER(ProcessId);


	// use CreateProcessNotifyRoutineEvent and FpSendRaw to send process creation/termination info to user-mode app
	(void)FpSendRaw(&(CreateProcessNotifyRoutineEvent) { Process, ProcessId, CreateInfo },
		sizeof(CreateProcessNotifyRoutineEvent), NULL, 0, NULL);



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

NTSTATUS registerProcessNotifyRoutine() {
	NTSTATUS status = PsSetCreateProcessNotifyRoutineEx(CreateProcessNotifyRoutineEx, FALSE);
	if (!NT_SUCCESS(status)) {
		DbgPrintEx(DPFLTR_IHVDRIVER_ID, DPFLTR_ERROR_LEVEL,
			"PsSetCreateProcessNotifyRoutineEx failed with status 0x%08X\n", status);
	}
	return status;
}

void unregisterProcessNotifyRoutine() {
	PsSetCreateProcessNotifyRoutineEx(CreateProcessNotifyRoutineEx, TRUE);
}

