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

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# ESK-7: derselbe Grund wie in backup.sh — postgres liegt auf einem
# `internal: true`-Netz, das Host-`pg_restore` nicht erreichen kann. Der Fix nur
# am Dump haette die halbe Kette geheilt (Variant-Miss-Klasse).
# shellcheck source=scripts/backup-pg-target.sh
source "${SCRIPT_DIR}/backup-pg-target.sh"

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

# Same resolution as backup.sh: the uploads volume the running stack actually
# mounts is prefixed with the Compose project name (default: the invoking
# directory's basename), not the bare "uploads_data" key from the YAML.
# Restoring into the wrong (unprefixed) volume would leave evidence files
# invisible to the running vakt-api/vakt-worker containers (SA06-D2).
resolve_uploads_volume() {
	local project="${COMPOSE_PROJECT_NAME:-}"
	if [ -z "$project" ]; then
		project=$(basename "$(pwd)" | tr '[:upper:]' '[:lower:]' | tr -cd 'a-z0-9_-')
	fi
	printf '%s' "${project}_uploads_data"
}

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

if [ ! -f "$WORK_DIR/db.pgdump.gpg" ] || [ ! -f "$WORK_DIR/secret.key.enc" ]; then
	echo "ERROR: Backup archive is missing required files (db.pgdump.gpg, secret.key.enc)" >&2
	exit 1
fi

if [ -f "$WORK_DIR/manifest.json" ]; then
	echo "→ Manifest:"
	cat "$WORK_DIR/manifest.json"
	echo
fi

# Resolve the backup passphrase once — used for both GPG (db dump) and openssl (secret key).
PASSPHRASE=""
if [ -n "${VAKT_BACKUP_PASSPHRASE:-}" ]; then
	PASSPHRASE="$VAKT_BACKUP_PASSPHRASE"
elif [ -n "${VAKT_BACKUP_PASSPHRASE_FILE:-}" ] && [ -f "$VAKT_BACKUP_PASSPHRASE_FILE" ]; then
	PASSPHRASE=$(cat "$VAKT_BACKUP_PASSPHRASE_FILE")
else
	read -r -s -p "→ Enter backup passphrase: " PASSPHRASE
	echo
fi

# Decrypt db.pgdump.gpg → db.pgdump
echo "→ Decrypting database dump..."
printf '%s' "$PASSPHRASE" | gpg --batch --yes --passphrase-fd 0 --pinentry-mode loopback \
	--decrypt --output "$WORK_DIR/db.pgdump" "$WORK_DIR/db.pgdump.gpg"
echo "✓ Database dump decrypted"

# Decrypt secret.key.enc into a variable. The plaintext key is NEVER echoed (S89-1).
RESTORED_KEY=$(printf '%s' "$PASSPHRASE" | openssl enc -d -aes-256-cbc -pbkdf2 -pass stdin \
	-in "$WORK_DIR/secret.key.enc")
unset PASSPHRASE
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

export VAKT_PG_URL="$DB_URL"
# Siehe backup.sh: eine ungueltige Mode-Angabe muss den Lauf beenden, bevor eine
# Kommandosubstitution sie in ein "dann eben Host" verwandelt.
vakt_pg_require_valid_mode

echo "→ Restoring (this will DROP existing database objects) via $(vakt_pg_describe)..."
read -r -p "   Continue? [y/N] " confirm
if [[ "$confirm" != "y" && "$confirm" != "Y" ]]; then
	echo "Aborted."
	exit 0
fi

