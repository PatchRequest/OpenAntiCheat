package main

import (
	"fmt"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

func detectDLLInjection(DLLInjectionChannel chan interface{}) {
	for event := range DLLInjectionChannel {
		switch e := event.(type) {
		case LoadImageNotifyRoutineEvent:

			if checkDLLInjection(e) {
				// Additional actions can be taken here if needed
			}

		default:
			// Ignore other events
		}
	}
}

func checkDLLInjection(event LoadImageNotifyRoutineEvent) bool {
	processID := event.ProcessID
	imageFile := windows.UTF16ToString(event.ImageFileW[:])
	base := event.ImageBase

	// get the start and size of the proc behind processID
	if AddrInProc(int(processID), base) {
		return false
	}
	if AddrInProc(int(processID), base) {
		fmt.Printf("[ALERT] DLL Injection detected! ProcessID: %d, DLL: %s\n", processID, imageFile)
		return true
	}
	return true
}

var (
	k32             = syscall.NewLazyDLL("kernel32.dll")
	pOpen           = k32.NewProc("OpenProcess")
	pVirtualQueryEx = k32.NewProc("VirtualQueryEx")
)

const (
	PROCESS_QUERY_INFORMATION = 0x0400
	PROCESS_VM_READ           = 0x0010

	MEM_COMMIT = 0x1000
)

type MEMORY_BASIC_INFORMATION struct {
	BaseAddress       uintptr
	AllocationBase    uintptr
	AllocationProtect uint32
	PartitionId       uint16
	RegionSize        uintptr
	State             uint32
	Protect           uint32
	Type              uint32
}

func AddrInProc(pid int, addr uintptr) bool {
	h, _, _ := pOpen.Call(PROCESS_QUERY_INFORMATION|PROCESS_VM_READ, 0, uintptr(pid))
	if h == 0 {
		return false
	}
	defer pClose.Call(h)

	var mbi MEMORY_BASIC_INFORMATION
	ret, _, _ := pVirtualQueryEx.Call(h, addr, uintptr(unsafe.Pointer(&mbi)), unsafe.Sizeof(mbi))
	if ret == 0 {
		return false
	}
	return mbi.State == MEM_COMMIT
}
