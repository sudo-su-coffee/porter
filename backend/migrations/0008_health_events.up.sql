-- 0008_health_events: health history feed for the dashboard.
CREATE TABLE IF NOT EXISTS health_events (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    vm_id      UUID REFERENCES vms(id) ON DELETE CASCADE,
    service_id UUID REFERENCES services(id) ON DELETE CASCADE,
    status     TEXT NOT NULL,          -- healthy | unhealthy | checking
    detail     TEXT NOT NULL DEFAULT '',
    ts         TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_health_events_vm_ts ON health_events(vm_id, ts);
