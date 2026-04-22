package tokens

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ductm54/cc-proxy/internal/config"
	"github.com/ductm54/cc-proxy/internal/httputil"
)

func TestRefresh_RequestShape(t *testing.T) {
	var capturedBody map[string]string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if err := json.NewDecoder(r.Body).Decode(&capturedBody); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		// Return a new token.
		resp := tokenResponse{
			AccessToken:  "sk-ant-oat01-new",
			RefreshToken: "sk-ant-ort01-new",
			ExpiresIn:    28800,
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	tok, err := Refresh(context.Background(), &http.Client{}, srv.URL, "sk-ant-ort01-old")
	if err != nil {
		t.Fatalf("Refresh error: %v", err)
	}

	// Verify request body fields.
	if capturedBody["grant_type"] != "refresh_token" {
		t.Errorf("grant_type: got %q", capturedBody["grant_type"])
	}
	if capturedBody["client_id"] != oauthClientID {
		t.Errorf("client_id: got %q, want %q", capturedBody["client_id"], oauthClientID)
	}
	if capturedBody["refresh_token"] != "sk-ant-ort01-old" {
		t.Errorf("refresh_token: got %q", capturedBody["refresh_token"])
	}

	// Verify returned token.
	if tok.AccessToken != "sk-ant-oat01-new" {
		t.Errorf("AccessToken: got %q", tok.AccessToken)
	}
	if tok.RefreshToken != "sk-ant-ort01-new" {
		t.Errorf("RefreshToken: got %q", tok.RefreshToken)
	}
	if tok.ExpiresAt.Before(time.Now().Add(7 * time.Hour)) {
		t.Errorf("ExpiresAt seems too early: %v", tok.ExpiresAt)
	}
}

func TestRefresh_TokenPersisted(t *testing.T) {
	dir := t.TempDir()
	tokFile := filepath.Join(dir, ".credentials.json")

	// Write initial token.
	initial := Token{
		AccessToken:  "sk-ant-oat01-old",
		RefreshToken: "sk-ant-ort01-old",
		ExpiresAt:    time.Now().Add(10 * time.Second),
	}
	if err := Save(tokFile, initial); err != nil {
		t.Fatal(err)
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := tokenResponse{
			AccessToken:  "sk-ant-oat01-rotated",
			RefreshToken: "sk-ant-ort01-rotated",
			ExpiresIn:    28800,
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	newTok, err := Refresh(context.Background(), &http.Client{}, srv.URL, initial.RefreshToken)
	if err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	if err := Save(tokFile, newTok); err != nil {
		t.Fatalf("Save: %v", err)
	}

	// Read back and verify.
	loaded, err := Load(tokFile)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if loaded.AccessToken != "sk-ant-oat01-rotated" {
		t.Errorf("persisted AccessToken: got %q", loaded.AccessToken)
	}
	if loaded.RefreshToken != "sk-ant-ort01-rotated" {
		t.Errorf("persisted RefreshToken: got %q", loaded.RefreshToken)
	}

	// Verify file has 0600 permissions.
	fi, err := os.Stat(tokFile)
	if err != nil {
		t.Fatal(err)
	}
	if fi.Mode().Perm() != 0600 {
		t.Errorf("file perms: got %o, want 0600", fi.Mode().Perm())
	}
}

func TestRefresh_KeepsOldRefreshTokenIfNoneReturned(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// No refresh_token in response.
		resp := map[string]interface{}{
			"access_token": "sk-ant-oat01-fresh",
			"expires_in":   28800,
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	tok, err := Refresh(context.Background(), &http.Client{}, srv.URL, "sk-ant-ort01-keep-me")
	if err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	if tok.RefreshToken != "sk-ant-ort01-keep-me" {
		t.Errorf("RefreshToken should be kept: got %q", tok.RefreshToken)
	}
}

func TestIntegration_Refresh(t *testing.T) {
	t.Skip("integration test: requires valid tokens at default location")

	tokFile := config.DefaultTokensFile()
	tok, err := Load(tokFile)
	if err != nil {
		t.Fatalf("Load(%s): %v", tokFile, err)
	}

	refreshed, err := Refresh(context.Background(), httputil.NewLoggingClient(t), oauthTokenURL, tok.RefreshToken)
	if err != nil {
		t.Fatalf("Refresh: %v", err)
	}

	if refreshed.AccessToken == "" {
		t.Fatal("refreshed access_token is empty")
	}
	if refreshed.RefreshToken == "" {
		t.Fatal("refreshed refresh_token is empty")
	}
	if refreshed.ExpiresAt.Before(time.Now()) {
		t.Fatalf("refreshed token already expired: %v", refreshed.ExpiresAt)
	}

	t.Logf("access_token: %s...%s", refreshed.AccessToken[:16], refreshed.AccessToken[len(refreshed.AccessToken)-4:])
	t.Logf("expires_at:   %v", refreshed.ExpiresAt)
}
