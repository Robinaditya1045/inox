package live

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sync"
	"sync/atomic"
)

// errCDPClosed is returned for calls on a connection whose browser has gone away.
var errCDPClosed = errors.New("browser connection closed")

// cdpConn speaks the Chrome DevTools Protocol over the pipe Chromium opens with
// --remote-debugging-pipe: NUL-terminated JSON messages, commands on the browser's
// fd 3 and replies and events on its fd 4.
//
// A pipe rather than --remote-debugging-port, because a debugging port is a listening
// socket that grants full control of the browser -- including file:// reads -- to
// anything on the host that can reach it. The backend runs with host networking.
//
// Only the handful of methods the browser resolver needs are ever called, and payloads
// are decoded into small local structs that ignore fields they do not name. Generated
// protocol bindings reject enum values newer than themselves, which turns every
// Chromium upgrade into a chance of events silently failing to decode.
type cdpConn struct {
	w       io.WriteCloser
	writeMu sync.Mutex
	nextID  atomic.Int64

	mu      sync.Mutex
	pending map[int64]chan cdpMessage
	// sinks receive events by session ID; "" is the browser's own session. Called
	// on the read loop, so a sink must return quickly and never wait on a call.
	sinks    map[string]func(method string, params json.RawMessage)
	closed   bool
	closeErr error
	done     chan struct{}
}

type cdpMessage struct {
	ID        int64           `json:"id,omitempty"`
	SessionID string          `json:"sessionId,omitempty"`
	Method    string          `json:"method,omitempty"`
	Params    json.RawMessage `json:"params,omitempty"`
	Result    json.RawMessage `json:"result,omitempty"`
	Error     *cdpError       `json:"error,omitempty"`
}

type cdpError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (e *cdpError) Error() string { return fmt.Sprintf("cdp error %d: %s", e.Code, e.Message) }

func newCDPConn(w io.WriteCloser, r io.Reader) *cdpConn {
	c := &cdpConn{
		w:       w,
		pending: make(map[int64]chan cdpMessage),
		sinks:   make(map[string]func(string, json.RawMessage)),
		done:    make(chan struct{}),
	}
	go c.readLoop(r)
	return c
}

func (c *cdpConn) readLoop(r io.Reader) {
	br := bufio.NewReaderSize(r, 256<<10)
	for {
		raw, err := br.ReadBytes(0)
		if err != nil {
			c.close(fmt.Errorf("%w: %v", errCDPClosed, err))
			return
		}
		var msg cdpMessage
		if err := json.Unmarshal(raw[:len(raw)-1], &msg); err != nil {
			continue
		}

		if msg.ID != 0 {
			c.mu.Lock()
			reply, ok := c.pending[msg.ID]
			delete(c.pending, msg.ID)
			c.mu.Unlock()
			if ok {
				reply <- msg // buffered; never blocks the read loop
			}
			continue
		}
		if msg.Method == "" {
			continue
		}
		c.mu.Lock()
		sink := c.sinks[msg.SessionID]
		c.mu.Unlock()
		if sink != nil {
			sink(msg.Method, msg.Params)
		}
	}
}

// call sends one command and decodes its result into out, which may be nil.
func (c *cdpConn) call(ctx context.Context, sessionID, method string, params, out any) error {
	var rawParams json.RawMessage
	if params != nil {
		b, err := json.Marshal(params)
		if err != nil {
			return fmt.Errorf("encode %s params: %w", method, err)
		}
		rawParams = b
	}

	id := c.nextID.Add(1)
	reply := make(chan cdpMessage, 1)
	c.mu.Lock()
	if c.closed {
		err := c.closeErr
		c.mu.Unlock()
		return err
	}
	c.pending[id] = reply
	c.mu.Unlock()

	frame, err := json.Marshal(cdpMessage{ID: id, SessionID: sessionID, Method: method, Params: rawParams})
	if err != nil {
		c.forget(id)
		return err
	}
	c.writeMu.Lock()
	_, err = c.w.Write(append(frame, 0))
	c.writeMu.Unlock()
	if err != nil {
		c.forget(id)
		return fmt.Errorf("%w: %v", errCDPClosed, err)
	}

	select {
	case msg := <-reply:
		if msg.Error != nil {
			return fmt.Errorf("%s: %w", method, msg.Error)
		}
		if out != nil && len(msg.Result) > 0 {
			if err := json.Unmarshal(msg.Result, out); err != nil {
				return fmt.Errorf("decode %s result: %w", method, err)
			}
		}
		return nil
	case <-ctx.Done():
		c.forget(id)
		return ctx.Err()
	case <-c.done:
		return c.closeErr
	}
}

func (c *cdpConn) forget(id int64) {
	c.mu.Lock()
	delete(c.pending, id)
	c.mu.Unlock()
}

// listen routes a session's events to sink until unlisten is called.
func (c *cdpConn) listen(sessionID string, sink func(method string, params json.RawMessage)) {
	c.mu.Lock()
	c.sinks[sessionID] = sink
	c.mu.Unlock()
}

func (c *cdpConn) unlisten(sessionID string) {
	c.mu.Lock()
	delete(c.sinks, sessionID)
	c.mu.Unlock()
}

// close fails every pending and future call with err. Idempotent.
func (c *cdpConn) close(err error) {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return
	}
	c.closed = true
	c.closeErr = err
	c.pending = nil
	c.sinks = map[string]func(string, json.RawMessage){}
	c.mu.Unlock()

	close(c.done)
	_ = c.w.Close()
}
