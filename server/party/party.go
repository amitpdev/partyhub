package party

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"partyhub/server/player"
	"partyhub/server/youtube"
)

const (
	MaxGuestTracks   = 3
	MaxQueueTracks   = 100
	MaxHistoryTracks = 100
	MaxTrackSeconds  = 15 * 60
)

type Track struct {
	QueueID    string  `json:"queueId"`
	Source     string  `json:"source"`
	ProviderID string  `json:"providerId"`
	URL        string  `json:"url"`
	Title      string  `json:"title"`
	Channel    string  `json:"channel"`
	Thumbnail  string  `json:"thumbnail"`
	Duration   float64 `json:"duration"`
	AddedBy    string  `json:"addedBy"`
	CanRemove  bool    `json:"canRemove"`
	CanMove    bool    `json:"canMove"`
	CanPlay    bool    `json:"canPlay"`
	ownerID    string
}

type Snapshot struct {
	Revision     int64         `json:"revision"`
	Tracks       []Track       `json:"tracks"`
	CurrentIndex int           `json:"currentIndex"`
	Volume       float64       `json:"volume"`
	Player       player.Status `json:"player"`
	StorageError string        `json:"storageError,omitempty"`
}

type ShareInfo struct {
	Active    bool   `json:"active"`
	PublicURL string `json:"publicUrl"`
	GuestURL  string `json:"guestUrl"`
}

type Session struct {
	ID   string
	Name string
}

type Player interface {
	Load(trackID, rawURL string, volume float64) (player.Status, error)
	Pause(paused bool) (player.Status, error)
	Seek(seconds float64) (player.Status, error)
	SeekTo(seconds float64) (player.Status, error)
	SetVolume(volume float64) (player.Status, error)
	Stop() player.Status
	Status() player.Status
}

type Resolver interface {
	Resolve(rawURL string) (youtube.Track, error)
}

type Manager struct {
	mu                sync.Mutex
	player            Player
	resolver          Resolver
	statePath         string
	history           []Track
	current           *Track
	upcoming          []Track
	volume            float64
	revision          int64
	handledGeneration int
	storageError      string
	shareActive       bool
	publicURL         string
	guestToken        string
	sessions          map[string]Session
	attempts          map[string][]time.Time
}

type persistedState struct {
	Version      int              `json:"version"`
	History      []persistedTrack `json:"history,omitempty"`
	Current      *persistedTrack  `json:"current,omitempty"`
	Upcoming     []persistedTrack `json:"upcoming,omitempty"`
	Volume       float64          `json:"volume"`
	Tracks       []persistedTrack `json:"tracks,omitempty"`
	CurrentIndex int              `json:"currentIndex,omitempty"`
}

type persistedTrack struct {
	QueueID    string  `json:"queueId"`
	Source     string  `json:"source"`
	ProviderID string  `json:"providerId"`
	URL        string  `json:"url"`
	Title      string  `json:"title"`
	Channel    string  `json:"channel"`
	Thumbnail  string  `json:"thumbnail"`
	Duration   float64 `json:"duration"`
	AddedBy    string  `json:"addedBy"`
	OwnerID    string  `json:"ownerId"`
}

func DefaultStatePath() string {
	if configured := strings.TrimSpace(os.Getenv("PARTYHUB_STATE_PATH")); configured != "" {
		return configured
	}
	directory, err := os.UserConfigDir()
	if err != nil {
		return ""
	}
	return filepath.Join(directory, "partyhub", "state.json")
}

func New(localPlayer Player, resolver Resolver, statePath string) *Manager {
	manager := &Manager{
		player:     localPlayer,
		resolver:   resolver,
		statePath:  statePath,
		volume:     80,
		guestToken: randomID(24),
		sessions:   make(map[string]Session),
		attempts:   make(map[string][]time.Time),
	}
	manager.load()
	return manager
}

func (manager *Manager) Snapshot(viewerID string, isDJ bool) Snapshot {
	status := manager.player.Status()
	manager.mu.Lock()
	defer manager.mu.Unlock()
	return manager.snapshotWithStatusLocked(viewerID, isDJ, status)
}

