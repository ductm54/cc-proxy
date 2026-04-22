package proxy

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"go.uber.org/zap"

	"github.com/ductm54/cc-proxy/internal/auth"
	"github.com/ductm54/cc-proxy/internal/usage"
)

// hopByHop are headers that must not be forwarded.
var hopByHop = map[string]bool{
	"Connection":          true,
	"Keep-Alive":          true,
	"Transfer-Encoding":   true,
	"Te":                  true,
	"Trailers":            true,
	"Upgrade":             true,
	"Proxy-Connection":    true,
	"Proxy-Authenticate":  true,
	"Proxy-Authorization": true,
}

// overriddenByProxy are request headers that the proxy sets itself and
// should not be taken from the incoming client request. Everything else
// — including Anthropic-Version, X-App, User-Agent, and the whole
// X-Stainless-* family — is passed through from Claude Code verbatim.
var overriddenByProxy = map[string]bool{
	http.CanonicalHeaderKey(HeaderAuthorization): true,
	http.CanonicalHeaderKey(HeaderAnthropicBeta): true,
	http.CanonicalHeaderKey(HeaderXApiKey):       true,
}

// shouldForward returns true for headers that the proxy passes through
// from the client to upstream.
func shouldForward(key string) bool {
	canon := http.CanonicalHeaderKey(key)
	if hopByHop[canon] || overriddenByProxy[canon] {
		return false
	}
	// Allow Content-Type, Accept, Content-Length, X-Stainless-*, and anything else
	// that hasn't been explicitly excluded.
	return true
}

// handleMessages forwards a POST /v1/messages request to upstream.
func (s *Server) handleMessages(w http.ResponseWriter, r *http.Request) {
	tok, refreshErr := s.tokens.Current()
	if refreshErr != nil && tok.AccessToken == "" {
		writeErrJSON(w, http.StatusServiceUnavailable, "cc_proxy_auth",
			"upstream auth refresh failed — run cc-proxy bootstrap again")
		return
	}

	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		writeErrJSON(w, http.StatusBadGateway, "cc_proxy_internal", err.Error())
		return
	}

	var reqBody struct {
		Model string `json:"model"`
	}
	_ = json.Unmarshal(bodyBytes, &reqBody)

	if tok.AccountUUID != "" {
		rewritten, rerr := rewriteAccountUUID(bodyBytes, tok.AccountUUID)
		if rerr != nil {
			s.log.Warn("account_uuid rewrite skipped", zap.Error(rerr))
		} else {
			bodyBytes = rewritten
		}
	} else {
		s.log.Warn("no cached account_uuid — forwarding body unchanged")
	}

	up, err := http.NewRequestWithContext(r.Context(), http.MethodPost, s.messagesURL, bytes.NewReader(bodyBytes))
	if err != nil {
		writeErrJSON(w, http.StatusBadGateway, "cc_proxy_internal", err.Error())
		return
	}
	up.ContentLength = int64(len(bodyBytes))

	for k, v := range r.Header {
		if shouldForward(k) {
			up.Header[k] = v
		}
	}
	up.Header.Set(HeaderAuthorization, "Bearer "+tok.AccessToken)
	up.Header.Set(HeaderAnthropicBeta, SubscriptionBetaList)
	up.Header.Set(HeaderContentLength, strconv.Itoa(len(bodyBytes)))
	up.Header.Del(HeaderXApiKey)

	resp, err := s.http.Do(up)
	if err != nil {
		writeErrJSON(w, http.StatusBadGateway, "cc_proxy_upstream", err.Error())
		return
	}
	defer resp.Body.Close()

	copyRespHeaders(w, resp)
	w.WriteHeader(resp.StatusCode)

	email := auth.GetUserEmail(r.Context())
	isSSE := strings.Contains(resp.Header.Get(HeaderContentType), "text/event-stream")

	if s.usage != nil && email != "" && resp.StatusCode == http.StatusOK {
		if isSSE {
			tu := streamCopyWithUsage(w, resp.Body)
			go s.recordUsage(email, reqBody.Model, tu)
		} else {
			tu := copyWithUsage(w, resp.Body)
			go s.recordUsage(email, reqBody.Model, tu)
		}
	} else {
		streamCopy(w, resp.Body)
	}
}

func (s *Server) recordUsage(email, model string, tu usage.TokenUsage) {
	if tu.InputTokens == 0 && tu.OutputTokens == 0 {
		return
	}
	if err := s.usage.Record(context.Background(), email, model, tu); err != nil {
		s.log.Error("failed to record usage", zap.Error(err), zap.String("email", email))
	}
}

type sseUsage struct {
	InputTokens              int `json:"input_tokens"`
	OutputTokens             int `json:"output_tokens"`
	CacheCreationInputTokens int `json:"cache_creation_input_tokens"`
	CacheReadInputTokens     int `json:"cache_read_input_tokens"`
}

func streamCopyWithUsage(w http.ResponseWriter, body io.Reader) usage.TokenUsage {
	var buf bytes.Buffer
	tee := io.TeeReader(body, &buf)

	streamCopy(w, tee)

	return parseSSEUsage(&buf)
}

