# Daya order API contract — verification status

## Scope and safety

This document records only the order request shape observed in browser traffic and local implementation checks. It is **not** authorization to submit orders. Live order creation remains disabled until the remaining broker contract is independently verified and the dispatch path is separately reviewed.

## Observed request

- Method: `POST`
- URL: `https://mg.dayatrader.ir/orders`
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

The instrument identifier, price, quantity, side, null validity date, and `JUST_TODAY` value are represented in the local payload builder. The observed side values are `BUY` and `SELL`.

## Observed rejection

A browser request returned HTTP 400 from `/orders` with the message:

> امکان ثبت سفارش با توجه به وضعیت گروه وجود ندارد

This proves only that one observed request reached the endpoint and was rejected for a group-state condition. It does not establish the successful response schema.

## Additional observed rejection

On 2026-11-08, a manual order attempt in the Daya panel during the reported closed-group period displayed:

> امكان ثبت سفارش با توجه به وضعيت كروه وجود ندارد

This matches the earlier observed HTTP 400 group-state rejection. The dispatcher now recognizes this Persian rejection text when returned as plain text as well as through the structured JSON error parser. It still does not imply that all requests made outside trading hours are necessarily discarded or that another endpoint cannot queue an order.

## DevTools evidence supplied by the operator

The operator clarified that the supplied fragments came from different panels of the same Network item named `order` (such as Headers, Payload, and Preview). The fragments show a six-field request payload, an `ENTRY / SENT_TO_TRADING_ENGINE` record with an `orderId` and null `hostOrderNumber`, and a `CONFIRMATION / BOOKED` record with an `orderId` and numeric `hostOrderNumber`.

The supplied fragments do not independently establish which exact panel contains each record or whether the BOOKED record came from the POST response or a later order-list request. The Network Name label `order` is not the Request URL. Do not claim that POST and GET phases have been proven until the Request URL, Method, Status, and full Response view are checked on that same Network entry.

The code now supports both record shapes conservatively: an exact matching ENTRY acknowledgement becomes pending, and an exact matching CONFIRMATION/BOOKED record with empty error fields is required for final local confirmation. Pending checks use read-only GET `/orders`; that polling path is an implementation assumption to validate, not a fact established by the partial DevTools snippets.

## Still unverified

- Full required headers and authentication requirements for order creation.
- Whether the observed session cookies alone are sufficient for POST.
- The successful HTTP status and response JSON schema, including the definitive broker order ID.
- All accepted validity types and date semantics.
- Instrument-, account-, session-, group-state-, price-band-, quantity-, and balance-specific constraints.
- Idempotency behavior, duplicate submission handling, and final order-status reconciliation.

## Current implementation facts

- `dayaOrderAPICapable()` intentionally remains `false`; the API execution option is disabled.
- A server-side dispatcher is now wired to the scheduler, but it exits before loading credentials or making any network request while the capability gate is closed.
- The dormant dispatch path builds a POST using the fixed Daya endpoint and allowlisted session cookies only; redirects are not followed.
- Response HTTP status, Daya `requestId` (when returned), response timestamp, and broker message are persisted on the local order and displayed in history.
- A 4xx response is recorded as a rejection; the observed group-state message is normalized for display. A 2xx response is **not** marked as a confirmed order because the success schema and broker order ID are not verified; it becomes `submission_unknown`.
- Network errors, 3xx responses, 5xx responses, and service restarts during submission are treated as ambiguous. Automatic retry is deliberately not performed to avoid duplicate orders.
- An authenticated operator can reconcile `submission_unknown` after manually checking the official Daya panel. A confirmed registration requires a broker order ID; the action is local-only and never resubmits.
- The session monitor continues to perform read-only `GET /orders` checks only.
- The Preview `/daya-api-test` page remains a loopback-only mock and does not use saved broker credentials.

## Test interpretation

Local tests validate request construction, cookie allowlisting, fail-closed gate behavior, and conservative outcome classification. They do not prove broker connectivity, authentication, acceptance, or order placement.

Live order dispatch remains disabled by design.
