#!/usr/bin/env python3
# Gate G10 — S17/S29 (Codeaudit-v4, Sprint 135, Spur B).
#
# Fund-Klasse (S29-01/S29-02, dieselbe Klasse wie S121-A1/O1): ein Backup-Skript,
# das `pg_dump ... | gzip > out`
# schreibt, hat drei stille Fehlerquellen, die jede für sich ein leeres Backup als
# "OK" durchgehen lassen — und ein leeres Backup ist schlimmer als ein
# fehlgeschlagenes, weil es gute Kopien per Retention verdrängt, bevor jemand
# reagiert:
#   1. Ohne `set -o pipefail` meldet `$?` gzips Exit-Code, nicht pg_dumps — ein
#      fehlgeschlagener Dump hinter einem erfolgreichen gzip sieht wie Erfolg aus.
#   2. Ohne eine Mindestgrößen-Assertion wird ein ~20-Byte-Leergzip behalten und
#      repliziert.
#   3. Eine Retention, die NICHT auf den eigenen Datei-Präfix scoped ist (bare
#      `rm *` / `-name '*'`), kann über die eigenen Backups hinaus löschen.
# Separat (S29-02): ein hartcodiertes Klartext-PGPASSWORD in einem committeten
# Skript ist CWE-798/CWE-260, unabhängig vom Pipe-Muster oben.
#
# K2-04 (2026-07-30) — was dieses Gate NICHT sah und NICHT zählte:
#   PG_DUMP_PIPE_RE konnte keine Zeilengrenze überschreiten (`[^\n|]*`). Ein
#   Backup-Skript, das seinen pg_dump-Aufruf mit `\` umbricht — die normale
#   Shell-Schreibweise für eine lange Kommandozeile — war für ALLE DREI Prüfungen
#   unsichtbar: pipefail, Mindestgrößen-Assertion und Präfix-Scoping. Die Datei
#   wurde im `checked`-Zähler mitgezählt, im ausgewiesenen Nenner der Pipe-Prüfung
#   aber stillschweigend nicht — also genau die Klasse "grün über einer Teilmenge,
#   und die Teilmenge steht nirgends". Der Selbsttest fuhr fünf einzeilige Fälle.
#   Jetzt: Zeilenfortsetzungen (`\` am Zeilenende UND `|` am Zeilenende) werden vor
#   dem Matchen zusammengefaltet, Kommentarzeilen fallen weg, und drei neue Zähler
#   machen den Rest sichtbar (siehe Nenner unten).
#   Nebenbefund derselben Stelle, mitgefixt: BARE_WILDCARD_DELETE_RE lief NUR
#   innerhalb check_pipe_pattern, also nur für Skripte mit Pipe-Muster. Eine
#   bare-Wildcard-Retention in einem Skript ohne Pipe war unsichtbar. Läuft jetzt
#   über jedes Skript.
#
# Scope-Grenzen (bewusst, damit dieses Gate nicht wie check_routes.py/check_docs.py
# still einen Teil überspringt und trotzdem "OK" meldet):
#   - Die pipefail/Mindestgröße/Präfix-Prüfung gilt NUR für Skripte, die exakt das
#     `pg_dump ... | gzip` (oder `| gzip -c` / `| gzip -9` etc.) -Muster enthalten —
#     das ist die Klasse, die S29-01 tatsächlich betraf. Ein Skript, das pg_dump
#     direkt in eine Datei schreibt (`--format=custom -f out`, kein Pipe), hat
#     dieses Fehlerbild strukturell nicht (ein `set -e` lässt einen fehlgeschlagenen
#     pg_dump sofort abbrechen) und wird hier nicht verlangt, eine inline-
#     Mindestgrößen-Assertion zu tragen — dafür sorgt ggf. ein aufrufender Wrapper
#     (z. B. backup-cron.sh für backup.sh). Diese Skripte werden aber NAMENTLICH
#     ausgegeben, nicht stillschweigend übergangen.
#   - Ein Skript, das `pg_dump` in eine Pipe schreibt, die dieses Gate NICHT
#     klassifizieren kann, ist ein FEHLER (`unparsed`), kein sauberes Ergebnis —
#     sonst wäre die Faltung oben nur eine größere Version desselben Blindflecks.
#   - Der Secret-Scan (PGPASSWORD) und die bare-Wildcard-Prüfung laufen über JEDES
#     .sh unter scripts/ und infra/server/scripts/, unabhängig vom Pipe-Muster.
#   - Jede gescannte/übersprungene Datei wird gezählt und ausgegeben — kein Skip
#     bleibt unsichtbar.
#   - Weiterhin NICHT gesehen (benannt, nicht implizit): ein pg_dump, das erst zur
#     Laufzeit entsteht (`$DUMP_CMD | gzip`, `eval`, ein Heredoc, das ein anderes
#     Skript schreibt). Die Faltung löst die Zeilengrenze auf, nicht die
#     Indirektion. Solche Fälle tauchen weder in `pipeline checked` noch in
#     `unparsed` auf — sie sehen für dieses Gate aus wie kein Dump.
#
# R-02 (2026-07-30) — die Zusage oben („was ich nicht klassifizieren kann, ist
# `unparsed`") galt für pfad-qualifizierte Aufrufe NICHT: die Anker-Klasse von
# PG_DUMP_CALL_RE/PG_DUMP_LOOSE_PIPE_RE kannte kein `/`, also fiel
# `/usr/bin/pg_dump | zstd` aus allen drei Zählern und das Gate meldete OK.
# Jetzt erkannt (gemessen, nicht behauptet — siehe Formen-Matrix im Commit):
#   /usr/bin/pg_dump · ./scripts/pg_dump · ~/bin/pg_dump · ${BIN}/pg_dump
#   "$BIN_DIR"/pg_dump · $(dirname "$0")/pg_dump · pg_dump
#   scripts/pg_dump (N-04, bare relativ, in Kommandoposition)
#
# N-04 (2026-07-30) — R-02 nannte "$BIN_DIR/pg_dump" als DIE Restform. Das war
# eine Aufzählung, die sich abschließend las und es nicht war: der bare relative
# Pfad `scripts/pg_dump` (ohne `./`) fiel ebenfalls aus allen Zählern und stand
# in keiner der beiden Grenzen. Aufgenommen (siehe _CMD_POS_STRICT). Was jetzt
# NOCH wegfällt — vollständig aufgezählt, soweit gemessen, und im gedruckten
# `blind by design`-Block wiederholt:
#   1. "$BIN_DIR/pg_dump" — Pfad UND Programm in EINEM Quote. Ein Lookahead, der
#      ein schließendes Quote erlaubt, würde `grep 'host pg_dump'` wieder als
#      Dump zählen; das ist genau der interface-ratchet-Fehler.
#   2. Ein BARER RELATIVER Pfad in ARGUMENTPOSITION, also nach Whitespace statt
#      nach `| ; & ( ) { }` oder Zeilenanfang: `sudo -u postgres scripts/pg_dump`,
#      `PGPASSWORD=x scripts/pg_dump`, `env FOO=1 scripts/pg_dump`. Dort ist die
#      Form von Prosa (`echo "siehe docs/pg_dump …"`) nicht zu unterscheiden, und
#      der Prosa-Schutz gewinnt. MIT führendem `/ . ~ $` sind dieselben Aufrufe
#      erkannt (`sudo -u postgres /usr/bin/pg_dump` ist gemessen drin) — die
#      Lücke ist genau die Kombination bar+relativ+Argumentposition.
#   3. Gegenrichtung, damit auch die falsch-POSITIVE Seite benannt ist: seit
#      N-04 zählt eine Zeile, die in Kommandoposition mit `wort/pg_dump ` be-
#      ginnt, als Aufruf — in Shell-Code ist sie das auch. Steht sie in einem
#      HEREDOC oder in einem mehrzeiligen Quote (beides entfernt `unfold_shell`
#      nicht), ist sie Text und wird trotzdem gezählt. Ergebnis wäre ein
#      `unparsed`-Befund, also ein lautes Rot statt eines stillen Grüns.
#
# Exit non-zero on any violation.

