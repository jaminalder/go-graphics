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

	"github.com/jaminalder/go-graphics/internal/logging"
	"github.com/jaminalder/go-graphics/internal/renderjob"
	"github.com/jaminalder/go-graphics/internal/studio"
	"github.com/jaminalder/go-graphics/internal/web"
)

var build = "development"

func main() {
	logger, err := logging.New("artweb", os.Getenv("ART_LOG_LEVEL"), os.Stderr)
	if err != nil {
		slog.Error("logging configuration failed", "error", err)
		os.Exit(1)
	}
	slog.SetDefault(logger)
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
	proxy, err := listenerProxy(addr, os.Getenv("ART_TRUSTED_PROXY"))
	if err != nil {
		return err
	}
	origin := env("ART_ORIGIN", "http://"+addr)
	client := renderjob.NewClient(env("ART_SOCKET", "out/artrender.sock"), build)
	jobs, err := renderjob.New(renderjob.Config{Directory: env("ART_CACHE", "out/cache"), Build: build, Renderer: client})
	if err != nil {
		return err
	}
	defer jobs.Close()
	jobs.Enable(os.Getenv("ART_GENERATION") != "off")
	handler, err := web.New(web.Config{Origin: origin, Studio: studio.New(jobs, build), Jobs: jobs, TrustedProxy: proxy})
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	srv := &http.Server{Addr: addr, Handler: logging.HTTP(slog.Default(), handler), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 30 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 16384, BaseContext: func(net.Listener) context.Context { return ctx }}
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
	admin.Handler = logging.HTTP(slog.Default(), mux)
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

// listenerProxy allows a container listener only with one explicit proxy address.
// Admin HTTP remains loopback-only regardless of this setting.
func listenerProxy(addr, trusted string) (netip.Addr, error) {
	var proxy netip.Addr
	if trusted != "" {
		var err error
		proxy, err = netip.ParseAddr(trusted)
		if err != nil || proxy.IsUnspecified() || proxy.IsMulticast() || proxy.Zone() != "" {
			return netip.Addr{}, errors.New("ART_TRUSTED_PROXY must be one unicast IP address")
		}
		proxy = proxy.Unmap()
	}
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return netip.Addr{}, err
	}
	ip, err := netip.ParseAddr(host)
	if err != nil || (!ip.IsLoopback() && !proxy.IsValid()) {
		return netip.Addr{}, errors.New("web listener must be loopback unless ART_TRUSTED_PROXY is set")
	}
	return proxy, nil
}
