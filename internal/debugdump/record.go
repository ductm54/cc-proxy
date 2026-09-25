package debugdump

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"
)

// Record is one proxied exchange: what the client sent, what the proxy sent
// upstream, and what came back.
type Record struct {
	Time     time.Time `json:"ts"`
	ReqID    string    `json:"req_id"`
	User     string    `json:"user,omitempty"`
	DurMS    int64     `json:"dur_ms"`
	Client   Request   `json:"client"`
	Upstream *Request  `json:"upstream,omitempty"`
	Response *Response `json:"response,omitempty"`
	Error    string    `json:"error,omitempty"`
}

// Request is one side of the outbound hop (client→proxy or proxy→upstream).
type Request struct {
	Method    string      `json:"method"`
	URL       string      `json:"url"`
	Headers   http.Header `json:"headers"`
	BodyBytes int         `json:"body_bytes"`
	Body      any         `json:"body,omitempty"`
	// BodySameAsClient is set on the upstream request when the proxy forwarded
	// the client body byte-for-byte, in which case Body is omitted.
	BodySameAsClient bool `json:"body_same_as_client,omitempty"`
}

// Response is the upstream response as relayed to the client.
type Response struct {
	Status     int         `json:"status"`
	Headers    http.Header `json:"headers"`
	BodyBytes  int64       `json:"body_bytes"`
	Body       any         `json:"body,omitempty"`
	Truncated  bool        `json:"body_truncated,omitempty"`
	BodyNotice string      `json:"body_notice,omitempty"`
}

// Dumper serialises Records onto a rotating Writer.
type Dumper struct {
	w *Writer
	// MaxResponseBody caps how many response bytes are kept per record.
	MaxResponseBody int
}

// DefaultMaxResponseBody is the per-record response body cap.
const DefaultMaxResponseBody = 4 << 20

// New creates a Dumper writing into dir, rotating at maxSize bytes and keeping
// maxFiles rotated files.
func New(dir string, maxSize int64, maxFiles int) (*Dumper, error) {
	w, err := NewWriter(dir, maxSize, maxFiles)
	if err != nil {
		return nil, err
	}
	return &Dumper{w: w, MaxResponseBody: DefaultMaxResponseBody}, nil
}

// Write appends rec as one JSON line.
func (d *Dumper) Write(rec *Record) error {
	line, err := json.Marshal(rec)
	if err != nil {
		return err
	}
	return d.w.WriteLine(line)
}

// Close closes the underlying writer.
func (d *Dumper) Close() error { return d.w.Close() }

// sensitiveHeaders have their values masked in dumps.
var sensitiveHeaders = map[string]bool{
	"Authorization":       true,
	"Proxy-Authorization": true,
	"X-Api-Key":           true,
	"Cookie":              true,
	"Set-Cookie":          true,
}

// RedactHeaders returns a copy of h with credential values masked, keeping the
// auth scheme and last 4 characters so different tokens remain distinguishable.
func RedactHeaders(h http.Header) http.Header {
	out := h.Clone()
	for k, vv := range out {
		if !sensitiveHeaders[http.CanonicalHeaderKey(k)] {
			continue
		}
		for i, v := range vv {
			vv[i] = mask(v)
		}
	}
	return out
}

func mask(v string) string {
	prefix := ""
	if scheme, rest, ok := strings.Cut(v, " "); ok {
		prefix, v = scheme+" ", rest
	}
	if len(v) <= 8 {
		return prefix + "***"
	}
	return prefix + "***" + v[len(v)-4:]
}

// Body renders b for a Record: inline JSON when valid, otherwise a string.
func Body(b []byte) any {
	if len(b) == 0 {
		return nil
	}
	if json.Valid(b) {
		return json.RawMessage(b)
	}
	return string(b)
}
