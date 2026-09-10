// Command artctl probes and controls private services from inside their containers.
package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/jaminalder/go-graphics/internal/renderjob"
)

var build = "development"

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) != 1 {
		return errors.New("usage: artctl live|ready|renderer-ready|metrics|generation-off|generation-on")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if args[0] == "renderer-ready" {
		if !renderjob.NewClient(os.Getenv("ART_SOCKET"), build).Health(ctx) {
			return errors.New("renderer unavailable")
		}
		return nil
	}
	method, endpoint := "GET", "http://127.0.0.1:8081/"
	switch args[0] {
	case "live":
		endpoint = "http://127.0.0.1:8080/health/live"
	case "ready", "metrics":
		endpoint += args[0]
	case "generation-off":
		method, endpoint = "POST", endpoint+"generation/off"
	case "generation-on":
		method, endpoint = "POST", endpoint+"generation/on"
	default:
		return errors.New("unknown private operation")
	}
	req, err := http.NewRequestWithContext(ctx, method, endpoint, nil)
	if err != nil {
		return err
	}
	if args[0] == "live" {
		origin, err := url.Parse(os.Getenv("ART_ORIGIN"))
		if err != nil || origin.Host == "" {
			return errors.New("ART_ORIGIN is required")
		}
		req.Host = origin.Host
	}
	client := &http.Client{Transport: &http.Transport{Proxy: nil}, Timeout: 3 * time.Second}
	defer client.CloseIdleConnections()
	res, err := client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != 200 && res.StatusCode != 204 {
		return fmt.Errorf("%s: HTTP %d", args[0], res.StatusCode)
	}
	_, err = io.Copy(os.Stdout, io.LimitReader(res.Body, 65536))
	return err
}
