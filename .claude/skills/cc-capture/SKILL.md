---
name: cc-capture
description: Capture the latest Claude Code request shape, diff it against what cc-proxy sends upstream (debug dump), scan the live dump for upstream errors, and propose concrete fixes to cc-proxy. Use when the user asks to check dumps, compare with the latest capture, update cc-proxy for a new Claude Code version, or investigate 4xx errors like "Extra inputs are not permitted" through the proxy.
---

# cc-capture: dump → compare → propose fix

Goal: make what cc-proxy sends to `api.anthropic.com` look like what Claude Code
sends when it talks to Anthropic directly, and find proxy-caused failures.
Background and field meanings: `internal/proxy/MITM_UPDATE.md`.

## 1. Preconditions

- cc-proxy must be running the **current** code with debug dump on
  (`CC_PROXY_DEBUG_DUMP_DIR`, set in `docker-compose.yml`). If Go code changed
  since the container was built, rebuild first:
  `docker compose --profile proxy up -d --build cc-proxy` (the user may need to
  run it with `!` if docker needs elevated access).
- The dump dir `.config/dumps/` is root-owned; read it with `sudo -n`. If
  `sudo -n` fails, ask the user to run the command with `!`.
- A proxy URL: `$ANTHROPIC_BASE_URL`, or ask the user for the
  `http://…/p/{token}` or `/k/{key}` URL. Never print the token/key; redact it.

## 2. Capture and compare

```bash
scripts/cc-capture/capture.sh                      # default prompt, default model
scripts/cc-capture/capture.sh -m <model> -p "<prompt>"   # if the user names one
```

This runs `claude -p` once directly through mitmdump and once through
cc-proxy, and writes `captures/<cc-version>-<ts>/report.txt`. If the user only
wants to re-check an existing capture (e.g. after a code change, with a fresh
proxy run), re-run `scripts/cc-capture/compare.py --real … --proxy … --const
internal/proxy/const.go`.

Never read or print `real.mitm` — it contains the unredacted OAuth token.

## 3. Scan the live dump

```bash
sudo -n sh -c 'cat .config/dumps/dump*.jsonl' | python3 scripts/cc-capture/dump_errors.py
```

Add `--since <ISO ts>` to limit to traffic after the last rebuild. This covers
real user traffic, which exercises far more paths than the one-shot capture.

## 4. Triage

Read `report.txt` and the dump summary. Classify each DIFF:

| Symptom | Cause | Where to fix |
|---|---|---|
| proxy 4xx `X: Extra inputs are not permitted` | beta for body field X dropped; compare "betas dropped by proxy" | `mergeBetas` / `messagesBetas` in `internal/proxy/forward.go`, `RequiredMessagesBetas` in `const.go` |
| "const.go vs real" beta/UA/URL DIFF | Claude Code version bump | `SubscriptionBetaList`, `FakeUserAgent`, URLs in `internal/proxy/const.go` |
| `header only in upstream` | proxy leaks a header (tunnel/CDN etc.) | `shouldForward` / `addedByFrontProxy` in `forward.go` (and `FRONT_PROXY_HEADERS` in `compare.py`) |
| `header missing in upstream` also missing in client | Claude Code omits it behind `ANTHROPIC_BASE_URL` | usually not fixable in proxy; report only |
| body field / key order differs, client→upstream | proxy re-marshals the body | `internal/proxy/rewrite.go` |
| `real vs client→proxy` diffs (tools, `thread`, `diagnostics`) | Claude Code behaves differently behind a custom base URL | report only, unless the user wants the proxy to emulate it |
| `Accept-Encoding` note | intentional (usage parsing) | none |

Ignore volatile values (session/request ids, content length) and Cloudflare
headers on the client side.

## 5. Propose the fix

Present to the user, don't edit yet:

1. A short findings table: symptom → root cause → evidence (req_id, error
   text, counts from the dump).
2. For each fixable item, the concrete change with `file:line` and a code
   sketch, plus the test to add/adjust in `internal/proxy/*_test.go`.
3. Items that are report-only, and why.

Once the user approves: apply the changes, run `go test ./...`, ask the user to rebuild the container,
then re-run step 2 and confirm the DIFFs are gone and the dump shows no new
4xx for the fixed errors.
