-- 0007_metrics: time-series samples for v0.9 Observability.
CREATE TABLE IF NOT EXISTS metrics_samples (
    id       UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    vm_id    UUID REFERENCES vms(id) ON DELETE CASCADE,
    metric   TEXT NOT NULL,
    value    DOUBLE PRECISION NOT NULL,
    ts       TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_metrics_vm_ts ON metrics_samples(vm_id, metric, ts);
