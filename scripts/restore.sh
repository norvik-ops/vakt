#!/usr/bin/env bash
set -euo pipefail

# S89-1: restrict permissions on every file this script creates (temp dirs,
# the recovered-key file). 077 = owner-only; protects the decrypted master key
# on a multi-user host.
umask 077

# Vakt restore script.
# Usage: ./scripts/restore.sh <backup-file.tar.gz> [--dry-run]
#   --dry-run  Validates the archive and decrypts the key without touching the database.
#
# Passphrase for the encrypted key may be supplied non-interactively via
# VAKT_BACKUP_PASSPHRASE or VAKT_BACKUP_PASSPHRASE_FILE (for automation / tests);
# otherwise it is prompted on a TTY.

BACKUP_FILE="${1:-}"
DRY_RUN=false
for arg in "$@"; do
	[ "$arg" = "--dry-run" ] && DRY_RUN=true
done

if [ -z "$BACKUP_FILE" ] || [ ! -f "$BACKUP_FILE" ]; then
	echo "ERROR: Usage: $0 <backup-file.tar.gz> [--dry-run]" >&2
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

DB_URL="${VAKT_DB_URL:-}"
if [ -z "$DB_URL" ] && [ "$DRY_RUN" = false ]; then
	echo "ERROR: VAKT_DB_URL not set" >&2
	exit 1
fi

# S89-1: single cleanup trap covering EVERY exit path (success, error, abort).
# Securely shreds the recovered-key file if one was written, and removes the
# work dir. KEY_FILE/WORK_DIR start empty so the trap is safe before they exist.
WORK_DIR=""
KEY_FILE=""
cleanup() {
	if [ -n "$KEY_FILE" ] && [ -f "$KEY_FILE" ]; then
		shred -u "$KEY_FILE" 2>/dev/null || rm -f "$KEY_FILE"
	fi
	[ -n "$WORK_DIR" ] && rm -rf "$WORK_DIR"
}
trap cleanup EXIT

# Verify signature BEFORE extracting or touching the database.
SIG_FILE="${BACKUP_FILE}.sig"
if [ ! -f "$SIG_FILE" ]; then
	echo "ERROR: Signature file not found: ${SIG_FILE} — refusing to restore unverified backup" >&2
	exit 1
fi
echo "→ Verifying backup signature..."
HMAC_KEY=$(printf 'vakt-backup-hmac:%s' "$SECRET_KEY" | sha256sum | cut -d' ' -f1)
EXPECTED_SIG=$(cat "$SIG_FILE")
ACTUAL_SIG=$(openssl dgst -sha256 -hmac "$HMAC_KEY" "$BACKUP_FILE" | awk '{print $NF}')
unset HMAC_KEY
if [ "$EXPECTED_SIG" != "$ACTUAL_SIG" ]; then
	echo "ERROR: HMAC signature mismatch — refusing to restore (archive may be corrupted or tampered with)" >&2
	exit 1
fi
echo "✓ Signature valid"

WORK_DIR=$(mktemp -d)

echo "→ Extracting backup..."
tar -xzf "$BACKUP_FILE" -C "$WORK_DIR"

if [ ! -f "$WORK_DIR/db.pgdump" ] || [ ! -f "$WORK_DIR/secret.key.enc" ]; then
	echo "ERROR: Backup archive is missing required files (db.pgdump, secret.key.enc)" >&2
	exit 1
fi

if [ -f "$WORK_DIR/manifest.json" ]; then
	echo "→ Manifest:"
	cat "$WORK_DIR/manifest.json"
	echo
fi

# Resolve the decryption passphrase (non-interactive for automation/tests).
PASS_ARGS=()
if [ -n "${VAKT_BACKUP_PASSPHRASE:-}" ]; then
	PASS_ARGS=(-pass env:VAKT_BACKUP_PASSPHRASE)
elif [ -n "${VAKT_BACKUP_PASSPHRASE_FILE:-}" ] && [ -f "$VAKT_BACKUP_PASSPHRASE_FILE" ]; then
	PASS_ARGS=(-pass "file:${VAKT_BACKUP_PASSPHRASE_FILE}")
else
	echo "→ Decrypting encryption key (enter passphrase)..."
fi

# Decrypt into a variable to verify the passphrase. The plaintext key is NEVER
# echoed to stdout/logs (S89-1), here or in the dry-run path.
RESTORED_KEY=$(openssl enc -d -aes-256-cbc -pbkdf2 -in "$WORK_DIR/secret.key.enc" "${PASS_ARGS[@]}")
if [ -z "$RESTORED_KEY" ]; then
	echo "ERROR: key decryption produced an empty result (wrong passphrase?)" >&2
	exit 1
fi
echo "✓ Encryption key decrypted successfully"

if [ "$DRY_RUN" = true ]; then
	# Dry-run never persists or prints the key.
	if [ "$RESTORED_KEY" = "$SECRET_KEY" ]; then
		echo "✓ Recovered key matches the configured VAKT_SECRET_KEY"
	else
		echo "⚠  Recovered key differs from the configured VAKT_SECRET_KEY (key rotation or different backup)"
	fi
	unset RESTORED_KEY
	echo "✓ Dry-run complete. Archive is valid, key decrypted successfully."
	echo "  Database was NOT modified. Run without --dry-run to restore."
	exit 0
fi

echo "→ Restoring PostgreSQL (this will DROP existing data)..."
read -r -p "   Continue? [y/N] " confirm
if [[ "$confirm" != "y" && "$confirm" != "Y" ]]; then
	echo "Aborted."
	exit 0
fi

pg_restore --clean --if-exists -d "$DB_URL" "$WORK_DIR/db.pgdump"

if [ -f "$WORK_DIR/uploads.tar.gz" ]; then
	echo "→ Restoring uploads volume (evidence attachments)..."
	if ! docker volume inspect uploads_data >/dev/null 2>&1; then
		docker volume create uploads_data
	fi
	docker run --rm \
		-v uploads_data:/data \
		-v "$WORK_DIR":/backup:ro \
		alpine:latest sh -c "cd /data && tar xzf /backup/uploads.tar.gz"
	echo "✓ Uploads volume restored"
else
	echo "   (No uploads.tar.gz in archive — uploads volume not restored)"
fi

# Hand the recovered key to the operator securely: a 0600 temp file that is
# SHREDDED when this script exits (cleanup trap). The operator copies it into
# .env during the pause below; it never lingers in /tmp and is never echoed.
if [ "$RESTORED_KEY" != "$SECRET_KEY" ] && [ -t 0 ]; then
	KEY_FILE=$(mktemp /tmp/vakt-restored-key-XXXXXX.txt)
	chmod 600 "$KEY_FILE"
	printf '%s\n' "$RESTORED_KEY" >"$KEY_FILE"
	echo ""
	echo "⚠  The backup's VAKT_SECRET_KEY differs from your current one."
	echo "   Recovered key written to (0600, auto-deleted on exit): $KEY_FILE"
	echo "   Copy it into your .env NOW (e.g. 'cat $KEY_FILE' in another terminal),"
	echo "   then press Enter — the file will be securely deleted."
	read -r -p "   Press Enter when done... " _
fi
unset RESTORED_KEY

echo "✓ Restore complete. Ensure VAKT_SECRET_KEY in .env matches the backup, then restart the application."
