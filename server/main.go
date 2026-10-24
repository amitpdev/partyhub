package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"partyhub/server/party"
	"partyhub/server/player"
	searchcoord "partyhub/server/search"
	"partyhub/server/youtube"
)

const defaultAddress = "127.0.0.1:8787"

type youtubeSearcher interface {
	Search(context.Context, string) ([]youtube.Track, error)
}

func main() {
	address := strings.TrimSpace(os.Getenv("PARTYHUB_ADDR"))
	if address == "" {
		address = defaultAddress
	}
	localPlayer := player.New()
	resolver := youtube.Resolver{}
	partyManager := party.New(localPlayer, resolver, party.DefaultStatePath())
	searchCoordinator := searchcoord.NewCoordinator(3)
	mux := http.NewServeMux()
	registerPlaylists(mux, partyManager)

	mux.HandleFunc("GET /api/health", func(response http.ResponseWriter, _ *http.Request) {
		writeJSON(response, http.StatusOK, localPlayer.Status())
	})

	mux.HandleFunc("POST /api/guest/session", func(response http.ResponseWriter, request *http.Request) {
		var body struct {
			Token string `json:"token"`
			Name  string `json:"name"`
		}
		if err := decode(request, &body); err != nil {
			writeError(response, http.StatusBadRequest, err)
			return
		}
		session, err := partyManager.CreateSession(body.Token, body.Name)
		if err != nil {
			writeError(response, http.StatusUnauthorized, err)
			return
		}
		http.SetCookie(response, &http.Cookie{
			Name:     "partyhub_guest",
			Value:    session.ID,
			Path:     "/",
			HttpOnly: true,
			Secure:   request.Header.Get("X-Forwarded-Proto") == "https",
			SameSite: http.SameSiteLaxMode,
			MaxAge:   12 * 60 * 60,
		})
		writeJSON(response, http.StatusOK, map[string]string{"name": session.Name})
	})

	mux.HandleFunc("GET /api/party", func(response http.ResponseWriter, request *http.Request) {
		viewerID, isDJ, ok := authorize(partyManager, request)
		if !ok {
			writeError(response, http.StatusUnauthorized, errors.New("join this party by scanning its QR code"))
			return
		}
		writeJSON(response, http.StatusOK, partyManager.Snapshot(viewerID, isDJ))
	})

	mux.HandleFunc("POST /api/queue/tracks", func(response http.ResponseWriter, request *http.Request) {
		viewerID, isDJ, ok := authorize(partyManager, request)
		if !ok {
			writeError(response, http.StatusUnauthorized, errors.New("your party session has expired"))
			return
		}
		var body struct {
			URL string `json:"url"`
		}
		if err := decode(request, &body); err != nil {
			writeError(response, http.StatusBadRequest, err)
			return
		}
		session := party.Session{ID: viewerID, Name: "DJ"}
		if !isDJ {
			session, _ = partyManager.Session(viewerID)
		}
		snapshot, err := partyManager.Add(body.URL, session, isDJ)
		if err != nil {
			status := http.StatusUnprocessableEntity
			if strings.Contains(err.Error(), "too many requests") {
				status = http.StatusTooManyRequests
			} else if strings.Contains(err.Error(), "already at queue position") ||
				strings.Contains(err.Error(), "upcoming tracks") {
				status = http.StatusConflict
			}
			writeError(response, status, err)
			return
		}
		writeJSON(response, http.StatusCreated, snapshot)
	})

	mux.HandleFunc("POST /api/youtube/search", youtubeSearchHandler(partyManager, searchCoordinator, resolver))

	mux.HandleFunc("DELETE /api/queue/tracks/{queueID}", func(response http.ResponseWriter, request *http.Request) {
		viewerID, isDJ, ok := authorize(partyManager, request)
		if !ok {
			writeError(response, http.StatusUnauthorized, errors.New("your party session has expired"))
			return
		}
		if err := partyManager.Remove(request.PathValue("queueID"), viewerID, isDJ); err != nil {
			writeError(response, http.StatusForbidden, err)
			return
		}
		writeJSON(response, http.StatusOK, partyManager.Snapshot(viewerID, isDJ))
	})

	mux.HandleFunc("POST /api/queue/action", func(response http.ResponseWriter, request *http.Request) {
		if !isLocalDJ(request) {
			writeError(response, http.StatusForbidden, errors.New("DJ controls are only available on the host laptop"))
			return
		}
		var body struct {
			Action    string `json:"action"`
			QueueID   string `json:"queueId"`
			Index     int    `json:"index"`
			Direction int    `json:"direction"`
		}
		if err := decode(request, &body); err != nil {
			writeError(response, http.StatusBadRequest, err)
			return
		}
		var err error
		switch body.Action {
		case "play":
			err = partyManager.Play(body.Index)
		case "next":
			err = partyManager.Next()
		case "previous":
			err = partyManager.Previous()
		case "move":
			err = partyManager.Move(body.QueueID, body.Direction)
		case "clear":
			partyManager.Clear()
		default:
			err = fmt.Errorf("unknown queue action %q", body.Action)
		}
		if err != nil {
			writeError(response, http.StatusConflict, err)
			return
		}
		writeJSON(response, http.StatusOK, partyManager.Snapshot("", true))
	})

	mux.HandleFunc("POST /api/queue/import", func(response http.ResponseWriter, request *http.Request) {
		if !isLocalDJ(request) {
			writeError(response, http.StatusForbidden, errors.New("queue import is only available locally"))
			return
		}
		var body struct {
			Tracks       []youtube.Track `json:"tracks"`
			CurrentIndex int             `json:"currentIndex"`
			Volume       float64         `json:"volume"`
		}
		if err := decode(request, &body); err != nil {
			writeError(response, http.StatusBadRequest, err)
			return
		}
		partyManager.Import(body.Tracks, body.CurrentIndex, body.Volume)
		writeJSON(response, http.StatusOK, partyManager.Snapshot("", true))
	})

	mux.HandleFunc("POST /api/player/control", func(response http.ResponseWriter, request *http.Request) {
		if !isLocalDJ(request) {
			writeError(response, http.StatusForbidden, errors.New("DJ controls are only available on the host laptop"))
			return
		}
		var body struct {
			Action string  `json:"action"`
			Value  float64 `json:"value"`
		}
		if err := decode(request, &body); err != nil {
			writeError(response, http.StatusBadRequest, err)
			return
		}
		var (
			status player.Status
			err    error
		)
		switch body.Action {
		case "pause":
			status, err = partyManager.Pause(true)
		case "resume":
			status, err = partyManager.Pause(false)
		case "seek":
			status, err = partyManager.Seek(body.Value)
		case "seekTo":
			status, err = partyManager.SeekTo(body.Value)
		case "volume":
			status, err = partyManager.SetVolume(body.Value)
		case "stop":
			status = partyManager.Stop()
		default:
			err = fmt.Errorf("unknown player action %q", body.Action)
		}
		if err != nil {
			writeError(response, http.StatusConflict, err)
			return
		}
		writeJSON(response, http.StatusOK, status)
	})

	mux.HandleFunc("GET /api/share", func(response http.ResponseWriter, request *http.Request) {
		if !isLocalDJ(request) {
			writeError(response, http.StatusForbidden, errors.New("sharing settings are only available locally"))
			return
		}
		writeJSON(response, http.StatusOK, partyManager.ShareInfo())
	})

	mux.HandleFunc("POST /api/share/configure", func(response http.ResponseWriter, request *http.Request) {
		if !isLocalDJ(request) {
			writeError(response, http.StatusForbidden, errors.New("sharing settings are only available locally"))
			return
		}
		var body struct {
			PublicURL string `json:"publicUrl"`
		}
		if err := decode(request, &body); err != nil {
			writeError(response, http.StatusBadRequest, err)
			return
		}
		info, err := partyManager.ConfigureShare(body.PublicURL)
		if err != nil {
			writeError(response, http.StatusBadRequest, err)
			return
		}
		searchCoordinator.CancelGuests()
		writeJSON(response, http.StatusOK, info)
	})

	mux.HandleFunc("POST /api/share/start", startSharingHandler(partyManager))

	mux.HandleFunc("POST /api/share/end", func(response http.ResponseWriter, request *http.Request) {
		if !isLocalDJ(request) {
			writeError(response, http.StatusForbidden, errors.New("sharing settings are only available locally"))
			return
		}
		partyManager.EndShare()
		searchCoordinator.CancelGuests()
		writeJSON(response, http.StatusOK, partyManager.ShareInfo())
	})

	mux.HandleFunc("GET /api/events", func(response http.ResponseWriter, request *http.Request) {
		viewerID, isDJ, ok := authorize(partyManager, request)
		if !ok {
			writeError(response, http.StatusUnauthorized, errors.New("your party session has expired"))
			return
		}
		flusher, ok := response.(http.Flusher)
		if !ok {
			writeError(response, http.StatusInternalServerError, errors.New("streaming is unavailable"))
			return
		}
		response.Header().Set("Content-Type", "text/event-stream")
		response.Header().Set("Cache-Control", "no-cache")
		response.Header().Set("Connection", "keep-alive")
		sendPartyEvent(response, partyManager.Snapshot(viewerID, isDJ))
		flusher.Flush()
		ticker := time.NewTicker(500 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-request.Context().Done():
				return
			case <-ticker.C:
				if !isDJ {
					if _, active := partyManager.Session(viewerID); !active {
						return
					}
				}
				sendPartyEvent(response, partyManager.Snapshot(viewerID, isDJ))
				flusher.Flush()
			}
		}
	})

	if info, err := os.Stat("dist"); err == nil && info.IsDir() {
		mux.Handle("/", spaHandler("dist"))
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		ticker := time.NewTicker(500 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				partyManager.Tick()
			}
		}
	}()

	server := &http.Server{
		Addr:              address,
		Handler:           requestLog(mux),
		ReadHeaderTimeout: 5 * time.Second,
	}
	go func() {
		log.Printf("Party Hub player listening on http://%s", address)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatal(err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop
	cancel()
	localPlayer.Close()
	shutdownContext, shutdownCancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer shutdownCancel()
	_ = server.Shutdown(shutdownContext)
}

func youtubeSearchHandler(
	manager *party.Manager,
	coordinator *searchcoord.Coordinator,
	searcher youtubeSearcher,
) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		viewerID, isDJ, ok := authorize(manager, request)
		if !ok {
			writeError(response, http.StatusUnauthorized, errors.New("your party session has expired"))
			return
		}
		var body struct {
			Query string `json:"query"`
		}
		if err := decode(request, &body); err != nil {
			writeError(response, http.StatusBadRequest, errors.New("enter a valid search"))
			return
		}
		searchKey := "dj"
		if !isDJ {
			searchKey = "guest:" + viewerID
		}
		searchContext, release, err := coordinator.Begin(request.Context(), searchKey)
		if err != nil {
			writeSearchCancellation(response, err)
			return
		}
		defer release()

		results, err := searcher.Search(searchContext, body.Query)
		if cause := context.Cause(searchContext); cause != nil {
			writeSearchCancellation(response, cause)
			return
		}
		if err != nil {
			switch {
			case errors.Is(err, youtube.ErrEmptySearchQuery),
				errors.Is(err, youtube.ErrSearchQueryTooLong):
				writeError(response, http.StatusBadRequest, err)
			case errors.Is(err, youtube.ErrSearchTimedOut):
				writeError(response, http.StatusGatewayTimeout, err)
			case errors.Is(err, youtube.ErrUnreadableResults):
				writeError(response, http.StatusBadGateway, errors.New("YouTube returned an unexpected response; try again"))
			default:
				writeError(response, http.StatusServiceUnavailable, errors.New("YouTube search is unavailable; check the connection and try again"))
			}
			return
		}
		writeJSON(response, http.StatusOK, map[string][]youtube.Track{"results": results})
	}
}

