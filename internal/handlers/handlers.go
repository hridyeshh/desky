package handlers

import (
	"encoding/json"
	"net/http"

	"desky/internal/db"
)

// Handlers holds dependencies shared by all HTTP handlers.
type Handlers struct {
	DB  *db.DB
	Hub *Hub
}

// New returns a Handlers wired to the given database.
func New(database *db.DB) *Handlers {
	return &Handlers{DB: database, Hub: NewHub()}
}

// writeJSON marshals v to JSON and writes it with the given status code.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

// writeError writes a JSON error body with the given status code.
func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

// Health responds with {"status": "ok"}.
func (h *Handlers) Health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
