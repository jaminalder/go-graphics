package main

import (
	"errors"
	"fmt"
	"image"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"

	"github.com/jaminalder/go-graphics/internal/render"
)

// Sweeping is how these sketches are actually judged.
//
// A change to a sketch is not judged by one render: a seed is one point in
// an output space, and the question is always what the *space* looks like
// now. That means twenty renders and a way to see them at once — which was
// being done with throwaway shell scripts and a scratch montage tool, badly
// and differently every time. This is that loop, made repeatable: a seed
// range or a parameter grid in, a directory of images and one contact sheet
// out, with a manifest saying which tile is which.

// sweepLimit caps a run. It is a guard against a typo in a --vary list
// turning into a thousand print-size renders, not a considered maximum.
const sweepLimit = 240

func runSweep(args []string) error {
	if len(args) == 0 || strings.HasPrefix(args[0], "-") {
		return fmt.Errorf("sweep needs a sketch name (try: staticart list)")
	}
	name := args[0]

	// Sweep's own flags are stripped out here; everything else is passed
	// through to render untouched, so any flag a sketch has is sweepable
	// without this command knowing about it.
	opts, rest, err := splitSweepArgs(args[1:])
	if err != nil {
		return err
	}

	combos, labels, err := expand(opts)
	if err != nil {
		return err
	}
	if len(combos) > sweepLimit {
		return fmt.Errorf("that is %d renders; %d is the cap (narrow --seeds or --vary)", len(combos), sweepLimit)
	}

	if err := os.MkdirAll(opts.out, 0o755); err != nil {
		return err
	}

	type result struct {
		img   image.Image
		path  string
		label string
	}
	results := make([]result, len(combos))
	errs := make([]error, len(combos))

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
				full := append([]string{name}, rest...)
				full = append(full, combos[i]...)
				full = append(full, "--out", opts.out)
				out, err := renderOne(full)
				if err != nil {
					errs[i] = err
					continue
				}
				// Tiles are numbered so the sheet and the directory agree;
				// the recipe stays in the file's own metadata.
				out.path = filepath.Join(opts.out,
					fmt.Sprintf("%02d_%s", i+1, filepath.Base(out.path)))
				if err := writeRender(out); err != nil {
					errs[i] = err
					continue
				}
				results[i] = result{out.img, out.path, labels[i]}
			}
		}()
	}
	for i := range combos {
		queue <- i
	}
	close(queue)
	wg.Wait()

	if err := errors.Join(errs...); err != nil {
		return err
	}

	var man strings.Builder
	imgs := make([]image.Image, 0, len(results))
	for i, r := range results {
		imgs = append(imgs, r.img)
		fmt.Fprintf(&man, "%02d  %-52s %s\n", i+1, filepath.Base(r.path), r.label)
	}
	if err := os.WriteFile(filepath.Join(opts.out, "manifest.txt"), []byte(man.String()), 0o644); err != nil {
		return err
	}

	cols := opts.cols
	if cols < 1 {
		// A roughly square sheet, which is what fits on a screen.
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
	fmt.Printf("%d renders in %s\n%s\n", len(imgs), opts.out, sheet)
	return nil
}

// sweepOpts is what sweep itself consumes; everything else goes to render.
type sweepOpts struct {
	seeds []uint64
	vary  []varied
	out   string
	cols  int
	cell  int
	jobs  int
}

// varied is one flag and the values to walk it through.
type varied struct {
	flag   string
	values []string
}

func splitSweepArgs(args []string) (sweepOpts, []string, error) {
	o := sweepOpts{out: "out/sweep", cell: 300}
	var rest []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		key := strings.TrimLeft(a, "-")
		val := ""
		if k, v, ok := strings.Cut(key, "="); ok {
			key, val = k, v
		} else if isSweepFlag(key) {
			if i+1 >= len(args) {
				return o, nil, fmt.Errorf("--%s needs a value", key)
			}
			i++
			val = args[i]
		}
		switch key {
		case "seeds":
			s, err := parseSeeds(val)
			if err != nil {
				return o, nil, err
			}
			o.seeds = s
		case "vary":
			f, vs, ok := strings.Cut(val, "=")
			if !ok || f == "" || vs == "" {
				return o, nil, fmt.Errorf("--vary wants flag=v1,v2 (got %q)", val)
			}
			o.vary = append(o.vary, varied{f, strings.Split(vs, ",")})
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

func isSweepFlag(k string) bool {
	switch k {
	case "seeds", "vary", "out", "cols", "cell", "jobs":
		return true
	}
	return false
}

// parseSeeds reads "1-20", "3,7,11" or a mix of both.
func parseSeeds(s string) ([]uint64, error) {
	var out []uint64
	for _, part := range strings.Split(s, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if lo, hi, ok := strings.Cut(part, "-"); ok {
			a, err1 := strconv.ParseUint(lo, 10, 64)
			b, err2 := strconv.ParseUint(hi, 10, 64)
			if err1 != nil || err2 != nil || b < a {
				return nil, fmt.Errorf("bad seed range %q", part)
			}
			for v := a; v <= b; v++ {
				out = append(out, v)
			}
			continue
		}
		v, err := strconv.ParseUint(part, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("bad seed %q", part)
		}
		out = append(out, v)
	}
	return out, nil
}

// expand builds the cartesian product of the seeds and every --vary list,
// as argument slices plus a human label per combination.
func expand(o sweepOpts) ([][]string, []string, error) {
	axes := make([]varied, 0, len(o.vary)+1)
	if len(o.seeds) > 0 {
		vals := make([]string, len(o.seeds))
		for i, s := range o.seeds {
			vals[i] = strconv.FormatUint(s, 10)
		}
		axes = append(axes, varied{"seed", vals})
	}
	axes = append(axes, o.vary...)
	if len(axes) == 0 {
		return nil, nil, errors.New("sweep needs --seeds or --vary")
	}

	combos := [][]string{{}}
	labels := []string{""}
	for _, ax := range axes {
		var nc [][]string
		var nl []string
		for i, c := range combos {
			for _, v := range ax.values {
				nc = append(nc, append(append([]string{}, c...), "--"+ax.flag, v))
				nl = append(nl, strings.TrimSpace(labels[i]+" "+ax.flag+"="+v))
			}
		}
		combos, labels = nc, nl
	}
	return combos, labels, nil
}
