"""mitmdump addon: write Anthropic/Claude traffic as JSONL.

Records use the same shape as cc-proxy's debug dump (internal/debugdump), with
the real request under "client" and no "upstream" section, so compare.py can
diff a direct Claude Code request against what cc-proxy sends upstream.

Output path comes from the CC_CAPTURE_OUT environment variable.
"""

import json
import os
from datetime import datetime, timezone

from mitmproxy import http

HOST_SUFFIXES = ("anthropic.com", "claude.com", "claude.ai")
SENSITIVE = {"Authorization", "Proxy-Authorization", "X-Api-Key", "Cookie", "Set-Cookie"}
MAX_RESPONSE_BODY = 4 << 20

_out = open(os.environ["CC_CAPTURE_OUT"], "a", encoding="utf-8")


def canonical(key):
    """Match Go's http.CanonicalHeaderKey."""
    return "-".join(p.capitalize() for p in key.split("-"))


def mask(v):
    prefix = ""
    if " " in v:
        scheme, v = v.split(" ", 1)
        prefix = scheme + " "
    if len(v) <= 8:
        return prefix + "***"
    return prefix + "***" + v[-4:]


def headers(h):
    out = {}
    for k, v in h.items(multi=True):
        k = canonical(k)
        out.setdefault(k, []).append(mask(v) if k in SENSITIVE else v)
    return out


def body(raw):
    if not raw:
        return None
    try:
        return json.loads(raw)
    except ValueError:
        return raw.decode("utf-8", "replace")


def wanted(flow):
    host = flow.request.pretty_host
    return any(host == s or host.endswith("." + s) for s in HOST_SUFFIXES)


def write(flow, error=None):
    req = flow.request
    raw_req = req.get_content(strict=False) or b""
    rec = {
        "ts": datetime.fromtimestamp(req.timestamp_start, timezone.utc).isoformat(),
        "req_id": flow.id,
        "source": "mitm",
        "client": {
            "method": req.method,
            "url": req.pretty_url,
            "headers": headers(req.headers),
            # Wire order is lost in the dict above; keep it for fingerprint checks.
            "header_order": [canonical(k) for k, _ in req.headers.items(multi=True)],
            "body_bytes": len(raw_req),
            "body": body(raw_req),
        },
    }
    resp = flow.response
    if resp is not None:
        raw = resp.get_content(strict=False) or b""
        rec["response"] = {
            "status": resp.status_code,
            "headers": headers(resp.headers),
            "body_bytes": len(raw),
            "body": body(raw[:MAX_RESPONSE_BODY]) if len(raw) <= MAX_RESPONSE_BODY
            else raw[:MAX_RESPONSE_BODY].decode("utf-8", "replace"),
            "body_truncated": len(raw) > MAX_RESPONSE_BODY,
        }
        rec["dur_ms"] = int((resp.timestamp_end - req.timestamp_start) * 1000)
    if error:
        rec["error"] = error
    _out.write(json.dumps(rec, separators=(",", ":")) + "\n")
    _out.flush()


def response(flow: http.HTTPFlow):
    if wanted(flow):
        write(flow)


def error(flow: http.HTTPFlow):
    if wanted(flow) and flow.response is None:
        write(flow, error=str(flow.error))
