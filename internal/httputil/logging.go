package httputil

import (
	"bytes"
	"compress/flate"
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httputil"
	"testing"
	"time"

	"github.com/andybalholm/brotli"
)

type loggingTransport struct {
	t         *testing.T
	transport http.RoundTripper
}

func (lt *loggingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	reqDump, _ := httputil.DumpRequestOut(req, true)
	lt.t.Logf(">>> REQUEST:\n%s", reqDump)

	resp, err := lt.transport.RoundTrip(req)
	if err != nil {
		lt.t.Logf("<<< ERROR: %v", err)
		return nil, err
	}

	rawBody, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	resp.Body = io.NopCloser(bytes.NewReader(rawBody))

	var reader io.Reader
	switch resp.Header.Get("Content-Encoding") {
	case "gzip":
		reader, _ = gzip.NewReader(bytes.NewReader(rawBody))
	case "br":
		reader = brotli.NewReader(bytes.NewReader(rawBody))
	case "deflate":
		reader = flate.NewReader(bytes.NewReader(rawBody))
	default:
		reader = bytes.NewReader(rawBody)
	}
	decoded, _ := io.ReadAll(reader)

	lt.t.Logf("<<< RESPONSE: %s (Content-Encoding: %s)\n%s", resp.Status, resp.Header.Get("Content-Encoding"), decoded)
	return resp, nil
}

// NewLoggingClient returns an *http.Client that logs full request/response
// details (including decompressed bodies) via t.Logf.
func NewLoggingClient(t *testing.T) *http.Client {
	return &http.Client{
		Timeout: 30 * time.Second,
		Transport: &loggingTransport{
			t:         t,
			transport: http.DefaultTransport,
		},
	}
}
