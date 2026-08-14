# Porter PostgreSQL backup and restore

Porter’s PostgreSQL database is the durable control-plane source of truth for users, organizations, roles, projects, deployments, and runtime metadata. Prometheus metrics and in-memory runtime rings are not a substitute for a database backup.

## Backup

Use a PostgreSQL custom-format dump so the backup can be checked and restored selectively:

```bash
sudo install -d -m 0700 /var/backups/porter
sudo -u porter pg_dump \
  --format=custom \
  --file=/var/backups/porter/porter-$(date -u +%Y%m%dT%H%M%SZ).dump \
  "$PORTER_DATABASE_URL"
sudo sha256sum /var/backups/porter/porter-*.dump
```

Do not place the connection URL in a public shell script or commit it to the repository. For a remote database, run `pg_dump` from a controlled host with the URL supplied through a protected environment file or an interactive prompt.

## Restore test

Restore into a disposable database before relying on a backup:

```bash
createdb porter_restore_check
pg_restore --exit-on-error --clean --if-exists \
  --dbname=porter_restore_check /var/backups/porter/porter-YYYYmmddTHHMMSSZ.dump
dropdb porter_restore_check
```

For a destructive production restore, stop Porter first, preserve the failed database for investigation, restore into a newly created database, update the protected `PORTER_DATABASE_URL`, run migrations only when the release documentation requires it, and start the service. Finish with `host-readiness.sh`, the API smoke test, and a dashboard login check.

## Retention

Keep at least one recent backup on the host and one encrypted copy on separate storage. Test both checksum verification and restore periodically. The installer does not upload backups automatically.
