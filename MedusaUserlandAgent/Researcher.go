package main

import (
	"context"
	"fmt"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

func ResearchPID(pid int32, event interface{}) {

	// get a handle to the process
	path, err := getPath(pid)
	if err != nil {
		fmt.Printf("[ERROR] ResearchPID: PID %d, cannot resolve image path: %v\n", pid, err)
		return // do not proceed without a valid app row
	}
	fmt.Println("[INFO] ResearchPID: PID", pid, "->", path)

	appID, err := UpsertApp(context.Background(), db, path)
	if err != nil {
		fmt.Printf("[ERROR] ResearchPID: PID %d, error upserting app: %v\n", pid, err)
	}

	// inser an event for this research action
	InsertEvent(context.Background(), db, appID, "research", fmt.Sprintf("%T", event), -5)

	_ = appID
	lifetime, err := getProcessLifetime(pid)
	scoreLifetime := int(easeInOutBack(float64(lifetime)))
	if err != nil {
		fmt.Printf("[ERROR] ResearchPID: PID %d, error getting lifetime: %v\n", pid, err)
	} else {
		fmt.Printf("[INFO] ResearchPID: PID %d, lifetime (ns): %d, Score: %d\n", pid, lifetime, scoreLifetime)
	}
	if lifetime < 10 {
		_, err = InsertEvent(context.Background(), db, appID, "lifetime", fmt.Sprintf("lifetime %d sec", lifetime), -10)
		if err != nil {
			fmt.Printf("[ERROR] ResearchPID: PID %d, error inserting lifetime event: %v\n", pid, err)
			// crash here
			panic(err)
		}
	} else if lifetime < 60 {
		InsertEvent(context.Background(), db, appID, "lifetime", fmt.Sprintf("lifetime %d sec", lifetime), -5)
	} else if lifetime < 300 {
		InsertEvent(context.Background(), db, appID, "lifetime", fmt.Sprintf("lifetime %d sec", lifetime), 1)
	} else {
		InsertEvent(context.Background(), db, appID, "lifetime", fmt.Sprintf("lifetime %d sec", lifetime), 5)
	}
	exeAge, err := getExeAge(path)
	if err != nil {
		fmt.Printf("[ERROR] ResearchPID: PID %d, error getting exe age: %v\n", pid, err)
	} else {
		fmt.Println("[INFO] ResearchPID: PID", pid, "exe age:", time.Since(exeAge).String())
	}
	ageDays := int(time.Since(exeAge).Hours() / 24)
	if ageDays < 1 {
		InsertEvent(context.Background(), db, appID, "exe_age", fmt.Sprintf("exe age %d days", ageDays), -10)
	} else if ageDays < 7 {
		InsertEvent(context.Background(), db, appID, "exe_age", fmt.Sprintf("exe age %d days", ageDays), -5)
	} else if ageDays < 30 {
		InsertEvent(context.Background(), db, appID, "exe_age", fmt.Sprintf("exe age %d days", ageDays), 1)
	} else if ageDays < 180 {
		InsertEvent(context.Background(), db, appID, "exe_age", fmt.Sprintf("exe age %d days", ageDays), 5)
	} else {
		InsertEvent(context.Background(), db, appID, "exe_age", fmt.Sprintf("exe age %d days", ageDays), 10)
	}

	// print the score of the app
	a, err := GetAppByID(context.Background(), db, appID)
	if err != nil {
		fmt.Printf("[ERROR] ResearchPID: PID %d, error getting app by ID: %v\n", pid, err)
	} else {
		fmt.Printf("[INFO] ResearchPID: PID %d, AppID %d, Score: %d\n", pid, appID, a.Score)
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
