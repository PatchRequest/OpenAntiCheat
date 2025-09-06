#pragma once
#include <ntifs.h>
#include "ComsEvents.h"

#ifndef CALLBACK_H
#define CALLBACK_H
PVOID callbackRegistrationHandle;
NTSTATUS createRegistration();

#endif
