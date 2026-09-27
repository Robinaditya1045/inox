package live_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/inox/inox/backend/internal/domain"
	"github.com/inox/inox/backend/internal/live"
)

// secret marks every credential-like value the fixtures hand out: in manifest URLs,
// in licence URLs, in licence requests. None of it may reach diagnostics or errors.
const secret = "SECRET-7f3a"

// eme is the EME configuration the fixture players ask for.
const eme = `[{initDataTypes: ['cenc', 'keyids'], videoCapabilities: [{contentType: 'video/mp4; codecs="avc1.42E01E"'}]}]`

// drmSite serves fixture players, each a page that behaves the way one kind of real
// player does, plus the manifests and licence server they use.
func drmSite(t *testing.T, pages map[string]string) *httptest.Server {
	t.Helper()
	const master = "#EXTM3U\n#EXT-X-STREAM-INF:BANDWIDTH=2000000\nv.m3u8\n"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/live/manifest.mpd":
			w.Header().Set("Content-Type", "application/dash+xml")
			fmt.Fprint(w, `<?xml version="1.0"?><MPD xmlns="urn:mpeg:dash:schema:mpd:2011" type="dynamic"><Period>
  <AdaptationSet contentType="video">
    <ContentProtection schemeIdUri="urn:mpeg:dash:mp4protection:2011" value="cenc"/>
    <ContentProtection schemeIdUri="urn:uuid:edef8ba9-79d6-4ace-a3c8-27dcd51d21ed"/>
    <Representation id="1080p" bandwidth="6000000"/>
  </AdaptationSet></Period></MPD>`)
		case "/clear/master.m3u8", "/ad/master.m3u8":
			w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
			fmt.Fprint(w, master)
		case "/clear/v.m3u8":
			fmt.Fprint(w, "#EXTM3U\n#EXT-X-TARGETDURATION:6\n#EXT-X-MEDIA-SEQUENCE:5\n#EXTINF:6,\nlive.ts\n")
		case "/ad/v.m3u8":
			fmt.Fprint(w, "#EXTM3U\n#EXT-X-TARGETDURATION:5\n#EXTINF:5,\nad.ts\n#EXT-X-ENDLIST\n")
		default:
			if strings.HasPrefix(r.URL.Path, "/drm/license/") && r.Method == http.MethodPost {
				w.Header().Set("Content-Type", "application/json")
				fmt.Fprint(w, `{"keys":[],"type":"temporary"}`)
				return
			}
			page, ok := pages[strings.TrimPrefix(r.URL.Path, "/")]
			if !ok {
				http.NotFound(w, r)
				return
			}
			fmt.Fprintf(w, `<!doctype html><html><body><video id="v"></video><script>
const eme = %s;
%s
</script></body></html>`, eme, page)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func resolvePage(t *testing.T, browser *live.Browser, pageURL string) (*live.Resolution, *domain.ResolveDiagnostics, error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	res, err := live.NewBrowserResolver(browser, testFetcher(), 20*time.Second).
		Resolve(ctx, &domain.LiveChannel{Slug: "drm-test", SourceURL: pageURL, ResolverConfig: map[string]any{}})
	if err != nil {
		var re *live.ResolveError
		if !errors.As(err, &re) || re.Diagnostics == nil {
			t.Fatalf("error %v carries no diagnostics", err)
		}
		assertNoSecrets(t, re.Diagnostics, err.Error())
		return nil, re.Diagnostics, err
	}
	if res.Diagnostics == nil {
		t.Fatal("a successful resolve carried no diagnostics")
	}
	assertNoSecrets(t, res.Diagnostics, "")
	return res, res.Diagnostics, nil
}

func assertNoSecrets(t *testing.T, diag *domain.ResolveDiagnostics, message string) {
	t.Helper()
	encoded, _ := json.Marshal(diag)
	if strings.Contains(string(encoded), secret) || strings.Contains(message, secret) {
		t.Errorf("a credential reached the diagnostics or error:\n%s\n%s", encoded, message)
	}
}

func TestBrowserResolverReportsProtectedDASHAsDRM(t *testing.T) {
	t.Parallel() // each test drives its own Chromium
	site := drmSite(t, map[string]string{
		"watch": `fetch('/live/manifest.mpd?token=` + secret + `');`,
	})

	_, diag, err := resolvePage(t, browserForTest(t), site.URL+"/watch")
	if !errors.Is(err, live.ErrDRMProtected) {
		t.Fatalf("error = %v, want ErrDRMProtected", err)
	}
	if diag.Outcome != domain.OutcomeDRMProtected || diag.Reason != "manifest_protected" {
		t.Errorf("outcome = %s/%s, want DRM_PROTECTED/manifest_protected", diag.Outcome, diag.Reason)
	}
	if diag.DRM == nil || !diag.DRM.Confirmed || strings.Join(diag.DRM.Systems, ",") != "Widevine" ||
		strings.Join(diag.DRM.Schemes, ",") != "cenc" {
		t.Errorf("drm diagnostics = %+v", diag.DRM)
	}
	if len(diag.Manifests) != 1 || diag.Manifests[0].Verdict != "drm_protected" || diag.Manifests[0].Kind != "mpd" {
		t.Errorf("manifests = %+v, want the MPD marked drm_protected", diag.Manifests)
	}
}

// A player that sets up EME never has to request anything that looks like a
// manifest; the licence exchange alone must be enough to recognise DRM.
func TestBrowserResolverRecognisesDRMFromEMEAlone(t *testing.T) {
	t.Parallel()
	site := drmSite(t, map[string]string{
		"watch": `
navigator.requestMediaKeySystemAccess('org.w3.clearkey', eme)
  .then((access) => access.createMediaKeys())
  .then((keys) => document.getElementById('v').setMediaKeys(keys).then(() => keys))
  .then((keys) => {
    const session = keys.createSession();
    session.addEventListener('message', (m) =>
      fetch('/drm/license/` + secret + `?token=` + secret + `', { method: 'POST', body: m.message }));
    return session.generateRequest('keyids', new TextEncoder().encode(JSON.stringify({ kids: ['AAAAAAAAAAAAAAAAAAAAAA'] })));
  });`,
	})

	_, diag, err := resolvePage(t, browserForTest(t), site.URL+"/watch")
	if !errors.Is(err, live.ErrDRMProtected) || diag.Reason != "eme_session" {
		t.Fatalf("error = %v (%s), want ErrDRMProtected/eme_session", err, diag.Reason)
	}
	d := diag.DRM
	if d == nil || !d.Confirmed {
		t.Fatalf("drm diagnostics = %+v, want confirmed", d)
	}
	for _, want := range []string{"media_keys_attached", "key_session_created", "license_request_generated", "license_server_contacted"} {
		if !contains(d.Signals, want) {
			t.Errorf("signals = %v, want %s", d.Signals, want)
		}
	}
	if strings.Join(d.KeySystems, ",") != "org.w3.clearkey" || strings.Join(d.Systems, ",") != "ClearKey" ||
		strings.Join(d.LicenseHosts, ",") != "127.0.0.1" || strings.Join(d.InitDataTypes, ",") != "keyids" {
		t.Errorf("drm diagnostics = %+v", d)
	}
	// Once DRM is certain the page is not watched for the rest of its budget.
	if diag.ElapsedMS > 12000 {
		t.Errorf("resolve took %dms; it should stop soon after DRM is confirmed", diag.ElapsedMS)
	}
}

func TestBrowserResolverOnlySuspectsDRMFromAProbe(t *testing.T) {
	t.Parallel()
	site := drmSite(t, map[string]string{
		"watch": `navigator.requestMediaKeySystemAccess('com.widevine.alpha', eme).catch(() => {});`,
	})
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	_, err := live.NewBrowserResolver(browserForTest(t), testFetcher(), 5*time.Second).
		Resolve(ctx, &domain.LiveChannel{SourceURL: site.URL + "/watch", ResolverConfig: map[string]any{}})

	var re *live.ResolveError
	if !errors.As(err, &re) || re.Outcome != domain.OutcomeNoStreamFound || re.Reason != "drm_suspected" {
		t.Fatalf("error = %v, want NO_STREAM_FOUND/drm_suspected: a probe proves nothing on its own", err)
	}
	if errors.Is(err, live.ErrDRMProtected) || re.Diagnostics.DRM.Confirmed {
		t.Error("a capability probe was reported as confirmed DRM")
	}
	if !strings.Contains(err.Error(), "Widevine") {
		t.Errorf("error = %v, want it to name what the player asked for", err)
	}
}

func TestBrowserResolverPlaysAClearStreamWhosePlayerProbesDRM(t *testing.T) {
	t.Parallel()
	site := drmSite(t, map[string]string{
		"watch": `navigator.requestMediaKeySystemAccess('com.widevine.alpha', eme).catch(() => {});
fetch('/clear/master.m3u8?token=` + secret + `');`,
	})

	res, diag, err := resolvePage(t, browserForTest(t), site.URL+"/watch")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if !strings.HasSuffix(strings.Split(res.ManifestURL, "?")[0], "/clear/master.m3u8") {
		t.Errorf("manifest = %s", res.ManifestURL)
	}
	if diag.Outcome != domain.OutcomeStreamFound || diag.Reason != "" {
		t.Errorf("outcome = %s/%s, want a plain STREAM_FOUND", diag.Outcome, diag.Reason)
	}
	if diag.DRM == nil || diag.DRM.Confirmed || !contains(diag.DRM.Signals, "eme_access_requested") {
		t.Errorf("drm diagnostics = %+v, want the probe recorded but not confirmed", diag.DRM)
	}
}

// The advert in front of a DRM channel is unprotected. Restreaming it would report a
// DRM-protected source as playable.
func TestBrowserResolverDoesNotPassAnAdvertOffAsTheChannel(t *testing.T) {
	t.Parallel()
	site := drmSite(t, map[string]string{
		"watch": `fetch('/ad/master.m3u8'); fetch('/live/manifest.mpd');`,
	})

	_, diag, err := resolvePage(t, browserForTest(t), site.URL+"/watch")
	if !errors.Is(err, live.ErrDRMProtected) || diag.Reason != "only_clear_stream_ended" {
		t.Fatalf("error = %v (%s), want ErrDRMProtected/only_clear_stream_ended", err, diag.Reason)
	}
	verdicts := map[string]string{}
	for _, m := range diag.Manifests {
		verdicts[m.Kind] = m.Verdict
	}
	if verdicts["master"] != "usable" || verdicts["mpd"] != "drm_protected" {
		t.Errorf("verdicts = %v, want the advert usable-but-not-chosen and the MPD protected", verdicts)
	}
}

func contains(list []string, want string) bool {
	for _, v := range list {
		if v == want {
			return true
		}
	}
	return false
}

// Plenty of sites POST to a /license endpoint for reasons that have nothing to do
// with DRM. Without any use of EME, that is not evidence of it.
func TestBrowserResolverIgnoresLicenceLikeURLsWithoutEME(t *testing.T) {
	t.Parallel()
	site := drmSite(t, map[string]string{
		"watch": `fetch('/drm/license/check', { method: 'POST', body: 'seat=1' }); fetch('/clear/master.m3u8');`,
	})
	_, diag, err := resolvePage(t, browserForTest(t), site.URL+"/watch")
	if err != nil || diag.DRM != nil || diag.Reason != "" {
		t.Errorf("err = %v, drm = %+v, reason = %q, want a plain STREAM_FOUND", err, diag.DRM, diag.Reason)
	}
}
