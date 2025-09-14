//go:build windows && amd64

package main

import (
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

const defaultPipe = `\\.\pipe\MedusaPipe`

func Start(pipe string) {
	if pipe == "" {
		pipe = defaultPipe
	}
	go server(pipe)
}

func server(pipe string) {
	evtSz := uint32(unsafe.Sizeof(ACEvent{}))

	for {
		h, err := windows.CreateNamedPipe(
			windows.StringToUTF16Ptr(pipe),
			windows.PIPE_ACCESS_INBOUND|windows.FILE_FLAG_OVERLAPPED, // reader
			windows.PIPE_TYPE_MESSAGE|windows.PIPE_READMODE_MESSAGE|windows.PIPE_WAIT,
			16,
			0,        // out buffer (unused)
			evtSz*16, // in buffer: room for multiple events
			0, nil)
		if err != nil || h == windows.InvalidHandle {
			continue
		}

		ov := new(windows.Overlapped)
		err = windows.ConnectNamedPipe(h, ov)
		switch err {
		case windows.ERROR_PIPE_CONNECTED:
			// ok
		case windows.ERROR_IO_PENDING:
			windows.GetOverlappedResult(h, ov, new(uint32), true)
		default:
			windows.CloseHandle(h)
			continue
		}

		go func(ph windows.Handle) {
			readLoop(ph)
			windows.FlushFileBuffers(ph)
			windows.DisconnectNamedPipe(ph)
			windows.CloseHandle(ph)
		}(h)
	}
}

func readLoop(h windows.Handle) {
	evtSz := uint32(unsafe.Sizeof(ACEvent{}))
	buf := make([]byte, evtSz)

	for {
		var n uint32
		err := windows.ReadFile(h, buf, &n, nil)
		if err != nil {
			// client closed or other error
			if errno, ok := err.(syscall.Errno); ok {
				switch errno {
				case windows.ERROR_BROKEN_PIPE, windows.ERROR_PIPE_NOT_CONNECTED:
					return
				case windows.ERROR_MORE_DATA:
					// message > evtSz: drain remaining bytes
					// (sender should write exactly sizeof(ACEvent))
					drain(h)
					continue
				}
			}
			return
		}
		if n == 0 {
			return
		}
		if n != evtSz {
			// unexpected size, skip (or handle framing)
			continue
		}

		// bytes → ACEvent (byte-for-byte copy)
		var ev ACEvent
		copy((*[1 << 30]byte)(unsafe.Pointer(&ev))[:evtSz:evtSz], buf[:evtSz])

		EventChannel <- ev
	}
}

func drain(h windows.Handle) {
	tmp := make([]byte, 16*1024)
	for {
		var n uint32
		err := windows.ReadFile(h, tmp, &n, nil)
		if err == windows.ERROR_MORE_DATA {
			continue
		}
		// stop on success (n==0 means end) or other errors
		return
	}
}
