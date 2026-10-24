package party

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"partyhub/server/player"
	"partyhub/server/youtube"
)

type fakePlayer struct {
	status player.Status
}

func (fake *fakePlayer) Load(trackID, _ string, volume float64) (player.Status, error) {
	fake.status.Generation++
	fake.status.TrackID = trackID
	fake.status.Volume = volume
	fake.status.State = "playing"
	fake.status.Available = true
	return fake.status, nil
}

func (fake *fakePlayer) Pause(paused bool) (player.Status, error) {
	if paused {
		fake.status.State = "paused"
	} else {
		fake.status.State = "playing"
	}
	return fake.status, nil
}

func (fake *fakePlayer) Seek(float64) (player.Status, error)   { return fake.status, nil }
func (fake *fakePlayer) SeekTo(float64) (player.Status, error) { return fake.status, nil }
func (fake *fakePlayer) SetVolume(volume float64) (player.Status, error) {
	fake.status.Volume = volume
	return fake.status, nil
}
func (fake *fakePlayer) Stop() player.Status {
	fake.status.State = "stopped"
	return fake.status
}
func (fake *fakePlayer) Status() player.Status { return fake.status }

type fakeResolver struct {
	tracks map[string]youtube.Track
}

func (fake fakeResolver) Resolve(rawURL string) (youtube.Track, error) {
	track, ok := fake.tracks[rawURL]
	if !ok {
		return youtube.Track{}, errors.New("not found")
	}
	return track, nil
}

func testTrack(id string, duration float64) youtube.Track {
	return youtube.Track{
		Source: "youtube", ProviderID: id, URL: "https://youtube.com/watch?v=" + id,
		Title: "Track " + id, Channel: "Channel", Duration: duration,
	}
}

