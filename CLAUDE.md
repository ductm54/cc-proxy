# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What is this?

cc-proxy is a local proxy that forwards Anthropic API requests to `api.anthropic.com` using a Claude Max/Pro subscription OAuth token. It includes optional Google OAuth for multi-user access and ClickHouse-backed usage tracking. A React SPA dashboard is embedded into the Go binary.

## Commands

```bash
make all          # Build everything (web + Go binary)
make build        # Build Go binary only (requires web/dist/ to exist)
make web          # Install deps and build frontend with Bun
make dev          # Run dev server (Vite frontend + Go backend)
make test         # Run all Go tests (go test ./...)
make clean        # Remove binary and web/dist/

# Run a single test
go test ./internal/proxy/ -run TestForward

# CLI usage
./cc-proxy bootstrap    # Import credentials from ~/.claude/.credentials.json
./cc-proxy serve        # Start proxy (default: 127.0.0.1:8787)
./cc-proxy status       # Show token expiry info
```

## Architecture

### Request flow

```
Client → chi router → [auth middleware] → forward.go → api.anthropic.com
                                              ↓
                                    rewrite account_uuid into body
                                    inject Bearer token + beta headers
                                    stream response back (with usage extraction for SSE)
```

### Package layout

- **`cmd/cc-proxy/`** — CLI entry point (urfave/cli v3): bootstrap, serve, status commands
- **`internal/proxy/`** — Core proxy: router, request forwarding, header management, rate limiting, account UUID rewriting, usage endpoint handlers
- **`internal/tokens/`** — Token lifecycle: in-memory holder with background refresh goroutine (ticks every 60s, refreshes 5min before expiry), file persistence to `.config/.credentials.json`, account profile fetching
- **`internal/auth/`** — Google OAuth2 flow, auth middleware, in-memory session store, email domain filtering
- **`internal/usage/`** — ClickHouse storage for per-request token consumption and cost tracking; optional (proxy works without it)
- **`internal/config/`** — Config types loaded from JSON file, overridable by CLI flags/env vars
- **`web/`** — React SPA (Vite + Tailwind + Recharts), embedded into Go binary via `embed.FS` in `web/web.go`

### Key patterns

- **Header forwarding**: Hop-by-hop headers are stripped. Authorization, anthropic-beta, x-api-key are overridden by proxy. Everything else is forwarded as-is.
- **Error responses**: JSON matching Anthropic API format with error types `cc_proxy_auth`, `cc_proxy_internal`, `cc_proxy_upstream`.
- **SSE usage extraction**: For streaming responses, scans `data:` lines to extract token counts from the final message event.
- **Token refresh**: OAuth refresh via `platform.claude.com/v1/oauth/token` with exponential backoff on failure.
- **Two routing modes**: With OAuth enabled, inference goes through `/p/{token}/v1/messages`. Without auth, it's the standard `/v1/messages`.

### Infrastructure

- **ClickHouse** (docker-compose.yml): Stores usage and session tables with TTL (365d / 1d). Migrations in `internal/usage/migrations/`.
- **Frontend**: Built with Bun, output to `web/dist/`, served as embedded static assets with SPA fallback routing.

## Stack

Go 1.25, chi v5, zap, golang.org/x/oauth2, ClickHouse, urfave/cli v3. Frontend: React 19, Vite, Tailwind CSS 4, Recharts, Bun.
