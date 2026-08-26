#!/usr/bin/env bash
# update-artifacts.sh — bringt die AUSLIEFERUNGS-ARTEFAKTE einer Installation auf
# den Stand der Zielversion: docker-compose*.yml, Caddyfile, scripts/, Makefile,
# .env.example — alles, was der Kunde als Dateien im Installationsverzeichnis
# liegen hat und was ein `docker compose pull` NICHT anfasst.
#
# WARUM DIESE DATEI EXISTIERT (R1-06-D00, Codeaudit-v5b)
# -----------------------------------------------------
# `update.sh` zog neue Images, startete neu und meldete "Update complete!" —
# ohne eine einzige dieser Dateien anzufassen. Gemessen an einer Installation auf
# v0.42.48: exit 0, und docker-compose.yml war danach byte-gleich mit dem alten
# Stand. Zwischen v0.42.48 und v0.42.49 liegen dort 112 geaenderte Zeilen (die
# komplette Docker-Secrets-Haertung) und 33 in der Caddyfile. Ein Bestandskunde
# bekam also die neuen Binaries in einer alten, ungehaerteten Umgebung: Secrets
# weiter im Klartext in `docker inspect`, caddy/postgres ohne cap_drop. Jeder
# Haertungs-Sprint der Vergangenheit nuetzte damit ausschliesslich Neukunden.
#
# Der Weg ist `git`, nicht ein Download: die dokumentierte Installation ist
# `git clone https://github.com/norvik-ops/vakt` (docs/setup.md, Schnellstart) —
# das Installationsverzeichnis IST ein Arbeitsbaum, und die Zielversion ist ein
# Commit bzw. ein Tag darin.
#
# WAS DIESES SKRIPT AUSDRUECKLICH NICHT TUT
# -----------------------------------------
# Es fasst NIE eine lokale Aenderung an. Kein `reset --hard`, kein `stash`, kein
# `checkout -f`, kein Merge-Commit — ausschliesslich `merge --ff-only`. Wer seine
# Caddyfile angepasst hat, bekommt einen Abbruch mit Dateiliste, keinen stillen
# Verlust. `.env` ist git-ignoriert und wird nie beruehrt.
#
# AUFRUF
#   scripts/update-artifacts.sh [--dir <installationsverzeichnis>] [--ref <tag|branch>]
#
# EXIT-CODES (der Aufrufer entscheidet, was daraus folgt)
#   0  Artefakte stehen auf dem Zielstand (aktualisiert ODER schon dort)
#   3  kann nicht aktualisieren — Grund steht auf stderr, NICHTS wurde veraendert
#   1  unerwarteter Fehler
set -euo pipefail

DIR="."
REF=""
while [ $# -gt 0 ]; do
	case "$1" in
	--dir)
		DIR="$2"
		shift 2
		;;
	--ref)
		REF="$2"
		shift 2
		;;
	-h | --help)
		sed -n '2,30p' "$0"
		exit 0
		;;
	*)
		echo "update-artifacts.sh: unbekanntes Argument: $1" >&2
		exit 1
		;;
	esac
done

# "Kann nicht" ist kein Fehlschlag des Updates, sondern eine Aussage an den
# Aufrufer — deshalb ein eigener Code und immer eine Handlungsanweisung dazu.
cannot() {
	echo "ARTEFAKTE NICHT AKTUALISIERBAR: $1" >&2
	shift
	for line in "$@"; do echo "  $line" >&2; done
	echo "  Es wurde nichts veraendert." >&2
	exit 3
}

command -v git >/dev/null 2>&1 ||
	cannot "git ist auf diesem Host nicht installiert." \
		"Ohne git kann dieses Skript den neuen Stand der Compose-/Caddy-/Skript-Dateien nicht holen." \
		"Installiere git, oder aktualisiere die Dateien von Hand (docs/UPGRADE.md, Abschnitt 'Installation ohne git-Checkout')."

DIR="$(cd "$DIR" 2>/dev/null && pwd)" || cannot "Verzeichnis nicht lesbar."

TOP="$(git -C "$DIR" rev-parse --show-toplevel 2>/dev/null || true)"
[ -n "$TOP" ] ||
	cannot "'$DIR' ist kein git-Arbeitsbaum." \
		"Die dokumentierte Installation ist ein 'git clone' (docs/setup.md)." \
		"Diese Installation ist anders entstanden (kopiert/entpackt) — welcher Stand hier liegt," \
		"ist von aussen nicht feststellbar, und ein Ueberschreiben waere geraten statt gewusst." \
		"Weg: docs/UPGRADE.md, Abschnitt 'Installation ohne git-Checkout'."

