select p.name from permissions p
join role_permissions rp on rp.permission_id=p.id
where rp.role_id='viewer' order by 1;
select r.name, count(*) from role_permissions rp join roles r on r.id=rp.role_id group by 1 order by 1;
select distinct r.name from roles r join role_permissions rp on rp.role_id=r.id
join permissions p on p.id=rp.permission_id where p.name='project.create';
