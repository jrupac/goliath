#!/usr/bin/env bash
#
# Restores a snapshot taken by tools/dev-db/snapshot.sh.
#
# Usage:
#   tools/dev-db/restore.sh <snapshot-name> [target-db]
#
# The target database defaults to `goliath_test`, NOT `goliath`, so that the
# common case (seed a throwaway for destructive API testing) cannot clobber
# live data by accident. Restoring over `goliath` itself is possible but
# requires typing the database name to confirm.
#
# Environment:
#   GOLIATH_CRDB_CONTAINER  CockroachDB container  (default: crdb-dev)
#   GOLIATH_APP_USER        SQL user the backend connects as (default: goliath)
#   GOLIATH_FORCE           Set to 1 to skip the confirmation prompt
#
# To point the backend at a restored test database, change the trailing path
# segment of `dbPath` in config.ini:
#   dbPath = postgresql://goliath@crdb:26257/goliath_test?sslmode=disable
# then `goliath-cli reload --env dev`. Credentials come from the restored
# usertable, so existing client auth tokens keep working.

set -euo pipefail

CONTAINER="${GOLIATH_CRDB_CONTAINER:-crdb-dev}"
NAME="${1:?usage: restore.sh <snapshot-name> [target-db]}"
DB="${2:-goliath_test}"

crdb() { docker exec -i "$CONTAINER" ./cockroach sql --insecure "$@"; }
csv1() { crdb --format=csv -e "$1" | tail -n +2 | tr -d '\r'; }

if ! docker exec "$CONTAINER" test -d "/cockroach/cockroach-data/extern/$NAME"; then
  echo "ERROR: no snapshot named '$NAME' on container '$CONTAINER'." >&2
  echo "Available:" >&2
  docker exec "$CONTAINER" ls /cockroach/cockroach-data/extern 2>/dev/null \
    | sed 's/^/  - /' >&2 || echo "  (none)" >&2
  exit 1
fi

exists="$(csv1 "SELECT count(*) FROM [SHOW DATABASES] WHERE database_name='$DB'")"
if [[ "$exists" != "0" ]]; then
  rows="$(csv1 "SELECT count(*) FROM $DB.public.article" 2>/dev/null || echo '?')"
  echo "WARNING: database '$DB' exists and holds $rows articles." >&2
  echo "         Restoring will DROP and replace it." >&2
  if [[ "${GOLIATH_FORCE:-0}" != "1" ]]; then
    read -r -p "Type the database name to confirm: " confirm
    [[ "$confirm" == "$DB" ]] || { echo "Aborted; nothing changed." >&2; exit 1; }
  fi
  crdb -e "DROP DATABASE $DB CASCADE;" >/dev/null
fi

# The database inside the backup is read from the backup rather than assumed to
# be `goliath`. snapshot.sh can be pointed at another database, and hardcoding
# the name here made those snapshots impossible to restore.
SRC_DB="$(csv1 "SELECT database_name FROM [SHOW BACKUP FROM LATEST IN 'nodelocal://1/$NAME'] WHERE database_name IS NOT NULL LIMIT 1")"
if [[ -z "$SRC_DB" ]]; then
  echo "ERROR: could not determine which database snapshot '$NAME' holds." >&2
  exit 1
fi

echo "Restoring '$NAME' ($SRC_DB) -> '$DB'" >&2
crdb --format=table -e \
  "RESTORE DATABASE $SRC_DB FROM LATEST IN 'nodelocal://1/$NAME' WITH new_db_name = $DB;" >&2

# RESTORE creates the database owned by root and does NOT carry over the
# original database's grants, so the application user has no access to the
# restored copy. Without this the backend starts, connects successfully, and
# then dies on the first query with "user <x> does not have SELECT privilege
# on relation usertable" — which reads like a corrupt restore but is only a
# missing grant.
APP_USER="${GOLIATH_APP_USER:-goliath}"
echo "Granting privileges on '$DB' to '$APP_USER'" >&2
crdb -e "
  GRANT ALL ON DATABASE $DB TO $APP_USER;
  GRANT ALL ON SCHEMA $DB.public TO $APP_USER;
  GRANT ALL ON ALL TABLES IN SCHEMA $DB.public TO $APP_USER;" >/dev/null

echo >&2
echo "Row counts in '$DB':" >&2
crdb --database="$DB" --format=table -e "
  SELECT 'article' AS t, count(*) FROM article
  UNION ALL SELECT 'feed', count(*) FROM feed
  UNION ALL SELECT 'folder', count(*) FROM folder
  UNION ALL SELECT 'usertable', count(*) FROM usertable
  ORDER BY t;" >&2
