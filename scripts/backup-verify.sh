#!/usr/bin/env bash
set -euo pipefail

BACKUP_FILE="${1:-}"
if [ -z "$BACKUP_FILE" ] || [ ! -f "$BACKUP_FILE" ]; then
	echo "ERROR: Usage: $0 <backup-file.tar.gz>" >&2
	exit 1
fi

SIG_FILE="${BACKUP_FILE}.sig"
if [ ! -f "$SIG_FILE" ]; then
	echo "ERROR: Signature file not found: $SIG_FILE" >&2
	exit 1
fi

if [ -f .env ]; then
	# shellcheck source=/dev/null
	set -a
	source .env
	set +a
fi

SECRET_KEY="${VAKT_SECRET_KEY:-}"
if [ -z "$SECRET_KEY" ]; then
	echo "ERROR: VAKT_SECRET_KEY not set" >&2
	exit 1
fi

# Passphrase needed to decrypt db.pgdump.gpg — env var or file, no interactive.
PASSPHRASE=""
if [ -n "${VAKT_BACKUP_PASSPHRASE:-}" ]; then
	PASSPHRASE="$VAKT_BACKUP_PASSPHRASE"
elif [ -n "${VAKT_BACKUP_PASSPHRASE_FILE:-}" ] && [ -f "$VAKT_BACKUP_PASSPHRASE_FILE" ]; then
	PASSPHRASE=$(cat "$VAKT_BACKUP_PASSPHRASE_FILE")
fi

echo "→ Verifying HMAC-SHA256 signature..."
EXPECTED=$(cat "$SIG_FILE")
# Use the same derived HMAC key as backup.sh.
HMAC_KEY=$(printf 'vakt-backup-hmac:%s' "$SECRET_KEY" | sha256sum | cut -d' ' -f1)
ACTUAL=$(openssl dgst -sha256 -hmac "$HMAC_KEY" "$BACKUP_FILE" | awk '{print $NF}')
unset HMAC_KEY
if [ "$EXPECTED" != "$ACTUAL" ]; then
	echo "ERROR: HMAC signature mismatch — archive may be corrupted or tampered with" >&2
	exit 1
fi
echo "✓ HMAC signature valid"

WORK_DIR=$(mktemp -d)
trap 'rm -rf "$WORK_DIR"' EXIT

echo "→ Extracting..."
tar -xzf "$BACKUP_FILE" -C "$WORK_DIR"

echo "→ Verifying manifest..."
cat "$WORK_DIR/manifest.json"

echo "→ Checking dump integrity..."
if [ -f "$WORK_DIR/db.pgdump.gpg" ]; then
	if [ -z "$PASSPHRASE" ]; then
		echo "WARNING: db.pgdump.gpg found but no passphrase — skipping dump integrity check" >&2
		echo "  Set VAKT_BACKUP_PASSPHRASE or VAKT_BACKUP_PASSPHRASE_FILE to verify the dump."
	else
		DUMP_TMP=$(mktemp)
		trap 'rm -f "$DUMP_TMP"; rm -rf "$WORK_DIR"' EXIT
		printf '%s' "$PASSPHRASE" | gpg --batch --yes --passphrase-fd 0 --pinentry-mode loopback \
			--decrypt --output "$DUMP_TMP" "$WORK_DIR/db.pgdump.gpg"
		pg_restore --list "$DUMP_TMP" >/dev/null && echo "✓ Dump decrypted and valid"
		rm -f "$DUMP_TMP"
		unset PASSPHRASE
	fi
elif [ -f "$WORK_DIR/db.pgdump" ]; then
	pg_restore --list "$WORK_DIR/db.pgdump" >/dev/null && echo "✓ Dump is valid"
else
	echo "ERROR: No db.pgdump or db.pgdump.gpg found in archive" >&2
	exit 1
fi

echo "✓ Backup verification passed: $BACKUP_FILE"
