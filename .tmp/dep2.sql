select id from deployments where project_id = 'aaf43af7-8b62-4599-ae04-0c29204750f6' order by revision desc;
select pg_typeof(project_id) from deployments limit 1;
