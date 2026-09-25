package auth

import (
	"crypto/subtle"
	"encoding/json"
	"net/http"
	"net/url"

	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"
)

type contextKey string

const UserEmailKey contextKey = "user_email"

func RequirePathToken(store *TokenStore, log *zap.Logger) func(http.Handler) http.Handler {
	if log == nil {
		log = zap.NewNop()
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token := chi.URLParam(r, "token")
			if token == "" {
				writeAuthError(w, http.StatusUnauthorized, "missing auth token in URL")
				return
			}
			sess, ok := store.Validate(token)
			if !ok {
				log.Debug("path token rejected", zap.String("token", token))
				writeAuthError(w, http.StatusUnauthorized, "invalid or expired token")
				return
			}
			ctx := withUserEmail(r.Context(), sess.Email)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// KeyUserEmail is the identity attached to requests authenticated with the
// static config key; it shows up in logs and usage tracking.
const KeyUserEmail = "config-key"

// RequirePathKey validates the {key} URL param against a static secret from
// config using a constant-time comparison.
func RequirePathKey(secret string, log *zap.Logger) func(http.Handler) http.Handler {
	if log == nil {
		log = zap.NewNop()
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := chi.URLParam(r, "key")
			// chi matches on the raw path when it contains escapes, so the
			// param may still be percent-encoded.
			if unescaped, err := url.PathUnescape(key); err == nil {
				key = unescaped
			}
			if secret == "" || subtle.ConstantTimeCompare([]byte(key), []byte(secret)) != 1 {
				log.Debug("path key rejected")
				writeAuthError(w, http.StatusUnauthorized, "invalid key")
				return
			}
			ctx := withUserEmail(r.Context(), KeyUserEmail)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func writeAuthError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"error": map[string]string{
			"type":    "authentication_error",
			"message": msg,
		},
	})
}
