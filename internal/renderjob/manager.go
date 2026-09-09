package renderjob

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"image/png"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/jaminalder/go-graphics/internal/publish"
)

// ErrBusy indicates finite global capacity; a caller may explicitly try later.
var ErrBusy = errors.New("generation is busy; try again shortly")

// Config fixes process-wide admission and cache limits.
type (
	Config struct {
		Directory, Build string
		Renderer         Renderer
		Queue            int
		CacheBytes       int64
		CacheCount       int
		MaxAge           time.Duration
	}
	// Status is a copied job snapshot. Failed work is never retried by a read.
	Status struct{ ID, State, Message string }
	job    struct {
		request     Request
		state       string
		subscribers map[string]bool
		owner       string
		created     time.Time
		cancel      context.CancelFunc
	}
	artifact struct {
		size    int64
		created time.Time
		digest  string
		refs    int
	}
	// Manager owns the only queue and every artifact publication transition.
	Manager struct {
		mu        sync.Mutex
		cfg       Config
		jobs      map[string]*job
		queue     []string
		artifacts map[string]artifact
		bytes     int64
		wake      chan struct{}
		ctx       context.Context
		cancel    context.CancelFunc
		done      chan struct{}
		enabled   bool
		open      int
		last      string
	}
)

// New reconciles the private cache and starts one serial worker.
func New(c Config) (*Manager, error) {
	if c.Renderer == nil || c.Build == "" || c.Directory == "" {
		return nil, errors.New("missing renderer configuration")
	}
	if c.Queue == 0 {
		c.Queue = 8
	}
	if c.CacheBytes == 0 {
		c.CacheBytes = 2 << 30
	}
	if c.CacheCount == 0 {
		c.CacheCount = 5000
	}
	if c.MaxAge == 0 {
		c.MaxAge = 24 * time.Hour
	}
	if c.Queue < 1 || c.Queue > 8 || c.CacheCount < 1 || c.CacheCount > 5000 || c.CacheBytes < MaxImage || c.MaxAge < 0 {
		return nil, errors.New("invalid capacity")
	}
	if err := os.MkdirAll(c.Directory, 0o700); err != nil {
		return nil, err
	}
	ctx, cancel := context.WithCancel(context.Background())
	m := &Manager{cfg: c, jobs: map[string]*job{}, artifacts: map[string]artifact{}, wake: make(chan struct{}, 1), ctx: ctx, cancel: cancel, done: make(chan struct{}), enabled: true}
	if err := m.reconcile(); err != nil {
		cancel()
		return nil, err
	}
	go m.work()
	return m, nil
}

// Close cancels active work, waits for termination and releases every reservation.
func (m *Manager) Close() { m.cancel(); <-m.done }

// Enable changes admission without disrupting browsing or completed downloads.
func (m *Manager) Enable(on bool) { m.mu.Lock(); defer m.mu.Unlock(); m.enabled = on }

