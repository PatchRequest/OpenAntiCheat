#pragma once
#include <ntifs.h>
#include "ComsWarp.h"
#include "ComsEvents.h"


VOID CreateProcessNotifyRoutineEx(
	_Inout_ PEPROCESS Process,
	_In_ HANDLE ProcessId,
	_In_opt_ PPS_CREATE_NOTIFY_INFO CreateInfo
);

NTSTATUS registerProcessNotifyRoutine();
void unregisterProcessNotifyRoutine();

