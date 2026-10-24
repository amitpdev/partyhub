package search

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestBeginSupersedesPreviousSearchForKey(t *testing.T) {
	coordinator := NewCoordinator(3)
	first, releaseFirst, err := coordinator.Begin(context.Background(), "guest:one")
	if err != nil {
		t.Fatal(err)
	}
	defer releaseFirst()

	second, releaseSecond, err := coordinator.Begin(context.Background(), "guest:one")
	if err != nil {
		t.Fatal(err)
	}
	defer releaseSecond()

	select {
	case <-first.Done():
		if !errors.Is(context.Cause(first), ErrSuperseded) {
			t.Fatalf("first cause = %v", context.Cause(first))
		}
	case <-time.After(time.Second):
		t.Fatal("first search was not canceled")
	}
	if err := second.Err(); err != nil {
		t.Fatalf("second search was canceled: %v", err)
	}
}

func TestBeginWaitsForGlobalSlot(t *testing.T) {
	coordinator := NewCoordinator(3)
	releases := make([]func(), 0, 3)
	for _, key := range []string{"guest:one", "guest:two", "dj"} {
		_, release, err := coordinator.Begin(context.Background(), key)
		if err != nil {
			t.Fatal(err)
		}
		releases = append(releases, release)
	}
	defer func() {
		for _, release := range releases {
			release()
		}
	}()

	type result struct {
		release func()
		err     error
	}
	started := make(chan result, 1)
	go func() {
		_, release, err := coordinator.Begin(context.Background(), "guest:three")
		started <- result{release: release, err: err}
	}()

	select {
	case <-started:
		t.Fatal("fourth search started before a slot was released")
	case <-time.After(40 * time.Millisecond):
	}

	releases[0]()
	select {
	case result := <-started:
		if result.err != nil {
			t.Fatal(result.err)
		}
		result.release()
	case <-time.After(time.Second):
		t.Fatal("fourth search did not start after a slot was released")
	}
}

func TestWaitingSearchStopsWhenRequestIsCanceled(t *testing.T) {
	coordinator := NewCoordinator(1)
	_, release, err := coordinator.Begin(context.Background(), "guest:one")
	if err != nil {
		t.Fatal(err)
	}
	defer release()

	parent, cancel := context.WithCancel(context.Background())
	finished := make(chan error, 1)
	go func() {
		_, _, err := coordinator.Begin(parent, "guest:two")
		finished <- err
	}()
	cancel()

	select {
	case err := <-finished:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("error = %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("canceled search remained waiting")
	}
}

func TestCancelGuestsLeavesDJSearchActive(t *testing.T) {
	coordinator := NewCoordinator(3)
	guest, releaseGuest, err := coordinator.Begin(context.Background(), "guest:one")
	if err != nil {
		t.Fatal(err)
	}
	defer releaseGuest()
	dj, releaseDJ, err := coordinator.Begin(context.Background(), "dj")
	if err != nil {
		t.Fatal(err)
	}
	defer releaseDJ()

	coordinator.CancelGuests()

	select {
	case <-guest.Done():
		if !errors.Is(context.Cause(guest), ErrSessionEnded) {
			t.Fatalf("guest cause = %v", context.Cause(guest))
		}
	case <-time.After(time.Second):
		t.Fatal("guest search was not canceled")
	}
	if err := dj.Err(); err != nil {
		t.Fatalf("DJ search was canceled: %v", err)
	}
}

func TestReleaseIsIdempotent(t *testing.T) {
	coordinator := NewCoordinator(1)
	_, release, err := coordinator.Begin(context.Background(), "guest:one")
	if err != nil {
		t.Fatal(err)
	}
	release()
	release()

	_, nextRelease, err := coordinator.Begin(context.Background(), "guest:two")
	if err != nil {
		t.Fatal(err)
	}
	nextRelease()
}
