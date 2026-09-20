# Commerce (thin, deferred tables) — shape now, money later

> Billing tables ship ONLY with subscriptions (MVP rule). This doc fixes the shape so
> admission/quota code written today needs no migration later.

## Model

`Product → Plan → Price/Charge → Entitlement → Subscription → Service → UsageEvent → Meter → Charge → Invoice → Payment`
(+ credits/coupons/refunds/taxes/dunning). Meters from day one (write-only `usage_events`
with `idempotency_key`): vcpu/mem/storage/net/IP/snapshot/backup/build/deploy/fn/queue/log/trace.
Rating/invoicing later. Every invoice line traceable: Invoice → Charge → Meter → UsageEvents → Resource → Node/Region.

## Lifecycle + dunning

`DRAFT→PENDING→TRIALING→ACTIVE→PAST_DUE→RECOVERY→SUSPENDED→CANCELLED→EXPIRED`.
Payment fail → retry → notify → grace → past-due → policy → suspend-if-required (storage preserved).
Billing emits policy actions; provisioning executes (never direct VM kills).

## Entitlements gate admission now

`max_vcpu/mem/storage/domains/projects/IPs/volumes`, `private_networking`, `custom_domains`, `gpu`
— checked at quota/entitlement step even before money flows. Quotas (ceilings) + entitlements
(allowances) are separate fields; scheduler enforces quotas, policy enforces entitlements.

## Serverless billing (ready when functions land)

Per-invocation + GB-s + egress meters ride the same `usage_events` spine; dashboard shows cost
per deploy/preview/function before invoices exist.
