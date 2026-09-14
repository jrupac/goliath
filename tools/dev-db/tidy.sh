#!/usr/bin/env bash
#
# Puts the development database back in its standing shape: `goliath` is the
# database the dev backend serves, at the newest migration in the checkout,
# with the backend running the checkout's code, and a snapshot of it taken
# afterwards and checked by restoring it.
#
# Usage:
#   tools/dev-db/tidy.sh
#
# Environment:
#   GOLIATH_CRDB_CONTAINER  CockroachDB container (default: crdb-dev)
#   GOLIATH_VERSION_URL     Where the dev backend reports its build and schema
#                           (default: http://127.0.0.1:9999/version)
#   DOCKER_HOST             As for the rest of the dev stack
#
# It removes nothing it did not create. Databases other than `goliath`, older
# snapshots and migration checkpoints are listed at the end, with the commands
# that remove them, for a person to run once sure they are not wanted.
#
# The migration goes through `goliath-cli migrate-schema` rather than
# `upgrade`, since `upgrade` prunes older checkpoints on its way out.
#
# The snapshot is taken with the backend stopped. With the fetcher writing, the
# database has moved on by the time the restored copy is compared with it, and
# a comparison that is expected to differ checks nothing.

set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
HERE="$ROOT/tools/dev-db"
cd "$ROOT"

CONTAINER="${GOLIATH_CRDB_CONTAINER:-crdb-dev}"
VERSION_URL="${GOLIATH_VERSION_URL:-http://127.0.0.1:9999/version}"
DB=goliath
KEEP_DBS="'$DB', 'defaultdb', 'postgres', 'system'"
CHECKPOINTS='nodelocal://1/goliath-checkpoints'

# compose.yaml interpolates a few variables with no default, for services this
# script never starts, and every compose command warns about each one unset,
# goliath-cli's included. A blank export quiets that. One that .env sets is
# left alone, since a variable in this shell would override it.
for var in $(grep -o -E '\$\{[A-Z_][A-Z0-9_]*\}' compose.yaml | tr -d '${}' | sort -u); do
  if [[ -z "${!var+set}" ]] && ! grep -q -E "^[[:space:]]*(export[[:space:]]+)?$var=" .env 2>/dev/null; then
    export "$var="
  fi
done

crdb() { docker exec -i "$CONTAINER" ./cockroach sql --insecure "$@"; }
csv() { crdb --format=csv -e "$1" | tail -n +2 | tr -d '\r'; }
step() { printf '\n==> %s\n' "$*" >&2; }
compose() { docker compose --profile dev "$@"; }

tmp="$(mktemp -d)"
scratch=""
backend_stopped=0
cleanup() {
  if [[ -n "$scratch" ]]; then
    crdb -e "DROP DATABASE IF EXISTS $scratch CASCADE;" >/dev/null 2>&1 || true
  fi
  if ((backend_stopped)); then
    echo "Starting backend-dev again after a failure." >&2
    compose start backend-dev >/dev/null 2>&1 || true
  fi
  rm -rf "$tmp"
}
trap cleanup EXIT

# Waits for the backend to report the build it was given and a schema matching
# the checkout's newest migration.
wait_for_backend() { # <build hash>
  local want_hash="$1" v
  for _ in $(seq 1 120); do
    if v="$(curl -fsS "$VERSION_URL" 2>/dev/null)" &&
      [[ "$(jq -r .build_hash <<<"$v")" == "$want_hash" ]] &&
      [[ "$(jq -r .schema_version <<<"$v")" == "$newest" ]] &&
      [[ "$(jq -r .db_schema_version <<<"$v")" == "$newest" ]]; then
      echo "  $v" >&2
      return 0
    fi
    sleep 1
  done
  echo "The backend did not report build $want_hash at schema v$newest within 2 minutes:" >&2
  echo "  ${v:-no answer from $VERSION_URL}" >&2
  return 1
}

# The rest is about `goliath`, so a config pointing the backend elsewhere is the
# first thing to put right.
served="$(sed -nE 's#^[[:space:]]*dbPath[[:space:]]*=.*/([A-Za-z0-9_]+)(\?.*)?$#\1#p' config.ini | tail -1)"
if [[ "$served" != "$DB" ]]; then
  echo "config.ini points the dev backend at '${served:-?}', not '$DB'; fix dbPath first." >&2
  exit 1
