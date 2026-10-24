package party

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"partyhub/server/youtube"
)

type Playlist struct {
	ID        string          `json:"id"`
	Name      string          `json:"name"`
	CreatedAt time.Time       `json:"createdAt"`
	UpdatedAt time.Time       `json:"updatedAt"`
	Tracks    []youtube.Track `json:"tracks"`
}
type PlaylistLink struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Dirty bool   `json:"dirty"`
}
type PlaylistBackup struct {
	Version   int        `json:"version"`
	Playlists []Playlist `json:"playlists"`
}
type playlistLibrary = PlaylistBackup

func (manager *Manager) playlistForViewer(dj bool) *PlaylistLink {
	if !dj || manager.playlist == nil {
		return nil
	}
	link := *manager.playlist
	return &link
}
func (manager *Manager) markPlaylistDirtyLocked() {
	if manager.playlist != nil {
		manager.playlist.Dirty = true
	}
}
func (manager *Manager) libraryPath() string {
	if manager.statePath == "" {
		return ""
	}
	return filepath.Join(filepath.Dir(manager.statePath), "playlists.json")
}

// PlaylistStore separates library persistence from party and playback behavior.
// A cloud-backed implementation can replace the local JSON file later.
type PlaylistStore interface {
	Load() (PlaylistBackup, error)
	Save(PlaylistBackup) error
}

type filePlaylistStore struct{ path string }

func (store filePlaylistStore) Load() (PlaylistBackup, error) {
	library := playlistLibrary{Version: 1, Playlists: []Playlist{}}
	if store.path == "" {
		return library, nil
	}
	data, err := os.ReadFile(store.path)
	if errors.Is(err, os.ErrNotExist) {
		return library, nil
	}
	if err != nil {
		return library, err
	}
	if err = json.Unmarshal(data, &library); err != nil {
		return library, err
	}
	return library, validateLibrary(&library)
}

func (store filePlaylistStore) Save(library PlaylistBackup) error {
	if store.path == "" {
		return nil
	}
	data, err := json.MarshalIndent(library, "", "  ")
	if err != nil {
		return err
	}
	if err = os.MkdirAll(filepath.Dir(store.path), 0700); err != nil {
		return err
	}
	temporary := store.path + ".tmp"
	defer os.Remove(temporary)
	if err = os.WriteFile(temporary, data, 0600); err != nil {
		return err
	}
	return os.Rename(temporary, store.path)
}

func (manager *Manager) loadLibrary() {
	manager.playlistStore = filePlaylistStore{path: manager.libraryPath()}
	manager.library, manager.libraryError = manager.playlistStore.Load()
	if manager.libraryError != nil {
		manager.libraryError = fmt.Errorf("playlist library could not be loaded; repair %s and restart: %w", manager.libraryPath(), manager.libraryError)
	}
}
func (manager *Manager) writeLibrary(next playlistLibrary) error {
	if manager.libraryError != nil {
		return manager.libraryError
	}
	if err := manager.playlistStore.Save(next); err != nil {
		return fmt.Errorf("playlist could not be saved: %w", err)
	}
	manager.library = next
	return nil
}
func (manager *Manager) Playlists() (PlaylistBackup, error) {
	manager.mu.Lock()
	defer manager.mu.Unlock()
	// Copy through JSON so callers never share mutable slices with the manager.
	data, _ := json.Marshal(manager.library)
	var copy PlaylistBackup
	_ = json.Unmarshal(data, &copy)
	return copy, manager.libraryError
}

var playlistVideoID = regexp.MustCompile(`^[A-Za-z0-9_-]{11}$`)

