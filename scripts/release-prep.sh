#!/usr/bin/env bash
# Bereitet ein Release vor: Versionen bumpen, Gates fahren, Commit + Tag vorschlagen.
#
# ── Warum es dieses Skript gibt ───────────────────────────────────────────────
#
# Frueher hat der Release-Workflow selbst gebumpt und nach dem Tag auf `main`
# committet (`chore(release): bump helm + openapi version`). Zwei Dinge waren daran
# kaputt, und beide sind teuer geworden:
#
#   1. Dieser Commit lief durch KEIN Gate. GitHub loest fuer Pushes, die ein Workflow
#      mit dem GITHUB_TOKEN macht, keine weiteren Workflows aus (Loop-Schutz). Der
#      Commit lag also ungeprueft auf main, bis zufaellig jemand anderes pushte —
#      und genau dieser sed hat in v0.42.42 den OLLAMA-Tag mit der Vakt-Version
#      ueberschrieben. Diesen Tag gibt es bei Ollama nicht: `docker compose up` war
#      22 Releases lang fuer JEDEN Neukunden tot.
#
#   2. Der Tag konnte per Konstruktion nie auf dem Bump-Commit sitzen — der Bump
#      entstand ja IM Release-Lauf, den der Tag ausloest. S123-G9 („Release-Tag auf
#      den Bump-Commit setzen") war damit unerfuellbar, stand aber als erledigt im
#      Ledger.
#
# Also andersherum: Der Bump ist jetzt ein GANZ NORMALER Commit, den ein Mensch
# macht — er laeuft durch CI wie jeder andere, und der Tag sitzt auf ihm. Der
# Workflow committet nichts mehr, er PRUEFT nur noch (Job "test" → "Assert version
# consistency"). Ein vergessener Bump bricht das Release, bevor ein einziges Image
# gebaut ist.
#
#   ./scripts/release-prep.sh v0.42.45
#
set -euo pipefail

cd "$(dirname "$0")/.."

TAG="${1:-}"
if [[ ! "$TAG" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
	echo "Aufruf: $0 vX.Y.Z    (z. B. $0 v0.42.45)" >&2
	exit 1
fi
VER="${TAG#v}"

if [[ -n "$(git status --porcelain)" ]]; then
	echo "FEHLER: Der Working Tree ist nicht sauber. Ein Release-Bump gehoert in einen" >&2
	echo "eigenen Commit — sonst taggst du fremde Arbeit mit." >&2
	exit 1
fi

echo "→ Bump auf ${VER}"

# ── helm/vakt/values.yaml — NUR unsere eigenen Images ─────────────────────────
#
# Ein blindes sed ueber alle `tag:`-Zeilen trifft auch ollama/ollama. Der Kommentar
# an der Zeile ("pinned; update manually") ist eine Bitte, keine Durchsetzung — also
# wird hier am `repository:` entschieden, nicht an der Einrueckung.
python3 - "$VER" <<'PY'
import re, sys
ver = sys.argv[1]
path = "helm/vakt/values.yaml"
out, repo, hits = [], "", 0
for line in open(path):
    m = re.match(r'\s*repository:\s*"?([^\s"#]+)', line)
    if m:
        repo = m.group(1)
    elif re.match(r'\s*tag:\s*"', line):
        if repo.startswith("ghcr.io/norvik-ops/"):
            line = re.sub(r'(tag:\s*)"[^"]*"', r'\g<1>"%s"' % ver, line)
            hits += 1
        repo = ""  # ein tag: gehoert zu genau einem repository:
    out.append(line)
if hits == 0:
    sys.exit("helm/vakt/values.yaml: keine eigene image.tag-Zeile gefunden — die "
             "Struktur hat sich geaendert, der Bump haette still nichts getan")
open(path, "w").writelines(out)
print(f"  values.yaml: {hits} eigene Image-Tags → {ver}")
PY

# ── helm/vakt/Chart.yaml ──────────────────────────────────────────────────────
# version/appVersion stehen auf Spalte 0. Ein frueheres sed ankerte auf "^  version:"
# (zwei Leerzeichen) und traf deshalb NIE — mit "|| true" dahinter, das den
# Fehlschlag verschluckte. Das Chart meldete sich als 0.29.0, waehrend die Images
# auf 0.42.x liefen. Deshalb hier: ersetzen UND nachpruefen.
sed -i "s/^version: .*/version: ${VER}/" helm/vakt/Chart.yaml
sed -i "s/^appVersion: .*/appVersion: \"${VER}\"/" helm/vakt/Chart.yaml
grep -q "^version: ${VER}$" helm/vakt/Chart.yaml
grep -q "^appVersion: \"${VER}\"$" helm/vakt/Chart.yaml
echo "  Chart.yaml:  version + appVersion → ${VER}"

# ── openapi.yaml info.version ─────────────────────────────────────────────────
sed -i "s/^  version: \"[^\"]*\"/  version: \"${VER}\"/" backend/internal/shared/apidocs/openapi.yaml
grep -q "^  version: \"${VER}\"$" backend/internal/shared/apidocs/openapi.yaml
echo "  openapi.yaml: info.version → ${VER}"

# ── Gates JETZT fahren, nicht hoffen ──────────────────────────────────────────
#
# check_image_tags.py ist der wichtigste: er faengt genau den Ollama-Fall (ein
# Fremd-Image, das die Vakt-Version traegt) — offline, ohne Registry-Abfrage.
echo
echo "→ Gates"
for gate in check_image_tags.py check-docs.py; do
	printf '  %-24s' "$gate"
	if python3 "scripts/$gate" >/dev/null 2>&1; then
		echo "OK"
	else
		echo "FAIL"
		echo
		echo "FEHLER: $gate ist rot. Der Bump wird NICHT committet." >&2
		python3 "scripts/$gate" >&2 || true
		exit 1
	fi
done

# ── CHANGELOG ─────────────────────────────────────────────────────────────────
# Das Release-Gate verlangt einen eigenen Abschnitt. Ein [Unreleased]-Abschnitt wird
# ausdruecklich nicht akzeptiert — genau diese Luecke liess 22 Releases undokumentiert
# durchgehen.
if ! grep -qE "^## \[v?${VER}\]" CHANGELOG.md; then
	echo
	echo "FEHLER: CHANGELOG.md hat keinen Abschnitt '## [${VER}]'." >&2
	echo "Promote [Unreleased] auf '## [${VER}] — $(date +%F)' — das Release-Gate" >&2
	echo "bricht sonst ab, NACHDEM du getaggt hast." >&2
	exit 1
fi
echo "  CHANGELOG                OK (Abschnitt [${VER}] vorhanden)"

echo
echo "Bereit. Jetzt:"
echo
echo "  git add helm/vakt/values.yaml helm/vakt/Chart.yaml backend/internal/shared/apidocs/openapi.yaml"
echo "  git commit -m 'chore(release): bump helm + openapi version to ${TAG}'"
echo "  git push origin main"
echo "  git tag -a ${TAG} -m '${TAG}'"
echo "  git push origin ${TAG}"
echo
echo "Der Tag sitzt damit AUF dem Bump-Commit, und der Bump laeuft durch CI wie"
echo "jeder andere Commit. Der Release-Workflow prueft die Versionen nur noch."
