# Updating cc-proxy for a new Claude Code version

When a new Claude Code version ships, capture its traffic via mitm proxy and
update `const.go` in this package. All mitm-derived values live in that single
file.

## What to capture

Run Claude Code through mitmproxy and note:

1. **`SubscriptionBetaList`** — the full `anthropic-beta` header value sent on
   `/v1/messages` requests. This is a comma-separated list of beta flags.
2. **`UsageBetaValue`** — the `anthropic-beta` value sent on the
   `/api/oauth/usage` endpoint (usually a subset of the full list).
3. **`FakeUserAgent`** — the `User-Agent` string, e.g. `claude-code/2.1.118`.
4. **Upstream URLs** — verify `UpstreamMessagesURL`, `UpstreamModelsURL`,
   `UpstreamBaseURL`, and `AccountUsageURL` haven't changed.

## Where to update

All values are in **`const.go`**:

```
internal/proxy/const.go
```

No other file should contain hard-coded header names, header values, or
upstream URLs. If you find one, move it to `const.go`.

## After updating

```bash
go test ./internal/proxy/ -v
```

Ensure all tests pass — the test suite validates header rewriting against
these constants.
