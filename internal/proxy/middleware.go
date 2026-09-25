package proxy

import (
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5/middleware"
	"go.uber.org/zap"

	"github.com/ductm54/cc-proxy/internal/auth"
)

func zapLogger(log *zap.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
			start := time.Now()
			next.ServeHTTP(ww, r)
			fields := []zap.Field{
				zap.String("method", r.Method),
				zap.String("path", redactPath(r.URL.Path)),
				zap.Int("status", ww.Status()),
				zap.Int64("dur_ms", time.Since(start).Milliseconds()),
				zap.String("req_id", middleware.GetReqID(r.Context())),
			}
			if email := auth.GetUserEmail(r.Context()); email != "" {
				fields = append(fields, zap.String("user", email))
			}
			log.Info("request", fields...)
		})
	}
}

// redactPath hides the static key in /k/{key}/... paths so it never lands in logs.
func redactPath(p string) string {
	rest, ok := strings.CutPrefix(p, "/k/")
	if !ok {
		return p
	}
	if i := strings.IndexByte(rest, '/'); i >= 0 {
		return "/k/REDACTED" + rest[i:]
	}
	return "/k/REDACTED"
}