import pathlib
import re
import sys

ROOT = pathlib.Path(__file__).resolve().parent.parent
SCAN_DIRS = ["scripts", "infra/server/scripts"]

# Wie viele Skripte eine ERKANNTE Dump-Pipeline tragen müssen. Untergrenze, kein
# Freeze: ein neues Backup-Skript mit Pipe wird ohnehin geprüft, aber ein
# BESTEHENDES, dessen Pipeline aus der Erkennung herausfällt (umgeschrieben auf
# `$DUMP_CMD | gzip`, in eine Funktion verschoben), würde die Abdeckung still von
# 3 auf 2 senken — und genau dieses stille Absinken ist der Defekt, den K2-04
# beschreibt, eine Ebene höher.
PIPE_PATTERN_BASELINE = 3

PG_DUMP_PIPE_RE = re.compile(r"pg_dump\b[^\n|]*\|\s*gzip\b")
# Ein pg_dump-AUFRUF, nicht das Wort. Zwei Bedingungen tragen das:
#   1. Der Lookahead auf Whitespace/Zeilenende ist der Unterschied zwischen Code
#      und Prosa: `printf 'host pg_dump/pg_restore'` und `grep 'host pg_dump'`
#      sind Log-Text, keine Dumps — ohne diese Bedingung hätte das Gate genau
#      das gezählt, was der interface-ratchet-Fund verbietet.
#   2. Der Aufruf steht in KOMMANDOPOSITION: Zeilenanfang oder nach `| ; & ( ) { }`
#      bzw. Whitespace, optional mit PFAD davor.
# R-02 (2026-07-30) — die Anker-Klasse enthielt `/` nicht und kannte keinen
# Pfad-Präfix. `/usr/bin/pg_dump | zstd` — im Cron-/systemd-Kontext die normale
# Schreibweise — fiel damit aus ALLEN VIER Zählern: nicht `pipeline checked`,
# nicht `unparsed`, nicht `pg_dump, no pipe`, und es senkte auch die Baseline
# nicht. Das Gate meldete OK über ein ungeprüftes Backup — exakt die Klasse
# "grün über einer Teilmenge, und die Teilmenge steht nirgends", gegen die der
# `unparsed`-Zweig unten gebaut wurde.
#
# Der Pfad-Präfix ist bewusst NICHT `[\w./]*/`: dann hätte `echo "docs/pg_dump ..."`
# als Aufruf gezählt. Er muss mit `/`, `.`, `~` oder einer Variablen beginnen —
# also so aussehen, wie ein Pfad aussieht, und nicht wie ein Wort in einem Satz.
#
# N-04 (2026-07-30) — der R-02-Fix nahm den ABSOLUTEN Pfad auf und übersah die
# FORM: `scripts/pg_dump | zstd`, der bare relative Pfad OHNE `./`, fiel weiter
# aus allen Zählern, weil `_PATH_PREFIX` einen Beginn mit `/`, `.`, `~` oder `$`
# verlangt. `./scripts/` ging, `scripts/` nicht — gemeldet als eine Restform,
# die in KEINER der beiden dokumentierten Grenzen stand. Genau der Fehler, den
# R-02 eine Runde vorher machte: die gemeldete Variante gefixt, nicht die Klasse.
#
# Aufgenommen wird sie über eine ZWEITE, engere Kommandoposition: Zeilenanfang
# oder direkt nach `| ; & ( ) { }`. NICHT nach blossem Whitespace — dort steht
# ein bares Wort mit Schrägstrich in ARGUMENTPOSITION, und dort ist
# `echo "siehe docs/pg_dump …"` von einem Aufruf nicht zu unterscheiden. Das ist
# dieselbe Abwägung wie oben, nur eine Ebene feiner: der Prosa-Schutz gewinnt,
# wo die Form mehrdeutig ist, und die Grenze steht unten im `blind by design`-
# Block, nicht nur hier.
_CMD_POS = r"(?:^|[|;&(){}\s])"
_PATH_PREFIX = r"""(?:["']?(?:\$\{?\w+\}?|~|\.{1,2})?["']?/[\w.$/{}"'-]*)?"""
# Echte Kommandoposition — kein Whitespace, weil Whitespace Argumentposition ist.
_CMD_POS_STRICT = r"(?:^|[|;&(){}])[ \t]*"
# `scripts/`, `infra/server/scripts/` — ein Pfad ohne führendes `/ . ~ $`.
_BARE_REL_PREFIX = r"(?:\w[\w.-]*/)+"
_PG_DUMP_CALL = (
    r"(?:" + _CMD_POS + _PATH_PREFIX
    + r"|" + _CMD_POS_STRICT + _BARE_REL_PREFIX
    + r")pg_dump(?=[ \t]|$)"
)
PG_DUMP_CALL_RE = re.compile(_PG_DUMP_CALL, re.MULTILINE)
# Lockere Gegenprobe: ein pg_dump-Aufruf, der in IRGENDEINE Pipe schreibt. Trifft
# sie, während PG_DUMP_PIPE_RE nicht trifft, ist die Pipeline vorhanden, aber
# unklassifiziert -> `unparsed`, nicht "kein Pipe-Muster".
PG_DUMP_LOOSE_PIPE_RE = re.compile(_PG_DUMP_CALL + r"[^\n]*\|", re.MULTILINE)
PIPEFAIL_RE = re.compile(r"\bset\s+-[a-zA-Z]*o\s+pipefail\b|\bset\s+-o\s+pipefail\b")
# A minimum-size assertion: a byte-count read (stat -c%s / wc -c) whose result is
# later compared with -lt/-le against a numeric threshold, followed somewhere
# after by a rejection (rm + exit, or a bare exit non-zero).
SIZE_READ_RE = re.compile(r"stat\s+-c%s|wc\s+-c")
SIZE_CMP_RE = re.compile(r"-lt\s+\"?\$\{?\w+\}?\"?|\[\s*\"?\$\{?\w+\}?\"?\s+-lt\b")
# Rejecting the undersized file: either removing it (rm -f — the STATUS=1/return
# pattern used by multi-DB batch scripts) or a hard `exit N` (single-DB scripts).
REJECT_RE = re.compile(r"\brm\s+-\w*f\w*\b|\bexit\s+[1-9]\d*\b")
# A delete/prune whose glob target is a bare wildcard, not scoped to a prefix.
BARE_WILDCARD_DELETE_RE = re.compile(
    r"(?:-name\s+['\"]\*['\"]|rm\s+-\w*f\w*\s+\*(?:\s|$)|>\s*\*\s)"
)

