package httpapi

import (
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

func (s *Server) events(w http.ResponseWriter, r *http.Request) {
	if !allowMethod(w, r, http.MethodGet) {
		return
	}
	user, _ := requestUser(r)
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "streaming is not supported")
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	client, unsubscribe := s.hub.Subscribe(user.ID, parseEventSequence(r))
	defer unsubscribe()

	writeSSE(w, "ready", map[string]any{
		"version":      s.version,
		"lastSequence": s.hub.Latest(user.ID),
	})
	flusher.Flush()
	keepAlive := time.NewTicker(25 * time.Second)
	defer keepAlive.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case event := <-client:
			writeSSEID(w, "clip", event.Sequence, event)
			flusher.Flush()
		case <-keepAlive.C:
			_, _ = io.WriteString(w, ": keep-alive\n\n")
			flusher.Flush()
		}
	}
}

func parseEventSequence(r *http.Request) uint64 {
	raw := strings.TrimSpace(r.Header.Get("Last-Event-ID"))
	if raw == "" {
		raw = strings.TrimSpace(r.URL.Query().Get("since"))
	}
	sequence, _ := strconv.ParseUint(raw, 10, 64)
	return sequence
}
