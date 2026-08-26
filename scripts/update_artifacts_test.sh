#!/usr/bin/env bash
set -uo pipefail

# R1-06-D00 / R1-06-D01 (Codeaudit-v5b, Welle 1 Spur C) — Regressionstest fuer
# scripts/update.sh + scripts/update-artifacts.sh.
#
# Der Defekt, den dieser Test festhaelt: `scripts/update.sh` zog neue Images und
# meldete "Update complete!", ohne die AUSLIEFERUNGS-ARTEFAKTE der Installation
# anzufassen (docker-compose.yml, Caddyfile, scripts/). Gemessen an einer echten
# Installation auf v0.42.48: exit 0, und docker-compose.yml war danach byte-gleich
# mit dem alten Stand — die 112 Zeilen Docker-Secrets-Haertung aus v0.42.49
# erreichten den Bestandskunden nie.
#
# Der Test baut eine vollstaendige, kuenstliche Kunden-Installation (eigenes
# Upstream-Repo, eigener Container-Stellvertreter) und braucht deshalb WEDER
# Docker-Daemon NOCH Netz NOCH Datenbank.

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
UPDATE_SH="${SCRIPT_DIR}/update.sh"
ARTIFACTS_SH="${SCRIPT_DIR}/update-artifacts.sh"

passed=0
failed=0
skipped=0
fail() {
	echo "FAIL: $1" >&2
	[ -n "${2:-}" ] && sed 's/^/      | /' "$2" >&2
	failed=$((failed + 1))
}
pass() {
	echo "PASS: $1"
	passed=$((passed + 1))
}

WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT

# ── Container-Stellvertreter ─────────────────────────────────────────────────
# Beantwortet genau die Aufrufe, die update.sh macht, und protokolliert sie.
# Er ersetzt die Container-Laufzeit, nicht das zu pruefende Skript.
BIN="$WORK/bin"
mkdir -p "$BIN"
cat >"$BIN/docker" <<'SHIM'
#!/usr/bin/env bash
echo "docker $*" >>"${DOCKER_LOG:?DOCKER_LOG nicht gesetzt}"
if [ "${1:-}" = "compose" ]; then
	shift
	args=()
	while [ $# -gt 0 ]; do
		case "$1" in
		-f) shift 2 ;;
		*)
			args+=("$1")
			shift
			;;
		esac
	done
	# DOCKER_FAIL_ON=<unterbefehl> laesst genau diesen Aufruf scheitern. Gebraucht
	# fuer Fall 8b: ein Abbruch NACH der Artefakt-Umstellung und VOR `up -d`.
	if [ -n "${DOCKER_FAIL_ON:-}" ] && [ "${args[0]:-}" = "$DOCKER_FAIL_ON" ]; then
		echo "compose ${args[*]}: FEHLER (Stellvertreter, DOCKER_FAIL_ON)" >&2
		exit 1
	fi
	case "${args[0]:-}" in
	version) echo "Docker Compose version v2.99.0" ;;
	images) echo '[{"Repository":"ghcr.io/norvik-ops/vakt-api","Tag":"v0.0.1-alt"}]' ;;
	*) echo "compose ${args[*]}: ok" ;;
	esac
fi
exit 0
SHIM
chmod +x "$BIN/docker"
echo ok >"$WORK/health.txt"
export PATH="$BIN:$PATH"
export VAKT_HEALTH_URL="file://$WORK/health.txt"

G() { git -c user.email=t@vakt.test -c user.name=Test "$@"; }

