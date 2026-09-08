package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/jaminalder/go-graphics/internal/trait"
)

func TestSplitFlockArgs(t *testing.T) {
	o, rest, err := splitFlockArgs([]string{
		"--count", "12", "--seed-base", "5", "--from", "out/f1",
		"--likes", "3,7", "--boost", "2.5", "--out", "tmp",
		"--quality", "24",
	})
	if err != nil {
		t.Fatal(err)
	}
	if o.count != 12 || o.seedBase != 5 || o.from != "out/f1" || o.boost != 2.5 || o.out != "tmp" {
		t.Fatalf("opts %+v", o)
	}
	if len(o.likes) != 2 || o.likes[0] != 3 || o.likes[1] != 7 {
		t.Fatalf("likes %v", o.likes)
	}
	if len(rest) != 2 || rest[0] != "--quality" || rest[1] != "24" {
		t.Fatalf("rest %v", rest)
	}
}

func TestPinArgsCoversSchema(t *testing.T) {
	schema := trait.Schema{
		{Name: "structure", Key: "s", Values: []trait.Value{{Name: "spindle", Weight: 1}}},
		{Name: "cast", Key: "c", Values: []trait.Value{{Name: "zander-spindle", Weight: 1}}},
	}
	got := pinArgs(schema, trait.Set{"structure": "spindle", "cast": "zander-spindle"})
	want := []string{"--structure", "spindle", "--cast", "zander-spindle"}
	if len(got) != len(want) {
		t.Fatalf("got %v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v want %v", got, want)
		}
	}
}

func TestLoadFlockRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "flock.jsonl")
	body := `{"seed":7,"traits":{"structure":"spindle"},"file":"01_x.png","mode":"explore"}
{"seed":11,"traits":{"structure":"filament"},"file":"02_y.png","mode":"explore"}
`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := loadFlock(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].Seed != 7 || got[1].Traits["structure"] != "filament" {
		t.Fatalf("%+v", got)
	}
}
