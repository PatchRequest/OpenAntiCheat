package main

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

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

func getHash(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := md5.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}

	return hex.EncodeToString(h.Sum(nil)), nil
}