func writeSearchCancellation(response http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, searchcoord.ErrSuperseded):
		writeError(response, http.StatusConflict, errors.New("this search was replaced by a newer search"))
	case errors.Is(err, searchcoord.ErrSessionEnded):
		writeError(response, http.StatusUnauthorized, errors.New("your party session has expired"))
	case errors.Is(err, context.Canceled):
		return
	default:
		writeError(response, http.StatusServiceUnavailable, errors.New("YouTube search is unavailable; try again"))
	}
}

func decode(request *http.Request, value any) error {
	decoder := json.NewDecoder(io.LimitReader(request.Body, 64*1024))
	decoder.DisallowUnknownFields()
	return decoder.Decode(value)
}

func writeJSON(response http.ResponseWriter, status int, value any) {
	response.Header().Set("Content-Type", "application/json")
	response.WriteHeader(status)
	_ = json.NewEncoder(response).Encode(value)
}

func writeError(response http.ResponseWriter, status int, err error) {
	writeJSON(response, status, map[string]string{"error": err.Error()})
}

func sendPartyEvent(response http.ResponseWriter, snapshot party.Snapshot) {
	payload, _ := json.Marshal(snapshot)
	_, _ = fmt.Fprintf(response, "event: party\ndata: %s\n\n", payload)
}

