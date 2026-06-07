package handlers

import "sync"

// Hub manages SSE subscribers. Each subscriber is a buffered channel that
// receives config JSON strings. All access is mutex-protected.
type Hub struct {
	mu          sync.Mutex
	subscribers map[chan string]struct{}
}

// NewHub returns an empty Hub.
func NewHub() *Hub {
	return &Hub{subscribers: make(map[chan string]struct{})}
}

// register adds a new subscriber and returns its channel.
func (h *Hub) register() chan string {
	ch := make(chan string, 8)
	h.mu.Lock()
	h.subscribers[ch] = struct{}{}
	h.mu.Unlock()
	return ch
}

// unregister removes a subscriber and closes its channel. Safe to call once.
func (h *Hub) unregister(ch chan string) {
	h.mu.Lock()
	if _, ok := h.subscribers[ch]; ok {
		delete(h.subscribers, ch)
		close(ch)
	}
	h.mu.Unlock()
}

// broadcast sends msg to every subscriber without blocking. A subscriber whose
// buffer is full is skipped for this message rather than stalling the caller.
func (h *Hub) broadcast(msg string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for ch := range h.subscribers {
		select {
		case ch <- msg:
		default: // slow subscriber — drop this message for it
		}
	}
}
