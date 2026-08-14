# Porter observability

Porter uses a layered observability model rather than sending every signal to one vendor. **OpenTelemetry is the instrumentation boundary** for traces, metrics, and future log correlation. **Prometheus and Grafana** are the operational metrics path. **Sentry** is the opt-in exception and frontend error path. **PostHog** is the opt-in product-usage path for the dashboard and public website.

> Production remains disabled by default. No telemetry provider receives data unless its explicit configuration is present.

## Signal ownership

| Signal | Primary path | Examples | Default |
|---|---|---|---|
| Structured application logs | Go `log/slog` and systemd journal | request ID, status, duration, migration error | Enabled at sanitized info level |
| Traces | OpenTelemetry SDK/exporter | HTTP request, PostgreSQL query span, deployment reconciliation | Disabled |
| Metrics | Prometheus exposition or OTel metrics pipeline | request latency, VM lifecycle, replica health, deployment outcomes, DB pool health | Disabled until configured |
| Operational dashboards | Grafana reading Prometheus | host readiness, control-plane health, Firecracker lifecycle | External/local operator choice |
| Exceptions | Sentry Go/Vue SDKs | uncaught API/frontend errors and stack traces | Disabled |
| Product analytics | PostHog JS SDK | navigation, project creation flow, deployment workflow completion | Disabled and consent-controlled |

OpenTelemetry Go supports traces and metrics as stable components and logs as a beta component in its current documentation [1]. Grafana has built-in Prometheus data-source support and PromQL exploration [2]. Sentry’s Vue integration supports explicit initialization and configurable data collection [3]. PostHog’s Vue integration supports event capture, identification, session correlation, and opt-out/reset behavior [4].

## Configuration contract

The Go service should use environment-specific values, never hard-coded provider credentials:

```bash
# Core instrumentation
PORTER_OTEL_ENABLED=false
OTEL_SERVICE_NAME=porter-control-plane
OTEL_EXPORTER_OTLP_ENDPOINT=http://127.0.0.1:4317
OTEL_TRACES_SAMPLER=parentbased_traceidratio
OTEL_TRACES_SAMPLER_ARG=0.1

# Metrics
PORTER_METRICS_ENABLED=false
PORTER_METRICS_LISTEN_ADDR=127.0.0.1:9090

# Error tracking
PORTER_SENTRY_ENABLED=false
SENTRY_DSN=
SENTRY_ENVIRONMENT=development
SENTRY_TRACES_SAMPLE_RATE=0

# Frontend product analytics and error tracking
VITE_POSTHOG_ENABLED=false
VITE_POSTHOG_KEY=
VITE_POSTHOG_HOST=https://app.posthog.com
VITE_SENTRY_ENABLED=false
VITE_SENTRY_DSN=
VITE_SENTRY_ENVIRONMENT=development
```

The dashboard preview may enable provider SDKs only with local development keys. Production should use deployment-secret injection and an explicit consent/configuration decision. The frontend must not send database URLs, bearer tokens, CSRF tokens, environment-variable values, image manifests, rootfs metadata, private IP inventories, or raw log bodies to PostHog or Sentry.

## Metrics to expose

The first operational dashboard should use bounded labels only. Recommended metrics are `porter_http_requests_total`, `porter_http_request_duration_seconds`, `porter_vm_lifecycle_total`, `porter_replicas`, `porter_deployments_total`, `porter_database_pool_acquired`, `porter_database_pool_idle`, `porter_runtime_prerequisites`, and `porter_events_connected`. Project IDs, user IDs, domains, request paths containing secrets, and arbitrary image references should not become unbounded metric labels.

Grafana should read Prometheus rather than query Porter’s PostgreSQL database directly. A local development stack may run Prometheus and Grafana beside the mock API, but neither should be required for the Porter daemon or production installer.

## Correlation

Every HTTP request gets an `X-Request-Id` in development request logging. When OpenTelemetry is enabled, the request span should carry the same correlation context. The frontend can include the request ID in an error report without sending the request payload. PostHog tracing headers, if enabled, must be limited to approved local/API hostnames and must not be copied into public production requests without an explicit privacy review.

## Rollout sequence

The safe order is: structured Go logs; bounded Prometheus metrics; OpenTelemetry request and runtime spans; local Grafana dashboards; opt-in Sentry; and finally consent-controlled PostHog product events. This keeps operational debugging available even when external SaaS integrations are disabled.

## References

[1]: https://opentelemetry.io/docs/languages/go/ "OpenTelemetry Go documentation"
[2]: https://grafana.com/docs/grafana/latest/datasources/prometheus/ "Grafana Prometheus data source documentation"
[3]: https://docs.sentry.io/platforms/javascript/guides/vue/ "Sentry Vue documentation"
[4]: https://posthog.com/docs/libraries/vue-js "PostHog Vue.js documentation"
