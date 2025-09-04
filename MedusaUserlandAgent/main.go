//go:build windows

package main

import (
	"encoding/binary"
	"fmt"
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
	modFltlib  = windows.NewLazySystemDLL("fltlib.dll")
	modK32     = windows.NewLazySystemDLL("kernel32.dll")
	pConnect   = modFltlib.NewProc("FilterConnectCommunicationPort")
	pGetMsg    = modFltlib.NewProc("FilterGetMessage")
	pClose     = modK32.NewProc("CloseHandle")
	pFmtMsgW   = modK32.NewProc("FormatMessageW")
)

const (
	FMT_FROM_SYSTEM    = 0x00001000
	FMT_IGNORE_INSERTS = 0x00000200
)

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

		switch tag {
		case PROC_TAG:
			var ev CreateProcessNotifyRoutineEvent
			copy((*[unsafe.Sizeof(ev)]byte)(unsafe.Pointer(&ev))[:], payload[:szProc])
			kind := "EXIT"
			if ev.IsCreate != 0 {
				kind = "CREATE"
			}
			img := u16zToString(ev.ImageFileW[:])
			cmd := u16zToString(ev.CommandLineW[:])
			fmt.Printf("[PROC] %s pid=%d image=%s cmd=%s msgId=%d\n", kind, ev.ProcessID, img, cmd, hdr.MessageId)

		case FLT_TAG:
			var ev FLT_PREOP_CALLBACK_Event
			copy((*[unsafe.Sizeof(ev)]byte)(unsafe.Pointer(&ev))[:], payload[:szFlt])
			op := irpMajor[ev.Operation]
			if op == "" {
				op = fmt.Sprintf("0x%02X", ev.Operation)
			}
			fn := u16zToString(ev.FileNameW[:])
			fmt.Printf("[FILE] %s pid=%d file=%s msgId=%d\n", op, ev.ProcessID, fn, hdr.MessageId)

		case OB_TAG:
			var ev OB_OPERATION_HANDLE_Event
			copy((*[unsafe.Sizeof(ev)]byte)(unsafe.Pointer(&ev))[:], payload[:szOb])
			op := obOp[ev.Operation]
			if op == "" {
				op = fmt.Sprintf("OP_%d", ev.Operation)
			}
			fmt.Printf("[OB] %s pid=%d msgId=%d\n", op, ev.ProcessID, hdr.MessageId)

		case THREAD_TAG:
			var ev CreateThreadNotifyRoutineEvent
			copy((*[unsafe.Sizeof(ev)]byte)(unsafe.Pointer(&ev))[:], payload[:szThr])
			kind := "EXIT"
			if ev.IsCreate != 0 {
				kind = "CREATE"
			}
			fmt.Printf("[THREAD] %s pid=%d tid=%d msgId=%d\n", kind, ev.ProcessID, ev.ThreadID, hdr.MessageId)

		case LOADIMG_TAG:
			var ev LoadImageNotifyRoutineEvent
			copy((*[unsafe.Sizeof(ev)]byte)(unsafe.Pointer(&ev))[:], payload[:szImg])
			img := u16zToString(ev.ImageFileW[:])
			fmt.Printf("[LOADIMG] pid=%d image=%s base=0x%X size=%d msgId=%d\n",
				ev.ProcessID, img, ev.ImageBase, ev.ImageSize, hdr.MessageId)

		default:
			fmt.Printf("[WARN] unknown tag=%d msgId=%d\n", tag, hdr.MessageId)
		}
	}
}

func main() {
	r := NewReceiver(`\MedusaComPort`)
	if err := r.Connect(); err != nil {
		fmt.Println(err)
		return
	}
	defer r.Close()
	r.Loop()
}