# PGPASSWORD (or a *_DB_PASSWORD-style var) assigned a literal value, not a
# reference to an env var (${...} / $VAR) and not left empty.
HARDCODED_SECRET_RE = re.compile(
    r"^\s*(?:export\s+)?(PGPASSWORD|[A-Z_]*DB_PASSWORD|[A-Z_]*PGPASS)\s*=\s*"
    r"['\"]?([^\s'\"$][^\n'\"]*)['\"]?\s*$",
    re.MULTILINE,
)


def scripts():
    for sub in SCAN_DIRS:
        d = ROOT / sub
        if not d.is_dir():
            continue
        for p in sorted(d.glob("*.sh")):
            yield p


def unfold_shell(text):
    """Ganze-Zeilen-Kommentare weg, Zeilenfortsetzungen zusammenfalten.

    Zwei Fortsetzungsformen, beide gültige Shell und beide für die alte
    zeilengebundene Regex unsichtbar (K2-04):
        pg_dump ... \\        <- Backslash am Zeilenende
          | gzip -9 > out
        pg_dump ... |         <- Pipe am Zeilenende
          gzip -9 > out
    Kommentare fallen vorher weg, damit ein auskommentiertes `# pg_dump | gzip`
    das Gate weder auslöst noch (schlimmer) einen Nenner füllt, den niemand fährt.
    """
    out, buf = [], None
    for line in text.splitlines():
        if line.lstrip().startswith("#"):
            continue
        buf = line if buf is None else buf + " " + line.strip()
        b = buf.rstrip()
        if b.endswith("\\"):
            buf = b[:-1].rstrip()
            continue
        if b.endswith("|"):
            buf = b
            continue
        out.append(buf)
        buf = None
    if buf is not None:
        out.append(buf)
    return "\n".join(out)


