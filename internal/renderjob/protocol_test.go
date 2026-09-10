package renderjob_test

import (
	"bytes"
	"context"
	"io"
	"os"
	"testing"
	"time"

	"github.com/jaminalder/go-graphics/internal/publish"
	"github.com/jaminalder/go-graphics/internal/trait"

	"github.com/jaminalder/go-graphics/internal/renderjob"
)

// TestSupervisorRejectsUnknownBuildBeforeStartingChild protects release identity.
func TestSupervisorRejectsUnknownBuildBeforeStartingChild(t *testing.T) {
	s := renderjob.Supervisor{Executable: "/must-not-run", Build: "current"}
	var b bytes.Buffer
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := s.Render(ctx, renderjob.Request{Build: "old"}, &b); err == nil {
		t.Fatal("accepted old release")
	}
}

// TestMain supplies a controlled disposable child to exercise actual OS termination.
func TestMain(m *testing.M) {
	if len(os.Args) == 2 && os.Args[1] == "--child" {
		switch os.Getenv("ART_TEST_CHILD") {
		case "hang":
			time.Sleep(time.Hour)
		case "panic":
			panic("controlled test panic")
		case "overflow":
			_, _ = os.Stdout.Write(make([]byte, 17<<20))
		}
		os.Exit(0)
	}
	os.Exit(m.Run())
}

// TestSupervisorReapsHungPanickingAndOversizedChildren defends the hard isolation boundary.
func TestSupervisorReapsHungPanickingAndOversizedChildren(t *testing.T) {
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	r, err := publish.Complete("iris", 42, "diebenkorn-seawall", trait.Set{})
	if err != nil {
		t.Fatal(err)
	}
	q := renderjob.Request{Version: 1, Build: "test", Recipe: r.Bytes(), Tier: "preview"}
	for _, mode := range []string{"hang", "panic", "overflow"} {
		t.Run(mode, func(t *testing.T) {
			t.Setenv("ART_TEST_CHILD", mode)
			s := renderjob.Supervisor{Executable: exe, Build: "test"}
			ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
			defer cancel()
			start := time.Now()
			if err := s.Render(ctx, q, io.Discard); err == nil {
				t.Fatal("accepted failed child")
			}
			if time.Since(start) > 2*time.Second {
				t.Fatal("child was not reaped promptly")
			}
		})
	}
}
