#!/usr/bin/env bash
# backup_doc_calls_test.sh — die in der Doku stehenden Backup-Aufrufe gegen das
# TATSAECHLICHE Verhalten der Skripte, nicht gegen ihren Wortlaut.
#
# Warum es diesen Test gibt (2026-07-30)
# --------------------------------------
# Seit #76 bricht `backup-verify.sh` ohne Passphrase mit exit 1 ab (richtig: ohne
# sie ist die Dump-Pruefung strukturell unmoeglich, und ein "verification passed"
# ohne geprueften Dump ist eine falsche Zusage). `backup.sh` faellt ebenso nur
# unter `[ -t 0 ]` auf die interaktive Abfrage zurueck — ein Cronjob hat kein
# Terminal, dort ist es exit 1.
#
# Die Doku zog nicht mit. Vier Stellen zeigten den nackten Aufruf weiter, darunter
# CRON-ZEILEN: der woechentliche Verify-Job lief jeden Sonntag ins Leere, der
# TAEGLICHE Backup-Job jede Nacht — zwei Zeilen ueber der Stelle, die zuerst
# repariert wurde (Review REV-R76 §2.2/F2). Ein Test, der nur den Text prueft,
# haette den zweiten Fall genauso uebersehen wie der Mensch davor. Deshalb faehrt
# dieser Test die dokumentierten Zeilen AUS:
#
#   A  Bestandsaufnahme: jede Cron-Zeile in *.md, die backup.sh/backup-verify.sh
#      aufruft, wird gefunden und muss VAKT_BACKUP_PASSPHRASE[_FILE] tragen.
#   B  Ausfuehrung: jede solche Zeile aus den Dateien dieses Diffs wird — mit auf
#      eine Sandbox umgebogenen Pfaden — WIRKLICH GESTARTET. Sie darf nicht am
#      Passphrase-Gate sterben.
#   C  Gegenprobe je Zeile: dieselbe Zeile OHNE die Passphrase-Zuweisung MUSS am
#      Passphrase-Gate sterben. Ohne C waere B vakuoes — gruen auch dann, wenn
#      das Gate gar nicht existierte.
#
# Was dieser Test NICHT prueft (Hausregel: benennen statt still ueberspringen)
# ---------------------------------------------------------------------------
# * Er faehrt kein Backup zu Ende. Ohne Postgres scheitert `backup.sh` NACH dem
#   Passphrase-Gate am Dump; das ist Umgebung, nicht Doku. Gemessen wird genau
#   das Gate, das die Doku-Zeile betrifft.
# * `pg_restore --list` auf dem entschluesselten Dump: die Sandbox-Attrappe ist
#   kein echter custom-format-Dump. Dass `backup-verify.sh` mit der
#   dokumentierten Passphrase bis zur ENTSCHLUESSELUNG kommt (und mit einer
#   falschen dort scheitert), wird gemessen; der Dump-Inhalt danach nicht.
#   Den Vollweg faehrt scripts/backup_restore_wiring_test.sh gegen echtes Postgres.
# * `backup-cron.sh`-Cron-Zeilen (docs/operations.md, docs/wiki/configuration.md)
#   gehoeren NICHT zu dieser Klasse: der Wrapper zieht die Passphrase aus der
#   internen Backup-Config-API bzw. der `.env` (scripts/backup-cron.sh:46). Sie
#   werden unten ausdruecklich gezaehlt und ausgenommen, nicht stillschweigend
#   uebergangen.
# * Dateien ausserhalb des Eigentums dieses Diffs werden GEMELDET, nicht rot
#   gefaerbt — siehe Abschnitt "fremde Dateien" in der Ausgabe.

set -uo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
# `|| exit 2` statt nacktem cd (SC2164): schlaegt das cd fehl, liefe der Rest gegen ein
# falsches Verzeichnis und meldete "keine Fundstelle" — ein gruenes Ergebnis fuer eine
# Pruefung, die nie stattgefunden hat.
cd "$ROOT" || exit 2

# Dateien, die dieser Diff besitzt: hier ist ein Verstoss ein FEHLER.
OWNED_DOCS=(
	"docs/backup-restore.md"
	"docs/operations/backup-restore.md"
	"docs/runbooks/disaster-recovery.md"
)

PASS=0
FAIL=0
WARN=0
SKIP=0

