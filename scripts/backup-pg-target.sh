#!/usr/bin/env bash
# ESK-7 (Codeaudit-v5b, R1-06-D01) — "wie erreiche ich Postgres?" fuer den
# Backup-/Restore-Weg. Wird von scripts/backup.sh UND scripts/restore.sh
# gesourct; der "backup-"-Praefix benennt das Subsystem, nicht die Richtung.
#
# WARUM diese Datei existiert
# ---------------------------
# Der Standard-Compose-Stack legt `postgres` auf `db-net`, und `db-net` ist
# `internal: true` (docker-compose.yml). Der Service publiziert keinen Port. Ein
# Prozess auf dem HOST kann die Datenbank damit grundsaetzlich nicht erreichen —
# gemessen scheitert Host-`pg_dump` mit
#   pg_dump: error: could not translate host name "postgres" to address
# und Port-Publishing ist auf einem internal-Netz wirkungslos. `backup.sh` rief
# trotzdem das Host-Binary, `restore.sh` ebenso; damit war der beworbene
# skriptgestuetzte Backup-Weg auf jeder Standardinstallation tot, waehrend der
# manuelle `docker compose exec postgres pg_dump` aus docs/setup.md funktionierte.
#
# Die Loesung ist NICHT, eine erreichbare URL zu erraten, sondern den Dump dort
# auszufuehren, wo die Datenbank liegt: im DB-Container, ueber dessen lokalen
# Unix-Socket. Das braucht weder ein Passwort noch URL-Parsing:
#   * das offizielle postgres-Image liefert `local all all trust` (gemessen), und
#   * `POSTGRES_USER`/`POSTGRES_DB` setzt dasselbe Compose-File, das den
#     Container erzeugt hat — sie sind also per Konstruktion korrekt.
# Jede Alternative (URL umschreiben, Host auf 127.0.0.1 patchen, Port erraten)
# muesste eine Verbindungs-URL interpretieren und waere bei pgbouncer, externem
# Managed-Postgres oder abweichendem Port still falsch.
#
# Konfiguration (Ueberschreibungen; im Normalfall braucht niemand davon etwas):
#   VAKT_BACKUP_PG_MODE=auto|container|host   (Default: auto)
#       auto      — Container bevorzugen, sonst Host-Binary. Deckt beide
#                   Topologien ab: Compose-Stack UND externes/managed Postgres,
#                   das vom Host aus ohnehin erreichbar ist.
#       container — Container erzwingen; findet sich keiner, ist das ein Fehler
#                   (kein stiller Rueckfall auf einen Weg, der nicht gemeint war).
#       host      — Host-Binary erzwingen (externe DB, kein Docker im Spiel).
#   VAKT_BACKUP_PG_CONTAINER=<name|id>
#       Expliziter DB-Container; ueberspringt die Erkennung. Notwendig, wenn der
#       Compose-Projektname vom Verzeichnisnamen abweicht oder mehrere Stacks
#       parallel laufen.
#
# Der Aufrufer muss VAKT_PG_URL gesetzt haben (nur fuer den host-Weg benutzt).
#
# AUFRUF-KONTRAKT (KONV-K1 F2)
# ----------------------------
# Der Aufrufer MUSS `vakt_pg_require_valid_mode` EINMAL im Haupt-Shell aufrufen,
# bevor er irgendeine andere vakt_pg_*-Funktion benutzt. Diese eine Stelle ist
# die einzige, die den Zielweg bestimmt und die einzige, die abbrechen kann —
# siehe die Begruendung ueber der Funktion.

# Ergebnis der EINMALIGEN Aufloesung, gesetzt von vakt_pg_require_valid_mode im
# HAUPT-Shell. Leer = noch nicht aufgeloest; das ist ein Aufrufer-Fehler und wird
# von _vakt_pg_pick als rc=2 gemeldet, NICHT als "dann eben Host".
_VAKT_PG_RESOLVED_KIND=""
_VAKT_PG_RESOLVED_CID=""

