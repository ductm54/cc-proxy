package proxy

import (
	"bytes"
	"io"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5/middleware"
	"go.uber.org/zap"

	"github.com/ductm54/cc-proxy/internal/auth"
	"github.com/ductm54/cc-proxy/internal/debugdump"
)

// dumpExchange accumulates one debug dump record. All methods are no-ops on a
// nil receiver, so handlers can call them unconditionally.
type dumpExchange struct {
	s          *Server
	start      time.Time
	rec        debugdump.Record
	clientBody []byte
	capture    *captureBody
}

// startDump begins a record for r, or returns nil when dumping is disabled.
func (s *Server) startDump(r *http.Request, clientBody []byte) *dumpExchange {
	if s.dump == nil {
		return nil
	}
	url := redactPath(r.URL.Path)
	if r.URL.RawQuery != "" {
		url += "?" + r.URL.RawQuery
	}
	return &dumpExchange{
		s:          s,
		start:      time.Now(),
		clientBody: clientBody,
		rec: debugdump.Record{
			Time:  time.Now().UTC(),
			ReqID: middleware.GetReqID(r.Context()),
			User:  auth.GetUserEmail(r.Context()),
			Client: debugdump.Request{
				Method:    r.Method,
				URL:       url,
				Headers:   debugdump.RedactHeaders(r.Header),
				BodyBytes: len(clientBody),
				Body:      debugdump.Body(clientBody),
			},
		},
	}
}

// upstream records the outbound request exactly as it will be sent.
func (d *dumpExchange) upstream(up *http.Request, body []byte) {
	if d == nil {
		return
	}
	req := &debugdump.Request{
		Method:    up.Method,
		URL:       up.URL.String(),
		Headers:   debugdump.RedactHeaders(up.Header),
		BodyBytes: len(body),
	}
	if bytes.Equal(body, d.clientBody) {
		req.BodySameAsClient = len(body) > 0
	} else {
		req.Body = debugdump.Body(body)
	}
	d.rec.Upstream = req
}

// response wraps resp.Body so relayed bytes are captured (up to the cap).
func (d *dumpExchange) response(resp *http.Response) {
	if d == nil {
		return
	}
	d.rec.Response = &debugdump.Response{
		Status:  resp.StatusCode,
		Headers: debugdump.RedactHeaders(resp.Header),
	}
	d.capture = &captureBody{ReadCloser: resp.Body, limit: d.s.dump.MaxResponseBody}
	resp.Body = d.capture
}

// fail records a transport-level error.
func (d *dumpExchange) fail(err error) {
	if d == nil {
		return
	}
	d.rec.Error = err.Error()
}

// finish writes the record. Call after the response body has been relayed.
func (d *dumpExchange) finish() {
	if d == nil {
		return
	}
	d.rec.DurMS = time.Since(d.start).Milliseconds()
	if c, resp := d.capture, d.rec.Response; c != nil && resp != nil {
		resp.BodyBytes = c.total
		resp.Truncated = c.truncated
		// Upstream bodies are only readable when Go's transport decompressed
		// them; if the client negotiated an encoding, the bytes are opaque.
		if enc := resp.Headers.Get("Content-Encoding"); enc != "" && enc != "identity" {
			resp.BodyNotice = "body omitted: content-encoding " + enc
		} else if resp.Truncated {
			resp.Body = c.buf.String()
		} else {
			resp.Body = debugdump.Body(c.buf.Bytes())
		}
	}
	if err := d.s.dump.Write(&d.rec); err != nil {
		d.s.log.Warn("debug dump write failed", zap.Error(err), zap.String("req_id", d.rec.ReqID))
	}
}

// captureBody tees up to limit bytes of a body into buf while counting all bytes.
type captureBody struct {
	io.ReadCloser
	buf       bytes.Buffer
	limit     int
	total     int64
	truncated bool
}

func (c *captureBody) Read(p []byte) (int, error) {
	n, err := c.ReadCloser.Read(p)
	if n > 0 {
		c.total += int64(n)
		if room := c.limit - c.buf.Len(); room > 0 {
			if n > room {
				c.buf.Write(p[:room])
				c.truncated = true
			} else {
				c.buf.Write(p[:n])
			}
		} else {
			c.truncated = true
		}
	}
	return n, err
}
