package live

import (
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"sort"
	"strings"
	"time"

	"golang.org/x/net/publicsuffix"
)

// maxSessionCookies bounds how many cookies one page session hands the server. A
// page thick with advertising can hold hundreds; none of them is worth more memory.
const maxSessionCookies = 300

// newCookieJar is the cookie store for one channel's upstream session: what a page
// or an upstream set, sent back only to the hosts each cookie belongs to, exactly as
// a browser would. The public suffix list stops an upstream from setting a cookie
// for a whole registry such as co.uk.
func newCookieJar() http.CookieJar {
	jar, _ := cookiejar.New(&cookiejar.Options{PublicSuffixList: publicsuffix.List}) // never fails
	return jar
}

// browserCookie is a cookie as Chromium's Storage.getCookies reports it.
type browserCookie struct {
	Name     string  `json:"name"`
	Value    string  `json:"value"`
	Domain   string  `json:"domain"` // a leading dot marks a domain cookie; none, a host-only one
	Path     string  `json:"path"`
	Expires  float64 `json:"expires"` // seconds since the epoch
	Secure   bool    `json:"secure"`
	HTTPOnly bool    `json:"httpOnly"`
	Session  bool    `json:"session"`
}

// seedJar carries a page session's cookies over to the server, so that the requests
// the proxy makes for a stream carry the same ones the page's player did. The
// cookies were issued to our own headless browser, as an anonymous visitor, by the
// sites it loaded; they are request context in the same way the Referer is.
//
// Bot-protection clearances are the exception, and are withheld: they certify that
// the browser passed a check, and presenting one from a client that never took it
// would be getting around that check. A stream behind one stays unrestreamable.
func seedJar(jar http.CookieJar, cookies []browserCookie, now time.Time) (carried int, withheld []string) {
	for _, c := range cookies {
		switch {
		case c.Name == "" || c.Domain == "":
			continue
		case !c.Session && c.Expires > 0 && time.Unix(int64(c.Expires), 0).Before(now):
			continue
		case botProtectionCookie(c.Name):
			// Named here so the operator can see exactly which cookies were held
			// back and check the reason for themselves; the value is never kept.
			withheld = appendUnique(withheld, c.Name)
			continue
		case carried == maxSessionCookies:
			continue
		}
		host := strings.TrimPrefix(c.Domain, ".")
		hc := &http.Cookie{Name: c.Name, Value: c.Value, Path: c.Path, Secure: c.Secure, HttpOnly: c.HTTPOnly}
		if strings.HasPrefix(c.Domain, ".") {
			hc.Domain = host
		}
		if hc.Path == "" {
			hc.Path = "/"
		}
		if !c.Session && c.Expires > 0 {
			hc.Expires = time.Unix(int64(c.Expires), 0)
		}
		jar.SetCookies(&url.URL{Scheme: "https", Host: host, Path: hc.Path}, []*http.Cookie{hc})
		carried++
	}
	return carried, withheld
}

// botProtectionNames and botProtectionPrefixes are the cookies bot-management and
// waiting-room services issue to a client that passed their check or their queue.
var (
	botProtectionNames = map[string]bool{
		// Cloudflare
		"cf_clearance": true, "__cf_bm": true, "__cfruid": true, "_cfuvid": true, "__cfwaitingroom": true,
		// Akamai Bot Manager
		"_abck": true, "bm_sz": true, "bm_sv": true, "bm_mi": true, "bm_so": true, "bm_s": true, "ak_bmsc": true,
		// DataDome, PerimeterX / HUMAN, Imperva, Kasada, AWS WAF, DDoS-Guard, reCAPTCHA
		"datadome": true, "_px": true, "_px2": true, "_px3": true, "_pxhd": true, "_pxvid": true, "pxcts": true,
		"reese84": true, "kp_uidz": true, "aws-waf-token": true, "_grecaptcha": true,
	}
	botProtectionPrefixes = []string{
		"cf_chl_", "__cf_chl", "_pxff_", "incap_ses_", "visid_incap_", "nlbi_", "__ddg",
		"sucuri_cloudproxy_", "akavpau_", "akavpwr_", "queueitaccepted-",
	}
)

func botProtectionCookie(name string) bool {
	name = strings.ToLower(name)
	if botProtectionNames[name] {
		return true
	}
	for _, prefix := range botProtectionPrefixes {
		if strings.HasPrefix(name, prefix) {
			return true
		}
	}
	return false
}

// sentCookies reads a Cookie request header for how many cookies it carried and
// which of them are bot-protection clearances. Names only: the values are credentials.
func sentCookies(header string) (count int, botNames []string) {
	for _, pair := range strings.Split(header, ";") {
		name, _, _ := strings.Cut(strings.TrimSpace(pair), "=")
		if name == "" {
			continue
		}
		count++
		if botProtectionCookie(name) {
			botNames = appendUnique(botNames, name)
		}
	}
	return count, botNames
}

// scopeCookieHeader takes a Cookie out of a broadcast header set and puts it in jar,
// scoped to manifestURL's host. This is the one supported way to give a source you
// control a cookie: it reaches that host alone, the way the jar sends everything
// else, rather than being copied onto every request the stream makes to every CDN.
// Set-Cookie is dropped -- it is a response header with no meaning on a request.
//
// It returns the headers without Cookie, and the host the cookie was scoped to (empty
// if there was none). A nil jar just strips the cookie, since there is nowhere to
// keep it.
func scopeCookieHeader(headers map[string]string, manifestURL string, jar http.CookieJar) (map[string]string, string) {
	var cookie string
	out := make(map[string]string, len(headers))
	for name, value := range headers {
		switch http.CanonicalHeaderKey(name) {
		case "Cookie":
			cookie = value
		case "Set-Cookie":
		default:
			out[http.CanonicalHeaderKey(name)] = value
		}
	}
	if cookie == "" || jar == nil {
		return out, ""
	}
	u, err := url.Parse(manifestURL)
	if err != nil || u.Host == "" {
		return out, ""
	}
	var cookies []*http.Cookie
	for _, pair := range strings.Split(cookie, ";") {
		name, value, ok := strings.Cut(strings.TrimSpace(pair), "=")
		if ok && name != "" {
			cookies = append(cookies, &http.Cookie{Name: name, Value: value, Path: "/"})
		}
	}
	if len(cookies) == 0 {
		return out, ""
	}
	jar.SetCookies(u, cookies)
	return out, u.Host
}

// cookieDomains lists the hosts a page session's cookies belong to, deduplicated and
// sorted. Hostnames only -- no names, no values -- so it is safe to log and show.
func cookieDomains(cookies []browserCookie) []string {
	var domains []string
	for _, c := range cookies {
		if host := strings.TrimPrefix(c.Domain, "."); host != "" {
			domains = appendUnique(domains, host)
		}
	}
	sort.Strings(domains)
	return domains
}
