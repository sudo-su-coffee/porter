-- 0016_versioned_deployments: concurrent Vercel-style deployment pools.
-- A project may keep multiple immutable deployment revisions alive at once.
ALTER TABLE deployments ADD COLUMN IF NOT EXISTS version_label TEXT NOT NULL DEFAULT '';
ALTER TABLE deployments ADD COLUMN IF NOT EXISTS guest_base TEXT NOT NULL DEFAULT 'alpine';
ALTER TABLE deployments ADD COLUMN IF NOT EXISTS environment TEXT NOT NULL DEFAULT 'preview';
ALTER TABLE deployments ADD COLUMN IF NOT EXISTS is_production BOOLEAN NOT NULL DEFAULT false;
ALTER TABLE deployments ADD COLUMN IF NOT EXISTS route_weight INT NOT NULL DEFAULT 0;
ALTER TABLE deployments ADD COLUMN IF NOT EXISTS vm_ids JSONB NOT NULL DEFAULT '[]';

ALTER TABLE vms ADD COLUMN IF NOT EXISTS deployment_id UUID REFERENCES deployments(id) ON DELETE SET NULL;
ALTER TABLE vms ADD COLUMN IF NOT EXISTS deployment_version TEXT NOT NULL DEFAULT '';
ALTER TABLE vms ADD COLUMN IF NOT EXISTS deployment_environment TEXT NOT NULL DEFAULT '';

CREATE INDEX IF NOT EXISTS idx_deployments_project_created ON deployments(project_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_deployments_project_production ON deployments(project_id, is_production);
CREATE INDEX IF NOT EXISTS idx_vms_deployment ON vms(deployment_id);
