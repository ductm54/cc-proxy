#!/usr/bin/env python3
"""Diff a real Claude Code capture against cc-proxy's upstream requests.

Inputs are JSONL files in the cc-proxy debug-dump shape:
  --real   from mitm_addon.py (request under "client")
  --proxy  from cc-proxy's debug dump ("client" = what Claude Code sent the
           proxy, "upstream" = what the proxy sent Anthropic)
  --const  internal/proxy/const.go, to check the hardcoded values

The main question answered: does proxy.upstream look like real.client?
"""

import argparse
import json
import re
import sys
from collections import defaultdict
from urllib.parse import urlsplit

# Headers that legitimately differ per request or are transport-level.
VOLATILE_HEADERS = {
    "Content-Length",
    "Host",
    "Connection",
    "Proxy-Connection",
    "X-Stainless-Retry-Count",
    "X-Claude-Code-Session-Id",
    "X-Client-Request-Id",
}
# Known, intentional proxy changes; reported separately rather than as drift.
EXPECTED_HEADER_CHANGES = {
    "Accept-Encoding": "proxy strips it so Go decompresses SSE for usage parsing",
}
# Added by a tunnel/CDN in front of cc-proxy (must match addedByFrontProxy in
# forward.go). Expected on client→proxy; a leak if seen on proxy→upstream.
FRONT_PROXY_HEADERS = {
    "Cdn-Loop", "Forwarded", "True-Client-Ip", "X-Forwarded-For",
    "X-Forwarded-Host", "X-Forwarded-Proto", "X-Real-Ip",
}
BODY_SKIP = {"messages", "system", "tools"}


def is_front_proxy(k):
    return k in FRONT_PROXY_HEADERS or k.startswith("Cf-")


def load(path):
    out = []
    with open(path, encoding="utf-8") as f:
        for line in f:
            line = line.strip()
            if line:
                try:
                    out.append(json.loads(line))
                except ValueError:
                    pass
    return out


def path_of(url):
    return urlsplit(url).path


def is_messages(url):
    return path_of(url).endswith("/v1/messages")


def betas(headers):
    vals = headers.get("Anthropic-Beta") or []
    return [b.strip() for v in vals for b in v.split(",") if b.strip()]


def model_of(req):
    body = req.get("body")
    return body.get("model", "?") if isinstance(body, dict) else "?"


def pair_by_model(real, proxy):
    """Pair the i-th real request for a model with the i-th proxy one."""
    by_model = defaultdict(list)
    for r in proxy:
        by_model[model_of(r)].append(r)
    pairs, used = [], defaultdict(int)
    for r in real:
        m = model_of(r)
        i = used[m]
        pairs.append((r, by_model[m][i] if i < len(by_model[m]) else None))
        used[m] += 1
    return pairs


class Report:
    def __init__(self):
        self.lines = []
        self.drift = 0

    def h(self, title):
        self.lines += ["", title, "-" * len(title)]

    def ok(self, msg):
        self.lines.append(f"  ok    {msg}")

    def note(self, msg):
        self.lines.append(f"  note  {msg}")

    def diff(self, msg):
        self.drift += 1
        self.lines.append(f"  DIFF  {msg}")

    def text(self):
        return "\n".join(self.lines)


def short(v, n=160):
    s = v if isinstance(v, str) else json.dumps(v, separators=(",", ":"))
    return s if len(s) <= n else s[: n - 3] + "..."


def compare_headers(rep, want, got, label_want, label_got):
    keys = set(want) | set(got)
    clean = True
    front = []
    for k in sorted(keys):
        if k in VOLATILE_HEADERS or k == "Anthropic-Beta":
            continue
        a, b = want.get(k), got.get(k)
        if is_front_proxy(k) and (b is None or label_got == "client"):
            front.append(k)
            continue
        if k in EXPECTED_HEADER_CHANGES and a != b:
            rep.note(f"{k}: {label_want}={short(a)} {label_got}={short(b)} ({EXPECTED_HEADER_CHANGES[k]})")
            continue
        if k in ("Authorization", "X-Api-Key"):
            # Values are masked; only compare presence and auth scheme.
            sa = [v.split(" ")[0] for v in a] if a else None
            sb = [v.split(" ")[0] for v in b] if b else None
            if sa != sb:
                rep.diff(f"{k}: {label_want}={a} {label_got}={b}")
                clean = False
            continue
        if a is None:
            rep.diff(f"header only in {label_got}: {k}: {short(b)}")
            clean = False
        elif b is None:
            rep.diff(f"header missing in {label_got}: {k}: {short(a)}")
            clean = False
        elif a != b:
            rep.diff(f"header {k}: {label_want}={short(a)} {label_got}={short(b)}")
            clean = False
    if front:
        rep.note(f"front-proxy headers (tunnel/CDN, stripped by cc-proxy): {', '.join(front)}")
    if clean:
        rep.ok("headers match (ignoring volatile)")


