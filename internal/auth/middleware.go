package auth

import (
	"encoding/json"
	"net/http"

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
