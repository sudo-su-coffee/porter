# Dashboard development preview

The dashboard can be inspected without a running Firecracker host by using the development-only Express fixture server. This mode is intended for navigation and UX work; it does not represent production data.

From the repository root:

```bash
cd frontend
pnpm install
pnpm run mock
```

In a second terminal:

```bash
cd frontend
pnpm run dev:preview
```

Open the Vite URL printed by the command. The dashboard displays a **DEVELOPMENT PREVIEW** badge, uses a synthetic operator session, and reads representative JSON from `scripts/frontend/mock-server.mjs`. The production router guard and backend authorization code are not bypassed or changed by this preview.

## Go development observability

The Go daemon keeps production logging at its normal sanitized level by default. For local debugging, use a development-only TOML section or environment overrides:

```toml
[development]
enabled = true
log_level = "debug"
request_logging = true
api_errors = true
```

Equivalent environment variables are:

```bash
PORTER_DEV_ENABLED=true \
PORTER_LOG_LEVEL=debug \
PORTER_REQUEST_LOGGING=true \
PORTER_API_ERRORS=true \
PORTER_CONFIG=porter.toml \
go run ./cmd/porter
```

When request logging is enabled, the daemon emits structured records with a request ID, method, path, status, and duration. The request ID is also returned as `X-Request-Id` so a failed dashboard action can be correlated with the Go log. Passwords, database URLs, bearer tokens, CSRF tokens, authorization headers, private keys, and guest filesystem contents must never be logged.