def compare_betas(rep, want, got, label_want, label_got):
    sw, sg = set(want), set(got)
    if sw == sg:
        rep.ok(f"anthropic-beta match ({len(sw)} flags)")
        if want != got:
            rep.note("anthropic-beta order differs")
        return
    for b in want:
        if b not in sg:
            rep.diff(f"beta missing in {label_got}: {b}")
    for b in got:
        if b not in sw:
            rep.diff(f"beta only in {label_got}: {b}")


def user_id_keys(body):
    try:
        return sorted(json.loads(body["metadata"]["user_id"]).keys())
    except (KeyError, TypeError, ValueError):
        return None


def compare_body(rep, want, got, label_got, base="real"):
    if not isinstance(want, dict) or not isinstance(got, dict):
        if want != got:
            rep.diff(f"body not comparable as JSON objects ({type(want).__name__} vs {type(got).__name__})")
        return
    if list(want) != list(got):
        if set(want) == set(got):
            rep.note(f"body key order differs: {base}={list(want)} {label_got}={list(got)}")
        else:
            for k in want:
                if k not in got:
                    rep.diff(f"body field missing in {label_got}: {k}")
            for k in got:
                if k not in want:
                    rep.diff(f"body field only in {label_got}: {k}")
    for k in want:
        if k in BODY_SKIP or k not in got or k == "metadata":
            continue
        if want[k] != got[k]:
            rep.diff(f"body.{k}: {base}={short(want[k])} {label_got}={short(got[k])}")

    uk_w, uk_g = user_id_keys(want), user_id_keys(got)
    if uk_w != uk_g:
        rep.diff(f"metadata.user_id keys: {base}={uk_w} {label_got}={uk_g}")

    tw = [t.get("name") for t in want.get("tools") or [] if isinstance(t, dict)]
    tg = [t.get("name") for t in got.get("tools") or [] if isinstance(t, dict)]
    if tw != tg:
        missing = [t for t in tw if t not in tg]
        extra = [t for t in tg if t not in tw]
        rep.diff(f"tools differ: missing={missing} extra={extra}" + (" (order only)" if not missing and not extra else ""))

    sw, sg = want.get("system"), got.get("system")
    if isinstance(sw, list) and isinstance(sg, list):
        cw = [b.get("cache_control") for b in sw if isinstance(b, dict)]
        cg = [b.get("cache_control") for b in sg if isinstance(b, dict)]
        if len(sw) != len(sg):
            rep.diff(f"system blocks: {base}={len(sw)} {label_got}={len(sg)}")
        elif cw != cg:
            rep.diff(f"system cache_control: {base}={short(cw)} {label_got}={short(cg)}")
    elif type(sw) is not type(sg):
        rep.diff(f"system type: {base}={type(sw).__name__} {label_got}={type(sg).__name__}")

    nw, ng = len(want.get("messages") or []), len(got.get("messages") or [])
    if nw != ng:
        rep.note(f"messages count: {base}={nw} {label_got}={ng}")


def parse_const(path):
    src = open(path, encoding="utf-8").read()
    return dict(re.findall(r'^\s*(\w+)\s*=\s*"([^"]*)"', src, re.M))


def status_line(recs):
    counts = defaultdict(int)
    for r in recs:
        counts[(r.get("response") or {}).get("status", "err")] += 1
    return ", ".join(f"{k}×{v}" for k, v in sorted(counts.items(), key=lambda kv: str(kv[0])))


def error_message(rec):
    body = (rec.get("response") or {}).get("body")
    if isinstance(body, dict) and isinstance(body.get("error"), dict):
        return body["error"].get("message")
    return rec.get("error")


