// Package pdfthumb schedules and caches thumbnails from an existing PDF renderer.
package pdfthumb

import (
	"container/list"
	"context"
	"crypto/sha256"
	"sync"
)

// Key identifies a raster by PDF content, one-based page number and requested
// longest side in pixels. Layout and theme do not change the raster.
type Key struct {
	Document [sha256.Size]byte
	Page     int
	Side     int
}

type cachedPage struct {
	key  Key
	data []byte
}

type pendingPage struct {
	ctx     context.Context
	cancel  context.CancelFunc
	done    chan struct{}
	waiting int
	data    []byte
	err     error
}

// Scheduler bounds concurrent renders and retains encoded thumbnails in a
// byte-budgeted least-recently-used cache.
type Scheduler struct {
	workers chan struct{}
	budget  int

	mu      sync.Mutex
	bytes   int
	lru     list.List
	cached  map[Key]*list.Element
	pending map[Key]*pendingPage
}

// New creates a scheduler. A nonpositive cache size disables storage, and at
// least one render is allowed to run at once.
func New(cacheBytes, maxConcurrent int) *Scheduler {
	return &Scheduler{
		workers: make(chan struct{}, max(1, maxConcurrent)),
		budget:  max(0, cacheBytes),
		cached:  make(map[Key]*list.Element),
		pending: make(map[Key]*pendingPage),
	}
}

// Thumbnail returns an immutable raster from the cache or the supplied
// renderer. Concurrent requests for the same key share a render. The render
// context is canceled when the last waiting caller leaves. The renderer
// transfers ownership of its output slice to the scheduler.
func (s *Scheduler) Thumbnail(ctx context.Context, key Key, render func(context.Context) ([]byte, error)) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	s.mu.Lock()
	if element := s.cached[key]; element != nil {
		s.lru.MoveToFront(element)
		data := element.Value.(cachedPage).data
		s.mu.Unlock()
		return data, nil
	}
	job := s.pending[key]
	if job == nil {
		work, cancel := context.WithCancel(context.WithoutCancel(ctx))
		job = &pendingPage{ctx: work, cancel: cancel, done: make(chan struct{})}
		s.pending[key] = job
		go s.run(key, job, render)
	}
	job.waiting++
	s.mu.Unlock()

	select {
	case <-ctx.Done():
		s.mu.Lock()
		if s.pending[key] == job {
			job.waiting--
			if job.waiting == 0 {
				delete(s.pending, key)
				job.cancel()
			}
		}
		s.mu.Unlock()
		return nil, ctx.Err()
	case <-job.done:
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		return job.data, job.err
	}
}

func (s *Scheduler) run(key Key, job *pendingPage, render func(context.Context) ([]byte, error)) {
	defer job.cancel()
	select {
	case s.workers <- struct{}{}:
		if job.err = job.ctx.Err(); job.err == nil {
			job.data, job.err = render(job.ctx)
		}
		<-s.workers
	case <-job.ctx.Done():
		job.err = job.ctx.Err()
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.pending[key] == job {
		delete(s.pending, key)
		if job.err == nil && job.ctx.Err() == nil {
			s.remember(key, job.data)
		}
	}
	close(job.done)
}

func (s *Scheduler) remember(key Key, data []byte) {
	if len(data) == 0 || len(data) > s.budget {
		return
	}
	for s.bytes > s.budget-len(data) {
		page := s.lru.Remove(s.lru.Back()).(cachedPage)
		delete(s.cached, page.key)
		s.bytes -= len(page.data)
	}
	s.cached[key] = s.lru.PushFront(cachedPage{key: key, data: data})
	s.bytes += len(data)
}
