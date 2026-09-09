// Package renderjob owns bounded admission, isolated execution and disposable artifacts.
package renderjob

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
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
func (s *Supervisor) Render(ctx context.Context, q Request, w io.Writer) error {
	if err := q.Validate(s.Build); err != nil {
		return err
	}
	if !s.active.CompareAndSwap(false, true) {
		return errors.New("renderer busy")
	}
	defer s.active.Store(false)
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
	cmd.Stdout = &limitWriter{w: w, left: MaxImage, cancel: cancel}
	cmd.Stderr = &limitWriter{w: io.Discard, left: 8192, cancel: cancel}
	return cmd.Run()
}

type limitWriter struct {
	w      io.Writer
	left   int64
	cancel context.CancelFunc
}

func (w *limitWriter) Write(p []byte) (int, error) {
	if int64(len(p)) > w.left {
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
func (c *Client) Render(ctx context.Context, q Request, w io.Writer) error {
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
	if res.StatusCode != 200 || res.Header.Get("X-Renderer-Build") != c.Build {
		return errors.New("renderer unavailable or incompatible")
	}
	n, err := io.Copy(w, io.LimitReader(res.Body, MaxImage+1))
	if n > MaxImage {
		return errors.New("oversized renderer output")
	}
	return err
}

// Health checks availability without producing an image.
func (c *Client) Health(ctx context.Context) bool {
	r, err := http.NewRequestWithContext(ctx, "GET", "http://renderer/health", nil)
	if err != nil {
		return false
	}
	res, err := c.http.Do(r)
	if err != nil {
		return false
	}
	defer res.Body.Close()
	return res.StatusCode == 204 && res.Header.Get("X-Renderer-Build") == c.Build
}
