package web

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

func (a *app) events(w http.ResponseWriter, r *http.Request) {
	if a.cfg.Events == nil {
		http.NotFound(w, r)
		return
	}
	if origin := r.Header.Get("Origin"); origin != "" && origin != a.cfg.Origin {
		http.Error(w, "origin rejected", http.StatusForbidden)
		return
	}
	select {
	case a.streams <- struct{}{}:
		defer func() { <-a.streams }()
	default:
		http.Error(w, "too many streams", http.StatusServiceUnavailable)
		return
	}
	id := strings.TrimPrefix(r.URL.Path, "/events/")
	s, err := a.session(r)
	if err != nil {
		a.failError(w, r, err, 410)
		return
	}
	updates, unsubscribe := a.cfg.Events.Subscribe()
	defer unsubscribe()
	var previous string
	snapshot := func() (string, bool) {
		if _, e := a.session(r); e != nil {
			return "", false
		}
		x, e := a.store(r).Get(s.Token, id)
		if e != nil {
			return "", false
		}
		b, e := json.Marshal(x)
		return string(b), e == nil
	}
	previous, ok := snapshot()
	if !ok {
		http.Error(w, "unavailable", http.StatusServiceUnavailable)
		return
	}
	rc := http.NewResponseController(w)
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Accel-Buffering", "no")
	send := func(event string) bool {
		_ = rc.SetWriteDeadline(time.Now().Add(5 * time.Second))
		if _, e := fmt.Fprintf(w, "event: %s\ndata: refresh\n\n", event); e != nil {
			return false
		}
		if rc.Flush() != nil {
			return false
		}
		// Limit each write, not the time the connection waits for the next event.
		return rc.SetWriteDeadline(time.Time{}) == nil
	}
	if !send("snapshot") {
		return
	}
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case <-updates:
		case <-ticker.C:
		}
		current, ok := snapshot()
		if !ok {
			return
		}
		if current != previous {
			previous = current
			if !send("changed") {
				return
			}
		} else if !send("heartbeat") {
			return
		}
	}
}
