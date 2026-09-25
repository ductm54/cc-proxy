package proxy

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.uber.org/zap"

	"github.com/ductm54/cc-proxy/internal/debugdump"
)

func TestHandleMessages_DebugDump(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Request-Id", "req_upstream_1")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = io.WriteString(w, `{"type":"error","error":{"type":"invalid_request_error","message":"bad beta"}}`)
	}))
	defer upstream.Close()

	dir := t.TempDir()
	dumper, err := debugdump.New(dir, 1<<20, 2)
	if err != nil {
		t.Fatal(err)
	}
	tok := newTestToken()
	tok.AccountUUID = "acct-123"
	handler, _ := New(&fakeTokenProvider{tok: tok}, zap.NewNop(), Options{
		MessagesURL: upstream.URL + "/v1/messages",
		DebugDump:   dumper,
	})

	body := `{"model":"claude-opus-4-5","metadata":{"user_id":"{\"device_id\":\"d1\"}"},"messages":[]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(body))
	req.Header.Set("Anthropic-Beta", "client-beta-1")
	req.Header.Set("X-Api-Key", "sk-client-secret-key")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	_ = dumper.Close()

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d", rec.Code)
	}

	data, err := os.ReadFile(filepath.Join(dir, "dump.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	var got debugdump.Record
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("decode dump: %v\n%s", err, data)
	}

	if got.Client.Headers.Get("Anthropic-Beta") != "client-beta-1" {
		t.Errorf("client beta = %q", got.Client.Headers.Get("Anthropic-Beta"))
	}
	if v := got.Client.Headers.Get("X-Api-Key"); strings.Contains(v, "secret") {
		t.Errorf("client x-api-key not redacted: %q", v)
	}
	if got.Upstream == nil {
		t.Fatal("missing upstream request")
	}
	if got.Upstream.Headers.Get("Anthropic-Beta") != RequiredMessagesBetas+",client-beta-1" {
		t.Errorf("upstream beta = %q", got.Upstream.Headers.Get("Anthropic-Beta"))
	}
	if v := got.Upstream.Headers.Get("Authorization"); strings.Contains(v, "test-token") {
		t.Errorf("upstream authorization not redacted: %q", v)
	}
	if got.Upstream.BodySameAsClient {
		t.Error("account_uuid rewrite should make upstream body differ")
	}
	if !strings.Contains(string(mustJSON(t, got.Upstream.Body)), "acct-123") {
		t.Errorf("upstream body missing rewritten account_uuid: %s", mustJSON(t, got.Upstream.Body))
	}
	if got.Response == nil || got.Response.Status != http.StatusBadRequest {
		t.Fatalf("response = %+v", got.Response)
	}
	if got.Response.Headers.Get("Request-Id") != "req_upstream_1" {
		t.Errorf("response request-id = %q", got.Response.Headers.Get("Request-Id"))
	}
	if !strings.Contains(string(mustJSON(t, got.Response.Body)), "bad beta") {
		t.Errorf("response body = %s", mustJSON(t, got.Response.Body))
	}
}

func mustJSON(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return b
}
