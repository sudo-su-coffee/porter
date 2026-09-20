select id, scope, is_admin, sees_all, is_default, is_system from roles order by 1;
select role, count(*) from users group by 1;
select count(*) from role_permissions where role_id='admin';
select count(*) from role_permissions where role_id='member';
