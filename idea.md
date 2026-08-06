# The Idea

Porter is a **self-hosted PaaS** — the Vercel/Fly.io model running on your own
metal — built on **Firecracker microVMs**.

Most "serverless" platforms give you two bad choices:

- **Containers** — fast and cheap, but every workload shares the host kernel.
  Weak isolation for anything multi-tenant or untrusted.
- **Classic VMs** — strong isolation, but slow boots, hundreds of MB of
  baseline overhead, and a huge emulated device surface.

Firecracker is the third option: a real, hardware-isolated microVM that boots in
milliseconds, uses a few MB per instance, and exposes only the minimal devices a
Linux workload needs. It powers AWS Lambda + Fargate and Fly.io's Fly Machines.

**Porter is the self-hosted control plane for that engine.** Instead of spinning
raw Firecracker processes yourself, you deploy **Docker/OCI images** and Porter
boots each one as a kernel-isolated microVM through containerd + the
`aws.firecracker` shim — image pull, snapshots, jailer, networking, and the
in-VM agent are all handled for you.

## Why that matters

| | Containers | Classic VM | Porter (microVM) |
|---|---|---|---|
| Isolation | shared kernel | full VM | **real kernel isolation** |
| Cold start | ms | seconds | **sub-second** |
| Per-instance overhead | tiny | hundreds of MB | **few MB** |
| Multi-tenant safe | ❌ | ✅ | ✅ |
| Runs Docker images | ✅ | ❌ | ✅ |

## The fly.io model, self-hosted

Fly Machines = Firecracker microVMs you control with an API, a fast proxy, and
optional volumes. Porter is that model you run yourself:

- **Deploy** a Docker/OCI image or a `compose.yml` → each service becomes a microVM.
- **Run** fast, isolated, high-density — many workloads per host.
- **Manage** — create / stop / restart / delete, live logs, traffic, overview,
  all from a clean Vercel-style dashboard or the REST API.
- **One binary** — a single pure-Go control plane. No Docker daemon, no
  orchestrator of your own to babysit.

## What it is not

- Not a container orchestrator (K8s/Mesos).
- Not a UI that just spawns `firecracker` processes for you.
- Not "like Docker." The engine is a microVM, not a container runtime;
  containers are only the **packaging** format.

Porter is the point, the microVM is the engine, and your goal is to get a real
app running in a kernel-isolated microVM from a stock Docker image, through a
dashboard that feels like the PaaS you already wish you had.