# ── Kunden-Installation bauen ────────────────────────────────────────────────
# $1 = Name. Legt an:
#   <name>/upstream.git  bare Repo = "die neue Version"
#   <name>/install       der Checkout des Kunden (steht auf dem ALTEN Commit)
# Der Kunde steht auf einem Commit mit Marker ALT; upstream traegt zusaetzlich
# einen Commit, der docker-compose.yml auf NEU hebt.
# $2 (optional) = Koerper fuer scripts/backup.sh. Er wird VOR dem ersten Commit
# gesetzt, damit die Installation sauber bleibt und vorspulbar ist.
make_install() {
	local root="$WORK/$1" backup_body="${2:-echo \"stub backup\"}"
	mkdir -p "$root/src"
	(
		cd "$root/src" || exit 1
		G init -q -b main .
		mkdir -p scripts
		printf 'services:\n  api:\n    image: alt\n# MARKER: ALT\n' >docker-compose.yml
		printf ':80 {\n\treverse_proxy api:8080\n}\n# MARKER: ALT\n' >Caddyfile
		printf 'VAKT_TAG=latest\n' >.env.example
		cp "$UPDATE_SH" scripts/update.sh
		[ -f "$ARTIFACTS_SH" ] && cp "$ARTIFACTS_SH" scripts/update-artifacts.sh
		printf '#!/usr/bin/env bash\nset -e\n%s\n' "$backup_body" >scripts/backup.sh
		chmod +x scripts/*.sh
		printf '.env\nbackups/\n' >.gitignore
		G add -A && G commit -qm "alter Stand"
	)
	G clone -q --bare "$root/src" "$root/upstream.git"
	# Der "neue Stand" landet direkt im Upstream.
	(
		cd "$root/src" || exit 1
		G remote add up "$root/upstream.git" 2>/dev/null || true
	)
	G clone -q "$root/upstream.git" "$root/install"
	(
		cd "$root/install" || exit 1
		printf 'VAKT_SECRET_KEY=x\nVAKT_DB_URL=postgres://x\n' >.env
	)
	# Upstream einen Schritt weiter bringen (= die neue Version).
	local newsrc="$root/new"
	G clone -q "$root/upstream.git" "$newsrc"
	(
		cd "$newsrc" || exit 1
		sed -i 's/# MARKER: ALT/# MARKER: NEU/' docker-compose.yml Caddyfile
		printf 'VAKT_TAG=latest\nVAKT_NEUE_PFLICHTVARIABLE=bitte-setzen\n' >.env.example
		G commit -qam "neuer Stand" && G push -q origin main
	)
}

run_update() {
	# $1 = Installationsverzeichnis, Rest = Argumente fuer update.sh.
	# Exit-Code OHNE Pipe messen; Ausgabe in $OUT.
	#
	# REV-W1C F4: stdin kommt ausdruecklich aus /dev/null. Fall 5 haengt an
	# `[ ! -t 0 ]` in update.sh und setzte bis 2026-08-02 stillschweigend
	# voraus, dass der Testlauf kein Terminal hat. Das stimmte fuer CI und den
	# pre-push-Hook und nicht fuer die Entwickler-Shell: gemessen war
	# `script -qec 'bash scripts/update_artifacts_test.sh' /dev/null` rot
	# (passed=10 failed=1), derselbe Lauf ohne tty gruen. Ein Test, dessen
	# Aussage vom Aufrufweg abhaengt, misst den Aufrufweg mit. Jetzt hat der
	# Lauf nie ein Terminal, egal wie die Suite gestartet wird.
	local dir="$1"
	shift
	OUT="$WORK/out.$$.txt"
	(
		cd "$dir" || exit 1
		DOCKER_LOG="$dir/docker.log" bash ./scripts/update.sh "$@"
	) >"$OUT" 2>&1 </dev/null
	RC=$?
}

# ── 1: der Kern von R1-06-D00 ────────────────────────────────────────────────
# Nach einem Update stehen die Auslieferungs-Artefakte auf dem neuen Stand.
make_install t1
run_update "$WORK/t1/install" --no-backup
if [ "$RC" -ne 0 ]; then
	fail "1: update.sh endete mit rc=$RC" "$OUT"
elif grep -q "MARKER: NEU" "$WORK/t1/install/docker-compose.yml" &&
	grep -q "MARKER: NEU" "$WORK/t1/install/Caddyfile"; then
	pass "1: Update hebt docker-compose.yml UND Caddyfile auf den neuen Stand"
else
	fail "1: Artefakte stehen nach dem Update noch auf dem alten Stand" "$OUT"
fi

# ── 2: geaenderte Dienste werden neu erzeugt ─────────────────────────────────
# Zweite Haelfte desselben Defekts: eine neue docker-compose.yml wirkt nur, wenn
# die Dienste mit geaenderter Konfiguration auch neu erzeugt werden. Das alte
# `up -d --no-deps api worker frontend vakt-scanners` liess caddy, postgres,
# redis und pgbouncer unberuehrt — genau die Dienste, deren Haertung in den
# 112 Compose-Zeilen steckte.
if grep -qE '^docker compose .* up -d' "$WORK/t1/install/docker.log" &&
	! grep -qE '^docker compose .* up -d.*--no-deps' "$WORK/t1/install/docker.log"; then
	pass "2: Dienste werden ohne --no-deps neu erzeugt (Compose-Aenderungen greifen)"
else
	fail "2: der Neustart laesst Dienste mit geaenderter Konfiguration stehen" "$WORK/t1/install/docker.log"
fi

# ── 3: lokale Aenderungen werden nicht ueberfahren ───────────────────────────
make_install t3
echo "# lokale Anpassung des Kunden" >>"$WORK/t3/install/Caddyfile"
run_update "$WORK/t3/install" --no-backup
if [ "$RC" -eq 0 ]; then
	fail "3: update.sh lief trotz lokal geaenderter Caddyfile durch (rc=0)" "$OUT"
elif ! grep -q "lokale Anpassung des Kunden" "$WORK/t3/install/Caddyfile"; then
	fail "3: die lokale Aenderung wurde ueberschrieben" "$OUT"
elif ! grep -q "Caddyfile" "$OUT"; then
	fail "3: Abbruchmeldung nennt die betroffene Datei nicht" "$OUT"
elif grep -qE '^docker compose .* pull' "$WORK/t3/install/docker.log" 2>/dev/null; then
	fail "3: es wurde schon gezogen, bevor der Abbruch kam" "$WORK/t3/install/docker.log"
else
	pass "3: lokale Aenderung stoppt das Update, bevor irgendetwas passiert"
fi

# ── 3b: lokale Aenderung an einer Datei, die der neue Stand NICHT anfasst ────
# Der Fall, den `git merge --ff-only` NICHT abfaengt: der Merge geht glatt durch
# und laesst die lokale Aenderung obendrauf stehen. Die Installation ist danach
# eine Mischung aus zwei Staenden, und niemand hat es gesagt. Genau hier greift
# die eigene Dirty-Pruefung — 3 oben wuerde auch ohne sie gruen bleiben, weil
# git dort von sich aus abbricht.
make_install t3b
echo "# eigene Anpassung" >>"$WORK/t3b/install/scripts/backup.sh"
run_update "$WORK/t3b/install" --no-backup
if [ "$RC" -eq 0 ]; then
	fail "3b: lokale Aenderung an einer vom Update nicht beruehrten Datei blieb unbemerkt" "$OUT"
elif ! grep -q "scripts/backup.sh" "$OUT"; then
	fail "3b: Abbruchmeldung nennt die betroffene Datei nicht" "$OUT"
elif ! grep -q "eigene Anpassung" "$WORK/t3b/install/scripts/backup.sh"; then
	fail "3b: die lokale Aenderung wurde ueberschrieben" "$OUT"
else
	pass "3b: lokale Aenderung an einer nicht beruehrten Datei stoppt das Update ebenfalls"
fi

# ── 4: Installation ohne git — Abbruch, und mit --skip-artifacts ehrlich ─────
make_install t4
rm -rf "$WORK/t4/install/.git"
run_update "$WORK/t4/install" --no-backup
if [ "$RC" -eq 0 ]; then
	fail "4a: kein git-Checkout, trotzdem rc=0" "$OUT"
else
	pass "4a: Installation ohne git-Checkout bricht ab statt Erfolg zu melden"
fi
run_update "$WORK/t4/install" --no-backup --skip-artifacts
if [ "$RC" -ne 0 ]; then
	fail "4b: --skip-artifacts kam nicht durch (rc=$RC)" "$OUT"
elif grep -qi "NICHT aktualisiert" "$OUT"; then
	pass "4b: --skip-artifacts laeuft durch und weist die Luecke im Abschluss aus"
else
	fail "4b: Abschlussmeldung verschweigt, dass die Artefakte alt geblieben sind" "$OUT"
fi

# ── 5: R1-06-D01 — fehlende Passphrase stoppt VOR jeder Aenderung ────────────
# Ohne Passphrase und ohne Terminal bricht backup.sh ab (gemessen: rc=1). Das
# darf nicht erst passieren, nachdem die Artefakte schon umgestellt sind.
make_install t5
run_update "$WORK/t5/install"
if [ "$RC" -eq 0 ]; then
	fail "5: update.sh lief ohne konfigurierbare Backup-Passphrase durch" "$OUT"
elif grep -q "MARKER: NEU" "$WORK/t5/install/docker-compose.yml"; then
	fail "5: die Artefakte wurden umgestellt, obwohl das Backup nicht laufen kann" "$OUT"
elif [ -s "$WORK/t5/install/docker.log" ] && grep -qE 'pull|up -d' "$WORK/t5/install/docker.log"; then
	fail "5: die Container-Laufzeit wurde vor dem Abbruch schon angefasst" "$WORK/t5/install/docker.log"
elif grep -q "VAKT_BACKUP_PASSPHRASE" "$OUT"; then
	pass "5: fehlende Backup-Passphrase stoppt vor der ersten Aenderung und nennt die Variable"
else
	fail "5: Abbruchmeldung nennt die fehlende Variable nicht" "$OUT"
fi

# ── 6: R1-06-D01 — das Backup wird zugesichert, nicht geglaubt ───────────────
# Ein Erzeuger, der Erfolg meldet und nichts hinterlaesst, ist die Klasse des
# Vorfalls vom 2026-07-10. Ein Exit-Code allein darf ein Backup nicht abnehmen.
make_install t6a 'echo "✓ Backup saved"; exit 0'
VAKT_BACKUP_PASSPHRASE=eine-lange-passphrase run_update "$WORK/t6a/install"
if [ "$RC" -eq 0 ]; then
	fail "6a: Backup ohne Archiv wurde als Erfolg verbucht" "$OUT"
else
	pass "6a: Backup, das Erfolg meldet aber nichts hinterlaesst, stoppt das Update"
fi

make_install t6b 'mkdir -p "$1"; printf "zu klein" > "$1/vakt-backup-2026-01-01_00-00-00.tar.gz"; exit 0'
VAKT_BACKUP_PASSPHRASE=eine-lange-passphrase run_update "$WORK/t6b/install"
if [ "$RC" -eq 0 ]; then
	fail "6b: 8-Byte-Archiv wurde als Backup abgenommen" "$OUT"
else
	pass "6b: zu kleines Archiv stoppt das Update"
fi

make_install t6c '
mkdir -p "$1"; out="$(cd "$1" && pwd)"
mkdir -p /tmp/vakt-t6c && cd /tmp/vakt-t6c
head -c 4096 /dev/urandom > db.pgdump.gpg
printf "{\"uploads\":\"included\"}" > manifest.json
head -c 64 /dev/urandom > secret.key.enc
tar czf "$out/vakt-backup-2026-01-01_00-00-00.tar.gz" db.pgdump.gpg manifest.json secret.key.enc
printf "abc123" > "$out/vakt-backup-2026-01-01_00-00-00.tar.gz.sig"
exit 0'
VAKT_BACKUP_PASSPHRASE=eine-lange-passphrase run_update "$WORK/t6c/install"
if [ "$RC" -ne 0 ]; then
	fail "6c: gueltiges Backup wurde abgelehnt (rc=$RC)" "$OUT"
elif grep -q "vakt-backup-2026-01-01_00-00-00.tar.gz" "$OUT" && grep -qE "[0-9]{3,} Bytes" "$OUT"; then
	pass "6c: gueltiges Backup wird abgenommen, Pfad und Groesse werden genannt"
else
	fail "6c: das abgenommene Backup wird nicht mit Pfad und Groesse ausgewiesen" "$OUT"
fi
rm -rf /tmp/vakt-t6c

# ── 8: REV-W1C F3 — ein gescheitertes Backup laesst die Installation UNBERUEHRT
# Der Fall, den der Vorabtest aus Step 0 nicht sieht: die Passphrase ist da, das
# Backup scheitert trotzdem — Datenbank nicht erreichbar, Platte voll, gpg
# fehlt. Bis 2026-08-02 lief die Artefakt-Umstellung VOR dem Backup; danach
# stand die Installation gemischt da (neue Compose-Datei, alte Container, keine
# Sicherung), und der Betreiber las als letzte Zeile den pg_dump-Fehler.
# Gemessen wird deshalb der ZUSTAND, nicht die Meldung.
make_install t8 'echo "pg_dump: error: could not translate host name \"postgres\"" >&2; exit 1'
VAKT_BACKUP_PASSPHRASE=eine-lange-passphrase run_update "$WORK/t8/install"
if [ "$RC" -eq 0 ]; then
	fail "8: gescheitertes Backup wurde als Erfolg verbucht" "$OUT"
elif grep -q "MARKER: NEU" "$WORK/t8/install/docker-compose.yml" ||
	grep -q "MARKER: NEU" "$WORK/t8/install/Caddyfile"; then
	fail "8: die Auslieferungs-Dateien wurden umgestellt, obwohl das Backup scheiterte" "$OUT"
elif [ -s "$WORK/t8/install/docker.log" ] && grep -qE 'pull|up -d' "$WORK/t8/install/docker.log"; then
	fail "8: die Container-Laufzeit wurde vor dem Abbruch schon angefasst" "$WORK/t8/install/docker.log"
else
	pass "8: gescheitertes Backup stoppt VOR der Artefakt-Umstellung — kein gemischter Stand"
fi

# ── 8b: und wenn es doch mittendrin klemmt, wird der gemischte Stand GENANNT ─
# Die zweite Haelfte von F3: nach der Umstellung kann immer noch etwas
# scheitern. Dann darf der Abbruch nicht schweigend sein. Hier scheitert der
# Image-Pull (der docker-Stellvertreter faellt fuer `pull` auf rc=1), also nach
# Step 2 und vor `up -d`.
make_install t8b
VAKT_BACKUP_PASSPHRASE=eine-lange-passphrase \
	DOCKER_FAIL_ON=pull run_update "$WORK/t8b/install" --no-backup
if [ "$RC" -eq 0 ]; then
	fail "8b: der Lauf meldete Erfolg, obwohl der Pull scheiterte" "$OUT"
elif ! grep -q "GEMISCHTER STAND" "$OUT"; then
	fail "8b: Abbruch nach der Umstellung, ohne den gemischten Stand zu benennen" "$OUT"
elif ! grep -q "Zuruecknehmen" "$OUT"; then
	fail "8b: der gemischte Stand wird benannt, aber kein Rueckweg genannt" "$OUT"
else
	pass "8b: Abbruch NACH der Umstellung benennt den gemischten Stand und den Rueckweg"
fi

# ── 7: neue Pflichtvariable in .env.example wird gemeldet ────────────────────
# Die .env des Kunden wird von keinem Update angefasst (sie ist git-ignoriert).
# Eine neue Pflichtvariable erreicht ihn deshalb nur, wenn das Update sie nennt.
if grep -q "VAKT_NEUE_PFLICHTVARIABLE" "$WORK/t1/install/../install/.env" 2>/dev/null; then
	fail "7: die .env des Kunden wurde veraendert — das darf nicht passieren"
else
	OUT1="$WORK/t1-out.txt"
	make_install t7
	run_update "$WORK/t7/install" --no-backup
	cp "$OUT" "$OUT1"
	if grep -q "VAKT_NEUE_PFLICHTVARIABLE" "$OUT1"; then
		pass "7: neue Variable aus .env.example wird namentlich gemeldet"
	else
		fail "7: neue Pflichtvariable in .env.example bleibt unerwaehnt" "$OUT1"
	fi
fi

echo ""
echo "passed=$passed failed=$failed skipped=$skipped"
[ "$failed" -eq 0 ] || exit 1
echo "ALL TESTS PASSED"
