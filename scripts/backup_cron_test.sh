#!/usr/bin/env bash
set -euo pipefail

# S89-4 test: verifies backup-cron.sh retention + failure-notification logic
# without a database.
#   1. prune deletes archives older than the retention window and keeps recent ones.
#   2. a failed backup run triggers the notification hook.

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CRON_SH="${SCRIPT_DIR}/backup-cron.sh"

fail() { echo "FAIL: $1" >&2; exit 1; }
pass() { echo "PASS: $1"; }

WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT

# ── 1: retention ──────────────────────────────────────────────────────────
BDIR="$WORK/backups"
mkdir -p "$BDIR"

# Old archive + sig (40 days) — should be pruned with a 30-day window.
touch -d '40 days ago' "$BDIR/vakt-backup-old.tar.gz" "$BDIR/vakt-backup-old.tar.gz.sig"
# Recent archive + sig (2 days) — should be kept.
touch -d '2 days ago' "$BDIR/vakt-backup-new.tar.gz" "$BDIR/vakt-backup-new.tar.gz.sig"
# Unrelated file — must never be touched.
touch -d '40 days ago' "$BDIR/keep-me.txt"

VAKT_BACKUP_DIR="$BDIR" VAKT_BACKUP_RETENTION_DAYS=30 bash "$CRON_SH" prune >/dev/null

[ ! -f "$BDIR/vakt-backup-old.tar.gz" ]     || fail "old archive not pruned"
[ ! -f "$BDIR/vakt-backup-old.tar.gz.sig" ] || fail "old sig not pruned"
[ -f "$BDIR/vakt-backup-new.tar.gz" ]       || fail "recent archive wrongly pruned"
[ -f "$BDIR/vakt-backup-new.tar.gz.sig" ]   || fail "recent sig wrongly pruned"
[ -f "$BDIR/keep-me.txt" ]                  || fail "unrelated file wrongly deleted"
pass "retention prunes old archives, keeps recent + unrelated files"

# ── 2: failure notification ───────────────────────────────────────────────
# Run the full cycle with NO database configured so backup.sh fails; assert the
# notification hook fires. Run in a clean CWD so no real .env is sourced.
MARKER="$WORK/notified.txt"
set +e
(
	cd "$WORK"
	VAKT_BACKUP_DIR="$WORK/out" \
	VAKT_BACKUP_NOTIFY_CMD="echo \"\$MESSAGE\" > '$MARKER'" \
	VAKT_DB_URL="" VAKT_SECRET_KEY="" \
	bash "$CRON_SH" run >/dev/null 2>&1
)
RC=$?
set -e
[ "$RC" -ne 0 ] || fail "cycle should fail when backup.sh cannot run"
[ -f "$MARKER" ] || fail "failure did not trigger the notification hook"
grep -q "backup.sh failed" "$MARKER" || fail "notification message missing"
pass "failed backup triggers notification hook"

echo "ALL TESTS PASSED"
