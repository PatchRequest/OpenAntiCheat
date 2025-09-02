#include "JsonBuilder.h"

VOID JsonBuilder_Init(_Out_ PJSON_BUILDER builder) {
    if (!builder) return;

    RtlZeroMemory(builder, sizeof(JSON_BUILDER));
    builder->fieldCount = 0;
}

NTSTATUS JsonBuilder_AddString(
    _Inout_ PJSON_BUILDER builder,
    _In_z_ const WCHAR* fieldName,
    _In_z_ const WCHAR* value
) {
    if (!builder || !fieldName || !value || builder->fieldCount >= JSON_MAX_FIELDS) {
        return STATUS_INVALID_PARAMETER;
    }

    PJSON_FIELD field = &builder->fields[builder->fieldCount];

    // Copy field name
    NTSTATUS status = RtlStringCchCopyW(field->name, JSON_FIELD_NAME_LEN, fieldName);
    if (!NT_SUCCESS(status)) return status;

    // Copy value
    status = RtlStringCchCopyW(field->value, JSON_FIELD_VALUE_LEN, value);
    if (!NT_SUCCESS(status)) return status;

    field->type = JSON_TYPE_STRING;
    field->used = TRUE;
    builder->fieldCount++;

    return STATUS_SUCCESS;
}

NTSTATUS JsonBuilder_AddNumber(
    _Inout_ PJSON_BUILDER builder,
    _In_z_ const WCHAR* fieldName,
    _In_ ULONG value
) {
    if (!builder || !fieldName || builder->fieldCount >= JSON_MAX_FIELDS) {
        return STATUS_INVALID_PARAMETER;
    }

    PJSON_FIELD field = &builder->fields[builder->fieldCount];

    // Copy field name
    NTSTATUS status = RtlStringCchCopyW(field->name, JSON_FIELD_NAME_LEN, fieldName);
    if (!NT_SUCCESS(status)) return status;

    // Convert number to string
    status = RtlStringCchPrintfW(field->value, JSON_FIELD_VALUE_LEN, L"%lu", value);
    if (!NT_SUCCESS(status)) return status;

    field->type = JSON_TYPE_NUMBER;
    field->used = TRUE;
    builder->fieldCount++;

    return STATUS_SUCCESS;
}

NTSTATUS JsonBuilder_AddBoolean(
    _Inout_ PJSON_BUILDER builder,
    _In_z_ const WCHAR* fieldName,
    _In_ BOOLEAN value
) {
    if (!builder || !fieldName || builder->fieldCount >= JSON_MAX_FIELDS) {
        return STATUS_INVALID_PARAMETER;
    }

    PJSON_FIELD field = &builder->fields[builder->fieldCount];

    // Copy field name
    NTSTATUS status = RtlStringCchCopyW(field->name, JSON_FIELD_NAME_LEN, fieldName);
    if (!NT_SUCCESS(status)) return status;

    // Set boolean value
    status = RtlStringCchCopyW(field->value, JSON_FIELD_VALUE_LEN, value ? L"true" : L"false");
    if (!NT_SUCCESS(status)) return status;

    field->type = JSON_TYPE_BOOLEAN;
    field->used = TRUE;
    builder->fieldCount++;

    return STATUS_SUCCESS;
}

NTSTATUS JsonBuilder_Build(_Inout_ PJSON_BUILDER builder, _Out_ const WCHAR** jsonOutput) {
    if (!builder || !jsonOutput) {
        return STATUS_INVALID_PARAMETER;
    }

    // Start JSON object
    NTSTATUS status = RtlStringCchCopyW(builder->outputBuffer, JSON_OUTPUT_BUFFER_SIZE, L"{");
    if (!NT_SUCCESS(status)) return status;

    for (ULONG i = 0; i < builder->fieldCount; i++) {
        PJSON_FIELD field = &builder->fields[i];
        if (!field->used) continue;

        WCHAR fieldJson[512];

        // Format field based on type
        if (field->type == JSON_TYPE_STRING) {
            // Strings need quotes around the value
            status = RtlStringCchPrintfW(fieldJson, RTL_NUMBER_OF(fieldJson),
                L"\"%s\":\"%s\"", field->name, field->value);
        }
        else {
            // Numbers and booleans don't need quotes around the value
            status = RtlStringCchPrintfW(fieldJson, RTL_NUMBER_OF(fieldJson),
                L"\"%s\":%s", field->name, field->value);
        }

        if (!NT_SUCCESS(status)) return status;

        // Add comma if not the first field
        if (i > 0) {
            status = RtlStringCchCatW(builder->outputBuffer, JSON_OUTPUT_BUFFER_SIZE, L",");
            if (!NT_SUCCESS(status)) return status;
        }

        // Append field to output
        status = RtlStringCchCatW(builder->outputBuffer, JSON_OUTPUT_BUFFER_SIZE, fieldJson);
        if (!NT_SUCCESS(status)) return status;
    }

    // Close JSON object
    status = RtlStringCchCatW(builder->outputBuffer, JSON_OUTPUT_BUFFER_SIZE, L"}");
    if (!NT_SUCCESS(status)) return status;

    *jsonOutput = builder->outputBuffer;
    return STATUS_SUCCESS;
}