#!/usr/bin/env bash
set -euo pipefail

# ESK-7 (Codeaudit-v5b, R1-06-D01) — Regressionstest fuer die vier Wiring-Defekte
# des Kunden-Backup-/Restore-Wegs. Jeder Fall fuehrt den FEHLERFALL aus, nicht den
# Happy Path — genau das war die Luecke der bisherigen Abnahme:
#
#   W1  Host-pg_dump gegen ein `internal: true`-Netz. Der Standard-Compose-Stack
#       legt `postgres` auf `db-net` (internal, kein publizierter Port) — gemessen
#       schlaegt Host-pg_dump dort mit "could not translate host name" fehl. Der
#       Dump MUSS also im Container laufen.
#   W1b Derselbe Defekt auf der RUECKrichtung: restore.sh rief ebenfalls das
#       Host-`pg_restore`. Ein Fix nur am Dump haette die halbe Kette geheilt
#       (Variant-Miss-Klasse, CLAUDE.md 2026-07-11).
#   W2  pg_restore --clean --if-exists UEBER eine bestehende Vakt-DB endet mit
#       rc=1 ("cannot drop inherited constraint" je audit_log-Partition), obwohl
#       die Daten korrekt landen. Mit `set -e` riss das den Uploads-Block mit.
#   W3  Uploads muessen den DB-Schritt UEBERLEBEN: sie werden vor dem Punkt
#       gesichert, an dem der DB-Restore abbrechen kann.
#   W4  Ein LEERER Dump muss abgelehnt werden, auch wenn uploads.tar.gz das
#       Archiv ueber die Mindestgroesse hebt (gemessen: 1 KB Evidence genuegt,
#       damit ein 0-Byte-Dump die 1024-Byte-Schwelle passiert).
#   W5  Statisch: der Opt-in-Backup-Container teilt ein Netz mit postgres, und
#       docker-compose.backup.yml liegt im Public-Mirror.
#   W7  Die Container-Erkennung darf NIEMALS den Postgres eines fremden
#       Compose-Projekts nehmen. Auf einem MSP-Host laufen mehrere Kundenstacks;
#       ein Backup, das die falsche Datenbank dumpt, ist schlimmer als keins.
#   W11 (KONV-K1 F2) VAKT_BACKUP_PG_MODE=container ohne auffindbaren Container
#       MUSS abbrechen. Die erste Fassung fiel dort still auf den HOST-Weg
#       zurueck und fuhr `pg_restore --clean --if-exists` gegen die Datenbank
#       hinter VAKT_PG_URL — eine andere als die gewaehlte — und meldete rc=0.
#       W11b ist die Gegenprobe: auto/host ohne Container darf weiterhin
#       legitim auf den Host gehen, sonst ist der Fix eine Abrissbirne.
#   W12 (KONV-K1 F2) …und der ausdruecklich gewaehlte container-Modus muss auch
#       funktionieren. Die Suite fuhr vorher in 734 Zeilen ausschliesslich
#       MODE=host; der container-Modus stand in KEINEM Fall.
#
# W1–W4 brauchen Docker. Fehlt es, werden sie GEZAEHLT uebersprungen (kein
# stiller Skip — ein Gate, das Eingaben still verwirft, meldet Erfolg fuer Arbeit,
# die es nicht getan hat).
#
# Usage: bash scripts/backup_restore_wiring_test.sh

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

PASSED=0
FAILED=0
SKIPPED=0

pass() { echo "PASS: $1"; PASSED=$((PASSED + 1)); }
fail() { echo "FAIL: $1" >&2; FAILED=$((FAILED + 1)); }
skip() { echo "SKIP: $1"; SKIPPED=$((SKIPPED + 1)); }

# Zur LAUFZEIT erzeugt, nicht als Literal committet. Ein 32-Byte-Hex-String in
# einer eingecheckten Datei ist genau das, was ein Secret-Scanner finden SOLL —
# dass er hier harmlos ist, kann der Scanner nicht sehen (CLAUDE.md: die Regel
# gilt absolut, auch fuer Beispiel- und Platzhalterwerte). Ein Zufallswert macht
# die Ausnahme ueberfluessig, statt sie zu vermehren.
#
# WICHTIG: EINMAL erzeugen und in der Variablen halten. Dump und Restore muessen
# denselben Schluessel sehen — backup.sh leitet daraus den HMAC der Signatur ab,
# restore.sh prueft sie damit. Zweimal wuerfeln hiesse: jede Signatur ungueltig.
TEST_SECRET_KEY="$(openssl rand -hex 32)"
TEST_PASSPHRASE="esk7-$(openssl rand -hex 12)"

WORK="$(mktemp -d)"
PROJECT="esk7test$$"
PG_NAME="${PROJECT}-postgres-1"
NET_NAME="${PROJECT}_db-net"
UPLOADS_VOL="${PROJECT}_uploads_data"

# Der Trap raeumt ALLES ab, was dieser Lauf anlegt — auch die Container aus W6/W7/W9,
# die ihren eigenen `docker rm` haben. Bricht der Lauf vorher ab (und das ist bei
# einem Test, der Fehlerfaelle fährt, der Normalfall), bleiben sonst Container und
# Netze liegen: P14 „Aufraeumpflicht", und ein liegengebliebenes Netz laesst
# `docker network rm` beim naechsten Lauf scheitern.
# W8 fährt einen ECHTEN Compose-Stack hoch. Dessen Abbau muss im Trap stehen und
# nicht nur an den erwarteten Ausstiegspunkten — sonst bleiben bei jedem
# unerwarteten Abbruch Container UND Volumes liegen. W8_PROJECT/W8_PROJDIR sind
# hier bewusst leer vorbelegt, damit der Trap auch vor W8 schon sicher laeuft.
W8_PROJECT=""
W8_PROJDIR=""
cleanup() {
	docker rm -fv "$PG_NAME" "${PROJECT}-nodocker" "${PROJECT}-foreign-postgres-1" >/dev/null 2>&1 || true
	docker volume rm -f "$UPLOADS_VOL" >/dev/null 2>&1 || true
	docker network rm "$NET_NAME" >/dev/null 2>&1 || true
	if [ -n "$W8_PROJECT" ] && [ -n "$W8_PROJDIR" ]; then
		docker compose -p "$W8_PROJECT" --project-directory "$W8_PROJDIR" \
			-f "$REPO_ROOT/docker-compose.yml" -f "$REPO_ROOT/docker-compose.backup.yml" \
			--profile backup down -v --remove-orphans >/dev/null 2>&1 || true
	fi
	rm -rf "$WORK"
}
trap cleanup EXIT

# ── W5: statische Verdrahtung (braucht kein Docker) ────────────────────────────

set +e
python3 - "$REPO_ROOT" <<'PY'
import pathlib, sys, re
root = pathlib.Path(sys.argv[1])
rc = 0

# (a) Der backup-Service muss ein Netz mit postgres teilen.
base = (root / "docker-compose.yml").read_text()
overlay = (root / "docker-compose.backup.yml").read_text()

def service_networks(text, service):
    """Netzwerkliste eines Service aus einem Compose-Text (ohne PyYAML-Abhaengigkeit).

    Kommentarzeilen INNERHALB des networks:-Blocks werden uebersprungen — ein
    Parser, der an einem '#' abbricht, meldet 'kein Netz' fuer einen korrekt
    verdrahteten Service (in genau diese Falle ist dieser Test erst gelaufen).
    """
    m = re.search(rf"^  {service}:\n(.*?)(?=^  \S|\Z)", text, re.S | re.M)
    if not m:
        return None
    body = m.group(1)
    lines = body.split("\n")
    try:
        start = next(i for i, l in enumerate(lines) if l.strip() == "networks:")
    except StopIteration:
        return []
    nets = []
    for l in lines[start + 1:]:
        s = l.strip()
        if not s or s.startswith("#"):
            continue
        if s.startswith("- "):
            nets.append(s[2:].strip())
            continue
        break  # naechster Schlüssel auf Service-Ebene → Block zu Ende
    return nets