fi

newest="$(find backend/schema -maxdepth 1 -name 'v[0-9]*.sql' -printf '%f\n' |
  sed -E 's/^v([0-9]+).*/\1/' | sort -n | tail -1)"

step "Building goliath-cli from the checkout"
(cd cmd/goliath-cli && go build -o "$tmp/goliath-cli" .)
CLI="$tmp/goliath-cli"

step "Bringing $DB to v$newest"
"$CLI" migrate-schema --env dev

step "Rebuilding the dev backend at the checkout"
hash="$(git rev-parse HEAD)"
[[ -z "$(git status --porcelain --untracked-files=no)" ]] || hash="$hash-dirty"
BUILD_HASH="$hash" BUILD_TIMESTAMP="$(date +%s)" compose up -d --build --no-deps backend-dev
wait_for_backend "$hash"

step "Snapshotting $DB with the backend stopped"
compose stop backend-dev
backend_stopped=1
# snapshot.sh reads as of ten seconds ago, which has to be after the backend's
# last write, the retrieval cache it persists on the way out included.
sleep 11
name="$DB-v$newest-$(date -u +%Y%m%dT%H%M%SZ)"
"$HERE/snapshot.sh" "$name"

step "Checking the snapshot by restoring it"
scratch="snapshot_check_$(date +%s)"
"$HERE/restore.sh" "$name" "$scratch" 2>/dev/null
"$HERE/verify.sh" "$DB" "$scratch"
crdb -e "DROP DATABASE $scratch CASCADE;" >/dev/null
scratch=""

step "Starting the dev backend"
compose start backend-dev
backend_stopped=0
wait_for_backend "$hash"

step "Left for you to decide on"
others="$(csv "SELECT database_name FROM [SHOW DATABASES] WHERE database_name NOT IN ($KEEP_DBS) ORDER BY 1")"
if [[ -n "$others" ]]; then
  echo "Databases other than $DB (DROP DATABASE <name> CASCADE; in goliath-cli sql --env dev):" >&2
  sed 's/^/  /' <<<"$others" >&2
fi
snapshots="$(docker exec "$CONTAINER" ls /cockroach/cockroach-data/extern |
  grep -v -x -e "$name" -e goliath-checkpoints || true)"
if [[ -n "$snapshots" ]]; then
  echo "Older snapshots in $CONTAINER (docker exec $CONTAINER rm -rf /cockroach/cockroach-data/extern/<name>):" >&2
  sed 's/^/  /' <<<"$snapshots" >&2
fi
archives="$(find "$HERE/snapshots" -maxdepth 1 -name '*.tar.gz' ! -name "$name.tar.gz" -printf '%f\n' 2>/dev/null | sort)"
if [[ -n "$archives" ]]; then
  echo "Older archives in $HERE/snapshots:" >&2
  sed 's/^/  /' <<<"$archives" >&2
fi
# Listed here rather than by goliath-cli list-checkpoints, which does not say
# which database each checkpoint holds, and prints every rollback command as
# though it were this one's.
checkpoints="$(csv "SHOW BACKUPS IN '$CHECKPOINTS'" 2>/dev/null | grep -E '^/[0-9][0-9/.-]*$' || true)"
if [[ -n "$checkpoints" ]]; then
  echo "Migration checkpoints, oldest first, with the database each holds:" >&2
  # Read from its own descriptor: docker exec -i in the loop would otherwise
  # consume the rest of the list from standard input.
  while read -r cp <&3; do
    cpdb="$(csv "SELECT DISTINCT database_name FROM [SHOW BACKUP FROM '$cp' IN '$CHECKPOINTS'] WHERE database_name IS NOT NULL" | paste -sd, -)"
    echo "  $cp (${cpdb:-?})" >&2
    echo "    restore: goliath-cli rollback-schema --env dev --database ${cpdb:-<db>} --checkpoint $cp" >&2
    echo "    remove:  docker exec $CONTAINER rm -rf /cockroach/cockroach-data/extern/goliath-checkpoints$cp" >&2
  done 3<<<"$checkpoints"
fi

echo >&2
echo "Done: $DB at v$newest, the backend at $hash, snapshot $name." >&2
