#!/bin/bash
cd /mnt/d/github/porter1/backend
export PORTER_CONFIG=porter.toml
export PORTER_BOOTSTRAP_ADMIN_PASSWORD=porter-admin-123
export PORTER_SECRET_KEY=dev-secret-key-12345678901234567890
exec /tmp/porter-test >>/tmp/porter-server.log 2>&1
