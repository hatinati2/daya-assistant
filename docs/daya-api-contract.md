# Daya broker API contract

## Confirmed authentication flow

The browser Network trace supplied during project setup showed:

1. `GET https://mg.dayatrader.ir/captcha`
   - Returns a PNG image.
   - Exposes the matching challenge ID in the `captchaId` response header.
   - Sets a `cookiesession1` cookie. The same cookie must be sent with the login request.
2. `POST https://mg.dayatrader.ir/oms-login`
   - JSON fields: `username`, `password`, `captchaAnswer`, and `captchaId`.
   - Header: `x-client-type: WEB_CLIENT`.
   - Browser origin and referer: `https://dayatrader.ir`.
   - A successful response sets an HttpOnly `token` cookie. Its observed lifetime was approximately seven hours.

The implementation fetches the image server-side, displays it in the Daya settings page, and requires the account owner to enter the numeric answer. The challenge cookie and ID are kept together in short-lived server memory. The answer is not persisted. The returned token cookie is encrypted at rest using AES-256-GCM and a separately stored 32-byte key.

## Order submission contract — not yet verified

No authenticated Daya order-submission request, request payload, side enum, validity fields, or success/error response schema has been supplied or verified yet. Therefore:

- The order model, scheduling, batching, persistence, status lifecycle, calendar, and symbol search are implemented independently of the broker.
- Manual execution remains available.
- The API execution choice is intentionally disabled.
- No Daya order endpoint is guessed or called, and no live order is submitted during development or diagnostics.

To enable API dispatch, capture a legitimate order request from the browser Network panel with secrets and personal identifiers redacted. The request URL, method, non-secret headers, JSON field names/types, side/validity values, and success/error response shape are sufficient. Do not share the token cookie, password, account number, or other authentication values.
