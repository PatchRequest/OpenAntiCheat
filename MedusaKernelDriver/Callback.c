#include "Callback.h"
#define PROCESS_VM_READ       0x0010
#define PROCESS_VM_WRITE      0x0020
#define PROCESS_VM_OPERATION  0x0008
#ifndef PROCESS_QUERY_LIMITED_INFORMATION
#define PROCESS_QUERY_LIMITED_INFORMATION 0x1000 // Vista+
#endif
#define MINIMAL_ALLOW (PROCESS_QUERY_LIMITED_INFORMATION | SYNCHRONIZE)

PVOID callbackRegistrationHandle = NULL;



OB_PREOP_CALLBACK_STATUS CreateCallback(PVOID RegistrationContext, POB_PRE_OPERATION_INFORMATION OperationInformation) {
    UNREFERENCED_PARAMETER(RegistrationContext);
    UNREFERENCED_PARAMETER(OperationInformation);

    PEPROCESS Process = (PEPROCESS)OperationInformation->Object;
    if (OperationInformation->KernelHandle || OperationInformation->ObjectType != *PsProcessType) {
        return OB_PREOP_SUCCESS;
    }
    if (ToProtectPID == 99133799) {
        return OB_PREOP_SUCCESS;
    }

    
    HANDLE pid = PsGetProcessId(Process);
    if ((LONG)pid != ToProtectPID) {
        return OB_PREOP_SUCCESS;
    }
    
    HANDLE callerPID = PsGetCurrentProcessId();
    if (callerPID != AgentPID) {
        if (OperationInformation->Operation == OB_OPERATION_HANDLE_CREATE) {
            OperationInformation->Parameters->CreateHandleInformation.DesiredAccess &= MINIMAL_ALLOW;
        }
        else { // OB_OPERATION_HANDLE_DUPLICATE
            OperationInformation->Parameters->DuplicateHandleInformation.DesiredAccess &= MINIMAL_ALLOW;
        }
    }

    ACEvent event = { 0 };
    event.src = 0;
    wcscpy_s(event.EventType, 260, L"HandleOperation");
    event.CallerPID = (int)(ULONG_PTR)callerPID;
    event.TargetPID = (int)(ULONG_PTR)pid;
    event.ThreadID = PsGetCurrentThreadId();
    wcscpy_s(event.ImageFileName, 260, L"");
    wcscpy_s(event.CommandLine, 1024, L"");
    event.IsCreate = 1;
    event.ImageBase = (PVOID)0xffffffffffff;
    event.ImageSize = 0xffff;

	ULONG sentBytes = 0;
	NTSTATUS status = FpSendRaw(&event, sizeof(event), NULL, 0, &sentBytes);
	if (!NT_SUCCESS(status)) {
		DbgPrintEx(DPFLTR_IHVDRIVER_ID, DPFLTR_ERROR_LEVEL,"FpSendRaw failed with status 0x%08X\n", status);
	}
    return OB_PREOP_SUCCESS;
}

NTSTATUS createRegistration() {
    OB_CALLBACK_REGISTRATION registrationInfo;
    OB_OPERATION_REGISTRATION operationInfo;
    NTSTATUS status;

    RtlZeroMemory(&registrationInfo, sizeof(registrationInfo));
    RtlZeroMemory(&operationInfo, sizeof(operationInfo));

    operationInfo.ObjectType = PsProcessType;
    operationInfo.Operations = OB_OPERATION_HANDLE_CREATE | OB_OPERATION_HANDLE_DUPLICATE;
    operationInfo.PreOperation = CreateCallback;

    registrationInfo.Version = OB_FLT_REGISTRATION_VERSION;
    registrationInfo.OperationRegistrationCount = 1;
    registrationInfo.RegistrationContext = NULL;
    registrationInfo.OperationRegistration = &operationInfo;

    UNICODE_STRING altitude;
    RtlInitUnicodeString(&altitude, L"32897.8451");
    registrationInfo.Altitude = altitude;

    status = ObRegisterCallbacks(&registrationInfo, &callbackRegistrationHandle);
    if (!NT_SUCCESS(status)) {
        DbgPrintEx(DPFLTR_IHVDRIVER_ID, DPFLTR_ERROR_LEVEL, "ObRegisterCallbacks failed with status 0x%08X\n", status);
        return status;
    }

    DbgPrintEx(DPFLTR_IHVDRIVER_ID, DPFLTR_INFO_LEVEL, "Registered callback successfully\n");
    return STATUS_SUCCESS;
}