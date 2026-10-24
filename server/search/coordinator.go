package search

import (
	"context"
	"errors"
	"strings"
	"sync"
)

var (
	ErrSuperseded   = errors.New("search was replaced by a newer search")
	ErrSessionEnded = errors.New("guest search ended with party sharing")
)

type activeSearch struct {
	id     uint64
	cancel context.CancelCauseFunc
}

type Coordinator struct {
	slots  chan struct{}
	mu     sync.Mutex
	nextID uint64
	active map[string]activeSearch
}

func NewCoordinator(limit int) *Coordinator {
	if limit < 1 {
		panic("search concurrency limit must be positive")
	}
	return &Coordinator{
		slots:  make(chan struct{}, limit),
		active: make(map[string]activeSearch),
	}
}

func (coordinator *Coordinator) Begin(
	parent context.Context,
	key string,
) (context.Context, func(), error) {
	searchContext, cancel := context.WithCancelCause(parent)

	coordinator.mu.Lock()
	coordinator.nextID++
	id := coordinator.nextID
	if previous, ok := coordinator.active[key]; ok {
		previous.cancel(ErrSuperseded)
	}
	coordinator.active[key] = activeSearch{id: id, cancel: cancel}
	coordinator.mu.Unlock()

	select {
	case coordinator.slots <- struct{}{}:
	case <-searchContext.Done():
		coordinator.removeIfCurrent(key, id)
		return nil, nil, context.Cause(searchContext)
	}

	var once sync.Once
	release := func() {
		once.Do(func() {
			<-coordinator.slots
			coordinator.removeIfCurrent(key, id)
			cancel(context.Canceled)
		})
	}
	return searchContext, release, nil
}

func (coordinator *Coordinator) CancelGuests() {
	coordinator.mu.Lock()
	defer coordinator.mu.Unlock()
	for key, active := range coordinator.active {
		if strings.HasPrefix(key, "guest:") {
			active.cancel(ErrSessionEnded)
		}
	}
}

func (coordinator *Coordinator) removeIfCurrent(key string, id uint64) {
	coordinator.mu.Lock()
	defer coordinator.mu.Unlock()
	if active, ok := coordinator.active[key]; ok && active.id == id {
		delete(coordinator.active, key)
	}
}
