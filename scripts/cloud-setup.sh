#!/usr/bin/env bash
# cloud-setup.sh — runs on SessionStart in Claude Code cloud (CLAUDE_CODE_REMOTE=1)
# Pulls the pre-built dev image so all tools are available via docker run.
# No-op on local machines where CLAUDE_CODE_REMOTE is unset.
set -euo pipefail

if [ "${CLAUDE_CODE_REMOTE:-}" != "1" ]; then
    exit 0
fi

IMAGE="ghcr.io/vbonduro/kitchinv-dev:latest"

echo "[cloud-setup] pulling dev image $IMAGE ..."
docker pull "$IMAGE"
echo "[cloud-setup] done. Run commands with:"
echo "  docker run --rm -v \"\$PWD\":/workspace -w /workspace $IMAGE <cmd>"
