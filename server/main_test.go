package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"partyhub/server/party"
	"partyhub/server/player"
	searchcoord "partyhub/server/search"
	"partyhub/server/youtube"
)

type searcherFunc func(context.Context, string) ([]youtube.Track, error)

func (search searcherFunc) Search(ctx context.Context, query string) ([]youtube.Track, error) {
	return search(ctx, query)
}

type searchTestPlayer struct{}

func (searchTestPlayer) Load(string, string, float64) (player.Status, error) {
	return player.Status{}, nil
}
func (searchTestPlayer) Pause(bool) (player.Status, error) {
	return player.Status{}, nil
}
func (searchTestPlayer) Seek(float64) (player.Status, error) {
	return player.Status{}, nil
}
func (searchTestPlayer) SeekTo(float64) (player.Status, error) {
	return player.Status{}, nil
}
func (searchTestPlayer) SetVolume(float64) (player.Status, error) {
	return player.Status{}, nil
}
func (searchTestPlayer) Stop() player.Status { return player.Status{} }
func (searchTestPlayer) Status() player.Status {
	return player.Status{Available: true}
}

type searchTestResolver struct{}

func (searchTestResolver) Resolve(string) (youtube.Track, error) {
	return youtube.Track{}, nil
}

func newSearchTestHandler(
	t *testing.T,
	searcher youtubeSearcher,
) (http.Handler, *party.Manager, *searchcoord.Coordinator) {
	t.Helper()
	manager := party.New(
		searchTestPlayer{},
		searchTestResolver{},
		filepath.Join(t.TempDir(), "state.json"),
	)
	coordinator := searchcoord.NewCoordinator(3)
	return youtubeSearchHandler(manager, coordinator, searcher), manager, coordinator
}

func createSearchGuest(t *testing.T, manager *party.Manager) party.Session {
	t.Helper()
	info, err := manager.ConfigureShare("https://party.example.com")
	if err != nil {
		t.Fatal(err)
	}
	token := strings.TrimPrefix(strings.Split(info.GuestURL, "#token=")[1], "")
	session, err := manager.CreateSession(token, "Maya")
	if err != nil {
		t.Fatal(err)
	}
	return session
}

func guestSearchRequest(session party.Session, query string) *http.Request {
	request := httptest.NewRequest(
		http.MethodPost,
		"/api/youtube/search",
		strings.NewReader(`{"query":`+strconvQuote(query)+`}`),
	)
	request.Host = "party.example.com"
	request.AddCookie(&http.Cookie{Name: "partyhub_guest", Value: session.ID})
	return request
}

func strconvQuote(value string) string {
	encoded, _ := json.Marshal(value)
	return string(encoded)
}