func parseSSEUsage(r io.Reader) usage.TokenUsage {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	var tu usage.TokenUsage
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		var event struct {
			Type    string `json:"type"`
			Message struct {
				Usage sseUsage `json:"usage"`
			} `json:"message"`
			Usage sseUsage `json:"usage"`
		}
		if json.Unmarshal([]byte(line[6:]), &event) != nil {
			continue
		}
		switch event.Type {
		case "message_start":
			tu.InputTokens = event.Message.Usage.InputTokens
			tu.CacheCreationTokens = event.Message.Usage.CacheCreationInputTokens
			tu.CacheReadTokens = event.Message.Usage.CacheReadInputTokens
		case "message_delta":
			tu.OutputTokens = event.Usage.OutputTokens
		}
	}
	return tu
}

func copyWithUsage(w http.ResponseWriter, body io.Reader) usage.TokenUsage {
	data, err := io.ReadAll(body)
	if err != nil {
		return usage.TokenUsage{}
	}
	_, _ = w.Write(data)

	var resp struct {
		Usage sseUsage `json:"usage"`
	}
	if json.Unmarshal(data, &resp) == nil {
		return usage.TokenUsage{
			InputTokens:         resp.Usage.InputTokens,
			OutputTokens:        resp.Usage.OutputTokens,
			CacheCreationTokens: resp.Usage.CacheCreationInputTokens,
			CacheReadTokens:     resp.Usage.CacheReadInputTokens,
		}
	}
	return usage.TokenUsage{}
}

// handleModels forwards a GET /v1/models request to upstream.
func (s *Server) handleModels(w http.ResponseWriter, r *http.Request) {
	tok, refreshErr := s.tokens.Current()
	if refreshErr != nil && tok.AccessToken == "" {
		writeErrJSON(w, http.StatusServiceUnavailable, "cc_proxy_auth",
			"upstream auth refresh failed — run cc-proxy bootstrap again")
		return
	}

	up, err := http.NewRequestWithContext(r.Context(), http.MethodGet, s.modelsURL, nil)
	if err != nil {
		writeErrJSON(w, http.StatusBadGateway, "cc_proxy_internal", err.Error())
		return
	}
	for k, v := range r.Header {
		if shouldForward(k) {
			up.Header[k] = v
		}
	}
	up.Header.Set(HeaderAuthorization, "Bearer "+tok.AccessToken)
	up.Header.Del(HeaderXApiKey)

	resp, err := s.http.Do(up)
	if err != nil {
		writeErrJSON(w, http.StatusBadGateway, "cc_proxy_upstream", err.Error())
		return
	}
	defer resp.Body.Close()

	copyRespHeaders(w, resp)
	w.WriteHeader(resp.StatusCode)
	_, _ = io.Copy(w, resp.Body)
}

// handleHealthz returns proxy health and token state.
func (s *Server) handleHealthz(w http.ResponseWriter, r *http.Request) {
	tok, _ := s.tokens.Current()
	expiresIn := time.Until(tok.ExpiresAt).Seconds()
	w.Header().Set(HeaderContentType, "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"ok":                 true,
		"expires_in_seconds": expiresIn,
	})
}

// handleCatchAll forwards any unmatched request to upstream with auth rewriting.
func (s *Server) handleCatchAll(w http.ResponseWriter, r *http.Request) {
	tok, refreshErr := s.tokens.Current()
	if refreshErr != nil && tok.AccessToken == "" {
		writeErrJSON(w, http.StatusServiceUnavailable, "cc_proxy_auth",
			"upstream auth refresh failed — run cc-proxy bootstrap again")
		return
	}

	targetURL := s.upstreamBase + r.URL.RequestURI()

	var body io.Reader
	if r.Body != nil {
		body = r.Body
	}

	up, err := http.NewRequestWithContext(r.Context(), r.Method, targetURL, body)
	if err != nil {
		writeErrJSON(w, http.StatusBadGateway, "cc_proxy_internal", err.Error())
		return
	}

	for k, v := range r.Header {
		if shouldForward(k) {
			up.Header[k] = v
		}
	}
	up.Header.Set(HeaderAuthorization, "Bearer "+tok.AccessToken)
	up.Header.Del(HeaderXApiKey)
	if r.ContentLength > 0 {
		up.ContentLength = r.ContentLength
	}

	s.log.Info("catch-all forward", zap.String("method", r.Method), zap.String("url", targetURL))

	resp, err := s.http.Do(up)
	if err != nil {
		writeErrJSON(w, http.StatusBadGateway, "cc_proxy_upstream", err.Error())
		return
	}
	defer resp.Body.Close()

	copyRespHeaders(w, resp)
	w.WriteHeader(resp.StatusCode)
	_, _ = io.Copy(w, resp.Body)
}

func copyRespHeaders(w http.ResponseWriter, resp *http.Response) {
	for k, vv := range resp.Header {
		if hopByHop[http.CanonicalHeaderKey(k)] {
			continue
		}
		for _, v := range vv {
			w.Header().Add(k, v)
		}
	}
}

func streamCopy(w http.ResponseWriter, body io.Reader) {
	flusher, _ := w.(http.Flusher)
	buf := make([]byte, 4096)
	for {
		n, err := body.Read(buf)
		if n > 0 {
			_, _ = w.Write(buf[:n])
			if flusher != nil {
				flusher.Flush()
			}
		}
		if err != nil {
			return
		}
	}
}

func writeErrJSON(w http.ResponseWriter, status int, errType, msg string) {
	w.Header().Set(HeaderContentType, "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"error": map[string]string{
			"type":    errType,
			"message": msg,
		},
	})
}
