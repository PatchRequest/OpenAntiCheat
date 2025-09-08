//go:build windows

package main

import (
	"encoding/binary"
	"fmt"
	"os"
	"runtime"
	"sync"
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

	readers  = 4    // try 4..8
	workers  = 8    // try NumCPU()*2
	queueCap = 8192 // backlog
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

// ---- fast intake plumbing ----

type item struct {
	hdr     filterMessageHeader
	payload []byte  // view into buf[hdrSz:]
	buf     *[]byte // return to pool after processing
}

var (
	onceSizes sync.Once
	hdrSz     uint32
	maxSz     uint32
	bufPool   sync.Pool
)

func computeSizes() {
	hdrSz = uint32(unsafe.Sizeof(filterMessageHeader{}))
	szProc := uint32(unsafe.Sizeof(CreateProcessNotifyRoutineEvent{}))
	szThr := uint32(unsafe.Sizeof(CreateThreadNotifyRoutineEvent{}))
	szFlt := uint32(unsafe.Sizeof(FLT_PREOP_CALLBACK_Event{}))
	szOb := uint32(unsafe.Sizeof(OB_OPERATION_HANDLE_Event{}))
	szImg := uint32(unsafe.Sizeof(LoadImageNotifyRoutineEvent{}))
	maxSz = szProc
	for _, s := range []uint32{szThr, szFlt, szOb, szImg} {
		if s > maxSz {
			maxSz = s
		}
	}
	bufPool = sync.Pool{
		New: func() any {
			b := make([]byte, hdrSz+maxSz) // one buffer per in-flight read
			return &b
		},
	}
}

func (r *Receiver) readLoop(jobs chan<- item) {
	runtime.LockOSThread()
	for {
		bptr := bufPool.Get().(*[]byte)
		buf := *bptr

		ret, _, _ := pGetMsg.Call(
			uintptr(r.hPort),
			uintptr(unsafe.Pointer(&buf[0])),
			uintptr(uint32(len(buf))),
			0,
		)
		if ret != S_OK {
			fmt.Printf("[ERR] FilterGetMessage 0x%08X (%s)\n", uint32(ret), hresultText(ret))
			bufPool.Put(bptr)
			continue
		}

		// parse header without alloc
		hdr := *(*filterMessageHeader)(unsafe.Pointer(&buf[0]))
		payload := buf[hdrSz : hdrSz+maxSz] // view only, no copy

		jobs <- item{hdr: hdr, payload: payload, buf: bptr} // ownership of buffer moves to worker
	}
}

func worker(jobs <-chan item) {
	for it := range jobs {
		tag := int32(binary.LittleEndian.Uint32(it.payload[0:4]))
		ProcessEvent(it.payload, tag, it.hdr) // unchanged, handles decode + enrich + send
		// return buffer
		bufPool.Put(it.buf)
	}
}

func (r *Receiver) Loop() {
	onceSizes.Do(computeSizes)

	jobs := make(chan item, queueCap)

	// start workers
	for i := 0; i < workers; i++ {
		go worker(jobs)
	}
	// start multiple blocking readers
	for i := 0; i < readers; i++ {
		go r.readLoop(jobs)
	}

	// block forever
	select {}
}

func main() {

	var pid int
	_, err := fmt.Sscanf(os.Args[1], "%d", &pid)
	if err != nil {
		fmt.Printf("Invalid PID argument: %v\n", err)
		return
	}
	ToProtectPID = int32(pid)

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
