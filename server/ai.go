package main

import (
	"context"
	"errors"
	"net/http"
	"sync"
	"time"

	"partyhub/server/ai"
	"partyhub/server/party"
	"partyhub/server/youtube"
)

func registerAI(mux *http.ServeMux, service *ai.Service, manager *party.Manager, searcher youtubeSearcher, coordinator interface {
	Begin(context.Context, string) (context.Context, func(), error)
}) {
	var pickMu sync.Mutex
	local := func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Cache-Control", "no-store")
			if !isLocalDJ(r) {
				writeError(w, http.StatusForbidden, errors.New("AI controls are only available on the host laptop"))
				return
			}
			next(w, r)
		}
	}
	mux.HandleFunc("GET /api/ai/settings", local(func(w http.ResponseWriter, r *http.Request) {
		settings, err := service.Settings()
		if err != nil {
			writeError(w, 500, err)
			return
		}
		writeJSON(w, 200, settings)
	}))
	mux.HandleFunc("POST /api/ai/settings", local(func(w http.ResponseWriter, r *http.Request) {
		var draft ai.Draft
		if decode(r, &draft) != nil {
			writeError(w, 400, errors.New("invalid AI settings"))
			return
		}
		settings, err := service.Save(draft)
		if err != nil {
			writeError(w, 400, err)
			return
		}
		writeJSON(w, 200, settings)
	}))
	mux.HandleFunc("DELETE /api/ai/settings", local(func(w http.ResponseWriter, r *http.Request) {
		if err := service.Clear(); err != nil {
			writeError(w, 400, err)
			return
		}
		settings, _ := service.Settings()
		writeJSON(w, 200, settings)
	}))
	mux.HandleFunc("POST /api/ai/test", local(func(w http.ResponseWriter, r *http.Request) {
		var draft ai.Draft
		if decode(r, &draft) != nil {
			writeError(w, 400, errors.New("invalid AI settings"))
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 40*time.Second)
		defer cancel()
		if err := service.Test(ctx, draft); err != nil {
			writeError(w, 400, err)
			return
		}
		writeJSON(w, 200, map[string]bool{"ok": true})
	}))
	mux.HandleFunc("POST /api/ai/models", local(func(w http.ResponseWriter, r *http.Request) {
		var draft ai.Draft
		if decode(r, &draft) != nil {
			writeError(w, 400, errors.New("invalid AI settings"))
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 40*time.Second)
		defer cancel()
		models, err := service.Models(ctx, draft)
		if err != nil {
			writeError(w, 400, err)
			return
		}
		writeJSON(w, 200, map[string]any{"models": models})
	}))
	mux.HandleFunc("POST /api/ai/pick", local(func(w http.ResponseWriter, r *http.Request) {
		if !pickMu.TryLock() {
			writeError(w, 409, errors.New("an AI pick is already in progress"))
			return
		}
		defer pickMu.Unlock()
		snapshot := manager.Snapshot("", true)
		if len(snapshot.Tracks)-max(0, snapshot.CurrentIndex+1) >= party.MaxQueueTracks {
			writeError(w, 409, errors.New("the queue is full"))
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 90*time.Second)
		defer cancel()
		recent := []ai.Song{}
		end := min(len(snapshot.Tracks), max(0, snapshot.CurrentIndex+1))
		for _, track := range snapshot.Tracks[max(0, end-8):end] {
			recent = append(recent, ai.Song{Title: track.Title, Artist: track.Channel})
		}
		ids := []string{}
		for _, track := range snapshot.Tracks {
			ids = append(ids, track.ProviderID)
		}
		search := func(ctx context.Context, query string) ([]youtube.Track, error) {
			searchContext, release, err := coordinator.Begin(ctx, "ai-dj")
			if err != nil {
				return nil, err
			}
			defer release()
			return searcher.Search(searchContext, query)
		}
		track, err := service.Pick(ctx, recent, ids, search)
		if err != nil {
			writeError(w, 400, err)
			return
		}
		updated, queueID, err := manager.AddNext(ctx, track.URL)
		if err != nil {
			writeError(w, 409, err)
			return
		}
		title := track.Title
		for _, queued := range updated.Tracks {
			if queued.QueueID == queueID {
				title = queued.Title
			}
		}
		writeJSON(w, 200, map[string]any{"party": updated, "queueId": queueID, "title": title})
	}))
}
