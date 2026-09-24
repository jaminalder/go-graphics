// Command artrender consumes River jobs, supervises children and publishes images.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/jaminalder/go-graphics/internal/logging"
	"github.com/jaminalder/go-graphics/internal/objectstore"
	"github.com/jaminalder/go-graphics/internal/persistence"
	"github.com/jaminalder/go-graphics/internal/renderjob"
	"github.com/jaminalder/go-graphics/internal/studio"
)

var build = "development"

func main() {
	logger, err := logging.New("artrender", os.Getenv("ART_LOG_LEVEL"), os.Stderr)
	if err != nil {
		panic(err)
	}
	slog.SetDefault(logger)
	if err = run(); err != nil {
		slog.Error("renderer stopped", "error", err)
		os.Exit(1)
	}
}

func run() error {
	if len(os.Args) == 2 && os.Args[1] == "--child" {
		return renderjob.Child(os.Stdin, os.Stdout, build)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	db, err := persistence.OpenEnv(ctx, build)
	if err != nil {
		return err
	}
	defer db.Close()
	if err = db.CheckSchema(ctx); err != nil {
		return err
	}
	objects, err := objectstore.FromEnv()
	if err != nil {
		return err
	}
	if err = db.CheckBucket(ctx, objects); err != nil {
		return err
	}
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	client, err := persistence.NewWorkerClient(db, objects, &renderjob.Supervisor{Executable: exe, Build: build})
	if err != nil {
		return err
	}
	if err = client.Start(context.Background()); err != nil {
		return err
	}
	id := studio.Token()
	var ready atomic.Bool
	done := make(chan struct{})
	go func() {
		defer close(done)
		ticker := time.NewTicker(15 * time.Second)
		defer ticker.Stop()
		for {
			check, cancel := context.WithTimeout(ctx, 5*time.Second)
			ok := objects.Health(check) == nil
			_, e := db.Pool.Exec(check, "INSERT INTO art_renderers(id,build,touched,storage_ok) VALUES($1,$2,now(),$3) ON CONFLICT(id) DO UPDATE SET touched=now(),storage_ok=excluded.storage_ok", id, build, ok)
			ready.Store(ok && e == nil)
			cancel()
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
		}
	}()
	mux := http.NewServeMux()
	mux.HandleFunc("GET /live", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(204) })
	mux.HandleFunc("GET /ready", func(w http.ResponseWriter, _ *http.Request) {
		if !ready.Load() {
			http.Error(w, "unavailable", http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(204)
	})
	srv := &http.Server{Addr: "127.0.0.1:8082", Handler: mux, ReadHeaderTimeout: 3 * time.Second, WriteTimeout: 5 * time.Second}
	go func() {
		if e := srv.ListenAndServe(); e != nil && !errors.Is(e, http.ErrServerClosed) {
			slog.Error("renderer health listener", "error", e)
			stop()
		}
	}()
	<-ctx.Done()
	<-done
	shutdown, cancel := context.WithTimeout(context.Background(), 95*time.Second)
	defer cancel()
	if err = client.Stop(shutdown); err != nil {
		force, finish := context.WithTimeout(context.Background(), 5*time.Second)
		defer finish()
		_ = client.StopAndCancel(force)
	}
	_ = srv.Shutdown(shutdown)
	_, _ = db.Pool.Exec(shutdown, "DELETE FROM art_renderers WHERE id=$1", id)
	return nil
}
