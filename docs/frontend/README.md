# Porter Frontend Documentation

The Porter dashboard is a Vue 3 + Vite operator workspace. It is adapted from a general Whatomate workspace structure, but it contains no WhatsApp-specific navigation, stores, entities, or chat behavior.

| Document | Purpose |
|---|---|
| [`PORTER_FRONTEND.md`](PORTER_FRONTEND.md) | Frontend workstream overview and API/runtime contract |
| [`PAGE_FLOW_AUDIT.md`](PAGE_FLOW_AUDIT.md) | Page-flow and operational surface audit |
| [`VIEW_ARCHITECTURE.md`](VIEW_ARCHITECTURE.md) | Real component architecture and no-wrapper policy |
| [`../backend/PAGE_API_MATRIX.md`](../backend/PAGE_API_MATRIX.md) | Backend-to-page endpoint and permission matrix |
| [`../backend/PAGE_API_GAP_AUDIT.md`](../backend/PAGE_API_GAP_AUDIT.md) | Latest missing-page/action audit |

## Development

```bash
cd frontend
npm ci
npm run dev
npm run build
```

The production build is emitted to `backend/web/dist/` because the Go binary embeds the dashboard. The canonical repository-level validation command is:

```bash
make validate
```

## Architecture rules

The dashboard uses genuine dedicated components for unique behavior and a real schema-driven `ResourceManager.vue` for resources whose backend contract is uniform and explicitly declared by the router. One-line wrapper views are not counted or retained. Every page must use real API data, loading/error/empty states, validation where applicable, and honest permission or host-limitation messaging.
