// Package renderjob owns bounded admission, isolated execution and disposable artifacts.
package renderjob

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os/exec"
	"sync/atomic"
	"time"

	"github.com/jaminalder/go-graphics/internal/artwork"
	"github.com/jaminalder/go-graphics/internal/publish"
)

// MaxImage bounds child stdout and each cached artifact independently.
const MaxImage int64 = 16 << 20

// Request is the private, versioned and release-bound renderer input.
type Request struct {
	Version int             `json:"version"`
	Build   string          `json:"build"`
	Recipe  json.RawMessage `json:"recipe"`
	Tier    string          `json:"tier"`
}

// Validate rejects unsupported public input at both ends of the transport.
func (q Request) Validate(build string) error {
	if q.Version != 1 || q.Build != build || build == "" {
		return errors.New("renderer release mismatch")
	}
	r, err := publish.Validate(q.Recipe)
	if err != nil {
		return err
	}
	_, err = publish.Tier(r.ID(), q.Tier)
	return err
}

// Renderer writes exactly one bounded PNG or returns an error.
type (
	Renderer interface {
		Render(context.Context, Request, io.Writer) error
	}
	// Supervisor executes only its fixed executable and admits at most one child.
	Supervisor struct {
		Executable, Build string
		active            atomic.Bool
	}
)

// Render validates input, then kills and reaps a child on deadline or output overflow.
func (s *Supervisor) Render(ctx context.Context, q Request, w io.Writer) (err error) {
	if err := q.Validate(s.Build); err != nil {
		return err
	}
	if !s.active.CompareAndSwap(false, true) {
		return errors.New("renderer busy")
	}
	defer s.active.Store(false)
	started := time.Now()
	attrs := requestLogAttrs(q)
	slog.InfoContext(ctx, "renderer started", attrs...)
	defer func() {
		level := slog.LevelInfo
		attrs = append(attrs, "elapsed_ms", time.Since(started).Milliseconds())
		if err != nil {
			level = slog.LevelError
			attrs = append(attrs, "error", err)
		}
		slog.Log(ctx, level, "renderer finished", attrs...)
	}()
	timeout := 15 * time.Second
	if q.Tier == "download" {
		timeout = 30 * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	data, err := json.Marshal(q)
	if err != nil {
		return err
	}
	cmd := exec.CommandContext(ctx, s.Executable, "--child")
	cmd.Stdin = bytes.NewReader(data)
	cmd.WaitDelay = time.Second
	stdout := &limitWriter{w: w, left: MaxImage, cancel: cancel}
	stderr := &limitWriter{w: io.Discard, left: 8192, cancel: cancel}
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	err = cmd.Run()
	if stdout.exceeded || stderr.exceeded {
		return errors.New("renderer output exceeded limit")
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}
	return err
}

type limitWriter struct {
	w        io.Writer
	left     int64
	cancel   context.CancelFunc
	exceeded bool
}

func (w *limitWriter) Write(p []byte) (int, error) {
	if int64(len(p)) > w.left {
		w.exceeded = true
		w.cancel()
		return 0, errors.New("renderer output exceeded limit")
	}
	n, err := w.w.Write(p)
	w.left -= int64(n)
	return n, err
}

// Handler serves the private Unix-socket protocol without a second queue.
func (s *Supervisor) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" && r.Method == "GET" {
			w.Header().Set("X-Renderer-Build", s.Build)
			w.WriteHeader(204)
			return
		}
		if r.URL.Path != "/render" || r.Method != "POST" {
			http.NotFound(w, r)
			return
		}
		data, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 32768))
		var q Request
		if err == nil {
			err = artwork.StrictJSON(data, &q)
		}
		if err != nil || q.Validate(s.Build) != nil {
			http.Error(w, "invalid renderer request", http.StatusBadRequest)
			return
		}
		var b bytes.Buffer
		if err := s.Render(r.Context(), q, &b); err != nil {
			http.Error(w, "renderer failed", http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "image/png")
		w.Header().Set("X-Renderer-Build", s.Build)
		if _, err := w.Write(b.Bytes()); err != nil {
			return
		}
	})
}