# Der Arbeitsbaum muss GENAU diese Installation sein. Liegt sie in einem
# Unterverzeichnis eines groesseren Repos, wuerde ein Merge fremde Dateien
# mitziehen — das ist nicht das, was der Betreiber angefordert hat.
[ "$TOP" = "$DIR" ] ||
	cannot "'$DIR' liegt INNERHALB des git-Repos '$TOP', ist aber nicht dessen Wurzel." \
		"Ein Update wuerde das ganze umgebende Repo mitziehen. Abbruch."

git -C "$DIR" remote get-url origin >/dev/null 2>&1 ||
	cannot "Der Installation fehlt ein 'origin'-Remote." \
		"Nachtragen: git -C '$DIR' remote add origin https://github.com/norvik-ops/vakt"

# Nur VERFOLGTE Aenderungen zaehlen. Unverfolgte Dateien (Backups, Logs, eigene
# Overlays) stehen einem Fast-Forward nicht im Weg und sind kein Grund, den
# Sicherheits-Fix des Kunden aufzuhalten.
DIRTY="$(git -C "$DIR" status --porcelain --untracked-files=no 2>/dev/null || true)"
if [ -n "$DIRTY" ]; then
	cannot "Es gibt lokale Aenderungen an mitgelieferten Dateien:" \
		"$(printf '%s' "$DIRTY" | sed 's/^/    /')" \
		"" \
		"Diese Aenderungen werden NICHT ueberschrieben. Deine Moeglichkeiten:" \
		"  a) uebernehmen:   git -C '$DIR' commit -am 'lokale Anpassungen'  (dann greift der Merge)" \
		"  b) verwerfen:     git -C '$DIR' checkout -- <datei>" \
		"  c) uebergehen:    ./scripts/update.sh --skip-artifacts   (Artefakte bleiben dann ALT)"
fi

OLD="$(git -C "$DIR" rev-parse HEAD)"
OLD_SHORT="$(git -C "$DIR" rev-parse --short HEAD)"

echo "→ Hole den neuen Stand (git fetch)..."
if ! git -C "$DIR" fetch --tags --quiet origin 2>/tmp/vakt-fetch-err.$$; then
	ERR="$(head -3 /tmp/vakt-fetch-err.$$ 2>/dev/null || true)"
	rm -f /tmp/vakt-fetch-err.$$
	cannot "'git fetch' ist fehlgeschlagen — der neue Stand konnte nicht geholt werden." \
		"$ERR" \
		"Ohne fetch ist NICHT feststellbar, ob die Artefakte aktuell sind — deshalb Abbruch" \
		"statt einer Erfolgsmeldung ueber einem unbekannten Stand."
fi
rm -f /tmp/vakt-fetch-err.$$

# Ziel bestimmen: erst das ausdruecklich verlangte Tag, sonst der verfolgte Branch.
TARGET=""
TARGET_NAME=""
if [ -n "$REF" ] && [ "$REF" != "latest" ]; then
	for cand in "refs/tags/$REF" "refs/remotes/origin/$REF" "$REF"; do
		if TARGET="$(git -C "$DIR" rev-parse -q --verify "${cand}^{commit}" 2>/dev/null)"; then
			TARGET_NAME="$REF"
			break
		fi
		TARGET=""
	done
	[ -n "$TARGET" ] ||
		cannot "Die Zielversion '$REF' gibt es im Repository nicht (weder als Tag noch als Branch)." \
			"Verfuegbare Versionen: git -C '$DIR' tag --sort=-v:refname | head"
else
	UPSTREAM="$(git -C "$DIR" rev-parse --abbrev-ref --symbolic-full-name '@{u}' 2>/dev/null || true)"
	if [ -z "$UPSTREAM" ]; then
		UPSTREAM="$(git -C "$DIR" symbolic-ref -q --short refs/remotes/origin/HEAD 2>/dev/null || true)"
	fi
	[ -n "$UPSTREAM" ] ||
		cannot "Der aktuelle Branch verfolgt keinen Remote-Branch." \
			"Setzen: git -C '$DIR' branch --set-upstream-to=origin/main"
	TARGET="$(git -C "$DIR" rev-parse -q --verify "${UPSTREAM}^{commit}")" ||
		cannot "'$UPSTREAM' ist nach dem fetch nicht aufloesbar."
	TARGET_NAME="$UPSTREAM"
