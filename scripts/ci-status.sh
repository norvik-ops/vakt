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
# Beide Male dieselbe Klasse wie die Gate-Fehler in CLAUDE.md: Abwesenheit einer
# Meldung als Erfolg gelesen. Deshalb erzwingt dieses Skript die Reihenfolge
# "erst existieren, dann bestanden" und nennt immer seinen Nenner.
#
# Usage:
#   scripts/ci-status.sh              # HEAD
#   scripts/ci-status.sh <sha|ref>
#   scripts/ci-status.sh --wait       # wartet, bis alle Laeufe fertig sind
#
# Exit: 0 = alle Laeufe success · 1 = irgendein Lauf nicht success · 2 = keine Laeufe

set -euo pipefail

REPO="${VAKT_CI_REPO:-Matharnica/vakt-platform}"
WAIT=0
REF="HEAD"
for arg in "$@"; do
  case "$arg" in
    --wait) WAIT=1 ;;
    *)      REF="$arg" ;;
  esac
done

command -v gh >/dev/null || { echo "ci-status: gh ist nicht installiert" >&2; exit 2; }
SHA="$(git rev-parse "$REF")"
SHORT="${SHA:0:7}"

runs_json() { gh api "repos/$REPO/actions/runs?head_sha=$SHA" 2>/dev/null; }

# --- 1. Erst existieren. Null Laeufe ist KEIN Erfolg, sondern eine offene Frage:
#        Push nicht angekommen, Pfad-Filter greift nicht, oder GitHub ist noch
#        nicht so weit. Alle drei brauchen einen Menschen, keinen gruenen Haken.
tries=0
while [ "$(runs_json | jq -r '.total_count')" -eq 0 ]; do
  tries=$((tries + 1))
  if [ "$WAIT" -eq 0 ] || [ "$tries" -gt 12 ]; then
    echo "ci-status: $SHORT — KEINE Workflow-Laeufe gefunden." >&2
    echo "  Das ist kein Erfolg. Moegliche Gruende: Push noch nicht verarbeitet," >&2
    echo "  Commit nicht auf origin, oder die Pfad-Filter der Workflows greifen nicht." >&2
    exit 2
  fi
  sleep 10
done

# --- 2. Dann fertig.
if [ "$WAIT" -eq 1 ]; then
  while [ "$(runs_json | jq -r '[.workflow_runs[] | select(.status != "completed")] | length')" -ne 0 ]; do
    sleep 20
  done
fi

JSON="$(runs_json)"
TOTAL="$(echo "$JSON" | jq -r '.total_count')"
PENDING="$(echo "$JSON" | jq -r '[.workflow_runs[] | select(.status != "completed")] | length')"

echo "ci-status: $SHORT — $TOTAL Lauf/Laeufe geprueft"
echo "$JSON" | jq -r '.workflow_runs[] | "  \(.name): \(.status)/\(.conclusion // "-")"'

if [ "$PENDING" -ne 0 ]; then
  echo "ci-status: $PENDING Lauf/Laeufe noch nicht fertig — kein Urteil moeglich (--wait nutzen)" >&2
  exit 1
fi

# --- 3. Nur `success` ist bestanden. cancelled/skipped/timed_out/failure sind es
#        ausdruecklich NICHT — `cancelled` sieht in der Liste wie fertig aus.
BAD="$(echo "$JSON" | jq -r '[.workflow_runs[] | select(.conclusion != "success") | "\(.name)=\(.conclusion)"] | join(", ")')"
if [ -n "$BAD" ]; then
  echo "ci-status: $SHORT NICHT bestanden — $BAD" >&2
  exit 1
fi

echo "ci-status: $SHORT — alle $TOTAL Laeufe success"
