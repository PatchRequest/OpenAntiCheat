//go:build windows

package main

import (
	"encoding/binary"
	"fmt"
	"os"
	"unsafe"

	"golang.org/x/sys/windows"
	_ "modernc.org/sqlite"
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

var ToProtectPID int32 = 0
var EventChannel = make(chan Event, 100)
var toInjectDLL string = ""

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
		var toSendEvent Event
		switch tag {
		case PROC_TAG:
			var ev CreateProcessNotifyRoutineEvent
			copy((*[unsafe.Sizeof(ev)]byte)(unsafe.Pointer(&ev))[:], payload[:szProc])
			toSendEvent = FromCreateProcess(ev)
			if ev.IsCreate != 0 {
				injectDLLIntoPID(int(ev.ProcessID), toInjectDLL)
			}
		case FLT_TAG:
			var ev FLT_PREOP_CALLBACK_Event
			copy((*[unsafe.Sizeof(ev)]byte)(unsafe.Pointer(&ev))[:], payload[:szFlt])
			toSendEvent = FromFLT(ev)

		case OB_TAG:
			var ev OB_OPERATION_HANDLE_Event
			copy((*[unsafe.Sizeof(ev)]byte)(unsafe.Pointer(&ev))[:], payload[:szOb])
			toSendEvent = FromOB(ev)

		case THREAD_TAG:
			var ev CreateThreadNotifyRoutineEvent
			copy((*[unsafe.Sizeof(ev)]byte)(unsafe.Pointer(&ev))[:], payload[:szThr])
			toSendEvent = FromThread(ev)

		case LOADIMG_TAG:
			var ev LoadImageNotifyRoutineEvent
			copy((*[unsafe.Sizeof(ev)]byte)(unsafe.Pointer(&ev))[:], payload[:szImg])
			toSendEvent = FromLoadImage(ev)

		default:
			fmt.Printf("[WARN] unknown tag=%d msgId=%d\n", tag, hdr.MessageId)
		}
		enrich(&toSendEvent)
		EventChannel <- toSendEvent
		jsonData, err := toSendEvent.JSON()
		if err != nil {
			fmt.Printf("[ERR] JSON marshal: %v\n", err)
			continue
		}
		fmt.Println(string(jsonData))
	}
}

func main() {
	Start("")
	var pid int

	_, err := fmt.Sscanf(os.Args[1], "%d", &pid)
	if err != nil {
		fmt.Printf("Invalid PID argument: %v\n", err)
		return
	}

	ToProtectPID = int32(pid)
	toInjectDLL = os.Args[3]
	fmt.Println(toInjectDLL)

	r := NewReceiver(`\MedusaComPort`)
	if err := r.Connect(); err != nil {
		fmt.Println(err)
		return
	}
	defer r.Close()
	client := NewHydraWS(os.Args[2], EventChannel).
		WithOnMessage(func(b []byte) {
			fmt.Println("recv:", string(b))
		})
	defer client.Close()
	fmt.Println(client)
	r.Loop()
}
