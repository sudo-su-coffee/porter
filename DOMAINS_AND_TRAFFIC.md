# Domains & Traffic Log — Porter v1.0.0

Both features live inside the **Porter Gateway** (`backend/internal/gateway`) — the single HTTP front door already handling routing. Domain assignment and request-level traffic visibility are both naturally proxy-layer concerns, so no new component is needed.

---

## 1. Domains — the Vercel model

### 1.1 One-time setup: point your wildcard at Porter

You own a domain, e.g. `example.com`. Once, at install time, you create:

```
*.example.com        A       <host-public-ip>
```

Then tell Porter:
```bash
porter domain set-base example.com
```

From this point forward, **every deploy is reachable with zero further DNS work** — that's the entire point of the wildcard.

### 1.2 Two auto subdomains per service, always

Every service that declares a port gets **two** subdomains the instant it boots, matching the Vercel pattern of a stable production URL plus a unique preview URL per deploy:

| Type | Pattern | Points at |
|---|---|---|
| **Stable** | `<service>.<project>.example.com` | Whichever VM for that service is currently the *live* one — stays constant across restarts/redeploys of the same service |
| **Preview** | `<service>-<deploy-id>.<project>.example.com` | This *specific* deploy's VM, forever (or until that VM is deleted) — lets you test a new version at its own URL before it becomes the stable one |

Standalone (non-compose) VMs follow the same pattern without the project segment:
```
cache.example.com                 (stable)
cache-a1b2c3.example.com          (preview, this specific deploy)
```

**Promoting a preview to stable:** when you redeploy a service (`porter up` again with the same name), the new VM boots, gets its own fresh preview URL immediately, and — once it reaches `running` — Porter Gateway atomically repoints the **stable** subdomain at the new VM. The previous VM keeps its own preview URL working (still reachable directly) until you delete it, so nothing breaks mid-cutover and you always have a rollback target one command away (`porter rollback <service>`).

### 1.3 Custom domains (bring-your-own, CNAME)

Attach any fully-owned domain to a service whenever you're ready:

```bash
porter domains add shop.mybrand.com --service my-app-api
```

or via the dashboard's Domains tab.

**Setup flow:**
1. Operator adds the domain → Control API records it, returns the exact DNS record to create
2. Operator creates a CNAME at their DNS provider:
   ```
   shop.mybrand.com   CNAME   gateway.example.com
   ```
3. Gateway verifies the CNAME resolves correctly (polls every `PORTER_DOMAIN_VERIFY_INTERVAL`, default 30s) before routing live traffic to it — prevents accidentally serving someone else's misdirected traffic
4. Once verified, requests to `shop.mybrand.com` route the same as a stable subdomain would, including the same `X-Porter-VM-ID` / `X-Porter-Project-ID` identity headers

A custom domain always points at a service's **stable** slot (tracks whichever VM is currently live for that service) — it is not attached to one specific deploy's preview VM, matching how production custom domains behave on Vercel.

### 1.4 TLS

- v1.0.0 does **not** ship automatic Let's Encrypt provisioning for either wildcard subdomains or custom domains. Operators front Porter with their own TLS termination (Cloudflare, a load balancer, etc.), or wait for the roadmap item.
- **Deferred to v1.1:** automatic wildcard + per-custom-domain TLS via ACME (HTTP-01/DNS-01), matching how Vercel/Netlify auto-provision certs on domain add.

### 1.5 Config

| Env var | Default | Purpose |
|---|---|---|
| `PORTER_BASE_DOMAIN` | *(required, no default)* | The wildcard base domain, e.g. `example.com` — set via `porter domain set-base` |
| `PORTER_DOMAIN_VERIFY_INTERVAL` | `30s` | How often the gateway re-checks pending custom-domain CNAMEs |

### 1.6 Domain → route resolution order

1. Exact match on a verified custom domain (`Host` header)
2. Exact match on a stable subdomain
3. Exact match on a preview subdomain
4. Path-prefix match (`/<project>/<service>/...`) — fallback/manual-testing path
5. No match → `502` with a clear "no route for `<host><path>`" body

### 1.7 What happens to domains when a VM is deleted

- Deleting a **non-live** (older preview) VM removes only its own preview subdomain
- Deleting the **currently-live** VM for a service removes its stable subdomain too — the service shows as unreachable in the dashboard until redeployed, rather than silently falling back to an older VM
- Custom domains are **never** silently reassigned. If their target service's live VM is deleted, the custom domain is shown as "unassigned" in the dashboard until reattached to something

---

## 2. Traffic log

### 2.1 Scope

In-memory ring buffer per gateway process. No disk persistence, no external log drain in v1.0.0. Restarting the gateway process clears the log — this is a live/recent-activity view, not an audit trail.

### 2.2 What's captured per request

```json
{
  "timestamp": "2026-08-04T12:03:41.221Z",
  "method": "GET",
  "host": "api.my-app.example.com",
  "path": "/v1/orders",
  "status": 200,
  "duration_ms": 42,
  "vm_id": "5b2e...",
  "project_id": "9a1c...",
  "service_name": "api",
  "remote_ip": "203.0.113.4",
  "bytes_out": 1834
}
```

Ring buffer default size: last **2,000 requests per gateway process** (configurable, `PORTER_TRAFFIC_LOG_SIZE`), oldest entries drop as new ones come in.

### 2.3 Where it shows up

- **Project/VM detail page** — a "Traffic" tab, live-updating table (via the same SSE mechanism already used for state events — a `traffic.request` event type), most recent first
- Filterable client-side by status code range (2xx/4xx/5xx), method, and path substring — filters the buffer already sent to the browser, no new query round-trip
- A small live requests/sec sparkline at the top, computed client-side from the buffer's timestamps

### 2.4 API surface

Covered in `API_SPEC.md` — `GET /vms/{id}/traffic` and the `traffic.request` SSE event. No new endpoints beyond those two.

### 2.5 Explicitly deferred (not v1.0.0)

- Persisted/rotated log files on disk
- Log drains / export to external systems
- Per-project or global aggregate traffic dashboards (v1.0.0 ships per-VM/per-service only)
- Long-window analytics (daily/weekly traffic trends) — the ring buffer is a "what's happening right now" view
- Request/response body capture — headers/metadata only, never bodies, in v1.0.0
