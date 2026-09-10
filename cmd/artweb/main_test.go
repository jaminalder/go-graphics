package main

import "testing"

// A public container listener must never silently trust every forwarding peer.
func TestContainerListenerRequiresOneExplicitProxy(t *testing.T) {
	for _, tc := range []struct {
		addr, proxy string
		valid       bool
	}{
		{"127.0.0.1:8080", "", true},
		{"0.0.0.0:8080", "", false},
		{"0.0.0.0:8080", "172.30.80.2", true},
		{"0.0.0.0:8080", "0.0.0.0/0", false},
		{"0.0.0.0:8080", "0.0.0.0", false},
		{"0.0.0.0:8080", "caddy", false},
		{"0.0.0.0:8080", "ff02::1", false},
	} {
		t.Run(tc.addr+"/"+tc.proxy, func(t *testing.T) {
			_, err := listenerProxy(tc.addr, tc.proxy)
			if (err == nil) != tc.valid {
				t.Fatalf("valid=%v, error=%v", tc.valid, err)
			}
		})
	}
}
