package web

import (
	"context"
	"testing"
	"time"

	"golang.org/x/sync/semaphore"

	"github.com/jaminalder/go-graphics/internal/limits"
)

func TestImageBudgetBoundsBytesNotJustImageCount(t *testing.T) {
	p, _ := limits.Defaults("production")
	p.ImageWaitMS = 0
	a := &app{cfg: Config{Limits: p}, images: make(chan struct{}, 32), imageBytes: semaphore.NewWeighted(64 << 20)}
	var releases []func()
	for range 8 {
		release, reason := a.reserveImage(context.Background(), 1<<20)
		if reason != "" {
			t.Fatal("small grids should fit", reason)
		}
		releases = append(releases, release)
	}
	for _, release := range releases {
		release()
	}
	for range 4 {
		release, reason := a.reserveImage(context.Background(), 16<<20)
		if reason != "" {
			t.Fatal(reason)
		}
		defer release()
	}
	if _, reason := a.reserveImage(context.Background(), 1); reason != "image-bytes" {
		t.Fatalf("overcommitted byte budget: %s", reason)
	}
	if len(a.images) != 4 {
		t.Fatal("failed byte admission leaked a reader slot")
	}
}

func TestImageBudgetWaitsForReleaseAndHonorsCancellation(t *testing.T) {
	p, _ := limits.Defaults("production")
	p.ImageWaitMS = 200
	a := &app{cfg: Config{Limits: p}, images: make(chan struct{}, 1), imageBytes: semaphore.NewWeighted(64 << 20)}
	release, reason := a.reserveImage(context.Background(), 10)
	if reason != "" {
		t.Fatal(reason)
	}
	done := make(chan string, 1)
	go func() {
		r, s := a.reserveImage(context.Background(), 10)
		if r != nil {
			r()
		}
		done <- s
	}()
	time.Sleep(10 * time.Millisecond)
	release()
	if s := <-done; s != "" {
		t.Fatal("waiting request did not recover", s)
	}
	release, reason = a.reserveImage(context.Background(), 10)
	if reason != "" {
		t.Fatal(reason)
	}
	defer release()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, s := a.reserveImage(ctx, 10); s != "image-readers" {
		t.Fatal("cancelled wait admitted", s)
	}
}
