package live

import (
	"net/http"
	"net/url"
	"slices"
	"sort"
	"testing"
	"time"
)

func cookieNames(t *testing.T, jarURL string, jar http.CookieJar) []string {
	t.Helper()
	u, err := url.Parse(jarURL)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, c := range jar.Cookies(u) {
		names = append(names, c.Name)
	}
	sort.Strings(names)
	return names
}

func TestSeedJarScopesThePageSessionLikeABrowser(t *testing.T) {
	now := time.Now()
	later := float64(now.Add(time.Hour).Unix())
	jar := newCookieJar()
	carried, withheld := seedJar(jar, []browserCookie{
		{Name: "edge", Value: "1", Domain: "cdn.example.com", Path: "/", Expires: later}, // host-only
		{Name: "site", Value: "2", Domain: ".example.com", Path: "/", Session: true},     // whole domain
		{Name: "secure", Value: "3", Domain: ".example.com", Path: "/", Secure: true, Session: true},
		{Name: "scoped", Value: "4", Domain: "cdn.example.com", Path: "/live", Session: true},
		{Name: "stale", Value: "5", Domain: "cdn.example.com", Path: "/", Expires: float64(now.Add(-time.Hour).Unix())},
		{Name: "cf_clearance", Value: "6", Domain: ".example.com", Path: "/", Session: true},
		{Name: "_abck", Value: "7", Domain: ".example.com", Path: "/", Session: true},
		{Name: "tracker", Value: "8", Domain: ".ads.test", Path: "/", Session: true},
	}, now)

	if carried != 5 || !slices.Equal(withheld, []string{"cf_clearance", "_abck"}) {
		t.Errorf("carried, withheld = %d, %v; want 5 carried and the 2 bot-protection clearances named", carried, withheld)
	}
	for _, tc := range []struct {
		url  string
		want []string
	}{
		{"https://cdn.example.com/live/master.m3u8", []string{"edge", "scoped", "secure", "site"}},
		{"http://cdn.example.com/seg.ts", []string{"edge", "site"}},          // no secure cookie over http, no /live cookie
		{"https://edge2.cdn.example.com/seg.ts", []string{"secure", "site"}}, // host-only cookies stay on their host
		{"https://example.org/", nil},
	} {
		if got := cookieNames(t, tc.url, jar); !slices.Equal(got, tc.want) {
			t.Errorf("cookies for %s = %v, want %v", tc.url, got, tc.want)
		}
	}
}

func TestSentCookiesCountsAndNamesClearancesOnly(t *testing.T) {
	n, bots := sentCookies("sid=abc; CF_Clearance=xyz;  theme=dark; incap_ses_12_34=q")
	if n != 4 {
		t.Errorf("count = %d, want 4", n)
	}
	if !slices.Equal(bots, []string{"CF_Clearance", "incap_ses_12_34"}) {
		t.Errorf("clearances = %v", bots)
	}
	if n, bots := sentCookies(""); n != 0 || bots != nil {
		t.Errorf("empty header = %d %v", n, bots)
	}
	for _, name := range []string{"session", "hdntl", "CloudFront-Policy", "_pxl"} {
		if botProtectionCookie(name) {
			t.Errorf("%s taken for a bot-protection cookie", name)
		}
	}
}

func TestScopeCookieHeaderScopesToTheManifestHost(t *testing.T) {
	jar := newCookieJar()
	clean, domain := scopeCookieHeader(map[string]string{
		"Referer":    "https://watch.example/",
		"cookie":     "sid=abc; edge=xyz",
		"Set-Cookie": "leaked=1", // a response header; must never go out on a request
	}, "https://cdn.example.com/live/master.m3u8", jar)

	if domain != "cdn.example.com" {
		t.Errorf("scoped to %q, want the manifest host", domain)
	}
	if _, ok := clean["Cookie"]; ok {
		t.Error("Cookie left in the broadcast header set; it must travel in the jar")
	}
	if _, ok := clean["Set-Cookie"]; ok {
		t.Error("Set-Cookie left in the header set")
	}
	if clean["Referer"] != "https://watch.example/" {
		t.Errorf("Referer = %q, want it kept", clean["Referer"])
	}
	if got := cookieNames(t, "https://cdn.example.com/live/x.ts", jar); !slices.Equal(got, []string{"edge", "sid"}) {
		t.Errorf("jar for the manifest host = %v, want both operator cookies", got)
	}
	if got := cookieNames(t, "https://other.example.net/", jar); got != nil {
		t.Errorf("operator cookies reached %v; they must stay on the manifest host", got)
	}
}

func TestCookieDomainsAreHostsOnly(t *testing.T) {
	got := cookieDomains([]browserCookie{
		{Name: "a", Domain: ".example.com"},
		{Name: "b", Domain: "cdn.example.com"},
		{Name: "c", Domain: "example.com"}, // same host as the first, once the dot is stripped
		{Name: "d", Domain: ""},            // no host: ignored
	})
	if !slices.Equal(got, []string{"cdn.example.com", "example.com"}) {
		t.Errorf("domains = %v, want deduplicated hosts", got)
	}
}
