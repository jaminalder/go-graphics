package web_test

import (
	"net/http/httptest"
	"testing"

	"github.com/jaminalder/go-graphics/internal/studio"
	"github.com/jaminalder/go-graphics/internal/web"
)

// TestGalleryIsCheapAndRejectsHostSpoofing defends anonymous read safety.
func TestGalleryIsCheapAndRejectsHostSpoofing(t *testing.T) {
	h, e := web.New(web.Config{Origin: "http://example.test", Studio: studio.New(nil, "test")})
	if e != nil {
		t.Fatal(e)
	}
	for _, host := range []string{"example.test", "evil.test"} {
		r := httptest.NewRequest("GET", "http://"+host+"/", nil)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if host == "example.test" {
			if w.Code != 200 || len(w.Result().Cookies()) != 0 {
				t.Fatal(w.Code)
			}
		} else if w.Code != 421 {
			t.Fatal(w.Code)
		}
	}
}
