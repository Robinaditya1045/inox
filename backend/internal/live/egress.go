package live

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httputil"
	"sync"
	"time"
)

// egressProxy is the forward proxy every connection a headless page makes is routed
// through.
//
// The Fetcher's address guard only covers requests we make. A page's scripts choose
// their own destinations, and the browser runs inside our network -- with host
// networking in the Oracle deployment -- so without this any third-party script on a
// channel page, ads included, could reach Redis, MinIO, or the cloud metadata
// endpoint. Chromium is launched with this as its proxy for every scheme and with its
// implicit loopback bypass removed, so each frame, worker, and redirect dials through
// Fetcher.DialContext, which refuses private addresses at dial time and so also stops
// DNS rebinding.
//
// It deliberately does not apply the source host allowlist: a player page pulls
// scripts from a dozen hosts, and the stream it finds is fetched by the Fetcher,
// which applies the allowlist anyway.
// egressRefusedHeader is set on responses the proxy synthesises because the guard
// refused a destination.
const egressRefusedHeader = "X-Inox-Egress-Refused"

type egressProxy struct {
	dial     func(ctx context.Context, network, address string) (net.Conn, error)
	listener net.Listener
	server   *http.Server
	forward  *httputil.ReverseProxy

	mu      sync.Mutex
	tunnels map[net.Conn]struct{}
	closed  bool
}

func newEgressProxy(dial func(ctx context.Context, network, address string) (net.Conn, error)) (*egressProxy, error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}
	p := &egressProxy{dial: dial, listener: ln, tunnels: make(map[net.Conn]struct{})}
	p.forward = &httputil.ReverseProxy{
		// A proxied request is already in absolute form, so it goes where it says.
		// Rewrite rather than Director because Rewrite strips forwarding headers
		// instead of adding X-Forwarded-For.
		Rewrite: func(*httputil.ProxyRequest) {},
		Transport: &http.Transport{
			DialContext:           dial,
			TLSHandshakeTimeout:   10 * time.Second,
			ResponseHeaderTimeout: 30 * time.Second,
			IdleConnTimeout:       30 * time.Second,
		},
		ErrorHandler: func(w http.ResponseWriter, _ *http.Request, err error) {
			if errors.Is(err, ErrBlockedAddress) {
				// Marks the refusal as ours, so it can be told apart from a
				// 502 the site itself sent.
				w.Header().Set(egressRefusedHeader, err.Error())
			}
			http.Error(w, "egress refused: "+err.Error(), http.StatusBadGateway)
		},
	}
	p.server = &http.Server{Handler: p, ReadHeaderTimeout: 10 * time.Second}
	go func() {
		if err := p.server.Serve(ln); err != nil && err != http.ErrServerClosed {
			slog.Warn("browser egress proxy stopped", "error", err)
		}
	}()
	return p, nil
}

// URL is the proxy address handed to Chromium.
func (p *egressProxy) URL() string { return "http://" + p.listener.Addr().String() }

func (p *egressProxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodConnect {
		p.tunnel(w, r)
		return
	}
	// A proxied plain-HTTP request arrives in absolute form. Anything else is not
	// the browser talking to its proxy.
	if !r.URL.IsAbs() || r.URL.Scheme != "http" {
		http.Error(w, "not a proxy request", http.StatusBadRequest)
		return
	}
	p.forward.ServeHTTP(w, r)
}

// tunnel serves CONNECT, which carries HTTPS and secure WebSockets.
func (p *egressProxy) tunnel(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	upstream, err := p.dial(ctx, "tcp", r.Host)
	cancel()
	if err != nil {
		http.Error(w, "egress refused: "+err.Error(), http.StatusBadGateway)
		return
	}
	hijacker, ok := w.(http.Hijacker)
	if !ok {
		upstream.Close()
		http.Error(w, "tunnelling unsupported", http.StatusInternalServerError)
		return
	}
	client, buffered, err := hijacker.Hijack()
	if err != nil {
		upstream.Close()
		return
	}
	if !p.track(client, upstream) {
		client.Close()
		upstream.Close()
		return
	}
	defer p.untrack(client, upstream)

	if _, err := client.Write([]byte("HTTP/1.1 200 Connection Established\r\n\r\n")); err != nil {
		return
	}
	done := make(chan struct{}, 2)
	go func() {
		// Bytes the browser sent right behind the CONNECT line may already be
		// sitting in the hijacked reader.
		_, _ = io.Copy(upstream, buffered)
		closeWrite(upstream)
		done <- struct{}{}
	}()
	go func() {
		_, _ = io.Copy(client, upstream)
		closeWrite(client)
		done <- struct{}{}
	}()
	<-done
	// A CONNECT tunnel carries one bidirectional TLS/WebSocket stream, so one side
	// reaching EOF means the connection is ending. Close both ends now rather than
	// leave the other copy blocked on a peer that half-closed but kept its socket
	// open (HTTP keep-alive), which would leak the goroutine and both FDs for the
	// life of the browser. untrack (deferred) closes them again harmlessly.
	_ = client.Close()
	_ = upstream.Close()
	<-done
}

// track records a live tunnel so close can cut it; reports false once closed.
func (p *egressProxy) track(conns ...net.Conn) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed {
		return false
	}
	for _, c := range conns {
		p.tunnels[c] = struct{}{}
	}
	return true
}

func (p *egressProxy) untrack(conns ...net.Conn) {
	p.mu.Lock()
	for _, c := range conns {
		delete(p.tunnels, c)
		c.Close()
	}
	p.mu.Unlock()
}

// close stops the listener and cuts every open tunnel, so no page connection
// outlives the browser it belonged to.
func (p *egressProxy) close() {
	p.mu.Lock()
	p.closed = true
	for c := range p.tunnels {
		c.Close()
	}
	p.tunnels = map[net.Conn]struct{}{}
	p.mu.Unlock()
	_ = p.server.Close()
}

func closeWrite(c net.Conn) {
	if tcp, ok := c.(*net.TCPConn); ok {
		_ = tcp.CloseWrite()
		return
	}
	_ = c.Close()
}
