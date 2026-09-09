package artwork_test

import (
	"bytes"
	"flag"
	"image/png"
	"strconv"
	"sync"
	"testing"

	"github.com/jaminalder/go-graphics/internal/palette"
	"github.com/jaminalder/go-graphics/internal/sketch"
	"github.com/jaminalder/go-graphics/internal/sketch/iris"
	"github.com/jaminalder/go-graphics/internal/sketch/pools"
	"github.com/jaminalder/go-graphics/internal/trait"

	"github.com/jaminalder/go-graphics/internal/artwork"
)

// TestCanonicalRecipesRejectAmbiguity protects persisted recipes from reinterpretation.
func TestCanonicalRecipesRejectAmbiguity(t *testing.T) {
	raw := []byte(`{"recipe_version":1,"artwork_id":"iris","edition":"1","seed":"18446744073709551615","palette":"hokusai-great-wave","config":{"traits":{}}}`)
	r, e := artwork.Decode(raw)
	if e != nil {
		t.Fatal(e)
	}
	b := r.Bytes()
	r2, e := artwork.Decode(b)
	if e != nil || !bytes.Equal(b, r2.Bytes()) {
		t.Fatal(e)
	}
	for _, bad := range []string{`{"seed":"1","seed":"2"}`, string(raw) + `{}`, string(bytes.Replace(raw, []byte(`"seed"`), []byte(`"Seed"`), 1)), string(bytes.Replace(raw, []byte(`"traits":{}`), []byte(`"unknown":1`), 1))} {
		if _, e := artwork.Decode([]byte(bad)); e == nil {
			t.Fatal(bad)
		}
	}
}

// TestRecipeRenderingMatchesCLIAndKeepsOverridePresence defends both raster and painted consumers.
func TestRecipeRenderingMatchesCLIAndKeepsOverridePresence(t *testing.T) {
	for _, id := range []string{"iris", "pools"} {
		t.Run(id, func(t *testing.T) {
			s, _ := artwork.Registry().Get(id)
			c := s.(sketch.Configurable)
			fs := flag.NewFlagSet(id, flag.ContinueOnError)
			c.Flags(fs)
			args := []string{"--colourway", "from-flag", "--alpha", "0.74"}
			config := any(pools.Config{Traits: trait.Set{"colourway": "from-flag"}, Alpha: ptr(0.74)})
			if id == "iris" {
				value := iris.New().Scale
				args = []string{"--scale", strconv.FormatFloat(value, 'g', -1, 64)}
				config = iris.Config{Scale: &value}
			}
			if err := fs.Parse(args); err != nil {
				t.Fatal(err)
			}
			if _, err := c.Configure(); err != nil {
				t.Fatal(err)
			}
			pal, _ := palette.ByName("hokusai-great-wave")
			expected, err := s.Render(sketch.Context{Width: 96, Height: 96, Seed: 42, Palette: pal, AA: 1})
			if err != nil {
				t.Fatal(err)
			}
			r, err := artwork.New(id, 42, "hokusai-great-wave", config)
			if err != nil {
				t.Fatal(err)
			}
			round, err := artwork.Decode(r.Bytes())
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(r.Bytes(), round.Bytes()) {
				t.Fatal("override lost")
			}
			var encoded bytes.Buffer
			if err := round.Render(&encoded, artwork.Rendition{Width: 96, Height: 96, AA: 1, Format: "png"}, "test"); err != nil {
				t.Fatal(err)
			}
			actual, err := png.Decode(&encoded)
			if err != nil {
				t.Fatal(err)
			}
			for y := 0; y < 96; y++ {
				for x := 0; x < 96; x++ {
					ar, ag, ab, aa := actual.At(x, y).RGBA()
					er, eg, eb, ea := expected.At(x, y).RGBA()
					if ar != er || ag != eg || ab != eb || aa != ea {
						t.Fatalf("CLI pixels moved at %d,%d", x, y)
					}
				}
			}
		})
	}
}
func ptr(v float64) *float64 { return &v }

// TestConcurrentRecipesAreIndependentAndKeysBindOutput defends immutable ownership and cache identity.
func TestConcurrentRecipesAreIndependentAndKeysBindOutput(t *testing.T) {
	set := trait.Set{"weave": "radial"}
	a, err := artwork.New("iris", 42, "hokusai-great-wave", iris.Config{Traits: set})
	if err != nil {
		t.Fatal(err)
	}
	set["weave"] = "swirl"
	b, err := artwork.New("iris", 42, "hokusai-great-wave", iris.Config{Traits: set})
	if err != nil {
		t.Fatal(err)
	}
	tier := artwork.Rendition{Width: 48, Height: 48, AA: 1, Format: "png"}
	var expected [2][]byte
	for i, r := range []artwork.Recipe{a, b} {
		var out bytes.Buffer
		if err := r.Render(&out, tier, "build-a"); err != nil {
			t.Fatal(err)
		}
		expected[i] = out.Bytes()
	}
	if bytes.Equal(expected[0], expected[1]) {
		t.Fatal("fixture does not distinguish configuration")
	}
	var wg sync.WaitGroup
	for i := range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r := []artwork.Recipe{a, b}[i%2]
			var out bytes.Buffer
			if err := r.Render(&out, tier, "build-a"); err != nil {
				t.Error(err)
			}
			if !bytes.Equal(out.Bytes(), expected[i%2]) {
				t.Error("cross-job contamination")
			}
		}()
	}
	wg.Wait()
	changed := tier
	changed.Width++
	if a.Key(tier, "build-a") == a.Key(changed, "build-a") || a.Key(tier, "build-a") == a.Key(tier, "build-b") {
		t.Fatal("rendition/build omitted from identity")
	}
}