pg_nets = service_networks(base, "postgres")
bk_nets = service_networks(overlay, "backup")
if pg_nets is None:
    print("FAIL: W5a — postgres-Service in docker-compose.yml nicht gefunden"); rc = 1
elif not bk_nets:
    print(f"FAIL: W5a — backup-Service hat KEINE networks: (postgres liegt auf {pg_nets}); "
          "er landet damit im default-Netz und erreicht die DB nie"); rc = 1
elif not (set(bk_nets) & set(pg_nets)):
    print(f"FAIL: W5a — backup-Netze {bk_nets} und postgres-Netze {pg_nets} sind disjunkt"); rc = 1
else:
    print(f"PASS: W5a — backup teilt {sorted(set(bk_nets) & set(pg_nets))} mit postgres")

# (b) docker-compose.backup.yml muss im Public-Mirror liegen …
# Geprueft wird die KOPIER-LISTE, nicht das Vorkommen irgendwo in der Datei: ein
# blosses `"docker-compose.backup.yml" in text` wird auch von einem Kommentar
# erfuellt, der die Datei nur erwaehnt — beim Nicht-Vakuitaets-Nachweis blieb
# dieser Check deshalb gruen, obwohl der Eintrag entfernt war.
#
# W5b und W5c lesen zwei Dateien, die es NUR im privaten Repo gibt: die
# Bau-Vorschrift des Mirrors und den Sync-Workflow. Diese Suite laeuft aber auch
# IM gebauten Mirror (sie steht in dessen `make test`) — dort ist die Zusage
# strukturell nicht pruefbar, und ein ungeschuetztes read_text() hat `make test`
# beim Kunden mit einem Python-Traceback abgebrochen. Uebersprungen wird deshalb
# GEZAEHLT und in ERGEBNIS ausgewiesen, nie still.
skipped = 0
mirror_sh = root / "scripts" / "build-public-mirror.sh"
if not mirror_sh.is_file():
    print("SKIP: W5b — scripts/build-public-mirror.sh liegt nicht in diesem Repo "
          "(gebauter Public Mirror); die Kopier-Liste ist hier nicht pruefbar")
    skipped += 1
else:
    mirror = mirror_sh.read_text()
    copy_loop = re.search(r"^for f in (.*?);\s*do$", mirror, re.S | re.M)
    copy_list = copy_loop.group(1).replace("\\\n", " ").split() if copy_loop else []
    if "docker-compose.backup.yml" not in copy_list:
        print("FAIL: W5b — docker-compose.backup.yml steht nicht in der Kopier-Liste von "
              f"build-public-mirror.sh (gefunden: {copy_list}); der Self-Hoster hat den "
              "Scheduler nie gesehen"); rc = 1
    else:
        print("PASS: W5b — docker-compose.backup.yml steht in der Mirror-Kopier-Liste")

# (c) … und eine Aenderung daran muss den Sync ueberhaupt ausloesen.
# Auch hier STRUKTURELL: die `paths:`-Liste parsen, nicht die Datei nach einem
# Substring durchsuchen. Ein `in text` wird von jedem Kommentar erfuellt, der die
# Datei bloss erwaehnt — dieser Check blieb deshalb gruen, nachdem die
# Trigger-Zeile geloescht und nur der erklaerende Kommentar stehen geblieben war.
# Genau der Fehler, den W5b eine Funktion weiter oben schon behoben hatte.
sync_wf = root / ".github" / "workflows" / "sync-public-repo.yml"
if not sync_wf.is_file():
    print("SKIP: W5c — .github/workflows/sync-public-repo.yml liegt nicht in diesem Repo "
          "(gebauter Public Mirror); die paths:-Trigger-Liste ist hier nicht pruefbar")
    skipped += 1
    sys.exit(rc if rc else 10 + skipped)
sync_lines = sync_wf.read_text().split("\n")
trigger_paths, in_paths = [], False
for line in sync_lines:
    s = line.strip()
    if s == "paths:":
        in_paths = True
        continue
    if in_paths:
        if s.startswith("#") or not s:
            continue
        if s.startswith("- "):
            trigger_paths.append(s[2:].strip())
            continue
        in_paths = False  # naechster Schluessel (z. B. `tags:`) → Liste zu Ende
if "docker-compose.backup.yml" not in trigger_paths:
    print("FAIL: W5c — docker-compose.backup.yml steht nicht in der paths:-Trigger-Liste von "
          f"sync-public-repo.yml (gefunden: {trigger_paths}); eine Aenderung daran erreicht "
          "den Mirror erst zufaellig beim naechsten fremden Push"); rc = 1
else:
    print(f"PASS: W5c — docker-compose.backup.yml steht in der paths:-Trigger-Liste "
          f"({len(trigger_paths)} Eintraege geparst)")

# Exit-Kontrakt: 0 = alle drei gruen · 1 = mindestens eine FAIL ·
# 10+n = n von dreien strukturell nicht pruefbar (Datei nur im privaten Repo).
sys.exit(rc if rc else 10 + skipped if skipped else 0)
PY
W5_RC=$?
set -e
if [ "$W5_RC" -eq 0 ]; then
	PASSED=$((PASSED + 3))
elif [ "$W5_RC" -ge 10 ]; then
	W5_SKIPPED=$((W5_RC - 10))
	PASSED=$((PASSED + 3 - W5_SKIPPED))
	SKIPPED=$((SKIPPED + W5_SKIPPED))
else
	FAILED=$((FAILED + 1))
fi

# ── W11: VAKT_BACKUP_PG_MODE=container ohne Container MUSS abbrechen ──────────
# KONV-K1 F2. Die bisherige Suite fuhr in 734 Zeilen ausschliesslich MODE=host;
# der container-Modus stand in KEINEM Fall — weder Erfolgs- noch
# Nicht-gefunden-Pfad.
#
# Defektzustand (gemessen gegen 42763c0): `_vakt_pg_pick` machte sein `exit 1` im
# container-Zweig INNERHALB einer Kommandosubstitution. Das beendete nur die
# Subshell, der `if` sah rc!=0 und nahm den HOST-Zweig. Ergebnis:
#   DESCRIBE: host pg_dump/pg_restore          ← der ABGEWAEHLTE Weg, als Logzeile
#   HOST-pg_dump    ... postgres://…/WRONG_DB
#   HOST-pg_restore --clean --if-exists -d postgres://…/WRONG_DB
#   GESAMT-RC=0
# Also ein destruktiver Restore gegen eine andere Datenbank als die gewaehlte,
# unter Erfolgsmeldung.
#
# Der Fall braucht KEIN Docker und wird deshalb NICHT uebersprungen: er ist der
# eigentliche Regressionstest. Die Spione stehen im PATH, damit nicht "es ist
# nichts passiert" geprueft wird, sondern "der Host-Weg wurde nachweislich nicht
# betreten" — ohne je eine echte Datenbank anzufassen.
SPY_DIR="$WORK/spy"
SPY_LOG="$WORK/spy.log"
mkdir -p "$SPY_DIR"
cat >"$SPY_DIR/pg_dump" <<'SPY'
#!/usr/bin/env bash
echo "HOST-pg_dump: $*" >>"$SPYLOG"
out=""; prev=""
for a in "$@"; do
	[ "$prev" = "-f" ] && out="$a"
	case "$a" in --file=*) out="${a#--file=}" ;; esac
	prev="$a"
