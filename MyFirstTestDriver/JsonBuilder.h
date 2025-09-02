#pragma once
#pragma once
#include <ntifs.h>
#include "ntstrsafe.h"

#define JSON_MAX_FIELDS 32
#define JSON_FIELD_NAME_LEN 64
#define JSON_FIELD_VALUE_LEN 256
#define JSON_OUTPUT_BUFFER_SIZE 2048

typedef enum _JSON_FIELD_TYPE {
    JSON_TYPE_STRING,
    JSON_TYPE_NUMBER,
    JSON_TYPE_BOOLEAN
} JSON_FIELD_TYPE;

typedef struct _JSON_FIELD {
    WCHAR name[JSON_FIELD_NAME_LEN];
    WCHAR value[JSON_FIELD_VALUE_LEN];
    JSON_FIELD_TYPE type;
    BOOLEAN used;
} JSON_FIELD, * PJSON_FIELD;

typedef struct _JSON_BUILDER {
    JSON_FIELD fields[JSON_MAX_FIELDS];
    ULONG fieldCount;
    WCHAR outputBuffer[JSON_OUTPUT_BUFFER_SIZE];
} JSON_BUILDER, * PJSON_BUILDER;

// Initialize JSON builder
VOID JsonBuilder_Init(_Out_ PJSON_BUILDER builder);

// Add string field
NTSTATUS JsonBuilder_AddString(
    _Inout_ PJSON_BUILDER builder,
    _In_z_ const WCHAR* fieldName,
    _In_z_ const WCHAR* value
);

// Add number field (from ULONG)
NTSTATUS JsonBuilder_AddNumber(
    _Inout_ PJSON_BUILDER builder,
    _In_z_ const WCHAR* fieldName,
    _In_ ULONG value
);

// Add boolean field
NTSTATUS JsonBuilder_AddBoolean(
    _Inout_ PJSON_BUILDER builder,
    _In_z_ const WCHAR* fieldName,
    _In_ BOOLEAN value
);

// Build final JSON string
NTSTATUS JsonBuilder_Build(_Inout_ PJSON_BUILDER builder, _Out_ const WCHAR** jsonOutput);