// Child executes a validated request inside the separately limited process.
func Child(in io.Reader, out io.Writer, build string) error {
	b, err := io.ReadAll(io.LimitReader(in, 32769))
	if err != nil || len(b) > 32768 {
		return errors.New("invalid input size")
	}
	var q Request
	if err := artwork.StrictJSON(b, &q); err != nil {
		return err
	}
	if err := q.Validate(build); err != nil {
		return err
	}
	r, err := publish.Validate(q.Recipe)
	if err != nil {
		return err
	}
	tier, err := publish.Tier(r.ID(), q.Tier)
	if err != nil {
		return err
	}
	return r.Render(out, tier, build)
}

// Client is the web process's Unix-socket renderer adapter.
type Client struct {
	http  *http.Client
	Build string
}

// NewClient fixes the private socket path and finite transport timeouts.
func NewClient(socket, build string) *Client {
	return &Client{Build: build, http: &http.Client{Timeout: 35 * time.Second, Transport: &http.Transport{MaxConnsPerHost: 2, ResponseHeaderTimeout: 32 * time.Second, DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
		return (&net.Dialer{Timeout: time.Second}).DialContext(ctx, "unix", socket)
	}}}}
}

// Render returns only complete successful output from the matching renderer release.
func (c *Client) Render(ctx context.Context, q Request, w io.Writer) (err error) {
	started := time.Now()
	status := 0
	var size int64
	defer func() {
		attrs := append(requestLogAttrs(q), "direction", "outgoing", "operation", "render", "status", status, "elapsed_ms", time.Since(started).Milliseconds(), "bytes", size)
		level := slog.LevelInfo
		if err != nil {
			level = slog.LevelError
			attrs = append(attrs, "error", err)
		}
		slog.Log(ctx, level, "renderer request completed", attrs...)
	}()
	if err := q.Validate(c.Build); err != nil {
		return err
	}
	b, err := json.Marshal(q)
	if err != nil {
		return err
	}
	r, err := http.NewRequestWithContext(ctx, "POST", "http://renderer/render", bytes.NewReader(b))
	if err != nil {
		return err
	}
	r.Header.Set("Content-Type", "application/json")
	res, err := c.http.Do(r)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	status = res.StatusCode
	if status != 200 {
		return errors.New("renderer returned unsuccessful status")
	}
	if res.Header.Get("X-Renderer-Build") != c.Build {
		return errors.New("renderer release mismatch")
	}
	size, err = io.Copy(w, io.LimitReader(res.Body, MaxImage+1))
	if size > MaxImage {
		return errors.New("oversized renderer output")
	}
	return err
}

// Health checks availability without producing an image.
func (c *Client) Health(ctx context.Context) (healthy bool) {
	started := time.Now()
	status := 0
	var failure error
	defer func() {
		level := slog.LevelDebug
		attrs := []any{"direction", "outgoing", "operation", "health", "status", status, "elapsed_ms", time.Since(started).Milliseconds()}
		if !healthy {
			level = slog.LevelWarn
			attrs = append(attrs, "error", failure)
		}
		slog.Log(ctx, level, "renderer request completed", attrs...)
	}()
	r, err := http.NewRequestWithContext(ctx, "GET", "http://renderer/health", nil)
	if err != nil {
		failure = err
		return false
	}
	res, err := c.http.Do(r)
	if err != nil {
		failure = err
		return false
	}
	defer res.Body.Close()
	status = res.StatusCode
	if status != 204 {
		failure = errors.New("renderer health returned unsuccessful status")
		return false
	}
	if res.Header.Get("X-Renderer-Build") != c.Build {
		failure = errors.New("renderer release mismatch")
		return false
	}
	return true
}

func requestLogAttrs(q Request) []any {
	r, err := publish.Validate(q.Recipe)
	if err != nil {
		return nil
	}
	tier, err := publish.Tier(r.ID(), q.Tier)
	if err != nil {
		return nil
	}
	return []any{"job", r.Key(tier, q.Build), "artwork", r.ID(), "tier", q.Tier}
}
