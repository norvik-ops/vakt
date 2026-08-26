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

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# ESK-7: entscheidet, ob der Dump im DB-Container oder auf dem Host laeuft. Der
# Standard-Compose-Stack legt postgres auf ein `internal: true`-Netz ohne
# publizierten Port — vom Host ist die DB dort nicht erreichbar. Begruendung und
# Konfiguration in scripts/backup-pg-target.sh.
# shellcheck source=scripts/backup-pg-target.sh
source "${SCRIPT_DIR}/backup-pg-target.sh"

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

# ESK-7: "ich habe nichts gefunden" und "ich konnte nicht nachsehen" sind zwei
# verschiedene Aussagen, und nur eine davon ist beruhigend. Ohne Docker-CLI (z. B.
# im Opt-in-Scheduler-Container aus docker-compose.backup.yml, der bewusst KEINEN
# Docker-Socket bekommt) kann dieses Skript das Uploads-Volume grundsaetzlich
# nicht sehen — es meldete das bisher als "no evidence attachments yet?" und
# schrieb ein Archiv ohne Evidence-Dateien, das aussah wie ein vollstaendiges.
uploads_lookup_possible() {
	command -v docker >/dev/null 2>&1 && docker volume ls >/dev/null 2>&1
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

export VAKT_PG_URL="$DB_URL"
# Fehlkonfiguration hier abbrechen, nicht in einer Subshell weiter unten (dort
# wuerde nur die Subshell sterben und der Lauf einen Weg nehmen, den niemand
# gewaehlt hat).
vakt_pg_require_valid_mode
echo "→ Dumping PostgreSQL (via $(vakt_pg_describe))..."
vakt_pg_dump_to "$WORK_DIR/db.pgdump"

# ESK-7: Mindestgroesse auf den DUMP, nicht nur auf das fertige Archiv.
# Gemessen: bereits 1 KB Evidence im Uploads-Volume hebt ein Archiv mit einem
# 0-Byte-Dump auf ~1663 Bytes und damit ueber BEIDE bestehenden Schwellen
# (hier unten 1000 Bytes, backup-cron.sh 1024 Bytes). Die Archiv-Schwelle
# schuetzt also ausgerechnet die Instanz nicht, deren Backup etwas wert ist.
# Ein leerer Dump ist kein kleines Backup, sondern gar keines — und ein leeres
# Backup ist schlimmer als ein fehlgeschlagenes, weil die Retention die guten
# Kopien dahinter wegraeumt.
DUMP_SIZE=$(stat -c%s "$WORK_DIR/db.pgdump" 2>/dev/null || stat -f%z "$WORK_DIR/db.pgdump")
if [ "$DUMP_SIZE" -lt 512 ]; then
	echo "ERROR: PostgreSQL-Dump ist nur ${DUMP_SIZE} Bytes gross — das ist kein gueltiger Dump." >&2
	echo "       Kein Archiv geschrieben. Pruefe die DB-Erreichbarkeit (VAKT_BACKUP_PG_MODE, VAKT_DB_URL)." >&2
	exit 1
fi
# Der Dump muss auch LESBAR sein, nicht nur gross: ein abgebrochener pg_dump
# hinterlaesst eine halbe Datei, die jede Groessenschwelle passiert.
set +e
vakt_pg_dump_readable "$WORK_DIR/db.pgdump"
DUMP_READABLE_RC=$?
set -e
case "$DUMP_READABLE_RC" in
0) echo "   Dump ok (${DUMP_SIZE} Bytes, als custom-format lesbar)" ;;
2) echo "   WARNUNG: Dump-Integritaet NICHT geprueft (kein pg_restore auf dem Host und kein DB-Container erreichbar) — ${DUMP_SIZE} Bytes" >&2 ;;
*)
	echo "ERROR: PostgreSQL-Dump ist beschaedigt (pg_restore --list schlaegt fehl) — kein Archiv geschrieben." >&2
	exit 1
	;;
esac

echo "→ Encrypting dump with GPG..."
printf '%s' "$PASSPHRASE" | gpg --batch --yes --passphrase-fd 0 --pinentry-mode loopback \
	--symmetric --cipher-algo AES256 \
	--output "$WORK_DIR/db.pgdump.gpg" "$WORK_DIR/db.pgdump"
rm "$WORK_DIR/db.pgdump"

echo "→ Backing up uploads volume (evidence attachments)..."
UPLOADS_STATE="absent"
if ! uploads_lookup_possible; then
	# NICHT als "keine Anhaenge" verbuchen — wir haben nicht nachgesehen.
	UPLOADS_STATE="unchecked"
	echo "   WARNUNG: Uploads-Volume nicht pruefbar (kein Docker-Zugriff aus diesem Kontext)." >&2
	echo "            Dieses Archiv enthaelt NUR die Datenbank, keine Evidence-Dateien." >&2
	echo "            Sichere das Volume separat oder fahre backup.sh vom Host mit Docker-Zugriff." >&2
elif UPLOADS_VOLUME=$(resolve_uploads_volume); then
	docker run --rm \
		-v "${UPLOADS_VOLUME}:/data:ro" \
		-v "$WORK_DIR":/backup \
		alpine:latest tar czf /backup/uploads.tar.gz -C /data .
	UPLOADS_STATE="included"
	echo "   ${UPLOADS_VOLUME} volume archived"
