package live_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/inox/inox/backend/internal/live"
)

const testSecret = "test-secret-at-least-32-characters-long!!"

func TestSealerRoundTripsAndRejectsTampering(t *testing.T) {
	sealer, err := live.NewSealer(testSecret)
	if err != nil {
		t.Fatalf("NewSealer: %v", err)
	}

	upstream := "https://cdn.example.com/live/seg1.ts?token=abc123&exp=999"
	ref, err := sealer.Seal("channel-a", upstream)
	if err != nil {
		t.Fatalf("Seal: %v", err)
	}

	// The upstream URL is a credential; a sealed ref that merely encodes it would
	// hand the raw feed to anyone who can read a manifest.
	if strings.Contains(ref, "cdn.example.com") || strings.Contains(ref, "token=abc123") {
		t.Fatalf("sealed reference leaks the upstream url: %s", ref)
	}

	got, err := sealer.Open("channel-a", ref)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if got != upstream {
		t.Errorf("round trip = %q, want %q", got, upstream)
	}

	// Channel ID is authenticated data, so a ref minted for one channel must not
	// open against another.
	if _, err := sealer.Open("channel-b", ref); !errors.Is(err, live.ErrBadRef) {
		t.Errorf("cross-channel open error = %v, want ErrBadRef", err)
	}

	if _, err := sealer.Open("channel-a", ref[:len(ref)-4]+"AAAA"); !errors.Is(err, live.ErrBadRef) {
		t.Errorf("tampered ref error = %v, want ErrBadRef", err)
	}
	if _, err := sealer.Open("channel-a", "not-base64-$$$"); !errors.Is(err, live.ErrBadRef) {
		t.Errorf("malformed ref error = %v, want ErrBadRef", err)
	}

	// Another deployment's secret must not open our refs.
	other, _ := live.NewSealer("a-completely-different-secret-value-here")
	if _, err := other.Open("channel-a", ref); !errors.Is(err, live.ErrBadRef) {
		t.Errorf("foreign-key open error = %v, want ErrBadRef", err)
	}
}

func TestPlaybackTokenScopeAndExpiry(t *testing.T) {
	signer := live.NewTokenSigner(testSecret)
	now := time.Now()

	token := signer.Mint("channel-a", "user-1", now)

	userID, err := signer.Verify("channel-a", token, now.Add(time.Minute))
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if userID != "user-1" {
		t.Errorf("user = %q, want user-1", userID)
	}

	// A token that leaks grants this one channel and nothing else.
	if _, err := signer.Verify("channel-b", token, now); !errors.Is(err, live.ErrBadToken) {
		t.Errorf("cross-channel verify error = %v, want ErrBadToken", err)
	}
	if _, err := signer.Verify("channel-a", token, now.Add(live.TokenTTL+time.Second)); !errors.Is(err, live.ErrBadToken) {
		t.Errorf("expired verify error = %v, want ErrBadToken", err)
	}
	if _, err := signer.Verify("channel-a", token+"x", now); !errors.Is(err, live.ErrBadToken) {
		t.Errorf("tampered signature error = %v, want ErrBadToken", err)
	}
	if _, err := signer.Verify("channel-a", "no-dot-separator", now); !errors.Is(err, live.ErrBadToken) {
		t.Errorf("malformed token error = %v, want ErrBadToken", err)
	}
}

func TestFetcherEnforcesHostAllowlist(t *testing.T) {
	f := live.NewFetcher("example.com, stream.test", true)

	if err := f.AllowsHost("https://example.com/live.m3u8"); err != nil {
		t.Errorf("listed host rejected: %v", err)
	}
	// CDNs shard segments across per-edge hostnames, so a listed host covers its
	// subdomains.
	if err := f.AllowsHost("https://edge-42.example.com/seg.ts"); err != nil {
		t.Errorf("subdomain of a listed host rejected: %v", err)
	}
	if err := f.AllowsHost("https://evil.com/live.m3u8"); !errors.Is(err, live.ErrHostNotAllowed) {
		t.Errorf("unlisted host error = %v, want ErrHostNotAllowed", err)
	}
	// Suffix matching must not degrade into "ends with the string".
	if err := f.AllowsHost("https://notexample.com/live.m3u8"); !errors.Is(err, live.ErrHostNotAllowed) {
		t.Errorf("lookalike host was allowed through suffix matching: %v", err)
	}
}

func TestFetcherEmptyAllowlistIsClosedInProduction(t *testing.T) {
	prod := live.NewFetcher("", true)
	if err := prod.AllowsHost("https://anything.example.com/live.m3u8"); !errors.Is(err, live.ErrHostNotAllowed) {
		t.Errorf("production with no allowlist must refuse every host, got %v", err)
	}

	// Outside production an empty list stays open, so local development does not
	// push operators towards configuring a wildcard they then ship.
	dev := live.NewFetcher("", false)
	if err := dev.AllowsHost("https://anything.example.com/live.m3u8"); err != nil {
		t.Errorf("development with no allowlist should permit public hosts, got %v", err)
	}
}

func TestFetcherRefusesPrivateAddresses(t *testing.T) {
	// Allowlisted by name, yet still unreachable: the address check runs at dial
	// time, which is what closes the DNS-rebinding path a name-only check leaves open.
	f := live.NewFetcher("localhost,metadata.google.internal", false)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	for _, target := range []string{
		"http://127.0.0.1:9999/manifest.m3u8",
		"http://localhost:9999/manifest.m3u8",
		"http://169.254.169.254/latest/meta-data/",
		"http://10.0.0.5/internal.m3u8",
	} {
		if _, err := f.GetBytes(ctx, target, nil, live.MaxManifestBytes); err == nil {
			t.Errorf("%s: expected the request to be refused", target)
		}
	}
}
