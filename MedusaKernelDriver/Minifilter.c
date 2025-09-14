#include "Minifilter.h"

// Forward declarations (must be before tables)
FLT_PREOP_CALLBACK_STATUS FLTAPI PreOperationCreate(
    _Inout_ PFLT_CALLBACK_DATA Data,
    _In_ PCFLT_RELATED_OBJECTS FltObjects,
    _Flt_CompletionContext_Outptr_ PVOID* CompletionContext
);

NTSTATUS FLTAPI FilterUnloadCallback(_In_ FLT_FILTER_UNLOAD_FLAGS Flags);
NTSTATUS FLTAPI InstanceSetupCallback(
    _In_ PCFLT_RELATED_OBJECTS  FltObjects,
    _In_ FLT_INSTANCE_SETUP_FLAGS  Flags,
    _In_ DEVICE_TYPE  VolumeDeviceType,
    _In_ FLT_FILESYSTEM_TYPE  VolumeFilesystemType
);
NTSTATUS FLTAPI InstanceQueryTeardownCallback(
    _In_ PCFLT_RELATED_OBJECTS FltObjects,
    _In_ FLT_INSTANCE_QUERY_TEARDOWN_FLAGS Flags
);

PFLT_FILTER g_minifilterHandle = NULL;

CONST FLT_OPERATION_REGISTRATION g_callbacks[] = {
    { IRP_MJ_CREATE, 0, PreOperationCreate, NULL },
    { IRP_MJ_OPERATION_END, 0, NULL, NULL } // full initializer
};

CONST FLT_REGISTRATION g_filterRegistration = {
    sizeof(FLT_REGISTRATION),
    FLT_REGISTRATION_VERSION,
    0,                      // Flags
    NULL,                   // ContextRegistration
    g_callbacks,            // OperationRegistration
    FilterUnloadCallback,   // FilterUnloadCallback
    InstanceSetupCallback,  // InstanceSetupCallback
    InstanceQueryTeardownCallback, // InstanceQueryTeardownCallback
    NULL,                   // InstanceTeardownStartCallback
    NULL,                   // InstanceTeardownCompleteCallback
    NULL,                   // GenerateFileNameCallback
    NULL,                   // NormalizeNameComponentCallback
    NULL                    // NormalizeContextCleanupCallback
};

FLT_PREOP_CALLBACK_STATUS FLTAPI
PreOperationCreate(
    _Inout_ PFLT_CALLBACK_DATA Data,
    _In_ PCFLT_RELATED_OBJECTS FltObjects,
    _Flt_CompletionContext_Outptr_ PVOID* CompletionContext
)
{
    UNREFERENCED_PARAMETER(FltObjects);
    UNREFERENCED_PARAMETER(CompletionContext);

	if (Data->RequestorMode == KernelMode) {
		return FLT_PREOP_SUCCESS_NO_CALLBACK;
	}

    return FLT_PREOP_SUCCESS_NO_CALLBACK;
}

NTSTATUS createRegistrationMiniFilter(PDRIVER_OBJECT DriverObject) {
    NTSTATUS status;

    status = FltRegisterFilter(DriverObject, &g_filterRegistration, &g_minifilterHandle);
    DbgPrintEx(DPFLTR_IHVDRIVER_ID, DPFLTR_ERROR_LEVEL,
        "FltRegisterFilter returned 0x%08X\n", status);
    if (!NT_SUCCESS(status)) return status;

    status = FltStartFiltering(g_minifilterHandle);
    DbgPrintEx(DPFLTR_IHVDRIVER_ID, DPFLTR_ERROR_LEVEL,
        "FltStartFiltering returned 0x%08X\n", status);
    if (!NT_SUCCESS(status)) {
        FltUnregisterFilter(g_minifilterHandle);
    }

    return status;
}

NTSTATUS FLTAPI InstanceSetupCallback(
    _In_ PCFLT_RELATED_OBJECTS  FltObjects,
    _In_ FLT_INSTANCE_SETUP_FLAGS  Flags,
    _In_ DEVICE_TYPE  VolumeDeviceType,
    _In_ FLT_FILESYSTEM_TYPE  VolumeFilesystemType)
{
    UNREFERENCED_PARAMETER(FltObjects);
    UNREFERENCED_PARAMETER(Flags);
    UNREFERENCED_PARAMETER(VolumeDeviceType);
    UNREFERENCED_PARAMETER(VolumeFilesystemType);
    return STATUS_SUCCESS;
}

NTSTATUS FLTAPI InstanceQueryTeardownCallback(
    _In_ PCFLT_RELATED_OBJECTS FltObjects,
    _In_ FLT_INSTANCE_QUERY_TEARDOWN_FLAGS Flags)
{
    UNREFERENCED_PARAMETER(FltObjects);
    UNREFERENCED_PARAMETER(Flags);
    return STATUS_SUCCESS;
}

