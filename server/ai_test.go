package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"partyhub/server/ai"
	"partyhub/server/party"
	searchcoord "partyhub/server/search"
	"partyhub/server/youtube"
)

type aiMemorySecrets struct{ keys map[string]string }

func (m *aiMemorySecrets) Available() bool               { return true }
func (m *aiMemorySecrets) Get(id string) (string, error) { return m.keys[id], nil }
func (m *aiMemorySecrets) Set(id, key string) error      { m.keys[id] = key; return nil }
func (m *aiMemorySecrets) Delete(id string) error        { delete(m.keys, id); return nil }

type aiHTTPTransport func(*http.Request) (*http.Response, error)

func (f aiHTTPTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

type aiResolver struct{}

func (aiResolver) Resolve(rawURL string) (youtube.Track, error) {
	return youtube.Track{Source: "youtube", ProviderID: "ccccccccccc", URL: "https://www.youtube.com/watch?v=ccccccccccc", Title: "Picked song", Channel: "Artist", Duration: 180}, nil
}
func TestAIControlsAreHostOnlyAndSettingsDoNotReturnKeys(t *testing.T) {
	manager := party.New(searchTestPlayer{}, aiResolver{}, "")
	service := ai.New("", &aiMemorySecrets{keys: map[string]string{}})
	mux := http.NewServeMux()
	registerAI(mux, service, manager, searcherFunc(func(context.Context, string) ([]youtube.Track, error) { t.Fatal("unexpected search"); return nil, nil }), searchcoord.NewCoordinator(3))
	for _, route := range []struct{ method, path string }{{"GET", "settings"}, {"POST", "settings"}, {"DELETE", "settings"}, {"POST", "models"}, {"POST", "test"}, {"POST", "pick"}, {"GET", "chatgpt"}, {"POST", "chatgpt/login"}, {"DELETE", "chatgpt/login"}, {"POST", "chatgpt/signout"}, {"POST", "chatgpt/models"}, {"POST", "chatgpt/welcome"}} {
		request := httptest.NewRequest(route.method, "https://public.example/api/ai/"+route.path, strings.NewReader(`{}`))
		response := httptest.NewRecorder()
		mux.ServeHTTP(response, request)
		if response.Code != http.StatusForbidden {
			t.Errorf("%s %s status %d", route.method, route.path, response.Code)
		}
	}
	request := httptest.NewRequest("POST", "http://localhost/api/ai/settings", strings.NewReader(`{"provider":"openai","model":"test","apiKey":"not-a-real-secret","rememberKey":true}`))
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, request)
	if response.Code != 200 || strings.Contains(response.Body.String(), "not-a-real-secret") {
		t.Fatalf("unsafe save response: %d %s", response.Code, response.Body.String())
	}
	request = httptest.NewRequest("GET", "http://localhost/api/ai/settings", nil)
	response = httptest.NewRecorder()
	mux.ServeHTTP(response, request)
	if strings.Contains(response.Body.String(), "not-a-real-secret") || response.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("settings exposed credentials or were cacheable")
	}
}
func TestAIPickHTTPInsertsNextAndCanBeUndone(t *testing.T) {
	manager := party.New(searchTestPlayer{}, aiResolver{}, "")
	manager.Import([]youtube.Track{{Source: "youtube", ProviderID: "aaaaaaaaaaa", URL: "https://www.youtube.com/watch?v=aaaaaaaaaaa", Title: "Current", Duration: 180}, {Source: "youtube", ProviderID: "bbbbbbbbbbb", URL: "https://www.youtube.com/watch?v=bbbbbbbbbbb", Title: "Queued", Duration: 200}}, 0, 65)
	service := ai.New("", &aiMemorySecrets{keys: map[string]string{}})
	if _, err := service.Save(ai.Draft{Provider: "ollama", Model: "local"}); err != nil {
		t.Fatal(err)
	}
	calls := 0
	service.Client = &http.Client{Transport: aiHTTPTransport(func(r *http.Request) (*http.Response, error) {
		calls++
		text := `{"query":"artist next song"}`
		if calls > 1 {
			text = `{"providerId":"ccccccccccc"}`
		}
		data, _ := json.Marshal(map[string]any{"message": map[string]string{"content": text}})
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(string(data))), Header: make(http.Header)}, nil
	})}
	mux := http.NewServeMux()
	registerAI(mux, service, manager, searcherFunc(func(context.Context, string) ([]youtube.Track, error) {
		return []youtube.Track{{Source: "youtube", ProviderID: "ccccccccccc", URL: "https://www.youtube.com/watch?v=ccccccccccc", Title: "Picked song", Duration: 180}}, nil
	}), searchcoord.NewCoordinator(3))
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, httptest.NewRequest("POST", "http://localhost/api/ai/pick", nil))
	if response.Code != 200 {
		t.Fatalf("pick %d %s", response.Code, response.Body.String())
	}
	var result struct {
		Party   party.Snapshot `json:"party"`
		QueueID string         `json:"queueId"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if len(result.Party.Tracks) != 3 || result.Party.CurrentIndex != 0 || result.Party.Volume != 65 || result.Party.Tracks[1].ProviderID != "ccccccccccc" || result.Party.Tracks[2].ProviderID != "bbbbbbbbbbb" {
		t.Fatal("pick did not preserve the set")
	}
	if err := manager.Remove(result.QueueID, "", true); err != nil {
		t.Fatal(err)
	}
	if len(manager.Snapshot("", true).Tracks) != 2 {
		t.Fatal("undo failed")
	}
}

func TestAICrossSiteRequestsCannotStartSignInOrReadSettings(t *testing.T) {
	service := ai.New("", &aiMemorySecrets{keys: map[string]string{}})
	mux := http.NewServeMux()
	manager := party.New(searchTestPlayer{}, aiResolver{}, "")
	registerAI(mux, service, manager, searcherFunc(func(context.Context, string) ([]youtube.Track, error) { return nil, nil }), searchcoord.NewCoordinator(3))
	for _, route := range []struct{ method, path string }{{"GET", "settings"}, {"GET", "chatgpt"}, {"POST", "chatgpt/login"}} {
		req := httptest.NewRequest(route.method, "http://localhost/api/ai/"+route.path, strings.NewReader(`{}`))
		req.Header.Set("Origin", "https://untrusted.example")
		res := httptest.NewRecorder()
		mux.ServeHTTP(res, req)
		if res.Code != 403 {
			t.Fatalf("cross-site request accepted: %s", route.path)
		}
	}
	status, _ := service.ChatGPTStatus()
	if status.Login.Status != "idle" {
		t.Fatal("unauthorized request started a login")
	}
}
