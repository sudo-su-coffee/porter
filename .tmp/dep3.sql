select id, project_id, revision from deployments order by created_at desc limit 5;
select pg_typeof(project_id) from deployments limit 1;
