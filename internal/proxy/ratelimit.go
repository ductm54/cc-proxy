package proxy

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)


type QuotaWindow struct {
	Utilization float64    `json:"utilization"`
	ResetsAt    *time.Time `json:"resets_at"`
}

type ExtraUsage struct {
	Currency   string  `json:"currency"`
	IsEnabled  bool    `json:"is_enabled"`
	MonthlyLmt float64 `json:"monthly_limit"`
	UsedCredit float64 `json:"used_credits"`
	Util       float64 `json:"utilization"`
}

type AccountUsage struct {
	FiveHour   QuotaWindow `json:"five_hour"`
	SevenDay   QuotaWindow `json:"seven_day"`
	ExtraUsage *ExtraUsage `json:"extra_usage"`
}

// FetchAccountUsage calls the Anthropic usage endpoint and returns the parsed
// response. The usageURL parameter is injectable for testing (pass "" for default).
func FetchAccountUsage(ctx context.Context, client *http.Client, usageURL, accessToken string) (AccountUsage, error) {
	if usageURL == "" {
		usageURL = AccountUsageURL
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, usageURL, nil)
	if err != nil {
		return AccountUsage{}, fmt.Errorf("build usage request: %w", err)
	}
	req.Header.Set(HeaderAuthorization, "Bearer "+accessToken)
	req.Header.Set(HeaderAccept, AcceptJSON)
	req.Header.Set(HeaderUserAgent, FakeUserAgent)
	req.Header.Set(HeaderAnthropicBeta, UsageBetaValue)

	resp, err := client.Do(req)
	if err != nil {
		return AccountUsage{}, fmt.Errorf("usage request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return AccountUsage{}, fmt.Errorf("read usage response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return AccountUsage{}, fmt.Errorf("upstream returned %d: %s", resp.StatusCode, string(body))
	}

	var usage AccountUsage
	if err := json.Unmarshal(body, &usage); err != nil {
		return AccountUsage{}, fmt.Errorf("parse usage response: %w", err)
	}
	return usage, nil
}

func (srv *Server) handleAccountInfo(w http.ResponseWriter, r *http.Request) {
	tok, refreshErr := srv.tokens.Current()
	if refreshErr != nil && tok.AccessToken == "" {
		writeErrJSON(w, http.StatusServiceUnavailable, "cc_proxy_auth",
			"upstream auth refresh failed")
		return
	}

	usage, err := FetchAccountUsage(r.Context(), srv.http, "", tok.AccessToken)
	if err != nil {
		writeErrJSON(w, http.StatusBadGateway, "cc_proxy_upstream", err.Error())
		return
	}

	w.Header().Set(HeaderContentType, "application/json")
	_ = json.NewEncoder(w).Encode(usage)
}
