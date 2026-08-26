#!/usr/bin/env bash
# Copyright (c) 2026 NorvikOps. All rights reserved.
# SPDX-License-Identifier: Elastic-2.0
#
# ci_status_test.sh — Regressionstest fuer scripts/ci-status.sh.
#
# Der eingefrorene Defekt (R1-SA03-06): Das Skript holte
# `repos/<repo>/actions/runs?head_sha=<sha>` OHNE `per_page`. Die GitHub-API
# liefert dann 30 Eintraege pro Seite, meldet die Gesamtzahl aber im Feld
# `total_count`. Das Skript druckte `total_count` als seinen Nenner ("N Laeufe
# geprueft") und urteilte anschliessend ueber das zurueckgegebene Array — also
# ueber hoechstens 30 Laeufe. Ein fehlgeschlagener Lauf jenseits der ersten
# Seite war damit unsichtbar, und die Erfolgsmeldung nannte trotzdem die volle
# Zahl. Mit einem gestubbten `gh` belegt: "3 Laeufe geprueft ... alle 3
# success", exit 0, tatsaechlich geprueft: 1.
#
# Ehrliche Einordnung: In diesem Repo liegt der hoechste beobachtete
# `total_count` je SHA bei 5, es gab also nie ein echtes falsches Gruen. Der
# Test friert die Klasse trotzdem ein — ci-status.sh ist das Merge-Gate, und
# ein Gate, das seinen Nenner nicht prueft, ist genau die Fehlerklasse, die
# dieses Projekt wiederholt getroffen hat ("OK ueber einer Teilmenge").
#
# Der Test laeuft offline. `gh` und `git` werden durch Stubs im PATH ersetzt;
# der gh-Stub bildet die ECHTE Paginierungssemantik nach (Default per_page=30,
# `page`-Offset, konstantes `total_count`) und kann zusaetzlich eine API
# simulieren, die weniger liefert als sie behauptet.
#
# Nicht-Vakuitaet: gegen eine zurueckgedrehte Kopie zeigen, dann MUSS er rot
# werden:
#   git show <alter-commit>:scripts/ci-status.sh > /tmp/old-ci-status.sh
#   VAKT_CI_STATUS_SCRIPT=/tmp/old-ci-status.sh bash scripts/ci_status_test.sh
#
# Requires: bash, jq.

set -uo pipefail # bewusst kein -e: die Faelle unter Test sollen fehlschlagen

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SCRIPT="${VAKT_CI_STATUS_SCRIPT:-$SCRIPT_DIR/ci-status.sh}"

PASS=0
FAIL=0

pass() {
	PASS=$((PASS + 1))
	printf '  ok   %s\n' "$1"
}
fail() {
	FAIL=$((FAIL + 1))
	printf '  FAIL %s\n' "$1"
	[ -n "${2:-}" ] && printf '       | %s\n' "${2//$'\n'/$'\n'       | }"
	return 0
}

command -v jq >/dev/null 2>&1 || {
	echo "ci_status_test: jq fehlt" >&2
	exit 2
}
[ -f "$SCRIPT" ] || {
	echo "ci_status_test: Skript nicht gefunden: $SCRIPT" >&2
	exit 2
}

SB="$(mktemp -d)"
trap 'rm -rf "$SB"' EXIT
BIN="$SB/bin"
mkdir -p "$BIN"

FIXTURE="$SB/runs.json"
GH_CALLS="$SB/gh-calls.log"
export FIXTURE GH_CALLS

# ── Stubs ────────────────────────────────────────────────────────────────────
# `git` liefert nur eine feste SHA — das Skript braucht sonst nichts von git,
# und ein echter `git rev-parse` machte den Test vom Zustand des Arbeitsbaums
# abhaengig.
cat >"$BIN/git" <<'EOF'
#!/usr/bin/env bash
[ "${1:-}" = "rev-parse" ] && { echo "1111111222222233333334444444555555566666667"; exit 0; }
echo "stub git: nicht unterstuetzt: $*" >&2
exit 1
EOF

# `gh api` bildet die GitHub-Semantik nach:
#   · `total_count` ist IMMER die volle Zahl, unabhaengig von der Seite
#   · ohne `per_page` liefert die API 30 Eintraege (genau der Defekt)
#   · `page=N` verschiebt das Fenster
# GH_MODE steuert zwei Sonderfaelle:
#   partial  — die API meldet total_count, liefert aber nur einen Eintrag und
#              danach leere Seiten (Nenner nicht auffuellbar)
#   apierror — die Abfrage schlaegt fehl
cat >"$BIN/gh" <<'EOF'
#!/usr/bin/env bash
set -uo pipefail
echo "$*" >>"$GH_CALLS"
[ "${1:-}" = "api" ] || { echo "stub gh: nicht unterstuetzt: $*" >&2; exit 1; }

