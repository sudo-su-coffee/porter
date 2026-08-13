#!/usr/bin/env bash
# Validate the Vue dashboard without starting a development server.
set -euo pipefail

REPO_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$REPO_DIR/frontend"
npm run build
cd "$REPO_DIR"
for script in scripts/backend/*.sh scripts/frontend/*.sh; do
  bash -n "$script"
done
wrappers="$(grep -rL '<script\|<template' frontend/src/views/*.vue || true)"
if [[ -n "$wrappers" ]]; then
  echo "Wrapper-only Vue files detected:" >&2
  echo "$wrappers" >&2
  exit 1
fi
chat_hits="$(grep -riE 'whatsapp|chat' frontend/src/views/ --include='*.vue' || true)"
if [[ -n "$chat_hits" ]]; then
  echo "WhatsApp/chat-specific references detected:" >&2
  echo "$chat_hits" >&2
  exit 1
fi
printf 'Frontend build and repository shell syntax checks passed. API routes=%s, Vue views=%s, router declarations=%s.\n' \
  "$(grep -c 'mux.HandleFunc' backend/internal/api/api.go)" \
  "$(find frontend/src/views -maxdepth 1 -name '*.vue' | wc -l)" \
  "$(grep -c 'path:' frontend/src/router.js)"
