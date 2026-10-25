package ai

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type Draft struct {
	AccountID   string `json:"accountId,omitempty"`
	Provider    string `json:"provider"`
	Model       string `json:"model"`
	BaseURL     string `json:"baseUrl"`
	APIKey      string `json:"apiKey"`
	RememberKey bool   `json:"rememberKey"`
}
type Settings struct {
	AccountID              string `json:"accountId,omitempty"`
	Provider               string `json:"provider"`
	Model                  string `json:"model"`
	BaseURL                string `json:"baseUrl"`
	HasKey                 bool   `json:"hasKey"`
	RememberKey            bool   `json:"rememberKey"`
	Configured             bool   `json:"configured"`
	SecureStorageAvailable bool   `json:"secureStorageAvailable"`
}
type preferences struct {
	AccountID    string `json:"accountId,omitempty"`
	Provider     string `json:"provider"`
	Model        string `json:"model"`
	BaseURL      string `json:"baseUrl"`
	CredentialID string `json:"credentialId,omitempty"`
	NeedsKey     bool   `json:"needsKey,omitempty"`
}
type Secrets interface {
	Available() bool
	Get(string) (string, error)
	Set(string, string) error
	Delete(string) error
}
type Service struct {
	chatGPT    *chatGPT
	mu         sync.Mutex
	path       string
	prefs      preferences
	sessionKey string
	secrets    Secrets
	loadError  error
	Client     *http.Client
}

