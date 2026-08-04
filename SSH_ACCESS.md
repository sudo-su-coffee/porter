# SSH Access — Porter v1.0.0

## Goal

`porter ssh <vm-name>` should just work, from anywhere the operator has network access to the gateway — no hunting for internal bridge IPs, no per-VM port forwarding, no manually-managed `known_hosts` chaos as VMs come and go.

## Model: single gateway, certificate-based, routes by name — backed by `task.Exec()`, not a guest sshd

```
 operator
    │  ssh my-app-api@gateway.example.com -p 2222
    ▼
┌─────────────────────────┐
│   SSH Gateway (sshgw)     │   ← only SSH-facing thing exposed to the operator
│  - authenticates operator│
│  - issues short-lived    │
│    cert OR checks it     │
│  - looks up VM's          │
│    containerd task ID     │
│  - calls task.Exec(shell)│
│    and pipes stdio        │
└──────────┬───────────────┘
           │  containerd task API, over the same vsock
           │  channel firecracker-containerd's in-VM
           │  agent already uses for task control
           ▼
  ┌─────────────────────┐
  │  in-VM agent           │  ← ships with firecracker-containerd,
  │  (guest init, runs     │     not built or baked in by Porter
  │  the exec'd shell)     │
  └─────────────────────┘
```

There is no SSH server running inside the guest at all in v1.0.0. The SSH Gateway terminates a real SSH session with the operator, then internally turns it into a `task.Exec()` call against that VM's containerd task and streams stdio both ways — the same mechanism `ctr task exec` uses.

## Why this instead of exposing (or baking in) a guest sshd

- One place to manage auth, audit, and revocation instead of N — same rationale as a bastion, but there's no second hop to a real network daemon after the gateway
- Nothing to expose on VM bridges at all: the exec path never touches the guest's network stack, so there's no in-guest port to firewall, no host key to manage per VM, and a VM with no network configured yet is still SSH-reachable
- Renaming/restarting a VM never breaks the operator's muscle memory (`ssh my-app-api` always works) — the gateway just resolves the name to a current task ID instead of a current IP
- No per-image dependency: a bare `redis:7` with zero customization is exec-able the moment its task is `running`, because the exec path rides infrastructure `firecracker-containerd` already provides for every task, not something Porter has to inject into the image

## Auth flow (two options, v1.0.0 ships both)

### Option A — Certificate flow (recommended, default for `porter ssh`)
1. Operator runs `porter ssh my-app-api`
2. CLI generates (or reuses) a local ephemeral ed25519 keypair if none cached
3. CLI calls `POST /vms/{id}/ssh-cert` with the public key, authenticated via the same API token used for the dashboard
4. Gateway's CA signs a certificate valid for 10 minutes (configurable), scoped to that one VM's principal name
5. CLI opens the SSH connection using the signed cert; gateway validates the cert against its own CA, extracts the target VM name from the cert's principal, looks up the VM's current containerd task ID in the store, and calls `task.Exec()` to start a shell, piping the SSH session's stdio to/from it
6. Certificate expires after 10 minutes — no long-lived SSH keys sitting around

### Option B — Static authorized key (simple mode, opt-in)
For operators who just want `~/.ssh/config` to work without the CLI wrapper:
1. Operator adds their public key once via `porter auth add-key ~/.ssh/id_ed25519.pub`
2. Key is trusted by the gateway indefinitely (until revoked)
3. Plain `ssh <vm-name>@gateway.example.com -p 2222` works directly, e.g. from any terminal or IDE's SSH integration — no CLI required for the connection itself, only for the one-time key registration

Static keys are logged and can be listed/revoked via `porter auth list-keys` / `porter auth revoke-key <fingerprint>`.

## Guest-side setup: none required

Unlike a network-sshd design, there's no per-image baking step at all. `firecracker-containerd`'s in-VM agent is what runs as guest init on every VM regardless of source image, and `task.Exec()` is a capability it provides out of the box — Porter doesn't inject a binary, a CA key, or a host key into any image.

This means **every VM is exec-reachable by default**, with zero per-image configuration required — even a bare `redis:7` image booted with no customization gets a working shell the moment its task reaches `running`.

One tradeoff worth naming: because there's no real network sshd in the guest, an operator can't bypass the gateway and `ssh` directly into a VM's bridge IP for local debugging the way they could with a baked-in dropbear. Everything goes through `task.Exec()`, which means everything also goes through the gateway's auth and logging — a deliberate constraint, not an oversight.

## What you get once connected

A shell (`/bin/sh`, or the image's configured shell) as `root` inside the guest's minimal environment — whatever the source image's filesystem provides, nothing injected into it. Since there's no in-guest Docker daemon, this is a direct look at exactly what the service process sees.

## Session logging (v1.0.0 scope)

- The gateway logs connection metadata (who, which VM, when, cert fingerprint or key fingerprint used, duration) — not full session content/keystrokes in v1
- Full session recording (asciinema-style) is a roadmap item, not v1.0.0

## Limitations in v1.0.0

- No SSH access to a VM that's `stopped` — the gateway refuses with a clear error rather than hang, since there's no IP to route to
- No port-forwarding (`-L`/`-R`) support through the gateway yet — direct interactive sessions only
- No SFTP/SCP explicitly tested/supported yet (dropbear technically supports it, but it's not part of the v1.0.0 test matrix — treat as best-effort)
- Gateway itself is a single process/single point of failure in v1 — acceptable for the single-host target, flagged for HA work in `ROADMAP.md`
