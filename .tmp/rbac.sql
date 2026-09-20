select r.name as role, p.name as perm from permissions p
join role_permissions rp on rp.permission_id=p.id
join roles r on r.id=rp.role_id
where r.name in ('Viewer','Member') and p.name like 'project.%'
order by 1,2;
select id, name from roles order by 1;
