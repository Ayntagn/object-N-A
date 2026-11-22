// spec: FEAT-001 / EV-UI-061
package pdfthumb

import (
	"context"
	"crypto/sha256"
	"errors"
	"runtime"
	"testing"
	"time"
)

func TestSchedulerKeepsSharedRenderWhenOneRequestLeaves(t *testing.T) {
	scheduler := New(32, 1)
	key := Key{Page: 7, Side: 360}
	first, leave := context.WithCancel(t.Context())
	defer leave()
	started, finish := make(chan struct{}), make(chan struct{})
	one, two := make(chan error, 1), make(chan error, 1)
	go func() {
		_, err := scheduler.Thumbnail(first, key, func(ctx context.Context) ([]byte, error) {
			close(started)
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-finish:
				return []byte("thumbnail"), nil
			}
		})
		one <- err
	}()
	<-started
	go func() {
		page, err := scheduler.Thumbnail(t.Context(), key, func(context.Context) ([]byte, error) {
			return nil, errors.New("duplicate render")
		})
		if err == nil && string(page) != "thumbnail" {
			err = errors.New("wrong thumbnail")
		}
		two <- err
	}()
	waitForPending(t, scheduler, key, 2)
	leave()
	if err := <-one; !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled request returned %v", err)
	}
	close(finish)
	if err := <-two; err != nil {
		t.Fatal(err)
	}
}

func TestSchedulerDropsAbandonedPageBeforeRendering(t *testing.T) {
	scheduler := New(32, 1)
	first, vacant, current := Key{Page: 1, Side: 800}, Key{Page: 2, Side: 800}, Key{Page: 450, Side: 800}
	started, finish := make(chan struct{}), make(chan struct{})
	go func() {
		_, _ = scheduler.Thumbnail(t.Context(), first, func(ctx context.Context) ([]byte, error) {
			close(started)
			select {
			case <-finish:
				return []byte("first page"), nil
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		})
	}()
	<-started
	ctx, leave := context.WithCancel(t.Context())
	defer leave()
	executed := make(chan struct{}, 1)
	result := make(chan error, 1)
	go func() {
		_, err := scheduler.Thumbnail(ctx, vacant, func(context.Context) ([]byte, error) {
			executed <- struct{}{}
			return []byte("offscreen page"), nil
		})
		result <- err
	}()
	abandoned := waitForPending(t, scheduler, vacant, 1)
	leave()
	if err := <-result; !errors.Is(err, context.Canceled) {
		t.Fatalf("abandoned page returned %v", err)
	}
	close(finish)
	if _, err := scheduler.Thumbnail(t.Context(), current, func(context.Context) ([]byte, error) {
		return []byte("visible page"), nil
	}); err != nil {
		t.Fatal(err)
	}
	<-abandoned.done
	select {
	case <-executed:
		t.Fatal("abandoned page consumed a render")
	default:
	}
}

func waitForPending(t *testing.T, scheduler *Scheduler, key Key, want int) *pendingPage {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for {
		scheduler.mu.Lock()
		job := scheduler.pending[key]
		ok := job != nil && job.waiting == want
		scheduler.mu.Unlock()
		if ok {
			return job
		}
		if time.Now().After(deadline) {
			t.Fatalf("page %d did not reach %d waiting requests", key.Page, want)
		}
		runtime.Gosched()
	}
}

func TestSchedulerReusesPagesWithinByteBudget(t *testing.T) {
	scheduler := New(6, 1)
	document := sha256.Sum256([]byte("PDF version one"))
	other := sha256.Sum256([]byte("PDF version two"))
	first := Key{Document: document, Page: 1, Side: 360}
	second := Key{Document: document, Page: 2, Side: 360}
	large := Key{Document: document, Page: 1, Side: 800}
	changed := Key{Document: other, Page: 1, Side: 360}
	rendered := 0
	render := func(context.Context) ([]byte, error) {
		rendered++
		return []byte("jpg"), nil
	}
	for _, key := range []Key{first, second, first, large, first, changed, second} {
		if _, err := scheduler.Thumbnail(t.Context(), key, render); err != nil {
			t.Fatal(err)
		}
	}
	if rendered != 5 {
		t.Fatalf("rendered %d times, want five with layout reuse and LRU eviction", rendered)
	}
}
