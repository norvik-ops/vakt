#!/usr/bin/env bash
# update.sh — Safely update a self-hosted Vakt instance.
# Usage: ./scripts/update.sh [--no-backup] [--skip-artifacts] [--tag <version>]
#
# Zielversion (R1-KW-19): VAKT_TAG aus der .env, ohne Zeile "latest". --tag ist
# nur noch eine Gegenprobe: weicht es von der .env ab, stoppt das Update vor der
# ersten Aenderung — sonst startet das naechste `docker compose up -d` (das die
# .env liest) wieder die alte Version gegen die schon migrierte Datenbank.
set -euo pipefail

# ── Config ────────────────────────────────────────────────────────────────────
# R1-UPD-B1: KEIN `-f "$COMPOSE_FILE"` mehr an den compose-Aufrufen. Ein
# ausdrueckliches -f schaltet das automatische Laden von
# docker-compose.override.yml ab — genau der Ort, an dem docs/wiki/upgrade.md
# eigene Anpassungen empfiehlt — und kann keine Liste wie
# "docker-compose.yml:docker-compose.observability.yml". Compose liest
# COMPOSE_FILE selbst (Umgebung oder .env), ohne Angabe docker-compose.yml plus
# die Override-Datei.
SERVICE_API="api"
SERVICE_WORKER="worker"
SERVICE_FRONTEND="frontend"
SERVICE_SCANNERS="vakt-scanners"
HEALTH_URL="${VAKT_HEALTH_URL:-http://localhost/health/ready}"
HEALTH_RETRIES=30
HEALTH_WAIT=2
SKIP_BACKUP=false
SKIP_ARTIFACTS=false
TAG=""
TOTAL_STEPS=6

# ── Dependency checks ─────────────────────────────────────────────────────────
if ! command -v docker &>/dev/null; then
	echo "ERROR: Docker is not installed. See https://docs.docker.com/get-docker/"
	exit 1
fi

if docker compose version &>/dev/null 2>&1; then
	COMPOSE_CMD="docker compose"
elif docker-compose version &>/dev/null 2>&1; then
	COMPOSE_CMD="docker-compose"
else
	echo "ERROR: docker compose (v2) or docker-compose (v1) is required."
	exit 1
fi

# ── Parse flags ───────────────────────────────────────────────────────────────
while [[ $# -gt 0 ]]; do
	case "$1" in
	--no-backup)
		SKIP_BACKUP=true
		shift
		;;
	# R1-06-D00: Ausweg fuer Installationen, die kein git-Checkout sind oder
	# lokale Anpassungen tragen. Der Ausweg ist bewusst ein FLAG und kein
	# stiller Rueckfall — und der Abschluss sagt hinterher, dass die Artefakte
	# alt geblieben sind.
	--skip-artifacts)
		SKIP_ARTIFACTS=true
		shift
		;;
	--tag)
		TAG="${2:-}"
		[[ -n "$TAG" ]] || {
			echo "ERROR: --tag braucht eine Version, z. B. --tag v0.46.0" >&2
			exit 1
		}
		shift 2
		;;
	*)
		echo "WARNING: Unknown argument: $1"
		shift
		;;
	esac
done

# Load .env if present
if [ -f .env ]; then
	set -a
	source .env
	set +a
fi

BACKUP_DIR="${VAKT_BACKUP_DIR:-./backups}"