else
	echo "   uploads volume not found (tried \"${COMPOSE_PROJECT_NAME:-$(basename "$(pwd)" | tr '[:upper:]' '[:lower:]' | tr -cd 'a-z0-9_-')}_uploads_data\" and \"uploads_data\") — skipping (no evidence attachments yet?)"
fi

echo "→ Encrypting encryption key..."
echo "$SECRET_KEY" | PASSPHRASE="$PASSPHRASE" openssl enc -aes-256-cbc -pbkdf2 -pass env:PASSPHRASE -out "$WORK_DIR/secret.key.enc"
unset PASSPHRASE

# Write manifest.
# ESK-7: "uploads" haelt fest, ob Evidence-Dateien im Archiv liegen — und wenn
# nicht, ob sie fehlen (absent) oder ob nur niemand nachgesehen hat (unchecked).
# Ohne dieses Feld kann die Restore-Seite ein DB-only-Archiv nicht von einem
# vollstaendigen unterscheiden und meldet in beiden Faellen dasselbe.
cat >"$WORK_DIR/manifest.json" <<EOF
{
  "backup_date": "${DATE}",
  "schema_version": "$(date +%Y%m%d)",
  "uploads": "${UPLOADS_STATE}",
  "tool": "vakt-backup"
}
EOF

echo "→ Creating archive..."
tar -czf "${OUTPUT_DIR}/${BACKUP_NAME}.tar.gz" -C "$WORK_DIR" .

# An archive that is too small was written by a run that produced nothing useful
# (disk full, killed pg_dump, aborted docker run) — keeping it would mean "OK"
# stands for "wrote a corpse", and retention would then evict the good copies.
#
# Scope of THIS floor, stated honestly: it only judges the SIZE of a completed
# archive. It does NOT cover a failure DURING signing — that path is handled
# separately below. (The dump itself has its own size + readability assertion
# further up; the floor here cannot see an empty dump once uploads.tar.gz adds
# bulk — measured: 1 KB of evidence carries a 0-byte dump past this threshold.)
ARCHIVE_SIZE=$(stat -c%s "${OUTPUT_DIR}/${BACKUP_NAME}.tar.gz" 2>/dev/null || stat -f%z "${OUTPUT_DIR}/${BACKUP_NAME}.tar.gz")
if [ "$ARCHIVE_SIZE" -lt 1000 ]; then
	echo "ERROR: backup archive suspiciously small (${ARCHIVE_SIZE} bytes) — refusing to keep it" >&2
	rm -f "${OUTPUT_DIR}/${BACKUP_NAME}.tar.gz"
	exit 1
fi

# ESK-7: ein Fehlschlag BEIM Signieren hinterliess bisher genau den Zustand, den
# dieses Skript sonst ueberall vermeidet — ein Archiv, das gross genug ist, um
# vertrauenswuerdig zu wirken, daneben eine 0-Byte-`.sig` (gemessen: 2880 Bytes
# Archiv + leere Signatur, rc=1). `restore.sh` lehnt so ein Archiv korrekt ab
# (HMAC-Vergleich schlaegt fehl), und `backup-cron.sh` bricht ab — aber die Datei
# LIEGT da, sieht in einem Verzeichnislisting aus wie ein gueltiges Backup und
# zaehlt in jeder "gibt es ein aktuelles Backup?"-Pruefung mit, die nur Namen und
# Groesse ansieht. Ein halbes Backup ist kein Backup: beide Dateien wieder weg.
echo "→ Signing archive..."
HMAC_KEY=$(printf 'vakt-backup-hmac:%s' "$SECRET_KEY" | sha256sum | cut -d' ' -f1)
set +e
openssl dgst -sha256 -hmac "$HMAC_KEY" "${OUTPUT_DIR}/${BACKUP_NAME}.tar.gz" |
	awk '{print $NF}' >"${OUTPUT_DIR}/${BACKUP_NAME}.tar.gz.sig"
SIGN_RC=$?
set -e
unset HMAC_KEY

# Auch ein rc=0 mit leerer Ausgabe zaehlt als Fehlschlag — eine leere Signatur ist
# keine Signatur, und `openssl` in einer Pipe kann beides liefern.
SIG_SIZE=$(stat -c%s "${OUTPUT_DIR}/${BACKUP_NAME}.tar.gz.sig" 2>/dev/null || echo 0)
if [ "$SIGN_RC" -ne 0 ] || [ "$SIG_SIZE" -lt 32 ]; then
	echo "ERROR: Signieren des Archivs fehlgeschlagen (rc=${SIGN_RC}, Signatur ${SIG_SIZE} Bytes)." >&2
	echo "       Archiv UND Signatur werden entfernt — ein unsigniertes Archiv soll nicht" >&2
	echo "       wie ein gueltiges Backup im Verzeichnis liegen bleiben." >&2
	rm -f "${OUTPUT_DIR}/${BACKUP_NAME}.tar.gz" "${OUTPUT_DIR}/${BACKUP_NAME}.tar.gz.sig"
	exit 1
fi

echo "✓ Backup saved:    ${OUTPUT_DIR}/${BACKUP_NAME}.tar.gz"
echo "✓ Signature saved: ${OUTPUT_DIR}/${BACKUP_NAME}.tar.gz.sig"
