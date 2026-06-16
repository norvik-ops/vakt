#!/usr/bin/env bash
set -euo pipefail

# S89-1 test: verifies restore.sh hardening without a database.
#   1. A valid signed archive passes the dry-run.
#   2. The decrypted master key never appears in stdout.
#   3. No /tmp/vakt-restored-key-* file is left behind.
#   4. A tampered archive is rejected by the HMAC check.
#
# Self-contained: builds a minimal archive (dummy db.pgdump + real
# openssl-encrypted key), runs restore.sh --dry-run, asserts behaviour.

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
RESTORE_SH="${SCRIPT_DIR}/restore.sh"

fail() { echo "FAIL: $1" >&2; exit 1; }
pass() { echo "PASS: $1"; }

TEST_SECRET_KEY="0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
TEST_PASSPHRASE="correct horse battery staple"

WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT

# Snapshot /tmp key files that exist before the run (should be none of ours).
BEFORE_KEYS="$(find /tmp -maxdepth 1 -name 'vakt-restored-key-*' 2>/dev/null | wc -l)"

# ── Build a minimal valid archive ─────────────────────────────────────────
STAGE="$(mktemp -d)"
echo "dummy-pgdump-content" >"$STAGE/db.pgdump"
printf '%s\n' "$TEST_SECRET_KEY" |
	openssl enc -aes-256-cbc -pbkdf2 -pass "pass:${TEST_PASSPHRASE}" -out "$STAGE/secret.key.enc"
echo '{"backup_date":"test","tool":"vakt-backup"}' >"$STAGE/manifest.json"

ARCHIVE="$WORK/vakt-backup-test.tar.gz"
tar -czf "$ARCHIVE" -C "$STAGE" .
rm -rf "$STAGE"

# Sign with the same HMAC scheme as backup.sh.
HMAC_KEY=$(printf 'vakt-backup-hmac:%s' "$TEST_SECRET_KEY" | sha256sum | cut -d' ' -f1)
openssl dgst -sha256 -hmac "$HMAC_KEY" "$ARCHIVE" | awk '{print $NF}' >"${ARCHIVE}.sig"

# ── 1+2+3: valid archive dry-run ──────────────────────────────────────────
# Run in a clean CWD so no stray .env is sourced.
OUT="$(cd "$WORK" && \
	VAKT_SECRET_KEY="$TEST_SECRET_KEY" \
	VAKT_BACKUP_PASSPHRASE="$TEST_PASSPHRASE" \
	TEST_PASSPHRASE_ENV="$TEST_PASSPHRASE" \
	bash "$RESTORE_SH" "$ARCHIVE" --dry-run 2>&1)" || fail "dry-run exited non-zero:\n$OUT"

echo "$OUT" | grep -q "Signature valid" || fail "expected signature-valid message"
echo "$OUT" | grep -q "Dry-run complete" || fail "expected dry-run completion"
pass "valid archive passes dry-run"

if echo "$OUT" | grep -qF "$TEST_SECRET_KEY"; then
	fail "master key LEAKED to stdout"
fi
pass "master key never printed to stdout"

AFTER_KEYS="$(find /tmp -maxdepth 1 -name 'vakt-restored-key-*' 2>/dev/null | wc -l)"
if [ "$AFTER_KEYS" -gt "$BEFORE_KEYS" ]; then
	fail "a /tmp/vakt-restored-key-* file was left behind"
fi
pass "no recovered-key file left in /tmp"

# ── 4: tampered archive must be rejected ──────────────────────────────────
printf 'tamper' >>"$ARCHIVE" # corrupt the archive, keep the old signature
set +e
TAMPER_OUT="$(cd "$WORK" && \
	VAKT_SECRET_KEY="$TEST_SECRET_KEY" \
	VAKT_BACKUP_PASSPHRASE="$TEST_PASSPHRASE" \
	bash "$RESTORE_SH" "$ARCHIVE" --dry-run 2>&1)"
TAMPER_RC=$?
set -e
if [ "$TAMPER_RC" -eq 0 ]; then
	fail "tampered archive was accepted (expected non-zero exit)"
fi
echo "$TAMPER_OUT" | grep -qi "signature mismatch" || fail "expected signature mismatch error"
pass "tampered archive rejected by HMAC check"

echo "ALL TESTS PASSED"
