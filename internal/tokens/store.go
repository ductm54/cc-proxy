package tokens

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Token is the in-memory OAuth token state.
// AccountUUID is proxy-specific and kept in memory only.
type Token struct {
	AccessToken  string
	RefreshToken string
	ExpiresAt    time.Time
	AccountUUID  string
}

// credentialsFile is the Claude Code .credentials.json format — used for
// both reading and writing so the proxy and Claude Code stay compatible.
type credentialsFile struct {
	ClaudeAiOauth credentialsOauth `json:"claudeAiOauth"`
}

type credentialsOauth struct {
	AccessToken  string   `json:"accessToken"`
	RefreshToken string   `json:"refreshToken"`
	ExpiresAt    int64    `json:"expiresAt"`
	Scopes       []string `json:"scopes"`
}

// Load reads a Token from path in Claude Code .credentials.json format.
func Load(path string) (Token, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return Token{}, fmt.Errorf("tokens file not found at %s (run `cc-proxy bootstrap` first): %w", path, err)
	}

	var cf credentialsFile
	if err := json.Unmarshal(b, &cf); err != nil {
		return Token{}, fmt.Errorf("tokens file %s is corrupted: %w", path, err)
	}
	t := Token{
		AccessToken:  cf.ClaudeAiOauth.AccessToken,
		RefreshToken: cf.ClaudeAiOauth.RefreshToken,
		ExpiresAt:    time.UnixMilli(cf.ClaudeAiOauth.ExpiresAt),
	}
	if t.AccessToken == "" || t.RefreshToken == "" {
		return Token{}, fmt.Errorf("tokens file %s is missing accessToken or refreshToken", path)
	}
	return t, nil
}

// Save atomically writes t to path in Claude Code .credentials.json format.
func Save(path string, t Token) error {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return fmt.Errorf("create tokens dir: %w", err)
	}

	// Read existing file to preserve scopes and any other fields.
	var cf credentialsFile
	if existing, err := os.ReadFile(path); err == nil {
		_ = json.Unmarshal(existing, &cf)
	}

	cf.ClaudeAiOauth.AccessToken = t.AccessToken
	cf.ClaudeAiOauth.RefreshToken = t.RefreshToken
	cf.ClaudeAiOauth.ExpiresAt = t.ExpiresAt.UnixMilli()

	b, err := json.MarshalIndent(cf, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0600); err != nil {
		return fmt.Errorf("write tmp tokens: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("rename tokens: %w", err)
	}
	return nil
}