# _vakt_pg_container: schreibt die Container-ID nach stdout, rc=0 bei Treffer.
_vakt_pg_container() {
	command -v docker >/dev/null 2>&1 || return 1

	if [ -n "${VAKT_BACKUP_PG_CONTAINER:-}" ]; then
		# Explizit benannt: existiert er nicht, ist das ein Fehler und kein Anlass,
		# still etwas anderes zu nehmen.
		docker inspect -f '{{.Id}}' "$VAKT_BACKUP_PG_CONTAINER" 2>/dev/null && return 0
		echo "ERROR: VAKT_BACKUP_PG_CONTAINER='${VAKT_BACKUP_PG_CONTAINER}' existiert nicht" >&2
		return 1
	fi

	local cid
	# Bevorzugt der Compose-Weg (respektiert COMPOSE_PROJECT_NAME und -f-Overlays).
	cid="$(docker compose ps -q postgres 2>/dev/null | head -1)"
	if [ -z "$cid" ]; then
		# Fallback fuer Aufrufe ausserhalb des Compose-Verzeichnisses — aber IMMER
		# auf das eigene Compose-Projekt eingeschraenkt.
		#
		# Ein Filter nur auf `com.docker.compose.service=postgres` waere
		# gefaehrlich, nicht bloss unsauber: auf einem Host mit mehreren Stacks
		# (MSP-Szenario — genau die Zielgruppe, "MSPs deploy per-customer") haette
		# `head -1` irgendeinen Postgres genommen und damit unter Umstaenden die
		# Datenbank eines ANDEREN Kunden gedumpt, unter dem Namen dieses hier.
		# Ein falsches Backup ist schlimmer als keins.
		local project="${COMPOSE_PROJECT_NAME:-}"
		if [ -z "$project" ]; then
			# Dieselbe Normalisierung, die Compose auf seinen Default-Projektnamen
			# anwendet (identisch zu resolve_uploads_volume in backup.sh).
			project=$(basename "$(pwd)" | tr '[:upper:]' '[:lower:]' | tr -cd 'a-z0-9_-')
		fi
		[ -n "$project" ] || return 1
		cid="$(docker ps -q \
			--filter "label=com.docker.compose.service=postgres" \
			--filter "label=com.docker.compose.project=${project}" 2>/dev/null | head -1)"
	fi
	[ -n "$cid" ] || return 1
	printf '%s' "$cid"
}

# vakt_pg_require_valid_mode: MUSS vom Aufrufer einmal im Haupt-Shell aufgerufen
# werden, nachdem die Umgebung (.env) geladen ist. Der Name ist historisch
# (backup.sh/restore.sh rufen ihn so) — die Funktion validiert nicht nur den
# Mode-String, sie LOEST DAS ZIEL AUF und ist die einzige Stelle im Modul, die
# abbrechen darf.
#
# Grund: `_vakt_pg_pick` wird an mehreren Stellen in einer Kommandosubstitution
# benutzt (z. B. `$(vakt_pg_describe)`). Ein `exit 1` darin beendet nur die
# SUBSHELL — das Skript lief weiter und meldete sogar "via host pg_dump", also
# genau den Weg, den der Betreiber nicht gewaehlt hatte (live nachgestellt: eine
# Mode-Angabe mit Tippfehler gab eine Fehlermeldung UND arbeitete weiter). Eine
# Fehlkonfiguration muss den Lauf beenden, nicht kommentieren.
#
# KONV-K1 F2: die erste Fassung dieses Guards prueft nur den MODE-STRING. Damit
# blieb die zweite Haelfte derselben Klasse offen: `VAKT_BACKUP_PG_MODE=container`
# ohne auffindbaren Container lief in das `exit 1` IN `_vakt_pg_pick`, also
# wieder in einer Subshell — der `if` sah rc!=0 und nahm den HOST-Zweig. Auf dem
# Mehr-Stack-Host, den diese Datei selbst als Zielgruppe nennt, heisst das:
# `vakt_pg_restore_from` faehrt `pg_restore --clean --if-exists` gegen die DB
# hinter VAKT_PG_URL — eine ANDERE Datenbank als die gewaehlte —, und das Log
# meldet den abgewaehlten Weg als den genommenen.
# Deshalb faellt die Entscheidung jetzt GENAU EINMAL, hier, im Haupt-Shell:
# danach ist sie in _VAKT_PG_RESOLVED_KIND/_CID festgeschrieben und keine
# spaetere Funktion kann sie mehr in einen anderen Weg umdeuten.
vakt_pg_require_valid_mode() {
	local mode="${VAKT_BACKUP_PG_MODE:-auto}" cid=""

	case "$mode" in
	auto | container | host) ;;
	*)
		echo "ERROR: VAKT_BACKUP_PG_MODE='${VAKT_BACKUP_PG_MODE}' ungueltig (erlaubt: auto|container|host)" >&2
		exit 1
		;;
	esac

	case "$mode" in
	host)
		_VAKT_PG_RESOLVED_KIND="host"
		_VAKT_PG_RESOLVED_CID=""
		;;
	container)
		# `_vakt_pg_container` laeuft hier ebenfalls in einer Kommando-
		# substitution — es benutzt deshalb ausschliesslich `return`, nie `exit`.
		# Der Abbruch passiert HIER, im Haupt-Shell, wo er auch wirkt.
		if cid="$(_vakt_pg_container)" && [ -n "$cid" ]; then
			_VAKT_PG_RESOLVED_KIND="container"
			_VAKT_PG_RESOLVED_CID="$cid"
		else
			echo "ERROR: VAKT_BACKUP_PG_MODE=container, aber kein postgres-Container gefunden." >&2
			echo "       KEIN Rueckfall auf den Host-Weg: der Host-pg_restore wuerde" >&2
			echo "       '--clean --if-exists' gegen die Datenbank hinter VAKT_PG_URL fahren," >&2
			echo "       also gegen eine andere als die gewaehlte." >&2
			echo "       Setze VAKT_BACKUP_PG_CONTAINER=<name> oder VAKT_BACKUP_PG_MODE=host." >&2
			exit 1
		fi
		;;
	auto)
		if cid="$(_vakt_pg_container 2>/dev/null)" && [ -n "$cid" ]; then
			_VAKT_PG_RESOLVED_KIND="container"
			_VAKT_PG_RESOLVED_CID="$cid"
		else
			_VAKT_PG_RESOLVED_KIND="host"
			_VAKT_PG_RESOLVED_CID=""
		fi
		;;
	esac

	# Der Host-Weg braucht VAKT_PG_URL. Fehlt sie, stirbt sonst erst spaeter ein
	# `${VAKT_PG_URL:?}` — und in `vakt_pg_base_table_count` (Kommando-
	# substitution) waere das wieder nur eine tote Subshell, die restore.sh als
	# "Tabellenzahl nicht nachpruefbar" liest statt als Fehlkonfiguration.
	if [ "$_VAKT_PG_RESOLVED_KIND" = "host" ] && [ -z "${VAKT_PG_URL:-}" ]; then
		echo "ERROR: Host-Weg gewaehlt (VAKT_BACKUP_PG_MODE=${mode}), aber VAKT_PG_URL ist nicht gesetzt." >&2
		exit 1
	fi

	return 0
}

