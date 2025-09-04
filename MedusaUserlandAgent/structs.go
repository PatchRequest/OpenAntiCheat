package main

import (
	"fmt"
	"unsafe"

	"golang.org/x/sys/windows"
)

// FILTER_MESSAGE_HEADER (FltUser.h)
type filterMessageHeader struct {
	ReplyLength uint32
	MessageId   uint64
}

// ----- kernel payloads -----
// int == int32 on Windows C ABI for these fields
type CreateProcessNotifyRoutineEvent struct {
	Reserved     int32
	IsCreate     int32
	ProcessID    int32
	ImageFileW   [260]uint16
	CommandLineW [1024]uint16
}

type FLT_PREOP_CALLBACK_Event struct {
	Reserved  int32
	Operation int32
	ProcessID int32
	FileNameW [260]uint16
}

type OB_OPERATION_HANDLE_Event struct {
	Reserved  int32
	Operation int32
	ProcessID int32
}

type CreateThreadNotifyRoutineEvent struct {
	Reserved  int32
	IsCreate  int32 // 1 create, 0 exit
	ProcessID int32
	ThreadID  int32
	CallerPID int32
}

type LoadImageNotifyRoutineEvent struct {
	Reserved    int32
	ProcessID   int32
	ImageFileW  [260]uint16
	ImageBase   uintptr // PVOID
	ImageSize   uint32
	_pad_align_ uint32 // align to 8 on 64-bit, harmless on 32-bit
}

// IRP majors
var irpMajor = map[int32]string{
	0x00: "CREATE", 0x01: "CREATE_NAMED_PIPE", 0x02: "CLOSE", 0x03: "READ", 0x04: "WRITE",
	0x05: "QUERY_INFORMATION", 0x06: "SET_INFORMATION", 0x07: "QUERY_EA", 0x08: "SET_EA",
	0x09: "FLUSH_BUFFERS", 0x0A: "QUERY_VOLUME_INFORMATION", 0x0B: "SET_VOLUME_INFORMATION",
	0x0C: "DIRECTORY_CONTROL", 0x0D: "FILE_SYSTEM_CONTROL", 0x0E: "DEVICE_CONTROL",
	0x0F: "INTERNAL_DEVICE_CONTROL", 0x10: "SHUTDOWN", 0x11: "LOCK_CONTROL",
	0x12: "CLEANUP", 0x13: "CREATE_MAILSLOT", 0x14: "QUERY_SECURITY", 0x15: "SET_SECURITY",
	0x16: "POWER", 0x17: "SYSTEM_CONTROL", 0x18: "DEVICE_CHANGE", 0x19: "QUERY_QUOTA",
	0x1A: "SET_QUOTA", 0x1B: "PNP",
}

var obOp = map[int32]string{0: "CREATE", 1: "DUPLICATE"}

type Receiver struct {
	hPort windows.Handle
	name  string
}

func NewReceiver(portName string) *Receiver { return &Receiver{name: portName} }

func (r *Receiver) Connect() error {
	if err := modFltlib.Load(); err != nil {
		return err
	}
	u16, err := windows.UTF16PtrFromString(r.name)
	if err != nil {
		return err
	}
	var h windows.Handle
	ret, _, _ := pConnect.Call(
		uintptr(unsafe.Pointer(u16)),
		0, 0, 0, 0,
		uintptr(unsafe.Pointer(&h)),
	)
	if ret != S_OK {
		return fmt.Errorf("FilterConnectCommunicationPort failed: 0x%08X %s", uint32(ret), hresultText(ret))
	}
	r.hPort = h
	fmt.Printf("[OK] Connected %s\n", r.name)
	return nil
}

func (r *Receiver) Close() {
	if r.hPort != 0 {
		pClose.Call(uintptr(r.hPort))
		r.hPort = 0
	}
}