# ── Zielversion bestimmen (R1-KW-19) ─────────────────────────────────────────
# Bis 2026-10-31 stand hier fest TAG="latest", und weiter unten ueberschrieb
# `export VAKT_TAG="$TAG"` einen Pin aus der .env still. Der Betreiber glaubte,
# auf seiner gepinnten Version zu bleiben, und bekam latest — oder umgekehrt:
# Nach dem Update startete ein spaeteres `docker compose up -d` mit dem Pin aus
# der .env wieder die alte Version gegen die migrierte Datenbank.
#
# --tag muss mit der .env uebereinstimmen. Jedes spaetere `docker compose up -d`
# liest VAKT_TAG aus der .env — ohne Zeile ist das `latest`, und zwar das LOKAL
# liegende latest-Image, also meist das alte. Ein --tag, das die .env nicht
# traegt, gilt deshalb nur fuer diesen einen Lauf und kippt beim naechsten
# Neustart zurueck auf die alte Version gegen die migrierte Datenbank.
PINNED_TAG="${VAKT_TAG:-}"
if [[ -n "$TAG" ]]; then
	if [[ "${PINNED_TAG:-latest}" != "$TAG" ]]; then
		echo "ERROR: Das Update soll auf '${TAG}' (--tag), VAKT_TAG in der .env (bzw. Umgebung)" >&2
		echo "       steht aber auf '${PINNED_TAG:-<nicht gesetzt> = latest}'." >&2
		echo "       Jedes spaetere 'docker compose up -d' liest die .env und wuerde wieder" >&2
		echo "       '${PINNED_TAG:-latest}' starten — gegen eine schon auf '${TAG}' migrierte Datenbank." >&2
		echo "       Erst in der .env 'VAKT_TAG=${TAG}' setzen, dann ./scripts/update.sh" >&2
		echo "       (ohne --tag) starten. Es wurde nichts veraendert." >&2
		exit 1
	fi
elif [[ -n "$PINNED_TAG" ]]; then
	TAG="$PINNED_TAG"
else
	TAG="latest"
fi

echo "==> Vakt Update — $(date '+%Y-%m-%d %H:%M:%S')"
echo "    Target tag:   ${TAG}"
echo "    Compose file: ${COMPOSE_FILE:-docker-compose.yml (+ docker-compose.override.yml, falls vorhanden)}"
if [[ -n "$PINNED_TAG" ]] && [[ "$PINNED_TAG" != "latest" ]]; then
	echo ""
	echo "    HINWEIS: VAKT_TAG ist in der .env auf '${PINNED_TAG}' festgelegt."
	echo "    Dieses Update bleibt auf '${PINNED_TAG}'. Fuer eine neuere Version den Wert"
	echo "    in der .env anheben (oder die Zeile entfernen = latest) und erneut starten."
fi

# Show current running version as rollback reference.
CURRENT_TAG=$($COMPOSE_CMD images --format json "$SERVICE_API" 2>/dev/null |
	python3 -c "import sys,json; d=json.load(sys.stdin); print(d[0].get('Tag','unknown'))" 2>/dev/null ||
	echo "unknown")
echo "    Current tag:  ${CURRENT_TAG} (rollback reference)"

# Export VAKT_TAG so all compose commands below use the pinned image version
# (docker-compose.yml uses ${VAKT_TAG:-latest} for all vakt images).
export VAKT_TAG="$TAG"

# ── Step 0: Vorpruefung — alles, was das Update stoppen wuerde, VOR der ───────
#            ersten Aenderung feststellen.
# R1-06-D01: `backup.sh` bricht ohne Passphrase und ohne Terminal mit rc=1 ab
# (gemessen). Diese Pruefung nimmt das vorweg und nennt die Variable, statt den
# Betreiber einen gpg-Fehler lesen zu lassen.
#
# Sie ist NICHT die Absicherung gegen den gemischten Stand — das war sie bis
# 2026-08-02, und sie deckte dafuer genau einen von vielen Gruenden ab
# (REV-W1C F3). Diese Aufgabe traegt jetzt die Reihenfolge selbst: Step 1 ist
# das Backup, Step 2 die Umstellung. Was hier durchrutscht, faellt spaetestens
# in Step 1 auf — und dann ist noch nichts veraendert.
if [[ "$SKIP_BACKUP" == "false" ]]; then
	if [[ ! -f ./scripts/backup.sh ]]; then
		echo "ERROR: scripts/backup.sh nicht gefunden. Starte update.sh aus dem Vakt-Verzeichnis," >&2
		echo "       oder nutze --no-backup, wenn du von Hand gesichert hast." >&2
		exit 1
	fi
	if [ -z "${VAKT_BACKUP_PASSPHRASE:-}" ] &&
		{ [ -z "${VAKT_BACKUP_PASSPHRASE_FILE:-}" ] || [ ! -f "${VAKT_BACKUP_PASSPHRASE_FILE}" ]; } &&
		[ ! -t 0 ]; then
		echo "ERROR: Fuer das Backup ist keine Passphrase erreichbar, und dieser Lauf hat kein" >&2
		echo "       Terminal zum Nachfragen (Cron, ssh-Kommando, CI)." >&2
		echo "       Setze VAKT_BACKUP_PASSPHRASE oder VAKT_BACKUP_PASSPHRASE_FILE in der .env," >&2
		echo "       oder starte das Update interaktiv." >&2
		echo "       Es wurde nichts veraendert." >&2
		exit 1
	fi
