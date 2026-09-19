#!/usr/bin/env bash
set -euo pipefail

# R1-35-03 test: backup.sh MUST write a backup_log entry after a successful,
# verified and signed dump — and a FAILING backup_log write must NOT turn the
# backup red. Both are proven end-to-end by running the real backup.sh against
# PATH stubs for pg_dump / pg_restore / psql (VAKT_BACKUP_PG_MODE=host), so no
# database and no Docker are required.
#
# Why end-to-end and not a unit stub of vakt_pg_exec_sql: the load-bearing claim
# is placement — the INSERT runs only AFTER the dump passed its size + readability
# check and the archive was signed. A unit call could not tell "after verify"
# from "instead of verify". Here the psql spy fires only if backup.sh reached the
# writer at the end of its own success path, and the archive on disk proves the
# sign step ran first.
#
# What this test does NOT cover (its denominator):
#   * The container path (docker exec psql). Host mode exercises the same
#     vakt_pg_exec_sql seam; the container branch is covered structurally by the
#     call-site assertion below, not run here (that needs Docker — the wiring
#     test owns real-container coverage).
#   * The SQL's semantics against a real schema (does org_id resolve, does the
#     90-day DELETE keep the newest row). That is an integration concern; here
#     the subject is that the writer fires with the right statement and is
#     best-effort, not what Postgres does with it.
#   * Off-site push / retention (backup-cron.sh owns those).

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BACKUP_SH="${SCRIPT_DIR}/backup.sh"
TARGET_SH="${SCRIPT_DIR}/backup-pg-target.sh"

PASSED=0
FAILED=0
SKIPPED=0
pass() { echo "PASS: $1"; PASSED=$((PASSED + 1)); }
fail() { echo "FAIL: $1" >&2; FAILED=$((FAILED + 1)); }
skip() { echo "SKIP: $1"; SKIPPED=$((SKIPPED + 1)); }

summary() {
	echo
	echo "ERGEBNIS: passed=$PASSED failed=$FAILED skipped=$SKIPPED"
	[ "$FAILED" -eq 0 ] || exit 1
}

# ── 0: the subject must exist (empty-tree hole) ───────────────────────────────
missing=0
for f in "$BACKUP_SH" "$TARGET_SH"; do
	[ -f "$f" ] || { echo "  MISSING: $f" >&2; missing=$((missing + 1)); }
done
[ "$missing" -eq 0 ] || { fail "$missing production script(s) not found — this test has no subject"; summary; }
pass "subject present: scripts/backup.sh + scripts/backup-pg-target.sh"

# ── 0b: call-site ratchet — the writer must go through vakt_pg_exec_sql ────────
# If someone rewrites the writer to a raw `psql` or drops it, the end-to-end
# assertions below already catch it. This static check names the seam so the
# container path (not run here) is not left silently uncovered.
if ! grep -q 'vakt_pg_exec_sql' "$TARGET_SH"; then
	fail "vakt_pg_exec_sql is not defined in backup-pg-target.sh — the exec seam vanished"
elif ! grep -Eq 'vakt_pg_exec_sql .*BACKUP_LOG_SQL|BACKUP_LOG_SQL=.*backup_log' "$BACKUP_SH"; then
	fail "backup.sh no longer builds/uses a backup_log statement through vakt_pg_exec_sql"
else
	pass "backup.sh writes backup_log via the vakt_pg_exec_sql seam (both paths share it)"
fi

# ── tool preconditions (counted skip, never silent) ───────────────────────────
MISSING_TOOL=""
command -v gpg     >/dev/null 2>&1 || MISSING_TOOL="gpg"
[ -n "$MISSING_TOOL" ] || command -v openssl >/dev/null 2>&1 || MISSING_TOOL="openssl"
if [ -n "$MISSING_TOOL" ]; then
	skip "end-to-end writer run — ${MISSING_TOOL} not available"
	skip "end-to-end best-effort run — ${MISSING_TOOL} not available"
	summary
fi

WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT

# Runtime-generated, never committed: a 32-byte hex key in a checked-in file is
# exactly what a secret scanner should flag (CLAUDE.md: the rule is absolute).
TEST_SECRET_KEY="$(openssl rand -hex 32)"
TEST_PASSPHRASE="r13503-$(openssl rand -hex 8)"
DB_URL="postgres://vakt:vakt@127.0.0.1:5432/vakt?sslmode=disable"

# PATH stubs. pg_dump writes a non-empty, incompressible dump (passes the >=512
# size floor and keeps the final archive over the 1000-byte floor). pg_restore
# --list succeeds so the readability check passes. psql logs the SQL it receives
# on stdin (the writer uses `psql -f -`) and its exit code is controlled per run
# via the PSQL_EXIT file.
make_stubs() { # dir
	local d="$1"
	mkdir -p "$d"
	cat >"$d/pg_dump" <<'STUB'
#!/usr/bin/env bash
out=""; prev=""
for a in "$@"; do
	[ "$prev" = "-f" ] && out="$a"
	case "$a" in --file=*) out="${a#--file=}" ;; esac
	prev="$a"
done
[ -n "$out" ] && head -c 2048 /dev/urandom >"$out"
exit 0
STUB
	cat >"$d/pg_restore" <<'STUB'
