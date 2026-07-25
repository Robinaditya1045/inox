package middleware

import (
	"bufio"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/inox/inox/backend/pkg/logger"
)

// RequestLogger is a middleware that injects a request_id and logs HTTP request and response details.
func RequestLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		requestID := r.Header.Get("X-Request-Id")
		if requestID == "" {
			requestID = uuid.New().String()
		}

		// Attach requestID to response headers
		w.Header().Set("X-Request-Id", requestID)

		// Create contextual logger
		ctxLogger := slog.Default().With(
			slog.String("request_id", requestID),
			slog.String("method", r.Method),
			slog.String("path", r.URL.Path),
			slog.String("remote_ip", r.RemoteAddr),
		)

		ctx := logger.WithLogger(r.Context(), ctxLogger)
		r = r.WithContext(ctx)

		// Create a custom response writer to capture the status code
		rw := &responseWriter{ResponseWriter: w, status: http.StatusOK}

		// ctxLogger.Info("request started")

		next.ServeHTTP(rw, r)

		duration := time.Since(start)
		
		if rw.status >= 500 {
			ctxLogger.Error("request failed", slog.Int("status", rw.status), slog.Duration("duration", duration))
		} else {
			// ctxLogger.Info("request completed", slog.Int("status", rw.status), slog.Duration("duration", duration))
		}
	})
}

// responseWriter is a custom wrapper to capture the HTTP status code.
type responseWriter struct {
	http.ResponseWriter
	status int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.status = code
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *responseWriter) Unwrap() http.ResponseWriter {
	return rw.ResponseWriter
}

// Hijack implements http.Hijacker to allow WebSocket upgrades.
func (rw *responseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	hijacker, ok := rw.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, fmt.Errorf("underlying ResponseWriter does not implement http.Hijacker")
	}
	return hijacker.Hijack()
}

// Flush implements http.Flusher to allow streaming responses.
func (rw *responseWriter) Flush() {
	if flusher, ok := rw.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}
