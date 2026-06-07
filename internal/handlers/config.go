package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
)

// validWidgets is the set of widget names allowed in any screen slot.
var validWidgets = map[string]bool{
	"clock":   true,
	"music":   true,
	"weather": true,
	"tasks":   true,
	"gif":     true,
}

// configPatch is the request body for PUT /config. Pointers distinguish
// "absent" (nil, leave unchanged) from "set to value".
type configPatch struct {
	Screen1 *string `json:"screen1"`
	Screen2 *string `json:"screen2"`
	Screen3 *string `json:"screen3"`
	GifURL1 *string `json:"gif_url_1"`
	GifURL2 *string `json:"gif_url_2"`
	GifURL3 *string `json:"gif_url_3"`
}

// validWidget returns an error if w is not a known widget name.
func validWidget(slot string, w *string) error {
	if w == nil {
		return nil
	}
	if !validWidgets[*w] {
		return fmt.Errorf("%s: invalid widget %q", slot, *w)
	}
	return nil
}

// GetConfig returns the current screen config.
func (h *Handlers) GetConfig(w http.ResponseWriter, r *http.Request) {
	cfg, err := h.DB.GetConfig()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not read config")
		return
	}
	writeJSON(w, http.StatusOK, cfg)
}

// UpdateConfig applies a partial update to the screen config and returns the
// full updated config.
func (h *Handlers) UpdateConfig(w http.ResponseWriter, r *http.Request) {
	var patch configPatch
	if err := json.NewDecoder(r.Body).Decode(&patch); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	if patch.Screen1 == nil && patch.Screen2 == nil && patch.Screen3 == nil &&
		patch.GifURL1 == nil && patch.GifURL2 == nil && patch.GifURL3 == nil {
		writeError(w, http.StatusBadRequest, "no screen fields provided")
		return
	}

	for _, v := range []struct {
		slot string
		val  *string
	}{
		{"screen1", patch.Screen1},
		{"screen2", patch.Screen2},
		{"screen3", patch.Screen3},
	} {
		if err := validWidget(v.slot, v.val); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
	}

	cfg, err := h.DB.UpdateConfig(
		patch.Screen1, patch.Screen2, patch.Screen3,
		patch.GifURL1, patch.GifURL2, patch.GifURL3,
	)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not update config")
		return
	}

	// Broadcast the new config to all SSE subscribers (non-blocking).
	if b, err := json.Marshal(cfg); err == nil {
		h.Hub.broadcast(string(b))
	}

	writeJSON(w, http.StatusOK, cfg)
}
