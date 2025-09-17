//go:build windows

package main

import (
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
var EventChannel = make(chan ACEvent, 100)
var toInjectDLL string = ""

func (r *Receiver) Loop() {
	hdrSz := uint32(unsafe.Sizeof(filterMessageHeader{}))
	eventSz := uint32(unsafe.Sizeof(ACEvent{}))

	buf := make([]byte, hdrSz+eventSz)

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

		payload := buf[hdrSz : hdrSz+eventSz]
		var toSendEvent ACEvent
		copy((*[unsafe.Sizeof(toSendEvent)]byte)(unsafe.Pointer(&toSendEvent))[:], payload[:eventSz])
		jsonStr, _ := toSendEvent.ToJSON()
		_ = jsonStr
		//fmt.Println(jsonStr)
		/*if ev.IsCreate != 0 {
			injectDLLIntoPID(int(ev.ProcessID), toInjectDLL)
		}*/

		//enrich(&toSendEvent)
		EventChannel <- toSendEvent
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
			if string(b) == "scanDLL" {
				executeDLLScan(ToProtectPID)
				fmt.Println(string(b))
			}

		})
	defer client.Close()

	r.Loop()
}
