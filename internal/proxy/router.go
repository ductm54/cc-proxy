package proxy

import (
	"context"
	"encoding/json"
	"io/fs"
	"net"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"go.uber.org/zap"

	"github.com/ductm54/cc-proxy/internal/auth"
	"github.com/ductm54/cc-proxy/internal/config"
	"github.com/ductm54/cc-proxy/internal/tokens"
	"github.com/ductm54/cc-proxy/internal/usage"
)

// TokenProvider is the interface Manager implements.
type TokenProvider interface {
	Current() (tokens.Token, error)
}

// Server is the HTTP proxy server.
type Server struct {
	tokens       TokenProvider
	http         *http.Client
	log          *zap.Logger
	usage        *usage.Store
	messagesURL  string
	modelsURL    string
	upstreamBase string
}

// Options configures a new Server.
type Options struct {
	// MessagesURL overrides the upstream messages endpoint (for tests).
	MessagesURL string
	// ModelsURL overrides the upstream models endpoint (for tests).
	ModelsURL string
	// UpstreamBase overrides the upstream base URL (for tests).
	UpstreamBase string
	// AuthConfig enables OAuth authentication when non-nil and Enabled().
	AuthConfig *config.AuthConfig
	// WebFS serves the frontend SPA when non-nil.
	WebFS fs.FS
	// UsageStore enables per-user usage tracking when non-nil.
	UsageStore *usage.Store
}

// New creates a Server and returns its chi.Router.
func New(tp TokenProvider, log *zap.Logger, opts Options) (http.Handler, *Server) {
	messagesURL := UpstreamMessagesURL
	if opts.MessagesURL != "" {
		messagesURL = opts.MessagesURL
	}
	modelsURL := UpstreamModelsURL
	if opts.ModelsURL != "" {
		modelsURL = opts.ModelsURL
	}

	upstreamBase := UpstreamBaseURL
	if opts.UpstreamBase != "" {
		upstreamBase = opts.UpstreamBase
	}

	transport := &http.Transport{
		DialContext: (&net.Dialer{
			Timeout:   10 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		TLSHandshakeTimeout:   10 * time.Second,
		ResponseHeaderTimeout: 120 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
	}

	s := &Server{
		tokens:       tp,
		http:         &http.Client{Transport: transport},
		log:          log,
		usage:        opts.UsageStore,
		messagesURL:  messagesURL,
		modelsURL:    modelsURL,
		upstreamBase: upstreamBase,
	}

	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.Recoverer)
	r.Use(zapLogger(log))

	r.Get("/healthz", s.handleHealthz)

	if opts.AuthConfig.Enabled() {
		var persister auth.SessionPersister
		if s.usage != nil {
			persister = &sessionAdapter{s.usage}
		}
		store := auth.NewTokenStore(persister, log)
		oauthH := auth.NewOAuthHandler(
			opts.AuthConfig.OAuthClientID,
			opts.AuthConfig.OAuthClientSecret,
			opts.AuthConfig.RedirectURL(),
			opts.AuthConfig.OAuthDomain,
			opts.AuthConfig.ExternalURL,
			store,
			opts.AuthConfig.TokenTTL,
			log,
		)

		r.Get("/api/auth/info", oauthH.HandleAuthInfo)
		r.Get("/auth/start", oauthH.HandleStart)
		r.Get("/auth/callback", oauthH.HandleCallback)

		r.Route("/p/{token}", func(r chi.Router) {
			r.Use(auth.RequirePathToken(store, log))
			r.Get("/api/session", handleSession)
			r.Post("/v1/messages", s.handleMessages)
			r.Get("/v1/models", s.handleModels)
			r.HandleFunc("/v1/*", s.handleCatchAll)
			r.Get("/api/account", s.handleAccountInfo)
			if s.usage != nil {
				r.Get("/api/usage", s.handleUsageSummary)
				r.Get("/api/usage/{email}", s.handleUsageByEmail)
			}
		})
	} else {
		r.Post("/v1/messages", s.handleMessages)
		r.Get("/v1/models", s.handleModels)
		r.HandleFunc("/v1/*", s.handleCatchAll)
		r.Get("/api/account", s.handleAccountInfo)
		if s.usage != nil {
			r.Get("/api/usage", s.handleUsageSummary)
			r.Get("/api/usage/{email}", s.handleUsageByEmail)
		}
	}

	if opts.WebFS != nil {
		r.NotFound(spaHandler(opts.WebFS))
	}

	return r, s
}

type sessionAdapter struct {
	store *usage.Store
}

func (a *sessionAdapter) SaveSession(ctx context.Context, token, email string, expiresAt time.Time) error {
	return a.store.SaveSession(ctx, token, email, expiresAt)
}

func (a *sessionAdapter) LoadSessions(ctx context.Context) ([]auth.PersistedSession, error) {
	rows, err := a.store.LoadSessions(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]auth.PersistedSession, len(rows))
	for i, r := range rows {
		out[i] = auth.PersistedSession{Token: r.Token, Email: r.Email, ExpiresAt: r.ExpiresAt}
	}
	return out, nil
}

func handleSession(w http.ResponseWriter, r *http.Request) {
	email := auth.GetUserEmail(r.Context())
	w.Header().Set(HeaderContentType, "application/json")
	json.NewEncoder(w).Encode(map[string]string{"email": email})
}

func spaHandler(webFS fs.FS) http.HandlerFunc {
	fileServer := http.FileServer(http.FS(webFS))
	return func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		if len(path) > 0 && path[0] == '/' {
			path = path[1:]
		}
		if path == "" {
			path = "index.html"
		}
		if _, err := fs.Stat(webFS, path); err != nil {
			r.URL.Path = "/"
		}
		fileServer.ServeHTTP(w, r)
	}
}
