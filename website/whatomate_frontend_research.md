# Whatomate frontend review

Source repository: [shridarpatil/whatomate](https://github.com/shridarpatil/whatomate)

The public repository’s frontend is a Vue 3 + TypeScript + Vite application using Vue Router, Pinia, TanStack Vue Query, Axios, Vue Flow, Chart.js, and a reusable component system. Its latest inspected commit is `2e05458` on `main`.

## Reusable Porter patterns

| Whatomate pattern | Porter adaptation |
| --- | --- |
| `AppLayout.vue` with mobile drawer, collapsible desktop sidebar, active route state, skip link, and user menu | Reuse the interaction model for a Porter control-plane dashboard, with Porter routes and runtime states instead of WhatsApp sections. |
| `router/index.ts` with lazy-loaded views, auth metadata, permission checks, and first-accessible-route fallback | Adapt for Porter projects, deployments, replicas, images, logs, metrics, settings, and API keys. |
| `stores/auth.ts` and Axios API service boundary | Map to Porter auth/session endpoints and keep the backend URL configurable. |
| `services/websocket.ts` reconnect/auth lifecycle | Reuse only for Porter deployment, replica-health, build, and log events. |
| Vue Flow and Chart.js dependencies | Reuse for deployment graphs, replica topology, health history, and resource charts where the Porter API exposes the required data. |

## Exclusions

WhatsApp accounts, contacts, conversations, campaigns, calling, IVR, chatbot flows, WhatsApp permissions, Meta insights, WhatsApp-specific websocket payloads, and WhatsApp-specific API types are not part of Porter. They should not be copied into the Porter frontend.

## Adaptation boundary

The active Manus marketing project is React-based, while Whatomate’s frontend is Vue-based. Copying the Vue application wholesale would require replacing the managed React scaffold and would increase migration risk. The safer approach is to preserve the Whatomate information architecture and interaction patterns, then implement Porter views in the existing frontend stack unless a separate Vue frontend is explicitly requested.

The old Porter backend remains the source of truth for route names and payloads. Frontend adaptation must follow the actual `backend/internal/api` routes rather than copying Whatomate’s API client surface.