ok() {
	PASS=$((PASS + 1))
	echo "  ok    $1"
}
bad() {
	FAIL=$((FAIL + 1))
	echo "  FAIL  $1"
}
warn() {
	WARN=$((WARN + 1))
	echo "  WARN  $1"
}
skip() {
	SKIP=$((SKIP + 1))
	echo "  skip  $1"
}

echo "backup_doc_calls_test — dokumentierte Backup-Aufrufe gegen das echte Verhalten"
echo

# ── Werkzeuge ────────────────────────────────────────────────────────────────
MISSING=""
for t in gpg openssl tar sha256sum awk sed; do
	command -v "$t" >/dev/null 2>&1 || MISSING="$MISSING $t"
done
if [[ -n "$MISSING" ]]; then
	echo "NICHT PRUEFBAR: es fehlen im PATH:$MISSING"
	echo "ERGEBNIS: 0 gefahren — kein Urteil ueber die Doku-Aufrufe (exit 0)."
	exit 0
fi
for s in scripts/backup.sh scripts/backup-verify.sh; do
	if [[ ! -f "$s" ]]; then
		echo "NICHT PRUEFBAR: $s fehlt in diesem Repo."
		echo "ERGEBNIS: 0 gefahren (exit 0)."
		exit 0
	fi
done

# ── Sandbox: spielt /opt/vakt, /backups/vakt, /var/log und /etc/vakt ─────────
TMP="$(mktemp -d -t vakt-doc-calls-XXXXXX)"
cleanup() { rm -rf "$TMP"; }
trap cleanup EXIT

SANDBOX="$TMP/opt-vakt"
mkdir -p "$SANDBOX/backups" "$SANDBOX/log"
# Kein `.env` in der Sandbox: backup.sh/backup-verify.sh sourcen `.env` aus dem
# Arbeitsverzeichnis. Ein .env des Entwicklers wuerde diesen Test verfaelschen —
# genau die Verwechslung, die die Doku-Zeile ueberhaupt so lange ueberleben liess.
ln -s "$ROOT/scripts" "$SANDBOX/scripts"

TEST_PASS="test-passphrase-1234"
printf '%s' "$TEST_PASS" >"$SANDBOX/backup.key"
chmod 600 "$SANDBOX/backup.key"

# Ein signiertes Archiv, wie backup.sh es schreibt (gleiches Verfahren wie
# scripts/restore_test.sh): verschluesselter Dump + manifest + HMAC-Signatur.
SECRET_KEY="$(openssl rand -hex 32)"
STAGE="$TMP/stage"
mkdir -p "$STAGE"
echo "dummy-pgdump-content" >"$STAGE/db.pgdump"
printf '%s' "$TEST_PASS" | gpg --batch --yes --passphrase-fd 0 --pinentry-mode loopback \
	--symmetric --cipher-algo AES256 --output "$STAGE/db.pgdump.gpg" "$STAGE/db.pgdump" 2>/dev/null
rm -f "$STAGE/db.pgdump"
echo '{"backup_date":"test","tool":"vakt-backup"}' >"$STAGE/manifest.json"
ARCHIVE="$SANDBOX/backups/vakt-backup-2026-07-30_020000.tar.gz"
tar -czf "$ARCHIVE" -C "$STAGE" .
HMAC_KEY="$(printf 'vakt-backup-hmac:%s' "$SECRET_KEY" | sha256sum | cut -d' ' -f1)"
openssl dgst -sha256 -hmac "$HMAC_KEY" "$ARCHIVE" | awk '{print $NF}' >"${ARCHIVE}.sig"

PASSPHRASE_ERROR='No passphrase provided|keine Passphrase gesetzt'

# ── Eine Cron-Zeile auf die Sandbox umbiegen und ausfuehren ──────────────────
# Zurueck kommt der Log-Inhalt (die Cron-Zeile leitet selbst dorthin um) in
# $RUN_LOG_TEXT, der Exit-Code in $RUN_RC.
RUN_LOG_TEXT=""
RUN_RC=0
run_cron_command() { # $1 = Kommandoteil der Cron-Zeile (ohne Zeitplan)
	local cmd="$1"
	cmd="${cmd//\/opt\/vakt/$SANDBOX}"
	cmd="${cmd//\/backups\/vakt/$SANDBOX\/backups}"
	cmd="${cmd//\/var\/log\//$SANDBOX\/log\/}"
	cmd="${cmd//\/etc\/vakt\/backup.key/$SANDBOX\/backup.key}"
	: >"$SANDBOX/log/vakt-backup.log"
	: >"$SANDBOX/log/vakt-backup-verify.log"
	(
		cd "$SANDBOX" || exit 99
		# Wie unter cron: kein Terminal auf stdin, keine geerbte Passphrase.
		# VAKT_BACKUP_PG_MODE=host haelt den Lauf vom Docker-Suchlauf ab — ohne
		# DB scheitert er ohnehin, aber schnell und ohne Nebenwirkung.
		env -u VAKT_BACKUP_PASSPHRASE -u VAKT_BACKUP_PASSPHRASE_FILE \
			VAKT_DB_URL="postgres://vakt:vakt@127.0.0.1:1/vakt" \
			VAKT_SECRET_KEY="$SECRET_KEY" \
			VAKT_BACKUP_PG_MODE=host \
			bash -c "$cmd" </dev/null >/dev/null 2>&1
	)
	RUN_RC=$?
	RUN_LOG_TEXT="$(cat "$SANDBOX/log/"*.log 2>/dev/null)"
}

