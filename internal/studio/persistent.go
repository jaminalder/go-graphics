package studio

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/jaminalder/go-graphics/internal/artwork"
	"github.com/jaminalder/go-graphics/internal/persistence"
	"github.com/jaminalder/go-graphics/internal/renderjob"
)

// Persistent runs the existing bounded studio domain inside database transactions.
// The snapshot is a bounded aggregate, not process-local authoritative state.
type Persistent struct {
	DB      *persistence.DB
	Context context.Context
}

type (
	savedWorkspace struct {
		Explorations []savedExploration
		Entries      []savedEntry
	}
	savedEntry       struct{ ID, Artwork, Style, Colour, Exploration string }
	savedAction      struct{ ID, Digest, Batch string }
	savedExploration struct {
		View    Exploration
		Actions []savedAction
		Round   int
		Recipes []json.RawMessage
	}
)

func encodeWorkspace(w *workspace) ([]byte, error) {
	v := savedWorkspace{}
	for _, e := range w.entries {
		v.Entries = append(v.Entries, savedEntry{e.id, e.artwork, e.style, e.colour, e.exploration})
	}
	for _, x := range w.explorations {
		s := savedExploration{View: x.view, Round: x.round}
		s.View.Favourites = nil
		for _, a := range x.actions {
			s.Actions = append(s.Actions, savedAction{a.id, a.digest, a.batch})
		}
		for _, a := range x.view.Samples {
			s.Recipes = append(s.Recipes, a.Recipe.Bytes())
		}
		v.Explorations = append(v.Explorations, s)
	}
	return json.Marshal(v)
}

func decodeWorkspace(data []byte, w *workspace) error {
	if len(data) == 0 {
		return nil
	}
	var v savedWorkspace
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	for _, e := range v.Entries {
		w.entries = append(w.entries, entryAction{e.ID, e.Artwork, e.Style, e.Colour, e.Exploration})
	}
	for _, s := range v.Explorations {
		if len(s.Recipes) != len(s.View.Samples) {
			return errors.New("invalid persisted recipes")
		}
		x := &exploration{view: s.View, round: s.Round}
		for _, a := range s.Actions {
			x.actions = append(x.actions, action{a.ID, a.Digest, a.Batch})
		}
		for i, raw := range s.Recipes {
			r, err := artwork.Decode(raw)
			if err != nil {
				return err
			}
			x.view.Samples[i].Recipe = r
		}
		w.explorations[x.view.ID] = x
	}
	return nil
}

func (p *Persistent) ctx() context.Context {
	if p.Context != nil {
		return p.Context
	}
	return context.Background()
}