def check_pipe_pattern(path, text):
    """S29-01 class: pg_dump | gzip must be pipefail-guarded, size-asserted,
    and its retention must be prefix-scoped."""
    problems = []
    m = PG_DUMP_PIPE_RE.search(text)
    if not m:
        return problems, False

    if not PIPEFAIL_RE.search(text):
        problems.append(
            f"{path.relative_to(ROOT)}: pipes `pg_dump | gzip` without `set -o "
            f"pipefail` — a failed pg_dump is masked by gzip's exit code "
            f"(S29-01 class)."
        )

    has_size_read = SIZE_READ_RE.search(text)
    has_size_cmp = SIZE_CMP_RE.search(text)
    has_reject = REJECT_RE.search(text)
    if not (has_size_read and has_size_cmp and has_reject):
        problems.append(
            f"{path.relative_to(ROOT)}: pipes `pg_dump | gzip` without a visible "
            f"minimum-size assertion (byte-count read + `-lt` comparison + reject) "
            f"— an empty/partial gzip would be kept and replicated (S29-01 class)."
        )

    return problems, True


def check_bare_wildcard_delete(path, text):
    """K2-04-Nebenbefund: läuft über JEDES Skript, nicht nur über die mit
    Pipe-Muster. Eine Retention, die über die eigenen Dateien hinaus löscht, ist
    nicht weniger gefährlich, weil das Skript daneben keinen Dump pipet."""
    if not BARE_WILDCARD_DELETE_RE.search(text):
        return []
    return [
        f"{path.relative_to(ROOT)}: retention/cleanup deletes with a bare "
        f"wildcard (`*` / `-name '*'`) instead of a script-specific prefix — "
        f"can evict files it doesn't own."
    ]


