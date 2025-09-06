#pragma once
#include <fltKernel.h>
#include "ComsWarp.h"

#define PROC_TAG 0
#define FLT_TAG  1
#define OB_TAG   2
#define THREAD_TAG 3
#define LOADIMG_TAG 4
#define TAG_INIT(ev, tag) do { \
    RtlZeroMemory(&(ev), sizeof(ev)); \
    (ev).reserved = (tag); \
} while (0)


typedef struct _CreateProcessNotifyRoutineEvent {
	int reserved; // always 0
	int isCreate; // 1 = create, 0 = exit
	int ProcessId;
	wchar_t ImageFileName[260];
	wchar_t CommandLine[1024];

} CreateProcessNotifyRoutineEvent, * PCreateProcessNotifyRoutineEvent;

typedef struct _FLT_PREOP_CALLBACK_Event {
	int reserved; // always 1
	int operation; // IRP_MJ_CREATE = 0x00, IRP_MJ_READ = 0x03, IRP_MJ_WRITE = 0x04, etc.
	int ProcessId;
	wchar_t FileName[260];
} FLT_PREOP_CALLBACK_Event, * PFLT_PREOP_CALLBACK_Event;


typedef struct _OB_OPERATION_HANDLE_Event {
	int reserved; // always 2
	int operation; // OB_OPERATION_HANDLE_CREATE = 0, OB_OPERATION_HANDLE_DUPLICATE = 1
	int ProcessId;
	int CallerPID;
} OB_OPERATION_HANDLE_Event, * POB_OPERATION_HANDLE_Event;

typedef struct _CreateThreadNotifyRoutineEvent {
	int reserved; // always 3
	int isCreate; // 1 = create, 0 = exit
	int ProcessId;
	int ThreadId;
	int CallerPID;
} CreateThreadNotifyRoutineEvent, * PCreateThreadNotifyRoutineEvent;


typedef struct _LoadImageNotifyRoutineEvent {
	int reserved; // always 4
	int ProcessId;
	wchar_t ImageFileName[260];
	PVOID ImageBase;
	ULONG ImageSize;
} LoadImageNotifyRoutineEvent, * PLoadImageNotifyRoutineEvent;