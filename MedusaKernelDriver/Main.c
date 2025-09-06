#include <ntifs.h>
#define DRIVER_TAG 'bdwh'
#include "Callback.h"
#include "Minifilter.h"
#include "NotifyRoutine.h"
#include "Coms.h"


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

    



    NTSTATUS st;
    st = createRegistration();
    DbgPrintEx(DPFLTR_IHVDRIVER_ID, DPFLTR_ERROR_LEVEL,
        "createRegistration() returned 0x%08X\n", st);

    st = createRegistrationMiniFilter(DriverObject);
    DbgPrintEx(DPFLTR_IHVDRIVER_ID, DPFLTR_ERROR_LEVEL,
        "createRegistrationMiniFilter() returned 0x%08X\n", st);

	st = registerNotifyRoutine();
	DbgPrintEx(DPFLTR_IHVDRIVER_ID, DPFLTR_ERROR_LEVEL,
		"registerNotifyRoutine() returned 0x%08X\n", st);

	st = BindToExistingFilterAndCreatePort(L"TestDriver");
	DbgPrintEx(DPFLTR_IHVDRIVER_ID, DPFLTR_ERROR_LEVEL,
		"createRegistrationComFilter() returned 0x%08X\n", st);


    return STATUS_SUCCESS;
}


NTSTATUS FLTAPI FilterUnloadCallback(_In_ FLT_FILTER_UNLOAD_FLAGS Flags)
{
    UNREFERENCED_PARAMETER(Flags);

    MinifltPortFinalize();

    if (g_minifilterHandle) {
        FltUnregisterFilter(g_minifilterHandle);
        g_minifilterHandle = NULL;
    }

    unregisterNotifyRoutine();

    ObUnRegisterCallbacks(callbackRegistrationHandle);

    DbgPrint("Bye Bye from HelloWorld Driver\n");
    return STATUS_SUCCESS;
}