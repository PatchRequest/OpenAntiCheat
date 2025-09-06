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

type Event struct {
	Type         string `json:"type"`
	Timestamp    int64  `json:"ts,omitempty"`
	ProcessID    int32  `json:"pid,omitempty"`
	ThreadID     int32  `json:"tid,omitempty"`
	CallerPID    int32  `json:"caller_pid,omitempty"`
	Operation    int32  `json:"operation,omitempty"`
	IsCreate     *bool  `json:"is_create,omitempty"`
	ImageFile    string `json:"image_file,omitempty"`
	Command      string `json:"command_line,omitempty"`
	FileName     string `json:"file_name,omitempty"`
	ImageBase    uint64 `json:"image_base,omitempty"`
	ImageSize    uint32 `json:"image_size,omitempty"`
	Path         string `json:"path,omitempty"`
	PathAge      int64  `json:"path_age,omitempty"`
	PathHash     string `json:"path_hash,omitempty"`
	Lifetime     int64  `json:"lifetime,omitempty"`
	ToProctedPID int32  `json:"to_protect_pid,omitempty"`
	Reserved     int32  `json:"reserved,omitempty"`
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
	s := NewHydraWSServer(":8080", "/ws", 1024)
	if err := s.Start(); err != nil {
		log.Fatal(err)
	}
	log.Println("listening on :8080 /ws")

	// consumer
	go func() {
		for ev := range s.Recv {
			log.Printf("event type=%s pid=%d op=%d", ev.Type, ev.ProcessID, ev.Operation)
		}
	}()

	// block
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	<-sig
	_ = s.Close()
}
