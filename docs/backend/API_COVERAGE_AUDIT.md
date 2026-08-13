# Porter API coverage audit

This audit records the completed review of `internal/api/api.go` against the Porter direct-Firecracker PaaS plan. The audit is intentionally scoped to the backend clone under `/home/ubuntu/porter-old/backend`; no upstream GitHub repository was modified.

## Coverage result

The API registration table now contains **311 registered routes**. Every unique handler referenced by `api.go` has a declaration, and every authenticated route is present in the `routePerms` map. The static route coverage test also confirms that no permission-map entry is dead.

| Check | Result |
|---|---|
| Registered route entries | 311 |
| Central RBAC route-permission entries | 296 |
| Registered handlers without declarations | 0 |
| Dead RBAC route mappings | 0 |
| Authenticated routes missing RBAC mapping | 0 |
| `go test ./...` | Passed |
| `go vet ./...` | Passed |

## Completed endpoint groups

The audit completed the compatibility and operator surfaces that had been declared in handler code but were not reachable from the route table.

| Area | Endpoints added or completed | Permission boundary |
|---|---|---|
| Organization aliases | `GET /org`, `PATCH /org` | `project.read`, `org.settings` |
| Environment inspection | `GET /projects/{projectId}/environments/{envId}/range` | `project.read` |
| Global replicas | `GET /replicas`, `GET /replicas/{replicaId}` | `replica.list` |
| VM compatibility | Domains, logs, metrics, traffic, health, console, exec, SSH info, and SSH certificate aliases under `/vms/{replicaId}` | Existing replica, observability, console, exec, and SSH permissions |
| Host readiness | `GET /host/prerequisites` | `metric.read` |
| Runtime inspection | `GET /host/runtime` | `metric.read` |

The host endpoints are read-only. `GET /host/prerequisites` reuses the same startup checks used by `porter server`, including the Firecracker API socket directory, KVM availability, Firecracker binary, optional jailer, and volumes directory. `GET /host/runtime` returns only non-secret runtime settings; it does not expose the API token, admin credentials, database URL, or SMTP password.

## Direct Firecracker boundary

The runtime remains direct-only. Replica lifecycle operations use the Firecracker process and its per-VM Unix-domain HTTP API socket. The backend does not initialize containerd, does not use an OCI runtime, and does not provide a Dockerfile-to-VM fallback. Deployable artifacts resolve to the kernel and `rootfs.ext4` pair required by Firecracker.

All Firecracker socket ownership is now explicit inside `internal/runtime`: `fc.go` owns the Unix-socket HTTP client, `socket.go` owns socket naming, readiness polling, and idempotent cleanup, and `manager.go` owns the Firecracker process lifecycle. The API, network manager, and startup checker do not open or clean up per-VM sockets. A source scan found no socket lifecycle implementation outside `internal/runtime`; the startup checker only validates the configured socket directory.

The API intentionally exposes SSH, console, and exec routes for the dashboard contract, but the current direct runtime returns an explicit guest-vsock-agent limitation rather than pretending that a host-side container task bridge exists. This boundary is documented in [`DIRECT_FIRECRACKER.md`](./DIRECT_FIRECRACKER.md) and should remain unchanged until a real in-guest agent is implemented.

## Validation commands

```bash
cd backend
gofmt -w internal/api/api.go internal/api/handlers_impl.go internal/api/rbac_test.go cmd/porter/main.go
go test ./...
go vet ./...
```

The focused RBAC tests now include the new org, environment, global replica, host, VM observability, SSH, and exec aliases. The route-coverage test continues to enforce both directions of the registration-to-permission relationship.

## References

1. [Firecracker official getting-started guide](https://github.com/firecracker-microvm/firecracker/blob/main/docs/getting-started.md)
