package proxy

import (
	"encoding/json"
	"testing"
)

func TestRewriteAccountUUID(t *testing.T) {
	const want = "876cb664-1be3-44b2-af9e-10f8bcda336d"

	t.Run("replaces empty account_uuid", func(t *testing.T) {
		in := []byte(`{"model":"x","metadata":{"user_id":"{\"device_id\":\"d\",\"account_uuid\":\"\",\"session_id\":\"s\"}"}}`)
		out, err := rewriteAccountUUID(in, want)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		got := extractAccountUUID(t, out)
		if got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})

	t.Run("overwrites populated account_uuid", func(t *testing.T) {
		in := []byte(`{"metadata":{"user_id":"{\"account_uuid\":\"stale\"}"}}`)
		out, err := rewriteAccountUUID(in, want)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		if got := extractAccountUUID(t, out); got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})

	t.Run("injects missing account_uuid key", func(t *testing.T) {
		in := []byte(`{"metadata":{"user_id":"{\"device_id\":\"d\"}"}}`)
		out, err := rewriteAccountUUID(in, want)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		if got := extractAccountUUID(t, out); got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})

	t.Run("no metadata — pass through", func(t *testing.T) {
		in := []byte(`{"model":"x","messages":[]}`)
		out, err := rewriteAccountUUID(in, want)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		if string(out) != string(in) {
			t.Errorf("body should be unchanged, got %s", out)
		}
	})

	t.Run("invalid outer JSON — returns original with error", func(t *testing.T) {
		in := []byte(`not-json`)
		out, err := rewriteAccountUUID(in, want)
		if err == nil {
			t.Errorf("expected error for invalid JSON")
		}
		if string(out) != string(in) {
			t.Errorf("body should be unchanged")
		}
	})

	t.Run("user_id not a string — pass through", func(t *testing.T) {
		in := []byte(`{"metadata":{"user_id":42}}`)
		out, err := rewriteAccountUUID(in, want)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		if string(out) != string(in) {
			t.Errorf("body should be unchanged")
		}
	})

	t.Run("empty accountUUID — pass through", func(t *testing.T) {
		in := []byte(`{"metadata":{"user_id":"{\"account_uuid\":\"stale\"}"}}`)
		out, err := rewriteAccountUUID(in, "")
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		if string(out) != string(in) {
			t.Errorf("body should be unchanged with empty uuid")
		}
	})
}

func extractAccountUUID(t *testing.T, body []byte) string {
	t.Helper()
	var outer map[string]any
	if err := json.Unmarshal(body, &outer); err != nil {
		t.Fatalf("parse outer: %v", err)
	}
	meta := outer["metadata"].(map[string]any)
	userIDStr := meta["user_id"].(string)
	var inner map[string]any
	if err := json.Unmarshal([]byte(userIDStr), &inner); err != nil {
		t.Fatalf("parse user_id: %v", err)
	}
	u, _ := inner["account_uuid"].(string)
	return u
}
