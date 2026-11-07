#!/usr/bin/env python3
"""Extract a sanitized view of Daya trading POST requests from a browser HAR.

Raw HAR files can contain passwords, CAPTCHA answers, cookies, bearer tokens,
account identifiers, symbols, prices, quantities, and order IDs. This tool
never writes a raw copy: it emits only request/response structure and a small
allowlist of protocol enums needed to implement the broker adapter.
"""

from __future__ import annotations

import argparse
import json
import re
import sys
from urllib.parse import parse_qsl, urlencode, urlsplit, urlunsplit


SENSITIVE_KEY = re.compile(
    r"(password|token|authorization|cookie|captcha|account|customer|"
    r"user(name|id)?|national|mobile|phone|email|session|secret|csrf|"
    r"bank|card|portfolio|clientid|personnel)",
    re.IGNORECASE,
)
ENUM_KEYS = {
    "side",
    "orderside",
    "ordertype",
    "validity",
    "validitytype",
    "ordervalidity",
    "ordervaliditytype",
    "orderpaymentgateway",
    "coretype",
    "ordermodeltype",
    "status",
}
ERROR_KEYS = {"message", "msg", "error", "errormessage", "description"}
SAFE_HEADERS = {
    "accept",
    "content-type",
    "x-client-type",
    "origin",
    "referer",
    "user-agent",
}
UUID_RE = re.compile(
    r"^[0-9a-f]{8}-[0-9a-f]{4}-[1-8][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$",
    re.IGNORECASE,
)
ISIN_RE = re.compile(r"\b[A-Z]{2}[A-Z0-9]{9}[0-9]\b", re.IGNORECASE)
LONG_DIGITS_RE = re.compile(r"\b\d{5,}\b")
JWT_RE = re.compile(r"^[A-Za-z0-9_-]{8,}\.[A-Za-z0-9_-]{8,}\.[A-Za-z0-9_-]{8,}$")


def normalized_key(key: str) -> str:
    return re.sub(r"[^a-z0-9]", "", key.lower())


def safe_error_text(value: str) -> str:
    value = ISIN_RE.sub("<ISIN>", value)
    value = LONG_DIGITS_RE.sub("<digits>", value)
    return value[:240]


def sanitize_value(key: str, value):
    normalized = normalized_key(key)

    if SENSITIVE_KEY.search(key) or normalized.endswith("id") and normalized not in ENUM_KEYS:
        return "<redacted>"

    if normalized in {"isin", "instrumentisin", "symbolisin"}:
        return "<ISIN>"
    if normalized in {"symbol", "ticker", "instrumentname", "companyname"}:
        return "<symbol>"
    if normalized in {"price", "orderprice", "quantity", "orderquantity",
                      "disclosedquantity", "volume", "amount", "minquantity",
                      "minimumquantity", "maxshow", "disclosedvolume"}:
        if value is None:
            return None
        return "<number>"

    if normalized in ENUM_KEYS:
        if isinstance(value, (str, int, float, bool)) or value is None:
            return value
        return "<enum>"

    if normalized in ERROR_KEYS and isinstance(value, str):
        return safe_error_text(value)

    if isinstance(value, dict):
        return {str(k): sanitize_value(str(k), v) for k, v in value.items()}
    if isinstance(value, list):
        return [sanitize_value(key, item) for item in value]
    if isinstance(value, bool) or value is None:
        return value
    if isinstance(value, (int, float)):
        return "<number>"
    if isinstance(value, str):
        if JWT_RE.match(value):
            return "<redacted>"
        return "<string>"
    return "<redacted>"


def sanitize_url(raw_url: str) -> str:
    parsed = urlsplit(raw_url)
    segments = parsed.path.split("/")
    cleaned = []
    redact_next = False
    sensitive_path_keys = {
        "account", "accounts", "customer", "customers", "user", "users",
        "client", "clients", "portfolio", "session",
    }
    for segment in segments:
        if redact_next and segment:
            cleaned.append("<id>")
            redact_next = False
            continue
        if segment.lower() in sensitive_path_keys:
            cleaned.append(segment)
            redact_next = True
        elif UUID_RE.match(segment) or LONG_DIGITS_RE.fullmatch(segment) or JWT_RE.match(segment):
            cleaned.append("<id>")
        else:
            cleaned.append(segment)

    query = [
        (key, "<redacted>")
        for key, _ in parse_qsl(parsed.query, keep_blank_values=True)
    ]
    return urlunsplit((parsed.scheme, parsed.netloc, "/".join(cleaned), urlencode(query), ""))


