package health

import (
	"encoding/json"
	"net/http"
)

// Handler sirve el endpoint /health.
type Handler struct{}

// NewHandler crea el handler HTTP para health.
func NewHandler() *Handler {
	return &Handler{}
}

// ServeHTTP escribe un JSON de health.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}