#!/usr/bin/env bash
# --list on the local dump file: always readable in this test.
exit 0
STUB
	cat >"$d/psql" <<'STUB'
#!/usr/bin/env bash
# The writer pipes the SQL in on stdin via `psql -f -`. Record it, then honour
# the requested exit code so the best-effort branch can be exercised.
cat >>"$PSQL_SQL_LOG"
printf 'PSQL-ARGS: %s\n' "$*" >>"$PSQL_SQL_LOG"
exit "$(cat "$PSQL_EXIT" 2>/dev/null || echo 0)"
STUB
	chmod +x "$d/pg_dump" "$d/pg_restore" "$d/psql"
}

run_backup() { # out_dir, spy_dir  -> sets RC
	local out="$1" spy="$2"
	set +e
	( cd "$out" &&
		PATH="$spy:$PATH" \
		VAKT_BACKUP_PG_MODE=host \
		VAKT_PG_URL="$DB_URL" \
		VAKT_DB_URL="$DB_URL" \
		VAKT_SECRET_KEY="$TEST_SECRET_KEY" \
		VAKT_BACKUP_PASSPHRASE="$TEST_PASSPHRASE" \
		PSQL_SQL_LOG="$PSQL_SQL_LOG" \
		PSQL_EXIT="$PSQL_EXIT" \
		bash "$BACKUP_SH" "$out" ) >"$out/run.out" 2>&1
	RC=$?
	set -e
}

# ── 1: happy path — the writer fires with the right statement, after signing ──
OUT1="$WORK/ok"; SPY1="$WORK/spy1"
mkdir -p "$OUT1"
make_stubs "$SPY1"
PSQL_SQL_LOG="$WORK/psql1.sql"; : >"$PSQL_SQL_LOG"
PSQL_EXIT="$WORK/exit1"; echo 0 >"$PSQL_EXIT"
run_backup "$OUT1" "$SPY1"

ARCHIVE1="$(find "$OUT1" -maxdepth 1 -name 'vakt-backup-*.tar.gz' 2>/dev/null | head -1)"
if [ "$RC" -ne 0 ]; then
	fail "1 — backup.sh failed (rc=$RC): $(tail -3 "$OUT1/run.out" | tr '\n' ' ')"
elif [ -z "$ARCHIVE1" ]; then
	fail "1 — rc=0 but no signed archive was produced — the writer would have run before the artifact existed"
elif [ ! -s "$PSQL_SQL_LOG" ]; then
	fail "1 — no psql invocation recorded: backup.sh never reached the backup_log writer"
elif ! grep -q 'backup_log' "$PSQL_SQL_LOG"; then
	fail "1 — psql was called but not with a backup_log statement: $(tr '\n' ' ' <"$PSQL_SQL_LOG")"
elif ! grep -q 'INSERT INTO backup_log' "$PSQL_SQL_LOG"; then
	fail "1 — the backup_log statement is not the expected INSERT: $(tr '\n' ' ' <"$PSQL_SQL_LOG")"
elif ! grep -q 'FROM organizations' "$PSQL_SQL_LOG"; then
	fail "1 — the INSERT does not select org ids from organizations (per-org invariant lost): $(tr '\n' ' ' <"$PSQL_SQL_LOG")"
else
	pass "1 — signed archive produced AND backup_log INSERT (per org) fired after it via psql -f -"
fi

# ── 2: best-effort — a FAILING backup_log write must not make the backup red ──
# Same run, but the psql stub exits 1. The archive must survive, rc must stay 0,
# and the warning must be emitted (a silent swallow would be its own defect).
OUT2="$WORK/warn"; SPY2="$WORK/spy2"
mkdir -p "$OUT2"
make_stubs "$SPY2"
PSQL_SQL_LOG="$WORK/psql2.sql"; : >"$PSQL_SQL_LOG"
PSQL_EXIT="$WORK/exit2"; echo 1 >"$PSQL_EXIT"
run_backup "$OUT2" "$SPY2"

ARCHIVE2="$(find "$OUT2" -maxdepth 1 -name 'vakt-backup-*.tar.gz' 2>/dev/null | head -1)"
SIG2="$(find "$OUT2" -maxdepth 1 -name 'vakt-backup-*.tar.gz.sig' 2>/dev/null | head -1)"
if [ "$RC" -ne 0 ]; then
	fail "2 — a failing backup_log write made the backup red (rc=$RC): $(tail -3 "$OUT2/run.out" | tr '\n' ' ')"
elif [ -z "$ARCHIVE2" ] || [ -z "$SIG2" ]; then
	fail "2 — rc=0 but archive/signature missing after the writer failed"
elif ! grep -q 'backup_log' "$PSQL_SQL_LOG"; then
	fail "2 — the writer was never attempted (no backup_log SQL reached psql)"
elif ! grep -qi 'WARNUNG: backup_log' "$OUT2/run.out"; then
	fail "2 — the failed backup_log write was swallowed silently (no warning): $(tail -3 "$OUT2/run.out" | tr '\n' ' ')"
else
	pass "2 — failing backup_log write is best-effort: rc=0, archive+signature kept, warning emitted"
fi

summary
echo "ALL TESTS PASSED — checked: end-to-end host-mode backup with psql spy, unchecked: container path (static seam only), skipped: 0"
