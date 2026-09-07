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
# Row counts are not evidence of a good restore. A round trip can alter the
# contents of a column while leaving every row count, and every derived column,
# identical. This hashes each row's whole tuple and aggregates, so a byte-level
# difference anywhere shows up.
#
# The table list is read from both databases rather than fixed here. A table
# present on only one side is a difference in its own right and is reported as
# one, and a schema change needs no edit to this script.
#
# Rows are digested individually before aggregation; string_agg over raw article
# content exceeds CockroachDB's default window memory budget.

set -euo pipefail

CONTAINER="${GOLIATH_CRDB_CONTAINER:-crdb-dev}"
A="${1:?usage: verify.sh <db-a> <db-b>}"
B="${2:?usage: verify.sh <db-a> <db-b>}"

TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

csv() { # <db> <query>
  docker exec -i "$CONTAINER" ./cockroach sql --insecure --database="$1" \
    --format=csv -e "$2" | tail -n +2 | tr -d '\r'
}

tables() { # <db>
  csv "$1" "SELECT table_name FROM information_schema.tables
            WHERE table_schema = 'public' AND table_type = 'BASE TABLE';"
}

sums() { # <db> <table>...
  local db="$1"
  shift
  (($#)) || return 0

  local parts=() t
  for t in "$@"; do
    parts+=("SELECT '$t' AS t, md5(string_agg(h,'|' ORDER BY h)) AS s FROM (SELECT md5(x::TEXT) h FROM \"$t\" x)")
  done

  local q
  q="$(printf '%s UNION ALL ' "${parts[@]}")"
  csv "$db" "${q% UNION ALL } ORDER BY 1;"
}

echo "Comparing '$A' vs '$B' on '$CONTAINER'" >&2
echo >&2

tables "$A" | LC_ALL=C sort >"$TMP/a"
tables "$B" | LC_ALL=C sort >"$TMP/b"

comm -23 "$TMP/a" "$TMP/b" >"$TMP/only_a"
comm -13 "$TMP/a" "$TMP/b" >"$TMP/only_b"
comm -12 "$TMP/a" "$TMP/b" >"$TMP/both"

mapfile -t shared <"$TMP/both"

: >"$TMP/sums_a"
: >"$TMP/sums_b"
if ((${#shared[@]})); then
  sums "$A" "${shared[@]}" >"$TMP/sums_a"
  sums "$B" "${shared[@]}" >"$TMP/sums_b"
fi

rc=0

# Reading from a file rather than a pipe keeps the loop in this shell, so that
# what it records about mismatches survives it.
while IFS=, read -r t sa sb; do
  if [[ "$sa" == "$sb" ]]; then
    printf '  ok    %-22s %s\n' "$t" "${sa:0:12}…"
  else
    printf '  DIFF  %-22s %s != %s\n' "$t" "${sa:0:12}…" "${sb:0:12}…"
    rc=1
  fi
done < <(join -t, "$TMP/sums_a" "$TMP/sums_b")

while read -r t; do
  printf '  ONLY  %-22s in %s, absent from %s\n' "$t" "$A" "$B"
  rc=1
done <"$TMP/only_a"

while read -r t; do
  printf '  ONLY  %-22s in %s, absent from %s\n' "$t" "$B" "$A"
  rc=1
done <"$TMP/only_b"

# Two databases that both report nothing are not a match; the likelier
# explanation is that neither name resolved.
if [[ ! -s "$TMP/a" && ! -s "$TMP/b" ]]; then
  echo "  no tables found in either database" >&2
  rc=1
fi

echo >&2
if ((rc)); then
  echo "RESULT: databases DIFFER" >&2
else
  echo "RESULT: identical" >&2
fi
exit "$rc"
