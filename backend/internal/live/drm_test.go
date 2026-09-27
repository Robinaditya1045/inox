package live

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/inox/inox/backend/internal/domain"
)

// Asking which DRM the browser supports is what players probing their capabilities
// and fingerprinting scripts do on pages whose stream is unprotected. It must never
// by itself make a stream count as DRM-protected.
func TestDRMEvidenceTellsProbingFromProtection(t *testing.T) {
	var probing drmEvidence
	for _, ev := range []probeEvent{
		{Type: "access", KeySystem: "com.widevine.alpha"},
		{Type: "access", KeySystem: "com.microsoft.playready"},
		{Type: "keys", KeySystem: "org.w3.clearkey"},
		{Type: "certificate"},
	} {
		probing.apply(ev)
	}
	if probing.confirmed() {
		t.Errorf("probing alone counted as DRM: %v", probing.signals)
	}
	if got := strings.Join(probing.requestedSystems(), ","); got != "Widevine,PlayReady" {
		t.Errorf("requested systems = %q", got)
	}

	for _, ev := range []probeEvent{
		{Type: "attach"}, {Type: "session"}, {Type: "generate", InitDataType: "cenc"},
		{Type: "update"}, {Type: "encrypted", InitDataType: "cenc"}, {Type: "waiting"},
	} {
		var e drmEvidence
		e.apply(ev)
		if !e.confirmed() {
			t.Errorf("%q did not confirm DRM", ev.Type)
		}
	}
	var licence drmEvidence
	licence.noteLicence("https://lic.drm.example.com/v1/widevine?token=SECRET&session=abc")
	if !licence.confirmed() || strings.Join(licence.licenseHosts, ",") != "lic.drm.example.com" {
		t.Errorf("licence evidence = %+v, want confirmed with the host alone", licence)
	}

	// usingEME gates the licence-URL heuristic: a page that only probed EME support
	// must not have a later POST to a licence-shaped URL taken for a licence request,
	// or a clear stream gets reported as DRM.
	var probeOnly drmEvidence
	probeOnly.apply(probeEvent{Type: "access", KeySystem: "com.widevine.alpha"})
	if probeOnly.usingEME() {
		t.Errorf("a bare EME capability probe counted as using EME: %v", probeOnly.signals)
	}
	for _, ev := range []probeEvent{{Type: "keys"}, {Type: "certificate"}, {Type: "session"}, {Type: "generate"}} {
		var e drmEvidence
		e.apply(ev)
		if !e.usingEME() {
			t.Errorf("%q not counted as using EME", ev.Type)
		}
	}
}

// Probe reports come from inside the page. Only well-formed names survive, and the
// only thing ever taken from init data is the public ID naming a DRM vendor.
func TestDRMEvidenceKeepsOnlyWhatIsSafeToStore(t *testing.T) {
	var e drmEvidence
	e.apply(probeEvent{Type: "access", KeySystem: "com.widevine.alpha"})
	e.apply(probeEvent{Type: "access", KeySystem: "<img src=x onerror=alert(1)>"})
	e.apply(probeEvent{Type: "access", KeySystem: strings.Repeat("a", 65)})
	e.apply(probeEvent{Type: "keys", KeySystem: "com.widevine.alpha"})
	e.apply(probeEvent{Type: "generate", InitDataType: "cenc", SystemIDs: []string{
		"edef8ba979d64acea3c827dcd51d21ed", // Widevine
		"deadbeefdeadbeefdeadbeefdeadbeef", // an unknown vendor, kept by ID
		"not-hex", "abcd",                  // junk
	}})
	e.apply(probeEvent{Type: "encrypted", InitDataType: "cenc; drop table"})
	e.apply(probeEvent{Type: "made-up"})

	if got := strings.Join(e.keySystems, ","); got != "com.widevine.alpha" {
		t.Errorf("key systems = %q, want only the well-formed one", got)
	}
	if got := strings.Join(e.active, ","); got != "com.widevine.alpha" {
		t.Errorf("active key systems = %q", got)
	}
	if got := strings.Join(e.systems, ","); got != "Widevine,DRM system deadbeef-dead-beef-dead-beefdeadbeef" {
		t.Errorf("systems = %q", got)
	}
	if got := strings.Join(e.initData, ","); got != "cenc" {
		t.Errorf("init data types = %q, want only known ones", got)
	}

	// A page calling EME in a loop cannot grow the evidence without bound.
	var flood drmEvidence
	for i := 0; i < 500; i++ {
		flood.apply(probeEvent{Type: "access", KeySystem: "com.vendor" + strings.Repeat("x", i%40) + string(rune('a'+i%26))})
	}
	if len(flood.keySystems) > maxEvidence {
		t.Errorf("key systems grew to %d entries", len(flood.keySystems))
	}
}

func TestLicenseURLRecognisesLicenceServers(t *testing.T) {
	for _, u := range []string{
		"https://lic.widevine.example.com/proxy",
		"https://drm.example.com/v1",
		"https://api.example.com/drm/license?token=x",
		"https://lic.drmtoday.com/license-proxy-widevine/cenc/",
		"https://example.ezdrm.com/playready",
		"https://keyos-server.example.net/api/v4/getLicense",
	} {
		if !licenseURL(u) {
			t.Errorf("licenseURL(%q) = false", u)
		}
	}
	for _, u := range []string{
		"https://stats.example.com/collect",
		"https://cdn.example.com/live/master.m3u8",
		"https://example.com/api/comments",
	} {
		if licenseURL(u) {
			t.Errorf("licenseURL(%q) = true", u)
		}
	}
}

