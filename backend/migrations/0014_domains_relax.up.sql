-- 0014: the store's AddDomain maps types.Domain.Type -> `kind` and
-- types.Domain.Status -> `status`, which the legacy CHECK constraints reject
-- for empty/unknown values. The row's `data` JSON is authoritative, so drop
-- the constraints (the app manages kinds/statuses).
ALTER TABLE domains DROP CONSTRAINT IF EXISTS domains_kind_check;
ALTER TABLE domains DROP CONSTRAINT IF EXISTS domains_status_check;
