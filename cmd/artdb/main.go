// Command artdb explicitly migrates and controls the persistent deployment.
package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/jaminalder/go-graphics/internal/objectstore"
	"github.com/jaminalder/go-graphics/internal/persistence"
)

var build = "development"

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	if len(os.Args) != 2 {
		return errors.New("usage: artdb migrate|status|activate|bind-bucket|create-local-bucket|enable|disable|pause|resume")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	db, err := persistence.OpenEnv(ctx, build)
	if err != nil {
		return err
	}
	defer db.Close()
	switch os.Args[1] {
	case "migrate":
		return db.Migrate(ctx)
	case "status":
		return db.CheckSchema(ctx)
	case "admission":
		var enabled bool
		if err := db.Pool.QueryRow(ctx, "SELECT enabled FROM art_control").Scan(&enabled); err != nil {
			return err
		}
		if enabled {
			fmt.Println("on")
		} else {
			fmt.Println("off")
		}
		return nil
	case "activate":
		return db.Activate(ctx)
	case "enable":
		return db.Enable(ctx, true)
	case "disable":
		return db.Enable(ctx, false)
	case "pause":
		return db.River.QueuePause(ctx, persistence.Queue(build), nil)
	case "resume":
		return db.River.QueueResume(ctx, persistence.Queue(build), nil)
	case "bind-bucket":
		s, e := objectstore.FromEnv()
		if e != nil {
			return e
		}
		return db.BindEmptyBucket(ctx, s)
	case "create-local-bucket":
		if os.Getenv("ART_S3_LOCAL") != "true" {
			return errors.New("local bucket bootstrap requires ART_S3_LOCAL=true")
		}
		s, e := objectstore.FromEnv()
		if e != nil {
			return e
		}
		return s.CreateBucket(ctx)
	default:
		return errors.New("unknown database operation")
	}
}