def check_hardcoded_secret(path, text):
    problems = []
    for m in HARDCODED_SECRET_RE.finditer(text):
        var, val = m.group(1), m.group(2).strip()
        if not val:
            continue  # `PGPASSWORD=` (empty) — not a leak
        problems.append(
            f"{path.relative_to(ROOT)}: hardcoded {var} with a literal value — "
            f"CWE-798/CWE-260 (S29-02 class). Use a `.pgpass` file (mode 0600) or "
            f"docker-exec against the container's local socket (peer/trust auth, "
            f"no PGPASSWORD needed — see infra/server/scripts/backup-vakt-isms.sh)."
        )
    return problems


def main():
    problems = []
    checked = 0
    pipe_hits = []       # erkannte Dump-Pipeline -> voll geprüft
    dump_no_pipe = []    # pg_dump ohne Pipe (legitim: --format=custom -f)
    dump_unparsed = []   # pg_dump IN eine Pipe, aber nicht klassifizierbar

    for path in scripts():
        checked += 1
        text = path.read_text(encoding="utf-8", errors="ignore")
        # Gefaltet + kommentarfrei: nur so sieht das Gate eine über Zeilen
        # umgebrochene Pipeline (K2-04). Die zeilenverankerten Prüfungen
        # (PGPASSWORD) laufen weiter auf dem Originaltext.
        folded = unfold_shell(text)

        pipe_problems, matched = check_pipe_pattern(path, folded)
        problems.extend(pipe_problems)

        rel = str(path.relative_to(ROOT))
        if matched:
            pipe_hits.append(rel)
        elif PG_DUMP_CALL_RE.search(folded):
            if PG_DUMP_LOOSE_PIPE_RE.search(folded):
                dump_unparsed.append(rel)
            else:
                dump_no_pipe.append(rel)

        problems.extend(check_bare_wildcard_delete(path, folded))
        problems.extend(check_hardcoded_secret(path, text))

    if checked == 0:
        print("❌ Gate G10: 0 shell scripts found under "
              f"{', '.join(SCAN_DIRS)} — scan directories moved or empty?")
        sys.exit(1)

    # Eine Pipeline, die dieses Gate sieht, aber nicht einordnen kann, ist kein
    # sauberes Ergebnis. Ohne diesen Zweig wäre die Faltung oben nur eine
    # größere Ausgabe desselben Blindflecks: "kein Muster gefunden" =
    # "nichts zu prüfen".
    for rel in dump_unparsed:
        problems.append(
            f"{rel}: pipes `pg_dump` into something this gate cannot classify "
            f"(no recognizable `| gzip`). Its pipefail/size/retention guards were "
            f"NOT verified. Write the pipeline as `pg_dump ... | gzip ...` or "
            f"extend PG_DUMP_PIPE_RE — do not leave it unmeasured."
        )

    if len(pipe_hits) < PIPE_PATTERN_BASELINE:
        problems.append(
            f"only {len(pipe_hits)} script(s) with a recognized `pg_dump | gzip` "
            f"pipeline, baseline is {PIPE_PATTERN_BASELINE} — the checked set "
            f"SHRANK. Either a backup script was removed (then lower the baseline "
            f"in the same commit) or its pipeline stopped being recognizable, in "
            f"which case it is now unguarded and this gate would have stayed green. "
            f"Recognized: {', '.join(pipe_hits) or '(none)'}"
        )

    if problems:
        print("❌ Gate G10 (backup hardening) failed:\n")
        for p in problems:
            print(" - " + p)
        print(f"\n   Nenner: {checked} script(s) scanned, "
              f"{len(pipe_hits)} with a recognized dump pipeline, "
              f"unparsed: {len(dump_unparsed)}, "
              f"pg_dump without pipeline: {len(dump_no_pipe)}")
        sys.exit(1)

    print(
        f"✓ Gate G10 OK — {checked} shell script(s) scanned "
        f"({', '.join(SCAN_DIRS)}), {len(pipe_hits)} with a `pg_dump | gzip` "
        f"pattern all pipefail-guarded/size-asserted/prefix-scoped "
        f"(baseline {PIPE_PATTERN_BASELINE}), 0 bare-wildcard deletes, "
        f"0 hardcoded PGPASSWORD/DB_PASSWORD literals."
    )
    print(f"   pipeline checked : {len(pipe_hits)} — {', '.join(pipe_hits)}")
    print(f"   unparsed         : {len(dump_unparsed)}"
          + (f" — {', '.join(dump_unparsed)}" if dump_unparsed else ""))
    print(f"   pg_dump, no pipe : {len(dump_no_pipe)} (direct-to-file dumps, out of "
          f"scope by design — see the header)"
          + (f"\n                      {', '.join(dump_no_pipe)}" if dump_no_pipe else ""))
    # Wo dieses Gate WEGSCHAUT — bei jedem Lauf mitgedruckt, nicht nur im Kopf.
    # Ein Nenner, den nur der Quelltext kennt, ist kein Nenner (K2-04/R-02).
    print("   blind by design  : ein pg_dump, der erst zur Laufzeit entsteht "
          "($DUMP_CMD | gzip, eval, Heredoc); ein ganz-gequoteter Pfad "
          '("$BIN/pg_dump"); ein BARER RELATIVER Pfad in ARGUMENTposition, also '
          "nach Whitespace statt nach Zeilenanfang/`| ; & ( ) { }` "
          "(`sudo -u postgres scripts/pg_dump`, `PGPASSWORD=x scripts/pg_dump` — "
          "mit fuehrendem `/ . ~ $` sind dieselben Aufrufe erkannt); ein Dump "
          f"außerhalb von {', '.join(SCAN_DIRS)} oder in einer nicht-.sh-Datei. "
          "Solche Aufrufe erscheinen in KEINEM der drei Zähler oben. "
          "Gegenrichtung: eine Heredoc-/Quote-Zeile, die in Kommandoposition mit "
          "`wort/pg_dump ` beginnt, wird als Aufruf GEZAEHLT — das endet in "
          "`unparsed`, also in einem lauten Rot, nicht in einem stillen Gruen.")


if __name__ == "__main__":
    main()