done
[ -n "$out" ] && : >"$out"
exit 0
SPY
cat >"$SPY_DIR/pg_restore" <<'SPY'
#!/usr/bin/env bash
echo "HOST-pg_restore: $*" >>"$SPYLOG"
exit 0
SPY
cat >"$SPY_DIR/psql" <<'SPY'
#!/usr/bin/env bash
echo "HOST-psql: $*" >>"$SPYLOG"
echo 0
exit 0
SPY
chmod +x "$SPY_DIR/pg_dump" "$SPY_DIR/pg_restore" "$SPY_DIR/psql"

# Fuehrt die Modul-Funktionen in der Reihenfolge, in der backup.sh/restore.sh sie
# benutzen — inklusive der Kommandosubstitution um vakt_pg_describe, denn genau
# die ist der Mechanismus des Defekts.
cat >"$WORK/drive-pg-target.sh" <<'DRIVE'
set -euo pipefail
source "$1"
vakt_pg_require_valid_mode
echo "DESCRIBE: $(vakt_pg_describe)"
vakt_pg_dump_to "$2"
vakt_pg_restore_from "$2"
echo "NICHT-ABGEBROCHEN"
DRIVE

WRONG_DB_URL="postgres://vakt:vakt@127.0.0.1:5432/DIESE_DB_DARF_NIE_GETROFFEN_WERDEN?sslmode=disable"
: >"$SPY_LOG"
set +e
( cd "$WORK" &&
	PATH="$SPY_DIR:$PATH" \
	SPYLOG="$SPY_LOG" \
	COMPOSE_PROJECT_NAME="konvk1-no-such-project-$$" \
	VAKT_BACKUP_PG_MODE=container \
	VAKT_BACKUP_PG_CONTAINER="konvk1-does-not-exist-$$" \
	VAKT_PG_URL="$WRONG_DB_URL" \
	bash "$WORK/drive-pg-target.sh" "$SCRIPT_DIR/backup-pg-target.sh" "$WORK/w11.dump" ) \
	>"$WORK/w11.out" 2>&1
W11_RC=$?
set -e
W11_SPY="$(cat "$SPY_LOG")"
if [ "$W11_RC" -eq 0 ]; then
	fail "W11 — MODE=container ohne Container meldete Erfolg (rc=0) statt abzubrechen"
elif [ -n "$W11_SPY" ]; then
	fail "W11 — Rueckfall auf den HOST-Weg trotz MODE=container: $(printf '%s' "$W11_SPY" | tr '\n' ' ')"
elif grep -q 'host pg_dump/pg_restore' "$WORK/w11.out"; then
	fail "W11 — das Log meldet den abgewaehlten Host-Weg: $(grep 'host pg_dump' "$WORK/w11.out" | head -1)"
elif grep -q 'NICHT-ABGEBROCHEN' "$WORK/w11.out"; then
	fail "W11 — Ausfuehrung lief nach dem Fehler weiter"
elif ! grep -q 'kein postgres-Container gefunden' "$WORK/w11.out"; then
	fail "W11 — abgebrochen (rc=${W11_RC}), aber ohne die erklaerende Meldung: $(tail -2 "$WORK/w11.out" | tr '\n' ' ')"
else
	pass "W11 — MODE=container ohne Container bricht ab (rc=${W11_RC}), kein Host-pg_dump/pg_restore aufgerufen"
fi

# ── W11b: MODE=host/auto ohne Container faellt weiterhin LEGITIM auf den Host ──
# Gegenprobe zu W11. Ohne sie wuerde W11 auch von einem Fix gruen gemacht, der
# die Container-Erkennung einfach ganz abschaltet oder jeden Nicht-Treffer zum
# Abbruch erklaert — dann waere der dokumentierte host-Weg (externes/managed
# Postgres) mitgestorben.
: >"$SPY_LOG"
set +e
( cd "$WORK" &&
	PATH="$SPY_DIR:$PATH" \
	SPYLOG="$SPY_LOG" \
	COMPOSE_PROJECT_NAME="konvk1-no-such-project-$$" \
	VAKT_BACKUP_PG_MODE=auto \
	VAKT_PG_URL="$WRONG_DB_URL" \
	bash "$WORK/drive-pg-target.sh" "$SCRIPT_DIR/backup-pg-target.sh" "$WORK/w11b.dump" ) \
	>"$WORK/w11b.out" 2>&1
W11B_RC=$?
set -e
if [ "$W11B_RC" -ne 0 ]; then
	fail "W11b — MODE=auto ohne Container bricht ab (rc=${W11B_RC}), obwohl der Host-Weg gemeint ist: $(tail -2 "$WORK/w11b.out" | tr '\n' ' ')"
elif ! grep -q 'DESCRIBE: host pg_dump/pg_restore' "$WORK/w11b.out"; then
	fail "W11b — MODE=auto meldet nicht den Host-Weg: $(grep DESCRIBE "$WORK/w11b.out" | head -1)"
elif ! grep -q 'HOST-pg_restore' "$SPY_LOG"; then
	fail "W11b — MODE=auto hat den Host-pg_restore nicht aufgerufen: $(cat "$SPY_LOG" | tr '\n' ' ')"
else
	pass "W11b — MODE=auto ohne Container nimmt weiterhin den Host-Weg und meldet ihn korrekt"
fi

# ── Docker-Vorbedingung fuer W1–W4 ────────────────────────────────────────────

MISSING_TOOL=""
docker info >/dev/null 2>&1 || MISSING_TOOL="Docker"
[ -n "$MISSING_TOOL" ] || command -v gpg >/dev/null 2>&1 || MISSING_TOOL="gpg"
if [ -n "$MISSING_TOOL" ]; then
	skip "W1 (Container-Dump gegen internal:true) — ${MISSING_TOOL} nicht verfuegbar"
	skip "W1b (Container-Restore gegen internal:true) — ${MISSING_TOOL} nicht verfuegbar"
	skip "W2 (pg_restore-Exit ueber bestehender DB) — ${MISSING_TOOL} nicht verfuegbar"
	skip "W3 (Uploads ueberleben den DB-Abbruch) — ${MISSING_TOOL} nicht verfuegbar"
	skip "W6 (Uploads ueberleben fatalen DB-Fehler) — ${MISSING_TOOL} nicht verfuegbar"
	skip "W4 (leerer Dump trotz Uploads-Volumen) — ${MISSING_TOOL} nicht verfuegbar"
	skip "W7 (fremdes Compose-Projekt) — ${MISSING_TOOL} nicht verfuegbar"
	skip "W12 (MODE=container Erfolgspfad) — ${MISSING_TOOL} nicht verfuegbar"
	echo
	echo "ERGEBNIS: passed=$PASSED failed=$FAILED skipped=$SKIPPED"
	[ "$FAILED" -eq 0 ] || exit 1
	exit 0
fi

# Produktions-Topologie nachbauen: internal-Netz, KEIN publizierter Port.
docker network create --internal "$NET_NAME" >/dev/null
docker volume create "$UPLOADS_VOL" >/dev/null
docker run -d --name "$PG_NAME" --network "$NET_NAME" \
	--label com.docker.compose.project="$PROJECT" \
	--label com.docker.compose.service=postgres \
	-e POSTGRES_USER=vakt -e POSTGRES_PASSWORD=vakt -e POSTGRES_DB=vakt \
	postgres:16-alpine >/dev/null

# 90 s statt 60: die Suite startet mehrere Container hintereinander, und unter
# Last braucht die Initialisierung des offiziellen Images laenger.
for _ in $(seq 1 90); do
	docker exec "$PG_NAME" pg_isready -U vakt >/dev/null 2>&1 && break
	sleep 1
