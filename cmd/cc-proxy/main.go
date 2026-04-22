package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/urfave/cli/v3"
	"go.uber.org/zap"

	"github.com/ductm54/cc-proxy/internal/config"
	"github.com/ductm54/cc-proxy/internal/logging"
	"github.com/ductm54/cc-proxy/internal/proxy"
	"github.com/ductm54/cc-proxy/internal/tokens"
	"github.com/ductm54/cc-proxy/internal/usage"
	"github.com/ductm54/cc-proxy/web"
)

func main() {
	app := &cli.Command{
		Name:  "cc-proxy",
		Usage: "Local Anthropic API proxy backed by a Claude Max/Pro subscription",
		Commands: []*cli.Command{
			bootstrapCmd(),
			serveCmd(),
			statusCmd(),
		},
	}
	if err := app.Run(context.Background(), os.Args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func bootstrapCmd() *cli.Command {
	return &cli.Command{
		Name:  "bootstrap",
		Usage: "Import OAuth credentials from Claude Code's credentials file",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "tokens-file",
				Usage:   "Path to write tokens.json",
				Value:   config.DefaultTokensFile(),
				Sources: cli.EnvVars("CC_PROXY_TOKENS_FILE"),
			},
			&cli.BoolFlag{
				Name:    "force",
				Usage:   "Overwrite existing tokens file",
				Sources: cli.EnvVars("CC_PROXY_BOOTSTRAP_FORCE"),
			},
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			path := cmd.String("tokens-file")
			force := cmd.Bool("force")
			if err := tokens.Bootstrap(path, force); err != nil {
				return err
			}
			fmt.Printf("symlinked %s → ~/.claude/.credentials.json\n", path)
			return nil
		},
	}
}

func serveCmd() *cli.Command {
	return &cli.Command{
		Name:  "serve",
		Usage: "Run the proxy server",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "addr",
				Usage:   "Listen address",
				Value:   "127.0.0.1:8787",
				Sources: cli.EnvVars("CC_PROXY_ADDR"),
			},
			&cli.StringFlag{
				Name:    "tokens-file",
				Usage:   "Path to tokens.json",
				Value:   config.DefaultTokensFile(),
				Sources: cli.EnvVars("CC_PROXY_TOKENS_FILE"),
			},
			&cli.DurationFlag{
				Name:    "refresh-skew",
				Usage:   "Refresh token this far before expiry",
				Value:   5 * time.Minute,
				Sources: cli.EnvVars("CC_PROXY_REFRESH_SKEW"),
			},
			&cli.BoolFlag{
				Name:    "log-dev",
				Usage:   "Use human-readable log output",
				Sources: cli.EnvVars("CC_PROXY_LOG_DEV"),
			},
			&cli.StringFlag{
				Name:    "auth-config",
				Usage:   "Path to auth.json config file",
				Value:   config.DefaultAuthConfigFile(),
				Sources: cli.EnvVars("CC_PROXY_AUTH_CONFIG"),
			},
			&cli.StringFlag{
				Name:    "oauth-client-id",
				Usage:   "Google OAuth2 client ID (enables auth when set)",
				Sources: cli.EnvVars("CC_PROXY_OAUTH_CLIENT_ID"),
			},
			&cli.StringFlag{
				Name:    "oauth-client-secret",
				Usage:   "Google OAuth2 client secret",
				Sources: cli.EnvVars("CC_PROXY_OAUTH_CLIENT_SECRET"),
			},
			&cli.StringFlag{
				Name:    "oauth-domain",
				Usage:   "Allowed email domain (e.g. company.com); empty allows any",
				Sources: cli.EnvVars("CC_PROXY_OAUTH_DOMAIN"),
			},
			&cli.DurationFlag{
				Name:    "auth-token-ttl",
				Usage:   "Auth token validity duration",
				Value:   2 * time.Hour,
				Sources: cli.EnvVars("CC_PROXY_AUTH_TOKEN_TTL"),
			},
			&cli.StringFlag{
				Name:    "external-url",
				Usage:   "Public URL of the proxy (for OAuth redirect URI)",
				Sources: cli.EnvVars("CC_PROXY_EXTERNAL_URL"),
			},
			&cli.StringFlag{
				Name:    "clickhouse-dsn",
				Usage:   "ClickHouse DSN for usage tracking (e.g. clickhouse://localhost:9000/cc_proxy)",
				Sources: cli.EnvVars("CC_PROXY_CLICKHOUSE_DSN"),
				Value:   "clickhouse://user:password@localhost:29000/cc_proxy",
			},
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			log, err := logging.New(cmd.Bool("log-dev"))
			if err != nil {
				return fmt.Errorf("init logger: %w", err)
			}
			defer log.Sync() //nolint:errcheck

			tokensFile := cmd.String("tokens-file")
			refreshSkew := cmd.Duration("refresh-skew")

			mgr, err := tokens.NewManager(tokensFile, refreshSkew, "", log)
			if err != nil {
				return err
			}

			ctx, cancel := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
			defer cancel()

			go mgr.Start(ctx)

			addr := cmd.String("addr")

			authCfg, err := buildAuthConfig(cmd, addr, log)
			if err != nil {
				return err
			}

			var usageStore *usage.Store
			if dsn := cmd.String("clickhouse-dsn"); dsn != "" {
				usageStore, err = usage.NewStore(dsn)
				if err != nil {
					return fmt.Errorf("init usage store: %w", err)
				}
				defer usageStore.Close()
				log.Info("usage tracking enabled", zap.String("clickhouse", dsn))
			}

			webFS, err := web.WebFS()
			if err != nil {
				return fmt.Errorf("load web assets: %w", err)
			}

			handler, _ := proxy.New(mgr, log, proxy.Options{AuthConfig: authCfg, WebFS: webFS, UsageStore: usageStore})

			srv := &http.Server{
				Addr:              addr,
				Handler:           handler,
				ReadHeaderTimeout: 10 * time.Second,
				IdleTimeout:       120 * time.Second,
			}

			go func() {
				<-ctx.Done()
				shutCtx, shutCancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer shutCancel()
				_ = srv.Shutdown(shutCtx)
			}()

			log.Info("cc-proxy listening", zap.String("addr", addr))
			if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				return fmt.Errorf("server error: %w", err)
			}
			return nil
		},
	}
}

