package ai

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"partyhub/server/youtube"
)

type memorySecrets struct {
	values map[string]string
	fail   bool
}

func (m *memorySecrets) Available() bool { return true }
func (m *memorySecrets) Get(id string) (string, error) {
	key, ok := m.values[id]
	if !ok {
		return "", errors.New("not found")
	}
	return key, nil
}
func (m *memorySecrets) Set(id, key string) error {
	if m.fail {
		return errors.New("blocked")
	}
	m.values[id] = key
	return nil
}
func (m *memorySecrets) Delete(id string) error { delete(m.values, id); return nil }

type transportFunc func(*http.Request) (*http.Response, error)

func (f transportFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func response(value any) *http.Response {
	data, _ := json.Marshal(value)
	return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(string(data))), Header: make(http.Header)}
}
func providerResponse(provider, text string) any {
	switch provider {
	case "openai":
		return map[string]any{"output": []any{map[string]any{"content": []any{map[string]string{"type": "output_text", "text": text}}}}}
	case "anthropic":
		return map[string]any{"content": []any{map[string]string{"type": "text", "text": text}}}
	case "gemini":
		return map[string]any{"candidates": []any{map[string]any{"content": map[string]any{"parts": []any{map[string]string{"text": text}}}}}}
	case "ollama":
		return map[string]any{"message": map[string]string{"content": text}}
	default:
		return map[string]any{"choices": []any{map[string]any{"message": map[string]string{"content": text}}}}
	}
}
func TestKeysNeverEnterPreferencesOrPublicSettings(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ai.json")
	secrets := &memorySecrets{values: map[string]string{}}
	s := New(path, secrets)
	settings, err := s.Save(Draft{Provider: "openai", Model: "test-model", APIKey: "test-key-not-real", RememberKey: true})
	if err != nil {
		t.Fatal(err)
	}
	if !settings.Configured || !settings.HasKey || !settings.RememberKey {
		t.Fatalf("settings %#v", settings)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	public, _ := json.Marshal(settings)
	if strings.Contains(string(data), "test-key-not-real") || strings.Contains(string(public), "test-key-not-real") {
		t.Fatal("credential leaked")
	}
	stat, _ := os.Stat(path)
	if stat.Mode().Perm() != 0600 {
		t.Fatalf("permissions %v", stat.Mode())
	}
	restored := New(path, secrets)
	connection, err := restored.Connection()
	if err != nil || connection.APIKey != "test-key-not-real" {
		t.Fatal("key did not restore securely")
	}
	if err = restored.Clear(); err != nil {
		t.Fatal(err)
	}
	if len(secrets.values) != 0 {
		t.Fatal("disconnect left a key in the credential store")
	}
}
func TestSessionKeysAreNotRestoredOrReusedAcrossServers(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ai.json")
	store := &memorySecrets{values: map[string]string{}}
	s := New(path, store)
	if _, err := s.Save(Draft{Provider: "compatible", Model: "local-model", BaseURL: "http://127.0.0.1:9000/v1", APIKey: "session-key"}); err != nil {
		t.Fatal(err)
	}
	if len(store.values) > 0 {
		t.Fatal("session key saved in keychain")
	}
	connection, err := s.Connection()
	if err != nil || connection.APIKey != "session-key" {
		t.Fatal("session key not available")
	}
	changed, err := s.resolve(Draft{Provider: "compatible", BaseURL: "http://127.0.0.1:9001/v1"}, false)
	if err != nil || changed.APIKey != "" {
		t.Fatal("a saved key was reused for another server")
	}
	restored := New(path, store)
	connection, err = restored.Connection()
	if err == nil || connection.APIKey != "" {
		t.Fatal("missing session credential was treated as a configured connection")
	}
	restoredSettings, _ := restored.Settings()
	if restoredSettings.Configured {
		t.Fatal("session-only connection should need setup after restart")
	}
	cloud := New("", store)
	_, _ = cloud.Save(Draft{Provider: "openai", Model: "test-model", APIKey: "session-key"})
	_, err = cloud.resolve(Draft{Provider: "anthropic", Model: "test-model"}, true)
	if err == nil {
		t.Fatal("key reused across providers")
	}
}
func TestFailedSecureSavePreservesExistingSettings(t *testing.T) {
	store := &memorySecrets{values: map[string]string{}}
	s := New("", store)
	before, err := s.Save(Draft{Provider: "openai", Model: "first", APIKey: "test-key"})
	if err != nil {
		t.Fatal(err)
	}
	store.fail = true
	if _, err = s.Save(Draft{Provider: "openai", Model: "second", APIKey: "new-key", RememberKey: true}); err == nil {
		t.Fatal("failed save reported success")
	}
	after, _ := s.Settings()
	if before != after {
		t.Fatal("failed save changed preferences")
	}
}
func TestEndpointValidation(t *testing.T) {
	for _, endpoint := range []string{"http://example.com/v1", "https://user:password@example.com/v1", "https://example.com/v1?key=secret", "https://example.com/v1#token", "file:///tmp/socket"} {
		if _, err := normalize(Draft{Provider: "compatible", BaseURL: endpoint}, false); err == nil {
			t.Errorf("accepted %s", endpoint)
		}
	}
	cloud, err := normalize(Draft{Provider: "openai", BaseURL: "https://evil.example"}, false)
	if err != nil || cloud.BaseURL != "https://api.openai.com/v1" {
		t.Fatal("built-in provider endpoint could be redirected")
	}
}
func TestProviderRequestsAndResponseParsing(t *testing.T) {
	for _, provider := range []string{"openai", "anthropic", "gemini", "ollama", "compatible"} {
		t.Run(provider, func(t *testing.T) {
			s := New("", &memorySecrets{values: map[string]string{}})
			called := false
			s.Client = &http.Client{Transport: transportFunc(func(r *http.Request) (*http.Response, error) {
				called = true
				var body map[string]any
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Fatal(err)
				}
				if body["model"] != "test-model" && provider != "gemini" {
					t.Fatal("model missing")
				}
				if strings.Contains(r.URL.RawQuery, "test-key") {
					t.Fatal("key in query string")
				}
				if provider == "gemini" {
					if r.Header.Get("x-goog-api-key") != "test-key" {
						t.Fatal("Gemini key missing")
					}
				} else if r.Header.Get("Authorization") != "Bearer test-key" {
					t.Fatal("authorization missing")
				}
				switch provider {
				case "openai":
					if r.URL.Path != "/v1/responses" || body["store"] != false {
						t.Fatal("incorrect Responses request")
					}
				case "anthropic":
					if r.Header.Get("anthropic-version") == "" || r.URL.Path != "/v1/messages" {
						t.Fatal("incorrect Anthropic request")
					}
				case "gemini":
					if r.URL.Path != "/v1beta/models/test-model:generateContent" {
						t.Fatal("incorrect Gemini endpoint")
					}
				case "ollama":
					if body["stream"] != false || body["format"] != "json" {
						t.Fatal("incorrect Ollama request")
					}
				case "compatible":
					if r.URL.Path != "/v1/chat/completions" {
						t.Fatal("incorrect compatible endpoint")
					}
				}
				return response(providerResponse(provider, `{"ok":true}`)), nil
			})}
			err := s.Test(context.Background(), Draft{Provider: provider, Model: "test-model", BaseURL: "http://127.0.0.1:8000/v1", APIKey: "test-key"})
			if err != nil || !called {
				t.Fatalf("test: %v", err)
			}
		})
	}
}
func TestHTTPFailuresNeverEchoProviderBodiesAndRedirectsAreBlocked(t *testing.T) {
	s := New("", &memorySecrets{values: map[string]string{}})
	s.Client = &http.Client{Transport: transportFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 401, Body: io.NopCloser(strings.NewReader("error with secret-key")), Header: make(http.Header)}, nil
	})}
	err := s.Test(context.Background(), Draft{Provider: "openai", Model: "test", APIKey: "secret-key"})
	if err == nil || strings.Contains(err.Error(), "secret-key") {
		t.Fatal("unsafe error response")
	}
	calls := 0
	s.Client = &http.Client{Transport: transportFunc(func(r *http.Request) (*http.Response, error) {
		calls++
		return &http.Response{StatusCode: 302, Header: http.Header{"Location": []string{"https://elsewhere.example"}}, Body: io.NopCloser(strings.NewReader(""))}, nil
	})}
	err = s.Test(context.Background(), Draft{Provider: "openai", Model: "test", APIKey: "secret-key"})
	if err == nil || calls != 1 {
		t.Fatal("provider redirect was followed")
	}
}
func TestModelDiscovery(t *testing.T) {
	s := New("", &memorySecrets{values: map[string]string{}})
	s.Client = &http.Client{Transport: transportFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path != "/v1/models" {
			t.Fatal("incorrect model endpoint")
		}
		return response(map[string]any{"data": []any{map[string]string{"id": "gpt-test"}, map[string]string{"id": "embedding-test"}, map[string]string{"id": "gpt-test"}}}), nil
	})}
	models, err := s.Models(context.Background(), Draft{Provider: "openai", APIKey: "test-key"})
	if err != nil || len(models) != 1 || models[0] != "gpt-test" {
		t.Fatalf("models %v %v", models, err)
	}
}
func TestPickUsesOnlyFreshYouTubeCandidates(t *testing.T) {
	for _, valid := range []bool{true, false} {
		t.Run(map[bool]string{true: "valid", false: "invented ID"}[valid], func(t *testing.T) {
			s := New("", &memorySecrets{values: map[string]string{}})
			_, _ = s.Save(Draft{Provider: "ollama", Model: "local"})
			calls := 0
			s.Client = &http.Client{Transport: transportFunc(func(r *http.Request) (*http.Response, error) {
				calls++
				if calls == 1 {
					return response(providerResponse("ollama", `{"query":"new artist song official audio"}`)), nil
				}
				var body map[string]any
				_ = json.NewDecoder(r.Body).Decode(&body)
				encoded, _ := json.Marshal(body)
				if strings.Contains(string(encoded), "aaaaaaaaaaa") {
					t.Fatal("queued candidate sent for ranking")
				}
				id := "bbbbbbbbbbb"
				if !valid {
					id = "ccccccccccc"
				}
				return response(providerResponse("ollama", `{"providerId":"`+id+`"}`)), nil
			})}
			search := func(ctx context.Context, query string) ([]youtube.Track, error) {
				if query != "new artist song official audio" {
					t.Fatal("incorrect search plan")
				}
				return []youtube.Track{{Source: "youtube", ProviderID: "aaaaaaaaaaa", Title: "Queued", Duration: 180}, {Source: "youtube", ProviderID: "bbbbbbbbbbb", Title: "New", Duration: 180, URL: "https://www.youtube.com/watch?v=bbbbbbbbbbb"}}, nil
			}
			track, err := s.Pick(context.Background(), []Song{{"Current", "Artist"}}, []string{"aaaaaaaaaaa"}, search)
			if valid && (err != nil || track.ProviderID != "bbbbbbbbbbb") {
				t.Fatalf("pick %v %v", track, err)
			}
			if !valid && err == nil {
				t.Fatal("invented candidate accepted")
			}
		})
	}
}
func TestBadModelJSONAndNoCandidatesFailCleanly(t *testing.T) {
	if ParseJSON("not JSON", &struct{}{}) == nil {
		t.Fatal("invalid model JSON accepted")
	}
	var value struct {
		OK bool `json:"ok"`
	}
	if ParseJSON("```json\n{\"ok\":true}\n```", &value) != nil || !value.OK {
		t.Fatal("JSON fence not supported")
	}
	s := New("", &memorySecrets{values: map[string]string{}})
	_, _ = s.Save(Draft{Provider: "ollama", Model: "local"})
	s.Client = &http.Client{Transport: transportFunc(func(*http.Request) (*http.Response, error) {
		return response(providerResponse("ollama", `{"query":"new song"}`)), nil
	})}
	_, err := s.Pick(context.Background(), nil, nil, func(context.Context, string) ([]youtube.Track, error) { return nil, nil })
	if err == nil {
		t.Fatal("empty results accepted")
	}
}
