#pragma once
#include <ntifs.h>
#ifndef CALLBACK_H
#define CALLBACK_H
PVOID callbackRegistrationHandle;
NTSTATUS createRegistration();

#endif
