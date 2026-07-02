package api

import (
	"encoding/json"
	"net/http"
)

// NewRouter initializes Go 1.22 standard library ServeMux and registers routes.
func NewRouter() http.Handler {
	mux := http.NewServeMux()

	// Register liveness check endpoint for Kubernetes / Docker health probes.
	mux.HandleFunc("GET /healthz", healthCheckHandler)

	return mux
}

// healthCheckHandler responds with HTTP 200 and JSON status indicating the server is alive.
func healthCheckHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	response := map[string]string{
		"status": "ok",
	}

	_ = json.NewEncoder(w).Encode(response)
}
