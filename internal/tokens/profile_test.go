package tokens

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestFetchProfile_Success(t *testing.T) {
	const body = `{"account":{"uuid":"876cb664-1be3-44b2-af9e-10f8bcda336d","email":"a@b"}}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer abc" {
			t.Errorf("Authorization: got %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, body)
	}))
	defer srv.Close()

	uuid, err := FetchProfile(context.Background(), srv.Client(), srv.URL, "abc")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if uuid != "876cb664-1be3-44b2-af9e-10f8bcda336d" {
		t.Errorf("got %q", uuid)
	}
}

func TestFetchProfile_Non200(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	_, err := FetchProfile(context.Background(), srv.Client(), srv.URL, "abc")
	if err == nil || !strings.Contains(err.Error(), "401") {
		t.Errorf("expected 401 error, got %v", err)
	}
}

func TestFetchProfile_EmptyUUID(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{"account":{"uuid":""}}`)
	}))
	defer srv.Close()

	_, err := FetchProfile(context.Background(), srv.Client(), srv.URL, "abc")
	if err == nil {
		t.Errorf("expected error for empty uuid")
	}
}
