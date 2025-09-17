package main

import (
	"context"
	"log"
	"net/http"
	"sync"
	"time"
	"unsafe"

	"github.com/gorilla/websocket"
)

type HydraWSServer struct {
	Addr      string
	Path      string
	Recv      chan ACEvent
	SendText  chan string // outbound (text commands)
	upgrader  websocket.Upgrader
	readLimit int64

	ctx    context.Context
	cancel context.CancelFunc
	srv    *http.Server

	mu    sync.RWMutex
	conns map[*websocket.Conn]struct{}
}

func NewHydraWSServer(addr, path string, buf int) *HydraWSServer {
	ctx, cancel := context.WithCancel(context.Background())
	return &HydraWSServer{
		Addr: addr, Path: path,
		Recv:     make(chan ACEvent, buf),
		SendText: make(chan string, buf),
		upgrader: websocket.Upgrader{
			ReadBufferSize: 4096, WriteBufferSize: 1024,
			CheckOrigin: func(*http.Request) bool { return true },
		},
		readLimit: 16 << 20,
		ctx:       ctx, cancel: cancel,
		conns: make(map[*websocket.Conn]struct{}),
	}
}

func (h *HydraWSServer) Start() error {
	mux := http.NewServeMux()
	mux.HandleFunc(h.Path, h.handleWS)
	h.srv = &http.Server{Addr: h.Addr, Handler: mux,
		ReadTimeout: 15 * time.Second, WriteTimeout: 15 * time.Second, IdleTimeout: 60 * time.Second,
	}

	// broadcaster: TEXT commands
	go func() {
		ping := time.NewTicker(30 * time.Second)
		defer ping.Stop()
		for {
			select {
			case cmd := <-h.SendText:
				h.mu.RLock()
				for c := range h.conns {
					_ = c.SetWriteDeadline(time.Now().Add(10 * time.Second))
					if err := c.WriteMessage(websocket.TextMessage, []byte(cmd)); err != nil {
						// drop broken conn
						_ = c.Close()
						h.mu.RUnlock()
						h.mu.Lock()
						delete(h.conns, c)
						h.mu.Unlock()
						h.mu.RLock()
					}
				}
				h.mu.RUnlock()
			case <-ping.C:
				h.mu.RLock()
				for c := range h.conns {
					_ = c.WriteControl(websocket.PingMessage, nil, time.Now().Add(5*time.Second))
				}
				h.mu.RUnlock()
			case <-h.ctx.Done():
				return
			}
		}
	}()

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
	conn.SetReadLimit(h.readLimit)
	_ = conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	conn.SetPongHandler(func(string) error {
		return conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	})

	h.mu.Lock()
	h.conns[conn] = struct{}{}
	h.mu.Unlock()
	defer func() { h.mu.Lock(); delete(h.conns, conn); h.mu.Unlock(); conn.Close() }()

	evtSz := int(unsafe.Sizeof(ACEvent{}))
	for {
		mt, msg, err := conn.ReadMessage()
		if err != nil {
			return
		}
		if mt != websocket.BinaryMessage || len(msg) != evtSz {
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
