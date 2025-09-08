// hydra_ws_server.go
package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/gorilla/websocket"
)

var RemoteThreadDetectorChannel = make(chan Event, 1024)
var HandleDetectorChannel = make(chan Event, 1024)

type Event struct {
	// Event type identifier (create_process, flt_preop, ob_operation, create_thread, load_image)
	Type string `json:"type"`
	// Unix nanosecond timestamp when event was processed by userland agent
	Timestamp int64 `json:"ts,omitempty"`

	// Process ID of the target process involved in the event
	// For process creation: new process PID
	// For file operations: process performing the operation
	// For object operations: target process PID
	// For thread events: process owning the thread
	// For image loads: process loading the image
	ProcessID int32 `json:"pid,omitempty"`

	// Thread ID for thread-specific events (create_thread)
	ThreadID int32 `json:"tid,omitempty"`

	// Process ID of the process that initiated/caused this event
	// For process creation: parent process PID
	// For file operations: same as ProcessID (process performing operation)
	// For object operations: process attempting to open/duplicate handle
	// For thread events: process creating/terminating the thread
	// For image loads: same as ProcessID (process loading image)
	CallerPID int32 `json:"caller_pid,omitempty"`

	// Operation type code
	// For file operations: IRP major function (0x00=CREATE, 0x03=READ, 0x04=WRITE, etc.)
	// For object operations: handle operation (0=CREATE, 1=DUPLICATE)
	Operation int32 `json:"operation,omitempty"`

	// Whether this is a creation (true) or termination (false) event
	// Used for process and thread lifecycle events
	IsCreate *bool `json:"is_create,omitempty"`

	// Path to executable image file (UTF-16 converted to UTF-8)
	// For process events: executable path of new/terminated process
	// For image load events: path of loaded DLL/module
	ImageFile string `json:"image_file,omitempty"`

	// Full command line arguments (UTF-16 converted to UTF-8)
	// Only populated for process creation events
	Command string `json:"command_line,omitempty"`

	// Target file name for file system operations (UTF-16 converted to UTF-8)
	// Only populated for minifilter callback events (flt_preop)
	FileName string `json:"file_name,omitempty"`

	// Base memory address where image/DLL was loaded
	// Only populated for load_image events
	ImageBase uint64 `json:"image_base,omitempty"`

	// Size in bytes of loaded image/DLL
	// Only populated for load_image events
	ImageSize uint32 `json:"image_size,omitempty"`

	// Full filesystem path of the calling process executable
	// Enriched by userland agent based on CallerPID
	Path string `json:"path,omitempty"`

	// Age of the calling process executable file in seconds
	// Calculated from file creation/modification time
	PathAge int64 `json:"path_age,omitempty"`

	// Hash (SHA-256/SHA-1/MD5) of the calling process executable
	// Computed by userland agent for integrity verification
	PathHash string `json:"path_hash,omitempty"`

	// Runtime lifetime of the calling process in seconds
	// How long the process has been running when event occurred
	Lifetime int64 `json:"lifetime,omitempty"`

	// Process ID that this anti-cheat system is protecting
	// Set by userland agent configuration
	ToProctedPID int32 `json:"to_protect_pid,omitempty"`

	// Event type tag from kernel driver for internal routing
	// 0=PROC_TAG, 1=FLT_TAG, 2=OB_TAG, 3=THREAD_TAG, 4=LOADIMG_TAG
	Reserved int32 `json:"reserved,omitempty"`
}

// HydraWSServer receives JSON Event frames from clients over WS
type HydraWSServer struct {
	Addr      string
	Path      string
	Recv      chan Event // your pipeline
	upgrader  websocket.Upgrader
	readLimit int64

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
	srv    *http.Server
}

func NewHydraWSServer(addr, path string, buf int) *HydraWSServer {
	ctx, cancel := context.WithCancel(context.Background())
	h := &HydraWSServer{
		Addr: addr,
		Path: path,
		Recv: make(chan Event, buf),
		upgrader: websocket.Upgrader{
			ReadBufferSize:  4096,
			WriteBufferSize: 1024,
			CheckOrigin:     func(r *http.Request) bool { return true }, // adjust
		},
		readLimit: 16 << 20, // 16 MiB
		ctx:       ctx,
		cancel:    cancel,
	}
	return h
}

func (h *HydraWSServer) Start() error {
	mux := http.NewServeMux()
	mux.HandleFunc(h.Path, h.handleWS)

	h.srv = &http.Server{
		Addr:         h.Addr,
		Handler:      mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	h.wg.Add(1)
	go func() {
		defer h.wg.Done()
		if err := h.srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("server error: %v", err)
			h.cancel()
		}
	}()
	return nil
}

func (h *HydraWSServer) Close() error {
	h.cancel()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if h.srv != nil {
		_ = h.srv.Shutdown(ctx)
	}
	h.wg.Wait()
	close(h.Recv)
	return nil
}

func (h *HydraWSServer) handleWS(w http.ResponseWriter, r *http.Request) {
	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	conn.SetReadLimit(h.readLimit)
	_ = conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	conn.SetPongHandler(func(string) error {
		return conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	})

	// optional: basic ack to confirm open
	_ = conn.WriteControl(websocket.PongMessage, nil, time.Now().Add(2*time.Second))

	for {
		mt, msg, err := conn.ReadMessage()
		if err != nil {
			return
		}
		if mt != websocket.TextMessage && mt != websocket.BinaryMessage {
			continue
		}

		var ev Event
		if err := json.Unmarshal(msg, &ev); err != nil {
			_ = conn.WriteMessage(websocket.TextMessage, []byte(`{"error":"bad_json"}`))
			continue
		}

		select {
		case h.Recv <- ev:

			// optional: tiny ack
			// _ = conn.WriteMessage(websocket.TextMessage, []byte(`{"ok":true}`))
		case <-h.ctx.Done():
			return
		}
	}
}

// ---------- minimal main ----------

func main() {
	s := NewHydraWSServer("0.0.0.0:8080", "/", 1024)
	if err := s.Start(); err != nil {
		log.Fatal(err)
	}
	log.Println("listening on :8080 /")
	remoteThreadDetector := NewRemoteThreadDetector(RemoteThreadDetectorChannel)
	handleDetector := NewHandleDetector(HandleDetectorChannel)
	// consumer
	go func() {
		for ev := range s.Recv {
			RemoteThreadDetectorChannel <- ev
			HandleDetectorChannel <- ev
			//fmt.Printf("%+v\n", ev)

		}
	}()
	go remoteThreadDetector.Start()
	go handleDetector.Start()

	// block
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	<-sig
	_ = s.Close()
}