done
docker exec "$PG_NAME" pg_isready -U vakt >/dev/null 2>&1 || {
	# Ueber den regulaeren Zaehler + die Zusammenfassung austreten, nicht per nacktem
	# `exit 1`: sonst fehlt die ERGEBNIS-Zeile und der Lauf sieht aus wie „abgebrochen"
	# statt wie „konnte nicht pruefen". Das ist ein UMGEBUNGS-Fehlschlag (Docker unter
	# Last), nicht ein Fehlschlag ueber den Fix — er wird deshalb ausdruecklich so
	# benannt, damit ihn niemand als Regression liest.
	fail "Testaufbau — Test-Postgres wurde binnen 90 s nicht bereit (Umgebung, nicht der Fix)"
	echo
	echo "ERGEBNIS: passed=$PASSED failed=$FAILED skipped=$SKIPPED"
	exit 1
}

# Vakt-artiges Schema: partitioniertes audit_log ist der Ausloeser von W2.
cat >"$WORK/schema.sql" <<'SQL'
CREATE TABLE audit_log (
  id UUID NOT NULL DEFAULT gen_random_uuid(),
  org_id UUID NOT NULL,
  action TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (id, created_at)
) PARTITION BY RANGE (created_at);
CREATE TABLE audit_log_2025    PARTITION OF audit_log FOR VALUES FROM ('2025-01-01') TO ('2026-01-01');
CREATE TABLE audit_log_2026    PARTITION OF audit_log FOR VALUES FROM ('2026-01-01') TO ('2027-01-01');
CREATE TABLE audit_log_2027    PARTITION OF audit_log FOR VALUES FROM ('2027-01-01') TO ('2028-01-01');
CREATE TABLE audit_log_default PARTITION OF audit_log DEFAULT;
INSERT INTO audit_log (org_id, action, created_at) VALUES (gen_random_uuid(), 'esk7', '2026-05-01');
CREATE TABLE ck_evidence (id serial PRIMARY KEY, note TEXT NOT NULL);
INSERT INTO ck_evidence (note) VALUES ('evidence-row-1'), ('evidence-row-2');
SQL
docker exec -i "$PG_NAME" psql -q -U vakt -d vakt <"$WORK/schema.sql"

# Evidence-Datei ins Uploads-Volume (muss den Restore ueberleben).
#
# Zusaetzlich 64 KB inkompressible Nutzlast: OHNE sie bleibt das Archiv selbst mit
# einem 0-Byte-Dump unter der 1000-Byte-Archivschwelle, und W4 wuerde von DIESER
# Schwelle gruen gemacht statt von der Dump-Assertion, die es pruefen soll —
# gemessen genau so passiert. Erst ab ~1 KB echter Evidence traegt das Archiv
# einen leeren Dump ueber beide Schwellen, und genau das ist der Defektzustand.
EVIDENCE_MARKER="esk7-evidence-must-survive"
docker run --rm -v "${UPLOADS_VOL}:/data" alpine:latest \
	sh -c "mkdir -p /data/evidence && printf '%s' '$EVIDENCE_MARKER' > /data/evidence/proof.txt && dd if=/dev/urandom of=/data/evidence/bulk.bin bs=1024 count=64 2>/dev/null"

# Die URL, die ein Betreiber laut .env.example hat: sie zeigt auf einen Host, den
# der Host-Prozess NICHT erreichen kann. Genau das ist der Defekt.
DB_URL="postgres://vakt:vakt@postgres:5432/vakt?sslmode=disable"

run_backup() {
	( cd "$WORK" &&
		COMPOSE_PROJECT_NAME="$PROJECT" \
		VAKT_BACKUP_PG_CONTAINER="$PG_NAME" \
		VAKT_DB_URL="$DB_URL" \
		VAKT_SECRET_KEY="$TEST_SECRET_KEY" \
		VAKT_BACKUP_PASSPHRASE="$TEST_PASSPHRASE" \
		bash "$SCRIPT_DIR/backup.sh" "$WORK" )
}

# ── W1: Dump gegen internal:true ──────────────────────────────────────────────

if run_backup >"$WORK/backup.out" 2>&1; then
	ARCHIVE="$(find "$WORK" -maxdepth 1 -name 'vakt-backup-*.tar.gz' | head -1)"
	if [ -z "$ARCHIVE" ]; then
		fail "W1 — backup.sh meldete Erfolg, aber kein Archiv liegt vor"
	else
		# Der Dump muss echte Vakt-Tabellen enthalten, nicht nur existieren.
		printf '%s' "$TEST_PASSPHRASE" >"$WORK/pp"
		tar -xzf "$ARCHIVE" -C "$WORK" ./db.pgdump.gpg
		gpg --batch --yes --passphrase-file "$WORK/pp" --pinentry-mode loopback \
			--decrypt --output "$WORK/db.pgdump" "$WORK/db.pgdump.gpg" 2>/dev/null
		# TOC lesen: bevorzugt mit dem Host-Binary, sonst im DB-Container. Ein Runner
		# ohne postgresql-client darf diesen Fall nicht ROT machen — das waere ein
		# Fehlschlag ueber die Testumgebung, nicht ueber den Fix.
		toc_list() {
			if command -v pg_restore >/dev/null 2>&1; then
				pg_restore --list "$WORK/db.pgdump" 2>/dev/null
			else
				docker exec -i "$PG_NAME" pg_restore --list 2>/dev/null <"$WORK/db.pgdump"
			fi
		}
		if toc_list | grep -q 'ck_evidence'; then
			pass "W1 — Dump gegen internal:true erzeugt, ck_evidence im TOC"
		else
			fail "W1 — Archiv da, aber der Dump enthaelt ck_evidence nicht"
		fi
	fi
else
	fail "W1 — backup.sh scheitert gegen internal:true: $(tail -3 "$WORK/backup.out" | tr '\n' ' ')"
fi

# ── W12: MODE=container Erfolgspfad ───────────────────────────────────────────
# KONV-K1 F2, zweite Haelfte. W1 faehrt den Container-Weg nur ueber `auto`; der
# AUSDRUECKLICH gewaehlte container-Modus stand in keinem Fall. Zusammen mit W11
# ist damit beides betestet: der Modus tut, was er verspricht — und wenn er es
# nicht kann, bricht er ab, statt etwas anderes zu tun.
#
# Der pg_dump-Spion liegt im PATH und legt seine -f-Zieldatei LEER an. Ein
# Rueckfall auf den Host-Weg ist dadurch doppelt sichtbar: er stuende im
# Spion-Log, und backup.sh wuerde den 0-Byte-Dump ohnehin ablehnen. `pg_restore`
# wird bewusst NICHT geschattet — `vakt_pg_dump_readable` benutzt es legitim auf
# der lokalen Datei, das ist keine Datenbank-Verbindung.
W12_DIR="$WORK/w12"
W12_SPY="$WORK/w12-spy"
W12_LOG="$WORK/w12-spy.log"
mkdir -p "$W12_DIR" "$W12_SPY"
cp "$SPY_DIR/pg_dump" "$W12_SPY/pg_dump"
: >"$W12_LOG"
set +e
( cd "$W12_DIR" &&
	PATH="$W12_SPY:$PATH" \
	SPYLOG="$W12_LOG" \
	COMPOSE_PROJECT_NAME="$PROJECT" \
	VAKT_BACKUP_PG_MODE=container \
	VAKT_BACKUP_PG_CONTAINER="$PG_NAME" \
	VAKT_DB_URL="$DB_URL" \
	VAKT_SECRET_KEY="$TEST_SECRET_KEY" \
	VAKT_BACKUP_PASSPHRASE="$TEST_PASSPHRASE" \
	bash "$SCRIPT_DIR/backup.sh" "$W12_DIR" ) >"$WORK/w12.out" 2>&1
