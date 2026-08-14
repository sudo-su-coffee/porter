# Porter observability

Porter uses a layered observability model rather than sending every signal to one vendor. **OpenTelemetry is the instrumentation boundary** for traces, metrics, and future log correlation. **Prometheus and Grafana** are the operational metrics path. **Sentry** is the opt-in exception and frontend error path. **PostHog** is the opt-in product-usage path for the dashboard and public website.

> Production remains disabled by default. No telemetry provider receives data unless its explicit configuration is present.

## Signal ownership

| Signal | Primary path | Examples | Default |
|---|---|---|---|
| Structured application logs | Go `log/slog` and systemd journal | request ID, status, duration, migration error | Enabled at sanitized info level |
| Traces | OpenTelemetry SDK/exporter | HTTP request and PostgreSQL query spans with request-ID correlation | Disabled |
| Metrics | Prometheus exposition at the control-plane `/metrics` route | bounded request rate, latency, status class, and 5xx counters | Disabled until configured |
| Operational dashboards | Grafana reading Prometheus | host readiness, control-plane health, Firecracker lifecycle | External/local operator choice |
| Exceptions | Sentry Go/Vue SDKs | Go panics, Vue exceptions, and sanitized 5xx API failures | Disabled |

OpenTelemetry Go supports traces and metrics as stable components and logs as a beta component in its current documentation [1]. Grafana has built-in Prometheus data-source support and PromQL exploration [2]. Sentry’s Vue and Go integrations support explicit initialization and configurable data collection [3] [4].

## Configuration contract

The Go service should use environment-specific values, never hard-coded provider credentials:

```bash
# Core instrumentation
PORTER_OTEL_ENABLED=false
PORTER_OTEL_SERVICE_NAME=porter-control-plane
OTEL_SERVICE_NAME=porter-control-plane
OTEL_EXPORTER_OTLP_ENDPOINT=http://127.0.0.1:4317
OTEL_TRACES_SAMPLER=parentbased_traceidratio
OTEL_TRACES_SAMPLER_ARG=0.1

# Metrics
PORTER_METRICS_ENABLED=false

# Error tracking
PORTER_SENTRY_ENABLED=false
PORTER_SENTRY_DSN=
PORTER_SENTRY_ENVIRONMENT=development

# Frontend error tracking
VITE_SENTRY_ENABLED=false
VITE_SENTRY_DSN=
VITE_SENTRY_ENVIRONMENT=development
```

The dashboard preview may enable provider SDKs only with local development keys. Production should use deployment-secret injection and an explicit consent/configuration decision. The frontend must not send database URLs, bearer tokens, CSRF tokens, environment-variable values, image manifests, rootfs metadata, private IP inventories, or raw log bodies to PostHog or Sentry.

## Metrics to expose

The first operational dashboard uses bounded labels only. The implemented HTTP metrics are `porter_http_requests_total`, `porter_http_requests_by_method_total`, `porter_http_responses_by_status_class_total`, `porter_http_server_errors_total`, and the request duration sum/count pair. Project IDs, user IDs, domains, request paths containing secrets, and arbitrary image references should not become unbounded metric labels. VM lifecycle and replica-health metrics can be added later using the same bounded-label rule.

Grafana should read Prometheus rather than query Porter’s PostgreSQL database directly. Prometheus should scrape the same control-plane address at `/metrics` when `PORTER_METRICS_ENABLED=true`. A local development stack may run Prometheus and Grafana beside the mock API, but neither should be required for the Porter daemon or production installer.

## Correlation

Every HTTP request gets an `X-Request-Id` in development request logging. When OpenTelemetry is enabled, the HTTP span and pgx query spans carry the same correlation context. The Vue client can include only the response request ID in a Sentry scope without sending the request payload.

## Rollout sequence

The implemented order is: structured Go logs; bounded Prometheus metrics; OpenTelemetry HTTP and PostgreSQL spans; Grafana dashboard queries over those metrics; and opt-in Sentry for actionable Go/Vue errors. This keeps operational debugging available even when external SaaS integrations are disabled. PostHog is intentionally not part of this minimal stack.

## References

[1]: https://opentelemetry.io/docs/languages/go/ "OpenTelemetry Go documentation"
[2]: https://grafana.com/docs/grafana/latest/datasources/prometheus/ "Grafana Prometheus data source documentation"
[3]: https://docs.sentry.io/platforms/javascript/guides/vue/ "Sentry Vue documentation"
[4]: https://docs.sentry.io/platforms/go/ "Sentry Go documentation"
