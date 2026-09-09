package artwork_test

import (
	"bytes"
	"testing"

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
	for _, bad := range []string{`{"seed":"1","seed":"2"}`, string(raw) + `{}`, string(bytes.Replace(raw, []byte(`"traits":{}`), []byte(`"unknown":1`), 1))} {
		if _, e := artwork.Decode([]byte(bad)); e == nil {
			t.Fatal(bad)
		}
	}
}
