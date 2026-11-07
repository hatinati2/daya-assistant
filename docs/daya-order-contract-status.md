# Daya order API contract — verification status

## Scope and safety

This document records only the order request shape observed in browser traffic and the local tests in this repository. It is **not** authorization to submit orders. The application must not send live order-creation requests until the remaining contract is independently verified and the dispatch implementation is separately reviewed.

## Observed request

- Method: `POST`
- URL: `https://mg.dayatrader.ir/orders`
- Content type observed in the local request construction: `application/json`
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

The instrument identifier, price, quantity, side, null validity date, and `JUST_TODAY` value are covered by local payload tests. The observed side values are `BUY` and `SELL`.

## Observed rejection

A browser request returned HTTP 400 from `/orders` with the message:

> امکان ثبت سفارش با توجه به وضعیت گروه وجود ندارد

This is evidence that a request reached the endpoint and was rejected for a group-state condition in that observed session. It is not evidence that a valid order can be accepted, nor does it establish the success response schema.

## Still unverified

- Full required headers and authentication requirements for order creation.
- Whether cookies, bearer authorization, or both are required for POST.
- The successful HTTP status and response JSON schema.
- All accepted validity types and date semantics.
- Instrument-specific, account-specific, market-session, group-state, price-band, quantity, and balance constraints.
- Idempotency behavior, duplicate submission handling, and final order-status reconciliation.

## Current implementation facts

- `dayaOrderAPICapable()` intentionally returns `false`.
- The API execution option therefore remains disabled.
- The `POST /orders` panel handler validates and stores a scheduled order locally; it does not dispatch to the broker.
- The session monitor performs read-only `GET /orders` checks only.
- The Preview `/daya-api-test` page starts an ephemeral loopback mock and sends only to that local mock. It does not use saved broker credentials.
- The local test transport rejects non-loopback destinations.

## Test interpretation

A local mock HTTP 201 proves that the local JSON builder, `POST /orders` path, and HTTP response handling work against the mock server. It does not test broker connectivity, authentication, acceptance, or order placement.

Live order dispatch remains disabled by design.
