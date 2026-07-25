package middleware

import (
	"net/http"
	"strings"
	"sync"
)

var (
	allowedOrigins   map[string]bool
	allowedOriginsMu sync.RWMutex
)

// SetAllowedOrigins parses the comma-separated CORS_ALLOWED_ORIGINS config value
// into an O(1) lookup set. Must be called before the HTTP server starts.
func SetAllowedOrigins(origins string) {
	allowedOriginsMu.Lock()
	defer allowedOriginsMu.Unlock()

	allowedOrigins = make(map[string]bool)
	for _, o := range strings.Split(origins, ",") {
		o = strings.TrimSpace(o)
		if o != "" {
			allowedOrigins[o] = true
		}
	}
}

// IsOriginAllowed checks whether the given origin is in the configured whitelist.
// Returns false if no origins have been configured (fail-closed).
func IsOriginAllowed(origin string) bool {
	allowedOriginsMu.RLock()
	defer allowedOriginsMu.RUnlock()

	if allowedOrigins == nil || len(allowedOrigins) == 0 {
		return false
	}
	if allowedOrigins["*"] {
		return true
	}
	return allowedOrigins[origin]
}

// CORS wraps an HTTP handler and applies Cross-Origin Resource Sharing headers.
// It validates the request Origin against the configured whitelist before reflecting it.
// Requests from non-whitelisted origins are served without CORS headers (browser blocks them).
func CORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin != "" && IsOriginAllowed(origin) {
			// Only reflect origins that are explicitly whitelisted
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Credentials", "true")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS, PATCH")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Session-ID, X-Requested-With, Cookie, Accept, Origin")
			w.Header().Set("Access-Control-Max-Age", "86400")
		}

		// Intercept OPTIONS preflight requests immediately before reaching router
		if r.Method == http.MethodOptions {
			if origin != "" && !IsOriginAllowed(origin) {
				w.WriteHeader(http.StatusForbidden)
				return
			}
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

