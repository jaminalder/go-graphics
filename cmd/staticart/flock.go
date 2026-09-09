package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"image"
	"io"
	"math/rand/v2"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"

	"github.com/jaminalder/go-graphics/internal/render"
	"github.com/jaminalder/go-graphics/internal/sketch"
	"github.com/jaminalder/go-graphics/internal/trait"
)

// flockLimit caps explore/breed size the same way sweep guards typos.
const flockLimit = 120

// neighborStride spaces child seeds away from a liked parent so the
// continuous recipe draw moves without leaving the trait cell.
const neighborStride uint64 = 100003

// flockEntry is one row of flock.jsonl — enough to like a sheep and breed.
type flockEntry struct {
	Seed   uint64            `json:"seed"`
	Traits map[string]string `json:"traits"`
	File   string            `json:"file"`
	Mode   string            `json:"mode"`
	Parent uint64            `json:"parent,omitempty"`
}

type flockMember struct {
	seed   uint64
	traits trait.Set
	mode   string
	parent uint64
	pins   []string // --dim value pairs for render
}

func runFlock(args []string) error {
	if len(args) == 0 || strings.HasPrefix(args[0], "-") {
		return fmt.Errorf("flock needs a sketch name (try: staticart list)")
	}
	name := args[0]
	s, ok := registry().Get(name)
	if !ok {
		return fmt.Errorf("unknown sketch %q (try: staticart list)", name)
	}
	t, ok := s.(sketch.Traited)
	if !ok {
		return fmt.Errorf("sketch %q has no traits — flock needs a Traited sketch", name)
	}

	opts, rest, err := splitFlockArgs(args[1:])
	if err != nil {
		return err
	}
	if opts.count < 1 {
		return fmt.Errorf("flock needs --count N")
	}
	if opts.count > flockLimit {
		return fmt.Errorf("that is %d renders; %d is the cap", opts.count, flockLimit)
	}
	// Apply sketch CLI overrides before planning so flock.jsonl records the
	// same trait pins the renders use (--medium wash, --tint split, …).
	if err := configureFlockSketch(s, rest); err != nil {
		return err
	}

	members, err := planFlock(t, opts)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(opts.out, 0o755); err != nil {
		return err
	}

	type result struct {
		img  image.Image
		path string
		ent  flockEntry
	}
	results := make([]result, len(members))
	errs := make([]error, len(members))

	jobs := opts.jobs
	if jobs < 1 {
		jobs = min(max(runtime.NumCPU()/2, 1), 4)
	}
	var wg sync.WaitGroup
	queue := make(chan int)
	for range jobs {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range queue {
				m := members[i]
				// Pins first, then CLI rest, so explore/breed overrides
				// (--medium wash) win over derived pins (ember).
				full := []string{name}
				full = append(full, m.pins...)
				full = append(full, rest...)
				full = append(full, "--seed", strconv.FormatUint(m.seed, 10))
				full = append(full, "--out", opts.out)
				out, err := renderOne(full)
				if err != nil {
					errs[i] = err
					continue
				}
				base := fmt.Sprintf("%02d_%s", i+1, filepath.Base(out.path))
				out.path = filepath.Join(opts.out, base)
				if err := writeRender(out); err != nil {
					errs[i] = err
					continue
				}
				results[i] = result{
					img:  out.img,
					path: out.path,
					ent: flockEntry{
						Seed:   m.seed,
						Traits: overlayCLITraits(map[string]string(m.traits), rest, t.Schema()),
						File:   base,
						Mode:   m.mode,
						Parent: m.parent,
					},
				}
			}
		}()
	}
	for i := range members {
		queue <- i
	}
	close(queue)
	wg.Wait()
	if err := errors.Join(errs...); err != nil {
		return err
	}

	var man, jl strings.Builder
	imgs := make([]image.Image, 0, len(results))
	for i, r := range results {
		imgs = append(imgs, r.img)
		label := t.Schema().Format(trait.Set(r.ent.Traits))
		if r.ent.Mode != "" {
			label = r.ent.Mode + " " + label
		}
		fmt.Fprintf(&man, "%02d  seed=%-8d %-52s %s\n", i+1, r.ent.Seed, r.ent.File, label)
		b, err := json.Marshal(r.ent)
		if err != nil {
			return err
		}
		jl.Write(b)
		jl.WriteByte('\n')
	}
	if err := os.WriteFile(filepath.Join(opts.out, "manifest.txt"), []byte(man.String()), 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(opts.out, "flock.jsonl"), []byte(jl.String()), 0o644); err != nil {
		return err
	}

	cols := opts.cols
	if cols < 1 {
		cols = 1
		for cols*cols < len(imgs) {
			cols++
		}
	}
	sheet := filepath.Join(opts.out, "sheet.png")
	if err := render.WritePNGMeta(sheet, render.ContactSheet(imgs, cols, opts.cell, 6),
		render.Meta{Software: "staticart " + buildRevision(), Comment: man.String()}); err != nil {
		return err
	}
	fmt.Printf("%d sheep in %s\n%s\n", len(imgs), opts.out, sheet)
	return nil
}