def main():
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("--real", required=True)
    ap.add_argument("--proxy")
    ap.add_argument("--const")
    args = ap.parse_args()

    rep = Report()
    real = load(args.real)
    real_msgs = [r for r in real if is_messages(r["client"]["url"])]

    rep.h(f"Real capture: {len(real)} flows, {len(real_msgs)} /v1/messages")
    endpoints = defaultdict(list)
    for r in real:
        c = r["client"]
        endpoints[f'{c["method"]} {urlsplit(c["url"]).netloc}{path_of(c["url"])}'].append(
            (r.get("response") or {}).get("status", "err"))
    for ep, st in sorted(endpoints.items()):
        rep.lines.append(f"  {ep}  {st}")
    if real_msgs:
        ua = real_msgs[0]["client"]["headers"].get("User-Agent", ["?"])[0]
        rep.lines.append(f"  user-agent: {ua}")

    if args.const:
        consts = parse_const(args.const)
        rep.h("const.go vs real")
        if real_msgs:
            union = []
            for r in real_msgs:
                union += [b for b in betas(r["client"]["headers"]) if b not in union]
            const_betas = [b for b in consts.get("SubscriptionBetaList", "").split(",") if b]
            rep.lines.append("  real betas per request:")
            for r in real_msgs:
                rep.lines.append(f"    {model_of(r['client'])}: {','.join(betas(r['client']['headers']))}")
            compare_betas(rep, union, const_betas, "real(union)", "SubscriptionBetaList")
            rep.lines.append(f"  suggested SubscriptionBetaList (union, real order): {','.join(union)}")

            ua = real_msgs[0]["client"]["headers"].get("User-Agent", [""])[0]
            m = re.search(r"claude-cli/([\d.]+)", ua)
            fake = consts.get("FakeUserAgent", "")
            if m and not fake.endswith("/" + m.group(1)):
                rep.diff(f"FakeUserAgent={fake!r} but real version is {m.group(1)} (UA {ua!r})")
            else:
                rep.ok(f"FakeUserAgent={fake!r}")

            real_url = real_msgs[0]["client"]["url"]
            if real_url != consts.get("UpstreamMessagesURL"):
                rep.diff(f"UpstreamMessagesURL={consts.get('UpstreamMessagesURL')!r} real={real_url!r}")
            else:
                rep.ok("UpstreamMessagesURL matches")
        for r in real:
            if path_of(r["client"]["url"]) == path_of(consts.get("AccountUsageURL", "")):
                ub = ",".join(betas(r["client"]["headers"]))
                if ub != consts.get("UsageBetaValue"):
                    rep.diff(f"UsageBetaValue={consts.get('UsageBetaValue')!r} real={ub!r}")
                else:
                    rep.ok("UsageBetaValue matches")
                break

    if args.proxy:
        proxy = load(args.proxy)
        proxy_msgs = [p for p in proxy if p.get("upstream") and is_messages(p["upstream"]["url"])]
        rep.h(f"Proxy capture: {len(proxy)} records, {len(proxy_msgs)} /v1/messages")
        rep.lines.append(f"  real statuses:  {status_line(real_msgs)}")
        rep.lines.append(f"  proxy statuses: {status_line(proxy_msgs)}")
        for p in proxy_msgs:
            if msg := error_message(p):
                rep.diff(f"proxy {p.get('req_id')} {p['response']['status'] if p.get('response') else ''}: {msg}")

        for i, (r, p) in enumerate(pair_by_model(real_msgs, proxy_msgs), 1):
            rc = r["client"]
            rep.h(f"Request {i}: {model_of(rc)}")
            if p is None:
                rep.diff("no matching proxy request for this model")
                continue
            up = p["upstream"]
            up_body = up.get("body") if not up.get("body_same_as_client") else p["client"].get("body")

            rep.lines.append("  [real vs proxy→upstream]  (what Anthropic sees)")
            if rc["url"] != up["url"]:
                rep.diff(f"url: real={rc['url']} upstream={up['url']}")
            compare_betas(rep, betas(rc["headers"]), betas(up["headers"]), "real", "upstream")
            compare_headers(rep, rc["headers"], up["headers"], "real", "upstream")
            compare_body(rep, rc.get("body"), up_body, "upstream")

            # Changes the proxy itself introduced, independent of Claude Code.
            rep.lines.append("  [client→proxy vs proxy→upstream]  (what cc-proxy changed)")
            client_body = p["client"].get("body")
            compare_betas(rep, betas(p["client"]["headers"]), betas(up["headers"]), "client", "upstream")
            compare_headers(rep, p["client"]["headers"], up["headers"], "client", "upstream")
            if not up.get("body_same_as_client"):
                compare_body(rep, client_body, up_body, "upstream", base="client")

            # Claude Code may itself behave differently behind a custom base URL.
            rep.lines.append("  [real vs client→proxy]  (does Claude Code change behaviour behind ANTHROPIC_BASE_URL?)")
            compare_betas(rep, betas(rc["headers"]), betas(p["client"]["headers"]), "real", "client")
            compare_headers(rep, rc["headers"], p["client"]["headers"], "real", "client")

    rep.h(f"Summary: {rep.drift} difference(s)")
    print(rep.text().lstrip("\n"))
    return 0


if __name__ == "__main__":
    sys.exit(main())