fi

# ── Die Umstellung ist ab hier zuruecknehmbar zu MELDEN ──────────────────────
# R1-06-D01 / REV-W1C F3: Ein Update, das mittendrin abbricht, hinterlaesst
# unter Umstaenden neue Auslieferungs-Dateien und alte Container. Das ist ein
# gemischter Stand, der sich fuer heil haelt. Der Betreiber sieht als letzte
# Zeile den Fehler, der ihn dorthin gebracht hat — nicht, was jetzt gemischt
# ist. Diese Falle schliesst der EXIT-Trap: er redet nur, wenn wirklich etwas
# umgestellt wurde UND der Lauf scheitert.
ARTIFACTS_UPDATED=false
ARTIFACTS_FROM=""
RUNTIME_TOUCHED=false
BACKUP_ARCHIVE=""
BACKUP_MARKER=""
on_exit() {
	local rc=$?
	# REV-W1C F6: die Zeitmarke wird sonst nur im Erfolgsfall entfernt und
	# sammelt sich nach jedem fehlgeschlagenen Lauf im Backup-Verzeichnis an.
	if [[ -n "$BACKUP_MARKER" ]]; then rm -f "$BACKUP_MARKER"; fi
	# Nur in dem Fenster, in dem die Aussage auch stimmt: die Dateien sind neu,
	# die Container noch alt. Nach `up -d` traegt die Laufzeit den neuen Stand,
	# und dann ist "gemischt" das falsche Wort — dort greift die
	# Health-Check-Meldung mit dem restore.sh-Weg.
	if [[ "$rc" -ne 0 ]] && [[ "$ARTIFACTS_UPDATED" == "true" ]] && [[ "$RUNTIME_TOUCHED" == "false" ]]; then
		echo "" >&2
		echo "!! GEMISCHTER STAND — bitte lesen." >&2
		echo "   Das Update ist mit rc=${rc} abgebrochen, NACHDEM die Auslieferungs-Dateien" >&2
		echo "   (docker-compose.yml, Caddyfile, scripts/) schon auf den neuen Stand" >&2
		echo "   gehoben wurden. Die Container laufen weiter auf dem alten." >&2
		echo "" >&2
		if [[ -n "$ARTIFACTS_FROM" ]]; then
			echo "   Zuruecknehmen:  git -C . reset --hard ${ARTIFACTS_FROM}" >&2
		fi
		echo "   Weitermachen :  Grund oben beheben, dann ./scripts/update.sh erneut starten." >&2
		if [[ -n "$BACKUP_ARCHIVE" ]]; then
			echo "   Backup dieses Laufs: ${BACKUP_ARCHIVE}" >&2
		fi
	fi
}
trap on_exit EXIT

