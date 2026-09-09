// Command artrender supervises one disposable renderer child over a private Unix socket.
package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jaminalder/go-graphics/internal/renderjob"
)

var build = "development"

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	if len(os.Args) == 2 && os.Args[1] == "--child" {
		return renderjob.Child(os.Stdin, os.Stdout, build)
	}
	socket := os.Getenv("ART_SOCKET")
	if socket == "" {
		socket = "out/artrender.sock"
	}
	if info, err := os.Lstat(socket); err == nil {
		if info.Mode()&os.ModeSocket == 0 {
			return errors.New("socket path is not a socket")
		}
		if c, err := net.DialTimeout("unix", socket, time.Second); err == nil {
			c.Close()
			return errors.New("renderer already running")
		}
		if err := os.Remove(socket); err != nil {
			return err
		}
	}
	l, err := net.Listen("unix", socket)
	if err != nil {
		return err
	}
	defer l.Close()
	if err := os.Chmod(socket, 0o660); err != nil {
		return err
	}
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	s := &renderjob.Supervisor{Executable: exe, Build: build}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	srv := &http.Server{BaseContext: func(net.Listener) context.Context { return ctx }, Handler: s.Handler(), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 5 * time.Second, WriteTimeout: 35 * time.Second, IdleTimeout: 15 * time.Second, MaxHeaderBytes: 4096}
	go func() {
		<-ctx.Done()
		closeCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(closeCtx)
	}()
	err = srv.Serve(l)
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}
