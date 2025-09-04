package main

import (
	"fmt"
	"unsafe"

	"golang.org/x/sys/windows"
)

const MEM_IMAGE = 0x1000000

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

	fmt.Printf("[ALERT] DLL Injection detected! ProcessID: %d, DLL: %s\n", processID, imageFile)

	return true
}

func AddrInProc(pid int, addr uintptr) bool {
	h, err := windows.OpenProcess(
		windows.PROCESS_QUERY_LIMITED_INFORMATION|windows.PROCESS_VM_READ,
		false, uint32(pid),
	)
	if err != nil {
		return false
	}
	defer windows.CloseHandle(h)

	var mbi windows.MemoryBasicInformation
	if err := windows.VirtualQueryEx(
		h, addr, &mbi, uintptr(unsafe.Sizeof(mbi)),
	); err != nil {
		return false
	}

	if mbi.State != windows.MEM_COMMIT {
		return false
	}
	return mbi.Type == MEM_IMAGE
}