func TestYouTubeSearchHandlerReturnsResults(t *testing.T) {
	handler, _, _ := newSearchTestHandler(t, searcherFunc(func(_ context.Context, query string) ([]youtube.Track, error) {
		if query != "dance floor" {
			t.Fatalf("query = %q", query)
		}
		return []youtube.Track{{
			Source:     "youtube",
			ProviderID: "aaaaaaaaaaa",
			URL:        "https://www.youtube.com/watch?v=aaaaaaaaaaa",
			Title:      "Dance Floor",
			Duration:   180,
		}}, nil
	}))
	request := httptest.NewRequest(http.MethodPost, "/api/youtube/search", strings.NewReader(`{"query":"dance floor"}`))
	request.Host = "127.0.0.1:8787"
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	var payload struct {
		Results []youtube.Track `json:"results"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if len(payload.Results) != 1 || payload.Results[0].ProviderID != "aaaaaaaaaaa" {
		t.Fatalf("payload = %#v", payload)
	}
}

func TestYouTubeSearchHandlerRequiresActiveSession(t *testing.T) {
	called := false
	handler, _, _ := newSearchTestHandler(t, searcherFunc(func(context.Context, string) ([]youtube.Track, error) {
		called = true
		return nil, nil
	}))
	request := httptest.NewRequest(http.MethodPost, "/api/youtube/search", strings.NewReader(`{"query":"party"}`))
	request.Host = "party.example.com"
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d", response.Code)
	}
	if called {
		t.Fatal("searcher was called for a remote request")
	}
}

func TestYouTubeSearchHandlerAllowsActiveGuest(t *testing.T) {
	handler, manager, _ := newSearchTestHandler(t, searcherFunc(func(_ context.Context, query string) ([]youtube.Track, error) {
		if query != "party" {
			t.Fatalf("query = %q", query)
		}
		return []youtube.Track{{ProviderID: "aaaaaaaaaaa"}}, nil
	}))
	session := createSearchGuest(t, manager)
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, guestSearchRequest(session, "party"))

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
}

func TestYouTubeSearchHandlerRejectsExpiredGuest(t *testing.T) {
	called := false
	handler, manager, _ := newSearchTestHandler(t, searcherFunc(func(context.Context, string) ([]youtube.Track, error) {
		called = true
		return nil, nil
	}))
	session := createSearchGuest(t, manager)
	manager.EndShare()
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, guestSearchRequest(session, "party"))

	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d", response.Code)
	}
	if called {
		t.Fatal("searcher was called for an expired guest")
	}
}

func TestYouTubeSearchHandlerSupersedesOlderGuestSearch(t *testing.T) {
	firstStarted := make(chan struct{})
	searcher := searcherFunc(func(ctx context.Context, query string) ([]youtube.Track, error) {
		if query == "first" {
			close(firstStarted)
			<-ctx.Done()
			return nil, ctx.Err()
		}
		return []youtube.Track{{ProviderID: "bbbbbbbbbbb"}}, nil
	})
	handler, manager, _ := newSearchTestHandler(t, searcher)
	session := createSearchGuest(t, manager)
	firstResponse := httptest.NewRecorder()
	firstDone := make(chan struct{})
	go func() {
		handler.ServeHTTP(firstResponse, guestSearchRequest(session, "first"))
		close(firstDone)
	}()
	select {
	case <-firstStarted:
	case <-time.After(time.Second):
		t.Fatal("first search did not start")
	}

	secondResponse := httptest.NewRecorder()
	handler.ServeHTTP(secondResponse, guestSearchRequest(session, "second"))

	select {
	case <-firstDone:
	case <-time.After(time.Second):
		t.Fatal("superseded search did not finish")
	}
	if firstResponse.Code != http.StatusConflict {
		t.Fatalf("first status = %d, body = %s", firstResponse.Code, firstResponse.Body.String())
	}
	if secondResponse.Code != http.StatusOK {
		t.Fatalf("second status = %d, body = %s", secondResponse.Code, secondResponse.Body.String())
	}
}

func TestYouTubeSearchHandlerMapsErrors(t *testing.T) {
	tests := []struct {
		name   string
		err    error
		status int
	}{
		{"empty query", youtube.ErrEmptySearchQuery, http.StatusBadRequest},
		{"long query", youtube.ErrSearchQueryTooLong, http.StatusBadRequest},
		{"timeout", youtube.ErrSearchTimedOut, http.StatusGatewayTimeout},
		{"unreadable results", youtube.ErrUnreadableResults, http.StatusBadGateway},
		{"unavailable", errors.New("network failed"), http.StatusServiceUnavailable},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			handler, _, _ := newSearchTestHandler(t, searcherFunc(func(context.Context, string) ([]youtube.Track, error) {
				return nil, test.err
			}))
			request := httptest.NewRequest(http.MethodPost, "/api/youtube/search", strings.NewReader(`{"query":"party"}`))
			request.Host = "localhost:8787"
			response := httptest.NewRecorder()

			handler.ServeHTTP(response, request)

			if response.Code != test.status {
				t.Fatalf("status = %d, want %d; body = %s", response.Code, test.status, response.Body.String())
			}
		})
	}
}

func TestYouTubeSearchHandlerRejectsMalformedBody(t *testing.T) {
	handler, _, _ := newSearchTestHandler(t, searcherFunc(func(context.Context, string) ([]youtube.Track, error) {
		t.Fatal("searcher should not be called")
		return nil, nil
	}))
	request := httptest.NewRequest(http.MethodPost, "/api/youtube/search", strings.NewReader(`{"query":`))
	request.Host = "localhost"
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
}
