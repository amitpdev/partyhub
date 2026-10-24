package main

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"partyhub/server/party"
)

func registerPlaylists(mux *http.ServeMux, manager *party.Manager) {
	mux.HandleFunc("/api/playlists", playlistHandler(manager))
}
func playlistHandler(manager *party.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !isLocalDJ(r) {
			writeError(w, http.StatusForbidden, errors.New("playlist management is only available on the host laptop"))
			return
		}
		path := r.URL.Path
		if r.Method == "GET" && path == "/api/playlists" {
			library, err := manager.Playlists()
			if err != nil {
				writeError(w, 500, err)
				return
			}
			writeJSON(w, 200, library)
			return
		}
		if r.Method != "POST" {
			writeError(w, 405, errors.New("method not allowed"))
			return
		}
		var body struct {
			Action  string               `json:"action"`
			ID      string               `json:"id"`
			Name    string               `json:"name"`
			Mode    string               `json:"mode"`
			Discard bool                 `json:"discard"`
			Backup  party.PlaylistBackup `json:"backup"`
		}
		r.Body = http.MaxBytesReader(w, r.Body, 8*1024*1024)
		decoder := json.NewDecoder(r.Body)
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&body); err != nil {
			writeError(w, 400, errors.New("invalid playlist request or file too large (8 MB maximum)"))
			return
		}
		if err := decoder.Decode(&struct{}{}); err != io.EOF {
			writeError(w, 400, errors.New("unexpected trailing content"))
			return
		}
		var err error
		switch body.Action {
		case "save", "rename":
			_, err = manager.SavePlaylist(body.ID, body.Name, body.Action == "save")
		case "delete":
			err = manager.DeletePlaylist(body.ID)
		case "import":
			err = manager.ImportPlaylists(body.Backup)
		case "load":
			snapshot, skipped, loadErr := manager.LoadPlaylist(body.ID, body.Mode, body.Discard)
			if loadErr != nil {
				writeError(w, 409, loadErr)
				return
			}
			writeJSON(w, 200, map[string]any{"party": snapshot, "skipped": skipped})
			return
		default:
			err = errors.New("unknown playlist action")
		}
		if err != nil {
			writeError(w, 400, err)
			return
		}
		writeJSON(w, 200, manager.Snapshot("", true))
	}
}
