-- 0018_deploy_status_open.down.sql
ALTER TABLE deployments DROP CONSTRAINT IF EXISTS deployments_build_status_check;
ALTER TABLE projects DROP CONSTRAINT IF EXISTS projects_replicas_desired_check;

ALTER TABLE deployments ADD CONSTRAINT deployments_build_status_check
    CHECK (build_status IN ('pending','building','ready','failed'));

ALTER TABLE projects ADD CONSTRAINT projects_replicas_desired_check
    CHECK (replicas_desired >= 1);