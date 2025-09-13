package main

import (
	"fmt"
	"sync"
	"syscall"

	"golang.org/x/sys/windows"
)

const defaultPipe = `\\.\pipe\MedusaPipe`

func Start(pipe string) {
	fmt.Println("Started PipeServer")
	if pipe == "" {
		pipe = defaultPipe
	}
	go server(pipe)
}

func server(pipe string) {
	var wg sync.WaitGroup
	for {
		h, err := windows.CreateNamedPipe(
			windows.StringToUTF16Ptr(pipe),
			windows.PIPE_ACCESS_DUPLEX|windows.FILE_FLAG_OVERLAPPED, // non-blocking accept
			windows.PIPE_TYPE_MESSAGE|windows.PIPE_READMODE_MESSAGE|windows.PIPE_WAIT,
			16, 64*1024, 64*1024, 0, nil)
		if err != nil || h == windows.InvalidHandle {
			continue
		}

		// accept one client per instance, then spin a reader goroutine
		ov := new(windows.Overlapped)
		err = windows.ConnectNamedPipe(h, ov)
		if err == windows.ERROR_PIPE_CONNECTED {
			// already connected
		} else if err == windows.ERROR_IO_PENDING {
			// wait until connected
			windows.GetOverlappedResult(h, ov, new(uint32), true)
		} else if err != nil {
			windows.CloseHandle(h)
			continue
		}

		wg.Add(1)
		go func(ph windows.Handle) {
			defer wg.Done()
			readLoop(ph)
			windows.FlushFileBuffers(ph)
			windows.DisconnectNamedPipe(ph)
			windows.CloseHandle(ph)
		}(h)
	}
}

func readLoop(h windows.Handle) {
	buf := make([]byte, 64*1024)
	for {
		var n uint32
		err := windows.ReadFile(h, buf, &n, nil)
		if err == windows.ERROR_MORE_DATA {
			// message larger than buffer: print chunk and continue to drain
			fmt.Println(string(buf[:n]))
			continue
		}
		if err != nil {
			// client closed or error
			if errno, ok := err.(syscall.Errno); ok {
				switch errno {
				case windows.ERROR_BROKEN_PIPE, windows.ERROR_PIPE_NOT_CONNECTED:
					return
				}
			}
			return
		}
		if n == 0 {
			return
		}
		fmt.Println(string(buf[:n]))
	}
}
