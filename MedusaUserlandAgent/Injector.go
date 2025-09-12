package main

import (
	"fmt"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

func injectDLLIntoPID(pid int, dllPath string) error {
	hProc, err := windows.OpenProcess(
		windows.PROCESS_CREATE_THREAD|
			windows.PROCESS_QUERY_INFORMATION|
			windows.PROCESS_VM_OPERATION|
			windows.PROCESS_VM_WRITE|
			windows.PROCESS_VM_READ,
		false, uint32(pid))
	if err != nil {
		return err
	}
	defer windows.CloseHandle(hProc)

	pathW, err := windows.UTF16FromString(dllPath)
	if err != nil {
		return err
	}
	nbytes := uintptr(len(pathW) * 2)

	kernel32 := windows.NewLazySystemDLL("kernel32.dll")
	vaex := kernel32.NewProc("VirtualAllocEx")
	r1, _, e := vaex.Call(uintptr(hProc), 0, nbytes,
		windows.MEM_RESERVE|windows.MEM_COMMIT, windows.PAGE_READWRITE)
	if r1 == 0 {
		return e
	}
	remoteBuf := uintptr(r1)

	if err := windows.WriteProcessMemory(hProc, remoteBuf,
		(*byte)(unsafe.Pointer(&pathW[0])), nbytes, nil); err != nil {
		return err
	}

	loadlibW, err := syscall.GetProcAddress(syscall.Handle(kernel32.Handle()), "LoadLibraryW")
	if err != nil {
		return err
	}

	crt := kernel32.NewProc("CreateRemoteThread")
	th, _, e := crt.Call(uintptr(hProc), 0, 0, uintptr(loadlibW), remoteBuf, 0, 0)
	if th == 0 {
		return e
	}
	hThread := windows.Handle(th)
	defer windows.CloseHandle(hThread)

	windows.WaitForSingleObject(hThread, windows.INFINITE)

	// optional: get HMODULE from thread exit code to verify success
	var exitcode uint32
	getExit := kernel32.NewProc("GetExitCodeThread")
	ok, _, _ := getExit.Call(uintptr(hThread), uintptr(unsafe.Pointer(&exitcode)))
	if ok == 0 || exitcode == 0 {
		return fmt.Errorf("LoadLibraryW failed, exit=%#x", exitcode)
	}
	return nil
}
