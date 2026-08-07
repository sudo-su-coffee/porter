-- 0016: seed the golden-image library (redis / postgresql / mysql) so the
-- dashboard's image picker is useful on a fresh install. OCI refs boot via
-- containerd + the aws.firecracker shim. Idempotent: ON CONFLICT (name).
INSERT INTO golden_images (id, name, image, description, vcpus, mem_mib, ports, env, tags, logo, version, data, created_at)
VALUES
  (gen_random_uuid(), 'redis',     'redis:7-alpine',    'Redis 7 (OCI image, boots via containerd)',    1, 256, '[{"container_port":6379}]'::jsonb, '{}'::jsonb, ARRAY['cache','redis'],     '', 'v1', '{"name":"redis","image":"redis:7-alpine","vcpus":1,"mem_mib":256,"ports":[{"container_port":6379}]}'::jsonb, now()),
  (gen_random_uuid(), 'postgresql','postgres:16-alpine','PostgreSQL 16 (OCI image, boots via containerd)',1, 512, '[{"container_port":5432}]'::jsonb, '{}'::jsonb, ARRAY['db','postgres'],    '', 'v1', '{"name":"postgresql","image":"postgres:16-alpine","vcpus":1,"mem_mib":512,"ports":[{"container_port":5432}]}'::jsonb, now()),
  (gen_random_uuid(), 'mysql',     'mysql:8',           'MySQL 8 (OCI image, boots via containerd)',     1, 512, '[{"container_port":3306}]'::jsonb, '{}'::jsonb, ARRAY['db','mysql'],       '', 'v1', '{"name":"mysql","image":"mysql:8","vcpus":1,"mem_mib":512,"ports":[{"container_port":3306}]}'::jsonb, now())
ON CONFLICT (name) DO NOTHING;
