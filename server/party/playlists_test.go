package party

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"partyhub/server/youtube"
)

func playlistManager(t *testing.T) *Manager {
	t.Helper()
	m := New(&fakePlayer{}, fakeResolver{tracks: map[string]youtube.Track{
		"one": testTrack("aaaaaaaaaaa", 180), "two": testTrack("bbbbbbbbbbb", 200), "three": testTrack("ccccccccccc", 220),
	}}, filepath.Join(t.TempDir(), "state.json"))
	for _, url := range []string{"one", "two"} {
		if _, err := m.Add(url, Session{}, true); err != nil {
			t.Fatal(err)
		}
	}
	return m
}
func TestPlaylistSaveRestartAndDirtyTracking(t *testing.T) {
	m := playlistManager(t)
	p, err := m.SavePlaylist("", "  Party  ", true)
	if err != nil {
		t.Fatal(err)
	}
	if p.Name != "Party" || len(p.Tracks) != 2 {
		t.Fatalf("saved %#v", p)
	}
	if err = m.Next(); err != nil {
		t.Fatal(err)
	}
	if m.Snapshot("", true).Playlist.Dirty {
		t.Fatal("skipping marked playlist dirty")
	}
	if err = m.Previous(); err != nil {
		t.Fatal(err)
	}
	if _, err = m.SetVolume(55); err != nil {
		t.Fatal(err)
	}
	if m.Snapshot("", true).Playlist.Dirty {
		t.Fatal("playback changed playlist")
	}
	if _, err = m.Add("three", Session{ID: "guest", Name: "Maya"}, false); err != nil {
		t.Fatal(err)
	}
	if !m.Snapshot("", true).Playlist.Dirty {
		t.Fatal("guest addition not tracked")
	}
	if m.Snapshot("guest", false).Playlist != nil {
		t.Fatal("guest received playlist details")
	}
	restored := New(&fakePlayer{}, m.resolver, m.statePath)
	if restored.playlist == nil || !restored.playlist.Dirty {
		t.Fatal("dirty state lost on restart")
	}
	library, err := restored.Playlists()
	if err != nil || len(library.Playlists) != 1 {
		t.Fatalf("restore %v %#v", err, library)
	}
	if len(library.Playlists[0].Tracks) != 2 {
		t.Fatal("queue automatically modified playlist")
	}
	if _, _, err = m.LoadPlaylist(p.ID, "replace", false); err == nil {
		t.Fatal("dirty queue replaced without decision")
	}
	if _, err = m.SavePlaylist(p.ID, "Updated", true); err != nil {
		t.Fatal(err)
	}
	if m.Snapshot("", true).Playlist.Dirty {
		t.Fatal("save did not clear dirty state")
	}
}
func TestPlaylistLoadSemantics(t *testing.T) {
	m := playlistManager(t)
	p, err := m.SavePlaylist("", "Party", true)
	if err != nil {
		t.Fatal(err)
	}
	old := m.Snapshot("", true)
	snapshot, skipped, err := m.LoadPlaylist(p.ID, "replace", false)
	if err != nil {
		t.Fatal(err)
	}
	if skipped != 0 || snapshot.CurrentIndex != 0 || snapshot.Player.State != "stopped" || snapshot.Volume != old.Volume {
		t.Fatalf("replace %#v", snapshot)
	}
	if snapshot.Tracks[0].QueueID == old.Tracks[0].QueueID || snapshot.Tracks[0].AddedBy != "DJ" {
		t.Fatal("queue identities were reused")
	}
	if err = m.Play(0); err != nil {
		t.Fatal(err)
	}
	snapshot, skipped, err = m.LoadPlaylist(p.ID, "append", false)
	if err != nil {
		t.Fatal(err)
	}
	if skipped != 2 || len(snapshot.Tracks) != 2 || snapshot.Player.State != "playing" || snapshot.Playlist.Dirty {
		t.Fatal("append duplicates changed playback or content")
	}
	// Repeated videos may exist across history and the upcoming queue.
	if err = m.Next(); err != nil {
		t.Fatal(err)
	}
	snapshot, skipped, err = m.LoadPlaylist(p.ID, "append", false)
	if err != nil {
		t.Fatal(err)
	}
	if skipped != 1 || !snapshot.Playlist.Dirty || len(snapshot.Tracks) != 3 {
		t.Fatal("append did not track new content")
	}
	if _, err = m.SavePlaylist("", "Repeated", true); err != nil {
		t.Fatal(err)
	}
	p2 := m.library.Playlists[1]
	snapshot, skipped, err = m.LoadPlaylist(p2.ID, "replace", true)
	if err != nil {
		t.Fatal(err)
	}
	if skipped != 1 || len(snapshot.Tracks) != 2 || !snapshot.Playlist.Dirty {
		t.Fatal("repeated tracks not reported")
	}
}
func TestPlaylistBackupAndAtomicFailures(t *testing.T) {
	m := playlistManager(t)
	p, err := m.SavePlaylist("", "Party", true)
	if err != nil {
		t.Fatal(err)
	}
	backup, err := m.Playlists()
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(backup)
	if strings.Contains(string(raw), "queueId") || strings.Contains(string(raw), "ownerId") {
		t.Fatal("backup leaked session data")
	}
	if err = m.ImportPlaylists(backup); err != nil {
		t.Fatal(err)
	}
	if m.library.Playlists[1].ID == p.ID {
		t.Fatal("import reused ID")
	}
	backup.Playlists[0].Tracks[0].URL = "https://example.com"
	if err = m.ImportPlaylists(backup); err != nil {
		t.Fatal(err)
	}
	if m.library.Playlists[2].Tracks[0].URL != "https://www.youtube.com/watch?v=aaaaaaaaaaa" {
		t.Fatal("URL not normalized")
	}
	backup.Playlists[0].Tracks[0].ProviderID = "invalid"
	if err = m.ImportPlaylists(backup); err == nil || len(m.library.Playlists) != 3 {
		t.Fatal("invalid import changed library")
	}
	// Force atomic rename failure with an existing directory at the file destination.
	path := m.libraryPath()
	if err = os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err = os.Mkdir(path, 0700); err != nil {
		t.Fatal(err)
	}
	if _, err = m.SavePlaylist(p.ID, "Lost", true); err == nil {
		t.Fatal("write failure reported success")
	}
	if m.library.Playlists[0].Name != "Party" {
		t.Fatal("failed write changed in-memory library")
	}
}
func TestPlaylistCapacityAndCorruption(t *testing.T) {
	m := playlistManager(t)
	p, _ := m.SavePlaylist("", "Party", true)
	for len(m.upcoming) < MaxQueueTracks {
		m.upcoming = append(m.upcoming, Track{ProviderID: "other"})
	}
	// Make both playlist tracks absent from the active queue.
	m.current = nil
	m.upcoming[0].ProviderID = "other"
	before := len(m.upcoming)
	if _, _, err := m.LoadPlaylist(p.ID, "append", false); err == nil || len(m.upcoming) != before {
		t.Fatal("over-capacity append was not atomic")
	}
	if err := os.WriteFile(m.libraryPath(), []byte("broken"), 0600); err != nil {
		t.Fatal(err)
	}
	restored := New(&fakePlayer{}, m.resolver, m.statePath)
	if _, err := restored.Playlists(); err == nil {
		t.Fatal("corruption hidden")
	}
	if _, err := restored.SavePlaylist("", "New", true); err == nil {
		t.Fatal("corrupt library overwritten")
	}
}
func TestPlaylistRenameDeleteAndEditorialChanges(t *testing.T) {
	m := playlistManager(t)
	p, _ := m.SavePlaylist("", "Party", true)
	if _, err := m.Add("three", Session{}, true); err != nil {
		t.Fatal(err)
	}
	if _, err := m.SavePlaylist(p.ID, "Party", true); err != nil {
		t.Fatal(err)
	}
	if err := m.Move(m.upcoming[1].QueueID, -1); err != nil {
		t.Fatal(err)
	}
	if !m.playlist.Dirty {
		t.Fatal("reorder not tracked")
	}
	if _, err := m.SavePlaylist(p.ID, "Renamed", false); err != nil {
		t.Fatal(err)
	}
	if m.playlist.Name != "Renamed" || !m.playlist.Dirty {
		t.Fatal("rename cleared dirty state")
	}
	if err := m.Remove(m.upcoming[0].QueueID, "", true); err != nil {
		t.Fatal(err)
	}
	if err := m.DeletePlaylist(p.ID); err != nil {
		t.Fatal(err)
	}
	if m.playlist != nil || len(m.Snapshot("", true).Tracks) != 2 {
		t.Fatal("delete affected queue")
	}
}
