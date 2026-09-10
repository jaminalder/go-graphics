package web

import (
	"net/http/httptest"
	"net/netip"
	"testing"
)

// Another container or a visitor must not manufacture a fresh admission identity.
func TestOnlyTheConfiguredProxyCanSupplyClientIdentity(t *testing.T) {
	for _, tc := range []struct{ peer, header, want string }{
		{"172.30.80.2:1234", "203.0.113.7", "203.0.113.7"},
		{"172.30.80.4:1234", "203.0.113.7", "172.30.80.4"},
		{"127.0.0.1:1234", "203.0.113.7", "127.0.0.1"},
		{"172.30.80.2:1234", "bad, 203.0.113.7", "172.30.80.2"},
		{"[::ffff:172.30.80.2]:1234", "::ffff:203.0.113.7", "203.0.113.7"},
	} {
		r := httptest.NewRequest("GET", "/", nil)
		r.RemoteAddr = tc.peer
		r.Header.Set("X-Art-Client", tc.header)
		r.Header.Set("X-Forwarded-For", "192.0.2.99")
		a := app{cfg: Config{TrustedProxy: netip.MustParseAddr("172.30.80.2")}}
		if got := a.client(r); got != tc.want {
			t.Errorf("peer %s: got %s, want %s", tc.peer, got, tc.want)
		}
		a.cfg.TrustedProxy = netip.Addr{}
		if got := a.client(r); got == "203.0.113.7" {
			t.Error("unconfigured proxy trusted")
		}
	}
}