# ── A. Bestandsaufnahme ueber ALLE *.md ──────────────────────────────────────
CRON_RE='^[[:space:]]*[-0-9*/,]+[[:space:]]+[-0-9*/,]+[[:space:]]+[-0-9*/,]+[[:space:]]+[-0-9*/,]+[[:space:]]+[-0-9*/,]+[[:space:]]+.*scripts/backup(-verify)?\.sh'

echo "A. Bestandsaufnahme — nicht-interaktive (Cron-)Aufrufe in *.md"
mapfile -t HITS < <(grep -rnE "$CRON_RE" --include='*.md' . 2>/dev/null |
	grep -v node_modules | grep -v 'backup-cron\.sh' | sort)

if [[ ${#HITS[@]} -eq 0 ]]; then
	bad "keine einzige Cron-Zeile gefunden — der Sucher greift nicht mehr (Doku umgebaut?)"
fi

OWNED_LINES=()
for hit in "${HITS[@]}"; do
	file="${hit%%:*}"
	file="${file#./}"
	rest="${hit#*:}"
	lineno="${rest%%:*}"
	text="${rest#*:}"
	is_owned=0
	for o in "${OWNED_DOCS[@]}"; do [[ "$file" == "$o" ]] && is_owned=1; done
	if grep -qE 'VAKT_BACKUP_PASSPHRASE(_FILE)?=' <<<"$text"; then
		if [[ $is_owned -eq 1 ]]; then
			ok "$file:$lineno traegt die Passphrase-Zuweisung"
			OWNED_LINES+=("$file:$lineno:$text")
		else
			echo "        · $file:$lineno (fremde Datei) traegt die Zuweisung"
		fi
	else
		if [[ $is_owned -eq 1 ]]; then
			bad "$file:$lineno ruft ohne VAKT_BACKUP_PASSPHRASE[_FILE] auf — dieser Cronjob endet mit exit 1"
			echo "        $text"
		else
			warn "$file:$lineno ruft ohne VAKT_BACKUP_PASSPHRASE[_FILE] auf (Datei ausserhalb dieses Diffs)"
			echo "        $text"
			echo "        -> offener Befund derselben Klasse, gehoert dem Eigentuemer dieser Datei"
		fi
	fi
done
echo "        gefunden: ${#HITS[@]} Cron-Zeile(n), davon ${#OWNED_LINES[@]} in den Dateien dieses Diffs"
echo

# ── B/C. Ausfuehrung der eigenen Zeilen + Gegenprobe ─────────────────────────
echo "B/C. Ausfuehrung — jede eigene Cron-Zeile wird gestartet, mit und ohne Passphrase"
for entry in "${OWNED_LINES[@]}"; do
	file="${entry%%:*}"
	rest="${entry#*:}"
	lineno="${rest%%:*}"
	text="${rest#*:}"
	# Zeitplan (5 Felder) abschneiden -> reiner Kommandoteil
	cmd="$(sed -E 's/^[[:space:]]*([-0-9*/,]+[[:space:]]+){5}//' <<<"$text")"
	label="$file:$lineno"

	# B — wie dokumentiert: darf NICHT am Passphrase-Gate sterben.
	run_cron_command "$cmd"
	if grep -qE "$PASSPHRASE_ERROR" <<<"$RUN_LOG_TEXT"; then
		bad "B $label — wie dokumentiert gestartet und trotzdem am Passphrase-Gate gestorben"
		sed 's/^/        | /' <<<"$RUN_LOG_TEXT" | head -8
	else
		ok "B $label — kommt am Passphrase-Gate vorbei (ausgefuehrt, nicht gelesen)"
	fi
	# Bei backup-verify.sh laesst sich mehr belegen: Signatur geprueft UND der
	# Dump mit der dokumentierten Passphrase-Datei wirklich entschluesselt.
	if grep -q 'backup-verify\.sh' <<<"$cmd"; then
		if grep -q 'HMAC signature valid' <<<"$RUN_LOG_TEXT" &&
			! grep -qi 'decryption failed' <<<"$RUN_LOG_TEXT"; then
			ok "B $label — Signatur geprueft und Dump mit /etc/vakt/backup.key entschluesselt"
		else
			bad "B $label — kam nicht bis zur Entschluesselung"
			sed 's/^/        | /' <<<"$RUN_LOG_TEXT" | head -8
		fi
	fi

	# C — Gegenprobe: dieselbe Zeile ohne die Zuweisung MUSS am Gate sterben.
	cmd_nopass="$(sed -E 's/VAKT_BACKUP_PASSPHRASE(_FILE)?=[^[:space:]]+[[:space:]]+//' <<<"$cmd")"
	if [[ "$cmd_nopass" == "$cmd" ]]; then
		bad "C $label — die Zuweisung liess sich nicht entfernen, Gegenprobe unmoeglich"
		continue
	fi
	run_cron_command "$cmd_nopass"
	if grep -qE "$PASSPHRASE_ERROR" <<<"$RUN_LOG_TEXT" && [[ $RUN_RC -ne 0 ]]; then
		ok "C $label — ohne die Zuweisung: exit $RUN_RC am Passphrase-Gate (B ist nicht vakuoes)"
	else
		bad "C $label — ohne Passphrase erwartet: Abbruch am Gate. Bekommen: exit $RUN_RC"
		sed 's/^/        | /' <<<"$RUN_LOG_TEXT" | head -8
	fi
done
echo

# ── D. Falsche Passphrase: der Unterschied ist wirklich die Passphrase ───────
echo "D. Kontrolle — mit FALSCHER Passphrase scheitert die Entschluesselung"
printf '%s' "voellig-falsche-passphrase" >"$SANDBOX/wrong.key"
run_cron_command "cd /opt/vakt && VAKT_BACKUP_PASSPHRASE_FILE=$SANDBOX/wrong.key ./scripts/backup-verify.sh $ARCHIVE >> /var/log/vakt-backup-verify.log 2>&1"
if [[ $RUN_RC -ne 0 ]] && ! grep -qE "$PASSPHRASE_ERROR" <<<"$RUN_LOG_TEXT"; then
	ok "D falsche Passphrase — scheitert an der Entschluesselung, nicht am Gate (exit $RUN_RC)"
else
	bad "D falsche Passphrase — erwartet: Abbruch bei gpg. Bekommen: exit $RUN_RC"
	sed 's/^/        | /' <<<"$RUN_LOG_TEXT" | head -8
fi
echo

# ── E. Makefile-Hilfetext (Textpruefung, als solche benannt) ─────────────────
echo "E. Makefile — der Hilfetext von \`make backup-verify\` nennt die Passphrase"
if [[ -f Makefile ]]; then
	if grep -qE '^backup-verify:.*VAKT_BACKUP_PASSPHRASE' Makefile; then
		ok "E Makefile backup-verify: Hilfetext nennt VAKT_BACKUP_PASSPHRASE[_FILE] (Textpruefung)"
	else
		bad "E Makefile backup-verify: Hilfetext verschweigt die Passphrase"
	fi
else
	skip "E kein Makefile in diesem Repo"
fi

echo
echo "ERGEBNIS: passed=$PASS failed=$FAIL warned=$WARN skipped=$SKIP"
echo "  nicht geprueft: Dump-Inhalt nach der Entschluesselung (Attrappe, kein echter"
echo "                  custom-format-Dump) · der vollstaendige Backup-Lauf (kein Postgres)"
echo "                  · backup-cron.sh-Cron-Zeilen (Passphrase aus der Backup-Config-API)"
if [[ $FAIL -gt 0 ]]; then
	echo "FEHLER — eine dokumentierte Backup-Zeile verhaelt sich nicht wie dokumentiert."
	exit 1
fi
if [[ $WARN -gt 0 ]]; then
	echo "OK (mit $WARN offenen Befund(en) in fremden Dateien, oben namentlich genannt)."
else
	echo "OK — jede dokumentierte Cron-Zeile laeuft, und ohne ihre Passphrase-Zuweisung nicht."
fi
exit 0
