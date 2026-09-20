select id, name from permissions where lower(name) like '%create project%' or lower(name) like '%project%create%';
select r.name as role, p.name as perm from role_permissions rp
join roles r on r.id=rp.role_id join permissions p on p.id=rp.permission_id
where lower(p.name) like '%create project%' order by 1;
