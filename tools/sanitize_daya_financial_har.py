#!/usr/bin/env python3
"""Print schema-only diagnostics for read-only Daya financial GET responses.

The output contains endpoint paths, HTTP status, and safe JSON property paths
with value types. No response values, cookies, request headers, IDs, or raw
response bodies are emitted.
"""
from __future__ import annotations

import argparse
import json
import re
import sys
from urllib.parse import urlsplit

HOSTS = {"dayatrader.ir", "mg.dayatrader.ir"}
SENSITIVE_PARTS = (
    "password", "token", "authorization", "cookie", "captcha",
    "account", "customer", "username", "userid", "national", "mobile",
    "phone", "email", "session", "secret", "csrf", "bank", "card",
    "client", "personnel", "identity", "birth", "firstname", "lastname",
    "fullname", "address", "instrumentid", "orderid", "brokerid",
    "isin", "symbol",
)
UUID_RE = re.compile(
    r"^[0-9a-f]{8}-[0-9a-f]{4}-[1-8][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$",
    re.IGNORECASE,
)
LONG_DIGITS_RE = re.compile(r"^\d{5,}$")
MAX_ENTRIES = 100
MAX_FIELDS = 120
MAX_DEPTH = 10
MAX_ARRAY_SAMPLE = 3
MAX_RESPONSE_BYTES = 2 * 1024 * 1024


def normalized_key(key: str) -> str:
    return re.sub(r"[^a-z0-9]", "", key.lower())


def safe_key(key: str) -> bool:
    normalized = normalized_key(key)
    if not normalized or len(key) > 64:
        return False
    return not any(part in normalized for part in SENSITIVE_PARTS) and normalized != "id" and not normalized.endswith("id")


def safe_path(raw_url: str) -> str:
    parsed = urlsplit(raw_url)
    parts = []
    for segment in parsed.path.split("/"):
        if UUID_RE.fullmatch(segment) or LONG_DIGITS_RE.fullmatch(segment):
            parts.append("<id>")
        else:
            parts.append(segment[:80])
    return "/".join(parts) or "/"


def json_type(value) -> str:
    if value is None:
        return "null"
    if isinstance(value, bool):
        return "boolean"
    if isinstance(value, (int, float)):
        return "number"
    if isinstance(value, str):
        return "string"
    if isinstance(value, dict):
        return "object"
    if isinstance(value, list):
        return "array"
    return "other"


def collect_schema(value, path: str, out: set[str], depth: int = 0) -> None:
    if depth > MAX_DEPTH or len(out) >= MAX_FIELDS:
        return
    if isinstance(value, dict):
        for key in sorted(value):
            if len(out) >= MAX_FIELDS or not safe_key(str(key)):
                continue
            child = value[key]
            child_path = f"{path}.{key}" if path else str(key)
            kind = json_type(child)
            out.add(f"{child_path}:{kind}")
            if isinstance(child, dict):
                collect_schema(child, child_path, out, depth + 1)
            elif isinstance(child, list):
                for item in child[:MAX_ARRAY_SAMPLE]:
                    collect_schema(item, child_path + "[]", out, depth + 1)
    elif isinstance(value, list):
        for item in value[:MAX_ARRAY_SAMPLE]:
            collect_schema(item, path + "[]", out, depth + 1)


def extract_entries(har: dict) -> list[dict]:
    entries = []
    log = har.get("log", {}) if isinstance(har, dict) else {}
    for entry in log.get("entries", []) or []:
        request = entry.get("request", {}) or {}
        if str(request.get("method", "")).upper() != "GET":
            continue
        raw_url = str(request.get("url", ""))
        parsed = urlsplit(raw_url)
        if parsed.hostname not in HOSTS:
            continue
        response = entry.get("response", {}) or {}
        response_content = response.get("content", {}) or {}
        text = response_content.get("text")
        if not isinstance(text, str) or not text or len(text.encode("utf-8", errors="ignore")) > MAX_RESPONSE_BYTES:
            continue
        try:
            payload = json.loads(text)
        except json.JSONDecodeError:
            continue

        schema: set[str] = set()
        collect_schema(payload, "", schema)
        entries.append({
            "method": "GET",
            "path": safe_path(raw_url),
            "status": response.get("status"),
            "schema": sorted(schema)[:MAX_FIELDS],
        })
        if len(entries) >= MAX_ENTRIES:
            break
    return entries


def main() -> int:
    parser = argparse.ArgumentParser(
        description="Extract schema-only diagnostics from Daya read-only GET responses."
    )
    parser.add_argument("har_file", help="Local HAR file exported from browser DevTools")
    args = parser.parse_args()
    try:
        with open(args.har_file, "r", encoding="utf-8") as source:
            har = json.load(source)
    except (OSError, json.JSONDecodeError) as exc:
        print(f"Could not read HAR file: {exc}", file=sys.stderr)
        return 2

    result = {
        "notice": "Schema only: no values, cookies, headers, identifiers, or raw response bodies.",
        "entries": extract_entries(har),
    }
    json.dump(result, sys.stdout, ensure_ascii=False, indent=2)
    sys.stdout.write("\n")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
