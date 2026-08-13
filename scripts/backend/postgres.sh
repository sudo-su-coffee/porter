#!/usr/bin/env bash
# Shared PostgreSQL setup for the Linux Porter installers.
# This helper never starts Docker or installs a database inside a microVM.

porter_pg_die() { printf 'FAIL: %s\n' "$1" >&2; return 1; }
porter_pg_note() { printf '\n==> %s\n' "$1"; }

porter_pg_prompt() {
  local prompt="$1" variable="$2" input='/dev/stdin'
  if [ -r /dev/tty ]; then input='/dev/tty'; fi
  IFS= read -r -p "$prompt" "$variable" < "$input"
}

porter_pg_random_password() {
  if command -v openssl >/dev/null 2>&1; then openssl rand -hex 24
  else od -An -N24 -tx1 /dev/urandom | tr -d ' \n'
  fi
}

porter_pg_as_postgres() {
  if command -v runuser >/dev/null 2>&1; then runuser -u postgres -- "$@"
  elif command -v sudo >/dev/null 2>&1; then sudo -u postgres "$@"
  else porter_pg_die "runuser or sudo is required to administer local PostgreSQL"
  fi
}

porter_pg_install_packages() {
  command -v apt-get >/dev/null 2>&1 || porter_pg_die "PostgreSQL is missing; install it with the host package manager, then rerun"
  if [ "${PORTER_INSTALL_SYSTEM_DEPS:-0}" != 1 ]; then
    if [ -t 0 ] || [ -r /dev/tty ]; then
      local install_deps
      porter_pg_prompt "PostgreSQL is not installed. Install PostgreSQL server and client now? [Y/n]: " install_deps
      case "${install_deps:-y}" in
        y|Y|yes|YES) export PORTER_INSTALL_SYSTEM_DEPS=1 ;;
        *) porter_pg_die "PostgreSQL is missing; install it first or rerun with PORTER_INSTALL_SYSTEM_DEPS=1" ;;
      esac
    else
      porter_pg_die "PostgreSQL client/server is missing; install it first or rerun with PORTER_INSTALL_SYSTEM_DEPS=1 on Debian/Ubuntu"
    fi
  fi
  export DEBIAN_FRONTEND=noninteractive
  apt-get update
  apt-get install -y postgresql postgresql-client
}

porter_pg_setup_local() {
  porter_pg_note "Local host PostgreSQL"
  if ! command -v psql >/dev/null 2>&1 || ! command -v pg_isready >/dev/null 2>&1; then
    porter_pg_install_packages
  fi
  systemctl enable --now postgresql >/dev/null 2>&1 || true
  pg_isready -h 127.0.0.1 -p "${PORTER_PG_PORT:-5432}" >/dev/null 2>&1 || porter_pg_die "local PostgreSQL is not ready on 127.0.0.1:${PORTER_PG_PORT:-5432}"

  local user="${PORTER_PG_USER:-porter}" db="${PORTER_PG_DB:-porter}" port="${PORTER_PG_PORT:-5432}" password
  [[ "$user" =~ ^[a-z_][a-z0-9_]*$ ]] || porter_pg_die "PORTER_PG_USER must be a simple PostgreSQL identifier"
  [[ "$db" =~ ^[a-z_][a-z0-9_]*$ ]] || porter_pg_die "PORTER_PG_DB must be a simple PostgreSQL identifier"
  password="${PORTER_PG_PASSWORD:-$(porter_pg_random_password)}"
  if porter_pg_as_postgres psql -tAc "SELECT 1 FROM pg_roles WHERE rolname = '$user'" | grep -q 1; then
    porter_pg_as_postgres psql -v porter_password="$password" -v ON_ERROR_STOP=1 -c "ALTER ROLE \"$user\" WITH LOGIN PASSWORD :'porter_password'" >/dev/null
  else
    porter_pg_as_postgres psql -v porter_password="$password" -v ON_ERROR_STOP=1 -c "CREATE ROLE \"$user\" LOGIN PASSWORD :'porter_password'" >/dev/null
  fi
  if ! porter_pg_as_postgres psql -tAc "SELECT 1 FROM pg_database WHERE datname = '$db'" | grep -q 1; then
    porter_pg_as_postgres createdb -O "$user" "$db"
  fi
  PORTER_DATABASE_URL="postgres://${user}:${password}@127.0.0.1:${port}/${db}?sslmode=disable"
  export PORTER_DATABASE_URL
  psql "$PORTER_DATABASE_URL" -Atqc 'select 1' >/dev/null || porter_pg_die "could not connect to the configured local Porter database"
  printf '    database: %s@127.0.0.1:%s/%s\n' "$user" "$port" "$db"
}

porter_pg_setup_remote() {
  porter_pg_note "Operator-managed PostgreSQL"
  [ -n "${PORTER_DATABASE_URL:-}" ] || {
    if [ -t 0 ] || [ -r /dev/tty ]; then
      porter_pg_prompt "PostgreSQL connection URL (password may be embedded or supplied by your environment): " PORTER_DATABASE_URL
    fi
  }
  [ -n "${PORTER_DATABASE_URL:-}" ] || porter_pg_die "PORTER_DATABASE_URL is required for remote PostgreSQL mode"
  command -v psql >/dev/null 2>&1 || porter_pg_die "psql is required to verify the remote PostgreSQL URL"
  psql "$PORTER_DATABASE_URL" -Atqc 'select 1' >/dev/null || porter_pg_die "remote PostgreSQL connection check failed"
  export PORTER_DATABASE_URL
  printf '    database URL verified (credentials are not printed)\n'
}

porter_pg_setup() {
  local mode="${PORTER_POSTGRES_MODE:-}"
  if [ -z "$mode" ] && { [ -t 0 ] || [ -r /dev/tty ]; }; then
    printf '\nChoose the PostgreSQL data-store mode:\n'
    printf '  1) local host PostgreSQL (install/use PostgreSQL on this Linux server)\n'
    printf '  2) remote PostgreSQL (managed database or another server)\n'
    porter_pg_prompt 'Select [1/2]: ' choice
    case "$choice" in 1) mode=local ;; 2) mode=remote ;; *) porter_pg_die 'choose 1 or 2' ;; esac
  fi
  case "$mode" in
    local) porter_pg_setup_local ;;
    remote) porter_pg_setup_remote ;;
    *) porter_pg_die 'set PORTER_POSTGRES_MODE=local or remote; for curl | sudo bash use sudo PORTER_POSTGRES_MODE=local bash or choose a mode when prompted' ;;
  esac
}
