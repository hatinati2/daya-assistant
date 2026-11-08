# Daya order API contract — verification status

## Scope and safety

This document records only the order request shape observed in browser traffic and local implementation checks. It is **not** authorization to submit orders. Live order creation remains disabled until the remaining broker contract is independently verified and the dispatch path is separately reviewed.

## Observed request

- Method: `POST` (confirmed in the latest operator DevTools capture)
- URL: `https://mg.dayatrader.ir/orders` (confirmed in the same capture)
- Latest captured HTTP status: `200 OK`
- Content type observed in local request construction: `application/json`
- Observed JSON fields:

```json
{
  "instrumentId": "IRO3SIMC0001",
  "orderPrice": 63390,
  "orderQuantity": 300,
  "orderSide": "BUY",
  "orderValidityDate": null,
  "orderValidityType": "JUST_TODAY"
}
```

The instrument identifier, price, quantity, side, null validity date, and `JUST_TODAY` value are represented in the local payload builder. The observed side values are `BUY` and `SELL`. The latest full Response panel for this POST returned an `ENTRY / SENT_TO_TRADING_ENGINE` record, with a positive `orderId`, null `hostOrderNumber`, and null `errorCode`/`errorText`; echoed order fields matched that request. This is an observed engine-entry acknowledgement, not a BOOKED confirmation and not evidence that any quantity has traded.

## Observed rejection

A browser request returned HTTP 400 from `/orders` with the message:

> امکان ثبت سفارش با توجه به وضعیت گروه وجود ندارد

This proves only that one observed request reached the endpoint and was rejected for a group-state condition. It does not establish the successful response schema.

## Additional observed rejection

On 2026-11-08, a manual order attempt in the Daya panel during the reported closed-group period displayed:

> امكان ثبت سفارش با توجه به وضعيت كروه وجود ندارد

This matches the earlier observed HTTP 400 group-state rejection. The dispatcher now recognizes this Persian rejection text when returned as plain text as well as through the structured JSON error parser. It still does not imply that all requests made outside trading hours are necessarily discarded or that another endpoint cannot queue an order.

## Separately observed order-list request

The operator has now confirmed a separate browser request:

- `GET https://mg.dayatrader.ir/orders` — HTTP `200 OK`.

The GET Preview is an array containing a `CONFIRMATION / BOOKED` record. The observed row includes `matchedQuantity`, `tradedQuantity`, `tradedPrice`, and `remainingQuantityOfTheOrder`; the sample shows zero matched/traded quantity and the full order quantity remaining.

## Observed read-only order-list request headers

The latest operator DevTools capture for the separate read-only `GET /orders` request showed these header names/values (credential values are intentionally not recorded):

- `Accept: application/json`
- `Content-Type: application/json`
- `Origin: https://dayatrader.ir`
- `Referer: https://dayatrader.ir/`
- `x-client-type: WEB_CLIENT`
- The `Cookie` header contained the cookie names `cookiesession1` and `token`; cookie values are excluded.

This confirms the browser's observed headers for the read-only GET request only. It is useful corroborating evidence for the shared Daya session and client headers, but it does **not** prove that every one of these headers is required or sufficient for `POST /orders`. The POST authentication/header contract remains unverified until the corresponding POST request's headers are captured safely. No cookie or token values are stored in this repository.

## Observed follow-up requests after order submission

The operator reported this browser Network sequence after the captured `POST /orders`:

- `GET https://mg.dayatrader.ir/financials/buying-power` — HTTP 200.
- `GET https://mg.dayatrader.ir/market-data/market-view/trading-view/{instrumentId}/tradeVolumeAndCountOnCustomerType/on-minutes` — HTTP 200.

These are respectively a buying-power refresh and market-data request by endpoint path. Neither is an order-list endpoint or proof of final booking. The observed sequence does not prove that the browser issues `GET /orders` after submission; the source of the separate earlier `CONFIRMATION / BOOKED` sample remains to be identified.

## DevTools evidence supplied by the operator

The operator clarified that the supplied fragments came from different panels of the same Network item named `order` (such as Headers, Payload, and Preview). The fragments show a six-field request payload, an `ENTRY / SENT_TO_TRADING_ENGINE` record with an `orderId` and null `hostOrderNumber`, and a `CONFIRMATION / BOOKED` record with an `orderId` and numeric `hostOrderNumber`.

The latest full Response view establishes that the POST response is ENTRY / SENT_TO_TRADING_ENGINE. The ENTRY and BOOKED samples have different broker order IDs because the operator deleted and re-created several test orders between captures; they are separate orders and should not be correlated. The separate GET /orders capture independently confirms the BOOKED record shape.

The code supports the captured ENTRY response conservatively: an exact matching ENTRY acknowledgement with empty error fields becomes `submission_pending`. An exact matching CONFIRMATION/BOOKED record with empty error fields and the same broker `orderId` confirms order-book registration. The read-only confirmation check is used only to resolve ENTRY into BOOKED. It stops once BOOKED is confirmed. ENTRY and BOOKED samples with different broker IDs remain separate and are never correlated.

## BOOKED is registration, not execution

The operator clarified that `BOOKED` means the order is registered in the order book. This project stops tracking the order at that point; matching and execution are explicitly out of scope.

## Still unverified

- Full required headers and authentication requirements for order creation.
- Whether the observed session cookies alone are sufficient for POST.

- All accepted validity types and date semantics.
- Instrument-, account-, session-, group-state-, price-band-, quantity-, and balance-specific constraints.
- Idempotency behavior, duplicate submission handling, and final order-status reconciliation.

## Current implementation facts

- `dayaOrderAPICapable()` intentionally remains `false`; the API execution option is disabled.
- A server-side dispatcher is now wired to the scheduler, but it exits before loading credentials or making any network request while the capability gate is closed.
- The dormant dispatch path builds a POST using the fixed Daya endpoint and allowlisted session cookies only; redirects are not followed.
- Response HTTP status, Daya `requestId` (when returned), response timestamp, and broker message are persisted on the local order and displayed in history.
- A 4xx response is recorded as a rejection; the observed group-state message is normalized for display. A 2xx response containing an exact matching ENTRY / SENT_TO_TRADING_ENGINE record with an order ID and empty error fields becomes `submission_pending`; it is not marked as booked or executed. A matching CONFIRMATION/BOOKED record with the same broker order ID confirms registration. The read-only GET /orders check stops after this confirmation; execution tracking is out of scope. Other unrecognized 2xx responses remain `submission_unknown`.
- Network errors, 3xx responses, 5xx responses, and service restarts during submission are treated as ambiguous. Automatic retry is deliberately not performed to avoid duplicate orders.
- An authenticated operator can reconcile `submission_unknown` after manually checking the official Daya panel. A confirmed registration requires a broker order ID; the action is local-only and never resubmits.
- The read-only `GET /orders` confirmation check runs only for pending ENTRY acknowledgements and stops at BOOKED.
- The Preview `/daya-api-test` page remains a loopback-only mock and does not use saved broker credentials.

## Test interpretation

Local tests validate request construction, cookie allowlisting, fail-closed gate behavior, and conservative outcome classification. They do not prove broker connectivity, authentication, acceptance, or order placement.

Live order dispatch remains disabled by design.