W12_RC=$?
set -e
W12_ARCHIVES="$(find "$W12_DIR" -maxdepth 1 -name 'vakt-backup-*.tar.gz' 2>/dev/null | wc -l)"
if [ "$W12_RC" -ne 0 ]; then
	fail "W12 — backup.sh mit MODE=container scheitert (rc=${W12_RC}): $(tail -3 "$WORK/w12.out" | tr '\n' ' ')"
elif [ -s "$W12_LOG" ]; then
	fail "W12 — MODE=container hat den HOST-pg_dump aufgerufen: $(tr '\n' ' ' <"$W12_LOG")"
elif [ "$W12_ARCHIVES" -eq 0 ]; then
	fail "W12 — rc=0, aber kein Archiv geschrieben"
elif ! grep -q "via container ${PG_NAME}" "$WORK/w12.out"; then
	fail "W12 — Archiv da, aber das Log nennt nicht den gewaehlten Container: $(grep -i 'Dumping' "$WORK/w12.out" | head -1)"
else
	pass "W12 — MODE=container dumpt im Container ${PG_NAME}, Log nennt ihn, Host-pg_dump nie aufgerufen"
fi

ARCHIVE="$(find "$WORK" -maxdepth 1 -name 'vakt-backup-*.tar.gz' | head -1)"

# ── W1b/W2/W3: Restore UEBER die bestehende DB ────────────────────────────────

if [ -z "${ARCHIVE:-}" ]; then
	skip "W1b — kein Archiv (W1 rot)"
	skip "W2 — kein Archiv (W1 rot)"
	skip "W3 — kein Archiv (W1 rot)"
else
	# Evidence-Datei vorher entfernen: nur ein echter Uploads-Restore bringt sie zurueck.
	docker run --rm -v "${UPLOADS_VOL}:/data" alpine:latest \
		sh -c 'rm -rf /data/evidence' >/dev/null
	# Eine DB-Zeile veraendern, damit der Restore etwas zu tun hat.
	docker exec -i "$PG_NAME" psql -q -U vakt -d vakt \
		-c "DELETE FROM ck_evidence WHERE note = 'evidence-row-2';"

	set +e
	printf 'y\n' | ( cd "$WORK" &&
		COMPOSE_PROJECT_NAME="$PROJECT" \
		VAKT_BACKUP_PG_CONTAINER="$PG_NAME" \
		VAKT_DB_URL="$DB_URL" \
		VAKT_SECRET_KEY="$TEST_SECRET_KEY" \
		VAKT_BACKUP_PASSPHRASE="$TEST_PASSPHRASE" \
		bash "$SCRIPT_DIR/restore.sh" "$ARCHIVE" ) >"$WORK/restore.out" 2>&1
	RESTORE_RC=$?
	set -e

	if [ "$RESTORE_RC" -eq 0 ]; then
		pass "W1b/W2 — restore.sh ueber bestehender DB endet mit rc=0"
	else
		fail "W1b/W2 — restore.sh ueber bestehender DB endet mit rc=${RESTORE_RC}: $(grep -iE 'error|inherited|could not' "$WORK/restore.out" | head -2 | tr '\n' ' ')"
	fi

	# Die DB muss wirklich wiederhergestellt sein (Zaehler aus der DB, nicht aus der Absicht).
	# `|| true`: fehlt die Tabelle (weil ein kaputter Restore sie nicht angelegt
	# hat), scheitert psql — und unter `set -o pipefail` riss das den ganzen
	# Testlauf mit, statt diesen Fall rot zu melden. Ein Test, der beim
	# Fehlerfall STIRBT, meldet nichts; er muss ihn benennen.
	ROWS="$( (docker exec "$PG_NAME" psql -tA -U vakt -d vakt -c 'SELECT count(*) FROM ck_evidence;' 2>/dev/null || true) | tr -d '[:space:]')"
	if [ "$ROWS" = "2" ]; then
		pass "W2b — DB-Daten wiederhergestellt (ck_evidence=2)"
	else
		fail "W2b — ck_evidence hat ${ROWS} Zeilen, erwartet 2"
	fi

	# W3: die Evidence-Datei MUSS zurueck sein, auch wenn der DB-Schritt gemeckert hat.
	GOT="$(docker run --rm -v "${UPLOADS_VOL}:/data:ro" alpine:latest \
		sh -c 'cat /data/evidence/proof.txt 2>/dev/null' || true)"
	if [ "$GOT" = "$EVIDENCE_MARKER" ]; then
		pass "W3 — Uploads-Volume wiederhergestellt, Evidence-Datei inhaltsgleich"
	else
		fail "W3 — Evidence-Datei fehlt oder weicht ab (gelesen: '${GOT:-<leer>}')"
	fi
fi

# ── W6: Uploads ueberleben auch einen FATALEN DB-Fehler ───────────────────────
# W3 allein wuerde auch gruen bleiben, wenn nur die Exit-Klassifizierung (Fix 3)
# existierte und die Uploads weiter HINTER dem DB-Schritt lieferten — solange der
# DB-Schritt nicht abbricht, merkt man die falsche Reihenfolge nicht. Hier bricht
# er absichtlich ab, und zwar mit einer Fehlerklasse, die NICHT harmlos ist:
# ein VIEW steht dort, wo der Dump eine TABLE hat (gemessen: 3 nicht-harmlose
# pg_restore-Fehler). Der Restore MUSS fehlschlagen — und die Evidence-Dateien
# muessen trotzdem da sein.
if [ -n "${ARCHIVE:-}" ]; then
	docker run --rm -v "${UPLOADS_VOL}:/data" alpine:latest sh -c 'rm -rf /data/evidence' >/dev/null
	docker exec -i "$PG_NAME" psql -q -U vakt -d vakt \
		-c "DROP TABLE IF EXISTS ck_evidence CASCADE; CREATE VIEW ck_evidence AS SELECT 1::int AS id, 'x'::text AS note;" >/dev/null 2>&1

	set +e
	printf 'y\n' | ( cd "$WORK" &&
		COMPOSE_PROJECT_NAME="$PROJECT" \
		VAKT_BACKUP_PG_CONTAINER="$PG_NAME" \
		VAKT_DB_URL="$DB_URL" \
		VAKT_SECRET_KEY="$TEST_SECRET_KEY" \
		VAKT_BACKUP_PASSPHRASE="$TEST_PASSPHRASE" \
		bash "$SCRIPT_DIR/restore.sh" "$ARCHIVE" ) >"$WORK/restore_fatal.out" 2>&1
	FATAL_RC=$?
	set -e

	GOT6="$(docker run --rm -v "${UPLOADS_VOL}:/data:ro" alpine:latest \
		sh -c 'cat /data/evidence/proof.txt 2>/dev/null' || true)"
	if [ "$FATAL_RC" -ne 0 ] && [ "$GOT6" = "$EVIDENCE_MARKER" ]; then
		pass "W6 — fataler DB-Fehler gemeldet (rc=${FATAL_RC}), Evidence-Dateien trotzdem wiederhergestellt"
	elif [ "$FATAL_RC" -eq 0 ]; then
		fail "W6 — fataler DB-Fehler wurde als Erfolg gemeldet (rc=0)"
	else
		fail "W6 — DB-Fehler gemeldet, aber Evidence-Dateien fehlen (gelesen: '${GOT6:-<leer>}')"
	fi

	# Aufraeumen: das VIEW wieder gegen die Tabelle tauschen, damit W4 nicht
	# ueber einen kaputten Zustand faellt (W4 dumpt nicht, aber sauber ist sauber).
	docker exec -i "$PG_NAME" psql -q -U vakt -d vakt \
		-c "DROP VIEW IF EXISTS ck_evidence;" >/dev/null 2>&1 || true
