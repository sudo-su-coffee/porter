# Porter deploy

The Backend workstream keeps its operational scripts in one group:

```
scripts/backend/
├── install.sh             single production install for source checkouts and GitHub Releases
└── dev.sh                 only development entrypoint

The Firecracker, PostgreSQL, release-package, and source-build scripts remain
internal helpers called by these two entrypoints; they are not separate public
installation commands.
```

## Dev

```bash
cd /path/to/porter
bash scripts/backend/dev.sh up        # Docker Postgres + build + run (real boots)
bash scripts/backend/dev.sh down      # stop postgres (data kept)
bash scripts/backend/dev.sh clean     # down + remove ./bin
```

- Requires **Docker** (Postgres container) and **Go 1.25+**.
- Boots real microVMs through the official Firecracker HTTP API over per-VM Unix
  sockets on a host with `/dev/kvm`. There is no containerd, OCI runtime, or
  Firecracker shim in the Porter runtime path.
- Dashboard: http://localhost:8080. The initial database-seeded account is
  initialized with `PORTER_BOOTSTRAP_ADMIN_PASSWORD`.

## Linux beta (single Linux + KVM host)

For a published GitHub Release, the shortest path is one command:

```bash
curl -fsSL https://raw.githubusercontent.com/sudo-su-coffee/porter/main/scripts/backend/install.sh | sudo bash
```

The installer pauses at the terminal and asks for the PostgreSQL mode. Enter
`1` for local PostgreSQL on this Linux host. Enter `2` to use a remote database,
then provide its complete PostgreSQL connection URL when prompted. The prompt
is read from `/dev/tty`, so it remains available even though the installer
script itself is supplied through the curl pipe.

The equivalent non-interactive commands are:

```bash
# Local PostgreSQL on this host.
curl -fsSL https://raw.githubusercontent.com/sudo-su-coffee/porter/main/scripts/backend/install.sh \
  | sudo PORTER_POSTGRES_MODE=local bash

# Remote PostgreSQL.
curl -fsSL https://raw.githubusercontent.com/sudo-su-coffee/porter/main/scripts/backend/install.sh \
  | sudo PORTER_POSTGRES_MODE=remote \
       PORTER_DATABASE_URL='postgres://porter:password@db.example.com:5432/porter?sslmode=require' bash
```

Place the variables after `sudo`. A command such as `set PORTER_POSTGRES_MODE=local`
is Windows CMD syntax and does not export the variable in WSL Bash; `export
PORTER_POSTGRES_MODE=local` is the Bash form, but `sudo` may still remove it.

The bootstrap downloads the architecture-specific Porter package and its
checksum sidecar, verifies the package, and delegates to the same installer
used by the source checkout. Set `PORTER_RELEASE_TAG` for a different release,
or set `PORTER_RELEASE_PACKAGE_SHA256` explicitly when the checksum sidecar is
not reachable. The release must include a real base-image bundle; the installer
will not fabricate one.

Verified release archives are cached at `/var/cache/porter/releases` by
default, so repeated installer runs do not download the same package again. A
checksum mismatch removes the cached file and performs a clean download. Set
`PORTER_CACHE_DIR` to choose another cache location.

For files around 50 MB, you may upload the two files at the repository root as
`vmlinux` and `rootfs.ext4`, as the current main branch does, or create a GitHub
Release named
`base-images-v1.0.0-beta-dev` and upload two assets named exactly `vmlinux` and
`rootfs.ext4`. Then use **Actions → Porter Linux release → Run workflow**, enter
`v1.0.0-beta-dev` and `x86_64`, and start the run. The workflow downloads the
two separate assets and calculates SHA-256 values automatically; there are no
SHA inputs. Alternatively, smaller Git-LFS-managed files can live at
`release/guest-artifacts/x86_64/vmlinux` and
`release/guest-artifacts/x86_64/rootfs.ext4`. The workflow intentionally fails
when it cannot find either real file.

For a source checkout, use the daemon installer as root:

