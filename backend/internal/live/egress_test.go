package live

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

// connectThrough opens a CONNECT tunnel via the proxy and returns the status line.
func connectThrough(t *testing.T, proxy *egressProxy, target string) (string, net.Conn, *bufio.Reader) {
	t.Helper()
	conn, err := net.DialTimeout("tcp", proxy.listener.Addr().String(), 5*time.Second)
	if err != nil {
		t.Fatalf("dial proxy: %v", err)
	}
	t.Cleanup(func() { conn.Close() })
	_ = conn.SetDeadline(time.Now().Add(10 * time.Second))
	fmt.Fprintf(conn, "CONNECT %s HTTP/1.1\r\nHost: %s\r\n\r\n", target, target)
	reader := bufio.NewReader(conn)
	status, err := reader.ReadString('\n')
	if err != nil {
		t.Fatalf("read CONNECT reply: %v", err)
	}
	return strings.TrimSpace(status), conn, reader
}

func TestEgressProxyRefusesPrivateAddresses(t *testing.T) {
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		t.Error("a request reached a loopback service through the browser's proxy")
	}))
	defer target.Close()

	// The production guard: no loopback exception.
	proxy, err := newEgressProxy(NewFetcher("", false).DialContext)
	if err != nil {
		t.Fatalf("newEgressProxy: %v", err)
	}
	defer proxy.close()

	// HTTPS and wss arrive as CONNECT.
	for _, dest := range []string{target.Listener.Addr().String(), "169.254.169.254:80", "10.0.0.5:6379"} {
		if status, _, _ := connectThrough(t, proxy, dest); !strings.Contains(status, "502") {
			t.Errorf("CONNECT %s: %q, want 502", dest, status)
		}
	}

	// Plain HTTP arrives in absolute form.
	proxyURL, _ := url.Parse(proxy.URL())
	client := &http.Client{Transport: &http.Transport{Proxy: http.ProxyURL(proxyURL)}, Timeout: 10 * time.Second}
	resp, err := client.Get(target.URL + "/internal")
	if err != nil {
		t.Fatalf("GET through proxy: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadGateway {
		t.Errorf("plain HTTP to loopback: status %d, want 502", resp.StatusCode)
	}
}

func TestEgressProxyCarriesPublicTraffic(t *testing.T) {
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Origins must not learn they are being reached through a proxy.
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			t.Errorf("X-Forwarded-For = %q, want none", xff)
		}
		fmt.Fprint(w, "hello from "+r.URL.Path)
	}))
	defer target.Close()

	// Loopback stands in for the public internet here.
	proxy, err := newEgressProxy(NewFetcher("", false, AllowLoopback()).DialContext)
	if err != nil {
		t.Fatalf("newEgressProxy: %v", err)
	}
	defer proxy.close()

	proxyURL, _ := url.Parse(proxy.URL())
	client := &http.Client{Transport: &http.Transport{Proxy: http.ProxyURL(proxyURL)}, Timeout: 10 * time.Second}
	resp, err := client.Get(target.URL + "/plain")
	if err != nil {
		t.Fatalf("GET through proxy: %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK || string(body) != "hello from /plain" {
		t.Errorf("forwarded GET = %d %q", resp.StatusCode, body)
	}

	status, conn, reader := connectThrough(t, proxy, target.Listener.Addr().String())
	if !strings.Contains(status, "200") {
		t.Fatalf("CONNECT: %q, want 200", status)
	}
	for line, _ := reader.ReadString('\n'); strings.TrimSpace(line) != ""; line, _ = reader.ReadString('\n') {
	}
	fmt.Fprintf(conn, "GET /tunnelled HTTP/1.1\r\nHost: %s\r\nConnection: close\r\n\r\n", target.Listener.Addr())
	tunnelled, err := http.ReadResponse(reader, nil)
	if err != nil {
		t.Fatalf("read tunnelled response: %v", err)
	}
	body, _ = io.ReadAll(tunnelled.Body)
	if string(body) != "hello from /tunnelled" {
		t.Errorf("tunnelled body = %q", body)
	}

	// Anything that is not a proxy request is refused outright.
	direct, err := http.Get(proxy.URL() + "/")
	if err != nil {
		t.Fatalf("direct GET: %v", err)
	}
	direct.Body.Close()
	if direct.StatusCode != http.StatusBadRequest {
		t.Errorf("direct request status = %d, want 400", direct.StatusCode)
	}
}

func TestEgressProxyCloseCutsOpenTunnels(t *testing.T) {
	target, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer target.Close()
	go func() {
		for {
			c, err := target.Accept()
			if err != nil {
				return
			}
			go io.Copy(io.Discard, c) // hold the connection open
		}
	}()

	proxy, err := newEgressProxy(NewFetcher("", false, AllowLoopback()).DialContext)
	if err != nil {
		t.Fatalf("newEgressProxy: %v", err)
	}
	status, conn, _ := connectThrough(t, proxy, target.Addr().String())
	if !strings.Contains(status, "200") {
		t.Fatalf("CONNECT: %q", status)
	}

	proxy.close()
	_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	if _, err := conn.Read(make([]byte, 1)); err == nil || strings.Contains(err.Error(), "timeout") {
		t.Errorf("tunnel read after close = %v, want the connection cut", err)
	}
}