func (p *Persistent) transaction(token string, write bool, fn func(*Store, string) error) (result error) {
	defer func() {
		var pgErr interface{ SQLState() string }
		if errors.As(result, &pgErr) {
			slog.Error("studio transaction failed", "sqlstate", pgErr.SQLState())
			result = persistence.ErrUnavailable
		}
	}()
	ctx, cancel := context.WithTimeout(p.ctx(), 10*time.Second)
	defer cancel()
	tx, err := p.DB.Pool.Begin(ctx)
	if err != nil {
		return persistence.ErrUnavailable
	}
	defer persistence.Rollback(tx)
	// All mutations use one lock order: control, workspace, requests, then River.
	if write {
		if _, err = tx.Exec(ctx, "SELECT singleton FROM art_control FOR UPDATE"); err != nil {
			return persistence.ErrUnavailable
		}
	}
	id := persistence.TokenHash(token)
	var csrf string
	var data []byte
	suffix := ""
	if write {
		suffix = " FOR UPDATE"
	}
	err = tx.QueryRow(ctx, "SELECT csrf,data FROM art_workspaces WHERE id=$1 AND expires>now()"+suffix, id).Scan(&csrf, &data)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrExpired
	}
	if err != nil {
		return persistence.ErrUnavailable
	}
	w := &workspace{Workspace: Workspace{Token: id, CSRF: csrf}, explorations: map[string]*exploration{}}
	if err = decodeWorkspace(data, w); err != nil {
		return persistence.ErrUnavailable
	}
	queue := &persistence.QueueTx{DB: p.DB, Tx: tx, Context: ctx}
	s := New(queue, p.DB.Build)
	s.persistent = true
	s.workspaces[id] = w
	if err = fn(s, id); err != nil {
		return err
	}
	if queue.Err != nil {
		return persistence.ErrUnavailable
	}
	if write {
		data, err = encodeWorkspace(w)
		if err != nil {
			return err
		}
		var total int64
		if err = tx.QueryRow(ctx, "SELECT coalesce(sum(octet_length(data)),0) FROM art_workspaces WHERE id<>$1", id).Scan(&total); err != nil {
			return persistence.ErrUnavailable
		}
		if total+int64(len(data)) > 64<<20 {
			return renderjob.ErrBusy
		}
		if _, err = tx.Exec(ctx, "UPDATE art_workspaces SET data=$2 WHERE id=$1", id, data); err != nil {
			return persistence.ErrUnavailable
		}
		if _, err = tx.Exec(ctx, "DELETE FROM art_pins WHERE workspace=$1", id); err != nil {
			return persistence.ErrUnavailable
		}
		keep := map[string]bool{}
		for _, x := range w.explorations {
			for _, a := range x.view.Samples {
				if a.Favourite && a.Job != "" {
					if _, err = tx.Exec(ctx, "INSERT INTO art_pins VALUES($1,$2,$3)", id, a.ID, a.Job); err != nil {
						return persistence.ErrUnavailable
					}
				}
				if a.Job != "" {
					keep[a.Job] = true
				}
				if a.Download != "" {
					keep[a.Download] = true
				}
			}
		}
		rows, e := tx.Query(ctx, "SELECT request FROM art_interests WHERE workspace=$1", id)
		if e != nil {
			return persistence.ErrUnavailable
		}
		var obsolete []string
		for rows.Next() {
			var key string
			if e = rows.Scan(&key); e != nil {
				rows.Close()
				return persistence.ErrUnavailable
			}
			if !keep[key] {
				obsolete = append(obsolete, key)
			}
		}
		rows.Close()
		if rows.Err() != nil {
			return persistence.ErrUnavailable
		}
		for _, key := range obsolete {
			queue.Cancel(id, key)
		}
		if queue.Err != nil {
			return persistence.ErrUnavailable
		}
		if err = persistence.Notify(ctx, tx, id); err != nil {
			return persistence.ErrUnavailable
		}
	}
	if err = tx.Commit(ctx); err != nil {
		return persistence.ErrUnavailable
	}
	return nil
}

// Create establishes an anonymous identity only after explicit interaction.
func (p *Persistent) Create() (Workspace, error) {
	ctx, cancel := context.WithTimeout(p.ctx(), 10*time.Second)
	defer cancel()
	tx, err := p.DB.Pool.Begin(ctx)
	if err != nil {
		return Workspace{}, persistence.ErrUnavailable
	}
	defer persistence.Rollback(tx)
	if _, err = tx.Exec(ctx, "SELECT singleton FROM art_control FOR UPDATE"); err != nil {
		return Workspace{}, persistence.ErrUnavailable
	}
	var count int
	if err = tx.QueryRow(ctx, "SELECT count(*) FROM art_workspaces").Scan(&count); err != nil {
		return Workspace{}, persistence.ErrUnavailable
	}
	if count >= 1000 {
		return Workspace{}, renderjob.ErrBusy
	}
	w := Workspace{Token: Token(), CSRF: Token()}
	if _, err = tx.Exec(ctx, "INSERT INTO art_workspaces(id,csrf,expires) VALUES($1,$2,now()+interval '90 days')", persistence.TokenHash(w.Token), w.CSRF); err != nil {
		return Workspace{}, persistence.ErrUnavailable
	}
	if err = tx.Commit(ctx); err != nil {
		return Workspace{}, persistence.ErrUnavailable
	}
	return w, nil
}

// Session looks up an existing capability without prolonging its lifetime.
func (p *Persistent) Session(token string) (Workspace, error) {
	if len(token) != 48 {
		return Workspace{}, ErrExpired
	}
	w := Workspace{Token: token}
	ctx, cancel := context.WithTimeout(p.ctx(), 10*time.Second)
	defer cancel()
	err := p.DB.Pool.QueryRow(ctx, "SELECT csrf FROM art_workspaces WHERE id=$1 AND expires>now()", persistence.TokenHash(token)).Scan(&w.CSRF)
	if errors.Is(err, pgx.ErrNoRows) {
		return Workspace{}, ErrExpired
	}
	if err != nil {
		return Workspace{}, persistence.ErrUnavailable
	}
	return w, nil
}

