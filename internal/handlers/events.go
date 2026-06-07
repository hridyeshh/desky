package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// Events is the SSE endpoint (GET /events). It sends the current config
// immediately, then streams config updates as they are broadcast, plus a
// comment ping every 30s to keep the connection alive.
func (h *Handlers) Events(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "streaming unsupported")
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no") // disable proxy buffering

	ch := h.Hub.register()
	defer h.Hub.unregister(ch)

	// Send the current config as the first event.
	if cfg, err := h.DB.GetConfig(); err == nil {
		if b, err := json.Marshal(cfg); err == nil {
			fmt.Fprintf(w, "data: %s\n\n", b)
			flusher.Flush()
		}
	}

	ping := time.NewTicker(30 * time.Second)
	defer ping.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case msg, ok := <-ch:
			if !ok {
				return
			}
			fmt.Fprintf(w, "data: %s\n\n", msg)
			flusher.Flush()
		case <-ping.C:
			fmt.Fprint(w, ": ping\n\n")
			flusher.Flush()
		}
	}
}