else
	skip "W6 — kein Archiv (W1 rot)"
fi

# ── W4: leerer Dump trotz Uploads-Volumen ─────────────────────────────────────
# Gemessen: 1 KB Evidence hebt ein Archiv mit 0-Byte-Dump auf ~1663 Bytes und
# damit ueber BEIDE Mindestgroessen-Schwellen (backup.sh 1000, backup-cron 1024).
# Nur eine Assertion auf den DUMP selbst faengt das.
#
# Der Leerdump wird ueber einen PATH-Schatten erzeugt, NICHT ueber einen Schalter
# im Produktionscode: `backup.sh` liest seine Umgebung aus `.env`, also waere ein
# `VAKT_BACKUP_FORCE_EMPTY_DUMP` eine Zeile, mit der sich in einer echten
# Installation ein leeres Backup bestellen liesse. Test-Geruest gehoert nicht in
# den Produktionspfad.
EMPTY_DIR="$WORK/empty"
SHIM_DIR="$WORK/shim"
mkdir -p "$EMPTY_DIR" "$SHIM_DIR"
cat >"$SHIM_DIR/pg_dump" <<'SHIM'
#!/usr/bin/env bash
# Schattet pg_dump: legt die -f-Zieldatei LEER an und meldet Erfolg — genau das
# Fehlerbild eines pg_dump, der nichts geschrieben hat, aber rc=0 liefert.
out=""
prev=""
for a in "$@"; do
	[ "$prev" = "-f" ] && out="$a"
	case "$a" in --file=*) out="${a#--file=}" ;; esac
	prev="$a"
done
[ -n "$out" ] && : >"$out"
exit 0
SHIM
chmod +x "$SHIM_DIR/pg_dump"
set +e
( cd "$EMPTY_DIR" &&
	PATH="$SHIM_DIR:$PATH" \
	COMPOSE_PROJECT_NAME="$PROJECT" \
	VAKT_BACKUP_PG_MODE=host \
	VAKT_DB_URL="$DB_URL" \
	VAKT_SECRET_KEY="$TEST_SECRET_KEY" \
	VAKT_BACKUP_PASSPHRASE="$TEST_PASSPHRASE" \
	bash "$SCRIPT_DIR/backup.sh" "$EMPTY_DIR" ) >"$WORK/empty.out" 2>&1
EMPTY_RC=$?
set -e
LEFTOVER="$(find "$EMPTY_DIR" -maxdepth 1 -name 'vakt-backup-*.tar.gz' | wc -l)"
if [ "$EMPTY_RC" -ne 0 ] && [ "$LEFTOVER" -eq 0 ]; then
	pass "W4 — leerer Dump abgelehnt (rc=${EMPTY_RC}), kein Archiv zurueckgelassen"
else
	fail "W4 — leerer Dump akzeptiert (rc=${EMPTY_RC}, ${LEFTOVER} Archiv(e) behalten)"
fi

# ── W7: fremdes Compose-Projekt darf nicht getroffen werden ───────────────────
# Ein zweiter Postgres mit `com.docker.compose.service=postgres`, aber einem
# ANDEREN Projekt-Label. Die Erkennung laeuft ohne VAKT_BACKUP_PG_CONTAINER, mit
# COMPOSE_PROJECT_NAME auf unser Projekt — und darf den fremden nicht liefern.
FOREIGN="${PROJECT}-foreign-postgres-1"
docker rm -fv "$FOREIGN" >/dev/null 2>&1 || true
docker run -d --name "$FOREIGN" \
	--label com.docker.compose.project="someone-elses-stack" \
	--label com.docker.compose.service=postgres \
	-e POSTGRES_USER=other -e POSTGRES_PASSWORD=other -e POSTGRES_DB=other \
	postgres:16-alpine >/dev/null 2>&1 || true

set +e
PICKED="$( cd "$WORK" && COMPOSE_PROJECT_NAME="$PROJECT" bash -c \
	"source '${SCRIPT_DIR}/backup-pg-target.sh'; _vakt_pg_container 2>/dev/null" )"
set -e
FOREIGN_ID="$(docker inspect -f '{{.Id}}' "$FOREIGN" 2>/dev/null || echo "none")"
OWN_ID="$(docker inspect -f '{{.Id}}' "$PG_NAME" 2>/dev/null || echo "none")"
# Positiv pruefen, nicht nur "nicht der fremde": ein leeres Ergebnis waere
# ebenfalls "nicht der fremde" und wuerde diesen Test gruen machen, obwohl die
# Erkennung dann gar nichts findet.
if [ -z "$PICKED" ]; then
	fail "W7 — Erkennung findet gar keinen Container (erwartet: den eigenen)"
elif [ "$FOREIGN_ID" != "none" ] && [ "${PICKED}" = "${FOREIGN_ID:0:${#PICKED}}" ]; then
	fail "W7 — Erkennung hat den Postgres eines FREMDEN Compose-Projekts gewaehlt"
elif [ "${PICKED}" = "${OWN_ID:0:${#PICKED}}" ]; then
	pass "W7 — Erkennung waehlt den eigenen Projekt-Postgres, nicht den fremden"
else
	fail "W7 — Erkennung waehlte '${PICKED}', weder eigener (${OWN_ID:0:12}) noch fremder (${FOREIGN_ID:0:12})"
fi
docker rm -fv "$FOREIGN" >/dev/null 2>&1 || true

# ── W10: Fehlschlag BEIM Signieren laesst nichts Halbes liegen ────────────────
#
# Die Archiv-Mindestgroesse prueft nur ein FERTIGES Archiv. Scheitert `openssl
# dgst`, blieb bisher ein Archiv ueber beiden Schwellen liegen (gemessen: 2880 B)
# plus eine 0-Byte-`.sig`. restore.sh lehnt das korrekt ab — aber die Datei sieht
# in einem Verzeichnislisting aus wie ein gueltiges Backup und zaehlt in jeder
# Pruefung mit, die nur Namen und Groesse ansieht.
W10_DIR="$WORK/w10"
W10_SHIM="$WORK/w10-shim"
mkdir -p "$W10_DIR" "$W10_SHIM"
cat >"$W10_SHIM/openssl" <<'SHIM'
#!/usr/bin/env bash
# Schattet openssl: `enc` (Schluessel-Verschluesselung) muss funktionieren, damit
# der Lauf ueberhaupt bis zum Signieren kommt; nur `dgst` scheitert.
if [ "${1:-}" = "dgst" ]; then
	echo "openssl: simulierter Fehler beim Signieren" >&2
	exit 1
fi
exec /usr/bin/openssl "$@"
SHIM
chmod +x "$W10_SHIM/openssl"
set +e
( cd "$W10_DIR" &&
	PATH="$W10_SHIM:$PATH" \
	COMPOSE_PROJECT_NAME="$PROJECT" \
	VAKT_BACKUP_PG_CONTAINER="$PG_NAME" \
	VAKT_DB_URL="$DB_URL" \
	VAKT_SECRET_KEY="$TEST_SECRET_KEY" \
	VAKT_BACKUP_PASSPHRASE="$TEST_PASSPHRASE" \
	bash "$SCRIPT_DIR/backup.sh" "$W10_DIR" ) >"$WORK/w10.out" 2>&1
W10_RC=$?
set -e
W10_ARCHIVES="$(find "$W10_DIR" -maxdepth 1 -name 'vakt-backup-*.tar.gz' 2>/dev/null | wc -l)"
W10_SIGS="$(find "$W10_DIR" -maxdepth 1 -name 'vakt-backup-*.tar.gz.sig' 2>/dev/null | wc -l)"
if [ "$W10_RC" -eq 0 ]; then
	fail "W10 — Signaturfehler wurde als Erfolg gemeldet (rc=0)"