func buildAuthConfig(cmd *cli.Command, addr string, log *zap.Logger) (*config.AuthConfig, error) {
	cfg, err := config.LoadAuthConfig(cmd.String("auth-config"))
	if err != nil {
		return nil, fmt.Errorf("load auth config: %w", err)
	}

	if v := cmd.String("oauth-client-id"); v != "" {
		cfg.OAuthClientID = v
	}
	if v := cmd.String("oauth-client-secret"); v != "" {
		cfg.OAuthClientSecret = v
	}
	if v := cmd.String("oauth-domain"); v != "" {
		cfg.OAuthDomain = v
	}
	if v := cmd.String("external-url"); v != "" {
		cfg.ExternalURL = v
	}
	if cmd.IsSet("auth-token-ttl") {
		cfg.AuthTokenTTL = cmd.Duration("auth-token-ttl").String()
	}

	if !cfg.Enabled() {
		return nil, nil
	}

	if cfg.OAuthClientSecret == "" {
		return nil, fmt.Errorf("--oauth-client-secret is required when --oauth-client-id is set")
	}

	if cfg.AuthTokenTTL != "" {
		ttl, err := time.ParseDuration(cfg.AuthTokenTTL)
		if err != nil {
			return nil, fmt.Errorf("invalid auth-token-ttl %q: %w", cfg.AuthTokenTTL, err)
		}
		cfg.TokenTTL = ttl
	}
	if cfg.TokenTTL == 0 {
		cfg.TokenTTL = 2 * time.Hour
	}

	if cfg.ExternalURL == "" {
		cfg.ExternalURL = "http://" + addr
	}

	log.Info("oauth auth enabled",
		zap.String("domain", cfg.OAuthDomain),
		zap.Duration("token_ttl", cfg.TokenTTL),
		zap.String("redirect_url", cfg.RedirectURL()),
	)
	return cfg, nil
}

func statusCmd() *cli.Command {
	return &cli.Command{
		Name:  "status",
		Usage: "Show current token status",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "tokens-file",
				Usage:   "Path to tokens.json",
				Value:   config.DefaultTokensFile(),
				Sources: cli.EnvVars("CC_PROXY_TOKENS_FILE"),
			},
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			tok, err := tokens.Load(cmd.String("tokens-file"))
			if err != nil {
				return err
			}
			remaining := time.Until(tok.ExpiresAt)
			fmt.Printf("access_token: %s...\n", tok.AccessToken[:min(20, len(tok.AccessToken))])
			fmt.Printf("expires_at:   %s (%s remaining)\n", tok.ExpiresAt.Format(time.RFC3339), remaining.Round(time.Second))
			if remaining < 0 {
				fmt.Println("WARNING: token is expired — run `cc-proxy serve` to auto-refresh, or `cc-proxy bootstrap` to re-import")
			}
			return nil
		},
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