```bash
sudo PORTER_BASE_IMAGE_DIR=/var/porter/base-images/default \
  bash scripts/backend/install.sh
```

The installer builds `frontend/` first, writes the result to `backend/web/dist`,
compiles the Go daemon so those assets are embedded through `backend/embed.go`,
installs the checksum-pinned official Firecracker binary from GitHub, creates a
`porter` system user, installs `release/porter.service`, and enables the daemon.
It refuses to claim a working microVM runtime unless the configured directory
contains real, non-empty `vmlinux` and `rootfs.ext4` artifacts. For control-plane
development only, `PORTER_ALLOW_MISSING_BASE_IMAGE=1` can bypass that refusal;
replicas will not boot in that mode.

After installation, the dashboard is served by the same Go process at
`http://127.0.0.1:8080` by default. The database-seeded super-admin username is
`admin`. The installer generates a random bootstrap password on first install,
stores it in `/var/porter/porter.env` with root ownership and daemon-group-only
read access, and prints it once at the end. The editable runtime configuration
is `/var/porter/porter.toml`; it contains paths and non-secret settings, while
database credentials and key material remain in the protected environment file.
There is **no reusable default password in the repository**. Rotate it after
the first login.

```bash
systemctl status porter.service
journalctl -u porter.service -f
```

## Prod (single Linux + KVM host)

```bash
sudo -E bash scripts/backend/install.sh  # everything, self-contained
/usr/local/bin/porter kernel set <path|URL>   # provision vmlinux
systemctl start porter
```

`install.sh` provisions PostgreSQL for production, checks KVM and TAP
prerequisites, downloads a checksum-pinned official Firecracker binary from
GitHub, validates a real Porter base-image bundle when configured, builds the
Go daemon, and writes direct-runtime configuration. Config lands in the local
installer state directory; credentials are supplied through
`PORTER_BOOTSTRAP_ADMIN_PASSWORD` and `PORTER_SECRET_KEY`, never TOML.

For a compiled GitHub release, run `scripts/backend/build-release.sh` with
`PORTER_BASE_IMAGE_DIR` pointing to a directory containing real `vmlinux` and
`rootfs.ext4`. The builder first compiles the Vue dashboard and embeds it in the
Linux Go binary, then creates a daemon package and a separate base-image package.
Upload both assets to the Porter GitHub Release, then install with the public
`install.sh` bootstrap and a mandatory `PORTER_RELEASE_PACKAGE_SHA256` only
when the checksum sidecar is unavailable. No AWS bucket or arbitrary mirror is
used.

The current development sandbox does not expose `/dev/kvm` and does not contain
a user-supplied `vmlinux`/`rootfs.ext4` pair, so it can validate compilation,
embedding, checksums, scripts, and package structure but cannot honestly perform
a privileged Firecracker boot smoke test or produce a complete bootable guest
release without those real artifacts.

PostgreSQL stores Porter’s control-plane state. The Linux installer asks whether
to use PostgreSQL on the same host or a remote operator-managed PostgreSQL
service; local mode automatically configures the official PostgreSQL PGDG APT
repository and installs the current stable upstream server/client packages
with `--no-install-recommends`, while remote mode verifies the URL with `psql`. The
local setup creates only the dedicated `porter` application role and `porter`
database and gives that role no superuser, database-creation, or role-creation
privileges. It does not use
Docker for the Linux installation and does not place the database in a
Firecracker guest. `/var/porter` contains the editable TOML, protected
environment file, image/artifact state, and local volume backing data.

## Where does PostgreSQL run in production?

**On the host — not inside a Firecracker microVM.** MicroVMs isolate *untrusted
tenant workloads*; the DB + control plane are trusted infrastructure. Postgres
is stateful + high-IOPS (WAL, fsync) and Firecracker has no persistent disk by
design — running a DB there means fighting the platform. Host Postgres (or
managed RDS/Neon) keeps backups, WAL archiving, and replication simple.
