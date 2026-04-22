package proxy

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"go.uber.org/zap"

	"github.com/ductm54/cc-proxy/internal/tokens"
)

// fakeTokenProvider implements TokenProvider for tests.
type fakeTokenProvider struct {
	tok tokens.Token
	err error
}

func (f *fakeTokenProvider) Current() (tokens.Token, error) {
	return f.tok, f.err
}

func newTestToken() tokens.Token {
	return tokens.Token{
		AccessToken:  "sk-ant-oat01-test-token",
		RefreshToken: "sk-ant-ort01-test-refresh",
		ExpiresAt:    time.Now().Add(8 * time.Hour),
	}
}

func TestHandleMessages_HeaderRewrite(t *testing.T) {
	var capturedHeaders http.Header

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedHeaders = r.Header.Clone()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, `{"type":"message"}`)
	}))
	defer upstream.Close()

	tp := &fakeTokenProvider{tok: newTestToken()}
	handler, _ := New(tp, zap.NewNop(), Options{
		MessagesURL: upstream.URL + "/v1/messages",
	})

	body := strings.NewReader(`{"model":"claude-opus-4-5","max_tokens":10,"messages":[]}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/messages", body)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Api-Key", "should-be-stripped")
	req.Header.Set("X-Stainless-Os", "linux")
	req.Header.Set("Anthropic-Version", "2023-06-01")
	req.Header.Set("X-App", "cli")
	req.Header.Set("Anthropic-Dangerous-Direct-Browser-Access", "true")
	req.Header.Set("User-Agent", "claude-cli/2.1.109 (external, cli)")

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}

	// Authorization header must be set from token.
	got := capturedHeaders.Get("Authorization")
	want := "Bearer " + tp.tok.AccessToken
	if got != want {
		t.Errorf("Authorization: got %q, want %q", got, want)
	}

	// anthropic-beta must be the subscription list.
	if capturedHeaders.Get("Anthropic-Beta") != SubscriptionBetaList {
		t.Errorf("Anthropic-Beta: got %q", capturedHeaders.Get("Anthropic-Beta"))
	}

	// Client-supplied headers must pass through untouched.
	if capturedHeaders.Get("X-App") != "cli" {
		t.Errorf("X-App: got %q", capturedHeaders.Get("X-App"))
	}
	if capturedHeaders.Get("Anthropic-Version") != "2023-06-01" {
		t.Errorf("Anthropic-Version: got %q", capturedHeaders.Get("Anthropic-Version"))
	}
	if capturedHeaders.Get("Anthropic-Dangerous-Direct-Browser-Access") != "true" {
		t.Errorf("Anthropic-Dangerous-Direct-Browser-Access: got %q",
			capturedHeaders.Get("Anthropic-Dangerous-Direct-Browser-Access"))
	}
	if capturedHeaders.Get("User-Agent") != "claude-cli/2.1.109 (external, cli)" {
		t.Errorf("User-Agent: got %q", capturedHeaders.Get("User-Agent"))
	}

	// x-api-key must be stripped.
	if capturedHeaders.Get("X-Api-Key") != "" {
		t.Errorf("X-Api-Key should be stripped, got %q", capturedHeaders.Get("X-Api-Key"))
	}

	// X-Stainless-Os must pass through.
	if capturedHeaders.Get("X-Stainless-Os") != "linux" {
		t.Errorf("X-Stainless-Os: got %q, want %q", capturedHeaders.Get("X-Stainless-Os"), "linux")
	}
}

func TestShouldForward(t *testing.T) {
	cases := []struct {
		key  string
		want bool
	}{
		{"Content-Type", true},
		{"Accept", true},
		{"Content-Length", true},
		{"X-Stainless-Os", true},
		{"X-Stainless-Arch", true},
		{"Anthropic-Version", true},
		{"anthropic-dangerous-direct-browser-access", true},
		{"X-App", true},
		{"User-Agent", true},
		{"Authorization", false},
		{"X-Api-Key", false},
		{"Anthropic-Beta", false},
		{"Connection", false},
		{"Transfer-Encoding", false},
	}
	for _, tc := range cases {
		got := shouldForward(tc.key)
		if got != tc.want {
			t.Errorf("shouldForward(%q) = %v, want %v", tc.key, got, tc.want)
		}
	}
}

func TestHandleMessages_RewritesAccountUUID(t *testing.T) {
	var capturedBody []byte

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, `{}`)
	}))
	defer upstream.Close()

	tok := newTestToken()
	tok.AccountUUID = "876cb664-1be3-44b2-af9e-10f8bcda336d"
	tp := &fakeTokenProvider{tok: tok}
	handler, _ := New(tp, zap.NewNop(), Options{MessagesURL: upstream.URL + "/v1/messages"})

	body := `{"model":"x","messages":[],"metadata":{"user_id":"{\"device_id\":\"dev\",\"account_uuid\":\"\",\"session_id\":\"s\"}"}}`
	req := httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	var parsed map[string]any
	if err := json.Unmarshal(capturedBody, &parsed); err != nil {
		t.Fatalf("parse upstream body: %v", err)
	}
	meta, _ := parsed["metadata"].(map[string]any)
	userIDStr, _ := meta["user_id"].(string)
	var inner map[string]any
	if err := json.Unmarshal([]byte(userIDStr), &inner); err != nil {
		t.Fatalf("parse user_id: %v", err)
	}
	if inner["account_uuid"] != tok.AccountUUID {
		t.Errorf("account_uuid: got %v, want %s", inner["account_uuid"], tok.AccountUUID)
	}
	if inner["device_id"] != "dev" || inner["session_id"] != "s" {
		t.Errorf("device_id/session_id mangled: %v", inner)
	}
}
