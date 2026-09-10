package renderjob_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/jaminalder/go-graphics/internal/publish"
	"github.com/jaminalder/go-graphics/internal/renderjob"
)

type logBuffer struct {
	mu   sync.Mutex
	data bytes.Buffer
}

func (b *logBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.data.Write(p)
}
func (b *logBuffer) String() string { b.mu.Lock(); defer b.mu.Unlock(); return b.data.String() }
func captureLogs(t *testing.T) *logBuffer {
	t.Helper()
	output := &logBuffer{}
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(output, &slog.HandlerOptions{Level: slog.LevelInfo})))
	t.Cleanup(func() { slog.SetDefault(previous) })
	return output
}

func rendererSocket(t *testing.T, handler http.Handler) string {
	t.Helper()
	directory, err := os.MkdirTemp("", "art-log-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(directory) })
	socket := filepath.Join(directory, "render.sock")
	listener, err := net.Listen("unix", socket)
	if err != nil {
		t.Fatal(err)
	}
	server := &http.Server{Handler: handler}
	t.Cleanup(func() { _ = server.Close() })
	go func() { _ = server.Serve(listener) }()
	return socket
}

// TestRendererCallsReportActualStatusAndBodyFailures exercises the real Unix
// transport, including a successful status whose body fails partway through.
func TestRendererCallsReportActualStatusAndBodyFailures(t *testing.T) {
	cases := []struct {
		name      string
		status    int
		build     string
		short     bool
		wantError bool
	}{
		{"success", 200, "test", false, false},
		{"unavailable", 503, "test", false, true},
		{"wrong release", 200, "old", false, true},
		{"truncated body", 200, "test", true, true},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			output := captureLogs(t)
			socket := rendererSocket(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("X-Renderer-Build", test.build)
				if test.short {
					w.Header().Set("Content-Length", "1000")
				}
				w.WriteHeader(test.status)
				_, _ = io.WriteString(w, "SECRET-RESPONSE-BODY")
			}))
			client := renderjob.NewClient(socket, "test")
			q := request(t, 42)
			err := client.Render(context.Background(), q, io.Discard)
			if (err != nil) != test.wantError {
				t.Fatal(err)
			}
			var record map[string]any
			if err := json.Unmarshal([]byte(output.String()), &record); err != nil {
				t.Fatal(err)
			}
			recipe, _ := publish.Validate(q.Recipe)
			tier, _ := publish.Tier(recipe.ID(), q.Tier)
			if record["status"] != float64(test.status) || record["operation"] != "render" || record["direction"] != "outgoing" || record["job"] != recipe.Key(tier, "test") {
				t.Fatal(record)
			}
			if _, ok := record["error"]; ok != test.wantError {
				t.Fatal("missing or unexpected failure", record)
			}
			if strings.Contains(output.String(), "SECRET") {
				t.Fatal("response body leaked into logs")
			}
		})
	}
}

// TestUnavailableRendererIsVisibleButHealthyProbesStayQuiet keeps readiness
// useful during an incident without logging every successful background check.
func TestUnavailableRendererIsVisibleButHealthyProbesStayQuiet(t *testing.T) {
	t.Run("missing socket", func(t *testing.T) {
		output := captureLogs(t)
		client := renderjob.NewClient(filepath.Join(t.TempDir(), "missing.sock"), "test")
		if err := client.Render(context.Background(), request(t, 42), io.Discard); err == nil {
			t.Fatal("missing renderer succeeded")
		}
		var record map[string]any
		if err := json.Unmarshal([]byte(output.String()), &record); err != nil {
			t.Fatal(err)
		}
		if record["status"] != float64(0) || record["error"] == nil {
			t.Fatal(record)
		}
	})
	for _, status := range []int{204, 503} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			output := captureLogs(t)
			socket := rendererSocket(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("X-Renderer-Build", "test")
				w.WriteHeader(status)
			}))
			healthy := renderjob.NewClient(socket, "test").Health(context.Background())
			if healthy != (status == 204) {
				t.Fatal("wrong health result")
			}
			if status == 204 && output.String() != "" {
				t.Fatal("healthy probe was noisy")
			}
			if status == 503 && (!strings.Contains(output.String(), `"operation":"health"`) || !strings.Contains(output.String(), `"level":"WARN"`)) {
				t.Fatal(output.String())
			}
		})
	}
}
