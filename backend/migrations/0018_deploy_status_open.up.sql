-- 0018_deploy_status_open.up.sql
-- Release the deployment build_status CHECK so the real preview/live lifecycle
-- can persist, and allow projects to scale to 0 (paused) replicas.

ALTER TABLE deployments DROP CONSTRAINT IF EXISTS deployments_build_status_check;
ALTER TABLE projects DROP CONSTRAINT IF EXISTS projects_replicas_desired_check;

ALTER TABLE deployments ADD CONSTRAINT deployments_build_status_check
    CHECK (build_status IN ('pending','building','queued','ready','failed','preview','live'));

ALTER TABLE projects ADD CONSTRAINT projects_replicas_desired_check
    CHECK (replicas_desired >= 0);
