-- 0009_orgs_dns_buildlogs: project=VM model support.
-- Projects can be grouped into orgs and tagged; each project carries DNS
-- records and a rolling build log; volumes are /mnt mount points.

CREATE TABLE IF NOT EXISTS orgs (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name       TEXT NOT NULL UNIQUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

ALTER TABLE projects ADD COLUMN IF NOT EXISTS org_id UUID REFERENCES orgs(id) ON DELETE SET NULL;
ALTER TABLE projects ADD COLUMN IF NOT EXISTS tags TEXT[] NOT NULL DEFAULT '{}';

CREATE TABLE IF NOT EXISTS dns_records (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    name       TEXT NOT NULL,          -- e.g. web.myapp.porter.test
    type       TEXT NOT NULL DEFAULT 'A',
    value      TEXT NOT NULL,          -- IP or CNAME target
    ttl        INT NOT NULL DEFAULT 300,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (project_id, name)
);

CREATE TABLE IF NOT EXISTS build_logs (
    id         BIGSERIAL PRIMARY KEY,
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    line       TEXT NOT NULL,
    ts         TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_build_logs_project ON build_logs(project_id, id);
CREATE INDEX IF NOT EXISTS idx_dns_records_project ON dns_records(project_id);
