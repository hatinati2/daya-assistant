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
- A 4xx response is recorded as a definite rejection. A 2xx response is **not** marked as a confirmed order because the success schema is not verified; it becomes `submission_unknown`.
- Network errors, 3xx responses, 5xx responses, and service restarts during submission are treated as ambiguous. Automatic retry is deliberately not performed to avoid duplicate orders.
- The session monitor continues to perform read-only `GET /orders` checks only.
- The Preview `/daya-api-test` page remains a loopback-only mock and does not use saved broker credentials.

## Test interpretation

Local tests validate request construction, cookie allowlisting, fail-closed gate behavior, and conservative outcome classification. They do not prove broker connectivity, authentication, acceptance, or order placement.

Live order dispatch remains disabled by design.
