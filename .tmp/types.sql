select 'users.id=' || pg_typeof(id) from users limit 1;
select column_name, data_type from information_schema.columns where table_name in ('users','project_members','org_members','team_members') and column_name in ('id','user_id') order by 1,2;
select * from project_members;