elif [ "$W10_ARCHIVES" -ne 0 ] || [ "$W10_SIGS" -ne 0 ]; then
	fail "W10 — nach Signaturfehler liegen ${W10_ARCHIVES} Archiv(e) und ${W10_SIGS} Signatur(en) herum"
else
	pass "W10 — Signaturfehler gemeldet (rc=${W10_RC}), weder Archiv noch 0-Byte-.sig zurueckgelassen"
fi

# ── W9: Restore auf einem Host OHNE Docker-CLI ────────────────────────────────
#
# Die Topologie, fuer die VAKT_BACKUP_PG_MODE=host gebaut ist (externes/managed
# Postgres, kein Docker auf der Maschine). Nachdem der Uploads-Schritt VOR den
# DB-Restore gezogen wurde, starb restore.sh dort an `docker volume inspect`
# (rc=127, `set -e`) — und zwar BEVOR die Datenbank zurueckkam. Vorher fehlten nur
# die Uploads, danach fehlte beides: ein Fix darf keinen schlechteren Zustand
# erzeugen als der Defekt. Der Uploads-Schritt ist deshalb best-effort.
#
# Nachgestellt wird das nicht durch PATH-Basteln, sondern echt: der Restore laeuft
# IN einem Container, der schlicht keine Docker-CLI hat.
if [ -z "${ARCHIVE:-}" ]; then
	skip "W9 (Restore ohne Docker-CLI) — kein Archiv (W1 rot)"
else
	W9_NAME="${PROJECT}-nodocker"
	docker rm -fv "$W9_NAME" >/dev/null 2>&1 || true
	# Jeder Aufbauschritt wird einzeln geprueft. Stirbt ein Test-Setup unter
	# `set -e` ohne Meldung, sieht das wie "Fall nicht vorhanden" aus statt wie
	# "Fall nicht geprueft" — dieselbe Klasse wie ein stiller Skip, und genau das
	# ist hier zuerst passiert.
	W9_SETUP=""
	# Erst am Standard-Bridge-Netz starten (braucht Ausgang fuer apk), …
	set +e
	docker run -d --name "$W9_NAME" \
		-v "$SCRIPT_DIR":/scripts:ro \
		-v "$WORK":/archive \
		postgres:16-alpine sleep infinity >"$WORK/w9-run.log" 2>&1 ||
		W9_SETUP="docker run: $(tail -1 "$WORK/w9-run.log")"
	if [ -z "$W9_SETUP" ]; then
		docker exec "$W9_NAME" apk add --no-cache bash gnupg openssl >"$WORK/w9-apk.log" 2>&1 ||
			W9_SETUP="apk add: $(tail -1 "$WORK/w9-apk.log")"
	fi
	# … dann zusaetzlich ins interne DB-Netz haengen.
	if [ -z "$W9_SETUP" ]; then
		docker network connect "$NET_NAME" "$W9_NAME" >"$WORK/w9-net.log" 2>&1 ||
			W9_SETUP="network connect: $(tail -1 "$WORK/w9-net.log")"
	fi
	set -e
	if [ -n "$W9_SETUP" ]; then
		fail "W9 — Testaufbau fehlgeschlagen (${W9_SETUP})"
		docker rm -fv "$W9_NAME" >/dev/null 2>&1 || true
		W9_SKIP_BODY=1
	fi

	HAS_DOCKER="$(docker exec "$W9_NAME" sh -c 'command -v docker >/dev/null 2>&1 && echo ja || echo nein' 2>/dev/null || echo unbekannt)"
	# Ausgangszustand: eine Zeile loeschen, damit der Restore etwas zu tun hat.
	# BEST-EFFORT: W6 laeuft vorher und laesst `ck_evidence` bewusst geloescht
	# zurueck (dort wird die Tabelle durch ein VIEW ersetzt und danach verworfen).
	# Ohne `|| true` scheiterte dieses DELETE und `set -e` beendete den Test-Lauf
	# LAUTLOS mitten in W9 — es sah aus wie "W9 existiert nicht" statt
	# "W9 nicht geprueft". Der Restore muss die Tabelle ohnehin neu anlegen; genau
	# das ist die Aussage dieses Falls.
	docker exec -i "$PG_NAME" psql -q -U vakt -d vakt \
		-c "DELETE FROM ck_evidence WHERE note = 'evidence-row-2';" >/dev/null 2>&1 || true

	W9_ARCHIVE="/archive/$(basename "$ARCHIVE")"
	set +e
	docker exec -i \
		-e VAKT_BACKUP_PG_MODE=host \
		-e VAKT_DB_URL="postgres://vakt:vakt@${PG_NAME}:5432/vakt?sslmode=disable" \
		-e VAKT_SECRET_KEY="$TEST_SECRET_KEY" \
		-e VAKT_BACKUP_PASSPHRASE="$TEST_PASSPHRASE" \
		"$W9_NAME" sh -c "printf 'y\n' | bash /scripts/restore.sh '$W9_ARCHIVE'" \
		>"$WORK/w9.out" 2>&1
	W9_RC=$?
	set -e
	# Siehe W2b: tolerant lesen, damit ein kaputter Restore als FAIL erscheint und
	# nicht als abgebrochener Testlauf.
	W9_ROWS="$( (docker exec "$PG_NAME" psql -tA -U vakt -d vakt -c 'SELECT count(*) FROM ck_evidence;' 2>/dev/null || true) | tr -d '[:space:]')"

	if [ -n "${W9_SKIP_BODY:-}" ]; then
		: # Aufbau schon als FAIL gemeldet, keine zweite Meldung
	elif [ "$HAS_DOCKER" != "nein" ]; then
		fail "W9 — Testaufbau falsch: der Container HAT eine Docker-CLI, der Fall wird nicht geprueft"
	elif [ "$W9_RC" -ne 0 ]; then
		fail "W9 — restore.sh ohne Docker-CLI endet mit rc=${W9_RC}: $(grep -iE 'error|not found' "$WORK/w9.out" | head -2 | tr '\n' ' ')"
	elif [ "$W9_ROWS" != "2" ]; then
		fail "W9 — rc=0, aber die Datenbank ist nicht wiederhergestellt (ck_evidence=${W9_ROWS}, erwartet 2)"
	elif ! grep -q 'keine Docker-CLI' "$WORK/w9.out"; then
		fail "W9 — DB wiederhergestellt, aber der fehlende Uploads-Restore wurde NICHT gemeldet (stiller Teilerfolg)"
	else
		pass "W9 — ohne Docker-CLI: DB wiederhergestellt (ck_evidence=2), fehlende Uploads ausdruecklich gemeldet"
	fi
	# `docker rm -f` loest die Netzanbindungen mit; ein separates `network
	# disconnect` waere toter Code und wuerde nur suggerieren, es brauche ihn.
	docker rm -fv "$W9_NAME" >/dev/null 2>&1 || true
fi

