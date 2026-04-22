# cc-proxy

Local Anthropic API proxy that forwards requests to `api.anthropic.com` using
a Claude Max/Pro subscription OAuth token. Lets any Anthropic-API-compatible
client consume your subscription quota.

> ⚠️ Uses undocumented endpoints of Claude.ai's subscription service and may
> violate the Claude.ai ToS. Confirm your use case is allowed before relying on it.

## Quick start

### 1. Bootstrap credentials

The proxy reads the OAuth tokens that Claude Code stores after `claude login`:

```sh
cc-proxy bootstrap
```

This copies `~/.claude/.credentials.json` into `.config/.credentials.json`
(perms `0600`). Use `--force` to overwrite an existing file.

### 2. Start the proxy

```sh
cc-proxy serve
```

Listens on `127.0.0.1:8787` by default. Tokens are refreshed automatically
5 minutes before expiry.

### 3. Point a client at it

```sh
ANTHROPIC_BASE_URL=http://127.0.0.1:8787 \
ANTHROPIC_AUTH_TOKEN=placeholder \
claude -p "reply with the single word: ok"
```

Any Anthropic SDK client works — just set `ANTHROPIC_BASE_URL` (or the
equivalent base-URL option) and provide any non-empty value for the API key
(the proxy strips it and injects the subscription token).

## OAuth Authentication (optional)

You can require users to sign in with Google before using the proxy. When
enabled, users visit a login page in their browser, complete Google OAuth, and
receive a short-lived token to use with the proxy.

### Setup

1. Create a Google OAuth2 application at
   [console.cloud.google.com](https://console.cloud.google.com/apis/credentials).
   Add `http://<your-proxy-addr>/auth/callback` as an authorised redirect URI.

2. Configure auth via a JSON file, CLI flags, or environment variables (flags
   override the file). Create `.config/auth.json`:

   ```json
   {
     "oauth_client_id": "YOUR_CLIENT_ID.apps.googleusercontent.com",
     "oauth_client_secret": "YOUR_CLIENT_SECRET",
     "oauth_domain": "company.com",
     "auth_token_ttl": "2h",
     "external_url": "https://cc-proxy.example.com"
   }
   ```

   - **oauth_domain** — restrict login to a specific email domain (leave empty
     to allow any Google account).
   - **auth_token_ttl** — how long issued tokens are valid (default `2h`).
   - **external_url** — the public URL of the proxy. Defaults to `http://<addr>`.

3. Start the proxy as usual:

   ```sh
   cc-proxy serve
   ```

   When `oauth_client_id` is set the proxy enables auth automatically; when it
   is absent, auth is disabled and behaviour is unchanged.

### User flow

1. Open `https://<proxy>/auth/login` in a browser.
2. Click **Sign in with Google** and complete the OAuth flow.
3. On success, the page shows ready-to-copy bash commands:

   ```sh
   export ANTHROPIC_BASE_URL="https://cc-proxy.example.com/p/a1b2c3d4"
   export ANTHROPIC_API_KEY="x"
   ```

   The auth token is embedded in the URL path (`/p/<token>`), so
   `ANTHROPIC_API_KEY` can be any non-empty placeholder — no conflict with a
   real API key.

4. Paste into a terminal and use any Anthropic SDK client as normal.

Tokens are stored in memory and do not survive proxy restarts.
The `/healthz` endpoint remains unauthenticated.

## Commands

| Command | Description |
|---------|-------------|
| `cc-proxy bootstrap [--force]` | Import credentials from Claude Code |
| `cc-proxy serve` | Start the proxy server |
| `cc-proxy status` | Show token expiry |

## Environment variables / flags

All flags can be set via environment variable.

### Proxy

| Flag | Env var | Default | Description |
|------|---------|---------|-------------|
| `--addr` | `CC_PROXY_ADDR` | `127.0.0.1:8787` | Listen address |
| `--tokens-file` | `CC_PROXY_TOKENS_FILE` | `.config/.credentials.json` | Token storage |
| `--refresh-skew` | `CC_PROXY_REFRESH_SKEW` | `5m` | Refresh this far before expiry |
| `--log-dev` | `CC_PROXY_LOG_DEV` | `false` | Human-readable console logs |
| `--force` (bootstrap) | `CC_PROXY_BOOTSTRAP_FORCE` | `false` | Overwrite existing tokens file |

### Authentication

| Flag | Env var | Default | Description |
|------|---------|---------|-------------|
| `--auth-config` | `CC_PROXY_AUTH_CONFIG` | `.config/auth.json` | Path to auth config file |
| `--oauth-client-id` | `CC_PROXY_OAUTH_CLIENT_ID` | — | Google OAuth2 client ID (enables auth) |
| `--oauth-client-secret` | `CC_PROXY_OAUTH_CLIENT_SECRET` | — | Google OAuth2 client secret |
| `--oauth-domain` | `CC_PROXY_OAUTH_DOMAIN` | — | Restrict to email domain |
| `--auth-token-ttl` | `CC_PROXY_AUTH_TOKEN_TTL` | `2h` | Auth token lifetime |
| `--external-url` | `CC_PROXY_EXTERNAL_URL` | `http://<addr>` | Public URL (for OAuth redirect) |

## Endpoints

| Method | Path | Auth required | Description |
|--------|------|:---:|-------------|
| `GET` | `/auth/login` | No | Login page (HTML) |
| `GET` | `/auth/start` | No | Starts Google OAuth flow |
| `GET` | `/auth/callback` | No | OAuth callback → shows token + bash instructions |
| `POST` | `/p/{token}/v1/messages` | Yes* | Inference (streaming + non-streaming) |
| `GET` | `/p/{token}/v1/models` | Yes* | Model list |
| `POST` | `/v1/messages` | No** | Inference (no-auth mode only) |
| `GET` | `/v1/models` | No** | Model list (no-auth mode only) |
| `GET` | `/healthz` | No | Health check + token expiry |

\* When auth is enabled, API routes are served under `/p/{token}/` with the
short token from the login page.

\*\* Without auth config, routes are served at the root as before.

## Build

```sh
go build -o cc-proxy ./cmd/cc-proxy
```

Requires Go 1.22+.
