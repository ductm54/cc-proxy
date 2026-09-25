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
	//
	// RequiredMessagesBetas are added to every /v1/messages request; the
	// client's own betas are kept, since body fields like safeguards or
	// per-message output_config are rejected without their beta.
	// SubscriptionBetaList is the fallback when the client sends no betas.
	RequiredMessagesBetas = "claude-code-20250219,oauth-2025-04-20"
	SubscriptionBetaList  = "claude-code-20250219,oauth-2025-04-20,context-1m-2025-08-07,interleaved-thinking-2025-05-14,thinking-token-count-2026-05-13,context-management-2025-06-27,prompt-caching-scope-2026-01-05,mid-conversation-system-2026-04-07,per-turn-control-2026-07-01,mid-conversation-tool-changes-2026-07-01,advisor-tool-2026-03-01,advanced-tool-use-2025-11-20,mid-conversation-system-clear-at-2026-08-21,effort-2025-11-24,thinking-binding-controls-2026-08-01,afk-mode-2026-01-31,extended-cache-ttl-2025-04-11,cache-diagnosis-2026-04-07,message-threads-2026-08-12"
	UsageBetaValue        = "oauth-2025-04-20"
	FakeUserAgent         = "claude-code/2.1.282"
	AcceptJSON            = "application/json, text/plain, */*"

	// Upstream URLs
	UpstreamMessagesURL = "https://api.anthropic.com/v1/messages?beta=true"
	UpstreamModelsURL   = "https://api.anthropic.com/v1/models"
	UpstreamBaseURL     = "https://api.anthropic.com"
	AccountUsageURL     = "https://api.anthropic.com/api/oauth/usage"
)
