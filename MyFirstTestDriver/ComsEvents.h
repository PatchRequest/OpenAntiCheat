#pragma once
#include <fltKernel.h>
#include "ComsWarp.h"

typedef struct _CreateProcessNotifyRoutineEvent {

	int isCreate; // 1 = create, 0 = exit
	int ProcessId;
	wchar_t ImageFileName[260];

} CreateProcessNotifyRoutineEvent, * PCreateProcessNotifyRoutineEvent;


typedef struct _OB_OPERATION_HANDLE_Event {
	int operation; // OB_OPERATION_HANDLE_CREATE = 0, OB_OPERATION_HANDLE_DUPLICATE = 1
	int ProcessId;
} OB_OPERATION_HANDLE_Event, * POB_OPERATION_HANDLE_Event;


typedef struct _FLT_PREOP_CALLBACK_Event {
	PFLT_CALLBACK_DATA Data;
} FLT_PREOP_CALLBACK_Event, * PFLT_PREOP_CALLBACK_Event;