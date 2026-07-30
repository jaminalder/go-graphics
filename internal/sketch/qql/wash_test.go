package qql

import (
	"bytes"
	"testing"

	"github.com/jaminalder/go-graphics/internal/sketch"
	"github.com/jaminalder/go-graphics/internal/sketch/sketchtest"
)

// TestMediumIsOptInOnly is the contract that lets the medium exist in the
// schema at all: QQL's output space is a port, and a seed must land where
// it always did. The wash is reachable only by asking for it.
func TestMediumIsOptInOnly(t *testing.T) {
	for seed := uint64(1); seed <= 40; seed++ {
		tr := configured(t).Traits(testCtx(t, seed))
		if got := tr.Get(dimMedium); got != "ink" {
			t.Fatalf("seed %d resolved to medium %q — the wash is not opt-in", seed, got)
		}
	}
	if got := configured(t, "--medium", "wash").Traits(testCtx(t, 1)).Get(dimMedium); got != mediumWash {
		t.Fatalf("--medium wash resolved to %q", got)
	}
}

// TestMediumChangesOnlyThePaint pins the whole design of the feature. The
// medium decides what the bands are laid with; where they go is QQL's
// business and must be identical either way, so the same seed is the same
// composition in either material.
func TestMediumChangesOnlyThePaint(t *testing.T) {
	ctx := testCtx(t, 42)
	ink, err := configured(t).plan(ctx)
	if err != nil {
		t.Fatal(err)
	}
	wash, err := configured(t, "--medium", "wash").plan(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(ink.dots) != len(wash.dots) {
		t.Fatalf("ink laid %d dots, wash laid %d", len(ink.dots), len(wash.dots))
	}
	for i := range ink.dots {
		a, b := ink.dots[i], wash.dots[i]
		if a.x != b.x || a.y != b.y || a.scale != b.scale || a.rings != b.rings || a.density != b.density {
			t.Fatalf("dot %d differs between media: %+v vs %+v", i, a, b)
		}
	}
	if ink.scheme.Background != wash.scheme.Background {
		t.Error("the medium moved the ground")
	}
}

func TestWashRendersAndDiffersFromInk(t *testing.T) {
	// Large sparse dots: the medium only shows where a band has room to
	// hold a pool, which is exactly what this pairing gives it.
	args := []string{"--ring-size", "large", "--spacing", "sparse"}
	ctx := sketch.Context{Width: 128, Height: 160, Seed: 5, Palette: testCtx(t, 5).Palette}

	ink := sketchtest.RenderNRGBA(t, configured(t, args...), ctx)
	wash := sketchtest.RenderNRGBA(t, configured(t, append(args, "--medium", "wash")...), ctx)
	if bytes.Equal(ink.Pix, wash.Pix) {
		t.Fatal("the wash medium painted the same image as ink")
	}
	sketchtest.AssertDeterministic(t,
		configured(t, append(args, "--medium", "wash")...), ctx, testCtx(t, 6))
}
