#!/usr/bin/env bash
# Copyright (c) 2026 NorvikOps. All rights reserved.
# SPDX-License-Identifier: Elastic-2.0
#
# Fragt den CI-Zustand eines Commits so ab, dass die Antwort etwas bedeutet.
#
# Warum als Skript und nicht von Hand: Die Abfrage wurde am 2026-07-16 zweimal
# hintereinander falsch gebaut, beide Male mit gruen aussehendem Ergebnis.
#
#   1. `gh run list` zeigte "completed" — die Conclusion war `cancelled`. Ein
#      Nachfolge-Push hatte den Lauf per `cancel-in-progress` abgeraeumt (ci.yml
#      hat eine concurrency-group). Abgebrochen ist nicht bestanden.
#   2. Die Warteschleife pruefte "keine unfertigen Laeufe". Das ist auch erfuellt,
#      wenn es NULL Laeufe gibt: Direkt nach dem Push hatte GitHub die Runs noch
#      nicht angelegt, die Schleife fand nichts und meldete fertig.
#
# Und ein drittes Mal, im Skript selbst (R1-SA03-06, 2026-08-06): Die Abfrage
# lief OHNE `per_page`, die API liefert dann 30 Eintraege pro Seite und nennt die
# Gesamtzahl separat in `total_count`. Das Skript druckte `total_count` als
# seinen Nenner ("N Laeufe geprueft") und urteilte danach ueber das Array — also
# ueber hoechstens 30 Laeufe. Ein Fehlschlag jenseits der ersten Seite war
# unsichtbar, die Erfolgsmeldung nannte trotzdem die volle Zahl. In diesem Repo
# liegt der hoechste `total_count` je SHA bei 5, es gab also nie ein echtes
# falsches Gruen — die Klasse ist trotzdem dieselbe, und dieses Skript ist das
# Merge-Gate. Seitdem: `per_page=100` plus Paginierung, und der gedruckte Nenner
# wird gegen die Zahl der TATSAECHLICH geholten Laeufe geprueft.
#
# Alle drei Male dieselbe Klasse wie die Gate-Fehler in CLAUDE.md: Abwesenheit
# einer Meldung als Erfolg gelesen, "OK" ueber einer Teilmenge. Deshalb erzwingt
# dieses Skript die Reihenfolge "erst existieren, dann vollstaendig, dann
# bestanden" und nennt immer seinen Nenner.
#
# Regressionstest: scripts/ci_status_test.sh (gestubbtes `gh`, offline).
#
# Usage:
#   scripts/ci-status.sh              # HEAD
#   scripts/ci-status.sh <sha|ref>
#   scripts/ci-status.sh --wait       # wartet, bis alle Laeufe fertig sind
#
# Exit: 0 = alle Laeufe success
#       1 = irgendein Lauf nicht success (oder noch unfertig)
#       2 = kein Urteil moeglich: keine Laeufe, unvollstaendiger Nenner,
#           unlesbare API-Antwort, fehlendes Werkzeug

set -euo pipefail

REPO="${VAKT_CI_REPO:-Matharnica/vakt-platform}"

# 100 ist das Maximum der GitHub-API. MAX_PAGES ist eine Notbremse gegen eine
# endlos "weiter"-meldende API; wird sie erreicht, bleibt der Nenner
# unvollstaendig und die Pruefung unten faellt fail-closed aus.
PER_PAGE=100
MAX_PAGES=20

WAIT=0
REF="HEAD"
for arg in "$@"; do
  case "$arg" in
    --wait) WAIT=1 ;;
    *)      REF="$arg" ;;
  esac
done

command -v gh >/dev/null || { echo "ci-status: gh ist nicht installiert" >&2; exit 2; }
command -v jq >/dev/null || { echo "ci-status: jq ist nicht installiert" >&2; exit 2; }
SHA="$(git rev-parse "$REF")"
SHORT="${SHA:0:7}"

JSON=""
TOTAL=0
SEEN=0
PENDING=0

