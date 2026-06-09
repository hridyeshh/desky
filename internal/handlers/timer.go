package handlers

import (
	"encoding/json"
	"net/http"
	"time"
)

// timerRequest is the body for POST /api/timer.
type timerRequest struct {
	Screen  int `json:"screen"`
	Minutes int `json:"minutes"`
}

// SetTimer starts a countdown on a screen. The dropped timer carries a duration
// in minutes; the backend converts it to an absolute end time and snapshots the
// widget currently on that screen so the Pi can revert to it once the timer ends.
func (h *Handlers) SetTimer(w http.ResponseWriter, r *http.Request) {
	var req timerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	if req.Screen < 1 || req.Screen > 3 {
		writeError(w, http.StatusBadRequest, "screen must be 1, 2, or 3")
		return
	}
	if req.Minutes < 1 || req.Minutes > 180 {
		writeError(w, http.StatusBadRequest, "minutes must be between 1 and 180")
		return
	}

	// Snapshot the widget currently on this screen as the revert target.
	current, err := h.DB.GetConfig()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not read config")
		return
	}
	prev := map[int]string{1: current.Screen1, 2: current.Screen2, 3: current.Screen3}[req.Screen]

	endUnix := time.Now().Add(time.Duration(req.Minutes) * time.Minute).Unix()

	cfg, err := h.DB.SetTimer(req.Screen, endUnix, prev)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not start timer")
		return
	}

	// Broadcast so the Pi starts the countdown instantly.
	if b, err := json.Marshal(cfg); err == nil {
		h.Hub.broadcast(string(b))
	}

	writeJSON(w, http.StatusOK, cfg)
}
