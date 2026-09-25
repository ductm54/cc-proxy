#!/usr/bin/env bash
# Capture the latest Claude Code request shape and diff it against cc-proxy.
#
#   1. real  — `claude -p` straight to api.anthropic.com through mitmdump
#   2. proxy — the same prompt through cc-proxy, read back from its debug dump
#   3. compare.py diffs real vs proxy-upstream and checks const.go
#
# Usage:
#   scripts/cc-capture/capture.sh [options] [-- extra claude args]
#
# Options:
#   -p, --prompt TEXT     prompt to send (default: "Reply with exactly: pong")
#   -m, --model MODEL     model for both runs (default: Claude Code's default)
#   -u, --proxy-url URL   cc-proxy base URL incl. /p/{token} or /k/{key}
#                         (default: $ANTHROPIC_BASE_URL)
#       --dump-dir DIR    cc-proxy debug dump dir (default: .config/dumps)
#       --real-only       skip the proxy run; only compare against const.go
#   -o, --out DIR         output dir (default: captures/<cc-version>-<utc ts>)
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_DIR="$(cd "$SCRIPT_DIR/../.." && pwd)"

PROMPT="Reply with exactly: pong"
MODEL=""
PROXY_URL="${ANTHROPIC_BASE_URL:-}"
DUMP_DIR="$REPO_DIR/.config/dumps"
REAL_ONLY=0
OUT=""
EXTRA=()

while [[ $# -gt 0 ]]; do
  case "$1" in
    -p|--prompt)    PROMPT="$2"; shift 2 ;;
    -m|--model)     MODEL="$2"; shift 2 ;;
    -u|--proxy-url) PROXY_URL="$2"; shift 2 ;;
    --dump-dir)     DUMP_DIR="$2"; shift 2 ;;
    --real-only)    REAL_ONLY=1; shift ;;
    -o|--out)       OUT="$2"; shift 2 ;;
    --)             shift; EXTRA=("$@"); break ;;
    -h|--help)      sed -n '2,20p' "$0"; exit 0 ;;
    *) echo "unknown option: $1" >&2; exit 2 ;;
  esac
done

for bin in claude mitmdump python3; do
  command -v "$bin" >/dev/null || { echo "missing: $bin" >&2; exit 1; }
done
CA_CERT="$HOME/.mitmproxy/mitmproxy-ca-cert.pem"
[[ -f "$CA_CERT" ]] || { echo "missing $CA_CERT — run mitmdump once to generate it" >&2; exit 1; }
if [[ $REAL_ONLY -eq 0 && -z "$PROXY_URL" ]]; then
  echo "no proxy URL: pass --proxy-url or set ANTHROPIC_BASE_URL (or use --real-only)" >&2
  exit 1
fi

CC_VERSION="$(claude --version 2>/dev/null | awk '{print $1}')"
OUT="${OUT:-$REPO_DIR/captures/${CC_VERSION}-$(date -u +%Y%m%dT%H%M%SZ)}"
mkdir -p "$OUT"
chmod 700 "$OUT"
echo "claude code $CC_VERSION → $OUT"

# Both runs execute in the same empty dir with a minimal environment, so no
# project CLAUDE.md, parent-session variables, or ANTHROPIC_* overrides leak in.
WORK="$(mktemp -d)"
MITM_PID=""
cleanup() {
  [[ -n "$MITM_PID" ]] && kill "$MITM_PID" 2>/dev/null && wait "$MITM_PID" 2>/dev/null
  rm -rf "$WORK"
}
trap cleanup EXIT

# Each run gets its own session id so its records can be picked out of a
# shared dump even while other sessions are using the proxy.
REAL_SID="$(python3 -c 'import uuid; print(uuid.uuid4())')"
PROXY_SID="$(python3 -c 'import uuid; print(uuid.uuid4())')"

CLAUDE_ARGS=(-p "$PROMPT")
[[ -n "$MODEL" ]] && CLAUDE_ARGS+=(--model "$MODEL")
CLAUDE_ARGS+=("${EXTRA[@]}")

clean_env() {
  env -i HOME="$HOME" USER="${USER:-}" PATH="$PATH" TERM="${TERM:-dumb}" LANG="${LANG:-C.UTF-8}" "$@"
}

