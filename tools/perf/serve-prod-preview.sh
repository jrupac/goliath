#!/usr/bin/env bash
# Builds the frontend in production mode and serves it with `vite preview` on
# port 4173, proxying the API to the same dev backend the dev server uses.
#
# Dev-mode React/MUI/emotion carry large instrumentation overheads (prop-types
# validation, development emotion builds, jsxDEV). Benchmarking only the dev
# server therefore overstates real user-facing latency by a wide margin. Use
# this for headline numbers; use the dev server for fast iteration.
#
# Usage: tools/perf/serve-prod-preview.sh [container]
set -euo pipefail

CONTAINER="${1:-frontend_dev}"
export DOCKER_HOST="${DOCKER_HOST:-unix:///run/user/1000/docker.sock}"

echo "==> Stopping any previous preview server"
docker exec "$CONTAINER" sh -c 'pkill -f "vite preview" || true'

echo "==> Building production bundle (this takes a minute)"
docker exec "$CONTAINER" bun run build

# The build output directory is served as the web root, so the profiling
# harness has to be copied in to remain importable at /tools/perf/...
echo "==> Copying profiling harness into the build output"
docker exec "$CONTAINER" sh -c 'mkdir -p /frontend/build/tools/perf && cp /frontend/tools/perf/*.js /frontend/build/tools/perf/'

echo "==> Starting vite preview on :4173"
docker exec -d "$CONTAINER" sh -c 'bun run vite preview --port 4173 --host >/tmp/vite-preview.log 2>&1'

for _ in $(seq 1 30); do
  if curl -sSf -m 2 -o /dev/null "http://localhost:4173/"; then
    echo "==> Ready at http://localhost:4173/"
    curl -sSI -m 5 "http://localhost:4173/" | grep -iE 'document-policy|cross-origin' || true
    exit 0
  fi
  sleep 1
done

echo "!! preview server did not come up; log follows" >&2
docker exec "$CONTAINER" cat /tmp/vite-preview.log >&2
exit 1
