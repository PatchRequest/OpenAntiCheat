// wsclient.go
package main

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

type OnMessage func([]byte)

type HydraWS struct {
	URL       string
	Header    http.Header
	OnMessage OnMessage

	extCh    <-chan Event
	sendCh   chan []byte
	dialer   *websocket.Dialer
	pingIntv time.Duration
	backoff  struct {
		initial time.Duration
		max     time.Duration
		jitter  time.Duration
	}

	ctx    context.Context
	cancel context.CancelFunc

	connMu sync.RWMutex
	conn   *websocket.Conn
	wg     sync.WaitGroup
}

func NewHydraWS(url string, eventCh <-chan Event) *HydraWS {
	ctx, cancel := context.WithCancel(context.Background())
	h := &HydraWS{
		URL:       url,
		Header:    make(http.Header),
		OnMessage: func([]byte) {},
		extCh:     eventCh,
		sendCh:    make(chan []byte, 1024),
		dialer:    &websocket.Dialer{HandshakeTimeout: 10 * time.Second, EnableCompression: true},
		pingIntv:  20 * time.Second,
		ctx:       ctx,
		cancel:    cancel,
	}
	h.backoff.initial = 500 * time.Millisecond
	h.backoff.max = 30 * time.Second
	h.backoff.jitter = 250 * time.Millisecond

	h.wg.Add(1)
	go h.eventPump()
	h.wg.Add(1)
	go h.run()
	return h
}

func (h *HydraWS) WithHeader(hdr http.Header) *HydraWS { h.Header = hdr; return h }
func (h *HydraWS) WithPingInterval(d time.Duration) *HydraWS {
	h.pingIntv = d
	return h
}
func (h *HydraWS) WithOnMessage(f OnMessage) *HydraWS { h.OnMessage = f; return h }

func (h *HydraWS) Close() error {
	h.cancel()
	h.connMu.Lock()
	if h.conn != nil {
		_ = h.conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, "bye"))
		_ = h.conn.Close()
		h.conn = nil
	}
	h.connMu.Unlock()
	h.wg.Wait()
	close(h.sendCh)
	return nil
}

// SendJSON convenience for ad-hoc payloads
func (h *HydraWS) SendJSON(v any) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	select {
	case h.sendCh <- b:
		return nil
	case <-h.ctx.Done():
		return errors.New("closed")
	}
}

// ---- internals ----

func (h *HydraWS) eventPump() {
	defer h.wg.Done()
	for {
		select {
		case ev, ok := <-h.extCh:
			if !ok {
				return
			}
			b, err := ev.JSON()
			if err != nil {
				continue
			}
			select {
			case h.sendCh <- b:
			case <-h.ctx.Done():
				return
			}
		case <-h.ctx.Done():
			return
		}
	}
}

func (h *HydraWS) run() {
	defer h.wg.Done()
	attempt := 0
	for {
		if err := h.connect(); err != nil {
			if errors.Is(err, context.Canceled) {
				return
			}
			time.Sleep(h.nextBackoff(attempt))
			attempt++
			continue
		}
		attempt = 0

		readDone := make(chan error, 1)
		writeDone := make(chan error, 1)
		pingDone := make(chan error, 1)

		h.wg.Add(3)
		go func() { defer h.wg.Done(); readDone <- h.readLoop() }()
		go func() { defer h.wg.Done(); writeDone <- h.writeLoop() }()
		go func() { defer h.wg.Done(); pingDone <- h.pingLoop() }()

		var err error
		select {
		case err = <-readDone:
		case err = <-writeDone:
		case err = <-pingDone:
		case <-h.ctx.Done():
			err = context.Canceled
		}

		h.connMu.Lock()
		if h.conn != nil {
			_ = h.conn.Close()
			h.conn = nil
		}
		h.connMu.Unlock()

		if h.ctx.Err() != nil {
			return
		}
		_ = err // ignore, reconnect loop continues
	}
}

func (h *HydraWS) connect() error {
	if h.ctx.Err() != nil {
		return context.Canceled
	}
	conn, _, err := h.dialer.DialContext(h.ctx, h.URL, h.Header)
	if err != nil {
		return err
	}
	conn.SetReadLimit(16 << 20)
	conn.SetPongHandler(func(string) error {
		_ = conn.SetReadDeadline(time.Now().Add(2 * h.pingIntv))
		return nil
	})
	h.connMu.Lock()
	h.conn = conn
	h.connMu.Unlock()
	return nil
}

func (h *HydraWS) readLoop() error {
	h.connMu.RLock()
	conn := h.conn
	h.connMu.RUnlock()
	if conn == nil {
		return errors.New("no conn")
	}
	_ = conn.SetReadDeadline(time.Now().Add(2 * h.pingIntv))
	for {
		mt, msg, err := conn.ReadMessage()
		if err != nil {
			return err
		}
		if mt == websocket.TextMessage || mt == websocket.BinaryMessage {
			h.OnMessage(msg)
		}
	}
}

func (h *HydraWS) writeLoop() error {
	h.connMu.RLock()
	conn := h.conn
	h.connMu.RUnlock()
	if conn == nil {
		return errors.New("no conn")
	}
	for {
		select {
		case b := <-h.sendCh:
			if b == nil {
				return nil
			}
			_ = conn.SetWriteDeadline(time.Now().Add(15 * time.Second))
			if err := conn.WriteMessage(websocket.TextMessage, b); err != nil {
				return err
			}
		case <-h.ctx.Done():
			return h.ctx.Err()
		}
	}
}

func (h *HydraWS) pingLoop() error {
	t := time.NewTicker(h.pingIntv)
	defer t.Stop()

	h.connMu.RLock()
	conn := h.conn
	h.connMu.RUnlock()
	if conn == nil {
		return errors.New("no conn")
	}
	for {
		select {
		case <-t.C:
			_ = conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
			if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return err
			}
		case <-h.ctx.Done():
			return h.ctx.Err()
		}
	}
}

func (h *HydraWS) nextBackoff(n int) time.Duration {
	base := float64(h.backoff.initial)
	max := float64(h.backoff.max)
	if base == 0 {
		base = float64(500 * time.Millisecond)
	}
	if max == 0 {
		max = float64(30 * time.Second)
	}
	d := time.Duration(math.Min(base*math.Pow(2, float64(n)), max))
	if j := h.backoff.jitter; j > 0 {
		d += time.Duration(time.Now().UnixNano() % int64(j))
	}
	return d
}
