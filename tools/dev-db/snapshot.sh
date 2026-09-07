#!/usr/bin/env bash
#
# Takes a consistent snapshot of a Goliath CockroachDB database.
#
# Usage:
#   tools/dev-db/snapshot.sh [name]          # default name: goliath-<utc timestamp>
#
# Environment:
#   GOLIATH_CRDB_CONTAINER  CockroachDB container   (default: crdb-dev)
#   GOLIATH_DB              Database to snapshot    (default: goliath)
#   GOLIATH_ARCHIVE_DIR     Host directory to archive the snapshot into as a
#                           .tar.gz  (default: tools/dev-db/snapshots, which is
#                           gitignored). Set to "" to skip archiving.
#
# The archive matters: the in-container copy lives under the crdb_volume that
# the snapshot exists to protect, so it does not survive losing that volume.
#
# Uses CockroachDB's native BACKUP, taken AS OF SYSTEM TIME '-10s' so the
# snapshot is transactionally consistent across all tables even while the
# feed fetcher is writing.
#
# WHY NOT A SQL TEXT DUMP: `cockroach sql --format=sql` is not round-trip safe.
# On this dataset it silently corrupted 25 of 4962 article rows, emitting a
# plain `"` inside an e'...' string as `\\"`, which restores as `\"` — an
# extra backslash in the article HTML. Row counts and the `hash` column both
# still matched, so the corruption was invisible to every cheap check.
# BACKUP/RESTORE round-trips all nine tables to identical checksums.

set -euo pipefail

CONTAINER="${GOLIATH_CRDB_CONTAINER:-crdb-dev}"
DB="${GOLIATH_DB:-goliath}"
NAME="${1:-goliath-$(date -u +%Y%m%dT%H%M%SZ)}"
ARCHIVE_DIR="${GOLIATH_ARCHIVE_DIR-$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/snapshots}"

crdb() { docker exec -i "$CONTAINER" ./cockroach sql --insecure "$@"; }

echo "Snapshotting '$DB' on '$CONTAINER' as '$NAME'" >&2

crdb --format=table -e \
  "BACKUP DATABASE $DB INTO 'nodelocal://1/$NAME' AS OF SYSTEM TIME '-10s';" >&2

echo >&2
echo "Snapshot stored in the container at:" >&2
echo "  /cockroach/cockroach-data/extern/$NAME" >&2
echo "Restore it with:" >&2
echo "  tools/dev-db/restore.sh $NAME [target-db]" >&2

if [[ -n "$ARCHIVE_DIR" ]]; then
  mkdir -p "$ARCHIVE_DIR"
  out="$ARCHIVE_DIR/$NAME.tar.gz"
  echo >&2
  echo "Archiving to $out" >&2
  docker exec "$CONTAINER" tar -czf - -C /cockroach/cockroach-data/extern "$NAME" > "$out"
  echo "Wrote $out ($(du -h "$out" | cut -f1))" >&2
  echo "Re-import with:" >&2
  echo "  docker exec -i $CONTAINER tar -xzf - -C /cockroach/cockroach-data/extern < $out" >&2
fi
