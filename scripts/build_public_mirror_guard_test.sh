#!/usr/bin/env bash
# build_public_mirror_guard_test.sh — der `rm -rf`-Waechter in
# scripts/build-public-mirror.sh, gefahren statt behauptet.
#
# Warum es diesen Test gibt (2026-07-30)
# --------------------------------------
# `build-public-mirror.sh` faehrt in Schritt 0 ein `rm -rf "$MIRROR"`, und
# `$MIRROR` kommt seit dem Mirror-Gate aus der Umgebungsvariablen
# VAKT_MIRROR_OUT. Der erste Waechter prueste nur "absolut, nicht /, nicht
# $ROOT" — und liess damit ein ELTERNVERZEICHNIS des Repos durch. Gemessen
# (Review REV-R76 §4/F1):
#
#   VAKT_MIRROR_OUT=/tmp/rev-r76/anc  (Repo in /tmp/rev-r76/anc/repo)
#   -> [mirror] Cleaning /tmp/rev-r76/anc/ …
#   -> danach: .../anc/repo/Makefile existiert nicht mehr
#
# Genau dieser Fall ist Fall 1 unten.
#
# Warum dieser Test NICHTS loeschen kann
# --------------------------------------
# Er fasst das echte Repo nicht an. Er baut in einem Tempdir eine Attrappe:
#
#   $TMP/anc/            <- Marker-Datei, spielt das Elternverzeichnis
#   $TMP/anc/repo/       <- spielt das Repo (Marker-Datei Makefile)
#   $TMP/anc/repo/scripts/build-public-mirror.sh   <- KOPIE des echten Skripts
#
# `$ROOT` loest das Skript aus dem eigenen Ort auf ($0/..), also ist $ROOT hier
# $TMP/anc/repo. Waere der Waechter entfernt, loescht der Lauf die ATTRAPPE —
# der Test wird rot, das echte Repo bleibt unberuehrt. Der I8-Nachweis der
# Nicht-Vakuitaet kostet damit kein Risiko.
#
# Fall 8 ist die Gegenprobe: ein harmloser Pfad muss DURCHkommen. Ein Waechter,
# der alles ablehnt, waere sonst genauso "gruen".

set -uo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SCRIPT="$ROOT/scripts/build-public-mirror.sh"

if [[ ! -f "$SCRIPT" ]]; then
	# Im gespiegelten Kundenrepo liegt build-public-mirror.sh bewusst nicht
	# (rsync --exclude). Kein Pruefsubjekt heisst hier: nichts zu pruefen —
	# gezaehlt und benannt statt still gruen.
	echo "SKIP: scripts/build-public-mirror.sh gibt es in diesem Repo nicht"
	echo "      (Public Mirror — das Skript wird bewusst nicht gespiegelt)."
	echo "ERGEBNIS: 0 Faelle gefahren, 1 uebersprungen (kein Pruefsubjekt)."
	exit 0
fi

TMP="$(mktemp -d -t vakt-mirror-guard-XXXXXX)"
cleanup() { rm -rf "$TMP"; }
trap cleanup EXIT

FAKE_ROOT="$TMP/anc/repo"
mkdir -p "$FAKE_ROOT/scripts" "$TMP/anc/sibling"
cp "$SCRIPT" "$FAKE_ROOT/scripts/build-public-mirror.sh"
# Marker: WENN geloescht wird, verschwinden sie. Beides Dateien, die der Reviewer
# im gemessenen Fall verloren hat.
echo "MARKER-REPO" >"$FAKE_ROOT/Makefile"
echo "MARKER-ELTERN" >"$TMP/anc/MARKER"
echo "MARKER-GESCHWISTER" >"$TMP/anc/sibling/data"

PASS=0
FAIL=0
LAST_RC=0
LAST_OUT=""

run_guard() { # $1 = VAKT_MIRROR_OUT
	LAST_OUT="$(VAKT_MIRROR_OUT="$1" bash "$FAKE_ROOT/scripts/build-public-mirror.sh" 2>&1)"
	LAST_RC=$?
}

markers_intact() {
	[[ -f "$FAKE_ROOT/Makefile" && -f "$TMP/anc/MARKER" && -f "$TMP/anc/sibling/data" \
		&& -f "$FAKE_ROOT/scripts/build-public-mirror.sh" ]]
}

ok() {
	PASS=$((PASS + 1))
	echo "  ok    $1"
}
bad() {
	FAIL=$((FAIL + 1))
	echo "  FAIL  $1"
	echo "        rc=$LAST_RC"
	echo "$LAST_OUT" | sed 's/^/        | /'
}

