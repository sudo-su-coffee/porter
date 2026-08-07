-- 0017: durable daemon/audit log. The in-memory ring stays the fast path for
-- the dashboard; this table is the audit trail (retention is an ops choice).
CREATE TABLE IF NOT EXISTS daemon_logs (
    id   BIGSERIAL PRIMARY KEY,
    ts   TIMESTAMPTZ NOT NULL DEFAULT now(),
    line TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_daemon_logs_ts ON daemon_logs(ts);
