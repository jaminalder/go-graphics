// Package studio owns short-lived explorations and explicit visitor choices.
package studio

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/jaminalder/go-graphics/internal/artwork"
	"github.com/jaminalder/go-graphics/internal/explore"
	"github.com/jaminalder/go-graphics/internal/publish"
	"github.com/jaminalder/go-graphics/internal/renderjob"
)

// ErrExpired means transient state is absent or belongs to another workspace.
var (
	ErrExpired = errors.New("this exploration has expired; restore favourites or start again")
	// ErrConflict requires a refreshed page before another mutation.
	ErrConflict = errors.New("your choices changed in another tab; refresh this page and try again")
)

// Workspace is the capability and synchronizer token returned on an explicit start.
type (
	Workspace struct{ Token, CSRF string }
	// Sample binds an immutable recipe to optional rendition job identities.
	Sample struct {
		ID             string
		Recipe         artwork.Recipe
		Job, Download  string
		Favourite      bool
		Cancelled      bool
		Status         renderjob.Status
		DownloadStatus renderjob.Status
	}
	// Batch is four independently tracked samples, retained for navigation.
	Batch struct {
		ID      string
		Samples []string
	}
	// Exploration is an independently revisioned tab's visual choices.
	Exploration struct {
		ID, Artwork, Style, Colour string
		Revision                   int
		Batches                    []Batch
		Samples                    []Sample
		Favourites                 []Sample
		Active                     bool
	}
	action      struct{ id, digest, batch string }
	exploration struct {
		view    Exploration
		actions []action
		round   int
	}
	workspace struct {
		Workspace
		created, touched time.Time
		explorations     map[string]*exploration
	}
	// Store keeps bounded in-memory navigation; it never owns image bytes.
	Store struct {
		mu         sync.Mutex
		workspaces map[string]*workspace
		jobs       *renderjob.Manager
		build      string
	}
)

// New constructs a bounded transient studio.
func New(jobs *renderjob.Manager, build string) *Store {
	return &Store{workspaces: map[string]*workspace{}, jobs: jobs, build: build}
}

// Token creates an opaque cryptographic capability, never an artistic RNG stream.
func Token() string {
	var b [24]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b[:])
}

// Create starts state only in response to an explicit interaction.
func (s *Store) Create() (Workspace, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.prune()
	if len(s.workspaces) >= 1000 {
		return Workspace{}, renderjob.ErrBusy
	}
	w := Workspace{Token: Token(), CSRF: Token()}
	s.workspaces[w.Token] = &workspace{Workspace: w, created: time.Now(), touched: time.Now(), explorations: map[string]*exploration{}}
	return w, nil
}

func (s *Store) prune() {
	now := time.Now()
	for key, w := range s.workspaces {
		if now.Sub(w.touched) > 30*time.Minute || now.Sub(w.created) > 24*time.Hour {
			if s.jobs != nil {
				for _, x := range w.explorations {
					for _, sample := range x.view.Samples {
						s.jobs.Cancel(key, sample.Job)
						s.jobs.Cancel(key, sample.Download)
					}
				}
			}
			delete(s.workspaces, key)
		}
	}
}

func (s *Store) workspace(token string) (*workspace, error) {
	s.prune()
	w := s.workspaces[token]
	if w == nil {
		return nil, ErrExpired
	}
	w.touched = time.Now()
	return w, nil
}

// Session returns only an existing capability; GET never creates one.
func (s *Store) Session(token string) (Workspace, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	w, e := s.workspace(token)
	if e != nil {
		return Workspace{}, e
	}
	return w.Workspace, nil
}

