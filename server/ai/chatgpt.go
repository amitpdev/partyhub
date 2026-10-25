package ai

// ChatGPT plan access uses the documented public OAuth flow. Credentials never
// travel through the web UI; the browser receives only account labels and status.
import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const authIssuer = "https://auth.openai.com"
const apiResource = "https://api.openai.com/v1"
const planScope = "chatgpt.tokens.use.direct"

type ChatGPTAccount struct {
	ID           string `json:"id"`
	ClientID     string `json:"clientId"`
	Subject      string `json:"subject"`
	Email        string `json:"email"`
	CredentialID string `json:"credentialId,omitempty"`
	Welcomed     bool   `json:"welcomed"`
}
type chatGPTFile struct {
	HostID   string           `json:"hostId"`
	Accounts []ChatGPTAccount `json:"accounts"`
}
type tokenSet struct {
	Access    string    `json:"access_token"`
	Refresh   string    `json:"refresh_token"`
	ID        string    `json:"id_token"`
	Scope     string    `json:"scope"`
	Type      string    `json:"token_type"`
	ExpiresIn int64     `json:"expires_in"`
	ExpiresAt time.Time `json:"expiresAt"`
}
type AccountView struct {
	ID         string `json:"id"`
	Label      string `json:"label"`
	Connected  bool   `json:"connected"`
	Remembered bool   `json:"remembered"`
	Welcomed   bool   `json:"welcomed"`
}
type LoginView struct {
	Status    string `json:"status"`
	Message   string `json:"message,omitempty"`
	AccountID string `json:"accountId,omitempty"`
}
type ChatGPTStatus struct {
	Accounts []AccountView `json:"accounts"`
	Login    LoginView     `json:"login"`
}
type loginAttempt struct {
	state, nonce, verifier, redirect, accountID, clientID string
	remember                                              bool
	cancel                                                context.CancelFunc
	view                                                  LoginView
}
type chatGPT struct {
	mu        sync.Mutex
	service   *Service
	path      string
	file      chatGPTFile
	sessions  map[string]tokenSet
	attempt   *loginAttempt
	loadError error
	running   map[string]map[string]context.CancelFunc
}

