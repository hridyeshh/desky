package handlers

import (
	"net/http"
	"sync"
	"time"
)

// heartbeatWindow is how recently the Pi must have pinged for the device to
// count as connected.
const heartbeatWindow = 12 * time.Second

// liveness tracks the last heartbeat timestamp from the edge device.
// Package-level so it survives across requests for the process lifetime.
var liveness = struct {
	sync.Mutex
	lastSeen time.Time
}{}

// Heartbeat records a ping from the Pi (POST /api/heartbeat).
func (h *Handlers) Heartbeat(w http.ResponseWriter, r *http.Request) {
	liveness.Lock()
	liveness.lastSeen = time.Now()
	liveness.Unlock()
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// Status reports whether the device pinged within the heartbeat window
// (GET /api/status).
func (h *Handlers) Status(w http.ResponseWriter, r *http.Request) {
	liveness.Lock()
	last := liveness.lastSeen
	liveness.Unlock()

	status := "disconnected"
	if !last.IsZero() && time.Since(last) < heartbeatWindow {
		status = "connected"
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": status})
}
