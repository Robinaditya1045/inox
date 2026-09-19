package live

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"strings"
	"syscall"
	"time"
)

var (
	// ErrHostNotAllowed means the operator has not listed this host as a source they
	// are entitled to pull from.
	ErrHostNotAllowed = errors.New("source host is not in LIVE_SOURCE_ALLOWED_HOSTS")
	// ErrBlockedAddress means a hostname resolved to an address inside our own
	// network. Returned at dial time, so it also catches DNS rebinding.
	ErrBlockedAddress = errors.New("refusing to connect to a private or link-local address")
)

// MaxManifestBytes caps a manifest read. Real playlists are kilobytes; anything
// larger is either not a playlist or is trying to exhaust our memory.
const MaxManifestBytes = 8 << 20

// Fetcher performs outbound HTTP to operator-supplied hosts.
//
// Every request originates from a URL an operator typed into the admin portal and is
// executed from inside our network, which makes this an SSRF primitive by
// construction. Admin-gating is not sufficient mitigation on its own: admin here
// means an address listed in ADMIN_EMAILS, and the blast radius of a mistake is the
// cloud metadata endpoint and every internal service we can reach. Hence two
// independent controls -- a host allowlist checked before dialling, and an address
// check performed at dial time on every hop.
type Fetcher struct {
	client        *http.Client
	allowedHosts  []string
	allowAny      bool
	allowLoopback bool
}

// Option adjusts Fetcher construction.
type Option func(*Fetcher)

// AllowLoopback permits connections to loopback addresses.
//
// Exists so tests can point a resolver at an httptest server, which always listens
// on 127.0.0.1 and is otherwise refused by the address guard. Never enable it in a
// running deployment: loopback is where the metadata service and every unauthenticated
// admin port live.
func AllowLoopback() Option {
	return func(f *Fetcher) { f.allowLoopback = true }
}

// NewFetcher builds a guarded HTTP client.
//
// An empty allowlist is fatal in production and permissive-but-loud elsewhere: local
// development would otherwise require operators to enumerate hosts before they can
// try anything, which reliably teaches people to set the list to a wildcard. Private
// address ranges stay blocked in both cases -- that guard is never relaxed.
func NewFetcher(allowedHosts string, isProd bool, opts ...Option) *Fetcher {
	hosts := splitHosts(allowedHosts)
	allowAny := len(hosts) == 0 && !isProd
	if len(hosts) == 0 {
		if isProd {
			slog.Warn("LIVE_SOURCE_ALLOWED_HOSTS is empty; every live channel resolve and proxy fetch will be refused")
		} else {
			slog.Warn("LIVE_SOURCE_ALLOWED_HOSTS is empty; allowing any public host because this is not a production environment. Set it before deploying.")
		}
	}

	f := &Fetcher{allowedHosts: hosts, allowAny: allowAny}
	for _, opt := range opts {
		opt(f)
	}

	dialer := &net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second}
	// Control runs after DNS resolution with the concrete address about to be
	// dialled, which is the only place a rebinding attack can still be caught: a
	// hostname that passed a pre-flight lookup can resolve to 169.254.169.254 by the
	// time we actually connect.
	dialer.Control = func(network, address string, _ syscall.RawConn) error {
		host, _, err := net.SplitHostPort(address)
		if err != nil {
			return ErrBlockedAddress
		}
		ip := net.ParseIP(host)
		if ip == nil {
			return ErrBlockedAddress
		}
		if f.allowLoopback && ip.IsLoopback() {
			return nil
		}
		if isBlockedIP(ip) {
			return fmt.Errorf("%w: %s", ErrBlockedAddress, host)
		}
		return nil
	}

	f.client = &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			DialContext:           dialer.DialContext,
			TLSHandshakeTimeout:   10 * time.Second,
			ResponseHeaderTimeout: 15 * time.Second,
			MaxIdleConnsPerHost:   8,
			IdleConnTimeout:       90 * time.Second,
		},
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return errors.New("too many redirects")
			}
			// Re-check the allowlist on every hop. Checking only the URL the
			// operator supplied would let any allowed host bounce us anywhere.
			return f.checkHost(req.URL)
		},
	}
	return f
}

// Get issues a guarded GET with caller-supplied upstream headers attached.
func (f *Fetcher) Get(ctx context.Context, rawURL string, headers map[string]string) (*http.Response, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("malformed upstream url: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, fmt.Errorf("unsupported upstream scheme %q", u.Scheme)
	}
	if err := f.checkHost(u); err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	// A default User-Agent because a fair number of origins reject requests without
	// one outright; any operator-configured value overwrites it below.
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; InoxLive/1.0)")
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := f.client.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 {
		resp.Body.Close()
		return nil, fmt.Errorf("upstream returned %d", resp.StatusCode)
	}
	return resp, nil
}

// GetBytes fetches a resource and reads at most limit bytes of it.
func (f *Fetcher) GetBytes(ctx context.Context, rawURL string, headers map[string]string, limit int64) ([]byte, error) {
	resp, err := f.Get(ctx, rawURL, headers)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return io.ReadAll(io.LimitReader(resp.Body, limit))
}

// checkHost enforces the operator allowlist. A listed host also covers its
// subdomains, since CDNs routinely shard segments across per-edge hostnames.
func (f *Fetcher) checkHost(u *url.URL) error {
	if f.allowAny {
		return nil
	}
	host := strings.ToLower(u.Hostname())
	for _, allowed := range f.allowedHosts {
		if host == allowed || strings.HasSuffix(host, "."+allowed) {
			return nil
		}
	}
	return fmt.Errorf("%w: %s", ErrHostNotAllowed, host)
}

// AllowsHost reports whether a raw URL would pass the allowlist, for validating
// operator input before persisting a channel.
func (f *Fetcher) AllowsHost(rawURL string) error {
	u, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("malformed url: %w", err)
	}
	return f.checkHost(u)
}

func splitHosts(csv string) []string {
	var out []string
	for _, h := range strings.Split(csv, ",") {
		if h = strings.ToLower(strings.TrimSpace(h)); h != "" {
			out = append(out, h)
		}
	}
	return out
}

// isBlockedIP rejects everything that is not a routable public address.
func isBlockedIP(ip net.IP) bool {
	if ip.IsLoopback() || ip.IsPrivate() || ip.IsUnspecified() ||
		ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() ||
		ip.IsInterfaceLocalMulticast() || ip.IsMulticast() {
		return true
	}
	// 100.64.0.0/10, carrier-grade NAT, is where cloud providers commonly put
	// internal service endpoints; net.IP.IsPrivate does not cover it.
	if v4 := ip.To4(); v4 != nil {
		return v4[0] == 100 && v4[1] >= 64 && v4[1] <= 127
	}
	return false
}
