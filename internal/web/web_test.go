package web_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
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

// TestCookieMutationsRejectForgeryDuplicatesAndStaleRevisions defends the ordinary HTTP form contract.
func TestCookieMutationsRejectForgeryDuplicatesAndStaleRevisions(t *testing.T) {
	store := studio.New(nil, "test")
	session, e := store.Create()
	if e != nil {
		t.Fatal(e)
	}
	x, e := store.Start(session.Token, "iris", "fine", "")
	if e != nil {
		t.Fatal(e)
	}
	h, e := web.New(web.Config{Origin: "http://example.test", Studio: store})
	if e != nil {
		t.Fatal(e)
	}
	tests := []struct {
		body, origin string
		want         int
	}{{"csrf=" + session.CSRF + "&revision=0&style=winding&colour=", "http://evil.test", 403}, {"csrf=bad&revision=0&style=winding&colour=", "http://example.test", 403}, {"csrf=" + session.CSRF + "&revision=0&revision=1&style=winding&colour=", "http://example.test", 400}, {"csrf=" + session.CSRF + "&revision=99&style=winding&colour=", "http://example.test", 409}, {"csrf=" + session.CSRF + "&revision=0&style=winding&colour=&width=99999", "http://example.test", 400}}
	for _, test := range tests {
		r := httptest.NewRequest("POST", "http://example.test/explorations/"+x.ID+"/choices", strings.NewReader(test.body))
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		r.Header.Set("Origin", test.origin)
		r.AddCookie(&http.Cookie{Name: "art-studio", Value: session.Token})
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != test.want {
			t.Fatal(w.Code, test.want, w.Body.String())
		}
	}
	latest, e := store.Get(session.Token, x.ID)
	if e != nil || latest.Revision != 0 {
		t.Fatal("rejected action changed state")
	}
}

// TestReadOnlyMethodsCannotAdmitRendersOrExposeAnotherWorkspace covers hostile URL and method entry points.
func TestReadOnlyMethodsCannotAdmitRendersOrExposeAnotherWorkspace(t *testing.T) {
	store := studio.New(nil, "test")
	a, _ := store.Create()
	x, _ := store.Start(a.Token, "iris", "", "")
	b, _ := store.Create()
	h, e := web.New(web.Config{Origin: "http://example.test", Studio: store})
	if e != nil {
		t.Fatal(e)
	}
	for _, method := range []string{"GET", "HEAD"} {
		for _, path := range []string{"/images/" + strings.Repeat("a", 64), "/explorations/" + x.ID, "/explorations/" + x.ID + "/batches", "/assets/../private"} {
			r := httptest.NewRequest(method, "http://example.test"+path, nil)
			r.AddCookie(&http.Cookie{Name: "art-studio", Value: b.Token})
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			if w.Code < 400 {
				t.Fatal(method, path, w.Code)
			}
		}
	}
}