# ── Step 1: Backup ────────────────────────────────────────────────────────────
# REV-W1C F3: Dieser Schritt stand bis 2026-08-02 HINTER der Artefakt-Umstellung.
# Damit veraenderte das Update die Installation, bevor die Sicherung existierte —
# ein fehlgeschlagenes Backup liess einen gemischten Stand zurueck. Der
# Vorabtest in Step 0 deckte genau EINEN Grund ab (keine Passphrase und kein
# Terminal); Datenbank nicht erreichbar, Platte voll, gpg fehlt oder
# Uploads-Volume weg fielen alle in die Luecke. Auf Backup-Terrain ist ein halb
# umgestellter Stand der teuerste Fehler, deshalb steht das Backup wieder vorn.
#
# Der Preis, benannt: die Sicherung entsteht mit der ALTEN Fassung von
# backup.sh. Fuer einen Ruecksprung auf den alten Stand ist das die richtige
# Fassung — sie passt zu den Daten, die sie sichert.
echo ""
if [[ "$SKIP_BACKUP" == "false" ]]; then
	echo "==> Step 1/${TOTAL_STEPS}: Creating backup before update..."
	mkdir -p "$BACKUP_DIR"
	# Zeitmarke, damit nachher nur ein Archiv AUS DIESEM LAUF zaehlt — ein altes
	# Archiv im selben Verzeichnis darf einen fehlgeschlagenen Lauf nicht decken.
	# Die Sekunde Abstand ist kein Schoenheitsfehler: ohne sie kann das Archiv
	# dieselbe mtime tragen wie die Marke, und `find -newer` sieht es dann NICHT
	# — ein gutes Backup wuerde als fehlend gemeldet.
	BACKUP_MARKER="$(mktemp "${BACKUP_DIR}/.update-marker.XXXXXX")"
	sleep 1
	bash ./scripts/backup.sh "$BACKUP_DIR"

	# R1-06-D01: Ein Exit-Code nimmt kein Backup ab. Der Vorfall vom 2026-07-10
	# war ein Erzeuger, der Erfolg meldete und Leerdateien schrieb. Deshalb wird
	# hier nachgesehen, ob wirklich ein Archiv aus diesem Lauf existiert, wie
	# gross es ist und ob der Datenbank-Dump darin liegt.
	BACKUP_ARCHIVE="$(find "$BACKUP_DIR" -maxdepth 1 -type f -name 'vakt-backup-*.tar.gz' \
		-newer "$BACKUP_MARKER" 2>/dev/null | sort | tail -1)"
	rm -f "$BACKUP_MARKER"

	if [ -z "$BACKUP_ARCHIVE" ]; then
		echo "ERROR: backup.sh meldete Erfolg, aber in ${BACKUP_DIR} liegt kein Archiv aus diesem Lauf." >&2
		echo "       Das Update stoppt — ohne Sicherung wird hier nichts veraendert." >&2
		exit 1
	fi
	BACKUP_SIZE="$(stat -c%s "$BACKUP_ARCHIVE" 2>/dev/null || stat -f%z "$BACKUP_ARCHIVE")"
	if [ "$BACKUP_SIZE" -lt 1000 ]; then
		echo "ERROR: Das Backup-Archiv ist nur ${BACKUP_SIZE} Bytes gross (${BACKUP_ARCHIVE})." >&2
		echo "       Ein leeres Backup ist schlimmer als ein fehlgeschlagenes — Abbruch." >&2
		exit 1
	fi
	# Die Mitgliederliste wird ERST vollstaendig gelesen und DANN geprueft.
	# `tar … | grep -q` waere hier ein Wackler: grep beendet sich beim ersten
	# Treffer, tar bekommt SIGPIPE, und unter `set -o pipefail` ist der
	# Rueckgabewert der Kette dann 141 — ein gutes Archiv faellt sporadisch durch
	# (bei mir 1 von 5 Laeufen). Gleiche Klasse wie der $?-unter-set-e-Fund K1-F3.
	BACKUP_MEMBERS="$(tar -tzf "$BACKUP_ARCHIVE" 2>/dev/null || true)"
	case "$BACKUP_MEMBERS" in
	*db.pgdump.gpg*) ;;
	*)
		echo "ERROR: Im Archiv ${BACKUP_ARCHIVE} steckt kein Datenbank-Dump (db.pgdump.gpg)." >&2
		echo "       Abbruch — dieses Archiv wuerde eine Wiederherstellung nicht tragen." >&2
		exit 1
		;;
	esac
	if [ ! -s "${BACKUP_ARCHIVE}.sig" ]; then
		echo "ERROR: Zum Archiv ${BACKUP_ARCHIVE} fehlt die Signatur (.sig)." >&2
		echo "       restore.sh lehnt ein unsigniertes Archiv ab — Abbruch." >&2
		exit 1
	fi
	echo "    Backup abgenommen: ${BACKUP_ARCHIVE} (${BACKUP_SIZE} Bytes, Dump enthalten, signiert)"
