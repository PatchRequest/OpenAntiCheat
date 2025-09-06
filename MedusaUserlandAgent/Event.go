package main

import (
	"encoding/json"
	"time"
	"unicode/utf16"
)

// stable type tags
const (
	EvtCreateProcess = "create_process"
	EvtFltPreOp      = "flt_preop"
	EvtObOperation   = "ob_operation"
	EvtCreateThread  = "create_thread"
	EvtLoadImage     = "load_image"
)

type Event struct {
	Type      string `json:"type"`
	Timestamp int64  `json:"ts,omitempty"`

	// common
	ProcessID int32 `json:"pid,omitempty"`
	ThreadID  int32 `json:"tid,omitempty"`
	CallerPID int32 `json:"caller_pid,omitempty"`
	Operation int32 `json:"operation,omitempty"`
	IsCreate  *bool `json:"is_create,omitempty"`

	// strings normalized from UTF16
	ImageFile string `json:"image_file,omitempty"`
	Command   string `json:"command_line,omitempty"`
	FileName  string `json:"file_name,omitempty"`

	// image specific
	ImageBase uint64 `json:"image_base,omitempty"`
	ImageSize uint32 `json:"image_size,omitempty"`

	// computed fields
	Path         string `json:"path,omitempty"`
	PathAge      int64  `json:"path_age,omitempty"`  // seconds, days, whatever
	PathHash     string `json:"path_hash,omitempty"` // sha256/sha1/md5
	Lifetime     int64  `json:"lifetime,omitempty"`  // process lifetime in seconds
	ToProctedPID int32  `json:"to_protect_pid,omitempty"`

	Reserved int32 `json:"reserved,omitempty"`
}

// helpers
func utf16BufToString(buf []uint16) string {
	// trim at first zero
	n := 0
	for n < len(buf) && buf[n] != 0 {
		n++
	}
	return string(utf16.Decode(buf[:n]))
}

// builders
func FromCreateProcess(e CreateProcessNotifyRoutineEvent) Event {
	ic := e.IsCreate != 0
	return Event{
		Type:      EvtCreateProcess,
		ProcessID: e.ProcessID,
		IsCreate:  &ic,
		ImageFile: utf16BufToString(e.ImageFileW[:]),
		Command:   utf16BufToString(e.CommandLineW[:]),
		Reserved:  e.Reserved,
		CallerPID: e.ProcessID,
	}
}

func FromFLT(e FLT_PREOP_CALLBACK_Event) Event {
	return Event{
		Type:      EvtFltPreOp,
		ProcessID: e.ProcessID,
		Operation: e.Operation,
		FileName:  utf16BufToString(e.FileNameW[:]),
		Reserved:  e.Reserved,
		CallerPID: e.ProcessID,
	}
}

func FromOB(e OB_OPERATION_HANDLE_Event) Event {
	return Event{
		Type:      EvtObOperation,
		ProcessID: e.ProcessID,
		CallerPID: e.CallerPID,
		Operation: e.Operation,
		Reserved:  e.Reserved,
	}
}

func FromThread(e CreateThreadNotifyRoutineEvent) Event {
	ic := e.IsCreate != 0
	return Event{
		Type:      EvtCreateThread,
		ProcessID: e.ProcessID,
		ThreadID:  e.ThreadID,
		CallerPID: e.CallerPID,
		IsCreate:  &ic,
		Reserved:  e.Reserved,
	}
}

func FromLoadImage(e LoadImageNotifyRoutineEvent) Event {
	return Event{
		Type:      EvtLoadImage,
		ProcessID: e.ProcessID,
		ImageFile: utf16BufToString(e.ImageFileW[:]),
		ImageBase: e.ImageBase,
		ImageSize: e.ImageSize,
		Reserved:  e.Reserved,
		CallerPID: e.ProcessID,
	}
}

func enrich(ev *Event) {
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