func TestPSSHSystemIDs(t *testing.T) {
	widevine := append([]byte{0, 0, 0, 32, 'p', 's', 's', 'h', 0, 0, 0, 0},
		0xed, 0xef, 0x8b, 0xa9, 0x79, 0xd6, 0x4a, 0xce, 0xa3, 0xc8, 0x27, 0xdc, 0xd5, 0x1d, 0x21, 0xed, 0, 0, 0, 0)
	playready := append([]byte{0, 0, 0, 32, 'p', 's', 's', 'h', 0, 0, 0, 0},
		0x9a, 0x04, 0xf0, 0x79, 0x98, 0x40, 0x42, 0x86, 0xab, 0x92, 0xe6, 0x5b, 0xe0, 0x88, 0x5f, 0x95, 0, 0, 0, 0)
	ids := psshSystemIDs(append(widevine, playready...))
	if len(ids) != 2 || drmSystemName(ids[0]) != "Widevine" || drmSystemName(ids[1]) != "PlayReady" {
		t.Errorf("ids = %v", ids)
	}
	// A truncated or lying box ends the walk rather than reading past the data.
	if ids := psshSystemIDs(widevine[:20]); len(ids) != 0 {
		t.Errorf("truncated box gave %v", ids)
	}
	lying := append([]byte(nil), widevine...)
	lying[3] = 200
	if ids := psshSystemIDs(lying); len(ids) != 0 {
		t.Errorf("oversized box gave %v", ids)
	}
}

// Everything an operator sees about a failed resolve is also stored and logged, so
// none of it may carry a credential: no URL beyond its host, no header value.
func TestFailuresNeverCarryCredentials(t *testing.T) {
	const secret = "SECRET-TOKEN-123"

	// A transport failure: net/http puts the whole URL in the error.
	f := NewFetcher("", false) // no loopback exception, so this is refused at dial
	_, err := f.Get(context.Background(), "http://127.0.0.1:1/live/"+secret+"/master.m3u8?token="+secret, nil)
	if err == nil {
		t.Fatal("fetch to a blocked address succeeded")
	}
	if strings.Contains(err.Error(), secret) {
		t.Errorf("fetch error leaks the URL: %v", err)
	}
	if !errors.Is(err, ErrBlockedAddress) {
		t.Errorf("redaction broke errors.Is: %v", err)
	}
	if _, err := f.Get(context.Background(), "http://%zz/"+secret, nil); err == nil || strings.Contains(err.Error(), secret) {
		t.Errorf("malformed URL error = %v", err)
	}

	for _, raw := range []string{
		"https://user:" + secret + "@cdn.example.com/a.m3u8",
		"https://cdn.example.com?token=" + secret,
		"https://cdn.example.com/" + secret + "/a.m3u8",
	} {
		if got := hostOf(raw); got != "cdn.example.com" {
			t.Errorf("hostOf(%q) = %q, want the host alone", raw, got)
		}
	}

	diag := failureDiagnostics(domain.ResolverBrowser, errors.New("x"), time.Second)
	encoded, _ := json.Marshal(diag)
	if strings.Contains(string(encoded), secret) {
		t.Errorf("diagnostics carry a secret: %s", encoded)
	}
}

func TestOutcomesDriveNoticesAndBackoff(t *testing.T) {
	drm := resolveFailure(domain.OutcomeDRMProtected, "manifest_protected", "drm", ErrDRMProtected)
	cases := []struct {
		err     error
		outcome domain.ResolveOutcome
		notice  string
		retry   time.Duration
	}{
		{drm, domain.OutcomeDRMProtected, "DRM-protected", drmRetryAfter},
		// Wrapped, as resolve does when a channel has fallbacks.
		{errorsJoin("all sources failed", drm), domain.OutcomeDRMProtected, "DRM-protected", drmRetryAfter},
		{resolveFailure(domain.OutcomeNoStreamFound, "no_manifest_requested", "none", nil), domain.OutcomeNoStreamFound, "isn't showing a stream", resolveRetryAfter},
		{context.DeadlineExceeded, domain.OutcomeTimeout, "taking too long", resolveRetryAfter},
		{&UpstreamStatusError{Code: 403}, domain.OutcomeFailed, "could not be reached", resolveRetryAfter},
		{ErrHostNotAllowed, domain.OutcomeFailed, "could not be reached", resolveRetryAfter},
	}
	for _, tc := range cases {
		if outcome, _ := failureOutcome(tc.err); outcome != tc.outcome {
			t.Errorf("%v: outcome %s, want %s", tc.err, outcome, tc.outcome)
		}
		if notice := viewerNotice(tc.err); !strings.Contains(notice, tc.notice) {
			t.Errorf("%v: notice %q", tc.err, notice)
		}
		if retry := retryAfter(tc.err); retry != tc.retry {
			t.Errorf("%v: retry after %s, want %s", tc.err, retry, tc.retry)
		}
	}
}

func errorsJoin(msg string, err error) error { return &wrapped{msg: msg, err: err} }

type wrapped struct {
	msg string
	err error
}

func (w *wrapped) Error() string { return w.msg + ": " + w.err.Error() }
func (w *wrapped) Unwrap() error { return w.err }
