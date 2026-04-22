package tokens

import (
	"context"
	"net/http"
	"sync"
	"time"

	"go.uber.org/zap"
)

// Manager holds the current token in memory and runs a background goroutine
// that proactively refreshes it before expiry.
type Manager struct {
	mu          sync.RWMutex
	current     Token
	tokensPath  string
	refreshSkew time.Duration
	tokenURL    string
	profileURL  string
	client      *http.Client
	log         *zap.Logger
	refreshErr  error // last refresh error, guarded by mu
}

// NewManager creates a Manager loaded from tokensPath.
// tokenURL is injectable (use "" for production default).
func NewManager(tokensPath string, refreshSkew time.Duration, tokenURL string, log *zap.Logger) (*Manager, error) {
	tok, err := Load(tokensPath)
	if err != nil {
		return nil, err
	}
	if tokenURL == "" {
		tokenURL = oauthTokenURL
	}
	return &Manager{
		current:     tok,
		tokensPath:  tokensPath,
		refreshSkew: refreshSkew,
		tokenURL:    tokenURL,
		client:      &http.Client{Timeout: 30 * time.Second},
		log:         log,
	}, nil
}

// SetProfileURL overrides the OAuth profile endpoint (for tests).
func (m *Manager) SetProfileURL(u string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.profileURL = u
}

// Current returns the in-memory token. Returns the token and any stored
// refresh error so callers can decide whether to serve a 503.
func (m *Manager) Current() (Token, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.current, m.refreshErr
}

// Start launches the background refresh goroutine. It runs until ctx is
// cancelled. Call in a goroutine or with go m.Start(ctx).
func (m *Manager) Start(ctx context.Context) {
	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()
	// Run an immediate check so we refresh eagerly if already near expiry.
	m.maybeRefresh(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.maybeRefresh(ctx)
		}
	}
}

func (m *Manager) maybeRefresh(ctx context.Context) {
	m.mu.RLock()
	tok := m.current
	m.mu.RUnlock()

	if time.Until(tok.ExpiresAt) > m.refreshSkew {
		return
	}

	m.log.Info("refreshing token", zap.Time("expires_at", tok.ExpiresAt))

	backoff := 5 * time.Second
	const maxBackoff = 5 * time.Minute
	for {
		newTok, err := Refresh(ctx, m.client, m.tokenURL, tok.RefreshToken)
		if err == nil {
			// Carry forward the account UUID if we already have one. If we
			// don't, try to fetch it now — best-effort, don't fail refresh.
			newTok.AccountUUID = tok.AccountUUID
			if newTok.AccountUUID == "" {
				m.mu.RLock()
				profileURL := m.profileURL
				m.mu.RUnlock()
				if uuid, ferr := FetchProfile(ctx, m.client, profileURL, newTok.AccessToken); ferr == nil {
					newTok.AccountUUID = uuid
				} else {
					m.log.Warn("fetch account profile failed", zap.Error(ferr))
				}
			}
			if saveErr := Save(m.tokensPath, newTok); saveErr != nil {
				m.log.Warn("failed to persist refreshed token", zap.Error(saveErr))
			}
			m.mu.Lock()
			m.current = newTok
			m.refreshErr = nil
			m.mu.Unlock()
			m.log.Info("token refreshed", zap.Time("new_expires_at", newTok.ExpiresAt))
			return
		}
		m.log.Error("token refresh failed", zap.Error(err), zap.Duration("retry_in", backoff))
		m.mu.Lock()
		m.refreshErr = err
		m.mu.Unlock()

		select {
		case <-ctx.Done():
			return
		case <-time.After(backoff):
		}
		backoff *= 2
		if backoff > maxBackoff {
			backoff = maxBackoff
		}
		// Re-read current token (may have been updated by another call)
		m.mu.RLock()
		tok = m.current
		m.mu.RUnlock()
	}
}