func (manager *Manager) Add(rawURL string, owner Session, isDJ bool) (Snapshot, error) {
	if !isDJ {
		if err := manager.allowAttempt(owner.ID); err != nil {
			return Snapshot{}, err
		}
	}
	resolved, err := manager.resolver.Resolve(rawURL)
	if err != nil {
		return Snapshot{}, err
	}
	if !isDJ {
		if resolved.Duration <= 0 {
			return Snapshot{}, errors.New("live streams and videos with unknown duration are not supported")
		}
		if resolved.Duration > MaxTrackSeconds {
			return Snapshot{}, errors.New("guest tracks must be 15 minutes or shorter")
		}
	}

	manager.mu.Lock()
	defer manager.mu.Unlock()
	if len(manager.upcoming) >= MaxQueueTracks {
		return Snapshot{}, errors.New("the queue is full")
	}
	if manager.current != nil && manager.current.ProviderID == resolved.ProviderID {
		return Snapshot{}, fmt.Errorf("this video is already at queue position %d", len(manager.history)+1)
	}
	for index, track := range manager.upcoming {
		if track.ProviderID == resolved.ProviderID {
			return Snapshot{}, fmt.Errorf(
				"this video is already at queue position %d",
				len(manager.history)+index+2,
			)
		}
	}
	if !isDJ {
		pending := 0
		for _, track := range manager.upcoming {
			if track.ownerID == owner.ID {
				pending++
			}
		}
		if pending >= MaxGuestTracks {
			return Snapshot{}, fmt.Errorf("you can have up to %d upcoming tracks", MaxGuestTracks)
		}
	}

	addedBy := "DJ"
	if !isDJ {
		addedBy = owner.Name
	}
	track := Track{
		QueueID:    randomID(16),
		Source:     resolved.Source,
		ProviderID: resolved.ProviderID,
		URL:        resolved.URL,
		Title:      resolved.Title,
		Channel:    resolved.Channel,
		Thumbnail:  resolved.Thumbnail,
		Duration:   resolved.Duration,
		AddedBy:    addedBy,
		ownerID:    owner.ID,
	}
	status := manager.player.Status()
	if manager.current == nil {
		manager.current = &track
		manager.loadCurrentLocked()
	} else if len(manager.upcoming) == 0 && (status.State == "ended" || status.State == "error") {
		manager.finishCurrentLocked()
		manager.current = &track
		manager.loadCurrentLocked()
	} else {
		manager.upcoming = append(manager.upcoming, track)
	}
	manager.changedLocked()
	return manager.snapshotLocked(owner.ID, isDJ), nil
}

func (manager *Manager) Import(tracks []youtube.Track, currentIndex int, volume float64) {
	manager.mu.Lock()
	defer manager.mu.Unlock()
	if manager.current != nil || len(manager.history) > 0 || len(manager.upcoming) > 0 {
		return
	}
	seen := make(map[string]bool)
	imported := make([]Track, 0, min(len(tracks), MaxQueueTracks))
	for _, track := range tracks {
		if len(imported) >= MaxQueueTracks || seen[track.ProviderID] ||
			track.ProviderID == "" || track.URL == "" {
			continue
		}
		seen[track.ProviderID] = true
		imported = append(imported, Track{
			QueueID: randomID(16), Source: track.Source, ProviderID: track.ProviderID,
			URL: track.URL, Title: track.Title, Channel: track.Channel,
			Thumbnail: track.Thumbnail, Duration: track.Duration, AddedBy: "DJ",
		})
	}
	if len(imported) > 0 {
		index := max(0, min(len(imported)-1, currentIndex))
		manager.history = append(manager.history, imported[:index]...)
		current := imported[index]
		manager.current = &current
		manager.upcoming = append(manager.upcoming, imported[index+1:]...)
	}
	manager.volume = max(0, min(100, volume))
	manager.changedLocked()
}

func (manager *Manager) Remove(queueID, viewerID string, isDJ bool) error {
	manager.mu.Lock()
	defer manager.mu.Unlock()
	if manager.current != nil && manager.current.QueueID == queueID {
		return errors.New("the current track cannot be removed")
	}
	for index, track := range manager.upcoming {
		if track.QueueID != queueID {
			continue
		}
		if !isDJ && (track.ownerID == "" || track.ownerID != viewerID) {
			return errors.New("guests can only remove their own upcoming tracks")
		}
		manager.upcoming = append(manager.upcoming[:index], manager.upcoming[index+1:]...)
		manager.changedLocked()
		return nil
	}
	for index, track := range manager.history {
		if track.QueueID != queueID {
			continue
		}
		if !isDJ {
			return errors.New("guests can only remove their own upcoming tracks")
		}
		manager.history = append(manager.history[:index], manager.history[index+1:]...)
		manager.changedLocked()
		return nil
	}
	return errors.New("track not found")
}

