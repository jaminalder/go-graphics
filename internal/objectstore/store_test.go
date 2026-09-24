package objectstore_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/jaminalder/go-graphics/internal/objectstore"
)

// TestObjectReadDistinguishesLossOutageAndCorruption prevents false expiry and partial image responses.
func TestObjectReadDistinguishesLossOutageAndCorruption(t *testing.T) {
	for _, test := range []struct {
		name             string
		status           int
		body             string
		missing, success bool
	}{
		{"complete", 200, "image", false, true},
		{"missing", 404, `<Error><Code>NoSuchKey</Code></Error>`, true, false},
		{"denied", 403, `<Error><Code>AccessDenied</Code></Error>`, false, false},
		{"unavailable", 503, `<Error><Code>ServiceUnavailable</Code></Error>`, false, false},
		{"truncated", 200, "ima", false, false},
		{"corrupt", 200, "wrong", false, false},
		{"oversized", 200, "image-extra", false, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/images/prefix/key.png" {
					t.Errorf("unexpected path %s", r.URL.Path)
				}
				w.WriteHeader(test.status)
				_, _ = w.Write([]byte(test.body))
			}))
			defer server.Close()
			store, err := objectstore.New(objectstore.Config{Endpoint: server.URL, Region: "local", Bucket: "images", Prefix: "prefix", AccessKey: "test", SecretKey: "test", Local: true})
			if err != nil {
				t.Fatal(err)
			}
			sum := sha256.Sum256([]byte("image"))
			data, err := store.Get(context.Background(), "key.png", hex.EncodeToString(sum[:]), 5)
			if (err == nil) != test.success || errors.Is(err, os.ErrNotExist) != test.missing {
				t.Fatalf("data=%q err=%v", data, err)
			}
		})
	}
}

// TestObjectConfigurationRejectsUnsafeTransportAndKeys protects prefix and credential boundaries.
func TestObjectConfigurationRejectsUnsafeTransportAndKeys(t *testing.T) {
	base := objectstore.Config{Endpoint: "https://objects.example", Region: "test", Bucket: "images", Prefix: "prefix", AccessKey: "key", SecretKey: "secret"}
	for _, endpoint := range []string{"http://objects.example", "https://key:secret@objects.example", "https://objects.example?query=1"} {
		c := base
		c.Endpoint = endpoint
		if _, err := objectstore.New(c); err == nil {
			t.Fatalf("accepted %s", endpoint)
		}
	}
	store, err := objectstore.New(base)
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"../other", "/absolute", "a?x=y", ""} {
		if err := store.Put(context.Background(), key, []byte("x")); err == nil {
			t.Fatalf("accepted %s", key)
		}
	}
}