type flockOpts struct {
	count    int
	seedBase uint64
	from     string
	likes    []uint64
	boost    float64
	out      string
	cols     int
	cell     int
	jobs     int
}

func splitFlockArgs(args []string) (flockOpts, []string, error) {
	o := flockOpts{out: "out/flock", cell: 280, boost: 3, seedBase: 1, count: 0}
	var rest []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		key := strings.TrimLeft(a, "-")
		val := ""
		if k, v, ok := strings.Cut(key, "="); ok {
			key, val = k, v
		} else if isFlockFlag(key) {
			if i+1 >= len(args) {
				return o, nil, fmt.Errorf("--%s needs a value", key)
			}
			i++
			val = args[i]
		} else {
			rest = append(rest, a)
			continue
		}
		switch key {
		case "count":
			n, err := strconv.Atoi(val)
			if err != nil || n < 1 {
				return o, nil, fmt.Errorf("bad --count %q", val)
			}
			o.count = n
		case "seed-base":
			n, err := strconv.ParseUint(val, 10, 64)
			if err != nil {
				return o, nil, fmt.Errorf("bad --seed-base %q", val)
			}
			o.seedBase = n
		case "from":
			o.from = val
		case "likes":
			ids, err := parseSeeds(val)
			if err != nil {
				return o, nil, fmt.Errorf("bad --likes: %w", err)
			}
			o.likes = ids
		case "boost":
			f, err := strconv.ParseFloat(val, 64)
			if err != nil || f < 0 {
				return o, nil, fmt.Errorf("bad --boost %q", val)
			}
			o.boost = f
		case "out":
			o.out = val
		case "cols":
			o.cols, _ = strconv.Atoi(val)
		case "cell":
			o.cell, _ = strconv.Atoi(val)
		case "jobs":
			o.jobs, _ = strconv.Atoi(val)
		default:
			rest = append(rest, a)
		}
	}
	return o, rest, nil
}

func isFlockFlag(k string) bool {
	switch k {
	case "count", "seed-base", "from", "likes", "boost", "out", "cols", "cell", "jobs":
		return true
	}
	return false
}

