-- Rollback 0014 (re-add the legacy kind/status checks).
ALTER TABLE domains ADD CONSTRAINT domains_kind_check CHECK (kind IN ('stable','preview','custom'));
ALTER TABLE domains ADD CONSTRAINT domains_status_check CHECK (status IN ('pending','active','failed'));
