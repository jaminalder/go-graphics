package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jaminalder/go-graphics/internal/persistence"
)

func TestMonitorRendersNamesDistinctBootsAndLegacyJobs(t *testing.T) {
	t.Setenv("ART_MONITOR_NAMES", `{"abc123":"art-persistent-79509-web-1","def456":"singular-seed-renderer-2"}`)
	s := persistence.MonitorSnapshot{At: time.Now(), Enabled: true, Build: "build", Queue: persistence.QueueSummary{Waiting: 2, Running: 1}}
	i := persistence.Instance{ID: "renderer-unique-12345678", Role: "renderer", Name: "def456", Hostname: "def456", Build: "build"}
	s.Instances = []persistence.InstanceStatus{{Instance: i, Status: "stale", AgeSeconds: 90, Running: 1}}
	s.Flow = []persistence.Flow{{Producer: persistence.Instance{ID: "web-unique-87654321", Name: "abc123", Hostname: "abc123"}, Renderer: i, State: "running", Jobs: 1}, {State: "available", Jobs: 2}}
	var out bytes.Buffer
	if err := renderMonitor(&out, s); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"art-persistent-79509-web-1", "singular-seed-renderer-2", "12345678", "87654321", "stale", "(legacy/unknown)", "(not claimed)", "waiting=2"} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("missing %q in %s", want, out.String())
		}
	}
}

func TestMonitorSanitizesTerminalControlCharacters(t *testing.T) {
	s := persistence.MonitorSnapshot{Build: "evil\x1b[2J", Instances: []persistence.InstanceStatus{{Instance: persistence.Instance{Name: "name\n\t\x1b", ID: "id"}}}}
	var out bytes.Buffer
	if err := renderMonitor(&out, s); err != nil {
		t.Fatal(err)
	}
	if strings.ContainsRune(out.String(), '\x1b') {
		t.Fatal("terminal escape reached display")
	}
}

type redirectTransport struct {
	base    http.RoundTripper
	address string
}

func (r redirectTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	copy := req.Clone(req.Context())
	copy.URL.Scheme = "http"
	copy.URL.Host = r.address
	return r.base.RoundTrip(copy)
}

func TestMonitorFetchRejectsUnavailableDataAndDecodesSnapshot(t *testing.T) {
	for _, status := range []int{200, 503} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/monitor" {
					t.Error("wrong endpoint")
				}
				w.WriteHeader(status)
				_ = json.NewEncoder(w).Encode(persistence.MonitorSnapshot{Build: "test"})
			}))
			defer server.Close()
			client := &http.Client{Transport: redirectTransport{http.DefaultTransport, strings.TrimPrefix(server.URL, "http://")}}
			s, err := fetchMonitor(context.Background(), client)
			if status == 503 && err == nil {
				t.Fatal("accepted unavailable snapshot")
			}
			if status == 200 && (err != nil || s.Build != "test") {
				t.Fatalf("%+v %v", s, err)
			}
		})
	}
}

func TestWatchRejectsRapidPollingAndUnknownArguments(t *testing.T) {
	for _, args := range [][]string{{"watch", "--interval", "10ms"}, {"watch", "--json"}, {"status", "extra"}} {
		if err := monitorCommand(context.Background(), args, &bytes.Buffer{}); err == nil {
			t.Fatalf("accepted %v", args)
		}
	}
}
