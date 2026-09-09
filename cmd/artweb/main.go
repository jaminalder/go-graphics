// Command artweb serves the public studio, with one isolated renderer and no database.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/netip"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jaminalder/go-graphics/internal/renderjob"
	"github.com/jaminalder/go-graphics/internal/studio"
	"github.com/jaminalder/go-graphics/internal/web"
)

var build = "development"

func main() {
	if err := run(); err != nil {
		slog.Error("server stopped", "error", err)
		os.Exit(1)
	}
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func run() error {
	addr := env("ART_ADDR", "127.0.0.1:8080")
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return err
	}
	ip, err := netip.ParseAddr(host)
	if err != nil || !ip.IsLoopback() {
		return errors.New("web listener must be loopback; use the configured proxy")
	}
	origin := env("ART_ORIGIN", "http://"+addr)
	client := renderjob.NewClient(env("ART_SOCKET", "out/artrender.sock"), build)
	jobs, err := renderjob.New(renderjob.Config{Directory: env("ART_CACHE", "out/cache"), Build: build, Renderer: client})
	if err != nil {
		return err
	}
	defer jobs.Close()
	jobs.Enable(os.Getenv("ART_GENERATION") != "off")
	handler, err := web.New(web.Config{Origin: origin, Studio: studio.New(jobs, build), Jobs: jobs, TrustProxy: os.Getenv("ART_TRUST_PROXY") == "yes"})
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	srv := &http.Server{Addr: addr, Handler: handler, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 30 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 16384, BaseContext: func(net.Listener) context.Context { return ctx }}
	admin := &http.Server{Addr: env("ART_ADMIN_ADDR", "127.0.0.1:8081"), ReadHeaderTimeout: 3 * time.Second, WriteTimeout: 5 * time.Second}
	adminHost, _, err := net.SplitHostPort(admin.Addr)
	if err != nil {
		return err
	}
	adminIP, err := netip.ParseAddr(adminHost)
	if err != nil || !adminIP.IsLoopback() {
		return errors.New("admin listener must be loopback")
	}
	mux := http.NewServeMux()
	mux.Handle("GET /metrics", web.Metrics(jobs))
	mux.HandleFunc("POST /generation/off", func(w http.ResponseWriter, _ *http.Request) { jobs.Enable(false); w.WriteHeader(204) })
	mux.HandleFunc("POST /generation/on", func(w http.ResponseWriter, _ *http.Request) { jobs.Enable(true); w.WriteHeader(204) })
	mux.HandleFunc("GET /ready", func(w http.ResponseWriter, r *http.Request) {
		check, cancel := context.WithTimeout(r.Context(), time.Second)
		defer cancel()
		if !client.Health(check) {
			http.Error(w, "renderer unavailable", http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(204)
	})
	admin.Handler = mux
	go func() {
		if err := admin.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("admin listener failed", "error", err)
			stop()
		}
	}()
	shutdownDone := make(chan struct{})
	go func() {
		defer close(shutdownDone)
		<-ctx.Done()
		jobs.Enable(false)
		closeCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(closeCtx)
		_ = admin.Shutdown(closeCtx)
	}()
	slog.Info("studio listening", "address", addr, "build", build)
	err = srv.ListenAndServe()
	if errors.Is(err, http.ErrServerClosed) {
		<-shutdownDone
		return nil
	}
	return fmt.Errorf("serve: %w", err)
}
