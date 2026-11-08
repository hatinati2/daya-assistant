# Daya Assistant architecture

## Isolation boundary

Daya Assistant is independent from Rabin Assistant in repository, process, configuration, credentials, session storage, order data, and deployment. It must never import, deploy, restart, or modify Rabin.

## Application modules

- `cmd/server`: HTTP server, Persian responsive UI, scheduling, and request handlers.
- `internal/jalaali`: local Solar Hijri date conversion and formatting.
- `daya_credentials.go`: AES-256-GCM encrypted account credentials and session cookie storage.
- `daya_login.go`: Daya-only CAPTCHA and OMS login adapter.
- `daya_order.go`: broker order capability gate; stays disabled until the Daya order contract is verified.
- `daya_order_batch.go`, `daya_order_plan.go`, `daya_order_store.go`: broker-independent order planning and lifecycle functions.
- `symbol_catalog.go`: TSE symbol catalog and exact symbol/ISIN matching.

## Security and operations

- The Daya CAPTCHA image and challenge ID are fetched together; its session cookie is held server-side in short-lived memory.
- CAPTCHA answer is manually entered and is not stored.
- Daya's `token` and CAPTCHA `cookiesession1` cookies are encrypted together with AES-256-GCM using a separate 32-byte key file. Session-store v2 remains backward-compatible with encrypted v1 token-only records.
- The session monitor sends only a read-only `GET /orders` around the clock at a cryptographically randomized 1–7 minute interval, and only stores allowlisted cookies returned by Daya through `Set-Cookie`. It stops at the known session expiry unless Daya returns a valid rotated cookie; polling does not bypass a hard token expiry.
- A successful GET is not treated as proof of token renewal. Expiry is extended only when a refreshed token cookie includes a new Max-Age/Expires value or a later JWT expiration.
- HTTP 401/403 stops monitoring and requires a new manual CAPTCHA login. HTTP 429 stops monitoring without aggressive retries.
- Passwords, cookies, tokens, and CAPTCHA answers must never be committed to Git or printed in logs.
- Manual order workflow is available. Live API dispatch remains disabled until Daya's actual order request/response contract has been captured and tested with mock HTTP tests.
- CI and automated tests never call the live order-creation endpoint. A manual Preview-only online test can issue a read-only `GET /orders`; its separate one-shot form sends exactly one real `POST /orders` only after explicit operator confirmation. That one-shot order is not scheduled and is never automatically retried.
- Preview and production must have separate configs, keys, data directories, and systemd units.
