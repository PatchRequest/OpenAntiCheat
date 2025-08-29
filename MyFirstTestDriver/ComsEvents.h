#pragma once
#include <fltKernel.h>
#include "ComsWarp.h"

typedef struct _CreateProcessNotifyRoutineEvent {
	PEPROCESS Process;
	HANDLE ProcessId;
	PPS_CREATE_NOTIFY_INFO CreateInfo;

} CreateProcessNotifyRoutineEvent, * PCreateProcessNotifyRoutineEvent;


typedef struct _OB_OPERATION_HANDLE_Event {
	POB_PRE_OPERATION_INFORMATION OperationInformation;
	HANDLE pid;
} OB_OPERATION_HANDLE_Event, * POB_OPERATION_HANDLE_Event;


typedef struct _FLT_PREOP_CALLBACK_Event {
	PFLT_CALLBACK_DATA Data;
} FLT_PREOP_CALLBACK_Event, * PFLT_PREOP_CALLBACK_Event;