def sanitize_headers(headers) -> dict:
    result = {}
    for header in headers or []:
        name = str(header.get("name", "")).strip().lower()
        if name not in SAFE_HEADERS:
            continue
        value = str(header.get("value", ""))
        if name == "referer":
            value = sanitize_url(value)
        elif name == "user-agent":
            value = "<browser-user-agent>"
        result[name] = value[:300]
    return result


def sanitize_body(post_data):
    if not isinstance(post_data, dict):
        return None

    text = post_data.get("text")
    mime = str(post_data.get("mimeType", "")).lower()
    if isinstance(text, str) and text:
        if "json" in mime or text.lstrip().startswith(("{", "[")):
            try:
                parsed = json.loads(text)
            except json.JSONDecodeError:
                return {"body_type": "invalid_json"}
            if isinstance(parsed, dict):
                return sanitize_value("body", parsed)
            if isinstance(parsed, list):
                return [sanitize_value("item", item) for item in parsed]
            return sanitize_value("body", parsed)

        if "application/x-www-form-urlencoded" in mime:
            fields = {}
            for key, value in parse_qsl(text, keep_blank_values=True):
                fields[key] = sanitize_value(key, value)
            return fields

        return {"body_type": mime or "text", "value": "<non-json body>"}

    params = post_data.get("params")
    if isinstance(params, list) and params:
        return {
            str(item.get("name", "field")): sanitize_value(
                str(item.get("name", "field")), item.get("value")
            )
            for item in params
        }
    return None


def host_is_daya(host: str) -> bool:
    host = host.lower().split(":", 1)[0]
    return host == "dayatrader.ir" or host.endswith(".dayatrader.ir")


def extract_entries(har: dict) -> list[dict]:
    entries = []
    log = har.get("log", {}) if isinstance(har, dict) else {}
    for entry in log.get("entries", []) or []:
        request = entry.get("request", {}) or {}
        method = str(request.get("method", "GET")).upper()
        raw_url = str(request.get("url", ""))
        parsed = urlsplit(raw_url)
        if method not in {"POST", "PUT", "PATCH"} or not host_is_daya(parsed.netloc):
            continue

        response = entry.get("response", {}) or {}
        response_content = response.get("content", {}) or {}
        response_text = response_content.get("text")
        response_body = None
        if isinstance(response_text, str) and response_text:
            try:
                parsed_response = json.loads(response_text)
                response_body = sanitize_value("response", parsed_response)
            except json.JSONDecodeError:
                response_body = {"body_type": "non-json"}

        entries.append({
            "method": method,
            "url": sanitize_url(raw_url),
            "request_headers": sanitize_headers(request.get("headers")),
            "request_payload": sanitize_body(request.get("postData")),
            "response_status": response.get("status"),
            "response_headers": sanitize_headers(response.get("headers")),
            "response_payload": response_body,
        })
    return entries


def main() -> int:
    parser = argparse.ArgumentParser(
        description="Extract sanitized Daya POST/PUT/PATCH requests from a HAR file."
    )
    parser.add_argument("har_file", help="Path to the HAR file exported from browser DevTools")
    args = parser.parse_args()

    try:
        with open(args.har_file, "r", encoding="utf-8") as source:
            har = json.load(source)
    except (OSError, json.JSONDecodeError) as exc:
        print(f"Could not read HAR file: {exc}", file=sys.stderr)
        return 2

    result = {
        "notice": "Sanitized structure only. Raw cookies, tokens, credentials, CAPTCHA values, IDs, symbols, prices and quantities are not included.",
        "entries": extract_entries(har),
    }
    json.dump(result, sys.stdout, ensure_ascii=False, indent=2)
    sys.stdout.write("\n")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