// Admit reserves a complete batch atomically, coalescing cached and in-flight identities.
func (m *Manager) Admit(owner string, qs []Request) ([]string, error) {
	if len(owner) < 1 || len(owner) > 64 || len(qs) < 1 || len(qs) > 4 {
		return nil, errors.New("invalid batch")
	}
	ids := make([]string, len(qs))
	for i, q := range qs {
		if err := q.Validate(m.cfg.Build); err != nil {
			return nil, err
		}
		r, err := publish.Validate(q.Recipe)
		if err != nil {
			return nil, err
		}
		tier, err := publish.Tier(r.ID(), q.Tier)
		if err != nil {
			return nil, err
		}
		ids[i] = r.Key(tier, m.cfg.Build)
		qs[i].Recipe = append([]byte(nil), q.Recipe...)
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.prune()
	filtered := m.queue[:0]
	for _, id := range m.queue {
		if j := m.jobs[id]; j != nil && j.state == "queued" {
			filtered = append(filtered, id)
		}
	}
	m.queue = filtered
	if !m.enabled || m.ctx.Err() != nil {
		return nil, ErrBusy
	}
	fresh := map[string]bool{}
	queued := 0
	owned := 0
	added := map[string]bool{}
	for _, j := range m.jobs {
		if j.state == "running" || j.state == "queued" {
			if j.state == "queued" {
				queued++
			}
			if j.subscribers[owner] {
				owned++
			}
		}
	}
	for _, id := range ids {
		j := m.jobs[id]
		if j != nil && j.state == "ready" {
			if _, cached := m.artifacts[id]; cached && len(j.subscribers) >= 24 && !j.subscribers[owner] {
				return nil, ErrBusy
			}
		}
		if j != nil && (j.state == "running" || j.state == "queued") {
			if !j.subscribers[owner] {
				added[id] = true
			}
			if len(j.subscribers) >= 24 && !j.subscribers[owner] {
				return nil, ErrBusy
			}
			continue
		}
		if _, ok := m.artifacts[id]; !ok {
			fresh[id] = true
		}
	}
	if queued+len(fresh) > m.cfg.Queue || owned+len(fresh)+len(added) > 4 || len(m.jobs)+len(ids) > 5000 {
		return nil, ErrBusy
	}
	for i, id := range ids {
		if j := m.jobs[id]; j != nil && (j.state == "running" || j.state == "queued" || j.state == "ready") {
			if _, cached := m.artifacts[id]; j.state != "ready" || cached {
				if len(j.subscribers) >= 24 && !j.subscribers[owner] {
					return nil, ErrBusy
				}
				j.subscribers[owner] = true
				continue
			}
		}
		state := "queued"
		if _, ok := m.artifacts[id]; ok {
			state = "ready"
		}
		j := &job{request: qs[i], state: state, subscribers: map[string]bool{owner: true}, owner: owner, created: time.Now()}
		m.jobs[id] = j
		if state == "queued" {
			m.queue = append(m.queue, id)
		}
	}
	select {
	case m.wake <- struct{}{}:
	default:
	}
	return ids, nil
}

// Status returns only the requesting workspace's job, with no allocation or work creation.
func (m *Manager) Status(owner, id string) Status {
	m.mu.Lock()
	defer m.mu.Unlock()
	j := m.jobs[id]
	if j == nil || !j.subscribers[owner] {
		return Status{ID: id, State: "expired", Message: "This sample is no longer available. Generate it again."}
	}
	state := j.state
	if state == "ready" {
		if a, ok := m.artifacts[id]; !ok || time.Since(a.created) > m.cfg.MaxAge {
			state = "expired"
		}
	}
	message := ""
	if state == "failed" {
		message = "This sample could not finish. Try again when you are ready."
	}
	return Status{ID: id, State: state, Message: message}
}

// Cancel removes one subscriber and stops shared work only when none remain.
func (m *Manager) Cancel(owner, id string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	j := m.jobs[id]
	if j == nil {
		return
	}
	delete(j.subscribers, owner)
	if len(j.subscribers) == 0 && (j.state == "queued" || j.state == "running") {
		j.state = "cancelled"
		if j.cancel != nil {
			j.cancel()
		}
	}
}

// Open serves existing bytes from an open handle that survives Unix cache eviction.
func (m *Manager) Open(id string) (*Lease, string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	a, ok := m.artifacts[id]
	if !ok || time.Since(a.created) > m.cfg.MaxAge {
		return nil, "", os.ErrNotExist
	}
	f, err := os.Open(filepath.Join(m.cfg.Directory, id+".png"))
	if err != nil {
		return nil, "", err
	}
	if a.refs >= 32 || m.open >= 64 {
		f.Close()
		return nil, "", ErrBusy
	}
	a.refs++
	m.open++
	m.artifacts[id] = a
	return &Lease{File: f, release: func() {
		m.mu.Lock()
		defer m.mu.Unlock()
		v := m.artifacts[id]
		v.refs--
		m.open--
		m.artifacts[id] = v
	}}, a.digest, nil
}

// Counts exposes low-cardinality operational counters.
func (m *Manager) Counts() (queued, running, files int, size int64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, j := range m.jobs {
		if j.state == "queued" {
			queued++
		}
		if j.state == "running" {
			running++
		}
	}
	return queued, running, len(m.artifacts), m.bytes
}

func (m *Manager) work() {
	defer close(m.done)
	maintenance := time.NewTicker(min(time.Minute, m.cfg.MaxAge))
	defer maintenance.Stop()
	for {
		select {
		case <-m.ctx.Done():
			return
		case <-m.wake:
		case <-maintenance.C:
			m.mu.Lock()
			m.prune()
			m.mu.Unlock()
			continue
		}
		for {
			m.mu.Lock()
			index := -1
			for i, id := range m.queue {
				j := m.jobs[id]
				if j == nil || j.state != "queued" {
					continue
				}
				if time.Since(j.created) > time.Minute {
					j.state = "expired"
					continue
				}
				if index < 0 || j.owner != m.last {
					index = i
					if j.owner != m.last {
						break
					}
				}
			}
			if index < 0 {
				m.queue = nil
				m.mu.Unlock()
				break
			}
			id := m.queue[index]
			m.queue = append(m.queue[:index], m.queue[index+1:]...)
			j := m.jobs[id]
			m.last = j.owner
			j.state = "running"
			ctx, cancel := context.WithTimeout(m.ctx, 35*time.Second)
			j.cancel = cancel
			m.mu.Unlock()
			started := time.Now()
			data, err := m.execute(ctx, j.request)
			if ctx.Err() != nil {
				err = ctx.Err()
			}
			cancel()
			m.mu.Lock()
			if m.jobs[id] == j && j.state == "running" {
				if err == nil && ctx.Err() != context.DeadlineExceeded && m.ctx.Err() == nil {
					err = m.publish(id, data)
				}
				if err != nil || m.ctx.Err() != nil {
					j.state = "failed"
				} else {
					j.state = "ready"
				}
			}
			slog.Info("render finished", "job", id, "state", j.state, "elapsed_ms", time.Since(started).Milliseconds(), "bytes", len(data))
			m.mu.Unlock()
			if m.ctx.Err() != nil {
				return
			}
		}
	}
}

func (m *Manager) execute(ctx context.Context, q Request) ([]byte, error) {
	var b bytes.Buffer
	boundedCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	if err := m.cfg.Renderer.Render(boundedCtx, q, &limitWriter{w: &b, left: MaxImage, cancel: cancel}); err != nil {
		return nil, err
	}
	r, err := publish.Validate(q.Recipe)
	if err != nil {
		return nil, err
	}
	tier, err := publish.Tier(r.ID(), q.Tier)
	if err != nil {
		return nil, err
	}
	c, err := png.DecodeConfig(bytes.NewReader(b.Bytes()))
	if err != nil || c.Width != tier.Width || c.Height != tier.Height {
		return nil, errors.New("invalid renderer dimensions")
	}
	if _, err := png.Decode(bytes.NewReader(b.Bytes())); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

func (m *Manager) publish(id string, b []byte) error {
	m.prune()
	var fs syscall.Statfs_t
	if err := syscall.Statfs(m.cfg.Directory, &fs); err != nil {
		return err
	}
	if fs.Bavail*uint64(fs.Bsize) < uint64(len(b))+(64<<20) {
		return errors.New("disk capacity low")
	}
	for (m.bytes+int64(len(b)) > m.cfg.CacheBytes || len(m.artifacts) >= m.cfg.CacheCount) && len(m.artifacts) > 0 {
		if err := m.evictOldest(); err != nil {
			return err
		}
	}
	if m.bytes+int64(len(b)) > m.cfg.CacheBytes {
		return ErrBusy
	}
	f, err := os.CreateTemp(m.cfg.Directory, "pending-")
	if err != nil {
		return err
	}
	name := f.Name()
	defer os.Remove(name)
	if _, err = f.Write(b); err != nil {
		f.Close()
		return err
	}
	if err = f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	if err = os.Rename(name, filepath.Join(m.cfg.Directory, id+".png")); err != nil {
		return err
	}
	sum := sha256.Sum256(b)
	if old, ok := m.artifacts[id]; ok {
		m.bytes -= old.size
	}
	m.artifacts[id] = artifact{size: int64(len(b)), created: time.Now(), digest: hex.EncodeToString(sum[:])}
	m.bytes += int64(len(b))
	return nil
}

// Lease prevents eviction until a bounded download closes its open file.
type Lease struct {
	*os.File
	release func()
	once    sync.Once
}

// Close releases both the descriptor and its cache reservation.
func (l *Lease) Close() error { err := l.File.Close(); l.once.Do(l.release); return err }

func (m *Manager) prune() {
	for id, a := range m.artifacts {
		if a.refs == 0 && time.Since(a.created) > m.cfg.MaxAge {
			_ = m.remove(id)
		}
	}
	for id, j := range m.jobs {
		if j.state != "running" && j.state != "queued" && time.Since(j.created) > 30*time.Minute {
			delete(m.jobs, id)
		}
	}
	for m.bytes > m.cfg.CacheBytes || len(m.artifacts) > m.cfg.CacheCount {
		if m.evictOldest() != nil {
			break
		}
	}
}

func (m *Manager) remove(id string) error {
	a := m.artifacts[id]
	if a.refs > 0 {
		return ErrBusy
	}
	if err := os.Remove(filepath.Join(m.cfg.Directory, id+".png")); err != nil && !os.IsNotExist(err) {
		return err
	}
	m.bytes -= a.size
	delete(m.artifacts, id)
	return nil
}

func (m *Manager) evictOldest() error {
	var key string
	var oldest time.Time
	for id, a := range m.artifacts {
		if a.refs == 0 && (key == "" || a.created.Before(oldest)) {
			key = id
			oldest = a.created
		}
	}
	if key == "" {
		return ErrBusy
	}
	return m.remove(key)
}

func (m *Manager) reconcile() error {
	entries, err := os.ReadDir(m.cfg.Directory)
	if err != nil {
		return err
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
	for _, e := range entries {
		name := e.Name()
		path := filepath.Join(m.cfg.Directory, name)
		if strings.HasPrefix(name, "pending-") {
			if err := os.Remove(path); err != nil {
				return err
			}
			continue
		}
		id := strings.TrimSuffix(name, ".png")
		if len(id) != 64 || !strings.HasSuffix(name, ".png") || e.Type()&os.ModeSymlink != 0 {
			continue
		}
		if _, err := hex.DecodeString(id); err != nil {
			continue
		}
		info, err := e.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			continue
		}
		if info.Size() > MaxImage || time.Since(info.ModTime()) > m.cfg.MaxAge || len(m.artifacts) >= m.cfg.CacheCount || m.bytes+info.Size() > m.cfg.CacheBytes {
			if err := os.Remove(path); err != nil {
				return err
			}
			continue
		}
		f, err := os.Open(path)
		if err != nil {
			return err
		}
		h := sha256.New()
		_, err = io.Copy(h, f)
		closeErr := f.Close()
		if err != nil {
			return err
		}
		if closeErr != nil {
			return closeErr
		}
		m.artifacts[id] = artifact{size: info.Size(), created: info.ModTime(), digest: hex.EncodeToString(h.Sum(nil))}
		m.bytes += info.Size()
	}
	return nil
}
