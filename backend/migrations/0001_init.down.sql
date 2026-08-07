-- 0001_init: rollback (Section 4.3 — never hand-edit a merged migration; this
-- is the paired down file for local reset only).
DROP TABLE IF EXISTS servers;
DROP TABLE IF EXISTS ssh_keys;
DROP TABLE IF EXISTS volumes;
DROP TABLE IF EXISTS domains;
DROP TABLE IF EXISTS vms;
DROP TABLE IF EXISTS services;
DROP TABLE IF EXISTS projects;
DROP EXTENSION IF EXISTS pgcrypto;
