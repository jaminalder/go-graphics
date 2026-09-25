// Package persistence owns transactional studio storage and River integration.
package persistence

import (
	"context"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
	"github.com/riverqueue/river/rivermigrate"

	"github.com/jaminalder/go-graphics/internal/limits"
)

//go:embed schema.sql
var schema string

//go:embed monitor-schema.sql
var monitorSchema string

func applicationMigrations() []string { return []string{schema, monitorSchema} }

// ErrUnavailable keeps backend diagnostics out of public responses.
var ErrUnavailable = errors.New("the studio storage is temporarily unavailable; please try again")

// Lifetime is the sliding anonymous visitor lifetime.
const Lifetime = 90 * 24 * time.Hour

// DB contains bounded connections and an insert-only River client.
type DB struct {
	Pool     *pgxpool.Pool
	River    *river.Client[pgx.Tx]
	Build    string
	Instance Instance
	Limits   limits.Policy
}

// TokenHash converts a bearer cookie into a non-secret database identifier.
func TokenHash(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// Queue identifies a release's renderer queue using a bounded name.
func Queue(build string) string { return "render_" + TokenHash(build)[:24] }

// Open creates explicit database connections; migrations are a separate operation.
func Open(ctx context.Context, url, build string) (*DB, error) {
	policy, err := limits.FromEnv()
	if err != nil {
		return nil, err
	}
	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		return nil, errors.New("invalid database configuration")
	}
	cfg.MaxConns = 8
	cfg.ConnConfig.ConnectTimeout = 5 * time.Second
	cfg.ConnConfig.RuntimeParams["statement_timeout"] = "10000"
	cfg.ConnConfig.RuntimeParams["lock_timeout"] = "5000"
	cfg.ConnConfig.RuntimeParams["idle_in_transaction_session_timeout"] = "15000"
	cfg.ConnConfig.RuntimeParams["application_name"] = "singular-seed"
	p, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, err
	}
	if err := p.Ping(ctx); err != nil {
		p.Close()
		return nil, errors.New("database connection failed")
	}
	r, err := river.NewClient(riverpgxv5.New(p), &river.Config{})
	if err != nil {
		p.Close()
		return nil, err
	}
	return &DB{Pool: p, River: r, Build: build, Limits: policy}, nil
}

// OpenEnv loads the connection secret from a file, not a process argument.
func OpenEnv(ctx context.Context, build string) (*DB, error) {
	b, err := os.ReadFile(os.Getenv("ART_DATABASE_URL_FILE"))
	if err != nil {
		return nil, errors.New("read ART_DATABASE_URL_FILE")
	}
	return Open(ctx, strings.TrimSpace(string(b)), build)
}

// Close releases the query pool; callers stop workers/listeners first.
func (d *DB) Close() { d.Pool.Close() }

// Rollback releases an unfinished transaction even if its request context expired.
// ErrTxClosed after successful commit is expected; connection failures are handled by pgx.
func Rollback(tx pgx.Tx) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = tx.Rollback(ctx)
}

// Migrate serializes application and upstream River migrations across deployers.
func (d *DB) Migrate(ctx context.Context) error {
	c, err := d.Pool.Acquire(ctx)
	if err != nil {
		return err
	}
	defer c.Release()
	if _, err = c.Exec(ctx, "SELECT pg_advisory_lock(7192401)"); err != nil {
		return err
	}
	defer func() {
		cleanup, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if _, err := c.Exec(cleanup, "SELECT pg_advisory_unlock(7192401)"); err != nil {
			_ = c.Conn().Close(cleanup)
		}
	}()
	m, err := rivermigrate.New(riverpgxv5.New(d.Pool), nil)
	if err != nil {
		return err
	}
	if _, err = m.Migrate(ctx, rivermigrate.DirectionUp, nil); err != nil {
		return err
	}
	tx, err := c.Begin(ctx)
	if err != nil {
		return err
	}
	defer Rollback(tx)
	if _, err = tx.Exec(ctx, "CREATE TABLE IF NOT EXISTS art_schema(version integer PRIMARY KEY, checksum text NOT NULL)"); err != nil {
		return err
	}
	var latest int
	if err = tx.QueryRow(ctx, "SELECT coalesce(max(version),0) FROM art_schema").Scan(&latest); err != nil {
		return err
	}
	if latest > len(applicationMigrations()) {
		return errors.New("database schema is newer than this release")
	}
	for i, sql := range applicationMigrations() {
		var checksum string
		err = tx.QueryRow(ctx, "SELECT checksum FROM art_schema WHERE version=$1", i+1).Scan(&checksum)
		switch {
		case errors.Is(err, pgx.ErrNoRows):
			if latest >= i+1 {
				return errors.New("application migration history has a gap")
			}
			if _, err = tx.Exec(ctx, sql); err != nil {
				return err
			}
			if _, err = tx.Exec(ctx, "INSERT INTO art_schema VALUES($1,$2)", i+1, TokenHash(sql)); err != nil {
				return err
			}
		case err != nil:
			return err
		case checksum != TokenHash(sql):
			return errors.New("application migration checksum mismatch")
		}
	}
	return tx.Commit(ctx)
}

// CheckSchema rejects a different application or River migration version.
func (d *DB) CheckSchema(ctx context.Context) error {
	rows, err := d.Pool.Query(ctx, "SELECT version,checksum FROM art_schema ORDER BY version")
	if err != nil {
		return err
	}
	count := 0
	migrations := applicationMigrations()
	for rows.Next() {
		var version int
		var checksum string
		if err = rows.Scan(&version, &checksum); err != nil {
			rows.Close()
			return err
		}
		count++
		if version != count || count > len(migrations) || checksum != TokenHash(migrations[count-1]) {
			rows.Close()
			return errors.New("incompatible application schema")
		}
	}
	rows.Close()
	if err = rows.Err(); err != nil {
		return err
	}
	if count != len(migrations) {
		return errors.New("incompatible application schema")
	}
	m, err := rivermigrate.New(riverpgxv5.New(d.Pool), nil)
	if err != nil {
		return err
	}
	versions, err := m.ExistingVersions(ctx)
	if err != nil {
		return err
	}
	all := m.AllVersions()
	if len(versions) != len(all) {
		return errors.New("incompatible River schema")
	}
	for _, v := range versions {
		found := false
		for _, expected := range all {
			if expected.Version == v.Version && expected.Name == v.Name {
				found = true
				break
			}
		}
		if !found {
			return errors.New("incompatible River migration set")
		}
	}
	return nil
}

// Activate changes the release epoch only after the previous render jobs drained.
func (d *DB) Activate(ctx context.Context) error {
	tx, err := d.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer Rollback(tx)
	if _, err = tx.Exec(ctx, "SELECT singleton FROM art_control FOR UPDATE"); err != nil {
		return err
	}
	var n int
	if err = tx.QueryRow(ctx, `SELECT count(*) FROM river_job WHERE kind='render' AND state IN ('available','running','retryable','scheduled','pending')`).Scan(&n); err != nil {
		return err
	}
	if n != 0 {
		return fmt.Errorf("cannot activate with %d unfinished jobs", n)
	}
	_, err = tx.Exec(ctx, "UPDATE art_control SET build=$1,epoch=epoch+1 WHERE singleton", d.Build)
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// Enable controls admission globally without cancelling committed jobs.
func (d *DB) Enable(ctx context.Context, on bool) error {
	_, err := d.Pool.Exec(ctx, "UPDATE art_control SET enabled=$1 WHERE singleton", on)
	return err
}
