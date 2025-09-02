#include "Callback.h"
#define PROCESS_VM_WRITE 0x0020
PVOID callbackRegistrationHandle = NULL;

OB_PREOP_CALLBACK_STATUS CreateCallback(PVOID RegistrationContext, POB_PRE_OPERATION_INFORMATION OperationInformation) {
    UNREFERENCED_PARAMETER(RegistrationContext);
    UNREFERENCED_PARAMETER(OperationInformation);

    PEPROCESS Process = (PEPROCESS)OperationInformation->Object;
    if (OperationInformation->KernelHandle == 1) {
        return OB_PREOP_SUCCESS;
    }
    HANDLE pid = PsGetProcessId(Process);
    if (OperationInformation->Operation == OB_OPERATION_HANDLE_DUPLICATE) {
        return OB_PREOP_SUCCESS;
    }

    if (OperationInformation->Operation == OB_OPERATION_HANDLE_CREATE) {
        POB_PRE_CREATE_HANDLE_INFORMATION preInfo = &OperationInformation->Parameters->CreateHandleInformation;
        if (!(preInfo->DesiredAccess & PROCESS_VM_WRITE)) {
            return OB_PREOP_SUCCESS;
        }
    }

    DbgPrintEx(DPFLTR_IHVDRIVER_ID, DPFLTR_ERROR_LEVEL,
        "CreateCallback called with operation %d on process at address %p with PID %d\n",
        OperationInformation->Operation,
        Process,
        pid
    );

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