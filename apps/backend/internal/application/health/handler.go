package health

import (
	"encoding/json"
	"net/http"
)

// Handler serves GET /health.
type Handler struct{}

// NewHandler returns the HTTP health check handler.
func NewHandler() *Handler {
	return &Handler{}
}

// ServeHTTP returns JSON {"status":"ok"}.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}
