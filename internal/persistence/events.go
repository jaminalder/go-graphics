package persistence

import (
	"context"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
)

// Events fans one PostgreSQL listener out to bounded local subscribers.
// Events carry no authority or durable state; every receiver rereads its own snapshot.
type Events struct {
	mu   sync.Mutex
	subs map[chan struct{}]struct{}
}

// NewEvents constructs a local broadcast hub.
func NewEvents() *Events { return &Events{subs: map[chan struct{}]struct{}{}} }

// Subscribe registers before the caller reads its first snapshot to avoid attachment races.
func (e *Events) Subscribe() (chan struct{}, func()) {
	e.mu.Lock()
	defer e.mu.Unlock()
	ch := make(chan struct{}, 1)
	e.subs[ch] = struct{}{}
	return ch, func() { e.mu.Lock(); delete(e.subs, ch); e.mu.Unlock() }
}

func (e *Events) broadcast() {
	e.mu.Lock()
	defer e.mu.Unlock()
	for ch := range e.subs {
		select {
		case ch <- struct{}{}:
		default:
		}
	}
}

// Run reconnects with LISTEN-before-resync; its connection is separate from query pools.
func (e *Events) Run(ctx context.Context, d *DB) {
	for ctx.Err() == nil {
		c, err := pgx.ConnectConfig(ctx, d.Pool.Config().ConnConfig.Copy())
		if err == nil {
			_, err = c.Exec(ctx, "LISTEN art_results")
			if err == nil {
				e.broadcast()
				for ctx.Err() == nil {
					if _, err = c.WaitForNotification(ctx); err != nil {
						break
					}
					e.broadcast()
				}
			}
			closeCtx, cancel := context.WithTimeout(context.Background(), time.Second)
			_ = c.Close(closeCtx)
			cancel()
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(time.Second):
		}
	}
}
