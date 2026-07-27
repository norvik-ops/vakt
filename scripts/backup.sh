#!/usr/bin/env bash
set -euo pipefail

# Vakt backup script — exports PostgreSQL dump + encryption key.
# Usage: ./scripts/backup.sh [output-dir]
# Requires: pg_dump, gpg, openssl, docker (if using compose)

OUTPUT_DIR="${1:-.}"
DATE=$(date +%Y-%m-%d_%H-%M-%S)
BACKUP_NAME="vakt-backup-${DATE}"
WORK_DIR=$(mktemp -d)
trap 'rm -rf "$WORK_DIR"' EXIT

# Load env if .env exists
if [ -f .env ]; then
	# shellcheck source=/dev/null
	set -a
	source .env
	set +a
fi

# Resolves the uploads_data named volume as Compose itself would create it.
# The root docker-compose.yml has no pinned "name:" (unlike some deployment
# compose files), so Compose prefixes every volume with the project
# name — which defaults to the invoking directory's basename, not the literal
# volume key from the YAML. A bare `docker volume inspect uploads_data` only
# matches by accident; on a real deploy it silently misses, and backup.sh used
# to swallow that as "no evidence attachments yet" (SA06-D2).
resolve_uploads_volume() {
	local project="${COMPOSE_PROJECT_NAME:-}"
	if [ -z "$project" ]; then
		# Same normalization Compose applies to its own default project name.
		project=$(basename "$(pwd)" | tr '[:upper:]' '[:lower:]' | tr -cd 'a-z0-9_-')
	fi
	local prefixed="${project}_uploads_data"
	if docker volume inspect "$prefixed" >/dev/null 2>&1; then
		printf '%s' "$prefixed"
		return 0
	fi
	# Legacy/external-declared fallback: only accept the bare name if it
	# genuinely exists — never invent a volume name that isn't attached.
	if docker volume inspect uploads_data >/dev/null 2>&1; then
		printf '%s' "uploads_data"
		return 0
	fi
	return 1
}

DB_URL="${VAKT_DB_URL:-}"
SECRET_KEY="${VAKT_SECRET_KEY:-}"

if [ -z "$DB_URL" ]; then
	echo "ERROR: VAKT_DB_URL not set" >&2
	exit 1
fi

if [ -z "$SECRET_KEY" ]; then
	echo "ERROR: VAKT_SECRET_KEY not set" >&2
	exit 1
fi

if ! command -v gpg >/dev/null 2>&1; then
	echo "ERROR: gpg not found — install gnupg to run backup" >&2
	exit 1
fi

# Load passphrase early — needed for both dump and key encryption.
if [ -n "${VAKT_BACKUP_PASSPHRASE:-}" ]; then
	PASSPHRASE="$VAKT_BACKUP_PASSPHRASE"
elif [ -n "${VAKT_BACKUP_PASSPHRASE_FILE:-}" ] && [ -f "$VAKT_BACKUP_PASSPHRASE_FILE" ]; then
	PASSPHRASE=$(cat "$VAKT_BACKUP_PASSPHRASE_FILE")
elif [ -t 0 ]; then
	while true; do
		read -r -s -p "   Enter passphrase (min. 12 characters): " PASSPHRASE
		echo
		if [ "${#PASSPHRASE}" -lt 12 ]; then
			echo "   ERROR: Passphrase must be at least 12 characters." >&2
			continue
		fi
		read -r -s -p "   Confirm passphrase: " PASSPHRASE2
		echo
		if [ "$PASSPHRASE" != "$PASSPHRASE2" ]; then
			echo "   ERROR: Passphrases do not match." >&2
			continue
		fi
		break
	done
	unset PASSPHRASE2
else
	echo "ERROR: No passphrase provided. Set VAKT_BACKUP_PASSPHRASE, VAKT_BACKUP_PASSPHRASE_FILE, or run interactively." >&2
	exit 1
fi

echo "→ Dumping PostgreSQL..."
pg_dump "$DB_URL" --format=custom --compress=9 -f "$WORK_DIR/db.pgdump"

echo "→ Encrypting dump with GPG..."
printf '%s' "$PASSPHRASE" | gpg --batch --yes --passphrase-fd 0 --pinentry-mode loopback \
	--symmetric --cipher-algo AES256 \
	--output "$WORK_DIR/db.pgdump.gpg" "$WORK_DIR/db.pgdump"
rm "$WORK_DIR/db.pgdump"

echo "→ Backing up uploads volume (evidence attachments)..."
if UPLOADS_VOLUME=$(resolve_uploads_volume); then
	docker run --rm \
		-v "${UPLOADS_VOLUME}:/data:ro" \
		-v "$WORK_DIR":/backup \
		alpine:latest tar czf /backup/uploads.tar.gz -C /data .
	echo "   ${UPLOADS_VOLUME} volume archived"
else
	echo "   uploads volume not found (tried \"${COMPOSE_PROJECT_NAME:-$(basename "$(pwd)" | tr '[:upper:]' '[:lower:]' | tr -cd 'a-z0-9_-')}_uploads_data\" and \"uploads_data\") — skipping (no evidence attachments yet?)"
fi

echo "→ Encrypting encryption key..."
echo "$SECRET_KEY" | PASSPHRASE="$PASSPHRASE" openssl enc -aes-256-cbc -pbkdf2 -pass env:PASSPHRASE -out "$WORK_DIR/secret.key.enc"
unset PASSPHRASE

# Write manifest
cat >"$WORK_DIR/manifest.json" <<EOF
{
  "backup_date": "${DATE}",
  "schema_version": "$(date +%Y%m%d)",
  "tool": "vakt-backup"
}
EOF

echo "→ Creating archive..."
tar -czf "${OUTPUT_DIR}/${BACKUP_NAME}.tar.gz" -C "$WORK_DIR" .

# A dump that failed partway (disk full, killed pg_dump, aborted docker run)
# can still leave a small-but-valid gzip behind — "OK" would then mean
# "wrote a corpse". A minimum-size floor catches that class before the
# archive is signed and trusted (learned the hard way: a 20-byte gzip that
# replicated off-site as "success" and displaced the good copies).
ARCHIVE_SIZE=$(stat -c%s "${OUTPUT_DIR}/${BACKUP_NAME}.tar.gz" 2>/dev/null || stat -f%z "${OUTPUT_DIR}/${BACKUP_NAME}.tar.gz")
if [ "$ARCHIVE_SIZE" -lt 1000 ]; then
	echo "ERROR: backup archive suspiciously small (${ARCHIVE_SIZE} bytes) — refusing to keep it" >&2
	rm -f "${OUTPUT_DIR}/${BACKUP_NAME}.tar.gz"
	exit 1
fi

echo "→ Signing archive..."
HMAC_KEY=$(printf 'vakt-backup-hmac:%s' "$SECRET_KEY" | sha256sum | cut -d' ' -f1)
openssl dgst -sha256 -hmac "$HMAC_KEY" "${OUTPUT_DIR}/${BACKUP_NAME}.tar.gz" |
	awk '{print $NF}' >"${OUTPUT_DIR}/${BACKUP_NAME}.tar.gz.sig"
unset HMAC_KEY

echo "✓ Backup saved:    ${OUTPUT_DIR}/${BACKUP_NAME}.tar.gz"
echo "✓ Signature saved: ${OUTPUT_DIR}/${BACKUP_NAME}.tar.gz.sig"
