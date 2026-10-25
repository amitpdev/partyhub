package party

import (
	"context"
	"errors"
)

// AddNext validates a real video again, then inserts it atomically without
// loading or stopping the player. An empty queue becomes ready without autoplay.
func (manager *Manager) AddNext(ctx context.Context, rawURL string) (Snapshot, string, error) {
	if err := ctx.Err(); err != nil {
		return Snapshot{}, "", err
	}
	resolved, err := manager.resolver.Resolve(rawURL)
	if err != nil {
		return Snapshot{}, "", err
	}
	if resolved.Duration <= 0 || resolved.Duration > MaxTrackSeconds {
		return Snapshot{}, "", errors.New("AI picks must be videos with a duration of 15 minutes or less")
	}
	manager.mu.Lock()
	defer manager.mu.Unlock()
	if err = ctx.Err(); err != nil {
		return Snapshot{}, "", err
	}
	if len(manager.upcoming) >= MaxQueueTracks {
		return Snapshot{}, "", errors.New("the queue is full")
	}
	if manager.current != nil && manager.current.ProviderID == resolved.ProviderID {
		return Snapshot{}, "", errors.New("this song is already playing")
	}
	for _, track := range manager.upcoming {
		if track.ProviderID == resolved.ProviderID {
			return Snapshot{}, "", errors.New("this song is already queued")
		}
	}
	track := Track{QueueID: randomID(16), Source: resolved.Source, ProviderID: resolved.ProviderID, URL: resolved.URL, Title: resolved.Title, Channel: resolved.Channel, Thumbnail: resolved.Thumbnail, Duration: resolved.Duration, AddedBy: "DJ"}
	if manager.current == nil {
		manager.player.Stop()
		manager.handledGeneration = -1
		manager.current = &track
	} else {
		manager.upcoming = append([]Track{track}, manager.upcoming...)
	}
	manager.markPlaylistDirtyLocked()
	manager.changedLocked()
	return manager.snapshotLocked("", true), track.QueueID, nil
}
