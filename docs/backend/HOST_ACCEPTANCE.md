# Porter Linux host acceptance

This checklist is the boundary between repository validation and a real Porter installation. The sandbox can compile the daemon and Vue dashboard, but it cannot prove PostgreSQL service behavior, systemd installation, `/dev/kvm`, TAP networking, Firecracker boot, or the real `vmlinux` and `rootfs.ext4` artifact contract.

## Preconditions

Run the checks on the target Linux host as an operator with `sudo` access. The host must have a supported Linux distribution, systemd, PostgreSQL or a reachable PostgreSQL URL, `/dev/kvm`, the installed Firecracker binary, and the pinned guest artifacts.

## Read-only readiness check

After installation, run:

```bash
sudo bash scripts/backend/host-readiness.sh
sudo PORTER_REQUIRE_METRICS=1 bash scripts/backend/host-readiness.sh
sudo PORTER_REQUIRE_FIRECRACKER=1 PORTER_REQUIRE_METRICS=1 bash scripts/backend/host-readiness.sh
```

If Porter listens on another address, provide `PORTER_BASE_URL`, for example `PORTER_BASE_URL=http://127.0.0.1:8080`. The script does not mutate PostgreSQL, Firecracker, systemd, or project state.

## Control-plane smoke test

Once the health check passes, run the authenticated API smoke test with the credentials created by the installer:

```bash
sudo -u porter bash scripts/backend/api-smoke.sh
```

The smoke test verifies login, CSRF, organization reads, project creation, project status, replica reads, and the overview endpoint. A successful control-plane response does not by itself prove that a guest microVM booted; confirm the replica state and daemon logs separately.

## Firecracker acceptance

Create a disposable test project using the dashboard or API, then verify the following sequence in the UI and daemon logs: project creation, image registration, replica start, replica health transition, replica stop, replica restart, and project deletion. Confirm that the corresponding Firecracker Unix socket is created under the configured socket directory and removed after the replica stops.

## Observability acceptance

Keep external telemetry disabled for the first smoke test. Then perform a separate non-production test with explicit provider configuration:

```bash
sudo PORTER_METRICS_ENABLED=true systemctl restart porter.service
curl --fail http://127.0.0.1:8080/metrics | head
```

Prometheus should scrape the protected Porter address at `/metrics`. Configure OpenTelemetry and Sentry only after confirming that the endpoint, retention policy, and network policy are appropriate for the host. Verify that no bearer tokens, cookies, SQL text, database URLs, request bodies, or guest secrets appear in exported events.

## Rollback evidence

Before an upgrade, record the installed release tag, daemon checksum, configuration backup path, database backup path, and guest artifact checksums. If an upgrade fails, stop the service, restore the previous release and configuration, restore the database only when required by the migration contract, and rerun the readiness and API smoke tests.
