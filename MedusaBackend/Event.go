package main

import (
	"encoding/json"
	"syscall"
)

// stable type tags
// EventSource mirrors the C enum
type EventSource int32

const (
	KM  EventSource = 0
	UM  EventSource = 1
	DLL EventSource = 2
)

// ACEvent mirrors the C struct
type ACEvent struct {
	Src           EventSource
	EventType     [260]uint16 // wchar_t[260] → UTF-16 code units
	CallerPID     int32
	TargetPID     int32
	ThreadID      int32
	ImageFileName [260]int32   // matches int[260] in your struct
	CommandLine   [1024]uint16 // wchar_t[1024]
	IsCreate      int32
	ImageBase     uintptr // PVOID
	ImageSize     uint32  // ULONG
}

// helper: convert UTF-16 buffer to Go string
func utf16ToString(buf []uint16) string {
	n := 0
	for n < len(buf) && buf[n] != 0 {
		n++
	}
	return syscall.UTF16ToString(buf[:n])
}

// helper: convert int32 buffer (assuming wide chars stored as int32)
func int32ToString(buf []int32) string {
	u16 := make([]uint16, 0, len(buf))
	for _, v := range buf {
		if v == 0 {
			break
		}
		u16 = append(u16, uint16(v))
	}
	return syscall.UTF16ToString(u16)
}

// ToJSON marshals ACEvent into a JSON string
func (e *ACEvent) ToJSON() (string, error) {
	out := map[string]interface{}{
		"Src":           e.Src,
		"EventType":     utf16ToString(e.EventType[:]),
		"CallerPID":     e.CallerPID,
		"TargetPID":     e.TargetPID,
		"ThreadID":      e.ThreadID,
		"ImageFileName": int32ToString(e.ImageFileName[:]),
		"CommandLine":   utf16ToString(e.CommandLine[:]),
		"IsCreate":      e.IsCreate,
		"ImageBase":     uintptr(e.ImageBase),
		"ImageSize":     e.ImageSize,
	}

	data, err := json.Marshal(out)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

/*
func enrich(ev interface{}) {
	if ev.CallerPID == 0 {
		return
	}
	if p, err := getPath(ev.CallerPID); err == nil {
		ev.Path = p
		if age, err := getExeAge(p); err == nil {
			ev.PathAge = int64(age.Second())
		}
		if h, err := getHash(p); err == nil {
			ev.PathHash = h
		}
	}
	if life, err := getProcessLifetime(ev.CallerPID); err == nil {
		ev.Lifetime = int64(life)
	}
	ev.ToProctedPID = ToProtectPID
	ev.Timestamp = time.Now().UnixNano()
}

// encode
func (ev Event) JSON() ([]byte, error) { return json.Marshal(ev) }
*/
