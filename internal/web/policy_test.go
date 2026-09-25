package web

import (
	"net/http/httptest"
	"testing"

	"github.com/jaminalder/go-graphics/internal/limits"
	"github.com/jaminalder/go-graphics/internal/studio"
)

func TestWebEnforcesConfiguredReadBurstAndReportsReason(t *testing.T) {
	p, _ := limits.Parse("production", `{"read_ip":{"per_minute":1,"burst":2}}`)
	h, err := New(Config{Origin: "http://example.test", Studio: studio.New(nil, "test"), Limits: p})
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 3; i++ {
		r := httptest.NewRequest("GET", "http://example.test/", nil)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if i < 2 && w.Code != 200 {
			t.Fatal("early throttle")
		}
		if i == 2 && (w.Code != 429 || w.Header().Get("X-Art-Limit") != "read-ip") {
			t.Fatal("missing rate reason")
		}
	}
}
