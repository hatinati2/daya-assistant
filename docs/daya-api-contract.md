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

## Order submission contract — partially observed, not yet approved for live dispatch

The currently available evidence records these fragments:

- Candidate endpoint: `POST https://mg.dayatrader.ir/orders`.
- The local request builder represents these JSON fields: `instrumentId`, `orderPrice`, `orderQuantity`, `orderSide`, `orderValidityDate`, and `orderValidityType`.
- The represented side values are `BUY` and `SELL`; the represented validity type is `JUST_TODAY`.
- A Daya HTTP 400 response has been observed with the group-state rejection message: `امکان ثبت سفارش با توجه به وضعیت گروه وجود ندارد`.

This is not a complete authenticated request/response capture. In particular, the exact required headers, whether the observed session cookies are sufficient for order submission, the accepted enum values, and the successful response schema have not been verified from a successful broker response. The request shape in code is therefore a candidate adapter contract, not proof that the broker will accept it.


## Observed confirmation/order record

A redacted order record supplied during implementation contains these fields:

- `orderFunction: CONFIRMATION`
- `orderStatus: BOOKED`
- `orderId`: broker order identifier
- `hostOrderNumber`: separate host-side order number
- `instrumentId`, `orderPrice`, `orderQuantity`, `orderSide`
- `orderValidityType: JUST_TODAY`
- `orderValidityDate: null`
- `errorCode: null` and `errorText: null`

This establishes the shape of a confirmation/booked order record. It does **not** establish whether the sample came from the immediate `POST /orders` response or from a subsequent read of the order list. The code therefore recognizes it only when it appears in a 2xx dispatch response **and** its instrument, side, price, quantity, and validity exactly match the local order being dispatched. It stores both the broker `orderId` and `hostOrderNumber`. A response that does not match remains ambiguous.

## Current implementation and gate

- The order model, scheduling, batching, persistence, status lifecycle, calendar, and symbol search are implemented independently of the broker.
- Manual execution remains available.
- A server-side dispatcher is present, but `dayaOrderAPICapable()` remains hard-coded to `false`; it exits before loading credentials or making a network request.
- The session monitor performs read-only `GET /orders` checks. It does not submit, modify, or cancel orders.
- No live order has been submitted by the implementation.
- An arbitrary HTTP 2xx must not be treated as a confirmed order. Confirmation requires the verified success response and definitive broker order ID. Ambiguous results must not be automatically retried.

## Fastest safe next step: sanitized browser capture

To finish the contract without sharing credentials or trade details:

1. In Daya, sign in normally. In browser Developer Tools → Network, enable **Preserve log** and select **Fetch/XHR**.
2. Place an ordinary order through Daya's own interface only when you independently intend to place that real order. Do not create a speculative trade for development.
3. Export the Network log as HAR **with content** to a local file. Do not upload or send the raw HAR.
4. From the repository root, run:

   ```bash
   python3 tools/sanitize_daya_har.py /path/to/network.har > daya-order-contract-sanitized.json
   ```

5. Inspect the sanitized file and share only its JSON contents. It is designed to remove cookies, tokens, credentials, account identifiers, order IDs, symbols, prices, quantities, and most personal identifiers while retaining endpoint/method, request field names/types, protocol enums, response status, and response shape.

The sanitizer is a defensive aid, not a guarantee: manually remove any newly introduced sensitive field before sharing. Never send the raw HAR, Cookie, Set-Cookie, Authorization, token, username, password, account number, or customer ID.

Once a successful sanitized response is available, the adapter can be updated to parse the actual broker order ID and response status, with mocked tests for all response classes. Only then can a separate review decide whether to change the live capability gate.
