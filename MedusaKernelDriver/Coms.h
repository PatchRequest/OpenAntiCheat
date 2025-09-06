#pragma once
#include <fltKernel.h>

extern PFLT_FILTER flt_handle; 

NTSTATUS BindToExistingFilterAndCreatePort(_In_ PCWSTR FilterName);

NTSTATUS MinifltPortInitialize(_In_ PFLT_FILTER flt_handle);
VOID MinifltPortFinalize();

NTSTATUS MinifltPortNotifyRoutine(
	_In_ PFLT_PORT client_port,
	_In_ PVOID server_cookie,
	_In_ PVOID connection_context,
	_In_ ULONG connection_context_size,
	_Out_ PVOID* connection_port_cookie
);

VOID MinifltPortDisconnectRoutine(_In_ PVOID connection_cookie);

NTSTATUS MinifltPortMessageRoutine(
	_In_ PVOID port_cookie,
	_In_opt_ PVOID input_buffer,
	_In_ ULONG input_buffer_size,
	_Out_opt_ PVOID output_buffer,
	_In_ ULONG output_buffer_size,
	_Out_ PULONG return_output_buffer_length
);

NTSTATUS MinifltPortSendMessage(
	_In_ PVOID send_data,
	_In_ ULONG send_data_size,
	_Out_opt_ PVOID recv_buffer,
	_In_ ULONG recv_buffer_size,
	_Out_ PULONG written_bytes_to_recv_buffer
);


#define BUFFER_SIZE 4096 // page size
// this structure will be used when filter send message to user
typedef struct _FLT_TO_USER {

	wchar_t path[BUFFER_SIZE / sizeof(wchar_t)];

} FLT_TO_USER, * PFLT_TO_USER;

// this structure will be used when user reply to filter
typedef struct _FLT_TO_USER_REPLY {

	unsigned __int32 block; // if 1, file access will be denied

} FLT_TO_USER_REPLY, * PFLT_TO_USER_REPLY;

// this structure will be used when user send message to user
typedef struct _USER_TO_FLT {

	wchar_t msg[BUFFER_SIZE / sizeof(wchar_t)];

} USER_TO_FLT, * PUSER_TO_FLT;

// this structure will be used when filter reply to user
typedef struct _USER_TO_FLT_REPLY {

	wchar_t msg[BUFFER_SIZE / sizeof(wchar_t)];

} USER_TO_FLT_REPLY, * PUSER_TO_FLT_REPLY;


