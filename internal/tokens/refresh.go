package tokens

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

const (
	oauthClientID = "9d1c250a-e61b-44d9-88ed-5944d1962f5e"
	oauthTokenURL = "https://platform.claude.com/v1/oauth/token"
)

type refreshRequest struct {
	ClientID     string `json:"client_id"`
	GrantType    string `json:"grant_type"`
	RefreshToken string `json:"refresh_token"`
	Scope        string `json:"scope"`
}

type tokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int64  `json:"expires_in"` // seconds
}

// Refresh exchanges refreshToken for a new Token using the platform.claude.com
// OAuth endpoint. The tokenURL parameter is injectable for testing.
func Refresh(ctx context.Context, client *http.Client, tokenURL, refreshToken string) (Token, error) {
	body, err := json.Marshal(refreshRequest{
		ClientID:     oauthClientID,
		GrantType:    "refresh_token",
		RefreshToken: refreshToken,
		Scope:        "user:profile user:inference user:sessions:claude_code user:mcp_servers user:file_upload",
	})
	if err != nil {
		return Token{}, fmt.Errorf("marshal refresh request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL, bytes.NewReader(body))
	if err != nil {
		return Token{}, fmt.Errorf("build refresh request: %w", err)
	}
	req.Header.Set("Accept", "application/json, text/plain, */*")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "axios/1.13.6")

	resp, err := client.Do(req)
	if err != nil {
		return Token{}, fmt.Errorf("refresh request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return Token{}, fmt.Errorf("refresh: upstream returned %d", resp.StatusCode)
	}

	var tr tokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tr); err != nil {
		return Token{}, fmt.Errorf("decode refresh response: %w", err)
	}
	if tr.AccessToken == "" {
		return Token{}, fmt.Errorf("refresh response missing access_token")
	}

	refreshed := Token{
		AccessToken: tr.AccessToken,
		ExpiresAt:   time.Now().Add(time.Duration(tr.ExpiresIn) * time.Second),
	}
	// Refresh token may rotate; keep old one if none returned.
	if tr.RefreshToken != "" {
		refreshed.RefreshToken = tr.RefreshToken
	} else {
		refreshed.RefreshToken = refreshToken
	}
	return refreshed, nil
}
