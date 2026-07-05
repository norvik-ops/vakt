#!/usr/bin/env bash
set -euo pipefail

# S114 test: GPG symmetric encrypt/decrypt roundtrip used by backup.sh + backup-verify.sh.
# Tests the exact gpg flags used in both scripts without a real database.

fail() { echo "FAIL: $1" >&2; exit 1; }
pass() { echo "PASS: $1"; }

WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT

PASSPHRASE="test-passphrase-s114"
PLAINTEXT="fake-db-dump-content-for-test"
DUMP="$WORK/db.pgdump"
ENCRYPTED="$WORK/db.pgdump.gpg"
DECRYPTED="$WORK/db.pgdump.dec"

printf '%s' "$PLAINTEXT" >"$DUMP"

# ── 1: encrypt (backup.sh path) ───────────────────────────────────────────
printf '%s' "$PASSPHRASE" | gpg --batch --yes --passphrase-fd 0 --pinentry-mode loopback \
	--symmetric --cipher-algo AES256 \
	--output "$ENCRYPTED" "$DUMP" 2>/dev/null
[ -f "$ENCRYPTED" ] || fail "encrypted file not created"
[ ! -f "$DUMP" ] && : || rm "$DUMP"  # backup.sh removes original
pass "GPG encryption produces .gpg file"

# ── 2: decrypt (backup-verify.sh path) ────────────────────────────────────
printf '%s' "$PASSPHRASE" | gpg --batch --yes --passphrase-fd 0 --pinentry-mode loopback \
	--decrypt --output "$DECRYPTED" "$ENCRYPTED" 2>/dev/null
RESULT=$(cat "$DECRYPTED")
[ "$RESULT" = "$PLAINTEXT" ] || fail "decrypted content mismatch: got '$RESULT'"
pass "GPG decryption recovers original content"

# ── 3: wrong passphrase fails ─────────────────────────────────────────────
set +e
printf '%s' "wrong-passphrase" | gpg --batch --yes --passphrase-fd 0 --pinentry-mode loopback \
	--decrypt --output "$WORK/bad.dec" "$ENCRYPTED" 2>/dev/null
RC=$?
set -e
[ "$RC" -ne 0 ] || fail "wrong passphrase should fail"
pass "wrong passphrase correctly rejected"

echo "ALL TESTS PASSED"
