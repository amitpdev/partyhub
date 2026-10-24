package player

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type Status struct {
	Available  bool     `json:"available"`
	Missing    []string `json:"missing"`
	State      string   `json:"state"`
	Generation int      `json:"generation"`
	TrackID    string   `json:"trackId"`
	Position   float64  `json:"position"`
	Duration   float64  `json:"duration"`
	Volume     float64  `json:"volume"`
	Error      string   `json:"error,omitempty"`
}

type Player struct {
	mu         sync.Mutex
	command    *exec.Cmd
	socket     string
	generation int
	status     Status
}

func New() *Player {
	missing := MissingDependencies()
	return &Player{
		status: Status{
			Available: len(missing) == 0,
			Missing:   missing,
			State:     "idle",
			Volume:    80,
		},
	}
}

func MissingDependencies() []string {
	var missing []string
	for _, name := range []string{"yt-dlp", "mpv"} {
		if _, err := exec.LookPath(name); err != nil {
			missing = append(missing, name)
		}
	}
	return missing
}

func CommandArgs(socket, rawURL string, volume float64) []string {
	return []string{
		"--no-video",
		"--force-window=no",
		"--really-quiet",
		"--input-ipc-server=" + socket,
		fmt.Sprintf("--volume=%.0f", clamp(volume, 0, 100)),
		"--",
		rawURL,
	}
}

func (player *Player) Load(trackID, rawURL string, volume float64) (Status, error) {
	player.mu.Lock()
	defer player.mu.Unlock()
	if !player.status.Available {
		return player.status, fmt.Errorf("missing required programs: %s", strings.Join(player.status.Missing, ", "))
	}
	player.stopLocked()
	player.generation++
	generation := player.generation
	socket := filepath.Join(os.TempDir(), fmt.Sprintf("partyhub-%d-%d.sock", os.Getpid(), generation))
	_ = os.Remove(socket)
	command := exec.Command("mpv", CommandArgs(socket, rawURL, volume)...)
	command.Stdin = nil
	command.Stdout = nil
	var stderr strings.Builder
	command.Stderr = &stderr
	player.status = Status{
		Available:  true,
		State:      "loading",
		Generation: generation,
		TrackID:    trackID,
		Volume:     clamp(volume, 0, 100),
	}
	if err := command.Start(); err != nil {
		player.status.State = "error"
		player.status.Error = err.Error()
		return player.status, err
	}
	player.command = command
	player.socket = socket
	player.status.State = "playing"
	go player.wait(command, generation, socket, &stderr)
	return player.status, nil
}

func (player *Player) wait(command *exec.Cmd, generation int, socket string, stderr *strings.Builder) {
	err := command.Wait()
	player.mu.Lock()
	defer player.mu.Unlock()
	if generation != player.generation {
		_ = os.Remove(socket)
		return
	}
	player.command = nil
	player.socket = ""
	_ = os.Remove(socket)
	if err != nil {
		player.status.State = "error"
		player.status.Error = diagnostic(stderr.String(), err)
		return
	}
	player.status.State = "ended"
	player.status.Position = player.status.Duration
}

func (player *Player) Pause(paused bool) (Status, error) {
	player.mu.Lock()
	defer player.mu.Unlock()
	if player.command == nil {
		return player.status, errors.New("nothing is playing")
	}
	if err := sendCommand(player.socket, []any{"set_property", "pause", paused}); err != nil {
		return player.status, err
	}
	if paused {
		player.status.State = "paused"
	} else {
		player.status.State = "playing"
	}
	return player.status, nil
}

func (player *Player) Seek(seconds float64) (Status, error) {
	player.mu.Lock()
	defer player.mu.Unlock()
	if player.command == nil {
		return player.status, errors.New("nothing is playing")
	}
	if err := sendCommand(player.socket, []any{"seek", seconds, "relative+exact"}); err != nil {
		return player.status, err
	}
	player.status.Position = clamp(player.status.Position+seconds, 0, player.status.Duration)
	return player.status, nil
}

