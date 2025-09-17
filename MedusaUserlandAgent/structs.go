package main

import (
	"fmt"
	"os"
	"unsafe"

	"golang.org/x/sys/windows"
)

// FILTER_MESSAGE_HEADER (FltUser.h)
type filterMessageHeader struct {
	ReplyLength uint32
	MessageId   uint64
}

type Receiver struct {
	hPort windows.Handle
	name  string
}

func NewReceiver(portName string) *Receiver { return &Receiver{name: portName} }

type ConnectCtxV1 struct {
	Version      uint32 // =1
	ToProtectPID int32  // target
	AgentPID     int32  // this agent
}

func (r *Receiver) Connect() error {
	if err := modFltlib.Load(); err != nil {
		return err
	}
	u16, err := windows.UTF16PtrFromString(r.name)
	if err != nil {
		return err
	}

	ctx := ConnectCtxV1{
		Version:      1,
		ToProtectPID: int32(ToProtectPID),
		AgentPID:     int32(os.Getpid()),
	}

	var h windows.Handle
	ret, _, _ := pConnect.Call(
		uintptr(unsafe.Pointer(u16)),  // LPCWSTR PortName
		0,                             // DWORD Options
		uintptr(unsafe.Pointer(&ctx)), // PVOID  Context
		uintptr(unsafe.Sizeof(ctx)),   // WORD   SizeOfContext
		0,                             // PSECURITY_ATTRIBUTES
		uintptr(unsafe.Pointer(&h)),   // HANDLE* Port
	)
	if ret != S_OK {
		return fmt.Errorf("FilterConnectCommunicationPort failed: 0x%08X %s",
			uint32(ret), hresultText(ret))
	}

	r.hPort = h
	fmt.Printf("[OK] Connected %s sent target=%d agent=%d\n",
		r.name, ctx.ToProtectPID, ctx.AgentPID)
	return nil
}

func (r *Receiver) Close() {
	if r.hPort != 0 {
		pClose.Call(uintptr(r.hPort))
		r.hPort = 0
	}
}

func hresultText(hr uintptr) string {
	code := uint32(hr)
	// map HRESULT from 0x80070000 to Win32
	if (hr & 0xFFFF0000) == 0x80070000 {
		code = uint32(hr & 0xFFFF)
	}
	var buf [512]uint16
	r0, _, _ := pFmtMsgW.Call(
		FMT_FROM_SYSTEM|FMT_IGNORE_INSERTS,
		0, uintptr(code), 0,
		uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)), 0,
	)
	if r0 == 0 {
		return ""
	}
	return windows.UTF16ToString(buf[:])
}

// ------------------- utility functions -------------------
