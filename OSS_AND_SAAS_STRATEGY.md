# OSS & Future SaaS Strategy — Porter

## Where this stands today

Porter v1.0.0 ships as **self-hosted, open source, MIT licensed.** This is not a "SaaS with a self-hosted mode bolted on" — it's the other way around. The self-hosted core is the actual product being built and shipped first. A hosted/SaaS offering is a possibility to keep the door open for, not a plan being executed in parallel.

This doc exists so that architectural decisions made now don't accidentally foreclose a hosted option later, without letting that future possibility complicate or slow down v1.0.0 itself.

## License: MIT

Chosen for maximum adoption with the fewest questions asked. No copyleft obligations for self-hosters, no chilling effect on companies evaluating it internally, no "will my legal team allow this" friction. This is the standard choice for infra tools that want to be trusted defaults (Traefik-style, not a research project).

**What MIT means practically here:** anyone can fork Porter and run a competing hosted version of it. That's an accepted tradeoff of going MIT over AGPL. The bet is that adoption and ecosystem trust from a fully permissive license outweigh that risk at this stage — same bet projects like Supabase, PostHog (core), and Traefik made.

## Single-tenant by design, not by accident

The self-hosted OSS core stays single-tenant **forever** — this is a permanent architectural stance, not a v1.0.0 shortcut to be lifted later. Concretely, this means:

- One operator, one API token, one trust boundary — this does not change across versions
- No per-tenant resource quotas, billing hooks, or isolation-between-customers logic will be added to the OSS core
- The JSON-file state store, single static token auth, and every other "good enough for one operator" decision documented across `ARCHITECTURE.md` and `DEPLOYMENT.md` are permanent characteristics of the self-hosted product, not placeholders

**Why lock this in now instead of leaving it open:** multi-tenancy is not a feature you add — it's a set of assumptions (auth model, data isolation, resource accounting, billing) that has to be designed in from the start or retrofitted at real cost. Deciding now that OSS stays single-tenant means every future OSS contribution and every doc in this set can be written with a clear, stable audience in mind: one team, one host, full trust. That clarity is worth more to the OSS project's health than keeping the door open to a multi-tenant core nobody's actually building yet.

## If a hosted product happens later

It would be a **separate, closed-source product** built *around* the OSS core — not a fork or a mode-flag inside it. Roughly:

```
┌─────────────────────────────────────────┐
│   Hosted control plane (closed source)    │
│   - multi-tenant auth, billing, quotas    │
│   - orchestrates many isolated Porter      │
│     instances, one per customer            │
└───────────────┬───────────────────────┘
                │  (each customer gets their own)
┌───────────────▼───────────────────────┐
│  Porter core (OSS, MIT, single-tenant)    │
│  — exactly what's in this doc set          │
└───────────────────────────────────────────┘
```

Each customer effectively gets their own isolated single-tenant Porter instance underneath, managed by a hosted control plane that itself stays proprietary. This means:

- The OSS core never needs multi-tenant auth, billing, or quota code — that complexity lives entirely in the hosted layer, which can iterate fast without dragging the OSS project's stability along with it
- Self-hosters and hosted customers run literally the same core, so bugs/improvements benefit both, and self-hosters are never running a "lesser" version
- If the hosted product never happens, nothing about the OSS project needs to be unwound or apologized for — it was never contorted to prepare for it

## What NOT to build into v1.0.0 because of this

To keep the OSS core honest to "single-tenant forever," explicitly avoid, even as "just in case" scaffolding:
- Tenant IDs on any data model
- Per-resource ownership/ACL fields beyond the single API token
- Billing/usage-metering hooks of any kind
- Config flags like `PORTER_MULTI_TENANT=true` that gesture at a mode that doesn't exist

If any of these show up in a PR or a future doc revision, that's a signal the SaaS conversation has become real enough to warrant its own separate closed-source repo — not a reason to compromise the OSS core's simplicity.

## What TO keep in mind, without building it

A short list of things worth *designing around* (costs nothing now, saves pain later) versus *building* (which we're explicitly not doing):

| Consideration | OSS v1.0.0 stance |
|---|---|
| Could the state store be swapped for Postgres later? | Yes — `store.Store` is already a small interface (see `ARCHITECTURE.md` §2.9); swapping the backend doesn't require touching the API layer. This helps self-hosters who outgrow JSON-file storage just as much as it would help a future hosted layer — not SaaS-specific. |
| Could the Control API run behind a different auth layer? | Yes — it already expects a bearer token and doesn't assume *how* that token was issued. A hosted control plane could issue tokens differently without the core caring. Not a change, just a property that already falls out of the design. |
| Should VM/Project IDs be globally unique (UUIDs) rather than sequential? | Already true (`uuid.NewString()` throughout) — happens to help multi-instance-anything later, but was chosen for ordinary good-engineering reasons, not SaaS prep. |

None of these are changes to make — they're just existing v1.0.0 decisions worth naming so it's clear the door isn't accidentally welded shut, even though nothing is being built toward it.

## Bottom line

Ship Porter v1.0.0 as a genuinely good, permissively-licensed, single-tenant self-hosted tool. Judge it on whether people self-host it and like it. A hosted product is a question to revisit *if and when* that happens — not a constraint on how v1.0.0 gets built today.
