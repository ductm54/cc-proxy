package proxy

import (
	"bytes"
	"encoding/json"
	"fmt"
)

// rewriteAccountUUID sets metadata.user_id.account_uuid in body to accountUUID.
// Only the bytes of the user_id string literal are replaced, so key order,
// whitespace and escaping of the rest of the body stay exactly as the client
// sent them. If account_uuid already matches, body is returned unchanged. If
// body is not valid JSON or metadata.user_id is missing/unparseable, the
// original body is returned (with a non-nil error for parse failures so
// callers can log).
//
// Claude Code sends metadata.user_id as a JSON-encoded string rather than a
// nested object, e.g. `"metadata":{"user_id":"{\"device_id\":\"...\"}"}`.
func rewriteAccountUUID(body []byte, accountUUID string) ([]byte, error) {
	if accountUUID == "" {
		return body, nil
	}
	ms, me, found, err := findMember(body, "metadata")
	if err != nil {
		return body, fmt.Errorf("parse outer body: %w", err)
	}
	if !found || body[ms] != '{' {
		return body, nil
	}
	us, ue, found, err := findMember(body[ms:me], "user_id")
	if err != nil {
		return body, fmt.Errorf("parse metadata: %w", err)
	}
	us, ue = ms+us, ms+ue
	if !found || body[us] != '"' {
		return body, nil
	}
	var userID string
	if err := json.Unmarshal(body[us:ue], &userID); err != nil || userID == "" {
		return body, nil
	}
	newID, changed, err := setAccountUUID([]byte(userID), accountUUID)
	if err != nil {
		return body, fmt.Errorf("parse metadata.user_id: %w", err)
	}
	if !changed {
		return body, nil
	}
	lit := marshalNoEscape(string(newID))
	out := make([]byte, 0, len(body)-(ue-us)+len(lit))
	out = append(out, body[:us]...)
	out = append(out, lit...)
	return append(out, body[ue:]...), nil
}

// setAccountUUID sets account_uuid in the JSON object obj, keeping the other
// members byte-for-byte and in order. It reports whether obj changed.
func setAccountUUID(obj []byte, accountUUID string) ([]byte, bool, error) {
	s, e, found, err := findMember(obj, "account_uuid")
	if err != nil {
		return obj, false, err
	}
	val := marshalNoEscape(accountUUID)
	var out []byte
	if found {
		var cur string
		if json.Unmarshal(obj[s:e], &cur) == nil && cur == accountUUID {
			return obj, false, nil
		}
		out = append(append(append(out, obj[:s]...), val...), obj[e:]...)
	} else {
		end := bytes.LastIndexByte(obj, '}')
		head := bytes.TrimRight(obj[:end], " \t\r\n")
		out = append(out, head...)
		if head[len(head)-1] != '{' {
			out = append(out, ',')
		}
		out = append(append(append(out, `"account_uuid":`...), val...), obj[end:]...)
	}
	return out, true, nil
}

// findMember returns the byte span [start, end) of the value of member key in
// the top-level JSON object obj. If key appears more than once, the last one
// wins, matching encoding/json. The whole object is validated.
func findMember(obj []byte, key string) (start, end int, found bool, err error) {
	dec := json.NewDecoder(bytes.NewReader(obj))
	tok, err := dec.Token()
	if err != nil {
		return 0, 0, false, err
	}
	if d, ok := tok.(json.Delim); !ok || d != '{' {
		return 0, 0, false, fmt.Errorf("not a JSON object")
	}
	for dec.More() {
		tok, err := dec.Token()
		if err != nil {
			return 0, 0, false, err
		}
		k, _ := tok.(string)
		var raw json.RawMessage
		if err := dec.Decode(&raw); err != nil {
			return 0, 0, false, err
		}
		if k == key {
			// RawMessage holds the value's exact bytes, which end at the
			// decoder's current offset.
			end = int(dec.InputOffset())
			start = end - len(raw)
			found = true
		}
	}
	if _, err := dec.Token(); err != nil {
		return 0, 0, false, err
	}
	return start, end, found, nil
}

// marshalNoEscape encodes s as a JSON string without HTML-escaping <, > and &.
func marshalNoEscape(s string) []byte {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(s)
	return bytes.TrimRight(buf.Bytes(), "\n")
}
