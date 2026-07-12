package middleware

import (
	"net/http"
)

// CORS wraps an HTTP handler and applies Cross-Origin Resource Sharing headers.
// It intercepts OPTIONS preflight requests and allows credentials (cookies) for authentication.
func CORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin != "" {
			// Set exact origin to support credentials (cookies)
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Credentials", "true")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS, PATCH")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Session-ID, X-Requested-With, Cookie, Accept, Origin")
			w.Header().Set("Access-Control-Max-Age", "86400")
		}

		// Intercept OPTIONS preflight requests immediately before reaching router
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}