# ─── ESK-7 Fix 4: Uploads VOR dem Datenbank-Schritt ───────────────────────────
# Die Reihenfolge war umgekehrt, und der Datenbank-Schritt bricht in genau dem
# Fall ab, der der dokumentierte Anwendungsfall ist (Restore ueber eine
# bestehende Instanz). Ergebnis war end-to-end reproduziert: Datenbank
# wiederhergestellt, Evidence-Dateien weg, die API listete den Nachweis weiter
# und der Download gab 404 — ein Compliance-Nachweis, der auf nichts zeigt.
# Deshalb laufen die Dateien jetzt zuerst:
#   * Sie sind additiv (`tar xzf` ueberschreibt und legt an, es loescht nichts),
#     also ist der Schritt fuer sich harmlos und wiederholbar.
#   * Sie sind das einzige, was nicht rekonstruierbar ist. Ein abgebrochener
#     DB-Restore laesst sich erneut fahren; eine geloeschte Evidence-Datei nicht.
if [ -f "$WORK_DIR/uploads.tar.gz" ]; then
	# Best-effort, bewusst: dieser Schritt liegt jetzt VOR dem DB-Restore, darf
	# ihn also nicht verhindern. Auf einem Host ohne Docker-CLI (der Fall, für den
	# VAKT_BACKUP_PG_MODE=host existiert — externes/managed Postgres) lief hier
	# vorher `docker volume inspect` in ein `command not found` (rc=127) und `set -e`
	# beendete das Skript, bevor die Datenbank zurückkam. Vor der Umsortierung war
	# die DB da und nur die Uploads fehlten; danach fehlte beides. Ein Fix darf
	# keinen schlechteren Zustand erzeugen als der Defekt.
	UPLOADS_RESTORED=false
	if ! command -v docker >/dev/null 2>&1; then
		echo "⚠  Uploads-Archiv liegt vor, aber es gibt keine Docker-CLI auf diesem Host." >&2
		echo "   Die Evidence-Dateien werden NICHT wiederhergestellt; der Datenbank-Restore" >&2
		echo "   läuft weiter. uploads.tar.gz liegt im Archiv und kann manuell in das" >&2
		echo "   Uploads-Volume/-Verzeichnis entpackt werden." >&2
	else
		UPLOADS_VOLUME=$(resolve_uploads_volume)
		echo "→ Restoring uploads volume (evidence attachments) into ${UPLOADS_VOLUME}..."
		set +e
		docker volume inspect "$UPLOADS_VOLUME" >/dev/null 2>&1 ||
			docker volume create "$UPLOADS_VOLUME" >/dev/null
		docker run --rm \
			-v "${UPLOADS_VOLUME}:/data" \
			-v "$WORK_DIR":/backup:ro \
			alpine:latest sh -c "cd /data && tar xzf /backup/uploads.tar.gz"
		UPLOADS_RC=$?
		set -e
		if [ "$UPLOADS_RC" -eq 0 ]; then
			UPLOADS_RESTORED=true
			echo "✓ Uploads volume restored"
		else
			echo "⚠  Uploads-Wiederherstellung fehlgeschlagen (rc=${UPLOADS_RC}). Der" >&2
			echo "   Datenbank-Restore läuft weiter; die Dateien liegen unverändert im Archiv." >&2
		fi
	fi
	[ "$UPLOADS_RESTORED" = true ] || true
else
	# Das Archiv sagt selbst, warum nichts drin ist (manifest "uploads"-Feld,
	# von backup.sh gesetzt). "Nicht nachgesehen" darf nicht wie "nichts da"
	# aussehen — sonst haelt der Betreiber ein DB-only-Archiv fuer vollstaendig.
	ARCHIVE_UPLOADS_STATE="unknown"
	if [ -f "$WORK_DIR/manifest.json" ]; then
		ARCHIVE_UPLOADS_STATE="$(sed -n 's/.*"uploads"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' "$WORK_DIR/manifest.json" | head -1)"
		[ -n "$ARCHIVE_UPLOADS_STATE" ] || ARCHIVE_UPLOADS_STATE="unknown"
	fi
	case "$ARCHIVE_UPLOADS_STATE" in
	unchecked)
		echo "⚠  Dieses Archiv enthaelt KEINE Evidence-Dateien, weil das Backup sie nicht" >&2
		echo "   pruefen konnte (kein Docker-Zugriff im Backup-Kontext). Die Dateien sind" >&2
		echo "   NICHT verloren — sie sind nur nicht in diesem Archiv. Separat wiederherstellen." >&2
		;;
	absent)
		echo "   (Archiv wurde ohne Evidence-Dateien erstellt — es gab damals keine.)"
		;;
	*)
		echo "   (No uploads.tar.gz in archive, und das Manifest sagt nicht warum — Archiv aus einer aelteren Vakt-Version?)" >&2
		;;
	esac
fi

# ─── ESK-7 Fix 3: pg_restore-Exit auswerten statt pauschal daran zu sterben ───
# `pg_restore` beendet mit 1, sobald IRGENDEIN Statement fehlschlug — auch dann,
# wenn nur `--clean` ein Constraint nicht loesen konnte, das es gar nicht loesen
# muss. Ueber einer bestehenden Vakt-Datenbank ist das der Normalfall und nicht
# die Ausnahme: jede `audit_log`-Partition (Migration 151) erbt ihren
# Primaerschluessel vom Elternteil, und `ALTER TABLE ONLY <partition> DROP
# CONSTRAINT` ist dort per Definition unzulaessig. Gemessen gegen echtes
# Postgres 16: rc=1, eine Meldung je Partition, "errors ignored on restore: N" —
# und die Daten liegen danach korrekt in der Datenbank.
# Der Fix ist NICHT, den Exit-Code zu ignorieren (das waere die stille Variante
# desselben Fehlers), sondern die Fehlerzeilen zu klassifizieren UND danach an
# der Datenbank zu pruefen, dass der Restore wirklich gelandet ist.
RESTORE_LOG="$WORK_DIR/pg_restore.log"
set +e
vakt_pg_restore_from "$WORK_DIR/db.pgdump" >"$RESTORE_LOG" 2>&1
RESTORE_RC=$?
set -e

