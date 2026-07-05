#!/usr/bin/env bash
set -euo pipefail

# S89-1: executable end-to-end restore drill. Spins up a source Postgres,
# seeds it, runs backup.sh, then restores the archive into a SEPARATE fresh
# Postgres via restore.sh and verifies the data round-trips — measuring the
# restore RTO. Also confirms a tampered archive is rejected.
#
# Requires: docker, pg_dump, psql, openssl, bash.
# Usage: bash scripts/restore_drill.sh

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

SECRET_KEY="0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
PASSPHRASE="drill-passphrase-12+"
SRC_NAME="vakt-drill-src-$$"
DST_NAME="vakt-drill-dst-$$"
WORK="$(mktemp -d)"

cleanup() {
	docker rm -f "$SRC_NAME" "$DST_NAME" >/dev/null 2>&1 || true
	rm -rf "$WORK"
}
trap cleanup EXIT

wait_pg() { # $1 = container name
	# Require 2 consecutive successful checks 1s apart: the official postgres
	# image restarts once during init, and a single pg_isready can hit the
	# short-lived init instance — the host connection then fails (S120-10).
	local streak=0
	for _ in $(seq 1 60); do
		if docker exec "$1" pg_isready -U vakt >/dev/null 2>&1; then
			streak=$((streak + 1))
			if [ "$streak" -ge 2 ]; then return 0; fi
		else
			streak=0
		fi
		sleep 1
	done
	echo "ERROR: postgres $1 did not become ready" >&2
	return 1
}

echo "→ [drill] starting source + target Postgres"
docker run -d --name "$SRC_NAME" -e POSTGRES_USER=vakt -e POSTGRES_PASSWORD=vakt -e POSTGRES_DB=vakt -p 0:5432 postgres:18-alpine >/dev/null
docker run -d --name "$DST_NAME" -e POSTGRES_USER=vakt -e POSTGRES_PASSWORD=vakt -e POSTGRES_DB=vakt -p 0:5432 postgres:18-alpine >/dev/null
wait_pg "$SRC_NAME"
wait_pg "$DST_NAME"

SRC_PORT="$(docker port "$SRC_NAME" 5432/tcp | head -1 | sed 's/.*://')"
DST_PORT="$(docker port "$DST_NAME" 5432/tcp | head -1 | sed 's/.*://')"
SRC_URL="postgres://vakt:vakt@127.0.0.1:${SRC_PORT}/vakt?sslmode=disable"
DST_URL="postgres://vakt:vakt@127.0.0.1:${DST_PORT}/vakt?sslmode=disable"

echo "→ [drill] seeding source data"
psql "$SRC_URL" -q -c "CREATE TABLE drill_evidence (id serial primary key, note text);" \
	-c "INSERT INTO drill_evidence (note) VALUES ('audit-trail-row-1'),('audit-trail-row-2');"

echo "→ [drill] creating backup"
( cd "$WORK" && VAKT_DB_URL="$SRC_URL" VAKT_SECRET_KEY="$SECRET_KEY" VAKT_BACKUP_PASSPHRASE="$PASSPHRASE" \
	bash "$SCRIPT_DIR/backup.sh" "$WORK" >/dev/null )
ARCHIVE="$(find "$WORK" -name 'vakt-backup-*.tar.gz' | head -1)"
[ -n "$ARCHIVE" ] || { echo "ERROR: no archive produced" >&2; exit 1; }

echo "→ [drill] restoring into FRESH target (measuring RTO)"
START=$(date +%s.%N)
printf 'y\n' | ( cd "$WORK" && VAKT_DB_URL="$DST_URL" VAKT_SECRET_KEY="$SECRET_KEY" VAKT_BACKUP_PASSPHRASE="$PASSPHRASE" \
	bash "$SCRIPT_DIR/restore.sh" "$ARCHIVE" >/dev/null )
# Verify the data round-tripped (this is the "app usable" proxy).
ROWS="$(psql "$DST_URL" -tAc "SELECT count(*) FROM drill_evidence;")"
END=$(date +%s.%N)

if [ "$ROWS" != "2" ]; then
	echo "ERROR: restore verification failed — expected 2 rows, got '$ROWS'" >&2
	exit 1
fi
RTO=$(awk "BEGIN{printf \"%.1f\", $END-$START}")
echo "✓ [drill] restore verified: $ROWS rows recovered into a fresh database"
echo "✓ [drill] measured restore RTO (DB-level): ${RTO}s"

echo "→ [drill] negative test: tampered archive must be rejected"
cp "$ARCHIVE" "$WORK/tampered.tar.gz"; cp "${ARCHIVE}.sig" "$WORK/tampered.tar.gz.sig"
printf 'tamper' >> "$WORK/tampered.tar.gz"
set +e
( cd "$WORK" && VAKT_DB_URL="$DST_URL" VAKT_SECRET_KEY="$SECRET_KEY" VAKT_BACKUP_PASSPHRASE="$PASSPHRASE" \
	bash "$SCRIPT_DIR/restore.sh" "$WORK/tampered.tar.gz" --dry-run >/dev/null 2>&1 )
RC=$?
set -e
[ "$RC" -ne 0 ] || { echo "ERROR: tampered archive was NOT rejected" >&2; exit 1; }
echo "✓ [drill] tampered archive rejected"

echo ""
echo "DRILL PASSED — date=$(date -u +%Y-%m-%d) db_level_rto=${RTO}s rows=${ROWS}"
echo "$REPO_ROOT" >/dev/null
