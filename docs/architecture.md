# Daya Assistant — initial architecture

## Isolation boundary

Daya Assistant is a separate application and repository. It must not import, deploy, restart, or modify Rabin Assistant. It will use its own service account, systemd unit, port, environment file, credentials, session store, and order data directory.

## Initial runtime

- Language: Go
- HTTP server: Go standard library
- Default listen address: `:8100`
- Health endpoint: `GET /healthz`
- UI placeholder: `GET /`

## Planned modules

- `internal/daya`: Daya HTTP client and API contracts
- `internal/auth`: authentication lifecycle and secure session handling
- `internal/orders`: order model, validation, scheduling, and state transitions
- `internal/store`: persistent storage with atomic writes and backups
- `internal/web`: HTTP handlers and UI assets

## Safety and operational boundaries

- No broker credentials, cookies, access tokens, or CAPTCHA answers in Git.
- No live order submission in initial scaffold.
- Authentication diagnostics must not create or submit orders.
- Preview and production must use separate configuration and data paths.
- Any future deployment must target Daya-only paths and a Daya-only systemd service.
