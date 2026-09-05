#!/usr/bin/env bash
# Resets every article in the dev database to unread/unsaved, so that repeated
# runs of the keypress benchmark start from an identical state.
#
# Usage: tools/perf/reset-read-state.sh [container]
set -euo pipefail

CONTAINER="${1:-crdb-dev}"
export DOCKER_HOST="${DOCKER_HOST:-unix:///run/user/1000/docker.sock}"

docker exec "$CONTAINER" ./cockroach sql --insecure --host=crdb --database=goliath -e \
  "UPDATE Article SET read = false, saved = false WHERE read OR saved;" >/dev/null

docker exec "$CONTAINER" ./cockroach sql --insecure --host=crdb --database=goliath -e \
  "SELECT count(*) AS total,
          count(*) FILTER (WHERE NOT read) AS unread,
          count(*) FILTER (WHERE saved)    AS saved
   FROM Article;"
