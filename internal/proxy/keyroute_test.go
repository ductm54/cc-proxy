package proxy

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"go.uber.org/zap"

	"github.com/ductm54/cc-proxy/internal/config"
)

func TestKeyRoute(t *testing.T) {
	const secret = "s3cr&t^:key"

	var gotURI string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotURI = r.URL.RequestURI()
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, `{}`)
	}))
	defer upstream.Close()

	tp := &fakeTokenProvider{tok: newTestToken()}
	handler, _ := New(tp, zap.NewNop(), Options{
		UpstreamBase: upstream.URL,
		AuthConfig:   &config.AuthConfig{AuthSecret: secret},
	})

	cases := []struct {
		name string
		key  string
		want int
	}{
		{"valid", url.PathEscape(secret), http.StatusOK},
		{"wrong", "nope", http.StatusUnauthorized},
		{"prefix", url.PathEscape(secret[:4]), http.StatusUnauthorized},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gotURI = ""
			req := httptest.NewRequest(http.MethodGet, "/k/"+tc.key+"/v1/files?limit=1", nil)
			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)
			if rr.Code != tc.want {
				t.Fatalf("got status %d, want %d: %s", rr.Code, tc.want, rr.Body.String())
			}
			if tc.want == http.StatusOK && gotURI != "/v1/files?limit=1" {
				t.Fatalf("upstream got %q, want /v1/files?limit=1", gotURI)
			}
		})
	}
}

func TestKeyRoute_TestEndpoint(t *testing.T) {
	tp := &fakeTokenProvider{tok: newTestToken()}
	handler, _ := New(tp, zap.NewNop(), Options{AuthConfig: &config.AuthConfig{AuthSecret: "abc"}})

	req := httptest.NewRequest(http.MethodGet, "/k/abc/api/test", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("got status %d, want 200", rr.Code)
	}
	var body map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["ok"] != true || body["user"] != "config-key" || body["upstream_ready"] != true {
		t.Fatalf("unexpected body: %v", body)
	}

	req = httptest.NewRequest(http.MethodGet, "/k/wrong/api/test", nil)
	rr = httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("got status %d, want 401", rr.Code)
	}
}

func TestKeyRoute_DisabledWithoutSecret(t *testing.T) {
	tp := &fakeTokenProvider{tok: newTestToken()}
	handler, _ := New(tp, zap.NewNop(), Options{AuthConfig: &config.AuthConfig{}})

	req := httptest.NewRequest(http.MethodGet, "/k/anything/v1/models", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("got status %d, want 404", rr.Code)
	}
}
