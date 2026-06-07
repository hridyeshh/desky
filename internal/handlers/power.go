package handlers

import (
	"encoding/json"
	"net/http"
	"strings"
)

// powerRequest is the body for POST /api/power.
type powerRequest struct {
	State string `json:"state"`
}

// SetPower toggles the global display power state ("ON" or "OFF") and returns
// the full updated config.
func (h *Handlers) SetPower(w http.ResponseWriter, r *http.Request) {
	var req powerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	state := strings.ToUpper(strings.TrimSpace(req.State))
	if state != "ON" && state != "OFF" {
		writeError(w, http.StatusBadRequest, `state must be "ON" or "OFF"`)
		return
	}

	cfg, err := h.DB.SetPowerState(state)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not set power state")
		return
	}
	writeJSON(w, http.StatusOK, cfg)
}