else
	echo "==> Step 1/${TOTAL_STEPS}: Skipping backup (--no-backup)"
fi

# ── Step 2: Auslieferungs-Artefakte aktualisieren ────────────────────────────
# R1-06-D00: Ein `docker compose pull` bringt neue Binaries in eine ALTE
# Umgebung. docker-compose.yml, Caddyfile und scripts/ blieben auf dem Stand der
# Erstinstallation — zwischen v0.42.48 und v0.42.49 sind das allein in der
# Compose-Datei 112 Zeilen (die Docker-Secrets-Haertung). Sicherheitsfixes, die
# dort stecken, erreichten damit ausschliesslich Neuinstallationen.
echo ""
if [[ "$SKIP_ARTIFACTS" == "true" ]]; then
	echo "==> Step 2/${TOTAL_STEPS}: Auslieferungs-Artefakte uebersprungen (--skip-artifacts)"
else
	echo "==> Step 2/${TOTAL_STEPS}: Auslieferungs-Artefakte aktualisieren (Compose, Caddyfile, scripts)..."
	if [[ ! -f ./scripts/update-artifacts.sh ]]; then
		echo "ERROR: scripts/update-artifacts.sh fehlt — diese Installation kann ihre" >&2
		echo "       Compose-/Caddy-/Skript-Dateien nicht selbst aktualisieren." >&2
		echo "       Einmalig von Hand nachholen (docs/UPGRADE.md), dann erneut starten." >&2
		exit 1
	fi
	# Der Rueckfallpunkt, BEVOR etwas vorgespult wird — der EXIT-Trap gibt ihn
	# im Fehlerfall noch einmal aus, statt ihn sechs Zeilen weiter oben
	# vorbeiscrollen zu lassen.
	ARTIFACTS_FROM="$(git -C . rev-parse HEAD 2>/dev/null || echo '')"
	set +e
	bash ./scripts/update-artifacts.sh --dir . --ref "$TAG"
	ARTIFACTS_RC=$?
	set -e
	case "$ARTIFACTS_RC" in
	0) ARTIFACTS_UPDATED=true ;;
	3)
		echo "" >&2
		echo "ABBRUCH: Die Auslieferungs-Artefakte konnten nicht aktualisiert werden (Grund oben)." >&2
		echo "         Das Update stoppt hier, statt neue Images in eine alte Umgebung zu starten." >&2
		echo "         Bewusst trotzdem updaten: ./scripts/update.sh --skip-artifacts" >&2
		exit 1
		;;
	*)
		echo "ERROR: update-artifacts.sh endete unerwartet mit rc=${ARTIFACTS_RC}." >&2
		exit 1
		;;
	esac
fi

# ── Step 3: Pull new images ───────────────────────────────────────────────────
echo ""
echo "==> Step 3/${TOTAL_STEPS}: Pulling new images (VAKT_TAG=${VAKT_TAG})..."
# R1-KW-20: Ein gescheiterter Pull darf nie bis `up -d` durchlaufen. Dann
# startet Compose die ALTEN lokalen Images, migrate kennt die Datenbank-Version
# nicht ("no migration found for version …"), und die API kommt nicht hoch.
# `set -e` haette hier zwar auch abgebrochen, aber wortlos — der Betreiber
# braucht den Satz, was er jetzt tun soll und was er nicht tun darf.
if ! $COMPOSE_CMD pull "$SERVICE_API" "$SERVICE_WORKER" "$SERVICE_FRONTEND" "$SERVICE_SCANNERS"; then
	echo "" >&2
	echo "ABBRUCH: 'docker compose pull' ist fehlgeschlagen (Netz, Registry, Tag '${VAKT_TAG}' unbekannt?)." >&2
	echo "         Es wurde NICHT migriert und NICHTS neu gestartet — die laufenden Container" >&2
	echo "         sind unveraendert." >&2
	echo "         Jetzt NICHT 'docker compose up -d' ausfuehren: das startet die alten lokalen" >&2
	echo "         Images mit der neuen Compose-Datei." >&2
	echo "         Pull wiederholen: ./scripts/update.sh erneut starten." >&2
	exit 1
