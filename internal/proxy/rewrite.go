package proxy

import (
	"encoding/json"
	"fmt"
)

// rewriteAccountUUID parses body as JSON, rewrites metadata.user_id.account_uuid
// to accountUUID, and returns the re-encoded bytes. If body is not valid JSON
// or metadata.user_id is missing/unparseable, the original body is returned
// unchanged (with a non-nil error so callers can log).
//
// Claude Code sends metadata.user_id as a JSON-encoded string rather than a
// nested object, e.g. `"metadata":{"user_id":"{\"device_id\":\"...\"}"}`.
func rewriteAccountUUID(body []byte, accountUUID string) ([]byte, error) {
	if accountUUID == "" {
		return body, nil
	}
	var outer map[string]any
	if err := json.Unmarshal(body, &outer); err != nil {
		return body, fmt.Errorf("parse outer body: %w", err)
	}
	metaVal, ok := outer["metadata"]
	if !ok {
		return body, nil
	}
	meta, ok := metaVal.(map[string]any)
	if !ok {
		return body, nil
	}
	userIDVal, ok := meta["user_id"]
	if !ok {
		return body, nil
	}
	userIDStr, ok := userIDVal.(string)
	if !ok || userIDStr == "" {
		return body, nil
	}
	var inner map[string]any
	if err := json.Unmarshal([]byte(userIDStr), &inner); err != nil {
		return body, fmt.Errorf("parse metadata.user_id: %w", err)
	}
	inner["account_uuid"] = accountUUID
	innerBytes, err := json.Marshal(inner)
	if err != nil {
		return body, fmt.Errorf("marshal metadata.user_id: %w", err)
	}
	meta["user_id"] = string(innerBytes)
	outer["metadata"] = meta
	return json.Marshal(outer)
}