# Holt ALLE Laeufe zu $SHA und setzt JSON / TOTAL / SEEN / PENDING.
#
# Bricht das Skript ab (exit 2), sobald kein vollstaendiges Bild entsteht:
# unlesbare Antwort, fehlendes `total_count`, oder ein Nenner, der nicht
# aufgeht. Stilles Weiterlaufen mit einer Teilmenge ist genau der Defekt, den
# diese Funktion schliesst — deshalb gibt sie niemals Teildaten zurueck.
load_runs() {
  local page=1 body slice total='' acc='[]'

  while :; do
    if ! body="$(gh api "repos/$REPO/actions/runs?head_sha=$SHA&per_page=$PER_PAGE&page=$page")"; then
      echo "ci-status: $SHORT — GitHub-API-Abfrage fehlgeschlagen (Seite $page)." >&2
      echo "  Kein Urteil moeglich. Eine unlesbare Antwort ist ausdruecklich kein Erfolg." >&2
      exit 2
    fi

    total="$(printf '%s' "$body" | jq -r '.total_count // "null"')"
    if [ "$total" = "null" ]; then
      echo "ci-status: $SHORT — Antwort ohne total_count, der Nenner ist unbekannt." >&2
      echo "  Kein Urteil moeglich." >&2
      exit 2
    fi

    slice="$(printf '%s' "$body" | jq -c '.workflow_runs // []')"
    acc="$(jq -cn --argjson a "$acc" --argjson b "$slice" '$a + $b')"
    SEEN="$(printf '%s' "$acc" | jq 'length')"

    # Fertig, sobald der Nenner erreicht ist.
    if [ "$SEEN" -ge "$total" ]; then break; fi
    # Eine leere Seite trotz offenem Rest heisst: die API liefert nicht nach.
    # Nicht weiterfragen — die Nenner-Pruefung unten schlaegt dann zu.
    if [ "$(printf '%s' "$slice" | jq 'length')" -eq 0 ]; then break; fi

    page=$((page + 1))
    if [ "$page" -gt "$MAX_PAGES" ]; then break; fi
  done

  TOTAL="$total"
  JSON="$(jq -cn --argjson t "$TOTAL" --argjson r "$acc" '{total_count: $t, workflow_runs: $r}')"
  PENDING="$(printf '%s' "$JSON" | jq '[.workflow_runs[] | select(.status != "completed")] | length')"

  # --- Nenner-Selbstpruefung. Der gedruckte Nenner muss die Zahl der Laeufe
  #     sein, ueber die tatsaechlich geurteilt wurde — sonst ist jede Aussage
  #     darunter eine Aussage ueber eine Teilmenge.
  if [ "$SEEN" -ne "$TOTAL" ]; then
    echo "ci-status: $SHORT — Nenner unvollstaendig: die API meldet $TOTAL Laeufe, geholt wurden $SEEN." >&2
    printf '%s' "$JSON" | jq -r '.workflow_runs[] | "  \(.name): \(.status)/\(.conclusion // "-")"' >&2
    echo "  Kein Urteil moeglich — ueber die fehlenden Laeufe ist nichts bekannt." >&2
    exit 2
  fi
}

# --- 1. Erst existieren. Null Laeufe ist KEIN Erfolg, sondern eine offene Frage:
#        Push nicht angekommen, Pfad-Filter greift nicht, oder GitHub ist noch
#        nicht so weit. Alle drei brauchen einen Menschen, keinen gruenen Haken.
tries=0
load_runs
while [ "$TOTAL" -eq 0 ]; do
  tries=$((tries + 1))
  if [ "$WAIT" -eq 0 ] || [ "$tries" -gt 12 ]; then
    echo "ci-status: $SHORT — KEINE Workflow-Laeufe gefunden." >&2
    echo "  Das ist kein Erfolg. Moegliche Gruende: Push noch nicht verarbeitet," >&2
    echo "  Commit nicht auf origin, oder die Pfad-Filter der Workflows greifen nicht." >&2
    exit 2
  fi
  sleep 10
  load_runs
done

# --- 2. Dann fertig.
if [ "$WAIT" -eq 1 ]; then
  while [ "$PENDING" -ne 0 ]; do
    sleep 20
    load_runs
  done
fi

echo "ci-status: $SHORT — $TOTAL Lauf/Laeufe geprueft"
printf '%s' "$JSON" | jq -r '.workflow_runs[] | "  \(.name): \(.status)/\(.conclusion // "-")"'

if [ "$PENDING" -ne 0 ]; then
  echo "ci-status: $PENDING Lauf/Laeufe noch nicht fertig — kein Urteil moeglich (--wait nutzen)" >&2
  exit 1
fi

# --- 3. Nur `success` ist bestanden. cancelled/skipped/timed_out/failure sind es
#        ausdruecklich NICHT — `cancelled` sieht in der Liste wie fertig aus.
BAD="$(printf '%s' "$JSON" | jq -r '[.workflow_runs[] | select(.conclusion != "success") | "\(.name)=\(.conclusion)"] | join(", ")')"
if [ -n "$BAD" ]; then
  echo "ci-status: $SHORT NICHT bestanden — $BAD" >&2
  exit 1
fi

echo "ci-status: $SHORT — alle $TOTAL Laeufe success"
