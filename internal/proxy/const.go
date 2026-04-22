package proxy

// ---------------------------------------------------------------------------
// Constants derived from mitm capture. Update these when a new CC version
// changes headers, values, or upstream URLs.
// ---------------------------------------------------------------------------

const (
	// Header names
	HeaderAuthorization = "Authorization"
	HeaderContentType   = "Content-Type"
	HeaderContentLength = "Content-Length"
	HeaderAccept        = "Accept"
	HeaderUserAgent     = "User-Agent"
	HeaderAnthropicBeta = "anthropic-beta"
	HeaderXApiKey       = "x-api-key"

	// Header values from mitm capture
	SubscriptionBetaList = "claude-code-20250219,oauth-2025-04-20,interleaved-thinking-2025-05-14,redact-thinking-2026-02-12,context-management-2025-06-27,prompt-caching-scope-2026-01-05,advisor-tool-2026-03-01,advanced-tool-use-2025-11-20,effort-2025-11-24"
	UsageBetaValue       = "oauth-2025-04-20"
	FakeUserAgent        = "claude-code/2.1.119"
	AcceptJSON           = "application/json, text/plain, */*"

	// Upstream URLs
	UpstreamMessagesURL = "https://api.anthropic.com/v1/messages?beta=true"
	UpstreamModelsURL   = "https://api.anthropic.com/v1/models"
	UpstreamBaseURL     = "https://api.anthropic.com"
	AccountUsageURL     = "https://api.anthropic.com/api/oauth/usage"
)
