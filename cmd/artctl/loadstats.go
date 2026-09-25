package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

func loadStatsCommand(args []string, out io.Writer) error {
	f := flag.NewFlagSet("load-stats", flag.ContinueOnError)
	f.SetOutput(io.Discard)
	since := f.String("since", "", "RFC3339 timestamp")
	until := f.String("until", "", "RFC3339 timestamp")
	if err := f.Parse(args); err != nil {
		return err
	}
	a, e1 := time.Parse(time.RFC3339Nano, *since)
	b, e2 := time.Parse(time.RFC3339Nano, *until)
	if f.NArg() != 0 || e1 != nil || e2 != nil || !b.After(a) || b.Sub(a) > 25*time.Hour {
		return errors.New("load-stats requires --since and --until RFC3339 timestamps within 25 hours")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://127.0.0.1:8081/load-stats?"+url.Values{"since": {*since}, "until": {*until}}.Encode(), nil)
	if err != nil {
		return err
	}
	client := &http.Client{Transport: &http.Transport{Proxy: nil}}
	defer client.CloseIdleConnections()
	res, err := client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("load statistics HTTP %d", res.StatusCode)
	}
	_, err = io.Copy(out, io.LimitReader(res.Body, 1<<20))
	return err
}