# --- 1. real run through mitm ------------------------------------------------
PORT="$(python3 -c 'import socket; s=socket.socket(); s.bind(("127.0.0.1",0)); print(s.getsockname()[1])')"
: > "$OUT/real.jsonl"
CC_CAPTURE_OUT="$OUT/real.jsonl" mitmdump -q --listen-host 127.0.0.1 -p "$PORT" \
  -s "$SCRIPT_DIR/mitm_addon.py" -w "$OUT/real.mitm" >"$OUT/mitmdump.log" 2>&1 &
MITM_PID=$!
for _ in $(seq 50); do
  (exec 3<>"/dev/tcp/127.0.0.1/$PORT") 2>/dev/null && break
  sleep 0.2
done
kill -0 "$MITM_PID" 2>/dev/null || { echo "mitmdump failed:" >&2; cat "$OUT/mitmdump.log" >&2; exit 1; }

echo "[1/2] real: claude -p via mitm on :$PORT"
(cd "$WORK" && clean_env HTTPS_PROXY="http://127.0.0.1:$PORT" HTTP_PROXY="http://127.0.0.1:$PORT" \
  NODE_EXTRA_CA_CERTS="$CA_CERT" claude --session-id "$REAL_SID" "${CLAUDE_ARGS[@]}") >"$OUT/real.out" 2>&1 \
  || echo "  claude exited non-zero (see real.out)"
kill "$MITM_PID" 2>/dev/null; wait "$MITM_PID" 2>/dev/null || true
MITM_PID=""
# The raw flow file carries the unredacted OAuth token.
chmod 600 "$OUT/real.mitm" 2>/dev/null || true
echo "  captured $(wc -l < "$OUT/real.jsonl") flows"

# --- 2. proxy run, read back from the debug dump -----------------------------
if [[ $REAL_ONLY -eq 0 ]]; then
  read_dump() { if [[ -r "$DUMP_DIR/dump.jsonl" ]]; then cat "$@"; else sudo -n cat "$@"; fi; }
  if ! read_dump "$DUMP_DIR/dump.jsonl" >/dev/null 2>&1; then
    echo "cannot read $DUMP_DIR/dump.jsonl — is cc-proxy running with CC_PROXY_DEBUG_DUMP_DIR?" >&2
    exit 1
  fi
  echo "[2/2] proxy: claude -p via $(sed -E 's#/(p|k)/[^/]+#/\1/REDACTED#' <<<"$PROXY_URL")"
  (cd "$WORK" && clean_env ANTHROPIC_BASE_URL="$PROXY_URL" claude --session-id "$PROXY_SID" "${CLAUDE_ARGS[@]}") >"$OUT/proxy.out" 2>&1 \
    || echo "  claude exited non-zero (see proxy.out)"
  sleep 1 # dump records are written after the response body finishes relaying
  # Include rotated files in case the dump rotated mid-run; filter by session.
  mapfile -t DUMP_FILES < <( (ls -1 "$DUMP_DIR" 2>/dev/null || sudo -n ls -1 "$DUMP_DIR") \
    | grep -E '^dump.*\.jsonl$' | sed "s#^#$DUMP_DIR/#")
  read_dump "${DUMP_FILES[@]}" | python3 -c '
import json, sys
sid = sys.argv[1]
for line in sys.stdin:
    try:
        rec = json.loads(line)
    except ValueError:
        continue
    if sid in (rec.get("client", {}).get("headers", {}).get("X-Claude-Code-Session-Id") or []):
        sys.stdout.write(line)
' "$PROXY_SID" > "$OUT/proxy.jsonl"
  echo "  captured $(wc -l < "$OUT/proxy.jsonl") dump records"
fi

# --- 3. compare --------------------------------------------------------------
COMPARE=(python3 "$SCRIPT_DIR/compare.py" --real "$OUT/real.jsonl" --const "$REPO_DIR/internal/proxy/const.go")
[[ $REAL_ONLY -eq 0 ]] && COMPARE+=(--proxy "$OUT/proxy.jsonl")
"${COMPARE[@]}" | tee "$OUT/report.txt"
echo
echo "report: $OUT/report.txt"
