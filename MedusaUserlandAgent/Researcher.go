package main

import (
	"context"
	"fmt"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

func ResearchPID(pid int32) {

	// get a handle to the process
	path, err := getPath(pid)
	if err != nil {
		fmt.Printf("[ERROR] ResearchPID: PID %d, error getting path: %v\n", pid, err)
	}
	fmt.Println("[INFO] ResearchPID: PID", pid, "->", path)

	appID, err := UpsertApp(context.Background(), db, path)
	if err != nil {
		fmt.Printf("[ERROR] ResearchPID: PID %d, error upserting app: %v\n", pid, err)
	}
	_ = appID
	lifetime, err := getProcessLifetime(pid)
	scoreLifetime := int(easeInOutBack(float64(lifetime)))
	if err != nil {
		fmt.Printf("[ERROR] ResearchPID: PID %d, error getting lifetime: %v\n", pid, err)
	} else {
		fmt.Printf("[INFO] ResearchPID: PID %d, lifetime (ns): %d, Score: %d\n", pid, lifetime, scoreLifetime)
	}

	exeAge, err := getExeAge(path)
	if err != nil {
		fmt.Printf("[ERROR] ResearchPID: PID %d, error getting exe age: %v\n", pid, err)
	} else {
		fmt.Println("[INFO] ResearchPID: PID", pid, "exe age:", time.Since(exeAge).String())
	}
}

func getPath(pid int32) (string, error) {
	// get a handle to the process
	h, err := windows.OpenProcess(windows.PROCESS_QUERY_INFORMATION|windows.PROCESS_VM_READ, false, uint32(pid))
	if err != nil {
		return "", err
	}
	defer windows.CloseHandle(h)

	// get executable path
	var buf [windows.MAX_PATH]uint16
	err = windows.GetModuleFileNameEx(h, 0, &buf[0], windows.MAX_PATH)
	if err != nil {
		return "", err
	}
	exePath := windows.UTF16ToString(buf[:])
	if exePath == "" {
		return "", fmt.Errorf("empty path")
	}

	return exePath, nil
}

func getExeAge(path string) (time.Time, error) {
	var data windows.Win32FileAttributeData
	err := windows.GetFileAttributesEx(windows.StringToUTF16Ptr(path), windows.GetFileExInfoStandard, (*byte)(unsafe.Pointer(&data)))
	if err != nil {
		return time.Time{}, err
	}
	return time.Unix(0, data.CreationTime.Nanoseconds()), nil
}

func getProcessLifetime(pid int32) (uint64, error) {

	// get a handle to the process
	h, err := windows.OpenProcess(windows.PROCESS_QUERY_INFORMATION|windows.PROCESS_VM_READ, false, uint32(pid))
	if err != nil {
		return 0, err
	}
	defer windows.CloseHandle(h)
	var creationTime, exitTime, kernelTime, userTime windows.Filetime
	err = windows.GetProcessTimes(h, &creationTime, &exitTime, &kernelTime, &userTime)
	if err != nil {
		return 0, err
	}
	now := time.Now().Unix()
	secondsAgo := now - int64(creationTime.Nanoseconds()/1e9)

	return uint64(secondsAgo), nil
}