func (manager *Manager) Move(queueID string, direction int) error {
	manager.mu.Lock()
	defer manager.mu.Unlock()
	index := manager.upcomingIndexLocked(queueID)
	destination := index + direction
	if index < 0 || destination < 0 || destination >= len(manager.upcoming) {
		return errors.New("track cannot be moved there")
	}
	manager.upcoming[index], manager.upcoming[destination] =
		manager.upcoming[destination], manager.upcoming[index]
	manager.changedLocked()
	return nil
}

func (manager *Manager) Clear() {
	manager.mu.Lock()
	defer manager.mu.Unlock()
	manager.player.Stop()
	manager.history = nil
	manager.current = nil
	manager.upcoming = nil
	manager.handledGeneration = -1
	manager.changedLocked()
}

func (manager *Manager) Play(index int) error {
	manager.mu.Lock()
	defer manager.mu.Unlock()
	tracks := manager.combinedTracksLocked()
	if index < 0 || index >= len(tracks) {
		return errors.New("track not found")
	}
	manager.history = append([]Track(nil), tracks[:index]...)
	if len(manager.history) > MaxHistoryTracks {
		manager.history = append([]Track(nil), manager.history[len(manager.history)-MaxHistoryTracks:]...)
	}
	current := tracks[index]
	manager.current = &current
	manager.upcoming = append([]Track(nil), tracks[index+1:]...)
	manager.loadCurrentLocked()
	manager.changedLocked()
	return nil
}

func (manager *Manager) Next() error {
	manager.mu.Lock()
	defer manager.mu.Unlock()
	if len(manager.upcoming) == 0 {
		manager.player.Stop()
		return errors.New("there is no track in that direction")
	}
	manager.finishCurrentLocked()
	manager.promoteNextLocked()
	manager.loadCurrentLocked()
	manager.changedLocked()
	return nil
}

func (manager *Manager) Previous() error {
	manager.mu.Lock()
	defer manager.mu.Unlock()
	if len(manager.history) == 0 {
		return errors.New("there is no track in that direction")
	}
	if manager.current != nil {
		manager.upcoming = append([]Track{*manager.current}, manager.upcoming...)
	}
	index := len(manager.history) - 1
	current := manager.history[index]
	manager.history = manager.history[:index]
	manager.current = &current
	manager.loadCurrentLocked()
	manager.changedLocked()
	return nil
}

func (manager *Manager) Pause(paused bool) (player.Status, error) {
	return manager.player.Pause(paused)
}

func (manager *Manager) Seek(seconds float64) (player.Status, error) {
	return manager.player.Seek(seconds)
}

func (manager *Manager) SeekTo(seconds float64) (player.Status, error) {
	return manager.player.SeekTo(seconds)
}

func (manager *Manager) SetVolume(volume float64) (player.Status, error) {
	status, err := manager.player.SetVolume(volume)
	if err != nil {
		return status, err
	}
	manager.mu.Lock()
	manager.volume = max(0, min(100, volume))
	manager.changedLocked()
	manager.mu.Unlock()
	return status, nil
}

func (manager *Manager) Stop() player.Status {
	status := manager.player.Stop()
	manager.mu.Lock()
	manager.revision++
	manager.mu.Unlock()
	return status
}

func (manager *Manager) Tick() {
	status := manager.player.Status()
	if status.State != "ended" && status.State != "error" {
		return
	}
	manager.mu.Lock()
	defer manager.mu.Unlock()
	if status.Generation == manager.handledGeneration {
		return
	}
	manager.handledGeneration = status.Generation
	if manager.current != nil {
		manager.finishCurrentLocked()
		if manager.promoteNextLocked() {
			manager.loadCurrentLocked()
		}
		manager.changedLocked()
	}
}

func (manager *Manager) CreateSession(token, name string) (Session, error) {
	manager.mu.Lock()
	defer manager.mu.Unlock()
	name = strings.TrimSpace(name)
	if !manager.shareActive || token == "" || token != manager.guestToken {
		return Session{}, errors.New("this party link is no longer active")
	}
	if len([]rune(name)) < 1 || len([]rune(name)) > 30 {
		return Session{}, errors.New("enter a name between 1 and 30 characters")
	}
	session := Session{ID: randomID(20), Name: name}
	manager.sessions[session.ID] = session
	return session, nil
}

func (manager *Manager) Session(id string) (Session, bool) {
	manager.mu.Lock()
	defer manager.mu.Unlock()
	session, ok := manager.sessions[id]
	return session, ok && manager.shareActive
}

