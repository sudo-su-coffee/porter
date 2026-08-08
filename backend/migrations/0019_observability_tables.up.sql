-- 0019_observability_tables.up.sql
-- Persist observability data that previously lived only in the in-memory rings
-- (lost on restart): per-replica traffic, per-replica logs, and pre-aggregated
-- daily analytics. The rings stay as the dashboard's hot path; these tables are
-- the durable store the analytics/audit endpoints read from.

-- traffic_log: one row per proxied request (written by the gateway).
CREATE TABLE IF NOT EXISTS traffic_logs (
    id          BIGSERIAL PRIMARY KEY,
    vm_id       UUID,
    project_id  UUID,
    method      TEXT NOT NULL DEFAULT 'GET',
    host        TEXT NOT NULL DEFAULT '',
    path        TEXT NOT NULL DEFAULT '',
    status      INT NOT NULL DEFAULT 200,
    duration_ms INT NOT NULL DEFAULT 0,
    remote_ip   TEXT NOT NULL DEFAULT '',
    ts          TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_traffic_logs_vm  ON traffic_logs(vm_id);
CREATE INDEX IF NOT EXISTS idx_traffic_logs_proj ON traffic_logs(project_id, ts);

-- vm_logs: durable per-VM log tail (mirrors the in-memory ring).
CREATE TABLE IF NOT EXISTS vm_logs (
    id         BIGSERIAL PRIMARY KEY,
    vm_id      UUID,
    project_id UUID,
    line       TEXT NOT NULL DEFAULT '',
    ts         TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_vm_logs_vm ON vm_logs(vm_id, ts DESC);

-- analytics_daily: pre-aggregated per-project request/bandwidth counters per day
-- (what the analytics endpoints read for historical usage).
CREATE TABLE IF NOT EXISTS analytics_daily (
    project_id   UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    day          DATE NOT NULL,
    requests     BIGINT NOT NULL DEFAULT 0,
    bandwidth    BIGINT NOT NULL DEFAULT 0,
    invocations  BIGINT NOT NULL DEFAULT 0,
    PRIMARY KEY (project_id, day)
);

-- daemon_logs_history: durable daemon log (complements the ring).
CREATE TABLE IF NOT EXISTS daemon_logs_history (
    id   BIGSERIAL PRIMARY KEY,
    line TEXT NOT NULL DEFAULT '',
    ts   TIMESTAMPTZ NOT NULL DEFAULT now()
);