# ── W8: der Opt-in-Scheduler STARTET und schreibt wirklich ein Archiv ─────────
#
# W5a prueft nur YAML. Ein statischer Parse kann nicht sehen, dass der Container
# in einer Crash-Loop haengt — die erste Fassung dieses Fixes hing `backup` an
# zwei `internal: true`-Netze, `apk add` scheiterte, `set -e` beendete den
# Entrypoint und `restart: unless-stopped` startete endlos neu. W5a war dabei
# gruen. Deshalb hier eine echte Abnahme: `docker compose up`, RestartCount==0,
# und ein Archiv in /backups nach einem echten crond-Tick.
#
# WICHTIG — dieser Fall darf sich NICHT selbst ueberspringen. Die erste Fassung
# hatte `if [ -f "$REPO_ROOT/.env" ]; then skip …`, weil `env_file: .env` im
# Compose eine Datei am Projektverzeichnis verlangt. Jeder echte Arbeitsbaum HAT
# aber eine `.env` — der Guard fuer genau den Blocker, der diese Runde ausgeloest
# hat, waere fuer jeden Menschen, der `make test` faehrt, nie gelaufen. Der Skip
# war gezaehlt und damit sichtbar, aber die Bedingung war falsch gewaehlt.
#
# Loesung: ein eigenes Projektverzeichnis. `--project-directory` verschiebt die
# Auflösung ALLER relativen Pfade dorthin — `env_file: .env` und `./scripts`
# inklusive. Gemessen: `docker compose config` loest `./scripts` auf den Symlink
# und `env_file` auf die temporaere Datei auf. Das Repo-Root-`.env` wird nie
# gelesen, nie geschrieben, nie geloescht.
W8_PROJECT="esk7boot$$"
W8_PROJDIR="$WORK/w8-project"
W8_BACKUP_DIR="$W8_PROJDIR/backups"
mkdir -p "$W8_BACKUP_DIR"
ln -s "$SCRIPT_DIR" "$W8_PROJDIR/scripts"
# Platzhalter-Werte, keine echten Geheimnisse — und in einem mktemp-Verzeichnis,
# nicht im Repo.
{
	printf 'POSTGRES_PASSWORD=%s\n' "esk7bootpw"
	printf 'REDIS_PASSWORD=%s\n' "esk7bootpw"
	printf 'VAKT_SECRET_KEY=%s\n' "$TEST_SECRET_KEY"
	printf 'VAKT_BACKUP_PASSPHRASE=%s\n' "$TEST_PASSPHRASE"
	printf 'VAKT_DOMAIN=%s\n' "example.test"
	printf 'VAKT_BACKUP_SCHEDULE=%s\n' "* * * * *"
	printf 'VAKT_BACKUP_DIR=%s\n' "$W8_BACKUP_DIR"
} >"$W8_PROJDIR/.env"

w8_compose() { docker compose -p "$W8_PROJECT" --project-directory "$W8_PROJDIR" \
	-f "$REPO_ROOT/docker-compose.yml" -f "$REPO_ROOT/docker-compose.backup.yml" \
	--profile backup "$@"; }

if true; then
	set +e
	w8_compose up -d postgres >"$WORK/w8-up.log" 2>&1
	W8_UP_RC=$?
	set -e
	if [ "$W8_UP_RC" -eq 0 ]; then
		# Die DB MUSS Inhalt haben, bevor der Scheduler dumpt. Eine nackte, frisch
		# initialisierte Postgres-Instanz erzeugt einen ~800-Byte-Dump und faellt
		# damit korrekt durch die Archiv-Mindestgroesse — das waere ein Fehlschlag
		# ueber das Test-Fixture, nicht ueber den Fix (genau so zuerst passiert).
		W8_PG="$(w8_compose ps -q postgres 2>/dev/null | head -1)"
		for _ in $(seq 1 60); do
			docker exec "$W8_PG" pg_isready -U vakt >/dev/null 2>&1 && break
			sleep 1
		done
		# `|| true`: wird W8_PG leer oder Postgres nicht rechtzeitig gesund, darf das
		# den Lauf NICHT unter `set -e` beenden — sonst bliebe ein kompletter
		# Compose-Stack samt Volumes stehen, und der Fall waere nicht als FAIL
		# gemeldet, sondern gar nicht.
		docker exec -i "$W8_PG" psql -q -U vakt -d vakt \
			-c "CREATE TABLE ck_evidence (id serial PRIMARY KEY, note TEXT NOT NULL);" \
			-c "INSERT INTO ck_evidence (note) SELECT 'row-'||g FROM generate_series(1,500) g;" \
			>/dev/null 2>&1 || true
		set +e
		w8_compose up -d backup >>"$WORK/w8-up.log" 2>&1
		W8_UP_RC=$?
		set -e
	fi

	if [ "$W8_UP_RC" -ne 0 ]; then
		fail "W8 — 'compose up' schlug fehl: $(tail -3 "$WORK/w8-up.log" | tr '\n' ' ')"
	else
		BK_ID="$(w8_compose ps -q backup 2>/dev/null | head -1)"
		# 90 s Fenster: der Container muss laufen UND ein Archiv erscheinen
		# (Schedule steht auf '* * * * *', also spaetestens nach ~60 s).
		ARCHIVE_COUNT=0
		for _ in $(seq 1 90); do
			ARCHIVE_COUNT="$(find "$W8_BACKUP_DIR" -maxdepth 1 -name 'vakt-backup-*.tar.gz' 2>/dev/null | wc -l)"
			[ "$ARCHIVE_COUNT" -gt 0 ] && break
			sleep 1
		done
		# Ein Archiv auf der Platte ist NICHT dasselbe wie ein durchgelaufener
		# Zyklus. Genau daran ist die erste W8-Fassung vorbeigelaufen: das Archiv
		# lag da, aber `backup-cron.sh` starb danach an `find -printf` (busybox) und
		# meldete "no archive found after backup" — Verify, Off-site und Retention
		# liefen nie. Deshalb wird der Abschluss-Marker des Zyklus eingefordert.
		CYCLE_DONE=0
		for _ in $(seq 1 45); do
			if docker exec "$BK_ID" grep -q 'cycle complete' /backups/backup-cron.log 2>/dev/null; then
				CYCLE_DONE=1
				break
			fi
			sleep 1
		done
		RESTARTS="$(docker inspect -f '{{.RestartCount}}' "$BK_ID" 2>/dev/null || echo "?")"
		RUNNING="$(docker inspect -f '{{.State.Running}}' "$BK_ID" 2>/dev/null || echo "?")"

		if [ "$RESTARTS" != "0" ]; then
			fail "W8 — Scheduler-Container in Crash-Loop (RestartCount=${RESTARTS}): $(docker logs "$BK_ID" 2>&1 | tail -3 | tr '\n' ' ')"
		elif [ "$RUNNING" != "true" ]; then
			fail "W8 — Scheduler-Container laeuft nicht (Running=${RUNNING})"
		elif [ "$ARCHIVE_COUNT" -eq 0 ]; then
			fail "W8 — Container laeuft (RestartCount=0), aber KEIN Archiv in /backups: $(docker exec "$BK_ID" tail -5 /backups/backup-cron.log 2>/dev/null | tr '\n' ' ')"
		elif [ "$CYCLE_DONE" -ne 1 ]; then
			fail "W8 — Archiv geschrieben, aber der Zyklus lief nicht durch (kein 'cycle complete'): $(docker exec "$BK_ID" tail -4 /backups/backup-cron.log 2>/dev/null | tr '\n' ' ')"
		else
			pass "W8 — Scheduler startet (RestartCount=0), schrieb ${ARCHIVE_COUNT} Archiv(e) und der Zyklus lief nach einem echten crond-Tick vollstaendig durch"
		fi
	fi
fi
# Abgeraeumt wird im globalen Trap (siehe cleanup()) — nicht hier. Ein Aufraeumen,
# das nur auf den erwarteten Pfaden steht, laesst bei jedem unerwarteten Abbruch
# einen ganzen Compose-Stack samt Volumes liegen.

echo
echo "ERGEBNIS: passed=$PASSED failed=$FAILED skipped=$SKIPPED"
[ "$FAILED" -eq 0 ] || exit 1
echo "ALL PASS"
