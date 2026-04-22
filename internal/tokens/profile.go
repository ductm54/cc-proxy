package tokens

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

const oauthProfileURL = "https://api.anthropic.com/api/oauth/profile"

type profileResponse struct {
	Account struct {
		UUID string `json:"uuid"`
	} `json:"account"`
}

// FetchProfile calls GET /api/oauth/profile with the given access token and
// returns the account UUID. profileURL is injectable for tests (use "" for
// the production default).
func FetchProfile(ctx context.Context, client *http.Client, profileURL, accessToken string) (string, error) {
	if profileURL == "" {
		profileURL = oauthProfileURL
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, profileURL, nil)
	if err != nil {
		return "", fmt.Errorf("build profile request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "axios/1.13.6")

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("profile request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("profile: upstream returned %d", resp.StatusCode)
	}

	var pr profileResponse
	if err := json.NewDecoder(resp.Body).Decode(&pr); err != nil {
		return "", fmt.Errorf("decode profile response: %w", err)
	}
	if pr.Account.UUID == "" {
		return "", fmt.Errorf("profile response missing account.uuid")
	}
	return pr.Account.UUID, nil
}
