package ai

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func seededChatGPT(t *testing.T, remember bool) (*Service, *memorySecrets, string) {
	t.Helper()
	store := &memorySecrets{values: map[string]string{}}
	s := New(filepath.Join(t.TempDir(), "ai.json"), store)
	a := ChatGPTAccount{ID: "account-one", ClientID: "issued-client", Subject: "user-one", Email: "dj@example.test"}
	tokens := tokenSet{Access: "private-access", Refresh: "private-refresh", ID: "private-id", Scope: planScope + " resource.invoke", Type: "Bearer", ExpiresAt: time.Now().Add(time.Hour)}
	if err := s.chatGPT.storeLocked(&a, tokens, remember); err != nil {
		t.Fatal(err)
	}
	return s, store, a.ID
}
func signIdentity(t *testing.T, key *rsa.PrivateKey, claims map[string]any) string {
	t.Helper()
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"RS256","kid":"test-key"}`))
	raw, _ := json.Marshal(claims)
	payload := header + "." + base64.RawURLEncoding.EncodeToString(raw)
	sum := sha256.Sum256([]byte(payload))
	signature, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, sum[:])
	if err != nil {
		t.Fatal(err)
	}
	return payload + "." + base64.RawURLEncoding.EncodeToString(signature)
}
func jwksFor(key *rsa.PrivateKey) any {
	return map[string]any{"keys": []any{map[string]string{"kty": "RSA", "kid": "test-key", "alg": "RS256", "use": "sig", "n": base64.RawURLEncoding.EncodeToString(key.N.Bytes()), "e": base64.RawURLEncoding.EncodeToString(big.NewInt(int64(key.E)).Bytes())}}}
}
func TestChatGPTRegistrationCallbackAndIdentity(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name                                 string
		badNonce, badSignature, missingScope bool
	}{{name: "success"}, {name: "nonce", badNonce: true}, {name: "signature", badSignature: true}, {name: "permission", missingScope: true}} {
		t.Run(test.name, func(t *testing.T) {
			store := &memorySecrets{values: map[string]string{}}
			s := New(filepath.Join(t.TempDir(), "ai.json"), store)
			link, err := s.StartChatGPTLogin("", true)
			if err != nil {
				t.Fatal(err)
			}
			defer s.CancelChatGPTLogin()
			u, _ := url.Parse(link)
			q := u.Query()
			a := s.chatGPT.attempt
			if q.Get("client_id") != "dynamic_agent_client" || q.Get("agent_name_hint") != "Party Hub" || !strings.HasPrefix(q.Get("ext_agent_host_id"), "urn:uuid:") || q.Get("code_challenge_method") != "S256" {
				t.Fatalf("invalid authorization parameters")
			}
			challenge := sha256.Sum256([]byte(a.verifier))
			if q.Get("code_challenge") != base64.RawURLEncoding.EncodeToString(challenge[:]) {
				t.Fatal("incorrect PKCE")
			}
			s.Client = &http.Client{Transport: transportFunc(func(r *http.Request) (*http.Response, error) {
				switch r.URL.Path {
				case "/api/accounts/oauth/token":
					if err := r.ParseForm(); err != nil {
						t.Fatal(err)
					}
					if r.Form.Get("client_id") != "issued-client" || r.Form.Get("redirect_uri") != q.Get("redirect_uri") || r.Form.Get("code_verifier") != a.verifier {
						t.Error("invalid code exchange")
					}
					nonce := a.nonce
					if test.badNonce {
						nonce = "wrong"
					}
					claims := map[string]any{"iss": authIssuer, "sub": "user-one", "aud": "issued-client", "exp": time.Now().Add(time.Hour).Unix(), "iat": time.Now().Unix(), "nonce": nonce, "email": "dj@example.test"}
					id := signIdentity(t, key, claims)
					if test.badSignature {
						id = id[:len(id)-8] + "AAAAAAAA"
					}
					scope := planScope + " resource.invoke"
					if test.missingScope {
						scope = "openid email"
					}
					return response(map[string]any{"access_token": "private-access", "refresh_token": "private-refresh", "id_token": id, "token_type": "Bearer", "expires_in": 3600, "scope": scope}), nil
				case "/.well-known/jwks.json":
					return response(jwksFor(key)), nil
				default:
					t.Fatalf("unexpected request %s", r.URL.Path)
					return nil, nil
				}
			})}
			// A forged callback must not consume the pending login.
			forged := httptest.NewRecorder()
			s.chatGPT.callback(context.Background(), a, forged, httptest.NewRequest("GET", a.redirect+"?state=wrong&code=forged", nil))
			if forged.Code != 400 || a.view.Status != "pending" {
				t.Fatal("forged state consumed login")
			}
			values := url.Values{"state": {a.state}, "code": {"code"}, "client_id": {"issued-client"}}
			res := httptest.NewRecorder()
			s.chatGPT.callback(context.Background(), a, res, httptest.NewRequest("GET", a.redirect+"?"+values.Encode(), nil))
			status, _ := s.ChatGPTStatus()
			if test.name != "success" {
				if res.Code == 200 || status.Login.Status != "error" || len(status.Accounts) != 0 {
					t.Fatalf("accepted invalid sign-in: %#v", status)
				}
				return
			}
			if res.Code != 200 || len(status.Accounts) != 1 || !status.Accounts[0].Connected {
				t.Fatalf("callback failed: %d %#v", res.Code, status)
			}
			public, _ := json.Marshal(status)
			raw, err := os.ReadFile(s.chatGPT.path)
			if err != nil {
				t.Fatal(err)
			}
			for _, secret := range []string{"private-access", "private-refresh", a.verifier} {
				if strings.Contains(string(raw), secret) || strings.Contains(string(public), secret) {
					t.Fatal("credentials leaked")
				}
			}
			info, _ := os.Stat(s.chatGPT.path)
			if info.Mode().Perm() != 0600 {
				t.Fatal("unsafe file mode")
			}
			restored := New(s.path, store)
			if !restored.chatGPT.connected(status.Accounts[0].ID) {
				t.Fatal("secure login did not survive restart")
			}
			host := s.chatGPT.file.HostID
			next, err := s.StartChatGPTLogin(status.Accounts[0].ID, true)
			if err != nil {
				t.Fatal(err)
			}
			again, _ := url.Parse(next)
			if again.Query().Get("client_id") != "issued-client" || again.Query().Get("ext_agent_host_id") != host || again.Query().Get("agent_name_hint") != "" || again.Query().Get("id_token_hint") != "" {
				t.Fatal("registration was not reused safely")
			}
		})
	}
}
func TestChatGPTSessionOnlyAndFailedStorage(t *testing.T) {
	s, store, id := seededChatGPT(t, false)
	if len(store.values) != 0 {
		t.Fatal("session tokens persisted")
	}
	restored := New(s.path, store)
	if restored.chatGPT.connected(id) {
		t.Fatal("session login restored")
	}
	a := *s.chatGPT.accountLocked(id)
	tokens := s.chatGPT.sessions[id]
	store.fail = true
	if err := s.chatGPT.storeLocked(&a, tokens, true); err == nil {
		t.Fatal("storage failure ignored")
	}
	if !s.chatGPT.connected(id) {
		t.Fatal("failed save destroyed working session")
	}
}
func TestChatGPTRefreshIsSerializedAndRotatesCredentials(t *testing.T) {
	s, store, id := seededChatGPT(t, true)
	a := s.chatGPT.accountLocked(id)
	tokens, _ := s.chatGPT.tokensLocked(a)
	tokens.ExpiresAt = time.Now()
	if err := s.chatGPT.storeLocked(a, tokens, true); err != nil {
		t.Fatal(err)
	}
	var calls atomic.Int32
	s.Client = &http.Client{Transport: transportFunc(func(r *http.Request) (*http.Response, error) {
		calls.Add(1)
		_ = r.ParseForm()
		if r.Form.Get("client_id") != "issued-client" || r.Form.Get("refresh_token") != "private-refresh" || r.Form.Get("scope") != "" {
			t.Error("invalid refresh")
		}
		return response(map[string]any{"access_token": "renewed-access", "refresh_token": "renewed-refresh", "token_type": "Bearer", "expires_in": 3600}), nil
	})}
	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			token, err := s.chatGPT.access(context.Background(), id)
			if err != nil || token != "renewed-access" {
				t.Errorf("renewal failed: %v", err)
			}
		}()
	}
	wg.Wait()
	if calls.Load() != 1 {
		t.Fatalf("refresh raced: %d", calls.Load())
	}
	restored := New(s.path, store)
	token, err := restored.chatGPT.access(context.Background(), id)
	if err != nil || token != "renewed-access" {
		t.Fatal("rotated token was not persisted")
	}
	if len(store.values) != 1 {
		t.Fatal("old credentials retained")
	}
}
func TestChatGPTModelsAndRequestShape(t *testing.T) {
	s, _, id := seededChatGPT(t, false)
	s.Client = &http.Client{Transport: transportFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Host != "api.openai.com" || r.Header.Get("Authorization") != "Bearer private-access" {
			t.Error("invalid destination or credential")
		}
		if r.URL.Path == "/v1/models" {
			return response(map[string]any{"models": []any{map[string]string{"slug": "first", "display_name": "First model", "visibility": "list"}, map[string]string{"slug": "hidden", "visibility": "hide"}, map[string]string{"slug": "second", "display_name": "Second model", "visibility": "list"}}}), nil
		}
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body["stream"] != true || body["store"] != false || body["max_output_tokens"] != nil {
			t.Error("unsupported inference request")
		}
		if _, ok := body["input"].([]any); !ok {
			t.Error("input must be an array")
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader("data: {\"type\":\"response.output_text.delta\",\"delta\":\"{\\\"ok\\\":true}\"}\n\ndata: {\"type\":\"response.completed\",\"response\":{\"status\":\"completed\"}}\n\n"))}, nil
	})}
	models, err := s.ChatGPTModels(context.Background(), id)
	if err != nil || len(models) != 2 || models[0].ID != "first" || models[0].Name != "First model" {
		t.Fatalf("incorrect catalog: %#v %v", models, err)
	}
	result, err := s.Complete(context.Background(), Draft{Provider: "chatgpt", AccountID: id, Model: "first"}, "return JSON", "test")
	if err != nil || result != `{"ok":true}` {
		t.Fatalf("completion %q %v", result, err)
	}
	if _, err = s.Save(Draft{Provider: "chatgpt", AccountID: id, Model: "hidden"}); err == nil {
		t.Fatal("saved unavailable model")
	}
	if _, err = s.Save(Draft{Provider: "chatgpt", AccountID: id, Model: "first"}); err != nil {
		t.Fatal(err)
	}
	settings, _ := s.Settings()
	if !settings.Configured || settings.HasKey {
		t.Fatal("incorrect ChatGPT settings")
	}
}
func TestChatGPTStreamFailuresNeverReturnPartialOutput(t *testing.T) {
	for _, end := range []string{"", `{"type":"response.incomplete"}`, `{"type":"response.failed","response":{"error":{"code":"subscription_sharing_usage_limit_exceeded"}}}`, `{"type":"response.completed","response":{"status":"incomplete"}}`, `not json`} {
		stream := "data: {\"type\":\"response.output_text.delta\",\"delta\":\"partial\"}\n\n"
		if end != "" {
			stream += "data: " + end + "\n\n"
		}
		result, err := readChatGPTStream(strings.NewReader(stream))
		if err == nil || result != "" {
			t.Fatalf("accepted failed stream %s", end)
		}
	}
}
func TestChatGPTSignOutCancelsRequestsAndRetainsRegistration(t *testing.T) {
	for _, remoteOK := range []bool{true, false} {
		t.Run(fmt.Sprint(remoteOK), func(t *testing.T) {
			s, store, id := seededChatGPT(t, true)
			ctx, release, err := s.chatGPT.requestContext(context.Background(), id)
			if err != nil {
				t.Fatal(err)
			}
			defer release()
			s.Client = &http.Client{Transport: transportFunc(func(r *http.Request) (*http.Response, error) {
				if r.URL.Path == "/.well-known/openid-configuration" {
					return response(map[string]string{"issuer": authIssuer, "revocation_endpoint": authIssuer + "/api/accounts/oauth/revoke"}), nil
				}
				if r.URL.Path != "/api/accounts/oauth/revoke" {
					t.Fatalf("unexpected request %s", r.URL.Path)
				}
				_ = r.ParseForm()
				if r.Form.Get("token") != "private-refresh" || r.Form.Get("client_id") != "issued-client" {
					t.Error("incorrect revocation")
				}
				if !remoteOK {
					return nil, errorsForTest()
				}
				return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(""))}, nil
			})}
			warning, err := s.SignOutChatGPT(context.Background(), id)
			if err != nil {
				t.Fatal(err)
			}
			if (warning == "") != remoteOK {
				t.Fatal("revocation status was hidden")
			}
			if ctx.Err() == nil || s.chatGPT.connected(id) || len(store.values) != 0 {
				t.Fatal("sign-out did not stop access")
			}
			if s.chatGPT.accountLocked(id).ClientID != "issued-client" {
				t.Fatal("sign-out destroyed registration")
			}
		})
	}
}
func errorsForTest() error { return fmt.Errorf("offline") }
func TestChatGPTCanceledCallbackCannotReplaceAccount(t *testing.T) {
	s, _, _ := seededChatGPT(t, false)
	_, err := s.StartChatGPTLogin("", false)
	if err != nil {
		t.Fatal(err)
	}
	a := s.chatGPT.attempt
	s.CancelChatGPTLogin()
	res := httptest.NewRecorder()
	s.chatGPT.callback(context.Background(), a, res, httptest.NewRequest("GET", a.redirect+"?state="+a.state+"&code=x&client_id=issued", nil))
	if res.Code != 400 {
		t.Fatal("canceled callback accepted")
	}
}

func TestChatGPTIdentityRejectsInvalidClaims(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	s := New("", &memorySecrets{values: map[string]string{}})
	s.Client = &http.Client{Transport: transportFunc(func(r *http.Request) (*http.Response, error) { return response(jwksFor(key)), nil })}
	for _, test := range []struct {
		name, field string
		value       any
	}{{"issuer", "iss", "https://untrusted.example"}, {"audience", "aud", "another-client"}, {"expiry", "exp", time.Now().Add(-time.Minute).Unix()}, {"subject", "sub", ""}, {"issued in future", "iat", time.Now().Add(time.Hour).Unix()}, {"authorized party", "azp", "another-client"}, {"multiple audiences without authorized party", "aud", []string{"issued-client", "another-client"}}} {
		t.Run(test.name, func(t *testing.T) {
			claims := map[string]any{"iss": authIssuer, "sub": "user-one", "aud": "issued-client", "exp": time.Now().Add(time.Hour).Unix(), "iat": time.Now().Unix(), "nonce": "expected"}
			claims[test.field] = test.value
			token := signIdentity(t, key, claims)
			if _, err := s.chatGPT.validateIdentity(context.Background(), token, "issued-client", "expected"); err == nil {
				t.Fatal("invalid identity accepted")
			}
		})
	}
}
func TestChatGPTReturningIdentityCannotOverwriteSavedAccount(t *testing.T) {
	s, _, id := seededChatGPT(t, false)
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.StartChatGPTLogin(id, false)
	if err != nil {
		t.Fatal(err)
	}
	defer s.CancelChatGPTLogin()
	attempt := s.chatGPT.attempt
	s.Client = &http.Client{Transport: transportFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path == "/.well-known/jwks.json" {
			return response(jwksFor(key)), nil
		}
		claims := map[string]any{"iss": authIssuer, "sub": "another-person", "aud": "issued-client", "exp": time.Now().Add(time.Hour).Unix(), "iat": time.Now().Unix(), "nonce": attempt.nonce}
		return response(map[string]any{"access_token": "wrong-account-access", "refresh_token": "wrong-account-refresh", "id_token": signIdentity(t, key, claims), "token_type": "Bearer", "expires_in": 3600, "scope": planScope + " resource.invoke"}), nil
	})}
	req := httptest.NewRequest("GET", attempt.redirect+"?"+url.Values{"state": {attempt.state}, "code": {"code"}}.Encode(), nil)
	res := httptest.NewRecorder()
	s.chatGPT.callback(context.Background(), attempt, res, req)
	if res.Code == 200 {
		t.Fatal("different identity accepted")
	}
	token, err := s.chatGPT.access(context.Background(), id)
	if err != nil || token != "private-access" {
		t.Fatal("failed login replaced working credentials")
	}
}

func TestChatGPTLargeModelCatalogDoesNotRelaxTokenResponseLimit(t *testing.T) {
	s, _, id := seededChatGPT(t, false)
	metadata := strings.Repeat("x", 600*1024)
	s.Client = &http.Client{Transport: transportFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path == "/v1/models" {
			return response(map[string]any{"models": []any{map[string]string{"slug": "party-model", "display_name": "Party model", "visibility": "list", "capabilities_metadata": metadata}}}), nil
		}
		return response(map[string]string{"unexpected": metadata}), nil
	})}
	models, err := s.ChatGPTModels(context.Background(), id)
	if err != nil || len(models) != 1 || models[0].ID != "party-model" {
		t.Fatalf("large catalog failed: %#v %v", models, err)
	}
	if _, err = s.chatGPT.http(context.Background(), "POST", authIssuer+"/api/accounts/oauth/token", url.Values{}, ""); err == nil {
		t.Fatal("token response size limit was relaxed")
	}
	s.Client = &http.Client{Transport: transportFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(strings.Repeat("x", 8*1024*1024+1)))}, nil
	})}
	if _, err = s.ChatGPTModels(context.Background(), id); err == nil {
		t.Fatal("oversized model catalog accepted")
	}
}
