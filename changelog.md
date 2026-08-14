# Changelog

## Unreleased — dev-min-changes

This branch contains development-only improvements and is intentionally separate from `main` and the production beta release.

The dashboard navigation now separates **Projects**, **Deployments**, and **Replicas**. Project detail uses grouped workspace tabs for operation, observation, and configuration, while existing deep links remain available.

A development preview mode is available for UI work. It uses a clearly labeled synthetic operator session and a single Express fixture server. It must never be enabled in a production build and does not alter backend authentication or RBAC.

The Go service now has an opt-in development observability configuration for structured log levels, request IDs, request timing, and safe HTTP request traces. Production remains disabled by default and sensitive values are not logged.

## Release safety

The installer, PostgreSQL provisioning, Firecracker artifact distribution, guest image paths, migrations, and production authorization behavior are not changed by this branch unless a future entry explicitly says otherwise.

## Observability foundation

Added the opt-in observability contract under `docs/observability/`. OpenTelemetry is defined as the common instrumentation boundary; Prometheus/Grafana own bounded operational metrics; Sentry is reserved for redacted exception tracking; and PostHog is reserved for consent-controlled product analytics. Added a version-controlled Grafana overview dashboard for request rate, p95 latency, replica state, failed deployments, and connected event streams. Production provider integrations remain disabled unless explicitly configured.
