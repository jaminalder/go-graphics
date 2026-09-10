package logging_test

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jaminalder/go-graphics/internal/logging"
)

func logger(output io.Writer, level slog.Level) *slog.Logger {
	return slog.New(slog.NewJSONHandler(output, &slog.HandlerOptions{Level: level}))
}

// TestHTTPRecordsTheFinalStatusWithoutChangingDelivery protects normal forms,
// conditional downloads, informational headers and ResponseController streaming.
func TestHTTPRecordsTheFinalStatusWithoutChangingDelivery(t *testing.T) {
	tests := []struct {
		name   string
		status int
		serve  http.HandlerFunc
	}{
		{"implicit", 200, func(w http.ResponseWriter, _ *http.Request) { _, _ = io.WriteString(w, "PNG bytes") }},
		{"redirect", 303, func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(303) }},
		{"not modified", 304, func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(304) }},
		{"forbidden", 403, func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(403) }},
		{"unavailable", 503, func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(503) }},
		{"first final wins", 403, func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(403); w.WriteHeader(200) }},
		{"early hints", 201, func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(103); w.WriteHeader(201) }},
		{"stream", 200, func(w http.ResponseWriter, _ *http.Request) {
			if err := http.NewResponseController(w).Flush(); err != nil {
				t.Error(err)
			}
			_, _ = io.WriteString(w, "stream bytes")
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var output bytes.Buffer
			handler := logging.HTTP(logger(&output, slog.LevelDebug), test.serve)
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, httptest.NewRequest("GET", "http://example.test/", nil))
			var record map[string]any
			if err := json.Unmarshal(output.Bytes(), &record); err != nil {
				t.Fatal(err)
			}
			if record["status"] != float64(test.status) {
				t.Fatal(record)
			}
			if test.name != "early hints" && response.Code != test.status {
				t.Fatal("logger changed response", response.Code)
			}
			if test.name == "implicit" && response.Body.String() != "PNG bytes" {
				t.Fatal("logger changed body")
			}
			if test.name == "stream" && (!response.Flushed || response.Body.String() != "stream bytes") {
				t.Fatal("logger broke flushing")
			}
		})
	}
}

// TestQuietSuccessesDoNotHideFailuresAndNeverLogCapabilities ensures routine
// polling stays quiet while diagnostics exclude paths, queries and headers.
func TestQuietSuccessesDoNotHideFailuresAndNeverLogCapabilities(t *testing.T) {
	for _, path := range []string{"/health/live", "/assets/SECRET-HASH/SECRET-ASSET", "/images/SECRET-IMAGE", "/fragments/explorations/SECRET-WORKSPACE/samples/SECRET-SAMPLE"} {
		for _, status := range []int{200, 503} {
			var output bytes.Buffer
			handler := logging.HTTP(logger(&output, slog.LevelInfo), http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(status) }))
			request := httptest.NewRequest("GET", "http://example.test"+path+"?token=SECRET-QUERY", strings.NewReader("SECRET-BODY"))
			request.Header.Set("Cookie", "SECRET-COOKIE")
			handler.ServeHTTP(httptest.NewRecorder(), request)
			if status == 200 && output.Len() != 0 {
				t.Fatal("routine success was noisy", output.String())
			}
			if status == 503 && (output.Len() == 0 || strings.Contains(output.String(), "SECRET")) {
				t.Fatal("failure missing or credentials exposed", output.String())
			}
		}
	}
	var output bytes.Buffer
	handler := logging.HTTP(logger(&output, slog.LevelDebug), http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(204) }))
	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "http://example.test/health", nil))
	if !strings.Contains(output.String(), `"level":"DEBUG"`) {
		t.Fatal("debug did not reveal health completion")
	}
}

// TestAbortedHandlersRemainAborted avoids swallowing panics or pretending an
// HTTP status was sent before net/http closes an aborted connection.
func TestAbortedHandlersRemainAborted(t *testing.T) {
	var output bytes.Buffer
	handler := logging.HTTP(logger(&output, slog.LevelDebug), http.HandlerFunc(func(http.ResponseWriter, *http.Request) { panic("SECRET-PANIC") }))
	func() {
		defer func() {
			if recover() == nil {
				t.Fatal("panic swallowed")
			}
		}()
		handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "http://example.test/", nil))
	}()
	if !strings.Contains(output.String(), `"outcome":"aborted"`) || !strings.Contains(output.String(), `"status":0`) || strings.Contains(output.String(), "SECRET") {
		t.Fatal(output.String())
	}
}

// TestInvalidLogLevelsFailClearly keeps a mistyped setting from silently hiding logs.
func TestInvalidLogLevelsFailClearly(t *testing.T) {
	if _, err := logging.New("artweb", "typo", io.Discard); err == nil {
		t.Fatal("accepted invalid log level")
	}
	var output bytes.Buffer
	log, err := logging.New("artweb", "debug", &output)
	if err != nil {
		t.Fatal(err)
	}
	log.Debug("visible")
	if !strings.Contains(output.String(), "service=artweb") || !strings.Contains(output.String(), "visible") {
		t.Fatal(output.String())
	}
}