if [ "${GH_MODE:-normal}" = "apierror" ]; then
	echo "stub gh: HTTP 503 (simuliert)" >&2
	exit 1
fi

url="${2:-}"
qs=""
case "$url" in *\?*) qs="${url#*\?}" ;; esac
per_page=30
page=1
if [ -n "$qs" ]; then
	IFS='&' read -ra _kvs <<<"$qs"
	for kv in "${_kvs[@]}"; do
		case "$kv" in
			per_page=*) per_page="${kv#per_page=}" ;;
			page=*) page="${kv#page=}" ;;
		esac
	done
fi

all="$(cat "$FIXTURE")"
total="$(jq 'length' <<<"$all")"

if [ "${GH_MODE:-normal}" = "partial" ]; then
	if [ "$page" -eq 1 ]; then
		slice="$(jq -c '.[0:1]' <<<"$all")"
	else
		slice='[]'
	fi
else
	off=$(((page - 1) * per_page))
	slice="$(jq -c --argjson o "$off" --argjson n "$per_page" '.[$o:($o + $n)]' <<<"$all")"
fi

jq -n --argjson t "$total" --argjson r "$slice" '{total_count: $t, workflow_runs: $r}'
EOF

chmod 755 "$BIN"/*
export PATH="$BIN:$PATH"

# ── Helfer ───────────────────────────────────────────────────────────────────
# runs <n> <bad_index> <bad_conclusion> [pending_index]
# Baut eine Laufliste. bad_index < 0 = keiner schlaegt fehl.
make_fixture() {
	local n="$1" bad="$2" concl="$3" pending="${4:--1}"
	jq -n --argjson n "$n" --argjson bad "$bad" --arg concl "$concl" --argjson pend "$pending" '
		[range(0; $n) | {
			name: ("wf-" + (. | tostring)),
			status: (if . == $pend then "in_progress" else "completed" end),
			conclusion: (if . == $pend then null
			             elif . == $bad then $concl
			             else "success" end)
		}]' >"$FIXTURE"
}

# run_ci_status <mode> [args…] → setzt RC und OUT (stdout+stderr zusammen)
run_ci_status() {
	local mode="$1"
	shift
	: >"$GH_CALLS"
	OUT="$(GH_MODE="$mode" bash "$SCRIPT" "$@" 2>&1 </dev/null)"
	RC=$?
}

echo "ci_status_test: Skript unter Test = $SCRIPT"
echo

# ── Abnahme A: gruen auf der Baseline ────────────────────────────────────────
# Ein Gate, das bei gesundem Repo rot wird, wird abgeschaltet statt gefixt.
echo "── A. Baseline: alle Laeufe success ⇒ exit 0 ──"

make_fixture 3 -1 success
run_ci_status normal
if [ "$RC" -ne 0 ]; then
	fail "3× success wurde nicht als bestanden gewertet (rc=$RC)" "$OUT"
elif ! grep -q "alle 3 Laeufe success" <<<"$OUT"; then
	fail "Erfolgsmeldung nennt den Nenner nicht" "$OUT"
else
	pass "3× success ⇒ exit 0, Nenner genannt"
fi

run_ci_status normal --wait
if [ "$RC" -ne 0 ]; then
	fail "--wait bricht bei fertigen, erfolgreichen Laeufen (rc=$RC)" "$OUT"
else
	pass "--wait bei fertigen Laeufen ⇒ exit 0"
fi

# ── Abnahme B: ROT bei echter Regression, MIT Namensnennung ──────────────────
# "Einer dieser N" ist kein Befund — der Lauf muss beim Namen genannt werden.
echo "── B. Regression: nicht-success ⇒ exit 1, Lauf beim Namen ──"

for concl in failure cancelled timed_out skipped; do
	make_fixture 3 1 "$concl"
	run_ci_status normal
	if [ "$RC" -ne 1 ]; then
		fail "'$concl' wurde nicht als Fehlschlag gewertet (rc=$RC)" "$OUT"
	elif ! grep -q "wf-1=$concl" <<<"$OUT"; then
		fail "'$concl' gemeldet, aber ohne Namensnennung des Laufs" "$OUT"
	elif grep -q "alle 3 Laeufe success" <<<"$OUT"; then
		fail "'$concl' und trotzdem Erfolgsmeldung" "$OUT"
	else
		pass "conclusion=$concl ⇒ exit 1, 'wf-1=$concl' genannt"
	fi
done

# Ein noch laufender Lauf ist kein Urteil.
make_fixture 3 -1 success 2
run_ci_status normal
if [ "$RC" -ne 1 ]; then
	fail "unfertiger Lauf wurde nicht als 'kein Urteil' gewertet (rc=$RC)" "$OUT"
elif grep -q "alle 3 Laeufe success" <<<"$OUT"; then
	fail "unfertiger Lauf und trotzdem Erfolgsmeldung" "$OUT"
else
	pass "unfertiger Lauf ⇒ exit 1, keine Erfolgsmeldung"
fi

# Null Laeufe sind kein Erfolg (die Lektion vom 2026-07-16).
make_fixture 0 -1 success
run_ci_status normal
if [ "$RC" -ne 2 ]; then
	fail "null Laeufe wurden nicht mit exit 2 quittiert (rc=$RC)" "$OUT"
elif ! grep -q "KEINE Workflow-Laeufe" <<<"$OUT"; then
	fail "null Laeufe ohne erklaerende Meldung" "$OUT"
else
	pass "null Laeufe ⇒ exit 2"
fi

# ── Abnahme C: ROT, wenn der Nenner unvollstaendig ist ───────────────────────
# Der eigentliche Defekt. Die API meldet 3, liefert 1 — das Skript darf ueber
# die fehlenden 2 nicht urteilen und schon gar nicht "bestanden" sagen.
echo "── C. Nenner unvollstaendig ⇒ kein Erfolg ──"

make_fixture 3 -1 success
run_ci_status partial
if [ "$RC" -eq 0 ]; then
	fail "API meldet 3, liefert 1 — Skript meldete trotzdem BESTANDEN" "$OUT"
elif grep -q "alle 3 Laeufe success" <<<"$OUT"; then
	fail "Erfolgsmeldung ueber einem unvollstaendigen Nenner" "$OUT"
elif ! grep -qi "nenner" <<<"$OUT"; then
	fail "Abbruch ohne Hinweis auf den unvollstaendigen Nenner" "$OUT"
elif ! grep -q "3" <<<"$OUT" || ! grep -q "1" <<<"$OUT"; then
	fail "Nenner-Meldung nennt Soll/Ist nicht" "$OUT"
else
	pass "total_count=3 aber 1 geliefert ⇒ exit $RC, Nenner benannt"
fi

# Eine unlesbare API ist ebenfalls kein Erfolg, sondern "kein Urteil moeglich".
make_fixture 3 -1 success
run_ci_status apierror
if [ "$RC" -ne 2 ]; then
	fail "fehlgeschlagene API-Abfrage nicht mit exit 2 quittiert (rc=$RC)" "$OUT"
elif grep -q "success" <<<"$OUT"; then
	fail "fehlgeschlagene API-Abfrage und trotzdem Erfolgsmeldung" "$OUT"
else
	pass "API-Fehler ⇒ exit 2, keine Erfolgsmeldung"
fi

# ── D. Paginierung: der teure Fall ───────────────────────────────────────────
# Der Fehlschlag liegt auf Position 100 — mit dem Default per_page=30 und ohne
# `page` ist er unsichtbar, und das Skript meldete "alle 120 success".
echo "── D. Paginierung: Fehlschlag jenseits der ersten Seite ──"

make_fixture 120 100 failure
run_ci_status normal
if [ "$RC" -ne 1 ]; then
	fail "Fehlschlag auf Position 100 von 120 wurde uebersehen (rc=$RC)" "$OUT"
elif ! grep -q "wf-100=failure" <<<"$OUT"; then
	fail "Fehlschlag jenseits Seite 1 nicht beim Namen genannt" "$OUT"
else
	pass "Fehlschlag auf Position 100/120 gefunden und benannt"
fi

# … und der gleiche Umfang ohne Fehlschlag muss vollstaendig geprueft werden.
make_fixture 120 -1 success
run_ci_status normal
if [ "$RC" -ne 0 ]; then
	fail "120× success wurde nicht als bestanden gewertet (rc=$RC)" "$OUT"
elif ! grep -q "alle 120 Laeufe success" <<<"$OUT"; then
	fail "Erfolgsmeldung nennt nicht den vollen Nenner 120" "$OUT"
elif [ "$(grep -c '' <<<"$(grep -o 'wf-[0-9]*:' <<<"$OUT")")" -ne 120 ]; then
	fail "es wurden nicht alle 120 Laeufe aufgelistet" "$(grep -c 'wf-' <<<"$OUT") Zeilen"
else
	pass "120 Laeufe vollstaendig geholt und aufgelistet"
fi

# Beweisstueck statt Vermutung: die Abfrage MUSS per_page setzen und Seite 2
# tatsaechlich anfordern.
if ! grep -q "per_page=100" "$GH_CALLS"; then
	fail "die Abfrage setzt kein per_page=100" "$(cat "$GH_CALLS")"
elif ! grep -q "page=2" "$GH_CALLS"; then
	fail "Seite 2 wurde nie angefordert — der Nenner kann nicht vollstaendig sein" "$(cat "$GH_CALLS")"
else
	pass "Abfrage nutzt per_page=100 und fordert Seite 2 an"
fi

# ── Zusammenfassung ──────────────────────────────────────────────────────────
echo
echo "ci_status_test: script=$SCRIPT  passed=$PASS  failed=$FAIL"
[ "$FAIL" -eq 0 ] || exit 1
echo "OK"
