select column_name, data_type from information_schema.columns where table_name='deployments' order by ordinal_position;
select id, project_id, rollback_to, version_label, guest_base, environment from deployments;