# _vakt_pg_pick: liest die in vakt_pg_require_valid_mode getroffene Entscheidung
# vor. Diese Funktion trifft KEINE Entscheidung mehr und ruft nichts auf, das
# fehlschlagen koennte — sie laeuft ueberwiegend in Kommandosubstitutionen, wo
# weder `exit` noch eine Fehlermeldung den Aufrufer erreichen wuerden.
#
#   rc=0 : Container-Weg, Container-ID auf stdout
#   rc=1 : Host-Weg
#   rc=2 : NICHT aufgeloest — vakt_pg_require_valid_mode wurde nie aufgerufen.
#          Das ist ein Aufrufer-Fehler und ausdruecklich KEIN Host-Weg: jeder
#          Konsument unten bricht darauf ab, statt still den anderen Weg zu
#          nehmen (bei `pg_restore --clean` ist das die schlechteste Reaktion).
_vakt_pg_pick() {
	case "$_VAKT_PG_RESOLVED_KIND" in
	container)
		printf '%s' "$_VAKT_PG_RESOLVED_CID"
		return 0
		;;
	host)
		return 1
		;;
	*)
		echo "ERROR: PG-Ziel nicht aufgeloest — vakt_pg_require_valid_mode() wurde nicht aufgerufen." >&2
		return 2
		;;
	esac
}

# vakt_pg_describe: eine Zeile fuer das Log, damit im Nachhinein nachvollziehbar
# ist, WELCHEN Weg ein Lauf genommen hat. Ein Backup, dessen Weg man nicht kennt,
# ist im Zweifel nicht nachvollziehbar — und ein Backup, dessen Log den
# ABGEWAEHLTEN Weg meldet, ist schlimmer als eines ohne Log.
vakt_pg_describe() {
	# `|| rc=$?` statt einer nackten Zuweisung: unter `set -e` (beide Aufrufer)
	# wuerde eine Zuweisung aus einer Kommandosubstitution mit rc!=0 die Shell
	# beenden — und da vakt_pg_describe selbst in `$( )` steht, waere schon der
	# voellig normale Host-Weg (rc=1) ein stiller Subshell-Tod.
	local cid="" rc=0
	cid="$(_vakt_pg_pick)" || rc=$?
	case "$rc" in
	0) printf 'container %s' "$(docker inspect -f '{{.Name}}' "$cid" 2>/dev/null | sed 's|^/||')" ;;
	1) printf 'host pg_dump/pg_restore' ;;
	*)
		printf 'UNAUFGELOEST (Aufrufer-Fehler)'
		return "$rc"
		;;
	esac
}

# vakt_pg_dump_to <ausgabedatei>
# Schreibt einen custom-format-Dump. rc != 0 = Dump fehlgeschlagen.
vakt_pg_dump_to() {
	local out="$1" cid="" rc=0
	cid="$(_vakt_pg_pick)" || rc=$?
	case "$rc" in
	0)
		docker exec -i "$cid" sh -c 'exec pg_dump \
			-U "${POSTGRES_USER:?POSTGRES_USER ist im postgres-Container nicht gesetzt}" \
			-d "${POSTGRES_DB:?POSTGRES_DB ist im postgres-Container nicht gesetzt}" \
			--format=custom --compress=9' >"$out"
		;;
	1)
		pg_dump "${VAKT_PG_URL:?VAKT_PG_URL nicht gesetzt}" --format=custom --compress=9 -f "$out"
		;;
	*)
		# Kein Rueckfall. Ein Dump der falschen Datenbank unter dem Namen dieser
		# hier ist genau das "falsche Backup", das die Datei oben als schlimmer
		# als gar keins beschreibt.
		return "$rc"
		;;
	esac
}

