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

- Confirmed in the operator's latest DevTools capture for a newly created order: `POST https://mg.dayatrader.ir/orders`, returning HTTP `200 OK`.
- The local request builder represents these JSON fields: `instrumentId`, `orderPrice`, `orderQuantity`, `orderSide`, `orderValidityDate`, and `orderValidityType`.
- The represented side values are `BUY` and `SELL`; the represented validity type is `JUST_TODAY`.
- A Daya HTTP 400 response has been observed with the group-state rejection message: `امکان ثبت سفارش با توجه به وضعیت گروه وجود ندارد`.

The method, URL, HTTP status, and one actual response body are now confirmed. The captured POST returned HTTP `200 OK` with an `ENTRY / SENT_TO_TRADING_ENGINE` record, a positive broker `orderId`, `hostOrderNumber: null`, and empty `errorCode`/`errorText`. The response echoed the submitted instrument, side, price, quantity, and validity values. This proves that this particular request reached the trading engine and received an ENTRY acknowledgement; it does not prove final BOOKED confirmation or a trade execution. The exact required headers, whether the observed session cookies are sufficient in general, and the full accepted enum set remain unverified.


## Meaning of BOOKED versus execution

The latest observed `GET /orders` Preview contains a `CONFIRMATION / BOOKED` row with `matchedQuantity: 0`, `tradedQuantity: 0`, `tradedPrice: 0`, and `remainingQuantityOfTheOrder` equal to the submitted quantity. BOOKED means the order is registered in the order book; execution happens later only when a counterparty order matches. The implementation checks for a matching BOOKED record only while an ENTRY acknowledgement is pending. Once BOOKED is confirmed, tracking stops; execution and fill progress are outside scope.

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

The operator clarified why the broker `orderId` differs between the ENTRY and BOOKED examples: several test orders were deleted and re-created, so the two samples represent different orders. This is expected and is not evidence of a broken lifecycle. They must not be linked to one another. For an individual order, ENTRY-to-BOOKED reconciliation still requires the same broker `orderId` and matching instrument, side, price, quantity, and validity.

## Observed order-list request

The operator has now separately selected the `order` resource in the Daya interface and confirmed:

- `GET https://mg.dayatrader.ir/orders`
- HTTP `200 OK`

This confirms that `GET /orders` is an actual reachable order-resource request in the browser, not a guessed path. Its Preview contains an array with a `CONFIRMATION / BOOKED` record. The record includes `matchedQuantity`, `tradedQuantity`, `tradedPrice`, and `remainingQuantityOfTheOrder`; the observed sample has zero matched/traded quantity and the full order quantity remaining.

## Observed read-only order-list request headers

The latest operator DevTools capture for the separate read-only `GET /orders` request showed these header names/values (credential values are intentionally not recorded):

- `Accept: application/json`
- `Content-Type: application/json`
- `Origin: https://dayatrader.ir`
- `Referer: https://dayatrader.ir/`
- `x-client-type: WEB_CLIENT`
- The `Cookie` header contained the cookie names `cookiesession1` and `token`; cookie values are excluded.

This confirms the browser's observed headers for the read-only GET request only. It is useful corroborating evidence for the shared Daya session and client headers, but it does **not** prove that every one of these headers is required or sufficient for `POST /orders`. The POST authentication/header contract remains unverified until the corresponding POST request's headers are captured safely. No cookie or token values are stored in this repository.

## Observed post-submit browser requests

For the captured order submission, the operator observed this subsequent browser request sequence:

1. `POST /orders` — returns the `ENTRY / SENT_TO_TRADING_ENGINE` record documented above.
2. `GET /financials/buying-power` — HTTP 200; a buying-power refresh.
3. `GET /market-data/market-view/trading-view/{instrumentId}/tradeVolumeAndCountOnCustomerType/on-minutes` — HTTP 200; a minute-interval market-data request for trade volume/count by customer type.

Neither of the two observed follow-up GET requests is an order-list or final order-confirmation endpoint. This sequence does not establish where the earlier `CONFIRMATION / BOOKED` sample originated. A separate order-history/list view or another distinct request must be inspected before assigning that record to a specific endpoint. Do not infer that the browser automatically calls `GET /orders` after this POST.

## DevTools evidence supplied by the operator

The operator clarified that the supplied fragments came from different panels of the same Network item named `order` (for example, Headers, Payload, and Preview). The snippets include:

- A request payload with `instrumentId`, `orderPrice`, `orderQuantity`, `orderSide`, `orderValidityDate`, and `orderValidityType`.
- An order record with `orderFunction: ENTRY`, `orderStatus: SENT_TO_TRADING_ENGINE`, a numeric `orderId`, and `hostOrderNumber: null`.
- A confirmation-shaped record was also supplied earlier with `orderFunction: CONFIRMATION`, `orderStatus: BOOKED`, a numeric `orderId`, and a numeric `hostOrderNumber`.

The latest full Response panel for the POST confirms an ENTRY / SENT_TO_TRADING_ENGINE record. The ENTRY and BOOKED examples have different broker order IDs because several test orders were deleted and re-created between captures; they are separate orders and must not be correlated. The separate GET /orders capture independently confirms the BOOKED record shape.

The code recognizes a matching ENTRY acknowledgement as `submission_pending` and a matching CONFIRMATION/BOOKED row as order-book registration. Matching requires the same broker `orderId`; the differing IDs in the supplied examples are expected because they came from different re-created orders. After BOOKED, no further polling or execution tracking is performed. The broker order ID is used only to confirm the pending ENTRY acknowledgement.

## Current implementation and gate

- The order model, scheduling, batching, persistence, status lifecycle, calendar, and symbol search are implemented independently of the broker.
- Manual execution remains available.
- A server-side dispatcher is present, but `dayaOrderAPICapable()` remains hard-coded to `false`; it exits before loading credentials or making a network request.
- The confirmation check performs read-only `GET /orders` only while an ENTRY acknowledgement is pending. It does not submit, modify, or cancel orders.
- No live order has been submitted by the implementation.
- A matching ENTRY HTTP 2xx is recorded as pending, not confirmed or executed. A matching CONFIRMATION/BOOKED record is required for local registration. The read-only GET /orders check stops after registration is confirmed. An arbitrary or unrecognized HTTP 2xx remains ambiguous. Ambiguous results must not be automatically retried.

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

The ENTRY response shape and the separate read-only GET /orders BOOKED shape are captured and supported by the conservative adapter. The adapter stops checking after BOOKED confirmation. Complete POST authentication/header requirements remain to be verified before any separate review of the live capability gate.
