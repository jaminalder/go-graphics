package persistence

import (
	"context"
	"fmt"
	"os"
	"time"
)

// Instance identifies a process boot independently of container naming/restarts.
type Instance struct {
	ID       string `json:"id"`
	Role     string `json:"role"`
	Hostname string `json:"hostname"`
	Name     string `json:"name"`
	Build    string `json:"build"`
}

// Identify configures attribution before a process starts admitting or consuming work.
func (d *DB) Identify(role string) error {
	if role != "web" && role != "renderer" {
		return fmt.Errorf("invalid instance role %q", role)
	}
	host, err := os.Hostname()
	if err != nil {
		return err
	}
	name := os.Getenv("ART_INSTANCE_NAME")
	if name == "" {
		name = host
	}
	if len(name) > 200 || len(host) > 200 {
		return fmt.Errorf("instance name or hostname exceeds 200 bytes")
	}
	d.Instance = Instance{ID: role + "-" + executionID(), Role: role, Hostname: host, Name: name, Build: d.Build}
	return nil
}

// Heartbeat records presence; renderer uses the same boot ID as its River client.
func (d *DB) Heartbeat(ctx context.Context, storage *bool) error {
	i := d.Instance
	if i.ID == "" {
		return fmt.Errorf("instance identity not configured")
	}
	_, err := d.Pool.Exec(ctx, `INSERT INTO art_instances(id,role,hostname,name,build,storage_ok) VALUES($1,$2,$3,$4,$5,$6)
      ON CONFLICT(id) DO UPDATE SET touched=now(),storage_ok=excluded.storage_ok,stopped=NULL`, i.ID, i.Role, i.Hostname, i.Name, i.Build, storage)
	return err
}

// StopInstance preserves recent attribution while marking graceful process removal.
func (d *DB) StopInstance(ctx context.Context) error {
	_, err := d.Pool.Exec(ctx, "UPDATE art_instances SET stopped=now() WHERE id=$1", d.Instance.ID)
	return err
}

// TrackWeb refreshes presence every 15s and marks shutdown before its pool is closed.
func (d *DB) TrackWeb(ctx context.Context) {
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()
	defer func() {
		c, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = d.StopInstance(c)
	}()
	for {
		c, cancel := context.WithTimeout(ctx, 3*time.Second)
		_ = d.Heartbeat(c, nil)
		cancel()
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
