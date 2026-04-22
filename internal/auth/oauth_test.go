package auth

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"go.uber.org/zap"
)

func newTestOAuthHandler() *OAuthHandler {
	store := NewTokenStore(nil, zap.NewNop())
	return NewOAuthHandler("client-id", "client-secret", "http://localhost/auth/callback", "example.com", "http://localhost:8787", store, time.Hour, zap.NewNop())
}

func TestHandleAuthInfo(t *testing.T) {
	h := newTestOAuthHandler()
	req := httptest.NewRequest(http.MethodGet, "/api/auth/info", nil)
	rec := httptest.NewRecorder()
	h.HandleAuthInfo(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("got status %d, want 200", rec.Code)
	}
	var body map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body["domain"] != "example.com" {
		t.Fatalf("got domain %q, want example.com", body["domain"])
	}
}

func TestHandleAuthInfo_NoDomain(t *testing.T) {
	store := NewTokenStore(nil, zap.NewNop())
	h := NewOAuthHandler("client-id", "client-secret", "http://localhost/auth/callback", "", "http://localhost:8787", store, time.Hour, zap.NewNop())

	req := httptest.NewRequest(http.MethodGet, "/api/auth/info", nil)
	rec := httptest.NewRecorder()
	h.HandleAuthInfo(rec, req)

	var body map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body["domain"] != "" {
		t.Fatalf("got domain %q, want empty", body["domain"])
	}
}

func TestHandleStart_Redirect(t *testing.T) {
	h := newTestOAuthHandler()
	req := httptest.NewRequest(http.MethodGet, "/auth/start", nil)
	rec := httptest.NewRecorder()
	h.HandleStart(rec, req)

	if rec.Code != http.StatusFound {
		t.Fatalf("got status %d, want 302", rec.Code)
	}
	loc := rec.Header().Get("Location")
	if loc == "" {
		t.Fatal("no Location header")
	}
}

func TestHandleStart_SetsCookie(t *testing.T) {
	h := newTestOAuthHandler()
	req := httptest.NewRequest(http.MethodGet, "/auth/start", nil)
	rec := httptest.NewRecorder()
	h.HandleStart(rec, req)

	found := false
	for _, c := range rec.Result().Cookies() {
		if c.Name == "oauth_state" && c.Value != "" {
			found = true
		}
	}
	if !found {
		t.Fatal("oauth_state cookie not set")
	}
}

func TestHandleCallback_StateMismatch(t *testing.T) {
	h := newTestOAuthHandler()
	req := httptest.NewRequest(http.MethodGet, "/auth/callback?state=abc&code=xyz", nil)
	req.AddCookie(&http.Cookie{Name: "oauth_state", Value: "different"})
	rec := httptest.NewRecorder()
	h.HandleCallback(rec, req)

	if rec.Code != http.StatusFound {
		t.Fatalf("got status %d, want 302 redirect", rec.Code)
	}
	loc := rec.Header().Get("Location")
	if loc == "" {
		t.Fatal("no Location header on error redirect")
	}
}

func TestHandleCallback_NoCookie(t *testing.T) {
	h := newTestOAuthHandler()
	req := httptest.NewRequest(http.MethodGet, "/auth/callback?state=abc&code=xyz", nil)
	rec := httptest.NewRecorder()
	h.HandleCallback(rec, req)

	if rec.Code != http.StatusFound {
		t.Fatalf("got status %d, want 302 redirect", rec.Code)
	}
}

func TestWriteJSON(t *testing.T) {
	rec := httptest.NewRecorder()
	writeJSON(rec, http.StatusOK, map[string]string{"hello": "world"})

	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("got content-type %q", ct)
	}
	var body map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body["hello"] != "world" {
		t.Fatalf("unexpected body: %v", body)
	}
}