// Start opens an artwork in its own exploration, with bounded tabs.
func (s *Store) Start(token, id, style, colour string) (Exploration, error) {
	if _, _, e := publish.Pins(id, style, colour); e != nil {
		return Exploration{}, e
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	w, e := s.workspace(token)
	if e != nil {
		return Exploration{}, e
	}
	if len(w.explorations) >= 4 {
		return Exploration{}, errors.New("you have four explorations open; clear this session to begin again")
	}
	x := &exploration{view: Exploration{ID: Token(), Artwork: id, Style: style, Colour: colour}}
	w.explorations[x.view.ID] = x
	return x.view, nil
}

func (s *Store) find(token, id string) (*exploration, error) {
	w, e := s.workspace(token)
	if e != nil {
		return nil, e
	}
	x := w.explorations[id]
	if x == nil {
		return nil, ErrExpired
	}
	return x, nil
}

// Get returns an isolated snapshot with current rendition state.
func (s *Store) Get(token, id string) (Exploration, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	x, e := s.find(token, id)
	if e != nil {
		return Exploration{}, e
	}
	return s.snapshot(token, x), nil
}

func (s *Store) snapshot(token string, x *exploration) Exploration {
	v := x.view
	v.Batches = append([]Batch(nil), v.Batches...)
	for i := range v.Batches {
		v.Batches[i].Samples = append([]string(nil), v.Batches[i].Samples...)
	}
	v.Samples = append([]Sample(nil), v.Samples...)
	v.Favourites = nil
	v.Active = false
	for i := range v.Samples {
		a := &v.Samples[i]
		a.Status = renderjob.Status{State: "expired"}
		if s.jobs != nil {
			a.Status = s.jobs.Status(token, a.Job)
			if a.Download != "" {
				a.DownloadStatus = s.jobs.Status(token, a.Download)
			}
		}
		if a.Status.State != "ready" && a.DownloadStatus.State == "ready" {
			a.Status = a.DownloadStatus
			a.Job = a.Download
		}
		if a.Cancelled && a.Status.State != "ready" {
			a.Status.State = "cancelled"
		}
		if a.Status.State == "queued" || a.Status.State == "running" || a.DownloadStatus.State == "queued" || a.DownloadStatus.State == "running" {
			v.Active = true
		}
		if a.Favourite {
			v.Favourites = append(v.Favourites, *a)
		}
	}
	return v
}

// Choices validates revision and keeps existing favourites immutable.
func (s *Store) Choices(token, id string, revision int, style, colour string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	x, e := s.find(token, id)
	if e != nil {
		return e
	}
	if revision != x.view.Revision {
		return ErrConflict
	}
	if _, _, e := publish.Pins(x.view.Artwork, style, colour); e != nil {
		return e
	}
	x.view.Style = style
	x.view.Colour = colour
	x.view.Revision++
	return nil
}

// Generate performs idempotent four-sample admission without holding an HTTP request open.
func (s *Store) Generate(token, id string, revision int, actionID string, selected []string, different bool) (string, error) {
	seenSelected := map[string]bool{}
	for _, id := range selected {
		if seenSelected[id] {
			return "", errors.New("duplicate selected favourite")
		}
		seenSelected[id] = true
	}
	if len(actionID) != 48 || len(selected) > 4 {
		return "", errors.New("invalid selection")
	}
	if _, e := hex.DecodeString(actionID); e != nil {
		return "", errors.New("invalid action")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	x, e := s.find(token, id)
	if e != nil {
		return "", e
	}
	body, _ := json.Marshal(struct {
		Revision  int
		Selected  []string
		Different bool
	}{revision, selected, different})
	sum := sha256.Sum256(body)
	digest := hex.EncodeToString(sum[:])
	for _, a := range x.actions {
		if a.id == actionID {
			if a.digest != digest {
				return "", ErrConflict
			}
			return a.batch, nil
		}
	}
	if revision != x.view.Revision {
		return "", ErrConflict
	}
	if s.snapshot(token, x).Active {
		return "", errors.New("your current samples are still being made")
	}
	if s.jobs == nil {
		return "", renderjob.ErrBusy
	}
	var parents []explore.Candidate
	palettes := map[uint64]string{}
	if !different {
		for _, sid := range selected {
			found := false
			for _, a := range x.view.Samples {
				if a.ID == sid && a.Favourite {
					n, _ := strconv.ParseUint(a.Recipe.Seed(), 10, 64)
					parents = append(parents, explore.Candidate{Seed: n, Traits: a.Recipe.Traits()})
					palettes[n] = a.Recipe.Palette()
					found = true
				}
			}
			if !found {
				return "", errors.New("select existing favourites")
			}
		}
	}
	pins, pal, e := publish.Pins(x.view.Artwork, x.view.Style, x.view.Colour)
	if e != nil {
		return "", e
	}
	space, e := publish.Space(x.view.Artwork)
	if e != nil {
		return "", e
	}
	var entropy [8]byte
	if _, e := rand.Read(entropy[:]); e != nil {
		return "", e
	}
	var recent []uint64
	for _, a := range x.view.Samples {
		n, _ := strconv.ParseUint(a.Recipe.Seed(), 10, 64)
		recent = append(recent, n)
	}
	candidates, e := explore.Public(space, pins, parents, binary.LittleEndian.Uint64(entropy[:]), x.round, recent)
	if e != nil {
		return "", e
	}
	var recipes []artwork.Recipe
	var qs []renderjob.Request
	entry, _ := publish.Get(x.view.Artwork)
	for _, c := range candidates {
		actualPal := pal
		if actualPal == "" {
			actualPal = entry.Colours[c.Seed%uint64(len(entry.Colours))].ID
			if c.Mode == "neighbor" && palettes[c.Parent] != "" {
				actualPal = palettes[c.Parent]
			}
		}
		r, e := publish.Complete(x.view.Artwork, c.Seed, actualPal, c.Traits)
		if e != nil {
			return "", e
		}
		recipes = append(recipes, r)
		qs = append(qs, renderjob.Request{Version: 1, Build: s.build, Recipe: r.Bytes(), Tier: "preview"})
	}
	if s.recipeBytes()+int64(len(qs))*16384 > 32<<20 {
		return "", renderjob.ErrBusy
	}
	jobs, e := s.jobs.Admit(token, qs)
	if e != nil {
		return "", e
	}
	b := Batch{ID: Token()}
	for i, r := range recipes {
		a := Sample{ID: Token(), Recipe: r, Job: jobs[i]}
		x.view.Samples = append(x.view.Samples, a)
		b.Samples = append(b.Samples, a.ID)
	}
	x.view.Batches = append(x.view.Batches, b)
	if len(x.view.Batches) > 8 {
		x.view.Batches = x.view.Batches[1:]
	}
	keep := map[string]bool{}
	for _, b := range x.view.Batches {
		for _, id := range b.Samples {
			keep[id] = true
		}
	}
	samples := x.view.Samples[:0]
	for _, a := range x.view.Samples {
		if a.Favourite || keep[a.ID] {
			samples = append(samples, a)
		}
	}
	x.view.Samples = samples
	x.actions = append(x.actions, action{actionID, digest, b.ID})
	if len(x.actions) > 8 {
		x.actions = x.actions[1:]
	}
	x.round++
	x.view.Revision++
	return b.ID, nil
}

func (s *Store) recipeBytes() int64 {
	var n int64
	for _, w := range s.workspaces {
		for _, x := range w.explorations {
			for _, a := range x.view.Samples {
				n += int64(len(a.Recipe.Bytes()))
			}
		}
	}
	return n
}

// Favourite changes one explicit choice and enforces the per-workspace limit.
func (s *Store) Favourite(token, id, sample string, revision int, on bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	x, e := s.find(token, id)
	if e != nil {
		return e
	}
	if x.view.Revision != revision {
		return ErrConflict
	}
	w := s.workspaces[token]
	count := 0
	for _, x := range w.explorations {
		for _, a := range x.view.Samples {
			if a.Favourite {
				count++
			}
		}
	}
	for i := range x.view.Samples {
		a := &x.view.Samples[i]
		if a.ID != sample {
			continue
		}
		if on && !a.Favourite && count >= 24 {
			return errors.New("24 favourites kept; remove one before adding another")
		}
		a.Favourite = on
		x.view.Revision++
		return nil
	}
	return ErrExpired
}

// Cancel releases all unfinished interests while retaining completed samples.
func (s *Store) Cancel(token, id string, revision int, batchID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	x, e := s.find(token, id)
	if e != nil {
		return e
	}
	if x.view.Revision != revision || len(x.view.Batches) == 0 || x.view.Batches[len(x.view.Batches)-1].ID != batchID {
		return ErrConflict
	}
	members := map[string]bool{}
	for _, sid := range x.view.Batches[len(x.view.Batches)-1].Samples {
		members[sid] = true
	}
	for i := range x.view.Samples {
		a := &x.view.Samples[i]
		if members[a.ID] && s.jobs != nil {
			status := s.jobs.Status(token, a.Job)
			if status.State == "queued" || status.State == "running" {
				s.jobs.Cancel(token, a.Job)
				a.Cancelled = true
			}
		}
	}
	x.view.Revision++
	return nil
}

// Download explicitly admits a larger rendition of the same recipe.
func (s *Store) Download(token, id, sample string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	x, e := s.find(token, id)
	if e != nil {
		return e
	}
	for i := range x.view.Samples {
		a := &x.view.Samples[i]
		if a.ID != sample {
			continue
		}
		if s.jobs == nil {
			return renderjob.ErrBusy
		}
		if a.Download != "" {
			st := s.jobs.Status(token, a.Download)
			if st.State == "queued" || st.State == "running" || st.State == "ready" {
				return nil
			}
		}
		ids, e := s.jobs.Admit(token, []renderjob.Request{{Version: 1, Build: s.build, Recipe: a.Recipe.Bytes(), Tier: "download"}})
		if e != nil {
			return e
		}
		a.Download = ids[0]
		return nil
	}
	return ErrExpired
}

// Recovery exports bounded non-secret recipe records only.
func (s *Store) Recovery(token string) ([]json.RawMessage, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	w, e := s.workspace(token)
	if e != nil {
		return nil, e
	}
	out := []json.RawMessage{}
	for _, x := range w.explorations {
		for _, a := range x.view.Samples {
			if a.Favourite {
				out = append(out, a.Recipe.Bytes())
			}
		}
	}
	return out, nil
}

// Restore validates an untrusted bounded recipe file before allocating recovered state.
func (s *Store) Restore(token string, records []json.RawMessage) (string, error) {
	if len(records) > 24 {
		return "", errors.New("too many favourites")
	}
	recipes := make([]artwork.Recipe, 0, len(records))
	seen := map[string]bool{}
	for _, raw := range records {
		r, e := publish.Validate(raw)
		if e != nil {
			return "", e
		}
		if !seen[r.Digest()] {
			recipes = append(recipes, r)
			seen[r.Digest()] = true
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	w, e := s.workspace(token)
	if e != nil {
		return "", e
	}
	if len(w.explorations) != 0 {
		return "", errors.New("restore into a fresh session to preserve your current choices")
	}
	if s.recipeBytes()+int64(len(recipes))*16384 > 32<<20 {
		return "", renderjob.ErrBusy
	}
	byArt := map[string]*exploration{}
	first := ""
	for _, r := range recipes {
		x := byArt[r.ID()]
		if x == nil {
			x = &exploration{view: Exploration{ID: Token(), Artwork: r.ID()}}
			byArt[r.ID()] = x
			w.explorations[x.view.ID] = x
			if first == "" {
				first = x.view.ID
			}
		}
		x.view.Samples = append(x.view.Samples, Sample{ID: Token(), Recipe: r, Favourite: true})
	}
	return first, nil
}

// Clear explicitly removes server state and cancels active interests.
func (s *Store) Clear(token string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if w := s.workspaces[token]; w != nil {
		w.touched = time.Time{}
	}
	s.prune()
}

// String returns a concise sample label, keeping seed precision intact.
func (a Sample) String() string { return fmt.Sprintf("%s · %s", a.Recipe.ID(), a.Recipe.Seed()) }
