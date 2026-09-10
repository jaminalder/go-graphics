package logging

import (
	"log/slog"
	"net/http"
	"strings"
	"time"
)

// HTTP records one completion per request. Successful polling, health checks and
// asset/image reads are debug events; failures remain visible at normal levels.
func HTTP(logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		response := &responseWriter{ResponseWriter: w}
		returned := false
		defer func() {
			status := response.status
			if status == 0 && returned {
				status = http.StatusOK
			}
			path, quiet := route(r.URL.Path)
			level := slog.LevelInfo
			if quiet {
				level = slog.LevelDebug
			}
			if status >= 400 {
				level = slog.LevelWarn
			}
			if status >= 500 || !returned {
				level = slog.LevelError
			}
			method := r.Method
			switch method {
			case "GET", "HEAD", "POST", "PUT", "PATCH", "DELETE", "OPTIONS", "CONNECT", "TRACE":
			default:
				method = "OTHER"
			}
			attrs := []any{"direction", "incoming", "method", method, "path", path, "status", status, "elapsed_ms", time.Since(started).Milliseconds(), "bytes", response.bytes}
			if !returned {
				attrs = append(attrs, "outcome", "aborted")
			}
			logger.Log(r.Context(), level, "http completed", attrs...)
		}()
		next.ServeHTTP(response, r)
		returned = true
	})
}

type responseWriter struct {
	http.ResponseWriter
	status int
	bytes  int64
}

func (w *responseWriter) WriteHeader(status int) {
	if w.status != 0 {
		return
	}
	if status >= 200 || status == http.StatusSwitchingProtocols {
		w.status = status
	}
	w.ResponseWriter.WriteHeader(status)
}

func (w *responseWriter) Write(p []byte) (int, error) {
	if w.status == 0 {
		w.WriteHeader(http.StatusOK)
	}
	n, err := w.ResponseWriter.Write(p)
	w.bytes += int64(n)
	return n, err
}

// Unwrap retains ResponseController's access to the original transport.
func (w *responseWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }

// FlushError records implicit headers while preserving streaming semantics.
func (w *responseWriter) FlushError() error {
	if w.status == 0 {
		w.WriteHeader(http.StatusOK)
	}
	return http.NewResponseController(w.ResponseWriter).Flush()
}

// route keeps credential-bearing IDs and arbitrary user input out of logs.
func route(path string) (string, bool) {
	switch path {
	case "/", "/about", "/favourites", "/explorations", "/recover", "/recovery", "/restore", "/export", "/clear", "/render", "/generation/on", "/generation/off":
		return path, false
	case "/health", "/health/live", "/health/ready", "/ready", "/metrics":
		return path, true
	}
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) == 3 && parts[0] == "assets" {
		return "/assets/{asset}", true
	}
	if len(parts) == 2 {
		switch parts[0] {
		case "images":
			return "/images/{image}", true
		case "downloads":
			return "/downloads/{image}", false
		case "art":
			return "/art/{artwork}", false
		}
	}
	prefix := ""
	quiet := false
	if parts[0] == "fragments" {
		prefix = "/fragments"
		quiet = true
		parts = parts[1:]
	}
	if len(parts) >= 2 && parts[0] == "explorations" {
		base := prefix + "/explorations/{exploration}"
		if len(parts) == 2 {
			return base, quiet
		}
		if len(parts) == 4 && parts[2] == "samples" {
			return base + "/samples/{sample}", quiet
		}
		if len(parts) == 3 {
			switch parts[2] {
			case "choices", "batches", "similar", "favourites", "download", "cancel":
				return base + "/" + parts[2], quiet
			}
		}
	}
	return "/{unmatched}", false
}
