package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

// Config holds resolved runtime configuration.
type Config struct {
	Addr        string
	TokensFile  string
	RefreshSkew time.Duration
	LogDev      bool
}

// AuthConfig holds OAuth / auth-token configuration.
type AuthConfig struct {
	OAuthClientID     string `json:"oauth_client_id"`
	OAuthClientSecret string `json:"oauth_client_secret"`
	OAuthDomain       string `json:"oauth_domain"`
	AuthTokenTTL      string `json:"auth_token_ttl"`
	ExternalURL       string `json:"external_url"`

	// Resolved TTL (not serialised).
	TokenTTL time.Duration `json:"-"`
}

// Enabled returns true when OAuth authentication is configured.
func (a *AuthConfig) Enabled() bool {
	return a != nil && a.OAuthClientID != ""
}

// RedirectURL returns the OAuth2 callback URL.
func (a *AuthConfig) RedirectURL() string {
	return a.ExternalURL + "/auth/callback"
}

// LoadAuthConfig reads an AuthConfig JSON file. Returns a zero-value
// AuthConfig (not an error) when the file does not exist.
func LoadAuthConfig(path string) (*AuthConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &AuthConfig{}, nil
		}
		return nil, err
	}
	var cfg AuthConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

// DefaultTokensFile returns the default path for the tokens file.
func DefaultTokensFile() string {
	return filepath.Join(configDir(), ".credentials.json")
}

// DefaultAuthConfigFile returns the default path for the auth config file.
func DefaultAuthConfigFile() string {
	return filepath.Join(configDir(), "auth.json")
}

func configDir() string {
	return ".config"
}