func (manager *Manager) ConfigureShare(publicURL string) (ShareInfo, error) {
	publicURL = strings.TrimRight(strings.TrimSpace(publicURL), "/")
	if !strings.HasPrefix(publicURL, "https://") {
		return ShareInfo{}, errors.New("ngrok must provide an HTTPS URL")
	}
	manager.mu.Lock()
	defer manager.mu.Unlock()
	manager.shareActive = true
	manager.publicURL = publicURL
	manager.guestToken = randomID(24)
	manager.sessions = make(map[string]Session)
	manager.attempts = make(map[string][]time.Time)
	manager.revision++
	return manager.shareInfoLocked(), nil
}

func (manager *Manager) EndShare() {
	manager.mu.Lock()
	defer manager.mu.Unlock()
	manager.shareActive = false
	manager.publicURL = ""
	manager.guestToken = randomID(24)
	manager.sessions = make(map[string]Session)
	manager.attempts = make(map[string][]time.Time)
	manager.revision++
}

func (manager *Manager) ShareInfo() ShareInfo {
	manager.mu.Lock()
	defer manager.mu.Unlock()
	return manager.shareInfoLocked()
}

func (manager *Manager) shareInfoLocked() ShareInfo {
	info := ShareInfo{Active: manager.shareActive, PublicURL: manager.publicURL}
	if manager.shareActive {
		info.GuestURL = manager.publicURL + "/guest#token=" + manager.guestToken
	}
	return info
}

func (manager *Manager) loadCurrentLocked() {
	if manager.current == nil {
		return
	}
	status, err := manager.player.Load(
		manager.current.QueueID,
		manager.current.URL,
		manager.volume,
	)
	if err == nil {
		manager.handledGeneration = status.Generation - 1
	}
}

func (manager *Manager) upcomingIndexLocked(queueID string) int {
	for index, track := range manager.upcoming {
		if track.QueueID == queueID {
			return index
		}
	}
	return -1
}

func (manager *Manager) finishCurrentLocked() {
	if manager.current == nil {
		return
	}
	manager.history = append(manager.history, *manager.current)
	if len(manager.history) > MaxHistoryTracks {
		manager.history = append([]Track(nil), manager.history[len(manager.history)-MaxHistoryTracks:]...)
	}
	manager.current = nil
}

func (manager *Manager) promoteNextLocked() bool {
	if len(manager.upcoming) == 0 {
		manager.current = nil
		return false
	}
	current := manager.upcoming[0]
	manager.current = &current
	manager.upcoming = manager.upcoming[1:]
	return true
}

func (manager *Manager) combinedTracksLocked() []Track {
	tracks := make([]Track, 0, len(manager.history)+len(manager.upcoming)+1)
	tracks = append(tracks, manager.history...)
	if manager.current != nil {
		tracks = append(tracks, *manager.current)
	}
	tracks = append(tracks, manager.upcoming...)
	return tracks
}

func (manager *Manager) changedLocked() {
	manager.revision++
	if err := manager.persistLocked(); err != nil {
		manager.storageError = fmt.Sprintf(
			"Queue changes are only in memory and may be lost if Party Hub closes. Check write access to %s, then make another queue change to retry. (%v)",
			filepath.Dir(manager.statePath),
			err,
		)
	} else {
		manager.storageError = ""
	}
}

func (manager *Manager) snapshotLocked(viewerID string, isDJ bool) Snapshot {
	return manager.snapshotWithStatusLocked(viewerID, isDJ, manager.player.Status())
}

func (manager *Manager) snapshotWithStatusLocked(
	viewerID string,
	isDJ bool,
	status player.Status,
) Snapshot {
	tracks := manager.combinedTracksLocked()
	for index := range tracks {
		isHistory := index < len(manager.history)
		isCurrent := manager.current != nil && index == len(manager.history)
		isUpcoming := !isHistory && !isCurrent
		tracks[index].CanPlay = isDJ
		tracks[index].CanMove = isDJ && isUpcoming
		tracks[index].CanRemove = !isCurrent &&
			(isDJ || (isUpcoming && tracks[index].ownerID != "" && tracks[index].ownerID == viewerID))
	}
	currentIndex := -1
	if manager.current != nil || len(manager.history) > 0 {
		currentIndex = len(manager.history)
	}
	storageError := ""
	if isDJ {
		storageError = manager.storageError
	}
	return Snapshot{
		Revision:     manager.revision,
		Tracks:       tracks,
		CurrentIndex: currentIndex,
		Volume:       manager.volume,
		Player:       status,
		StorageError: storageError,
	}
}

