package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"text/tabwriter"
	"time"
	"unicode"

	"github.com/jaminalder/go-graphics/internal/persistence"
)

func monitorCommand(ctx context.Context, args []string, out io.Writer) error {
	flags := flag.NewFlagSet(args[0], flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	interval := flags.Duration("interval", 2*time.Second, "refresh interval")
	asJSON := flags.Bool("json", false, "output a JSON snapshot")
	if err := flags.Parse(args[1:]); err != nil {
		return err
	}
	if flags.NArg() != 0 || *interval < time.Second || *interval > time.Minute || (*asJSON && args[0] == "watch") {
		return errors.New("use status [--json] or watch [--interval 1s..1m]")
	}
	client := &http.Client{Transport: &http.Transport{Proxy: nil}, Timeout: 4 * time.Second}
	defer client.CloseIdleConnections()
	terminal := false
	if file, ok := out.(*os.File); ok {
		if info, err := file.Stat(); err == nil {
			terminal = info.Mode()&os.ModeCharDevice != 0 && os.Getenv("TERM") != "dumb"
		}
	}
	for {
		s, err := fetchMonitor(ctx, client)
		if ctx.Err() != nil {
			return nil
		}
		if args[0] == "status" {
			if err != nil {
				return err
			}
			if *asJSON {
				return json.NewEncoder(out).Encode(s)
			}
			return renderMonitor(out, s)
		}
		if terminal {
			if _, e := io.WriteString(out, "\033[H\033[2J"); e != nil {
				return e
			}
		}
		if err != nil {
			if _, e := fmt.Fprintf(out, "MONITOR UNAVAILABLE — %s (retrying; no stale data shown)\n", clean(err.Error())); e != nil {
				return e
			}
		} else if err = renderMonitor(out, s); err != nil {
			return err
		}
		timer := time.NewTimer(*interval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil
		case <-timer.C:
		}
	}
}

func fetchMonitor(ctx context.Context, client *http.Client) (persistence.MonitorSnapshot, error) {
	var snapshot persistence.MonitorSnapshot
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://127.0.0.1:8081/monitor", nil)
	if err != nil {
		return snapshot, err
	}
	res, err := client.Do(req)
	if err != nil {
		return snapshot, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return snapshot, fmt.Errorf("monitor: HTTP %d", res.StatusCode)
	}
	err = json.NewDecoder(io.LimitReader(res.Body, 1<<20)).Decode(&snapshot)
	return snapshot, err
}

func clean(s string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return ' '
		}
		return r
	}, s)
}

func short(s string, n int) string {
	r := []rune(clean(s))
	if len(r) > n {
		return string(r[:n]) + "…"
	}
	return string(r)
}

func boot(id string) string {
	id = clean(id)
	if len(id) > 8 {
		return id[len(id)-8:]
	}
	return id
}

func duration(seconds float64) string {
	if seconds < 0 {
		seconds = 0
	}
	return (time.Duration(seconds) * time.Second).String()
}

func renderMonitor(out io.Writer, s persistence.MonitorSnapshot) error {
	// Supplied by the host helper after docker inspect, never by the application itself.
	var names map[string]string
	if raw := os.Getenv("ART_MONITOR_NAMES"); raw != "" {
		if err := json.Unmarshal([]byte(raw), &names); err != nil {
			return errors.New("invalid monitor container-name map")
		}
	}
	resolve := func(i persistence.Instance) persistence.Instance {
		if i.Name == i.Hostname {
			if name := names[i.Hostname]; name != "" {
				i.Name = name
			}
		}
		return i
	}
	b := &strings.Builder{}
	fmt.Fprintf(b, "RENDER MONITOR  %s UTC  build=%s  admission=%t\n", s.At.UTC().Format("15:04:05"), short(s.Build, 12), s.Enabled)
	fmt.Fprintf(b, "POLICY  %s  outstanding=%d  visitor=%d  queue-age=%ds\n", clean(s.Limits.Profile), s.Limits.Outstanding, s.Limits.VisitorJobs, s.Limits.QueueAgeSeconds)
	fmt.Fprintf(b, "QUEUE  waiting=%d  running=%d  retrying=%d  failed(1h)=%d  completed(1h)=%d  oldest-wait=%s\n\n", s.Queue.Waiting, s.Queue.Running, s.Queue.Retrying, s.Queue.Failed, s.Queue.Completed, duration(s.Queue.OldestSeconds))
	fmt.Fprintln(b, "INSTANCES (last hour + owners of running jobs)")
	t := tabwriter.NewWriter(b, 0, 4, 2, ' ', 0)
	fmt.Fprintln(t, "INSTANCE\tROLE\tBOOT\tBUILD\tSTATE\tSEEN\tSTORAGE\tRUNNING\tDONE(1h)")
	for _, i := range s.Instances {
		i.Instance = resolve(i.Instance)
		storage := "—"
		if i.StorageOK != nil {
			storage = "ok"
			if !*i.StorageOK {
				storage = "error"
			}
		}
		fmt.Fprintf(t, "%s\t%s\t%s\t%s\t%s\t%s\t%s\t%d\t%d\n", short(i.Name, 80), i.Role, boot(i.ID), short(i.Build, 12), i.Status, duration(i.AgeSeconds), storage, i.Running, i.Completed)
	}
	if err := t.Flush(); err != nil {
		return err
	}
	if s.InstancesTruncated {
		fmt.Fprintln(b, "... limited to 200 instance boots")
	}
	fmt.Fprintln(b, "\nWORK FLOW (all outstanding + finalized in last hour)")
	t = tabwriter.NewWriter(b, 0, 4, 2, ' ', 0)
	fmt.Fprintln(t, "PRODUCER / BOOT\tLAST RENDERER / BOOT\tSTATE\tJOBS")
	label := func(i persistence.Instance, missing string) string {
		if i.ID == "" {
			return missing
		}
		i = resolve(i)
		return short(i.Name, 80) + " / " + boot(i.ID)
	}
	for _, f := range s.Flow {
		fmt.Fprintf(t, "%s\t%s\t%s\t%d\n", label(f.Producer, "(legacy/unknown)"), label(f.Renderer, "(not claimed)"), clean(f.State), f.Jobs)
	}
	if err := t.Flush(); err != nil {
		return err
	}
	if s.FlowTruncated {
		fmt.Fprintln(b, "... limited to 200 flow groups")
	}
	fmt.Fprintln(b, "\nCounts are jobs, not HTTP requests. Last claimant is not proof of liveness. Ctrl-C stops watching.")
	_, err := io.WriteString(out, b.String())
	return err
}