# $1 = Beschreibung, $2 = VAKT_MIRROR_OUT — muss mit exit 2 abgelehnt werden und
# darf nichts angefasst haben.
expect_refused() {
	local name="$1" path="$2"
	run_guard "$path"
	if [[ $LAST_RC -ne 2 ]]; then
		bad "$name — erwartet exit 2 (verweigert), bekommen exit $LAST_RC"
		return
	fi
	if ! markers_intact; then
		bad "$name — abgelehnt, aber es fehlen Dateien: der Lauf hat trotzdem geloescht"
		return
	fi
	if ! grep -q "verweigert" <<<"$LAST_OUT"; then
		bad "$name — exit 2, aber die Meldung sagt nicht, dass verweigert wurde"
		return
	fi
	ok "$name — verweigert (exit 2), Attrappe unversehrt"
}

echo "build-public-mirror.sh — Waechter auf dem rm -rf-Pfad"
echo "  Attrappe: $FAKE_ROOT (das echte Repo wird nicht angefasst)"
echo

# ── 1. DER GEMESSENE FALL: Elternverzeichnis des Repos ────────────────────────
expect_refused "1 Elternverzeichnis des Repos (der gemessene Loeschfall)" "$TMP/anc"

# ── 2. Weiter oben: Grosselternverzeichnis ────────────────────────────────────
expect_refused "2 Grosselternverzeichnis" "$TMP"

# ── 3. Das Repo selbst ────────────────────────────────────────────────────────
expect_refused "3 das Repo selbst" "$FAKE_ROOT"

# ── 4. Innerhalb des Repos, aber nicht der Standardpfad ───────────────────────
expect_refused "4 Verzeichnis IM Repo (scripts/)" "$FAKE_ROOT/scripts"

# ── 5. `..`-Segmente: derselbe Vorfahre, nur anders geschrieben ───────────────
expect_refused "5 Vorfahre ueber '..' geschrieben" "$FAKE_ROOT/public-mirror/../.."

# ── 6. Symlink, der auf den Vorfahren zeigt ───────────────────────────────────
ln -s "$TMP/anc" "$TMP/link-to-anc"
expect_refused "6 Symlink auf das Elternverzeichnis" "$TMP/link-to-anc"

# ── 7. Wurzel und relativer Pfad ──────────────────────────────────────────────
expect_refused "7a Wurzelverzeichnis" "/"
run_guard "relativer-pfad"
if [[ $LAST_RC -eq 2 ]] && markers_intact; then
	ok "7b relativer Pfad — verweigert (exit 2)"
else
	bad "7b relativer Pfad — erwartet exit 2, bekommen exit $LAST_RC"
fi

# ── 8. GEGENPROBE: ein harmloser Pfad muss durchkommen ────────────────────────
# Der Lauf scheitert danach an der ersten fehlenden Datei der Attrappe (sie ist
# kein echtes Repo) — das ist gewollt. Gemessen wird nur: der WAECHTER hat ihn
# durchgelassen (kein exit 2, und "Cleaning" ist gelaufen).
run_guard "$TMP/harmlos/out"
if [[ $LAST_RC -ne 2 ]] && grep -q "Cleaning" <<<"$LAST_OUT" && markers_intact; then
	ok "8 harmloser Pfad ausserhalb des Repos — durchgelassen (Waechter ist nicht pauschal)"
else
	bad "8 harmloser Pfad — der Waechter lehnt alles ab, damit waere er wertlos"
fi

# ── 9. Der Standardpfad im Repo bleibt erlaubt ────────────────────────────────
run_guard "$FAKE_ROOT/public-mirror"
if [[ $LAST_RC -ne 2 ]] && grep -q "Cleaning" <<<"$LAST_OUT" && markers_intact; then
	ok "9 \$ROOT/public-mirror (Standard) — weiterhin erlaubt"
else
	bad "9 \$ROOT/public-mirror — der Standardaufruf wurde mit abgeschossen"
fi

echo
echo "ERGEBNIS: passed=$PASS failed=$FAIL"
if [[ $FAIL -gt 0 ]]; then
	echo "FEHLER — der Waechter auf dem rm -rf-Pfad haelt nicht."
	exit 1
fi
echo "OK — jeder Pfad, der das Repo (oder einen Vorfahren) traefe, wird verweigert;"
echo "     harmlose Ziele und der Standard public-mirror/ kommen durch."
exit 0