func validatePlaylist(p *Playlist) error {
	p.Name = strings.TrimSpace(p.Name)
	if len([]rune(p.Name)) < 1 || len([]rune(p.Name)) > 80 {
		return errors.New("playlist names must be 1–80 characters")
	}
	if len(p.Tracks) == 0 || len(p.Tracks) > MaxHistoryTracks+MaxQueueTracks+1 {
		return errors.New("a playlist must contain 1–201 tracks")
	}
	for i := range p.Tracks {
		t := &p.Tracks[i]
		if t.Source != "youtube" || !playlistVideoID.MatchString(t.ProviderID) || strings.TrimSpace(t.Title) == "" || len(t.Title) > 2000 || len(t.Channel) > 2000 || len(t.Thumbnail) > 4000 || math.IsNaN(t.Duration) || math.IsInf(t.Duration, 0) || t.Duration < 0 {
			return errors.New("invalid YouTube track metadata")
		}
		t.URL = "https://www.youtube.com/watch?v=" + t.ProviderID
	}
	return nil
}
func validateLibrary(l *playlistLibrary) error {
	if l.Version != 1 {
		return errors.New("unsupported playlist file version")
	}
	if len(l.Playlists) > 1000 {
		return errors.New("a library can contain at most 1000 playlists")
	}
	for i := range l.Playlists {
		if err := validatePlaylist(&l.Playlists[i]); err != nil {
			return err
		}
	}
	return nil
}
func (manager *Manager) SavePlaylist(id, name string, replaceTracks bool) (Playlist, error) {
	manager.mu.Lock()
	defer manager.mu.Unlock()
	next := playlistLibrary{Version: 1, Playlists: append([]Playlist{}, manager.library.Playlists...)}
	index := -1
	for i, p := range next.Playlists {
		if p.ID == id {
			index = i
			break
		}
	}
	if id != "" && index < 0 {
		return Playlist{}, errors.New("playlist not found")
	}
	p := Playlist{ID: randomID(16), CreatedAt: time.Now().UTC()}
	if index >= 0 {
		p = next.Playlists[index]
		p.Tracks = append([]youtube.Track(nil), p.Tracks...)
	}
	p.Name = name
	p.UpdatedAt = time.Now().UTC()
	if replaceTracks {
		p.Tracks = []youtube.Track{}
		for _, t := range manager.combinedTracksLocked() {
			p.Tracks = append(p.Tracks, youtube.Track{Source: t.Source, ProviderID: t.ProviderID, URL: t.URL, Title: t.Title, Channel: t.Channel, Thumbnail: t.Thumbnail, Duration: t.Duration})
		}
	}
	if err := validatePlaylist(&p); err != nil {
		return Playlist{}, err
	}
	if index < 0 {
		next.Playlists = append(next.Playlists, p)
	} else {
		next.Playlists[index] = p
	}
	if len(next.Playlists) > 1000 {
		return Playlist{}, errors.New("playlist library is full")
	}
	if err := manager.writeLibrary(next); err != nil {
		return Playlist{}, err
	}
	if replaceTracks {
		manager.playlist = &PlaylistLink{ID: p.ID, Name: p.Name}
	} else if manager.playlist != nil && manager.playlist.ID == p.ID {
		manager.playlist.Name = p.Name
	}
	manager.changedLocked()
	return p, nil
}
func (manager *Manager) DeletePlaylist(id string) error {
	manager.mu.Lock()
	defer manager.mu.Unlock()
	next := playlistLibrary{Version: 1, Playlists: []Playlist{}}
	found := false
	for _, p := range manager.library.Playlists {
		if p.ID == id {
			found = true
		} else {
			next.Playlists = append(next.Playlists, p)
		}
	}
	if !found {
		return errors.New("playlist not found")
	}
	if err := manager.writeLibrary(next); err != nil {
		return err
	}
	if manager.playlist != nil && manager.playlist.ID == id {
		manager.playlist = nil
	}
	manager.changedLocked()
	return nil
}
func (manager *Manager) ImportPlaylists(backup PlaylistBackup) error {
	manager.mu.Lock()
	defer manager.mu.Unlock()
	if err := validateLibrary(&backup); err != nil {
		return err
	}
	next := playlistLibrary{Version: 1, Playlists: append([]Playlist{}, manager.library.Playlists...)}
	for _, p := range backup.Playlists {
		p.ID = randomID(16)
		p.CreatedAt = time.Now().UTC()
		p.UpdatedAt = p.CreatedAt
		next.Playlists = append(next.Playlists, p)
	}
	if len(next.Playlists) > 1000 {
		return errors.New("playlist library is full")
	}
	return manager.writeLibrary(next)
}
func (manager *Manager) LoadPlaylist(id, mode string, discard bool) (Snapshot, int, error) {
	manager.mu.Lock()
	defer manager.mu.Unlock()
	if manager.libraryError != nil {
		return Snapshot{}, 0, manager.libraryError
	}
	if mode != "append" && mode != "replace" {
		return Snapshot{}, 0, errors.New("choose append or replace")
	}
	if mode == "replace" && manager.playlist != nil && manager.playlist.Dirty && !discard {
		return Snapshot{}, 0, errors.New("save or discard playlist changes before replacing the queue")
	}
	var selected *Playlist
	for _, p := range manager.library.Playlists {
		if p.ID == id {
			copy := p
			selected = &copy
			break
		}
	}
	if selected == nil {
		return Snapshot{}, 0, errors.New("playlist not found")
	}
	seen := map[string]bool{}
	if mode == "append" {
		if manager.current != nil {
			seen[manager.current.ProviderID] = true
		}
		for _, t := range manager.upcoming {
			seen[t.ProviderID] = true
		}
	}
	tracks := []Track{}
	skipped := 0
	for _, t := range selected.Tracks {
		if seen[t.ProviderID] {
			skipped++
			continue
		}
		seen[t.ProviderID] = true
		tracks = append(tracks, Track{QueueID: randomID(16), Source: t.Source, ProviderID: t.ProviderID, URL: t.URL, Title: t.Title, Channel: t.Channel, Thumbnail: t.Thumbnail, Duration: t.Duration, AddedBy: "DJ"})
	}
	capacity := MaxQueueTracks + 1
	if mode == "append" {
		capacity = MaxQueueTracks - len(manager.upcoming)
		if manager.current == nil {
			capacity++
		}
	}
	if len(tracks) > capacity {
		return Snapshot{}, 0, errors.New("playlist exceeds the queue capacity; remove some tracks first")
	}
	if mode == "replace" {
		manager.player.Stop()
		manager.history = nil
		manager.current = nil
		manager.upcoming = nil
		manager.handledGeneration = -1
		manager.playlist = &PlaylistLink{ID: selected.ID, Name: selected.Name, Dirty: skipped > 0}
	} else if len(tracks) > 0 {
		manager.markPlaylistDirtyLocked()
	}
	if manager.current == nil && len(tracks) > 0 {
		t := tracks[0]
		manager.current = &t
		tracks = tracks[1:]
	}
	manager.upcoming = append(manager.upcoming, tracks...)
	manager.changedLocked()
	return manager.snapshotLocked("", true), skipped, nil
}
