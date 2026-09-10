// Command artbench measures representative public classes using isolated renderer children.
package main

import (
	"bytes"
	"context"
	"debug/buildinfo"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"sort"
	"strconv"
	"syscall"
	"time"

	"github.com/jaminalder/go-graphics/internal/explore"
	"github.com/jaminalder/go-graphics/internal/publish"
	"github.com/jaminalder/go-graphics/internal/renderjob"
)

type measurement struct {
	Artwork string  `json:"artwork"`
	Style   string  `json:"style"`
	Seed    string  `json:"seed"`
	Tier    string  `json:"tier"`
	WallMS  float64 `json:"wall_ms"`
	CPUMS   float64 `json:"cpu_ms"`
	RSSKiB  int64   `json:"rss_kib"`
	Bytes   int     `json:"bytes"`
	Error   string  `json:"error,omitempty"`
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	renderer := flag.String("renderer", "out/artrender", "fixed renderer binary")
	build := flag.String("build", "development", "renderer release identity")
	count := flag.Int("count", 100, "representative seeds per public style")
	tier := flag.String("tier", "preview", "preview or download")
	flag.Parse()
	if *count < 1 || *count > 1000 {
		return fmt.Errorf("count must be 1..1000")
	}
	rendererInfo, err := buildinfo.ReadFile(*renderer)
	if err != nil {
		return err
	}
	enc := json.NewEncoder(os.Stdout)
	if err := enc.Encode(map[string]any{"go": runtime.Version(), "renderer_go": rendererInfo.GoVersion, "os": runtime.GOOS, "arch": runtime.GOARCH, "cpus": runtime.NumCPU(), "gomaxprocs": runtime.GOMAXPROCS(0), "build": *build, "count_per_class": *count, "cache": "cold isolated child; no artifact cache", "tier": *tier}); err != nil {
		return err
	}
	for _, entry := range publish.All() {
		space, err := publish.Space(entry.ID)
		if err != nil {
			return err
		}
		for _, style := range entry.Styles {
			times := make([]float64, 0, *count)
			failures := 0
			for i := 1; i <= *count; i++ {
				pins, pal, err := publish.Pins(entry.ID, style.ID, entry.Colours[i%len(entry.Colours)].ID)
				if err != nil {
					return err
				}
				candidates, err := explore.Public(space, pins, nil, uint64(i), 0, nil)
				if err != nil {
					return err
				}
				r, err := publish.Complete(entry.ID, uint64(i), pal, candidates[0].Traits)
				if err != nil {
					return err
				}
				q := renderjob.Request{Version: 1, Build: *build, Recipe: r.Bytes(), Tier: *tier}
				if err := q.Validate(*build); err != nil {
					return err
				}
				input, err := json.Marshal(q)
				if err != nil {
					return err
				}
				ctx, cancel := context.WithTimeout(context.Background(), 35*time.Second)
				cmd := exec.CommandContext(ctx, *renderer, "--child")
				cmd.WaitDelay = time.Second
				cmd.Stdin = bytes.NewReader(input)
				var output bytes.Buffer
				cmd.Stdout = &output
				cmd.Stderr = os.Stderr
				start := time.Now()
				err = cmd.Run()
				elapsed := time.Since(start)
				cancel()
				m := measurement{Artwork: entry.ID, Style: style.ID, Seed: strconv.Itoa(i), Tier: *tier, WallMS: float64(elapsed.Microseconds()) / 1000, Bytes: output.Len()}
				if err != nil {
					m.Error = err.Error()
					failures++
				}
				if cmd.ProcessState != nil {
					m.CPUMS = float64((cmd.ProcessState.UserTime() + cmd.ProcessState.SystemTime()).Microseconds()) / 1000
					if usage, ok := cmd.ProcessState.SysUsage().(*syscall.Rusage); ok {
						m.RSSKiB = usage.Maxrss
						if runtime.GOOS == "darwin" {
							m.RSSKiB /= 1024
						}
					}
				}
				times = append(times, m.WallMS)
				if err := enc.Encode(m); err != nil {
					return err
				}
			}
			sort.Float64s(times)
			percentile := func(p float64) float64 { return times[min(int(float64(len(times))*p), len(times)-1)] }
			if err := enc.Encode(map[string]any{"class": entry.ID + "/" + style.ID, "failures": failures, "p50_ms": percentile(.50), "p95_ms": percentile(.95), "p99_ms": percentile(.99)}); err != nil {
				return err
			}
		}
	}
	return nil
}