func New(path string, secrets Secrets) *Service {
	if secrets == nil {
		secrets = SystemSecrets{}
	}
	s := &Service{path: path, secrets: secrets, Client: &http.Client{}}
	s.prefs = preferences{Provider: "openai", BaseURL: "https://api.openai.com/v1"}
	if path != "" {
		data, err := os.ReadFile(path)
		if err == nil {
			s.loadError = json.Unmarshal(data, &s.prefs)
			if s.loadError == nil {
				normalized, err := normalize(Draft{Provider: s.prefs.Provider, Model: s.prefs.Model, BaseURL: s.prefs.BaseURL}, false)
				s.loadError = err
				if err == nil {
					s.prefs.Provider = normalized.Provider
					s.prefs.Model = normalized.Model
					s.prefs.BaseURL = normalized.BaseURL
				}
			}
		} else if !errors.Is(err, os.ErrNotExist) {
			s.loadError = err
		}
	}
	s.chatGPT = newChatGPT(s)
	return s
}
func (s *Service) Settings() (Settings, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.loadError != nil {
		return Settings{}, errors.New("AI preferences could not be loaded; repair ai.json in the Party Hub configuration folder and restart")
	}
	return s.settingsLocked(), nil
}
func (s *Service) settingsLocked() Settings {
	hasKey := s.sessionKey != "" || s.prefs.CredentialID != ""
	if s.prefs.Provider == "chatgpt" {
		return Settings{AccountID: s.prefs.AccountID, Provider: "chatgpt", Model: s.prefs.Model, BaseURL: apiResource, Configured: s.prefs.Model != "" && s.chatGPT.connected(s.prefs.AccountID), SecureStorageAvailable: s.secrets.Available()}
	}
	return Settings{Provider: s.prefs.Provider, Model: s.prefs.Model, BaseURL: s.prefs.BaseURL, HasKey: hasKey, RememberKey: s.prefs.CredentialID != "", Configured: s.prefs.Model != "" && (hasKey || (!s.prefs.NeedsKey && (s.prefs.Provider == "ollama" || s.prefs.Provider == "compatible"))), SecureStorageAvailable: s.secrets.Available()}
}
func normalize(d Draft, requireModel bool) (Draft, error) {
	d.Provider = strings.TrimSpace(d.Provider)
	d.Model = strings.TrimSpace(d.Model)
	d.BaseURL = strings.TrimRight(strings.TrimSpace(d.BaseURL), "/")
	switch d.Provider {
	case "chatgpt":
		d.BaseURL = apiResource
		d.APIKey = ""
		d.RememberKey = false
	case "openai":
		d.BaseURL = "https://api.openai.com/v1"
	case "anthropic":
		d.BaseURL = "https://api.anthropic.com/v1"
	case "gemini":
		d.BaseURL = "https://generativelanguage.googleapis.com/v1beta"
	case "ollama":
		if d.BaseURL == "" {
			d.BaseURL = "http://127.0.0.1:11434"
		}
	case "compatible":
		if d.BaseURL == "" {
			return Draft{}, errors.New("enter the server URL, including /v1 when required")
		}
	default:
		return Draft{}, errors.New("choose a supported AI provider")
	}
	if requireModel && d.Model == "" {
		return Draft{}, errors.New("choose or enter a model")
	}
	if len(d.Model) > 200 || strings.ContainsAny(d.Model, "\r\n?#") {
		return Draft{}, errors.New("enter a valid model ID")
	}
	d.Model = strings.TrimPrefix(d.Model, "models/")
	u, err := url.Parse(d.BaseURL)
	if err != nil || u.Hostname() == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return Draft{}, errors.New("enter a server URL without credentials, query parameters, or fragments")
	}
	ip := net.ParseIP(u.Hostname())
	loopback := u.Hostname() == "localhost" || (ip != nil && ip.IsLoopback())
	if u.Scheme != "https" && !(u.Scheme == "http" && loopback) {
		return Draft{}, errors.New("use HTTPS for remote providers; HTTP is supported only for localhost")
	}
	d.APIKey = strings.TrimSpace(d.APIKey)
	if len(d.APIKey) > 4096 {
		return Draft{}, errors.New("API key is too long")
	}
	for _, r := range d.APIKey {
		if r < 33 || r > 126 {
			return Draft{}, errors.New("API key must not contain whitespace or control characters")
		}
	}
	return d, nil
}
func (s *Service) resolve(d Draft, requireModel bool) (Draft, error) {
	d, err := normalize(d, requireModel)
	if err != nil {
		return d, err
	}
	if d.Provider == "chatgpt" {
		if !s.chatGPT.connected(d.AccountID) {
			return d, errors.New("Sign in to ChatGPT in AI settings.")
		}
		return d, nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.loadError != nil {
		return d, errors.New("repair AI preferences before configuring a connection")
	}
	if d.APIKey == "" && d.Provider == s.prefs.Provider && d.BaseURL == s.prefs.BaseURL {
		d.APIKey = s.sessionKey
		if d.APIKey == "" && s.prefs.CredentialID != "" {
			d.APIKey, err = s.secrets.Get(s.prefs.CredentialID)
			if err != nil {
				return d, errors.New("the saved API key could not be read; unlock your credential store or enter the key again")
			}
		}
		if d.APIKey == "" && s.prefs.NeedsKey {
			return d, errors.New("enter the API key again; session-only keys are cleared when the Go service restarts")
		}
	}
	if d.APIKey == "" && d.Provider != "ollama" && d.Provider != "compatible" {
		return d, errors.New("enter an API key for this provider")
	}
	return d, nil
}
func (s *Service) Connection() (Draft, error) {
	s.mu.Lock()
	d := Draft{AccountID: s.prefs.AccountID, Provider: s.prefs.Provider, Model: s.prefs.Model, BaseURL: s.prefs.BaseURL}
	s.mu.Unlock()
	return s.resolve(d, true)
}
func (s *Service) Save(d Draft) (Settings, error) {
	d, err := s.resolve(d, true)
	if err != nil {
		return Settings{}, err
	}
	if d.Provider == "chatgpt" {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		models, err := s.ChatGPTModels(ctx, d.AccountID)
		if err != nil {
			return Settings{}, err
		}
		found := false
		for _, m := range models {
			if m.ID == d.Model {
				found = true
			}
		}
		if !found {
			return Settings{}, errors.New("Choose a model available to this ChatGPT account")
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	next := preferences{AccountID: d.AccountID, Provider: d.Provider, Model: d.Model, BaseURL: d.BaseURL, NeedsKey: d.APIKey != ""}
	if d.RememberKey && d.APIKey != "" {
		if !s.secrets.Available() {
			return Settings{}, errors.New("secure storage is unavailable; turn off Remember key to use this session only")
		}
		bytes := make([]byte, 16)
		if _, err = rand.Read(bytes); err != nil {
			return Settings{}, errors.New("could not create secure credential ID")
		}
		next.CredentialID = hex.EncodeToString(bytes)
		if err = s.secrets.Set(next.CredentialID, d.APIKey); err != nil {
			return Settings{}, errors.New("could not save the key securely; unlock your credential store or turn off Remember key")
		}
	}
	if err = s.persist(next); err != nil {
		if next.CredentialID != "" {
			_ = s.secrets.Delete(next.CredentialID)
		}
		return Settings{}, errors.New("could not save AI preferences; check write access to the Party Hub configuration folder")
	}
	old := s.prefs.CredentialID
	s.prefs = next
	s.sessionKey = ""
	if next.CredentialID == "" {
		s.sessionKey = d.APIKey
	}
	if old != "" {
		_ = s.secrets.Delete(old)
	}
	return s.settingsLocked(), nil
}
func (s *Service) Clear() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.loadError != nil {
		return errors.New("repair AI preferences before disconnecting")
	}
	if s.prefs.CredentialID != "" {
		if err := s.secrets.Delete(s.prefs.CredentialID); err != nil {
			return errors.New("could not remove the saved key; unlock your credential store and try again")
		}
	}
	next := preferences{Provider: "openai", BaseURL: "https://api.openai.com/v1"}
	if err := s.persist(next); err != nil {
		return errors.New("could not clear AI preferences")
	}
	s.prefs = next
	s.sessionKey = ""
	return nil
}
func (s *Service) persist(p preferences) error {
	if s.path == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return err
	}
	temp := s.path + ".tmp"
	defer os.Remove(temp)
	if err = os.WriteFile(temp, data, 0600); err != nil {
		return err
	}
	return os.Rename(temp, s.path)
}
