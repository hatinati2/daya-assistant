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

## Observed confirmation record

A redacted broker order record supplied by the operator contains `orderFunction=CONFIRMATION`, `orderStatus=BOOKED`, numeric `orderId`, numeric `hostOrderNumber`, and the submitted instrument/price/quantity/side/validity fields. The implementation now recognizes a 2xx confirmation only when the record says `CONFIRMATION` and `BOOKED`, both identifiers are present, and instrument, side, price, quantity, and validity exactly match the local order. Both identifiers are persisted and displayed in order history.

The sample's source endpoint is not yet established: it may be the immediate POST response or a subsequent order-list result. This record therefore improves the response parser but does not by itself prove the POST response contract.

## Two-stage broker acknowledgement now represented

The operator provided a request payload followed by an `ENTRY / SENT_TO_TRADING_ENGINE` record with a numeric `orderId` and no `hostOrderNumber`, then a later `CONFIRMATION / BOOKED` record with the same `orderId` and a numeric `hostOrderNumber`. The request and records share the same instrument, side, price, quantity, and validity fields.

The dispatcher now:
- stores the ENTRY `orderId` as a pending broker reference rather than reporting final success;
- checks read-only `GET /orders` at a randomized 1–7 minute cadence while pending;
- moves to submitted only when a CONFIRMATION/BOOKED record matches the broker `orderId` and every submitted field;
- stores both `orderId` and `hostOrderNumber`;
- never retries the order-creation POST automatically.

The Network label `order` is a browser request name, not proof of the request URL. The full URL and non-secret request headers remain to be verified from the Network Headers panel.

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