fi
echo "    Images pulled."
# Bewusst nur die vier Vakt-Images: Fremd-Images (postgres, redis, caddy,
# pgbouncer, ollama) zieht `up -d` in Schritt 5 selbst nach, falls die neue
# Compose-Datei dort eine andere Version verlangt — ein pauschales `pull` wuerde
# beim Kunden ohne Not mehrere GB neu laden.

# ── Step 4: Run migrations ────────────────────────────────────────────────────
echo ""
echo "==> Step 4/${TOTAL_STEPS}: Running database migrations..."
$COMPOSE_CMD run --rm migrate
echo "    Migrations complete."

# ── Step 5: Restart services ──────────────────────────────────────────────────
# R1-06-D00, zweite Haelfte: frueher lief hier
#   up -d --no-deps api worker frontend vakt-scanners
# — caddy, postgres, redis und pgbouncer blieben also unberuehrt. Genau in deren
# Compose-Bloecken steckt die Haertung (cap_drop, no-new-privileges, Docker-
# Secrets statt Klartext-Env). Eine neue Compose-Datei haette dort nichts
# bewirkt. `up -d` ueber die ganze Datei erzeugt genau die Dienste neu, deren
# Konfiguration sich geaendert hat, und laesst die uebrigen laufen — es ist
# derselbe Befehl, den docs/setup.md und docs/UPGRADE.md ohnehin nennen.
echo ""
echo "==> Step 5/${TOTAL_STEPS}: Restarting services (recreates every service whose config changed)..."
$COMPOSE_CMD up -d
RUNTIME_TOUCHED=true
echo "    Services restarting..."

# ── Step 6: Health check ──────────────────────────────────────────────────────
echo ""
echo "==> Step 6/${TOTAL_STEPS}: Waiting for health check (${HEALTH_URL})..."
for i in $(seq 1 "$HEALTH_RETRIES"); do
	# -k: Caddy redirects http://localhost → https with its INTERNAL CA cert,
	#     which curl does not trust (exit 60). The probe is loopback-only, so
	#     skipping cert verification is safe and avoids a false rollback advice
	#     after a successful upgrade.
	if curl -sfkL "$HEALTH_URL" >/dev/null 2>&1; then
		echo "    Health check passed after $((i * HEALTH_WAIT))s."
		echo ""
		echo "Update complete! Vakt is running with the new version."
		if [[ "$ARTIFACTS_UPDATED" == "false" ]]; then
			echo ""
			echo "⚠  Die Auslieferungs-Artefakte (docker-compose.yml, Caddyfile, scripts/) wurden"
			echo "   NICHT aktualisiert — sie stehen weiter auf dem Stand deiner letzten Aktualisierung."
			echo "   Neue Images laufen damit in der alten Umgebung; Haertungen und Konfigurations-"
			echo "   aenderungen dieser Version greifen NICHT. Weg: docs/UPGRADE.md."
		fi
		exit 0
	fi
	sleep "$HEALTH_WAIT"
done

echo ""
echo "ERROR: Health check failed after $((HEALTH_RETRIES * HEALTH_WAIT))s."
echo "  Check logs: $COMPOSE_CMD logs $SERVICE_API"
echo ""
if [ -n "$BACKUP_ARCHIVE" ]; then
	echo "  To rollback: ./scripts/restore.sh ${BACKUP_ARCHIVE}"
else
	echo "  To rollback: restore from your most recent backup — ./scripts/restore.sh <backup-file>"
fi
exit 1