TOTAL_ERRORS=$(grep -c '^pg_restore: error:' "$RESTORE_LOG" 2>/dev/null || true)
BENIGN_ERRORS=$(grep -c 'cannot drop inherited constraint' "$RESTORE_LOG" 2>/dev/null || true)
TOTAL_ERRORS=${TOTAL_ERRORS:-0}
BENIGN_ERRORS=${BENIGN_ERRORS:-0}

if [ "$RESTORE_RC" -eq 0 ]; then
	echo "✓ Datenbank wiederhergestellt (pg_restore ohne Fehler)"
elif [ "$TOTAL_ERRORS" -gt 0 ] && [ "$TOTAL_ERRORS" -eq "$BENIGN_ERRORS" ]; then
	echo "✓ Datenbank wiederhergestellt (${BENIGN_ERRORS}× 'cannot drop inherited constraint'"
	echo "  auf audit_log-Partitionen — erwartet beim Restore ueber eine bestehende DB,"
	echo "  die Daten sind davon nicht betroffen)."
else
	echo "ERROR: pg_restore endete mit rc=${RESTORE_RC} und ${TOTAL_ERRORS} Fehler(n)," >&2
	echo "       davon ${BENIGN_ERRORS} aus der bekannten harmlosen Klasse. Auszug:" >&2
	grep '^pg_restore: error:' "$RESTORE_LOG" | grep -v 'cannot drop inherited constraint' | head -10 >&2
	echo "       Die Evidence-Dateien wurden VORHER wiederhergestellt und sind unversehrt." >&2
	echo "       Vollstaendiges Protokoll: ${RESTORE_LOG} (wird beim Beenden geloescht — jetzt sichern)" >&2
	exit 1
fi

# Positive Gegenprobe: der Zaehler kommt aus der DATENBANK, nicht aus der Absicht
# des Skripts. Eine Klassifizierung von Fehlerzeilen sagt "die Meldungen sehen
# harmlos aus" — sie sagt nicht "die Daten sind da".
#
# Kalibrierung (bewusst, nicht aus Bequemlichkeit): HART faellt nur der Fall
# `0 Tabellen`. Der ist eindeutig kaputt und hat keine legitime Ursache. Eine
# ABWEICHUNG zwischen erwarteter und tatsaechlicher Zahl wird laut gemeldet, aber
# nicht als Fehler gewertet — sie hat legitime Ursachen, und ein Check, der bei
# gesundem System rot wird, wird abgeschaltet statt gelesen.
# Konkret gemessen an einem Vakt-artigen Schema: `pg_restore --list` fuehrt fuer
# eine partitionierte Tabelle NEBEN den Tabellen selbst auch `TABLE ATTACH`- und
# `TABLE DATA`-Eintraege. Ein naives `grep -c ' TABLE '` zaehlte deshalb 10 statt
# 6 und haette bei JEDEM Vakt-Restore Alarm geschlagen. Die Zeilenform ist daher
# verankert: `<id>; <oid> <oid> TABLE <schema> <name> <owner>` — genau drei Felder
# hinter TABLE. Eine Fehlinterpretation kann die Erwartung damit nur SENKEN, also
# nur zu spaet warnen, nie falsch.
EXPECTED_TABLES=0
if command -v pg_restore >/dev/null 2>&1; then
	EXPECTED_TABLES=$(pg_restore --list "$WORK_DIR/db.pgdump" 2>/dev/null |
		grep -cE '^[0-9]+; [0-9]+ [0-9]+ TABLE [^ ]+ [^ ]+ [^ ]+$' || true)
	EXPECTED_TABLES=${EXPECTED_TABLES:-0}
fi

set +e
ACTUAL_TABLES=$(vakt_pg_base_table_count)
COUNT_RC=$?
set -e
if [ "$COUNT_RC" -ne 0 ]; then
	echo "   WARNUNG: Tabellenzahl nicht nachpruefbar (psql nicht erreichbar) —" >&2
	echo "            der Restore ist damit NICHT gegengeprueft, nur unbeanstandet." >&2
elif [ "$ACTUAL_TABLES" -eq 0 ]; then
	echo "ERROR: Nach dem Restore stehen 0 Tabellen in der Datenbank. pg_restore hat" >&2
	echo "       nichts hergestellt — das ist kein erfolgreicher Restore." >&2
	echo "       Die Evidence-Dateien wurden VORHER wiederhergestellt und sind unversehrt." >&2
	exit 1
elif [ "$EXPECTED_TABLES" -gt 0 ] && [ "$ACTUAL_TABLES" -lt "$EXPECTED_TABLES" ]; then
	echo "⚠  Gegenprobe: der Dump deklariert ${EXPECTED_TABLES} Tabellen, in public stehen" >&2
	echo "   ${ACTUAL_TABLES}. Das kann legitim sein (Objekte in anderen Schemas), sollte" >&2
	echo "   vor der Freigabe aber angesehen werden." >&2
else
	echo "✓ Gegenprobe: ${ACTUAL_TABLES} Tabellen in der Datenbank (Dump deklariert ${EXPECTED_TABLES})"
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
