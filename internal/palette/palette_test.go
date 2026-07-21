package palette

import "testing"

func TestDatasetIntegrity(t *testing.T) {
	all := All()
	if len(all) < 100 {
		t.Fatalf("expected full ColorLisa dataset, got %d palettes", len(all))
	}
	seen := map[string]bool{}
	for _, p := range all {
		if p.Slug == "" || seen[p.Slug] {
			t.Errorf("empty or duplicate slug %q", p.Slug)
		}
		seen[p.Slug] = true
		if len(p.Colors) != 5 {
			t.Errorf("palette %s has %d colors, want 5", p.Slug, len(p.Colors))
		}
		if p.Artist == "" {
			t.Errorf("palette %s has no artist", p.Slug)
		}
	}
}

func TestByName(t *testing.T) {
	p, ok := ByName("staticart-seven")
	if !ok {
		t.Fatal("staticart-seven palette missing")
	}
	if got := p.Colors[0].Hex(); got != "#ED6A5A" {
		t.Errorf("staticart-seven[0] = %s, want #ED6A5A", got)
	}
	if _, ok := ByName("does-not-exist"); ok {
		t.Error("ByName returned ok for unknown slug")
	}
}

func TestColorWraps(t *testing.T) {
	p, _ := ByName("hokusai-great-wave")
	if p.Color(5) != p.Color(0) {
		t.Error("Color(5) should wrap to Color(0)")
	}
	if p.Color(-1) != p.Color(4) {
		t.Error("Color(-1) should wrap to Color(4)")
	}
}

func TestDesaturatedDoesNotMutate(t *testing.T) {
	p, _ := ByName("monet-water-lilies")
	before := p.Colors[0]
	_ = p.Desaturated(1)
	after, _ := ByName("monet-water-lilies")
	if after.Colors[0] != before {
		t.Error("Desaturated mutated the source palette data")
	}
}
