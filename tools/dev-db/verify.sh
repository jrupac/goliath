#!/usr/bin/env bash
#
# Compares two Goliath databases table by table, by content checksum.
#
# Usage:
#   tools/dev-db/verify.sh <db-a> <db-b>
#
# Environment:
#   GOLIATH_CRDB_CONTAINER  CockroachDB container (default: crdb-dev)
#
# Row counts are not evidence of a good restore: a text-dump round trip was
# found to corrupt 25 article rows while leaving every row count, and the
# `hash` column, identical. This hashes each row's full tuple and aggregates,
# so any byte-level difference in any column shows up.
#
# Rows are digested individually before aggregation; string_agg over raw
# article content exceeds CockroachDB's default window memory budget.

set -euo pipefail

CONTAINER="${GOLIATH_CRDB_CONTAINER:-crdb-dev}"
A="${1:?usage: verify.sh <db-a> <db-b>}"
B="${2:?usage: verify.sh <db-a> <db-b>}"

TABLES=(usertable folder folderchildren feed article userprefs
        userunmutefeeds userfeedmuteregexes retrievalcache)

sums() {
  local db="$1" parts=()
  for t in "${TABLES[@]}"; do
    parts+=("SELECT '$t' AS t, md5(string_agg(h,'|' ORDER BY h)) AS s FROM (SELECT md5(x::TEXT) h FROM $t x)")
  done
  local q; q="$(printf '%s UNION ALL ' "${parts[@]}")"
  docker exec -i "$CONTAINER" ./cockroach sql --insecure --database="$db" \
    --format=csv -e "${q% UNION ALL } ORDER BY 1;" 2>/dev/null | tail -n +2 | tr -d '\r'
}

echo "Comparing '$A' vs '$B' on '$CONTAINER'" >&2
echo >&2

rc=0
join -t, -a1 -a2 <(sums "$A") <(sums "$B") | while IFS=, read -r t sa sb; do
  if [[ "$sa" == "$sb" && -n "$sa" ]]; then
    printf '  ok    %-22s %s\n' "$t" "${sa:0:12}…"
  else
    printf '  DIFF  %-22s %s != %s\n' "$t" "${sa:0:12}…" "${sb:0:12}…"
    rc=1
  fi
done

# `while` runs in a subshell, so re-derive the exit status.
if ! diff -q <(sums "$A") <(sums "$B") >/dev/null; then
  echo >&2
  echo "RESULT: databases DIFFER" >&2
  exit 1
fi
echo >&2
echo "RESULT: identical" >&2
