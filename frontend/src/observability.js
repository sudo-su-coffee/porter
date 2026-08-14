import * as Sentry from "@sentry/vue";

const enabled = import.meta.env.VITE_SENTRY_ENABLED === "true";
const dsn = import.meta.env.VITE_SENTRY_DSN || "";
let active = false;

export function initSentry(app, router) {
  if (!enabled || !dsn) return false;
  Sentry.init({
    app,
    router,
    dsn,
    environment: import.meta.env.VITE_SENTRY_ENVIRONMENT || "development",
    release: import.meta.env.VITE_APP_VERSION || undefined,
    sendDefaultPii: false,
    tracesSampleRate: 0,
    beforeSend(event) {
      delete event.request;
      delete event.user;
      event.breadcrumbs = [];
      return event;
    },
  });
  active = true;
  return true;
}

export function captureApiFailure(error, { path, method, status, requestId } = {}) {
  if (!active || !error || status < 500) return;
  const pathname = path ? new URL(path, window.location.origin).pathname : "unknown";
  Sentry.withScope((scope) => {
    scope.setTag("porter.method", method || "GET");
    scope.setTag("porter.status", String(status || "unknown"));
    if (requestId) scope.setTag("porter.request_id", requestId);
    scope.setExtra("porter.path", pathname);
    Sentry.captureException(error);
  });
}

export function captureException(error) {
  if (active && error) Sentry.captureException(error);
}
