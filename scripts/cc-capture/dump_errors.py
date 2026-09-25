#!/usr/bin/env python3
"""Summarise cc-proxy's live debug dump: status counts per endpoint and the
distinct upstream error messages, with the betas the client sent vs what the
proxy forwarded for each error.

  sudo -n cat .config/dumps/dump*.jsonl | scripts/cc-capture/dump_errors.py
  scripts/cc-capture/dump_errors.py --since 2026-09-25T16:00 .config/dumps/dump.jsonl
"""

import argparse
import collections
import json
import sys
from urllib.parse import urlsplit


def betas(headers):
    vals = (headers or {}).get("Anthropic-Beta") or []
    return {b.strip() for v in vals for b in v.split(",") if b.strip()}


def error_message(body):
    if isinstance(body, str):
        try:
            body = json.loads(body)
        except ValueError:
            return body[:160]
    if isinstance(body, dict):
        err = body.get("error")
        if isinstance(err, dict):
            return f'{err.get("type")}: {err.get("message")}'
    return json.dumps(body)[:160]


def endpoint(url):
    path = urlsplit(url).path
    return "/v1/" + path.split("/v1/", 1)[1] if "/v1/" in path else path


def main():
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("files", nargs="*", help="dump files (default: stdin)")
    ap.add_argument("--since", help="only records with ts >= this ISO prefix")
    args = ap.parse_args()

    lines = (l for f in args.files for l in open(f, encoding="utf-8")) if args.files else sys.stdin
    statuses = collections.Counter()
    errors = collections.defaultdict(list)
    versions = collections.Counter()
    n = 0
    for line in lines:
        try:
            rec = json.loads(line)
        except ValueError:
            continue
        if args.since and rec.get("ts", "") < args.since:
            continue
        n += 1
        client = rec.get("client") or {}
        resp = rec.get("response") or {}
        status = resp.get("status") or ("transport-error" if rec.get("error") else None)
        statuses[(endpoint(client.get("url", "")), status)] += 1
        for ua in (client.get("headers") or {}).get("User-Agent") or []:
            versions[ua] += 1
        if rec.get("error"):
            errors[f"transport: {rec['error']}"].append(rec)
        elif isinstance(status, int) and status >= 400:
            errors[f"{status} {error_message(resp.get('body'))}"].append(rec)

    print(f"{n} records")
    for (ep, st), c in sorted(statuses.items(), key=lambda kv: -kv[1]):
        print(f"  {ep:<32} {st}  ×{c}")
    print("user agents:")
    for ua, c in versions.most_common():
        print(f"  {ua}  ×{c}")
    if not errors:
        print("no upstream errors")
        return
    print("errors:")
    for msg, recs in sorted(errors.items(), key=lambda kv: -len(kv[1])):
        last = recs[-1]
        up = last.get("upstream") or {}
        cb, ub = betas(last["client"].get("headers")), betas(up.get("headers"))
        print(f"  ×{len(recs)}  {msg}")
        print(f"       last: {last.get('ts')} req_id={last.get('req_id')}")
        if cb - ub:
            print(f"       betas dropped by proxy: {','.join(sorted(cb - ub))}")
        if ub - cb:
            print(f"       betas added by proxy:   {','.join(sorted(ub - cb))}")


if __name__ == "__main__":
    main()
