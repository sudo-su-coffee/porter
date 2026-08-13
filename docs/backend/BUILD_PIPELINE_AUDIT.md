# Build and Source Pipeline Audit

## Current Porter behavior

`POST /projects/{projectId}/builds`, the Git import path, and Git deploy path clone a repository with `git`, then accept only a prebuilt direct Firecracker artifact pair: `vmlinux` and `rootfs.ext4` at the repository root or under `.porter/`. If those files are absent, the build fails. The current Go module does not include BuildKit or a Git client library, and the build log endpoint returns a tail from the in-memory/store log ring rather than a dedicated live stream.

The current handler text that describes `OCI → microVM` is stale and must not be treated as implemented behavior. Porter’s direct-only runtime cannot boot a Docker image reference. A Dockerfile or Compose file needs a separate, explicit conversion pipeline that produces a compatible guest kernel/rootfs artifact or a registered release artifact before deployment.

## Official BuildKit boundary

The official BuildKit repository exposes a Go client under `github.com/moby/buildkit/client`, the Dockerfile frontend as `dockerfile.v0`, and exporters for image or filesystem results. A Go service can connect to a separately managed `buildkitd`, submit a solve request with local context/dockerfile sessions, consume `SolveStatus` progress, and export the result. BuildKit’s output is an OCI/container image or filesystem result; it is not automatically a Firecracker `rootfs.ext4` plus matching `vmlinux`.

Porter therefore needs two distinct phases:

1. **Source build:** GitHub repository detection, branch/commit pinning, Dockerfile or Compose discovery, BuildKit solve, and durable build provenance/logs.
2. **Guest conversion:** a reviewed, host-side image-to-rootfs pipeline that creates a compatible ext4 guest image and pairs it with a compatible kernel. This phase must be explicit and must not pass an OCI reference directly to Firecracker.

The build service should treat BuildKit as an optional host dependency with a configured Unix socket or TCP endpoint, stream progress into the Porter event hub and durable build-log table, and fail clearly when `buildkitd` or the guest conversion toolchain is unavailable. No fake “ready” image should be created when only the Dockerfile solve has completed.

## Sources

1. [Moby BuildKit official repository](https://github.com/moby/buildkit)
2. [BuildKit repository Dockerfile frontend and buildctl example](https://github.com/moby/buildkit#exploring-dockerfiles)
