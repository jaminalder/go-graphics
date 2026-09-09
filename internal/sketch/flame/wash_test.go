package flame

import (
	"bytes"
	"testing"

	"github.com/jaminalder/go-graphics/internal/sketch/sketchtest"
)

// TestMediumIsOptInOnly is the contract that lets wash exist without moving
// any seed: Derive never lands on weight-0 wash; --medium wash is deliberate.
func TestMediumIsOptInOnly(t *testing.T) {
	for seed := uint64(1); seed <= 80; seed++ {
		if got := New().Traits(testCtx(t, seed)).Get(dimMedium); got != "ember" {
			t.Fatalf("seed %d resolved to medium %q — wash is not opt-in", seed, got)
		}
	}
	if got := configured(t, "--medium", mediumWash).Traits(testCtx(t, 1)).Get(dimMedium); got != mediumWash {
		t.Fatalf("--medium wash resolved to %q", got)
	}
}

// TestWashDiffersFromEmber pins that the medium is not a no-op rename of
// paper-ground ember: absorption + tooth must move the pixels.
func TestWashDiffersFromEmber(t *testing.T) {
	ctx := testCtx(t, 26)
	args := []string{"--quality", "6", "--estimator", "0", "--oversample", "1", "--structure", "spindle", "--tint", "split", "--cast", "zander-spindle"}
	ember := sketchtest.RenderNRGBA(t, configured(t, append(args, "--ground", "paper")...), ctx)
	wash := sketchtest.RenderNRGBA(t, configured(t, append(args, "--medium", mediumWash)...), ctx)
	if bytes.Equal(ember.Pix, wash.Pix) {
		t.Fatal("wash medium painted the same image as ember on paper")
	}
}

func TestWashIsDeterministic(t *testing.T) {
	s := configured(t, "--medium", mediumWash, "--manner", mannerStain, "--quality", "8", "--estimator", "3", "--oversample", "2", "--structure", "spindle", "--cast", "zander-spindle")
	sketchtest.AssertDeterministic(t, s, testCtx(t, 26), testCtx(t, 100029))
}

// TestWashForcesPaperGround: void/dusk are nebula grounds; wash is pigment
// on paper even if --ground void is left on the command line.
func TestWashForcesPaperGround(t *testing.T) {
	ctx := testCtx(t, 5)
	rec := configured(t, "--medium", mediumWash, "--ground", "void").pin(draw(
		configured(t, "--medium", mediumWash, "--ground", "void").Traits(ctx),
		ctx.RNG(streamRecipe),
	))
	if rec.ground != groundPaper {
		t.Fatalf("wash ground %v, want paper", rec.ground)
	}
	if rec.gleam != 0 {
		t.Fatalf("wash gleam %g, want 0", rec.gleam)
	}
}
