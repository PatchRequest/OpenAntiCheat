package main

import (
	"fmt"
	"unsafe"

	"golang.org/x/sys/windows"
)

type ModuleInfo struct {
	Name string
	Path string
	Base uintptr
	Size uint32
}

const (
	TH32CS_SNAPMODULE   = 0x00000008
	TH32CS_SNAPMODULE32 = 0x00000010

	SE_DEBUG_NAME = "SeDebugPrivilege"
)

var (
	modKernel32                  = windows.NewLazySystemDLL("kernel32.dll")
	procCreateToolhelp32Snapshot = modKernel32.NewProc("CreateToolhelp32Snapshot")
	procModule32FirstW           = modKernel32.NewProc("Module32FirstW")
	procModule32NextW            = modKernel32.NewProc("Module32NextW")
)

type moduleEntry32W struct {
	Size         uint32
	ModuleID     uint32
	ProcessID    uint32
	GlblcntUsage uint32
	ProccntUsage uint32
	ModBaseAddr  uintptr // NOTE: winapi is LPBYTE; store as uintptr here
	ModBaseSize  uint32
	hModule      windows.Handle
	SzModule     [256]uint16
	SzExePath    [260]uint16
}

// Enable SeDebugPrivilege (best-effort; ignore error if not available)
func enableSeDebugPrivilege() error {
	var hTok windows.Token
	p := windows.CurrentProcess()
	if err := windows.OpenProcessToken(p, windows.TOKEN_ADJUST_PRIVILEGES|windows.TOKEN_QUERY, &hTok); err != nil {
		return err
	}
	defer hTok.Close()

	var luid windows.LUID
	if err := windows.LookupPrivilegeValue(nil, windows.StringToUTF16Ptr(SE_DEBUG_NAME), &luid); err != nil {
		return err
	}

	tp := windows.Tokenprivileges{
		PrivilegeCount: 1,
		Privileges: [1]windows.LUIDAndAttributes{{
			Luid:       luid,
			Attributes: windows.SE_PRIVILEGE_ENABLED,
		}},
	}
	return windows.AdjustTokenPrivileges(hTok, false, &tp, 0, nil, nil)
}

// GetProcessModules enumerates all loaded modules (DLLs + main image) of a PID.
func GetProcessModules(pid uint32) ([]ModuleInfo, error) {
	// Best-effort SeDebug
	_ = enableSeDebugPrivilege()

	// Snapshot
	snap, _, err := procCreateToolhelp32Snapshot.Call(
		uintptr(TH32CS_SNAPMODULE|TH32CS_SNAPMODULE32),
		uintptr(pid),
	)
	if snap == uintptr(windows.InvalidHandle) {
		if err != nil {
			return nil, fmt.Errorf("CreateToolhelp32Snapshot: %w", err)
		}
		return nil, fmt.Errorf("CreateToolhelp32Snapshot failed")
	}
	defer windows.CloseHandle(windows.Handle(snap))

	var me moduleEntry32W
	me.Size = uint32(unsafe.Sizeof(me))
	ok, _, e1 := procModule32FirstW.Call(snap, uintptr(unsafe.Pointer(&me)))
	if ok == 0 {
		if e1 != nil {
			return nil, fmt.Errorf("Module32FirstW: %w", e1)
		}
		return nil, fmt.Errorf("Module32FirstW failed")
	}

	out := make([]ModuleInfo, 0, 64)
	for {
		name := windows.UTF16ToString(me.SzModule[:])
		path := windows.UTF16ToString(me.SzExePath[:])

		out = append(out, ModuleInfo{
			Name: name,
			Path: path,
			Base: me.ModBaseAddr,
			Size: me.ModBaseSize,
		})

		me.Size = uint32(unsafe.Sizeof(me)) // must reset before next call
		ok, _, _ = procModule32NextW.Call(snap, uintptr(unsafe.Pointer(&me)))
		if ok == 0 {
			break
		}
	}
	return out, nil
}
