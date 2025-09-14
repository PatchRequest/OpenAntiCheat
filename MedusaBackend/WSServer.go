package main

import (
	"context"
	"log"
	"net/http"
	"time"
	"unsafe"

	"github.com/gorilla/websocket"
)

type HydraWSServer struct {
	Addr      string
	Path      string
	Recv      chan ACEvent
	upgrader  websocket.Upgrader
	readLimit int64

	ctx    context.Context
	cancel context.CancelFunc
	srv    *http.Server
}

func NewHydraWSServer(addr, path string, buf int) *HydraWSServer {
	ctx, cancel := context.WithCancel(context.Background())
	return &HydraWSServer{
		Addr: addr,
		Path: path,
		Recv: make(chan ACEvent, buf),
		upgrader: websocket.Upgrader{
			ReadBufferSize:  4096,
			WriteBufferSize: 1024,
			CheckOrigin:     func(r *http.Request) bool { return true },
		},
		readLimit: 16 << 20,
		ctx:       ctx,
		cancel:    cancel,
	}
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
	go func() {
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

	evtSz := int(unsafe.Sizeof(ACEvent{}))

	for {
		mt, msg, err := conn.ReadMessage()
		if err != nil {
			return
		}
		if mt != websocket.BinaryMessage {
			// ignore non-binary; you can add JSON fallback if needed
			continue
		}
		if len(msg) != evtSz {
			// wrong size frame; drop
			continue
		}

		var ev ACEvent
		copy((*[1 << 30]byte)(unsafe.Pointer(&ev))[:evtSz:evtSz], msg)

		select {
		case h.Recv <- ev:
		case <-h.ctx.Done():
			return
		}
	}
}
