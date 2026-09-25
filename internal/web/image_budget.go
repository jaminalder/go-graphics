package web

import (
	"context"
	"time"
)

// reserveImage admits only declared bytes plus one reader, with a common bounded wait.
func (a *app) reserveImage(ctx context.Context, size int64) (func(), string) {
	if a.cfg.Limits.ImageWaitMS == 0 {
		select {
		case a.images <- struct{}{}:
		default:
			return nil, "image-readers"
		}
		if !a.imageBytes.TryAcquire(size) {
			<-a.images
			return nil, "image-bytes"
		}
	} else {
		wait, cancel := context.WithTimeout(ctx, time.Duration(a.cfg.Limits.ImageWaitMS)*time.Millisecond)
		defer cancel()
		select {
		case a.images <- struct{}{}:
		case <-wait.Done():
			return nil, "image-readers"
		}
		if err := a.imageBytes.Acquire(wait, size); err != nil {
			<-a.images
			return nil, "image-bytes"
		}
	}
	return func() { a.imageBytes.Release(size); <-a.images }, ""
}
