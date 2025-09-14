#pragma once
#include <fltKernel.h>
#include "ComsWarp.h"



enum EventSource {
	KM = 0,
	UM = 1,
	DLL = 2
};


typedef struct _ACEvent {
	enum EventSource src;
	wchar_t EventType[260]; // String Like CreateProcess
	int CallerPID;
	int TargetPID;
	int ThreadID;
	int ImageFileName[260];
	wchar_t CommandLine[1024];
	int IsCreate;
	PVOID ImageBase;
	ULONG ImageSize;
} ACEvent, * PACEvent;