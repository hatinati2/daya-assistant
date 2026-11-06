# Daya Assistant

Independent assistant for Daya brokerage.

## Project status

Initial repository scaffold. No production deployment or broker integration is configured yet.

## Principles

- Independent from `rabin-assistant` in code, configuration, data, credentials, and deployment.
- Never place broker credentials or session cookies in source control.
- Keep broker authentication and order execution behind explicit interfaces.
- Do not submit live orders during development or diagnostics.

## Planned components

- Go backend service
- Responsive web interface
- Daya authentication/session adapter
- Order scheduling and lifecycle management
- Separate preview and production configurations

## Development

Implementation instructions will be added as the project structure is established.