fi

TARGET_SHORT="$(git -C "$DIR" rev-parse --short "$TARGET")"

if [ "$OLD" = "$TARGET" ]; then
	echo "   Artefakte sind bereits auf dem Zielstand (${TARGET_NAME} = ${TARGET_SHORT})."
	exit 0
fi

# Nur vorspulen. Ist der aktuelle Stand kein Vorfahr des Ziels, liegt entweder
# ein Downgrade oder ein eigener Commit dazwischen — beides braucht eine
# Entscheidung, die dieses Skript nicht treffen darf.
if ! git -C "$DIR" merge-base --is-ancestor "$OLD" "$TARGET"; then
	cannot "Der Zielstand ($TARGET_SHORT) ist kein Nachfolger des installierten Stands ($OLD_SHORT)." \
		"Moegliche Ursachen: eigene Commits in der Installation, oder ein Downgrade." \
		"Dieses Skript spult nur vor (--ff-only) und macht deshalb hier nichts." \
		"Uebergehen (Artefakte bleiben ALT): ./scripts/update.sh --skip-artifacts"
fi

# Was aendert sich? Der Betreiber soll es SEHEN, bevor die Container neu starten.
CHANGED="$(git -C "$DIR" diff --name-only "$OLD" "$TARGET" -- \
	'docker-compose*.yml' 'compose*.yml' Caddyfile Makefile scripts .env.example 2>/dev/null || true)"

git -C "$DIR" merge --ff-only --quiet "$TARGET" ||
	cannot "'git merge --ff-only' ist fehlgeschlagen." \
		"Der Arbeitsbaum ist unveraendert."

echo "✓ Artefakte aktualisiert: ${OLD_SHORT} → ${TARGET_SHORT} (${TARGET_NAME})"
echo "  Rueckfallpunkt (Artefakte): git -C '$DIR' checkout ${OLD_SHORT}"
if [ -n "$CHANGED" ]; then
	echo "  Geaenderte Auslieferungs-Dateien:"
	printf '%s\n' "$CHANGED" | sed 's/^/    /'
else
	echo "  (keine Compose-/Caddy-/Skript-Datei betroffen)"
fi

# Die .env des Kunden wird von keinem Update angefasst — sie ist git-ignoriert
# und traegt seine Passwoerter. Eine neue PFLICHT-Variable erreicht ihn deshalb
# nur, wenn das Update sie nennt. Verglichen werden die Schluessel-Namen aus
# .env.example (auch die auskommentierten, sie sind dort die Dokumentation).
# Kein `printf … | grep -q`: dieselbe SIGPIPE-unter-pipefail-Falle wie in
# update.sh (grep -q beendet sich beim ersten Treffer). Hier reicht ein Vergleich
# auf der Zeichenkette.
case "
$CHANGED
" in
*"
.env.example
"*) ENV_EXAMPLE_CHANGED=true ;;
*) ENV_EXAMPLE_CHANGED=false ;;
esac
if [ "$ENV_EXAMPLE_CHANGED" = true ] && [ -f "$DIR/.env" ]; then
	env_keys() { sed -nE 's/^#? ?([A-Z][A-Z0-9_]*)=.*/\1/p' "$1" 2>/dev/null | sort -u; }
	OLD_EXAMPLE="$(mktemp)"
	git -C "$DIR" show "${OLD}:.env.example" >"$OLD_EXAMPLE" 2>/dev/null || : >"$OLD_EXAMPLE"
	NEW_KEYS="$(comm -13 <(env_keys "$OLD_EXAMPLE") <(env_keys "$DIR/.env.example"))"
	MISSING="$(comm -23 <(printf '%s\n' "$NEW_KEYS" | grep -v '^$' | sort -u) <(env_keys "$DIR/.env"))"
	rm -f "$OLD_EXAMPLE"
	if [ -n "$MISSING" ]; then
		echo ""
		echo "⚠  Diese neuen Konfigurationswerte stehen in .env.example, aber nicht in deiner .env:"
		printf '%s\n' "$MISSING" | sed 's/^/     /'
		echo "   Deine .env wird von einem Update NIE veraendert. Pruefe in docs/UPGRADE.md,"
		echo "   ob einer der Werte fuer deine Installation gesetzt werden muss."
	fi
fi

exit 0