func newChatGPT(s *Service) *chatGPT {
	c := &chatGPT{service: s, sessions: map[string]tokenSet{}, running: map[string]map[string]context.CancelFunc{}}
	if s.path != "" {
		c.path = s.path + ".chatgpt"
		data, err := os.ReadFile(c.path)
		if err == nil {
			c.loadError = json.Unmarshal(data, &c.file)
		} else if !errors.Is(err, os.ErrNotExist) {
			c.loadError = err
		}
	}
	return c
}
func opaque() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic("system random source unavailable")
	}
	return base64.RawURLEncoding.EncodeToString(b)
}
func (c *chatGPT) persistLocked() error {
	if c.path == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(c.path), 0700); err != nil {
		return errors.New("could not save ChatGPT account settings")
	}
	data, _ := json.MarshalIndent(c.file, "", "  ")
	f, err := os.CreateTemp(filepath.Dir(c.path), "chatgpt-*")
	if err != nil {
		return errors.New("could not save ChatGPT account settings")
	}
	defer os.Remove(f.Name())
	_, err = f.Write(data)
	closeErr := f.Close()
	if err == nil {
		err = closeErr
	}
	if err == nil {
		err = os.Rename(f.Name(), c.path)
	}
	if err != nil {
		return errors.New("could not save ChatGPT account settings")
	}
	return nil
}
func (c *chatGPT) accountLocked(id string) *ChatGPTAccount {
	for i := range c.file.Accounts {
		if c.file.Accounts[i].ID == id {
			return &c.file.Accounts[i]
		}
	}
	return nil
}
func (c *chatGPT) connected(id string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	a := c.accountLocked(id)
	return c.loadError == nil && a != nil && (a.CredentialID != "" || c.sessions[id].Access != "")
}
func (s *Service) ChatGPTStatus() (ChatGPTStatus, error) {
	c := s.chatGPT
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.loadError != nil {
		return ChatGPTStatus{}, errors.New("ChatGPT account settings could not be loaded; check the Party Hub configuration folder")
	}
	result := ChatGPTStatus{Accounts: []AccountView{}, Login: LoginView{Status: "idle"}}
	for i, a := range c.file.Accounts {
		label := a.Email
		if label == "" {
			label = "ChatGPT account"
		}
		result.Accounts = append(result.Accounts, AccountView{a.ID, fmt.Sprintf("%s · %d", label, i+1), a.CredentialID != "" || c.sessions[a.ID].Access != "", a.CredentialID != "", a.Welcomed})
	}
	if c.attempt != nil {
		result.Login = c.attempt.view
	}
	return result, nil
}
func (s *Service) StartChatGPTLogin(accountID string, remember bool) (string, error) {
	c := s.chatGPT
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.loadError != nil {
		return "", errors.New("repair ChatGPT account settings before signing in")
	}
	if remember && !s.secrets.Available() {
		return "", errors.New("secure storage unavailable; use session-only sign-in")
	}
	retryClient := ""
	if accountID == "" && c.attempt != nil && c.attempt.view.Status == "error" && c.attempt.accountID == "" && c.attempt.clientID != "dynamic_agent_client" {
		retryClient = c.attempt.clientID
	}
	if c.attempt != nil {
		c.attempt.cancel()
	}
	if c.file.HostID == "" {
		b := make([]byte, 16)
		if _, err := rand.Read(b); err != nil {
			return "", errors.New("could not prepare sign-in")
		}
		b[6] = (b[6] & 15) | 64
		b[8] = (b[8] & 63) | 128
		c.file.HostID = fmt.Sprintf("urn:uuid:%x-%x-%x-%x-%x", b[:4], b[4:6], b[6:8], b[8:10], b[10:])
		if err := c.persistLocked(); err != nil {
			return "", err
		}
	}
	clientID := "dynamic_agent_client"
	if retryClient != "" {
		clientID = retryClient
	}
	var hint string
	if accountID != "" {
		a := c.accountLocked(accountID)
		if a == nil {
			return "", errors.New("choose a saved ChatGPT account")
		}
		clientID = a.ClientID
		hint = a.Email
	}
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		return "", errors.New("could not start local sign-in callback; try again")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	a := &loginAttempt{state: opaque(), nonce: opaque(), verifier: opaque(), redirect: "http://" + listener.Addr().String() + "/auth/callback", accountID: accountID, clientID: clientID, remember: remember, cancel: cancel, view: LoginView{Status: "pending"}}
	c.attempt = a
	server := &http.Server{ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 45 * time.Second, Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { c.callback(ctx, a, w, r) })}
	go func() { _ = server.Serve(listener) }()
	go func() {
		<-ctx.Done()
		_ = server.Close()
		c.mu.Lock()
		defer c.mu.Unlock()
		if c.attempt == a && (a.view.Status == "pending" || a.view.Status == "processing") {
			a.view = LoginView{Status: "error", Message: "Sign-in timed out. Try again."}
		}
	}()
	sum := sha256.Sum256([]byte(a.verifier))
	q := url.Values{"client_id": {clientID}, "ext_agent_host_id": {c.file.HostID}, "response_type": {"code"}, "redirect_uri": {a.redirect}, "scope": {"openid profile email offline_access resource.invoke " + planScope}, "resource": {apiResource}, "state": {a.state}, "nonce": {a.nonce}, "code_challenge_method": {"S256"}, "code_challenge": {base64.RawURLEncoding.EncodeToString(sum[:])}}
	if clientID == "dynamic_agent_client" {
		q.Set("agent_name_hint", "Party Hub")
	} else if hint != "" {
		q.Set("login_hint", hint)
	}
	return authIssuer + "/api/accounts/authorize?" + q.Encode(), nil
}
func (s *Service) CancelChatGPTLogin() {
	c := s.chatGPT
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.attempt != nil {
		c.attempt.view = LoginView{Status: "idle"}
		c.attempt.cancel()
	}
}
func (c *chatGPT) callback(ctx context.Context, a *loginAttempt, w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("Content-Security-Policy", "default-src 'none'; frame-ancestors 'none'")
	if r.URL.Path != "/auth/callback" || r.Method != "GET" {
		http.NotFound(w, r)
		return
	}
	q := r.URL.Query()
	c.mu.Lock()
	if c.attempt != a || a.view.Status != "pending" || q.Get("state") != a.state {
		c.mu.Unlock()
		http.Error(w, "Invalid or expired sign-in. Return to Party Hub.", 400)
		return
	}
	a.view.Status = "processing"
	c.mu.Unlock()
	finish := func(id string, err error) {
		c.mu.Lock()
		if c.attempt == a {
			if err != nil {
				a.view = LoginView{Status: "error", Message: err.Error()}
			} else {
				a.view = LoginView{Status: "success", AccountID: id}
			}
		}
		c.mu.Unlock()
		if err != nil {
			http.Error(w, "Sign-in could not finish. Return to Party Hub for details.", 400)
		} else {
			fmt.Fprint(w, "Connected. Return to Party Hub to choose a model.")
		}
		time.AfterFunc(time.Second, a.cancel)
	}
	if q.Get("error") != "" {
		finish("", errors.New("Sign-in was declined. Your current connection is unchanged."))
		return
	}
	clientID := a.clientID
	if clientID == "dynamic_agent_client" {
		clientID = q.Get("client_id")
		if clientID == "" || clientID == "dynamic_agent_client" || len(clientID) > 512 {
			finish("", errors.New("Registration was incomplete. Try signing in again."))
			return
		}
	} else if id := q.Get("client_id"); id != "" && id != clientID {
		finish("", errors.New("The sign-in account did not match. Try again."))
		return
	}
	c.mu.Lock()
	if c.attempt == a {
		a.clientID = clientID
	}
	c.mu.Unlock()
	if q.Get("code") == "" {
		finish("", errors.New("No authorization code received. Try again."))
		return
	}
	t, err := c.exchange(ctx, url.Values{"grant_type": {"authorization_code"}, "client_id": {clientID}, "code": {q.Get("code")}, "code_verifier": {a.verifier}, "redirect_uri": {a.redirect}, "resource": {apiResource}})
	if err != nil {
		finish("", err)
		return
	}
	claims, err := c.validateIdentity(ctx, t.ID, clientID, a.nonce)
	if err != nil {
		finish("", err)
		return
	}
	c.mu.Lock()
	if c.attempt != a || ctx.Err() != nil || a.view.Status != "processing" {
		c.mu.Unlock()
		finish("", errors.New("Sign-in was canceled. Try again."))
		return
	}
	var account ChatGPTAccount
	if a.accountID != "" {
		old := c.accountLocked(a.accountID)
		if old == nil || old.Subject != claims.Subject {
			c.mu.Unlock()
			finish("", errors.New("The signed-in identity does not match the selected account."))
			return
		}
		account = *old
	} else {
		for _, old := range c.file.Accounts {
			if old.ClientID == clientID && old.Subject == claims.Subject {
				account = old
				break
			}
		}
		if account.ID == "" {
			account = ChatGPTAccount{ID: opaque(), ClientID: clientID, Subject: claims.Subject}
		}
	}
	account.Email = claims.Email
	err = c.storeLocked(&account, t, a.remember)
	c.mu.Unlock()
	finish(account.ID, err)
}
func (c *chatGPT) http(ctx context.Context, method, endpoint string, form url.Values, bearer string) ([]byte, error) {
	var body io.Reader
	if form != nil {
		body = strings.NewReader(form.Encode())
	}
	req, err := http.NewRequestWithContext(ctx, method, endpoint, body)
	if err != nil {
		return nil, errors.New("could not prepare ChatGPT request")
	}
	if form != nil {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	client := *c.service.Client
	client.Timeout = 30 * time.Second
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	res, err := client.Do(req)
	if err != nil {
		return nil, errors.New("could not reach ChatGPT; check your connection and try again")
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return nil, errors.New("ChatGPT access could not be renewed or authorized. Sign in again.")
	}
	// Account model catalogs include substantial capability metadata. Keep token
	// and identity responses small, but allow the larger documented catalog.
	limit := int64(512 * 1024)
	if method == http.MethodGet && endpoint == apiResource+"/models" {
		limit = 8 * 1024 * 1024
	}
	b, err := io.ReadAll(io.LimitReader(res.Body, limit+1))
	if err != nil || int64(len(b)) > limit {
		return nil, errors.New("ChatGPT returned an unreadable response")
	}
	return b, nil
}
func hasScope(scope, want string) bool {
	for _, s := range strings.Fields(scope) {
		if s == want {
			return true
		}
	}
	return false
}
func (c *chatGPT) exchange(ctx context.Context, form url.Values, previous ...tokenSet) (tokenSet, error) {
	b, err := c.http(ctx, "POST", authIssuer+"/api/accounts/oauth/token", form, "")
	if err != nil {
		return tokenSet{}, err
	}
	var t tokenSet
	if json.Unmarshal(b, &t) != nil {
		return t, errors.New("ChatGPT returned incomplete credentials. Sign in again.")
	}
	if len(previous) > 0 {
		if t.Refresh == "" {
			t.Refresh = previous[0].Refresh
		}
		if t.Scope == "" {
			t.Scope = previous[0].Scope
		}
		if t.ID == "" {
			t.ID = previous[0].ID
		}
	}
	if t.Access == "" || t.Refresh == "" || !strings.EqualFold(t.Type, "Bearer") || t.ExpiresIn <= 0 {
		return t, errors.New("ChatGPT returned incomplete credentials. Sign in again.")
	}
	if !hasScope(t.Scope, planScope) || !hasScope(t.Scope, "resource.invoke") {
		return t, errors.New("ChatGPT plan access was not granted. Sign in again and allow plan usage.")
	}
	t.ExpiresAt = time.Now().Add(time.Duration(min(t.ExpiresIn, 86400)) * time.Second)
	return t, nil
}
func (c *chatGPT) tokensLocked(a *ChatGPTAccount) (tokenSet, error) {
	if a.CredentialID == "" {
		t := c.sessions[a.ID]
		if t.Access == "" {
			return t, errors.New("Sign in to ChatGPT again; this session has ended.")
		}
		return t, nil
	}
	raw, err := c.service.secrets.Get(a.CredentialID)
	var t tokenSet
	if err != nil || json.Unmarshal([]byte(raw), &t) != nil || t.Access == "" {
		return t, errors.New("Unlock your credential store or sign in to ChatGPT again.")
	}
	return t, nil
}
func (c *chatGPT) storeLocked(a *ChatGPTAccount, t tokenSet, remember bool) error {
	nextAccount := *a
	a = &nextAccount
	old := c.accountLocked(a.ID)
	oldCredential := ""
	if old != nil {
		oldCredential = old.CredentialID
	}
	a.CredentialID = ""
	if remember {
		a.CredentialID = "chatgpt-" + opaque()
		raw, _ := json.Marshal(t)
		if err := c.service.secrets.Set(a.CredentialID, string(raw)); err != nil {
			_ = c.service.secrets.Delete(a.CredentialID)
			return errors.New("Could not store ChatGPT credentials. Unlock your credential store or use session-only sign-in.")
		}
	}
	before := append([]ChatGPTAccount(nil), c.file.Accounts...)
	if old != nil {
		*old = *a
	} else {
		c.file.Accounts = append(c.file.Accounts, *a)
	}
	if err := c.persistLocked(); err != nil {
		c.file.Accounts = before
		if a.CredentialID != "" {
			_ = c.service.secrets.Delete(a.CredentialID)
		}
		return err
	}
	if remember {
		delete(c.sessions, a.ID)
	} else {
		c.sessions[a.ID] = t
	}
	if oldCredential != "" {
		_ = c.service.secrets.Delete(oldCredential)
	}
	return nil
}
func (c *chatGPT) access(ctx context.Context, id string) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.loadError != nil {
		return "", errors.New("Repair ChatGPT account settings before reconnecting.")
	}
	a := c.accountLocked(id)
	if a == nil {
		return "", errors.New("Choose a connected ChatGPT account in AI settings.")
	}
	t, err := c.tokensLocked(a)
	if err != nil {
		return "", err
	}
	if time.Until(t.ExpiresAt) > time.Minute {
		return t.Access, nil
	}
	next, err := c.exchange(ctx, url.Values{"grant_type": {"refresh_token"}, "client_id": {a.ClientID}, "refresh_token": {t.Refresh}, "resource": {apiResource}}, t)
	if err != nil {
		return "", err
	}
	// Retain the identity validated during interactive sign-in; refresh grants
	// renew access rather than switching the account identity.
	next.ID = t.ID
	copy := *a
	if err = c.storeLocked(&copy, next, a.CredentialID != ""); err != nil {
		return "", err
	}
	return next.Access, nil
}
func (s *Service) WelcomeChatGPT(id string) error {
	c := s.chatGPT
	c.mu.Lock()
	defer c.mu.Unlock()
	a := c.accountLocked(id)
	if a == nil {
		return errors.New("ChatGPT account not found")
	}
	old := a.Welcomed
	a.Welcomed = true
	if err := c.persistLocked(); err != nil {
		a.Welcomed = old
		return err
	}
	return nil
}
func (s *Service) SignOutChatGPT(ctx context.Context, id string) (string, error) {
	c := s.chatGPT
	c.mu.Lock()
	defer c.mu.Unlock()
	a := c.accountLocked(id)
	if a == nil {
		return "", errors.New("ChatGPT account not found")
	}
	if c.attempt != nil {
		c.attempt.cancel()
		c.attempt.view = LoginView{Status: "idle"}
	}
	for _, cancel := range c.running[id] {
		cancel()
	}
	t, err := c.tokensLocked(a)
	warning := ""
	if err == nil && t.Refresh != "" {
		metadata, discoveryErr := c.http(ctx, "GET", authIssuer+"/.well-known/openid-configuration", nil, "")
		var discovery struct {
			Issuer string `json:"issuer"`
			Revoke string `json:"revocation_endpoint"`
		}
		if discoveryErr == nil {
			discoveryErr = json.Unmarshal(metadata, &discovery)
		}
		endpoint, parseErr := url.Parse(discovery.Revoke)
		if discoveryErr != nil || parseErr != nil || discovery.Issuer != authIssuer || endpoint.Scheme != "https" || endpoint.Host != "auth.openai.com" || endpoint.User != nil || endpoint.RawQuery != "" || endpoint.Fragment != "" {
			err = errors.New("revocation discovery failed")
		} else {
			for attempt := 0; attempt < 2; attempt++ {
				_, err = c.http(ctx, "POST", discovery.Revoke, url.Values{"token": {t.Refresh}, "token_type_hint": {"refresh_token"}, "client_id": {a.ClientID}}, "")
				if err == nil {
					break
				}
				if attempt == 0 {
					select {
					case <-ctx.Done():
					case <-time.After(250 * time.Millisecond):
					}
				}
			}
		}
		if err != nil {
			warning = "Signed out locally. Remote sign-out was not confirmed; disconnect Party Hub in ChatGPT Settings."
		}
	} else if a.CredentialID != "" {
		warning = "Remote sign-out was not confirmed; disconnect Party Hub in ChatGPT Settings."
	}
	if a.CredentialID != "" {
		if err = c.service.secrets.Delete(a.CredentialID); err != nil {
			return "", errors.New("Could not remove ChatGPT credentials. Unlock your credential store and try again.")
		}
	}
	delete(c.sessions, id)
	a.CredentialID = ""
	if err = c.persistLocked(); err != nil {
		return "", err
	}
	return warning, nil
}

// A pick owns one cancelable context across both model calls and YouTube search.
func (c *chatGPT) requestContext(parent context.Context, id string) (context.Context, func(), error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	a := c.accountLocked(id)
	if a == nil || (a.CredentialID == "" && c.sessions[id].Access == "") {
		return nil, nil, errors.New("Sign in to ChatGPT again in AI settings.")
	}
	ctx, cancel := context.WithCancel(parent)
	key := opaque()
	if c.running[id] == nil {
		c.running[id] = map[string]context.CancelFunc{}
	}
	c.running[id][key] = cancel
	return ctx, func() { cancel(); c.mu.Lock(); delete(c.running[id], key); c.mu.Unlock() }, nil
}

// Keep the request context alive through queue insertion so sign-out cancels an
// in-flight pick even after its last model response has finished.
func (s *Service) PickContext(ctx context.Context) (context.Context, func(), error) {
	settings, err := s.Settings()
	if err != nil {
		return nil, nil, err
	}
	if settings.Provider != "chatgpt" {
		return ctx, func() {}, nil
	}
	return s.chatGPT.requestContext(ctx, settings.AccountID)
}