func TestGuestQueueRules(t *testing.T) {
	tracks := map[string]youtube.Track{
		"one":   testTrack("aaaaaaaaaaa", 180),
		"two":   testTrack("bbbbbbbbbbb", 200),
		"three": testTrack("ccccccccccc", 220),
		"four":  testTrack("ddddddddddd", 240),
	}
	manager := New(&fakePlayer{}, fakeResolver{tracks: tracks}, "")
	guest := Session{ID: "guest-1", Name: "Maya"}

	if _, err := manager.Add("one", guest, false); err != nil {
		t.Fatal(err)
	}
	for _, url := range []string{"two", "three", "four"} {
		if _, err := manager.Add(url, guest, false); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := manager.Add("one", guest, false); err == nil ||
		!strings.Contains(err.Error(), "already at queue position") {
		t.Fatalf("duplicate error = %v", err)
	}

	tracks["five"] = testTrack("eeeeeeeeeee", 260)
	if _, err := manager.Add("five", guest, false); err == nil ||
		!strings.Contains(err.Error(), "upcoming tracks") {
		t.Fatalf("limit error = %v", err)
	}
}

func TestGuestCanOnlyRemoveOwnUpcomingTrack(t *testing.T) {
	manager := New(&fakePlayer{}, fakeResolver{tracks: map[string]youtube.Track{
		"one": testTrack("aaaaaaaaaaa", 180),
		"two": testTrack("bbbbbbbbbbb", 200),
	}}, "")
	maya := Session{ID: "maya", Name: "Maya"}
	noah := Session{ID: "noah", Name: "Noah"}
	if _, err := manager.Add("one", maya, false); err != nil {
		t.Fatal(err)
	}
	if _, err := manager.Add("two", noah, false); err != nil {
		t.Fatal(err)
	}
	snapshot := manager.Snapshot("maya", false)
	if snapshot.Tracks[0].CanRemove {
		t.Fatal("current track should not be removable")
	}
	if snapshot.Tracks[1].CanRemove {
		t.Fatal("another guest's track should not be removable")
	}
	if err := manager.Remove(snapshot.Tracks[1].QueueID, "maya", false); err == nil {
		t.Fatal("guest removed another guest's track")
	}
	if err := manager.Remove(snapshot.Tracks[1].QueueID, "noah", false); err != nil {
		t.Fatal(err)
	}
}

func TestRejectsLongAndUnknownDurationTracks(t *testing.T) {
	manager := New(&fakePlayer{}, fakeResolver{tracks: map[string]youtube.Track{
		"live": testTrack("aaaaaaaaaaa", 0),
		"long": testTrack("bbbbbbbbbbb", MaxTrackSeconds+1),
	}}, "")
	guest := Session{ID: "guest", Name: "Maya"}
	for _, url := range []string{"live", "long"} {
		if _, err := manager.Add(url, guest, false); err == nil {
			t.Fatalf("Add(%q) returned no error", url)
		}
	}
}

func TestAutoAdvance(t *testing.T) {
	localPlayer := &fakePlayer{}
	manager := New(localPlayer, fakeResolver{tracks: map[string]youtube.Track{
		"one": testTrack("aaaaaaaaaaa", 180),
		"two": testTrack("bbbbbbbbbbb", 200),
	}}, "")
	if _, err := manager.Add("one", Session{}, true); err != nil {
		t.Fatal(err)
	}
	if _, err := manager.Add("two", Session{}, true); err != nil {
		t.Fatal(err)
	}
	localPlayer.status.State = "ended"
	manager.Tick()
	if snapshot := manager.Snapshot("", true); snapshot.CurrentIndex != 1 {
		t.Fatalf("currentIndex = %d, want 1", snapshot.CurrentIndex)
	}
}

func TestAddingAfterNaturalExhaustionStartsNewTrack(t *testing.T) {
	localPlayer := &fakePlayer{}
	manager := New(localPlayer, fakeResolver{tracks: map[string]youtube.Track{
		"one": testTrack("aaaaaaaaaaa", 180),
		"two": testTrack("bbbbbbbbbbb", 200),
	}}, "")
	if _, err := manager.Add("one", Session{}, true); err != nil {
		t.Fatal(err)
	}
	localPlayer.status.State = "ended"
	manager.Tick()

	if _, err := manager.Add("two", Session{}, true); err != nil {
		t.Fatal(err)
	}
	snapshot := manager.Snapshot("", true)
	if snapshot.CurrentIndex != 1 {
		t.Fatalf("currentIndex = %d, want 1", snapshot.CurrentIndex)
	}
	if snapshot.Player.State != "playing" ||
		snapshot.Player.TrackID != snapshot.Tracks[1].QueueID {
		t.Fatalf("player = %#v, want newly added track playing", snapshot.Player)
	}
}

func TestAddingAfterManualStopDoesNotStartNewTrack(t *testing.T) {
	localPlayer := &fakePlayer{}
	manager := New(localPlayer, fakeResolver{tracks: map[string]youtube.Track{
		"one": testTrack("aaaaaaaaaaa", 180),
		"two": testTrack("bbbbbbbbbbb", 200),
	}}, "")
	if _, err := manager.Add("one", Session{}, true); err != nil {
		t.Fatal(err)
	}
	manager.Stop()

	if _, err := manager.Add("two", Session{}, true); err != nil {
		t.Fatal(err)
	}
	snapshot := manager.Snapshot("", true)
	if snapshot.CurrentIndex != 0 {
		t.Fatalf("currentIndex = %d, want 0", snapshot.CurrentIndex)
	}
	if snapshot.Player.State != "stopped" {
		t.Fatalf("player state = %q, want stopped", snapshot.Player.State)
	}
}

func TestPlayedTrackCanBeAddedAgain(t *testing.T) {
	localPlayer := &fakePlayer{}
	manager := New(localPlayer, fakeResolver{tracks: map[string]youtube.Track{
		"one": testTrack("aaaaaaaaaaa", 180),
	}}, "")
	first, err := manager.Add("one", Session{}, true)
	if err != nil {
		t.Fatal(err)
	}
	firstQueueID := first.Tracks[0].QueueID
	localPlayer.status.State = "ended"
	manager.Tick()

	replayed, err := manager.Add("one", Session{}, true)
	if err != nil {
		t.Fatalf("re-adding a played track: %v", err)
	}
	if len(replayed.Tracks) != 2 || replayed.CurrentIndex != 1 {
		t.Fatalf("replayed snapshot = %#v", replayed)
	}
	if replayed.Tracks[1].QueueID == firstQueueID {
		t.Fatal("replayed track reused the old queue entry")
	}
}

func TestDuplicateCurrentOrUpcomingTrackIsRejected(t *testing.T) {
	manager := New(&fakePlayer{}, fakeResolver{tracks: map[string]youtube.Track{
		"one": testTrack("aaaaaaaaaaa", 180),
		"two": testTrack("bbbbbbbbbbb", 200),
	}}, "")
	if _, err := manager.Add("one", Session{}, true); err != nil {
		t.Fatal(err)
	}
	if _, err := manager.Add("two", Session{}, true); err != nil {
		t.Fatal(err)
	}
	for _, rawURL := range []string{"one", "two"} {
		if _, err := manager.Add(rawURL, Session{}, true); err == nil ||
			!strings.Contains(err.Error(), "already at queue position") {
			t.Fatalf("duplicate %q error = %v", rawURL, err)
		}
	}
}

func TestQueueCapacityCountsOnlyUpcomingTracks(t *testing.T) {
	manager := New(&fakePlayer{}, fakeResolver{tracks: map[string]youtube.Track{
		"new": testTrack("new-track-id", 180),
	}}, "")
	for index := 0; index < MaxHistoryTracks; index++ {
		manager.history = append(manager.history, Track{QueueID: randomID(4), ProviderID: string(rune(index + 1))})
	}
	if _, err := manager.Add("new", Session{}, true); err != nil {
		t.Fatalf("history blocked a new track: %v", err)
	}

	manager.upcoming = make([]Track, MaxQueueTracks)
	if _, err := manager.Add("new", Session{}, true); err == nil ||
		!strings.Contains(err.Error(), "queue is full") {
		t.Fatalf("full upcoming queue error = %v", err)
	}
}

func TestHistoryIsPrunedToMostRecentTracks(t *testing.T) {
	manager := New(&fakePlayer{}, fakeResolver{}, "")
	for index := 0; index <= MaxHistoryTracks; index++ {
		current := Track{QueueID: randomID(4), ProviderID: string(rune(index + 1))}
		manager.current = &current
		manager.finishCurrentLocked()
	}
	if len(manager.history) != MaxHistoryTracks {
		t.Fatalf("history length = %d, want %d", len(manager.history), MaxHistoryTracks)
	}
	if manager.history[0].ProviderID != string(rune(2)) {
		t.Fatal("history did not discard its oldest track")
	}
}

func TestPersistsQueueAcrossManagerRestart(t *testing.T) {
	statePath := filepath.Join(t.TempDir(), "state.json")
	resolver := fakeResolver{tracks: map[string]youtube.Track{
		"one": testTrack("aaaaaaaaaaa", 180),
		"two": testTrack("bbbbbbbbbbb", 200),
	}}
	localPlayer := &fakePlayer{}
	manager := New(localPlayer, resolver, statePath)
	if _, err := manager.Add("one", Session{ID: "maya", Name: "Maya"}, false); err != nil {
		t.Fatal(err)
	}
	if _, err := manager.Add("two", Session{}, true); err != nil {
		t.Fatal(err)
	}
	localPlayer.status.State = "ended"
	manager.Tick()

	restored := New(&fakePlayer{}, resolver, statePath).Snapshot("maya", false)
	if len(restored.Tracks) != 2 || restored.Tracks[0].AddedBy != "Maya" {
		t.Fatalf("restored tracks = %#v", restored.Tracks)
	}
	if restored.CurrentIndex != 1 {
		t.Fatalf("currentIndex = %d, want 1", restored.CurrentIndex)
	}
}

func TestMigratesLegacyPersistedQueue(t *testing.T) {
	statePath := filepath.Join(t.TempDir(), "state.json")
	legacy := persistedState{
		Tracks: []persistedTrack{
			{QueueID: "one", ProviderID: "aaaaaaaaaaa", URL: "https://example.com/one"},
			{QueueID: "two", ProviderID: "bbbbbbbbbbb", URL: "https://example.com/two"},
		},
		CurrentIndex: 1,
		Volume:       65,
	}
	data, err := json.Marshal(legacy)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(statePath, data, 0o600); err != nil {
		t.Fatal(err)
	}

	snapshot := New(&fakePlayer{}, fakeResolver{}, statePath).Snapshot("", true)
	if len(snapshot.Tracks) != 2 || snapshot.CurrentIndex != 1 || snapshot.Volume != 65 {
		t.Fatalf("migrated snapshot = %#v", snapshot)
	}
}

func TestSaveFailureIsVisibleOnlyToDJ(t *testing.T) {
	statePath := filepath.Join(t.TempDir(), "state.json")
	if err := os.Mkdir(statePath, 0o700); err != nil {
		t.Fatal(err)
	}
	manager := New(&fakePlayer{}, fakeResolver{tracks: map[string]youtube.Track{
		"one": testTrack("aaaaaaaaaaa", 180),
	}}, statePath)
	if _, err := manager.Add("one", Session{}, true); err != nil {
		t.Fatal(err)
	}

	dj := manager.Snapshot("", true)
	if !strings.Contains(dj.StorageError, "only in memory") ||
		!strings.Contains(dj.StorageError, "retry") {
		t.Fatalf("DJ storage error = %q", dj.StorageError)
	}
	if guest := manager.Snapshot("guest", false); guest.StorageError != "" {
		t.Fatalf("guest saw host storage error %q", guest.StorageError)
	}
}

func TestInvalidSavedQueueIsVisibleToDJ(t *testing.T) {
	statePath := filepath.Join(t.TempDir(), "state.json")
	if err := os.WriteFile(statePath, []byte("{invalid"), 0o600); err != nil {
		t.Fatal(err)
	}

	snapshot := New(&fakePlayer{}, fakeResolver{}, statePath).Snapshot("", true)
	if !strings.Contains(snapshot.StorageError, "invalid") ||
		!strings.Contains(snapshot.StorageError, "restart") {
		t.Fatalf("storage error = %q", snapshot.StorageError)
	}
}

func TestTrackActionsReflectHistoryCurrentAndUpcoming(t *testing.T) {
	localPlayer := &fakePlayer{}
	manager := New(localPlayer, fakeResolver{tracks: map[string]youtube.Track{
		"one":   testTrack("aaaaaaaaaaa", 180),
		"two":   testTrack("bbbbbbbbbbb", 200),
		"three": testTrack("ccccccccccc", 220),
	}}, "")
	maya := Session{ID: "maya", Name: "Maya"}
	for _, rawURL := range []string{"one", "two", "three"} {
		if _, err := manager.Add(rawURL, maya, false); err != nil {
			t.Fatal(err)
		}
	}
	localPlayer.status.State = "ended"
	manager.Tick()

	dj := manager.Snapshot("", true)
	if !dj.Tracks[0].CanPlay || !dj.Tracks[0].CanRemove || dj.Tracks[0].CanMove {
		t.Fatalf("DJ history actions = %#v", dj.Tracks[0])
	}
	if !dj.Tracks[1].CanPlay || dj.Tracks[1].CanRemove || dj.Tracks[1].CanMove {
		t.Fatalf("DJ current actions = %#v", dj.Tracks[1])
	}
	if !dj.Tracks[2].CanPlay || !dj.Tracks[2].CanRemove || !dj.Tracks[2].CanMove {
		t.Fatalf("DJ upcoming actions = %#v", dj.Tracks[2])
	}

	guest := manager.Snapshot("maya", false)
	if guest.Tracks[0].CanPlay || guest.Tracks[0].CanRemove || guest.Tracks[0].CanMove {
		t.Fatalf("guest history actions = %#v", guest.Tracks[0])
	}
	if !guest.Tracks[2].CanRemove || guest.Tracks[2].CanPlay || guest.Tracks[2].CanMove {
		t.Fatalf("guest upcoming actions = %#v", guest.Tracks[2])
	}
}

func TestPreviousNextAndPlayReshapeTheQueue(t *testing.T) {
	localPlayer := &fakePlayer{}
	manager := New(localPlayer, fakeResolver{tracks: map[string]youtube.Track{
		"one":   testTrack("aaaaaaaaaaa", 180),
		"two":   testTrack("bbbbbbbbbbb", 200),
		"three": testTrack("ccccccccccc", 220),
	}}, "")
	for _, rawURL := range []string{"one", "two", "three"} {
		if _, err := manager.Add(rawURL, Session{}, true); err != nil {
			t.Fatal(err)
		}
	}
	localPlayer.status.State = "ended"
	manager.Tick()

	if err := manager.Previous(); err != nil {
		t.Fatal(err)
	}
	if snapshot := manager.Snapshot("", true); snapshot.CurrentIndex != 0 ||
		snapshot.Tracks[0].ProviderID != "aaaaaaaaaaa" {
		t.Fatalf("after previous = %#v", snapshot)
	}
	if err := manager.Next(); err != nil {
		t.Fatal(err)
	}
	if snapshot := manager.Snapshot("", true); snapshot.CurrentIndex != 1 ||
		snapshot.Tracks[1].ProviderID != "bbbbbbbbbbb" {
		t.Fatalf("after next = %#v", snapshot)
	}
	if err := manager.Play(2); err != nil {
		t.Fatal(err)
	}
	if snapshot := manager.Snapshot("", true); snapshot.CurrentIndex != 2 ||
		snapshot.Tracks[2].ProviderID != "ccccccccccc" {
		t.Fatalf("after play = %#v", snapshot)
	}
}

func TestEndingShareRevokesGuestSessionsButKeepsQueue(t *testing.T) {
	manager := New(&fakePlayer{}, fakeResolver{tracks: map[string]youtube.Track{
		"one": testTrack("aaaaaaaaaaa", 180),
	}}, "")
	if _, err := manager.Add("one", Session{}, true); err != nil {
		t.Fatal(err)
	}
	info, err := manager.ConfigureShare("https://party.ngrok.app")
	if err != nil {
		t.Fatal(err)
	}
	token := strings.Split(info.GuestURL, "token=")[1]
	session, err := manager.CreateSession(token, "Maya")
	if err != nil {
		t.Fatal(err)
	}

	manager.EndShare()
	if _, active := manager.Session(session.ID); active {
		t.Fatal("guest session remained active")
	}
	if snapshot := manager.Snapshot("", true); len(snapshot.Tracks) != 1 {
		t.Fatal("ending share cleared the queue")
	}
}

func TestGuestAccessCanRestartWithoutReconfiguringTunnel(t *testing.T) {
	localPlayer := &fakePlayer{}
	manager := New(localPlayer, fakeResolver{tracks: map[string]youtube.Track{
		"one": testTrack("aaaaaaaaaaa", 180),
	}}, "")
	if _, err := manager.Add("one", Session{}, true); err != nil {
		t.Fatal(err)
	}
	before := manager.Snapshot("", true)
	original, err := manager.ConfigureShare("https://party.ngrok.app")
	if err != nil {
		t.Fatal(err)
	}
	oldToken := strings.Split(original.GuestURL, "#token=")[1]
	oldSession, err := manager.CreateSession(oldToken, "Maya")
	if err != nil {
		t.Fatal(err)
	}
	manager.EndShare()
	off := manager.ShareInfo()
	if off.Active || off.PublicURL != original.PublicURL || off.GuestURL != "" {
		t.Fatalf("tunnel not preserved while access is off: %#v", off)
	}
	if _, err := manager.CreateSession(oldToken, "Maya"); err == nil {
		t.Fatal("old link worked while disabled")
	}
	resumed, err := manager.StartShare()
	if err != nil {
		t.Fatal(err)
	}
	if !resumed.Active || resumed.PublicURL != original.PublicURL || resumed.GuestURL == original.GuestURL {
		t.Fatalf("unexpected resumed sharing: %#v", resumed)
	}
	if _, ok := manager.Session(oldSession.ID); ok {
		t.Fatal("old session was revived")
	}
	if _, err := manager.CreateSession(oldToken, "Maya"); err == nil {
		t.Fatal("old token was revived")
	}
	newToken := strings.Split(resumed.GuestURL, "#token=")[1]
	session, err := manager.CreateSession(newToken, "Maya")
	if err != nil {
		t.Fatal(err)
	}
	again, err := manager.StartShare()
	if err != nil || again.GuestURL != resumed.GuestURL {
		t.Fatal("repeated enable rotated the active link")
	}
	if _, ok := manager.Session(session.ID); !ok {
		t.Fatal("repeated enable revoked a valid session")
	}
	after := manager.Snapshot("", true)
	if len(after.Tracks) != len(before.Tracks) || after.Tracks[0].QueueID != before.Tracks[0].QueueID || after.Player.State != before.Player.State || after.Player.Generation != before.Player.Generation {
		t.Fatal("toggling guest access changed queue or playback")
	}
}

func TestStartSharingRequiresConfiguredTunnel(t *testing.T) {
	manager := New(&fakePlayer{}, fakeResolver{}, "")
	if _, err := manager.StartShare(); err == nil {
		t.Fatal("guest access enabled without a tunnel")
	}
	if manager.ShareInfo().Active {
		t.Fatal("guest access became active")
	}
}
