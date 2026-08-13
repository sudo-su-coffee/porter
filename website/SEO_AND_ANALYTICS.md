# Porter final site readiness

## SEO

The static shell now includes a clear page title, description, robots directive, Open Graph metadata, Twitter summary-card metadata, a favicon, a web manifest, JSON-LD software metadata, `robots.txt`, and `sitemap.xml`. The sitemap currently uses the active Manus preview URL. Update the two URLs in `client/public/robots.txt` and `client/public/sitemap.xml` when a permanent custom domain is assigned.

The Getting Started route updates its document title and description on entry and restores the homepage values on exit. It remains a client-rendered route, so the final published domain should be submitted to Google Search Console after launch.

## Analytics and traffic monitoring

The built-in Manus analytics script remains enabled through `VITE_ANALYTICS_ENDPOINT` and `VITE_ANALYTICS_WEBSITE_ID`. This is the current traffic-monitoring path and does not require a Google credential.

Optional GA4 support is implemented in `client/src/lib/analytics.ts`. Set `VITE_GA4_MEASUREMENT_ID` to the real GA4 measurement ID in the project’s secrets/environment settings. The integration remains inactive when the variable is unset; no fake or placeholder measurement ID is included. When active, Porter records page views plus first-deploy CTA clicks, Getting Started opens, and documentation code-copy actions.

The site does not claim to have traffic volume until the analytics property receives real visits. Review traffic in the built-in analytics dashboard and GA4 Realtime reports after publishing.

## Final verification

`pnpm run check` passes. `pnpm run build` passes. The build reports a non-blocking chunk-size warning from the existing dependency surface; the Getting Started page is now lazy-loaded so the initial homepage route can avoid loading the full docs bundle.
