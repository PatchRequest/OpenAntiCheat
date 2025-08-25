#include <ntifs.h>
#define DRIVER_TAG 'bdwh'
#include "Callback.h"
#include "Minifilter.h"
#include "NotifyRoutine.h"


UNICODE_STRING g_RegPath;

void UnloadMe(PDRIVER_OBJECT);

NTSTATUS DriverEntry(PDRIVER_OBJECT DriverObject, PUNICODE_STRING RegistryPath) {
    DbgPrint("HelloWorld from the Kernel Land!\n");
    DbgPrint("Driver Object:\t\t0x%p\n", DriverObject);
    DbgPrint("Registry Path:\t\t0x%p\n", RegistryPath);

    // Allocate memory for variableS
    g_RegPath.Buffer = (PWSTR)ExAllocatePool2(POOL_FLAG_PAGED, RegistryPath->Length, DRIVER_TAG);
    if (g_RegPath.Buffer == NULL) {
        DbgPrint("Error allocating memory!\n");
        return STATUS_NO_MEMORY;
    }

    // Copy Registry Path
    memcpy(g_RegPath.Buffer, RegistryPath->Buffer, RegistryPath->Length);
    g_RegPath.Length = g_RegPath.MaximumLength = RegistryPath->Length;
    DbgPrint("Parameter Key copy: %wZ\n", g_RegPath);

    // Unload Function
    DriverObject->DriverUnload = UnloadMe;



    NTSTATUS st;
    st = createRegistration();
    DbgPrintEx(DPFLTR_IHVDRIVER_ID, DPFLTR_ERROR_LEVEL,
        "createRegistration() returned 0x%08X\n", st);

    st = createRegistrationMiniFilter(DriverObject);
    DbgPrintEx(DPFLTR_IHVDRIVER_ID, DPFLTR_ERROR_LEVEL,
        "createRegistrationMiniFilter() returned 0x%08X\n", st);

	st = registerProcessNotifyRoutine();
	DbgPrintEx(DPFLTR_IHVDRIVER_ID, DPFLTR_ERROR_LEVEL,
		"registerProcessNotifyRoutine() returned 0x%08X\n", st);


    return STATUS_SUCCESS;
}

void UnloadMe(PDRIVER_OBJECT DriverObject) {
    UNREFERENCED_PARAMETER(DriverObject);
    if (callbackRegistrationHandle) {
        ObUnRegisterCallbacks(callbackRegistrationHandle);  // blocks until in-flight callbacks exit
        callbackRegistrationHandle = NULL;
    }

	if (g_minifilterHandle) {
		FltUnregisterFilter(g_minifilterHandle);
		g_minifilterHandle = NULL;
	}

	unregisterProcessNotifyRoutine();

    if (g_RegPath.Buffer != NULL) {
        ExFreePool(g_RegPath.Buffer);
    }

    DbgPrint("Bye Bye from HelloWorld Driver\n");
}