//go:build windows

package main

import (
	"encoding/binary"
	"fmt"
	"os"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	S_OK        = 0x00000000
	PROC_TAG    = 0
	FLT_TAG     = 1
	OB_TAG      = 2
	THREAD_TAG  = 3
	LOADIMG_TAG = 4
)

var (
	modFltlib = windows.NewLazySystemDLL("fltlib.dll")
	modK32    = windows.NewLazySystemDLL("kernel32.dll")
	pConnect  = modFltlib.NewProc("FilterConnectCommunicationPort")
	pGetMsg   = modFltlib.NewProc("FilterGetMessage")
	pClose    = modK32.NewProc("CloseHandle")
	pFmtMsgW  = modK32.NewProc("FormatMessageW")
)

const (
	FMT_FROM_SYSTEM    = 0x00001000
	FMT_IGNORE_INSERTS = 0x00000200
)

var DetectRemotThreadChannel = make(chan any)
var HandleGuardChannel = make(chan any)
var DLLInjectionChannel = make(chan any)
var ToProtectPID int32 = 0

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

func u16zToString(buf []uint16) string {
	for i, v := range buf {
		if v == 0 {
			return windows.UTF16ToString(buf[:i])
		}
	}
	return windows.UTF16ToString(buf)
}

func (r *Receiver) Loop() {
	hdrSz := uint32(unsafe.Sizeof(filterMessageHeader{}))
	szProc := uint32(unsafe.Sizeof(CreateProcessNotifyRoutineEvent{}))
	szThr := uint32(unsafe.Sizeof(CreateThreadNotifyRoutineEvent{}))
	szFlt := uint32(unsafe.Sizeof(FLT_PREOP_CALLBACK_Event{}))
	szOb := uint32(unsafe.Sizeof(OB_OPERATION_HANDLE_Event{}))
	szImg := uint32(unsafe.Sizeof(LoadImageNotifyRoutineEvent{}))

	maxSz := szProc
	for _, s := range []uint32{szThr, szFlt, szOb, szImg} {
		if s > maxSz {
			maxSz = s
		}
	}
	buf := make([]byte, hdrSz+maxSz)

	for {
		ret, _, _ := pGetMsg.Call(
			uintptr(r.hPort),
			uintptr(unsafe.Pointer(&buf[0])),
			uintptr(uint32(len(buf))),
			0,
		)
		if ret != S_OK {
			fmt.Printf("[ERR] FilterGetMessage 0x%08X (%s)\n", uint32(ret), hresultText(ret))
			continue
		}
		// Parse header
		var hdr filterMessageHeader
		copy((*[unsafe.Sizeof(hdr)]byte)(unsafe.Pointer(&hdr))[:], buf[:hdrSz])

		payload := buf[hdrSz : hdrSz+maxSz]
		tag := int32(binary.LittleEndian.Uint32(payload[0:4]))
		var toSend interface{}
		switch tag {
		case PROC_TAG:
			var ev CreateProcessNotifyRoutineEvent
			copy((*[unsafe.Sizeof(ev)]byte)(unsafe.Pointer(&ev))[:], payload[:szProc])
			toSend = ev
		case FLT_TAG:
			var ev FLT_PREOP_CALLBACK_Event
			copy((*[unsafe.Sizeof(ev)]byte)(unsafe.Pointer(&ev))[:], payload[:szFlt])
			toSend = ev

		case OB_TAG:
			var ev OB_OPERATION_HANDLE_Event
			copy((*[unsafe.Sizeof(ev)]byte)(unsafe.Pointer(&ev))[:], payload[:szOb])
			toSend = ev

		case THREAD_TAG:
			var ev CreateThreadNotifyRoutineEvent
			copy((*[unsafe.Sizeof(ev)]byte)(unsafe.Pointer(&ev))[:], payload[:szThr])
			toSend = ev

		case LOADIMG_TAG:
			var ev LoadImageNotifyRoutineEvent
			copy((*[unsafe.Sizeof(ev)]byte)(unsafe.Pointer(&ev))[:], payload[:szImg])
			toSend = ev

		default:
			fmt.Printf("[WARN] unknown tag=%d msgId=%d\n", tag, hdr.MessageId)
		}
		if toSend != nil {
			DetectRemotThreadChannel <- toSend
			HandleGuardChannel <- toSend
			DLLInjectionChannel <- toSend

		}
	}
}

func main() {
	// use the cli args to set ToProtectPID
	if len(os.Args) > 1 {
		var pid int
		_, err := fmt.Sscanf(os.Args[1], "%d", &pid)
		if err != nil {
			fmt.Printf("Invalid PID argument: %v\n", err)
			return
		}
		ToProtectPID = int32(pid)
	} else {
		fmt.Println("Usage: <program> <PID>")
		return
	}

	r := NewReceiver(`\MedusaComPort`)
	if err := r.Connect(); err != nil {
		fmt.Println(err)
		return
	}
	defer r.Close()
	go RemoteThreadDetectorLoop(DetectRemotThreadChannel)
	go HandleGuardLoop(HandleGuardChannel)
	go detectDLLInjection(DLLInjectionChannel)
	r.Loop()
}