func (manager *Manager) allowAttempt(sessionID string) error {
	manager.mu.Lock()
	defer manager.mu.Unlock()
	now := time.Now()
	cutoff := now.Add(-time.Minute)
	recent := manager.attempts[sessionID][:0]
	for _, attempt := range manager.attempts[sessionID] {
		if attempt.After(cutoff) {
			recent = append(recent, attempt)
		}
	}
	if len(recent) >= 10 {
		manager.attempts[sessionID] = recent
		return errors.New("too many requests; wait a minute and try again")
	}
	manager.attempts[sessionID] = append(recent, now)
	return nil
}

func (manager *Manager) load() {
	if manager.statePath == "" {
		return
	}
	data, err := os.ReadFile(manager.statePath)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			manager.storageError = fmt.Sprintf(
				"The saved queue could not be loaded from %s. Check that the file is readable, then restart Party Hub. (%v)",
				manager.statePath,
				err,
			)
		}
		return
	}
	var saved persistedState
	if err := json.Unmarshal(data, &saved); err != nil {
		manager.storageError = fmt.Sprintf(
			"The saved queue at %s is invalid. Move or repair the file, then restart Party Hub. (%v)",
			manager.statePath,
			err,
		)
		return
	}
	if saved.Version >= 2 {
		manager.history = persistedTracksToTracks(saved.History)
		if saved.Current != nil {
			current := persistedTrackToTrack(*saved.Current)
			manager.current = &current
		}
		manager.upcoming = persistedTracksToTracks(saved.Upcoming)
	} else {
		tracks := persistedTracksToTracks(saved.Tracks)
		if len(tracks) > 0 {
			index := max(0, min(len(tracks)-1, saved.CurrentIndex))
			manager.history = append(manager.history, tracks[:index]...)
			current := tracks[index]
			manager.current = &current
			manager.upcoming = append(manager.upcoming, tracks[index+1:]...)
		}
	}
	manager.volume = max(0, min(100, saved.Volume))
	if manager.volume == 0 && saved.Volume == 0 {
		manager.volume = 0
	}
}

func (manager *Manager) persistLocked() error {
	if manager.statePath == "" {
		return nil
	}
	saved := persistedState{
		Version:  2,
		History:  tracksToPersistedTracks(manager.history),
		Upcoming: tracksToPersistedTracks(manager.upcoming),
		Volume:   manager.volume,
	}
	if manager.current != nil {
		current := trackToPersistedTrack(*manager.current)
		saved.Current = &current
	}
	data, err := json.MarshalIndent(saved, "", "  ")
	if err != nil {
		return fmt.Errorf("encode queue: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(manager.statePath), 0o700); err != nil {
		return fmt.Errorf("create state directory: %w", err)
	}
	temporary := manager.statePath + ".tmp"
	if err := os.WriteFile(temporary, data, 0o600); err != nil {
		return fmt.Errorf("write queue: %w", err)
	}
	if err := os.Rename(temporary, manager.statePath); err != nil {
		_ = os.Remove(temporary)
		return fmt.Errorf("replace saved queue: %w", err)
	}
	return nil
}

func persistedTrackToTrack(track persistedTrack) Track {
	return Track{
		QueueID: track.QueueID, Source: track.Source, ProviderID: track.ProviderID,
		URL: track.URL, Title: track.Title, Channel: track.Channel,
		Thumbnail: track.Thumbnail, Duration: track.Duration, AddedBy: track.AddedBy,
		ownerID: track.OwnerID,
	}
}

func persistedTracksToTracks(tracks []persistedTrack) []Track {
	converted := make([]Track, 0, len(tracks))
	for _, track := range tracks {
		converted = append(converted, persistedTrackToTrack(track))
	}
	return converted
}

func trackToPersistedTrack(track Track) persistedTrack {
	return persistedTrack{
		QueueID: track.QueueID, Source: track.Source, ProviderID: track.ProviderID,
		URL: track.URL, Title: track.Title, Channel: track.Channel,
		Thumbnail: track.Thumbnail, Duration: track.Duration, AddedBy: track.AddedBy,
		OwnerID: track.ownerID,
	}
}

func tracksToPersistedTracks(tracks []Track) []persistedTrack {
	converted := make([]persistedTrack, 0, len(tracks))
	for _, track := range tracks {
		converted = append(converted, trackToPersistedTrack(track))
	}
	return converted
}

func randomID(bytes int) string {
	value := make([]byte, bytes)
	if _, err := rand.Read(value); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(value)
}