// configureFlockSketch applies sketch-owned flags from rest so Traits()
// reflects CLI overrides when planning explore/breed rows.
func configureFlockSketch(s sketch.Sketch, rest []string) error {
	c, ok := s.(sketch.Configurable)
	if !ok {
		return nil
	}
	fs := flag.NewFlagSet("flock-plan", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	c.Flags(fs)
	known := map[string]bool{}
	fs.VisitAll(func(f *flag.Flag) { known[f.Name] = true })
	var filtered []string
	for i := 0; i < len(rest); i++ {
		a := rest[i]
		key := strings.TrimLeft(a, "-")
		if k, v, ok := strings.Cut(key, "="); ok {
			if known[k] {
				filtered = append(filtered, "--"+k, v)
			}
			continue
		}
		if !known[key] {
			continue
		}
		filtered = append(filtered, a)
		if i+1 < len(rest) && !strings.HasPrefix(rest[i+1], "-") {
			i++
			filtered = append(filtered, rest[i])
		}
	}
	if err := fs.Parse(filtered); err != nil {
		return err
	}
	_, err := c.Configure()
	return err
}

// overlayCLITraits copies traits and applies --dim value pairs from rest so
// flock.jsonl matches what was actually rendered (e.g. medium=wash).
func overlayCLITraits(traits map[string]string, rest []string, schema trait.Schema) map[string]string {
	out := make(map[string]string, len(traits)+len(schema))
	for k, v := range traits {
		out[k] = v
	}
	known := map[string]bool{}
	for _, d := range schema {
		known[d.Name] = true
	}
	for i := 0; i < len(rest); i++ {
		a := rest[i]
		key := strings.TrimLeft(a, "-")
		if k, v, ok := strings.Cut(key, "="); ok {
			if known[k] {
				out[k] = v
			}
			continue
		}
		if !known[key] {
			continue
		}
		if i+1 < len(rest) && !strings.HasPrefix(rest[i+1], "-") {
			i++
			out[key] = rest[i]
		}
	}
	return out
}

func planFlock(t sketch.Traited, o flockOpts) ([]flockMember, error) {
	if o.from == "" {
		return planExplore(t, o)
	}
	return planBreed(t, o)
}

func planExplore(t sketch.Traited, o flockOpts) ([]flockMember, error) {
	out := make([]flockMember, o.count)
	for i := 0; i < o.count; i++ {
		seed := o.seedBase + uint64(i)
		set := t.Traits(sketch.Context{Width: 1, Height: 1, Seed: seed})
		out[i] = flockMember{seed: seed, traits: set, mode: "explore"}
	}
	return out, nil
}

func planBreed(t sketch.Traited, o flockOpts) ([]flockMember, error) {
	if len(o.likes) == 0 {
		return nil, fmt.Errorf("breed needs --likes (seed list from the parent flock)")
	}
	parentPath := o.from
	if st, err := os.Stat(parentPath); err == nil && st.IsDir() {
		parentPath = filepath.Join(parentPath, "flock.jsonl")
	}
	entries, err := loadFlock(parentPath)
	if err != nil {
		return nil, err
	}
	bySeed := make(map[uint64]flockEntry, len(entries))
	for _, e := range entries {
		bySeed[e.Seed] = e
	}
	liked := make([]flockEntry, 0, len(o.likes))
	likedSets := make([]trait.Set, 0, len(o.likes))
	for _, id := range o.likes {
		e, ok := bySeed[id]
		if !ok {
			return nil, fmt.Errorf("liked seed %d is not in %s", id, parentPath)
		}
		liked = append(liked, e)
		likedSets = append(likedSets, trait.Set(e.Traits))
	}

	boosted := t.Schema().Boost(likedSets, o.boost)
	nBoost := o.count / 2
	nNeigh := o.count - nBoost
	out := make([]flockMember, 0, o.count)

	for i := 0; i < nBoost; i++ {
		seed := o.seedBase + uint64(i)
		set := boosted.Derive(rand.New(rand.NewPCG(seed, 2)))
		out = append(out, flockMember{
			seed: seed, traits: set, mode: "boosted",
			pins: pinArgs(t.Schema(), set),
		})
	}
	for i := 0; i < nNeigh; i++ {
		parent := liked[i%len(liked)]
		seed := parent.Seed + uint64(i+1)*neighborStride
		set := trait.Set(parent.Traits)
		out = append(out, flockMember{
			seed: seed, traits: set, mode: "neighbor", parent: parent.Seed,
			pins: pinArgs(t.Schema(), set),
		})
	}
	return out, nil
}

func pinArgs(schema trait.Schema, set trait.Set) []string {
	args := make([]string, 0, len(schema)*2)
	for _, d := range schema {
		if v := set.Get(d.Name); v != "" {
			args = append(args, "--"+d.Name, v)
		}
	}
	return args
}

func loadFlock(path string) ([]flockEntry, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var out []flockEntry
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		var e flockEntry
		if err := json.Unmarshal([]byte(line), &e); err != nil {
			return nil, fmt.Errorf("%s: %w", path, err)
		}
		out = append(out, e)
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("%s: empty flock", path)
	}
	return out, nil
}
