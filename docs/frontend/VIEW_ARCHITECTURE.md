# Porter Vue view architecture

Porter intentionally does **not** create one-line wrapper files for every route. The dashboard has one real, reusable `ResourceManager.vue` that owns resource loading, filtering, create forms, row actions, loading/error/empty states, and authenticated API writes. Route metadata supplies the exact endpoint and field/action schema.

The dashboard keeps genuinely dedicated components where the workflow has unique behavior: deployment list, project workspace, deployment detail, builds/source, images/artifacts, domains, servers, logs, traffic, analytics, teams/RBAC, host settings, replica detail, replica streams, live SSE logs, login, and settings.

This keeps the implementation understandable while still providing complete route coverage. A route is counted as working only when it resolves to either a real dedicated component or `ResourceManager` with a live backend contract; a file containing only an import and template delegation is not accepted as a view.
