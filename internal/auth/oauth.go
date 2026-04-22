package auth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"go.uber.org/zap"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

type OAuthHandler struct {
	oauthCfg    *oauth2.Config
	store       *TokenStore
	ttl         time.Duration
	allowDomain string
	externalURL string
	log         *zap.Logger
}

func NewOAuthHandler(clientID, clientSecret, redirectURL, allowDomain, externalURL string, store *TokenStore, ttl time.Duration, log *zap.Logger) *OAuthHandler {
	return &OAuthHandler{
		oauthCfg: &oauth2.Config{
			ClientID:     clientID,
			ClientSecret: clientSecret,
			RedirectURL:  redirectURL,
			Scopes:       []string{"openid", "email"},
			Endpoint:     google.Endpoint,
		},
		store:       store,
		ttl:         ttl,
		allowDomain: allowDomain,
		externalURL: externalURL,
		log:         log,
	}
}

func (h *OAuthHandler) HandleAuthInfo(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{
		"domain": h.allowDomain,
	})
}

func (h *OAuthHandler) HandleStart(w http.ResponseWriter, r *http.Request) {
	state, err := randomHex(16)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"error": map[string]string{"type": "internal_error", "message": "failed to generate state"},
		})
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     "oauth_state",
		Value:    state,
		Path:     "/",
		MaxAge:   600,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
	http.Redirect(w, r, h.oauthCfg.AuthCodeURL(state), http.StatusFound)
}

func (h *OAuthHandler) HandleCallback(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("oauth_state")
	if err != nil || cookie.Value == "" || cookie.Value != r.URL.Query().Get("state") {
		h.redirectError(w, r, http.StatusBadRequest, "State mismatch — please try logging in again.")
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:   "oauth_state",
		Path:   "/",
		MaxAge: -1,
	})

	tok, err := h.oauthCfg.Exchange(r.Context(), r.URL.Query().Get("code"))
	if err != nil {
		h.log.Error("oauth exchange failed", zap.Error(err))
		h.redirectError(w, r, http.StatusBadGateway, "OAuth token exchange failed. Please try again.")
		return
	}

	email, err := h.fetchEmail(r.Context(), tok)
	if err != nil {
		h.log.Error("failed to fetch user email", zap.Error(err))
		h.redirectError(w, r, http.StatusBadGateway, "Failed to fetch your email from Google. Please try again.")
		return
	}

	if h.allowDomain != "" {
		parts := strings.SplitN(email, "@", 2)
		if len(parts) != 2 || parts[1] != h.allowDomain {
			h.log.Warn("domain rejected", zap.String("email", email), zap.String("allowed", h.allowDomain))
			h.redirectError(w, r, http.StatusForbidden,
				fmt.Sprintf("Access denied. Your email domain is not allowed — must be @%s.", h.allowDomain))
			return
		}
	}

	shortToken, sess, err := h.store.Create(email, h.ttl)
	if err != nil {
		h.redirectError(w, r, http.StatusInternalServerError, "Failed to issue auth token.")
		return
	}

	h.log.Info("auth token issued", zap.String("email", email), zap.String("token", shortToken), zap.Time("expires", sess.ExpiresAt))

	proxyURL := strings.TrimRight(h.externalURL, "/") + "/p/" + shortToken

	q := url.Values{}
	q.Set("email", email)
	q.Set("token", shortToken)
	q.Set("proxy_url", proxyURL)
	q.Set("expires_at", sess.ExpiresAt.Format(time.RFC3339))
	http.Redirect(w, r, "/auth/success?"+q.Encode(), http.StatusFound)
}

func (h *OAuthHandler) redirectError(w http.ResponseWriter, r *http.Request, status int, msg string) {
	q := url.Values{}
	q.Set("status", fmt.Sprintf("%d", status))
	q.Set("message", msg)
	http.Redirect(w, r, "/auth/error?"+q.Encode(), http.StatusFound)
}

func (h *OAuthHandler) fetchEmail(ctx context.Context, tok *oauth2.Token) (string, error) {
	client := h.oauthCfg.Client(ctx, tok)
	resp, err := client.Get("https://www.googleapis.com/oauth2/v2/userinfo")
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	var info struct {
		Email string `json:"email"`
	}
	if err := json.Unmarshal(body, &info); err != nil {
		return "", err
	}
	if info.Email == "" {
		return "", fmt.Errorf("no email in userinfo response")
	}
	return info.Email, nil
}

func randomHex(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