// Touch refreshes only an existing unexpired identity on meaningful navigation/actions.
func (p *Persistent) Touch(token string) error {
	ctx, cancel := context.WithTimeout(p.ctx(), 10*time.Second)
	defer cancel()
	tag, err := p.DB.Pool.Exec(ctx, "UPDATE art_workspaces SET touched=now(),expires=now()+interval '90 days' WHERE id=$1 AND expires>now()", persistence.TokenHash(token))
	if err != nil {
		return persistence.ErrUnavailable
	}
	if tag.RowsAffected() != 1 {
		return ErrExpired
	}
	return nil
}

// Get returns current navigation and River/artifact state.
func (p *Persistent) Get(t, id string) (v Exploration, err error) {
	err = p.transaction(t, false, func(s *Store, k string) error { v, err = s.Get(k, id); return err })
	return
}

// Enter persists a replay-safe initial batch and its River jobs together.
func (p *Persistent) Enter(t, a, st, c, action string) (v string, err error) {
	err = p.transaction(t, true, func(s *Store, k string) error { v, err = s.Enter(k, a, st, c, action); return err })
	return
}

// Favourites returns retained recipes and preview pointers across visits.
func (p *Persistent) Favourites(t string) (v []Favourite, err error) {
	err = p.transaction(t, false, func(s *Store, k string) error { v, err = s.Favourites(k); return err })
	return
}

// Choices changes revisioned visual choices.
func (p *Persistent) Choices(t, id string, r int, st, c string) error {
	return p.transaction(t, true, func(s *Store, k string) error { return s.Choices(k, id, r, st, c) })
}

// Retry repeats an unavailable batch's direction.
func (p *Persistent) Retry(t, id string, r int, a, b string) (v string, err error) {
	err = p.transaction(t, true, func(s *Store, k string) error { v, err = s.Retry(k, id, r, a, b); return err })
	return
}

// Generate atomically saves concrete random choices and queue admission.
func (p *Persistent) Generate(t, id string, r int, a string, selected []string, d bool) (v string, err error) {
	err = p.transaction(t, true, func(s *Store, k string) error { v, err = s.Generate(k, id, r, a, selected, d); return err })
	return
}

// Favourite changes a retained choice and pins its preview against cache eviction.
func (p *Persistent) Favourite(t, id, sample string, r int, on bool) error {
	return p.transaction(t, true, func(s *Store, k string) error {
		if err := s.Favourite(k, id, sample, r, on); err != nil {
			return err
		}
		if !on {
			return nil
		}
		x := s.workspaces[k].explorations[id]
		for i := range x.view.Samples {
			a := &x.view.Samples[i]
			if a.ID != sample {
				continue
			}
			status := s.jobs.Status(k, a.Job).State
			if status == "ready" || status == "queued" || status == "running" {
				return nil
			}
			ids, err := s.jobs.Admit(k, []renderjob.Request{{Version: 1, Build: s.build, Recipe: a.Recipe.Bytes(), Tier: "preview"}})
			if err != nil {
				return err
			}
			a.Job = ids[0]
		}
		return nil
	})
}

// Cancel removes this workspace's unfinished interests.
func (p *Persistent) Cancel(t, id string, r int, b string) error {
	return p.transaction(t, true, func(s *Store, k string) error { return s.Cancel(k, id, r, b) })
}

// Download explicitly admits a larger rendition.
func (p *Persistent) Download(t, id, sample string) error {
	return p.transaction(t, true, func(s *Store, k string) error { return s.Download(k, id, sample) })
}

// Recovery exports recipes without the anonymous capability.
func (p *Persistent) Recovery(t string) (v []json.RawMessage, err error) {
	err = p.transaction(t, false, func(s *Store, k string) error { v, err = s.Recovery(k); return err })
	return
}

// Restore imports explicitly supplied recipes, not a database snapshot.
func (p *Persistent) Restore(t string, records []json.RawMessage) (v string, err error) {
	err = p.transaction(t, true, func(s *Store, k string) error { v, err = s.Restore(k, records); return err })
	return
}

// Clear removes ownership, pins and unfinished interests transactionally.
func (p *Persistent) Clear(t string) error {
	return p.transaction(t, true, func(s *Store, k string) error {
		for _, x := range s.workspaces[k].explorations {
			for _, a := range x.view.Samples {
				s.jobs.Cancel(k, a.Job)
				s.jobs.Cancel(k, a.Download)
			}
		}
		q := s.jobs.(*persistence.QueueTx)
		_, err := q.Tx.Exec(q.Context, "UPDATE art_workspaces SET expires=now() WHERE id=$1", k)
		s.workspaces[k].explorations = map[string]*exploration{}
		s.workspaces[k].entries = nil
		return err
	})
}