func (player *Player) SeekTo(seconds float64) (Status, error) {
	player.mu.Lock()
	defer player.mu.Unlock()
	if player.command == nil {
		return player.status, errors.New("nothing is playing")
	}
	seconds = max(0, seconds)
	if player.status.Duration > 0 {
		seconds = min(seconds, player.status.Duration)
	}
	if err := sendCommand(player.socket, []any{"seek", seconds, "absolute+exact"}); err != nil {
		return player.status, err
	}
	player.status.Position = seconds
	return player.status, nil
}

func (player *Player) SetVolume(volume float64) (Status, error) {
	player.mu.Lock()
	defer player.mu.Unlock()
	volume = clamp(volume, 0, 100)
	player.status.Volume = volume
	if player.command == nil {
		return player.status, nil
	}
	if err := sendCommand(player.socket, []any{"set_property", "volume", volume}); err != nil {
		return player.status, err
	}
	return player.status, nil
}

func (player *Player) Stop() Status {
	player.mu.Lock()
	defer player.mu.Unlock()
	player.generation++
	player.stopLocked()
	player.status.State = "stopped"
	player.status.Generation = player.generation
	player.status.TrackID = ""
	player.status.Position = 0
	player.status.Duration = 0
	player.status.Error = ""
	return player.status
}

func (player *Player) Close() {
	player.Stop()
}

func (player *Player) Status() Status {
	player.mu.Lock()
	status := player.status
	socket := player.socket
	active := player.command != nil
	player.mu.Unlock()
	if !active {
		return status
	}
	if value, err := getNumber(socket, "time-pos"); err == nil {
		status.Position = value
	}
	if value, err := getNumber(socket, "duration"); err == nil {
		status.Duration = value
	}
	player.mu.Lock()
	if status.Generation == player.generation {
		player.status.Position = status.Position
		player.status.Duration = status.Duration
	}
	player.mu.Unlock()
	return status
}

func (player *Player) stopLocked() {
	if player.command != nil && player.command.Process != nil {
		_ = player.command.Process.Kill()
	}
	if player.socket != "" {
		_ = os.Remove(player.socket)
	}
	player.command = nil
	player.socket = ""
}

func sendCommand(socket string, command []any) error {
	connection, err := net.DialTimeout("unix", socket, 700*time.Millisecond)
	if err != nil {
		return err
	}
	defer connection.Close()
	_ = connection.SetDeadline(time.Now().Add(time.Second))
	if err := json.NewEncoder(connection).Encode(map[string]any{"command": command}); err != nil {
		return err
	}
	var response struct {
		Error string `json:"error"`
	}
	if err := json.NewDecoder(connection).Decode(&response); err != nil {
		return err
	}
	if response.Error != "success" {
		return errors.New(response.Error)
	}
	return nil
}

func getNumber(socket, property string) (float64, error) {
	connection, err := net.DialTimeout("unix", socket, 300*time.Millisecond)
	if err != nil {
		return 0, err
	}
	defer connection.Close()
	_ = connection.SetDeadline(time.Now().Add(500 * time.Millisecond))
	if err := json.NewEncoder(connection).Encode(map[string]any{
		"command": []any{"get_property", property},
	}); err != nil {
		return 0, err
	}
	var response struct {
		Data  float64 `json:"data"`
		Error string  `json:"error"`
	}
	if err := json.NewDecoder(connection).Decode(&response); err != nil {
		return 0, err
	}
	if response.Error != "success" {
		return 0, errors.New(response.Error)
	}
	return response.Data, nil
}

func diagnostic(output string, fallback error) string {
	lines := strings.Split(strings.TrimSpace(output), "\n")
	for index := len(lines) - 1; index >= 0; index-- {
		if line := strings.TrimSpace(lines[index]); line != "" {
			return line
		}
	}
	return fallback.Error()
}

func clamp(value, low, high float64) float64 {
	return min(high, max(low, value))
}