# vakt_pg_restore_from <dumpdatei>
# Faehrt `pg_restore --clean --if-exists`. Gibt pg_restores Exit-Code zurueck —
# die BEWERTUNG dieses Codes passiert beim Aufrufer (restore.sh), weil nur der
# weiss, welche Fehlerklassen in seinem Kontext harmlos sind.
vakt_pg_restore_from() {
	local dump="$1" cid="" rc=0
	cid="$(_vakt_pg_pick)" || rc=$?
	case "$rc" in
	0)
		# pg_restore liest das custom-format-Archiv von stdin (gemessen, rc=0).
		docker exec -i "$cid" sh -c 'exec pg_restore --clean --if-exists \
			-U "${POSTGRES_USER:?POSTGRES_USER ist im postgres-Container nicht gesetzt}" \
			-d "${POSTGRES_DB:?POSTGRES_DB ist im postgres-Container nicht gesetzt}"' <"$dump"
		;;
	1)
		pg_restore --clean --if-exists -d "${VAKT_PG_URL:?VAKT_PG_URL nicht gesetzt}" "$dump"
		;;
	*)
		# Die destruktivste Stelle der Datei. `--clean --if-exists` gegen ein
		# ungeklaertes Ziel ist der Fall, in dem "dann eben der andere Weg" die
		# schlechteste denkbare Reaktion ist.
		return "$rc"
		;;
	esac
}

# vakt_pg_dump_readable <dumpdatei>
# rc=0, wenn das Archiv als custom-format-Dump lesbar ist. Nutzt bevorzugt das
# Host-Binary und faellt sonst auf den DB-Container zurueck — ein Host ohne
# postgres-client duerfte die Integritaetspruefung nicht einfach ueberspringen,
# sonst ist "ok" nur die Abwesenheit einer Pruefung.
vakt_pg_dump_readable() {
	local dump="$1" cid="" rc=0
	# `pg_restore --list` liest nur die lokale DATEI, keine Datenbank — hier ist
	# das Host-Binary deshalb unabhaengig vom gewaehlten Ziel zulaessig.
	if command -v pg_restore >/dev/null 2>&1; then
		pg_restore --list "$dump" >/dev/null 2>&1
		return $?
	fi
	cid="$(_vakt_pg_pick)" || rc=$?
	if [ "$rc" -eq 0 ]; then
		docker exec -i "$cid" pg_restore --list >/dev/null 2>&1 <"$dump"
		return $?
	fi
	# Weder Host-Binary noch Container: nicht pruefbar. Der Aufrufer MUSS das
	# ausweisen statt es als Erfolg zu lesen. (rc=2 deckt beide Faelle ab: kein
	# Container-Ziel — und ein nicht aufgeloestes Ziel, das erst recht nicht
	# stillschweigend als "geprueft" durchgehen darf.)
	return 2
}

# vakt_pg_base_table_count: Anzahl der BASE TABLEs in public, aus der DATENBANK
# gelesen — nicht aus der Absicht des Skripts. rc != 0, wenn nicht ermittelbar.
vakt_pg_base_table_count() {
	local q="SELECT count(*) FROM information_schema.tables WHERE table_schema='public' AND table_type='BASE TABLE';"
	local cid="" out rc=0
	cid="$(_vakt_pg_pick)" || rc=$?
	case "$rc" in
	0)
		out="$(docker exec -i "$cid" sh -c \
			"exec psql -tA -U \"\${POSTGRES_USER:?}\" -d \"\${POSTGRES_DB:?}\" -c \"$q\"" 2>/dev/null)" || return 1
		;;
	1)
		command -v psql >/dev/null 2>&1 || return 1
		out="$(psql -tA -d "${VAKT_PG_URL:?}" -c "$q" 2>/dev/null)" || return 1
		;;
	*)
		# Nicht aufgeloest: restore.sh liest rc!=0 als "nicht nachpruefbar" und
		# sagt das ausdruecklich. Ein Zaehler aus der FALSCHEN Datenbank waere
		# schlimmer — er wuerde einen Restore gegenpruefen, der dort nie lief.
		return 1
		;;
	esac
	out="$(printf '%s' "$out" | tr -d '[:space:]')"
	case "$out" in
	'' | *[!0-9]*) return 1 ;;
	esac
	printf '%s' "$out"
}
