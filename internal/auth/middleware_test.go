package auth

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"
)

func setupMiddlewareTest(store *TokenStore, token string) (*httptest.ResponseRecorder, *http.Request, http.Handler) {
	var gotEmail string
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotEmail = GetUserEmail(r.Context())
		_ = gotEmail
		w.WriteHeader(http.StatusOK)
	})

	r := chi.NewRouter()
	r.Route("/p/{token}", func(r chi.Router) {
		r.Use(RequirePathToken(store, nil))
		r.Get("/*", inner)
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/p/"+token+"/v1/models", nil)
	return rec, req, r
}

func TestRequirePathToken_Valid(t *testing.T) {
	store := NewTokenStore(nil, zap.NewNop())
	token, _, _ := store.Create("user@test.com", time.Hour)

	var gotEmail string
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotEmail = GetUserEmail(r.Context())
		w.WriteHeader(http.StatusOK)
	})

	r := chi.NewRouter()
	r.Route("/p/{token}", func(r chi.Router) {
		r.Use(RequirePathToken(store, nil))
		r.Get("/*", inner)
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/p/"+token+"/v1/models", nil)
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("got status %d, want 200", rec.Code)
	}
	if gotEmail != "user@test.com" {
		t.Fatalf("got email %q, want user@test.com", gotEmail)
	}
}

func TestRequirePathToken_InvalidToken(t *testing.T) {
	store := NewTokenStore(nil, zap.NewNop())

	rec, req, handler := setupMiddlewareTest(store, "badtoken1")
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("got status %d, want 401", rec.Code)
	}
}

func TestRequirePathToken_ExpiredToken(t *testing.T) {
	store := NewTokenStore(nil, zap.NewNop())
	token, _, _ := store.Create("user@test.com", -time.Second)

	rec, req, handler := setupMiddlewareTest(store, token)
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("got status %d, want 401", rec.Code)
	}
}

func TestRequirePathToken_ErrorFormat(t *testing.T) {
	store := NewTokenStore(nil, zap.NewNop())

	rec, req, handler := setupMiddlewareTest(store, "badtoken1")
	handler.ServeHTTP(rec, req)

	var body struct {
		Error struct {
			Type    string `json:"type"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Error.Type != "authentication_error" {
		t.Fatalf("got type %q, want authentication_error", body.Error.Type)
	}
}
