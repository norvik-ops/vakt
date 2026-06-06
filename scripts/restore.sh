#!/usr/bin/env bash
set -euo pipefail

# Vakt restore script.
# Usage: ./scripts/restore.sh <backup-file.tar.gz> [--dry-run]
#   --dry-run  Validates the archive and decrypts the key without touching the database.

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
  set -a; source .env; set +a
fi

DB_URL="${VAKT_DB_URL:-}"
if [ -z "$DB_URL" ] && [ "$DRY_RUN" = false ]; then
  echo "ERROR: VAKT_DB_URL not set" >&2
  exit 1
fi

WORK_DIR=$(mktemp -d)
trap 'rm -rf "$WORK_DIR"' EXIT

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

echo "→ Decrypting encryption key (enter passphrase)..."
RESTORED_KEY=$(openssl enc -d -aes-256-cbc -pbkdf2 -in "$WORK_DIR/secret.key.enc")

KEY_FILE=$(mktemp /tmp/vakt-restored-key-XXXXXX.txt)
chmod 600 "$KEY_FILE"
echo "$RESTORED_KEY" > "$KEY_FILE"
unset RESTORED_KEY

echo ""
echo "⚠  Restored VAKT_SECRET_KEY written to: $KEY_FILE"
echo "   Set VAKT_SECRET_KEY from this file, then: rm -f $KEY_FILE"
echo "   Set this in your .env before starting the application."
echo ""

if [ "$DRY_RUN" = true ]; then
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

echo "✓ Restore complete. Update VAKT_SECRET_KEY and restart the application."
