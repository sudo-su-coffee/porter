# Analytics implementation notes

Porter already includes the built-in Manus analytics script from the static template, using the injected analytics endpoint and website ID. The final site should keep that traffic instrumentation rather than replacing it.

Google’s official gtag.js guidance confirms that GA4 uses a Google tag loaded from `googletagmanager.com/gtag/js` with a site-specific tag or measurement ID and a `config` call. Because no GA4 measurement ID is present in the current project configuration, the site should support GA4 conditionally through `VITE_GA4_MEASUREMENT_ID` and remain silent when it is unset. A placeholder ID must not be shipped as if it were real.

The project should also emit explicit client-side events for the primary conversion actions (first-deploy CTA, documentation open, source/GitHub click) only when the GA4 hook is active. Built-in traffic analytics remains available independently of GA4.

References:

- https://developers.google.com/tag-platform/gtagjs
- https://developers.google.com/analytics/devguides/collection/ga4/tag-options
