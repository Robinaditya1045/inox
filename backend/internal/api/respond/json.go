package respond

import (
	"encoding/json"
	"net/http"
)

// WriteJSON formats and sends a JSON response with the specified HTTP status code.
func WriteJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

// WriteError formats and sends a standard JSON error payload: {"error": "message"}.
func WriteError(w http.ResponseWriter, status int, message string) {
	WriteJSON(w, status, map[string]string{"error": message})
}