func requestLog(next http.Handler) http.Handler {
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if filepath.Ext(request.URL.Path) == "" && request.URL.Path != "/api/events" {
			log.Printf("%s %s", request.Method, request.URL.Path)
		}
		next.ServeHTTP(response, request)
	})
}

func authorize(manager *party.Manager, request *http.Request) (string, bool, bool) {
	if isLocalDJ(request) {
		return "", true, true
	}
	cookie, err := request.Cookie("partyhub_guest")
	if err != nil {
		return "", false, false
	}
	session, ok := manager.Session(cookie.Value)
	return session.ID, false, ok
}

func isLocalDJ(request *http.Request) bool {
	host := request.Host
	if hostname, _, err := net.SplitHostPort(host); err == nil {
		host = hostname
	}
	host = strings.Trim(host, "[]")
	return host == "127.0.0.1" || host == "localhost" || host == "::1"
}

func spaHandler(directory string) http.Handler {
	files := http.FileServer(http.Dir(directory))
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if strings.HasPrefix(request.URL.Path, "/api/") {
			http.NotFound(response, request)
			return
		}
		path := filepath.Join(directory, filepath.Clean(request.URL.Path))
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			files.ServeHTTP(response, request)
			return
		}
		http.ServeFile(response, request, filepath.Join(directory, "index.html"))
	})
}

func startSharingHandler(manager *party.Manager) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		if !isLocalDJ(request) {
			writeError(response, http.StatusForbidden, errors.New("sharing settings are only available locally"))
			return
		}
		info, err := manager.StartShare()
		if err != nil {
			writeError(response, http.StatusConflict, err)
			return
		}
		writeJSON(response, http.StatusOK, info)
	}
}
