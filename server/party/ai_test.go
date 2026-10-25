package party

import (
	"context"
	"partyhub/server/player"
	"partyhub/server/youtube"
	"testing"
)

func TestAIAddNextPreservesPlaybackAndOrder(t *testing.T) {
	local := &fakePlayer{}
	manager := New(local, fakeResolver{tracks: map[string]youtube.Track{"one": testTrack("aaaaaaaaaaa", 180), "two": testTrack("bbbbbbbbbbb", 200), "pick": testTrack("ccccccccccc", 220)}}, "")
	_, _ = manager.Add("one", Session{}, true)
	_, _ = manager.Add("two", Session{}, true)
	saved, err := manager.SavePlaylist("", "Set", true)
	if err != nil {
		t.Fatal(err)
	}
	before := manager.Snapshot("", true)
	after, queueID, err := manager.AddNext(context.Background(), "pick")
	if err != nil {
		t.Fatal(err)
	}
	if after.CurrentIndex != before.CurrentIndex || after.Player.State != before.Player.State || after.Player.Generation != before.Player.Generation || after.Tracks[0].QueueID != before.Tracks[0].QueueID {
		t.Fatal("AI insertion interrupted playback")
	}
	if len(after.Tracks) != 3 || after.Tracks[1].ProviderID != "ccccccccccc" || after.Tracks[2].ProviderID != "bbbbbbbbbbb" || after.Tracks[1].QueueID != queueID {
		t.Fatal("AI insertion changed the upcoming order")
	}
	if after.Playlist.ID != saved.ID || !after.Playlist.Dirty {
		t.Fatal("AI insertion not tracked as playlist edit")
	}
	if err = manager.Remove(queueID, "", true); err != nil {
		t.Fatal(err)
	}
	if _, _, err = manager.AddNext(context.Background(), "two"); err == nil {
		t.Fatal("queued duplicate accepted")
	}
}
func TestAIAddNextEmptyQueueDoesNotAutoplayAndHonorsCancellation(t *testing.T) {
	manager := New(&fakePlayer{status: player.Status{Available: true}}, fakeResolver{tracks: map[string]youtube.Track{"pick": testTrack("ccccccccccc", 180)}}, "")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, _, err := manager.AddNext(ctx, "pick"); err == nil || len(manager.Snapshot("", true).Tracks) != 0 {
		t.Fatal("cancelled pick inserted a song")
	}
	snapshot, _, err := manager.AddNext(context.Background(), "pick")
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.CurrentIndex != 0 || snapshot.Player.State == "playing" || len(snapshot.Tracks) != 1 {
		t.Fatal("empty queue auto-played")
	}
}
