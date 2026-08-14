/**
 * Harbor Glass reminder: analytics should stay invisible, optional, and
 * respectful. Measure the product path without making the interface noisy.
 */
type AnalyticsValue = string | number | boolean | null;
type AnalyticsParams = Record<string, AnalyticsValue | undefined>;
type Gtag = (...args: unknown[]) => void;

declare global {
  interface Window {
    dataLayer: unknown[];
    gtag?: Gtag;
  }
}

let initialized = false;

function measurementId() {
  return (import.meta.env.VITE_GA4_MEASUREMENT_ID as string | undefined)?.trim();
}

export function initGA4() {
  const id = measurementId();
  if (!id || initialized || typeof window === "undefined") return;

  window.dataLayer = window.dataLayer || [];
  window.gtag = window.gtag || ((...args: unknown[]) => window.dataLayer.push(args));

  if (!document.querySelector(`script[data-porter-ga4="${id}"]`)) {
    const script = document.createElement("script");
    script.async = true;
    script.src = `https://www.googletagmanager.com/gtag/js?id=${encodeURIComponent(id)}`;
    script.dataset.porterGa4 = id;
    document.head.appendChild(script);
  }

  window.gtag("js", new Date());
  window.gtag("config", id, { send_page_view: false });
  initialized = true;
}

export function trackPageView(pageTitle: string) {
  if (typeof window === "undefined") return;
  initGA4();
  if (!window.gtag || !measurementId()) return;
  window.gtag("event", "page_view", {
    page_title: pageTitle,
    page_location: window.location.href,
    page_path: window.location.pathname,
  });
}

export function trackEvent(name: string, params: AnalyticsParams = {}) {
  if (typeof window === "undefined" || !measurementId()) return;
  initGA4();
  if (!window.gtag) return;
  window.gtag("event", name, Object.fromEntries(Object.entries(params).filter(([, value]) => value !== undefined)));
}

export function trackConversion(name: string, params: AnalyticsParams = {}) {
  trackEvent(name, { ...params, conversion: true });
}
