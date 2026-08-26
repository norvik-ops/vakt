#!/usr/bin/env python3
# K5-17 (G15): FE↔BE FIELD-NAME reconciliation gate.
#
# ── THE DEFECT CLASS ─────────────────────────────────────────────────────────
# Route right, method right, status 200 — and then the frontend reads a field name
# the backend never writes. Sixteen instances of this were found in one pass
# (KONV-K5), across six modules:
#
#   the Statement of Applicability filtered on `group` (backend: `control_group`),
#   so the ISO-27001 core artefact showed an empty table for all 93 measures;
#   "create audit" and "complete audit" were 422 on every attempt; every phishing
#   campaign created through the UI got group_id = NULL; the git-scan "N findings"
#   badge could not render even when a repo leaked twelve cleartext secrets;
#   saving an SoA entry or an interested party overwrote the justification with
#   the empty string.
#
# ── WHY NO EXISTING GATE SEES IT ─────────────────────────────────────────────
# Each one stops exactly one level too early:
#   check_routes.py            compares (method, path). Never the body.
#   check_response_shape.py    only the XLSX-declared-but-CSV-written case.
#   check_openapi_coverage.py  a path ratchet, not a schema comparison.
#   TestOpenAPIContract        by its own header deliberately narrow; in practice
#                              only /health gets a 2xx body checked, the rest 401.
#   tsc                        the 593 TS interfaces are handwritten and
#                              module-local. It checks invention against
#                              invention, consistently.
#   the vitest suite           mocks the HOOK, never apiFetch, so it cannot cross
#                              the boundary the drift sits on — and froze two of
#                              the sixteen as green expectations.
#
# ── WHY NOT frontend/src/api/generated.ts ───────────────────────────────────
# The obvious idea is to diff the handwritten interfaces against the types
# generated from openapi.yaml — generated truth against invention. It was
# measured and rejected, because openapi.yaml has drifted from the handlers
# independently (R1-11-D03 records 136 spec-vs-handler deviations). Against the
# sixteen findings, at 42763c0:
#   * the spec carried the FRONTEND's invention verbatim for BIASummary
#     (total/high_critical/avg_rto_hours/avg_rpo_hours, all four with
#     `required:`) and for EmergencyContact (`available_247`) — anchoring on it
#     would have certified two findings as correct;
#   * for ten of the sixteen names (control_group, without_exclusion_reason,
#     total_audits, major_nc_count, needs_and_expectations, monitoring_frequency,
#     target_group_id, result_count, snippet, control_count) the spec says
#     nothing at all — the endpoint's response schema is not described at that
#     depth, so there is nothing to compare;
#   * and generated.ts is imported by zero of the 127 hooks, so there is no
#     route→type binding to exploit either.
# The oracle here is the Go `json:` tag. It does not describe the wire format; it
# IS the wire format, by definition of encoding/json. That is what this gate uses.
# (Spec-vs-handler is a real and separate gap, tracked as R1-11-D03. This gate
# deliberately does not try to be that gate too.)
#
# ── WHAT IT DOES ─────────────────────────────────────────────────────────────
# For every API-facing frontend type, every declared field is classified:
#
#   a namesake Go struct exists (FE `interface GitScan` ↔ Go `type GitScan struct`)
#     └ field is one of its json tags                     → OK
#     └ field is not                                      → FINDING  (paired)
#   no namesake Go struct
#     └ the name is not a json tag ANYWHERE in the backend,
#       nor a key in openapi.yaml, nor a migration column  → FINDING  (orphan)
#     └ the name exists somewhere but we cannot say where  → SKIP, counted
#
# The orphan tier needs no pairing at all and therefore cannot mispair: if no Go
# struct in the tree emits the name and no migration has the column, no response
# can carry it. The paired tier is the one with a false-positive rate — the K5
# pass got 58 machine candidates of which 42 were mispairings, and a gate that
# cries wolf 42 times gets switched off rather than fixed. Hence:
#
# ── RATCHET, NOT A THRESHOLD ─────────────────────────────────────────────────
# Every finding that is not a real defect lives in scripts/fe_be_fields_baseline.txt
# with a one-line REASON. That list may only ever get SHORTER:
#   * a finding not in the baseline            → FAIL (named, with the Go struct)
#   * a baseline entry that no longer occurs   → FAIL (stale: an improvement was
#                                                 made and not written down, so
#                                                 the gate would quietly allow
#                                                 the regression back in)
# Regenerate after a deliberate change:  python3 scripts/check_fe_be_fields.py --update-baseline
# and then WRITE A REASON on each new line. A baseline line without a reason is
# itself a finding (see --check-reasons, run as part of the normal invocation).
#
# ── WHAT IT STRUCTURALLY DOES NOT CHECK (also printed on every run) ─────────
#  1. VALUES. `audit_type: 'internal'` against `oneof=isms_internal …` is a 422
#     this gate cannot see; it compares names, not domains. Four of the sixteen
#     findings had a value-domain half (K5-03/05/06/07) that only the round-trip
#     vitest tests cover.
#  2. THE OTHER DIRECTION. A backend field no frontend reads (an unfinished
#     feature, a dead computation) is not reported.
#  3. INLINE/ANONYMOUS response types — `apiFetch<{ a: string }>(…)` has no name
#     to pair and no declaration to read.
#  4. WHICH ENDPOINT a type belongs to. Pairing is by TYPE NAME, not by route.
#     A frontend type whose name differs from the Go struct it consumes is
#     invisible here unless every one of its fields is an orphan — that is the
#     largest remaining hole, and it is why `skipped` matters.
#     A consequence, and the reason `name-collisions:` is printed: 25 type names
#     in this tree are declared more than once (20 of them with differing shapes),
#     and all of them are compared against the SAME namesake Go struct. Every one is
#     checked (see ts_types()), but a narrow view model sharing a wire type's name
#     is judged by the wire type's oracle, which is why such findings end up
#     baselined as `mispair:` rather than fixed.
#  5. UNITS, TIME ZONES, DATE FORMATS, nil-vs-empty-array, envelope-vs-bare.
#  6. Anything under a `map[string]…` / `additionalProperties` shape.
#
# ── WHAT IT USED TO NOT CHECK AND NOW DOES ──────────────────────────────────
#  Duplicate type names. Until 2026-07-30 the first declaration of a name won and
#  the rest were dropped into `unparsed` while the NAME still counted toward the
#  API-facing total. Six real wire types were displaced by narrower same-named view
#  models — `Control` (21 fields) by a 3-field page-local type among them — so the
#  gate reported coverage for types it never read, and injected drift in the
#  displaced declaration came out exit 0. Every declaration is now checked and the
#  denominator prints `declarations — checked/displaced/skipped`, where displaced
#  is derived, not counted, so a regression to that rule cannot report 0.
#
# No network, no Docker, no running stack, no third-party import: two regex
# passes over the tree. Exit 0 = clean, 1 = a finding or a stale baseline entry.

import argparse
import pathlib
import re
import sys

ROOT = pathlib.Path(__file__).resolve().parent.parent
BACKEND = ROOT / "backend"
FRONTEND = ROOT / "frontend" / "src"
SPEC = BACKEND / "internal" / "shared" / "apidocs" / "openapi.yaml"
MIGRATIONS = BACKEND / "db" / "migrations"
BASELINE = pathlib.Path(__file__).resolve().parent / "fe_be_fields_baseline.txt"

# ── Go side ──────────────────────────────────────────────────────────────────

GO_STRUCT_RE = re.compile(r"^type\s+([A-Z][A-Za-z0-9_]*)\s+struct\s*\{", re.MULTILINE)
JSON_TAG_RE = re.compile(r'json:"([^"]*)"')


# An embedded struct: a line in a struct body that is only a (possibly qualified,
# possibly pointer) type name, with no field name and no tag. encoding/json
# promotes its fields, so its json tags are part of THIS struct's wire shape.
# Without this, ApprovalWithDetails (which embeds Approval and adds four joined
# columns) reports eleven false findings.
EMBED_RE = re.compile(r"^\s*\*?(?:[a-z][\w]*\.)?([A-Z][A-Za-z0-9_]*)\s*(?://.*)?$", re.MULTILINE)


def go_structs(backend=BACKEND):
    """Return (structs, all_tags).

    structs:  Go struct name -> set of json field names it serialises, with
              embedded structs resolved.
              A name declared more than once (different packages) maps to the
              UNION of both, which is the conservative direction: it can only
              suppress a finding, never invent one.
    all_tags: every json tag seen anywhere in the backend.
    """
    structs: dict[str, set[str]] = {}
    embeds: dict[str, set[str]] = {}
    all_tags: set[str] = set()
    for f in sorted(backend.rglob("*.go")):
        if f.name.endswith("_test.go"):
            continue
        text = f.read_text(encoding="utf-8", errors="ignore")
        for m in GO_STRUCT_RE.finditer(text):
            name = m.group(1)
            body = _brace_body(text, m.end() - 1)
            tags = set()
            for tag in JSON_TAG_RE.findall(body):
                field = tag.split(",")[0].strip()
                if field and field != "-":
                    tags.add(field)
            structs.setdefault(name, set()).update(tags)
            embeds.setdefault(name, set()).update(
                e for e in EMBED_RE.findall(body) if e != name)
        for tag in JSON_TAG_RE.findall(text):
            field = tag.split(",")[0].strip()
            if field and field != "-":
                all_tags.add(field)

    # Promote embedded fields to a fixpoint (embedding can nest).
    for _ in range(8):
        changed = False
        for name, parents in embeds.items():
            for parent in parents:
                extra = structs.get(parent)
                if extra and not extra <= structs[name]:
                    structs[name] |= extra
                    changed = True
        if not changed:
            break
    return structs, all_tags


def _brace_body(text, open_idx):
    """Return the text between the brace at open_idx and its match."""
    depth = 0
    for i in range(open_idx, len(text)):
        if text[i] == "{":
            depth += 1
        elif text[i] == "}":
            depth -= 1
            if depth == 0:
                return text[open_idx + 1: i]
    return text[open_idx + 1:]


def spec_keys(spec=SPEC):
    """Every `key:` in openapi.yaml. Deliberately over-broad: this set only ever
    SUPPRESSES an orphan finding, so a wide net costs recall, not precision, and
    recall we can afford (the paired tier catches the same drift from the other
    side)."""
    if not spec.exists():
        return set()
    keys = set()
    for line in spec.read_text(encoding="utf-8", errors="ignore").splitlines():
        m = re.match(r"\s+([a-z_][a-z0-9_]*)\s*:", line)
        if m:
            keys.add(m.group(1))
    return keys


COL_RE = re.compile(r"^\s{2,}([a-z_][a-z0-9_]*)\s+[A-Za-z]", re.MULTILINE)


def migration_columns(migrations=MIGRATIONS):
    """Column names from CREATE TABLE / ADD COLUMN in the migrations. Catches
    fields that reach the wire through a map or a sqlc row rather than a tagged
    struct field."""
    cols = set()
    if not migrations.exists():
        return cols
    for f in migrations.glob("*.sql"):
        text = f.read_text(encoding="utf-8", errors="ignore")
        cols.update(COL_RE.findall(text))
        cols.update(re.findall(r"ADD\s+COLUMN\s+(?:IF\s+NOT\s+EXISTS\s+)?([a-z_][a-z0-9_]*)", text, re.I))
    return cols


# ── Frontend side ────────────────────────────────────────────────────────────

TS_DECL_RE = re.compile(
    r"^(?:export\s+)?(?:interface\s+([A-Za-z_$][\w$]*)\s*(?:extends\s+[^{]+)?\{"
    r"|type\s+([A-Za-z_$][\w$]*)\s*=\s*\{)",
    re.MULTILINE,
)
# The three idioms that name a wire type in this codebase:
#   apiFetch<Foo[]>('/path')                       — the transport itself
#   useQuery<Foo>({ queryFn: … })                  — the response type
#   useMutation<Foo, Error, CreateFooInput>({ … }) — response AND request type
# Anything else (component props, zustand stores, view models, filter objects) is
# not a wire type; those are counted as `not API-facing` and not scanned. This is
# the precision knob: widening it to "declared in a file that calls apiFetch"
# was tried and produced ~50 component-prop false positives, which is how gates
# get switched off.
API_GENERIC_RE = re.compile(r"\b(?:apiFetch|useQuery|useMutation)\s*<([^>(]*(?:<[^>]*>)?[^>(]*)>")
# Exactly two spaces of indentation = a top-level field of the declaration. Four
# or more is a nested object literal, whose keys belong to a different shape and
# cannot be paired against a Go struct name.
FIELD_RE = re.compile(
    r"^ {2}(?! )(?:readonly\s+)?['\"]?([A-Za-z_$][\w$]*)['\"]?\s*\??\s*:",
    re.MULTILINE,
)


def _strip_ts_comments(body: str) -> str:
    """Kommentare weg, ZEILEN- UND SPALTENERHALTEND.

    R-03-Klasse (2026-07-30): ein `/* … */` über mehrere Zeilen wurde ersatzlos
    gelöscht und zog damit Zeilen zusammen — gemessen an 170 echten .ts-Dateien
    war das bei 32 der Fall (generated.ts: 47369 -> 47061 Zeilen). Dieses Gate
    druckt zwar keine Zeilennummern, also war nichts falsch BESCHRIFTET; aber
    FIELD_RE verankert auf `^ {2}` — Zeilen zusammenzuziehen kann ein Feld
    hinter einem gelöschten Kommentar aus dem Anker fallen lassen. Blanking
    statt Löschen macht beides unmöglich und kostet nichts.

    Deshalb Leerzeichen statt Entfernen: die Ersetzung ist längengleich, `\\n`
    bleibt `\\n`. Nachgemessen: Ausgabe bitgleich zur vorherigen Fassung."""
    def blank(m):
        return re.sub(r"[^\n]", " ", m.group(0))
    body = re.sub(r"/\*.*?\*/", blank, body, flags=re.S)
    return re.sub(r"//[^\n]*", blank, body)


def ts_types(frontend=FRONTEND, root=None):
    """Return (decls, api_named, unparsed, collisions, dropped).

    decls:      list of (relative path, type name, set of top-level field names) —
                EVERY declaration, sorted by path. A type name declared in n files
                yields n entries, each judged on its own.
    api_named:  type names that appear inside an `apiFetch<…>` type argument
    unparsed:   declarations whose body we could not read as a field list
    collisions: type name -> sorted list of paths, for names declared more than
                once with DIFFERENT shapes. Reported, not skipped (see below).
    dropped:    declarations matched by TS_DECL_RE that ended up in NEITHER decls
                NOR unparsed — i.e. silently discarded. 0 by construction here;
                returned as a number rather than assumed, because the bug this
                replaced discarded them at exactly this point, and a count that is
                only computed downstream of the discard reads 0 while it happens
                (measured: simulating first-declaration-wins keeps the downstream
                identity intact and only this counter moves).

    ── WHY EVERY DECLARATION, AND NOT "THE FIRST ONE WINS" ──────────────────────
    Until 2026-07-30 this function keyed by type name alone and let the first
    declaration (alphabetically first file) win; later ones were dropped into
    `unparsed`. Measured consequence: `Control` was "checked" as the 3-field view
    model in pages/ISO27001ChecklistPage.tsx while the 21-field wire type in
    modules/vaktcomply/types.ts was dropped — and the NAME still counted toward
    the API-facing total, so the run reported coverage for a type it never read.
    Real drift injected into the dropped declaration (VVTEntry.legal_basis→
    legal_bases, Certificate.id→id_XX) came out exit 0. Six wire types were
    affected that way; 25 type names in this tree carry more than one declaration.

    That is this gate's own defect class one level up: reporting a green result
    for work it did not do. So the rule is now: check ALL declarations.

    The three candidate rules, and why this one:
      * first-declaration-wins  — arbitrary (alphabetical file order decides which
        shape represents the name) and silent. Rejected: it is the bug.
      * let modules/**/types.ts win — less arbitrary, but still drops the other
        declarations unchecked, and a narrow duplicate in a page file is exactly
        where a drift can hide with nobody looking. Rejected.
      * check all, qualify the finding key by declaring file — no declaration is
        dropped, coverage is monotone (everything checked before is still
        checked), and the key names the file so one baseline line can never
        silence a second declaration it was never written about. Chosen.
        Measured cost at the repaired baseline: 0 new findings, 61 → 61.

    A shape collision under one name is not itself a wire defect (pairing is by
    name — blind spot 4 — so both shapes are compared against the same Go struct
    and any drift in either one is reported on its own). It is printed and counted
    on every run so the ambiguity stays visible, but it does not fail the gate:
    failing would demand baselining 25 naming decisions that carry no wire risk,
    and a gate that fails on things nobody will fix gets switched off.
    """
    decls: list[tuple[str, str, set[str]]] = []
    seen: dict[str, dict[str, frozenset]] = {}
    api_named: set[str] = set()
    unparsed: list[str] = []
    matched = 0
    # Paths are reported relative to `root` (the repo root in production, the
    # temp tree in the self-test) so the message is copy-pasteable either way.
    root = root or ROOT
    files = sorted(list(frontend.rglob("*.ts")) + list(frontend.rglob("*.tsx")))
    for f in files:
        rel = str(f.relative_to(root)) if root in f.parents or root == f.parent else str(f)
        if f.name == "generated.ts":
            continue  # generated FROM openapi.yaml — not a handwritten claim
        text = f.read_text(encoding="utf-8", errors="ignore")

        for m in API_GENERIC_RE.finditer(text):
            for name in re.findall(r"[A-Za-z_$][\w$]*", m.group(1)):
                api_named.add(name)

        for m in TS_DECL_RE.finditer(text):
            matched += 1
            name = m.group(1) or m.group(2)
            body = _strip_ts_comments(_brace_body(text, text.index("{", m.start())))
            fields = {fm.group(1) for fm in FIELD_RE.finditer(body)}
            if not fields:
                unparsed.append(f"{rel}: {name} (no readable top-level fields)")
                continue
            decls.append((rel, name, fields))
            seen.setdefault(name, {})[rel] = frozenset(fields)

    collisions = {name: sorted(where) for name, where in seen.items()
                  if len(where) > 1 and len(set(where.values())) > 1}
    dropped = matched - len(decls) - len(unparsed)
    return decls, api_named, unparsed, collisions, dropped


API_TYPES_DIRS = ("frontend/src/modules/", "frontend/src/shared/types/")


def is_api_facing(name: str, rel: str, api_named: set[str]) -> bool:
    """A type is API-facing if it is named in an apiFetch/useQuery/useMutation
    type argument, or it is declared in a module/shared `types.ts` — this repo's
    convention for wire types, and where the module response/request shapes live.
    Nothing else is scanned."""
    if name in api_named:
        return True
    return rel.endswith("types.ts") and rel.startswith(API_TYPES_DIRS)


# ── Findings ─────────────────────────────────────────────────────────────────

# A finding is identified by the DECLARING FILE, then the type, then the field.
# The file belongs in the key because a type name is not unique in this tree (25
# names carry more than one declaration): without it, one baseline line would
# silence a second declaration nobody ever wrote a reason about. `::` and not `#`
# because `#` opens the reason comment in the baseline file.
def finding_key(rel: str, name: str, field: str) -> str:
    return f"{rel}::{name}.{field}"


class Tally:
    """Declaration-level accounting, so the denominator cannot overstate itself.

    `displaced` is the number that used to be missing: declarations the run never
    judged but whose type NAME still counted as covered. It is the sum of two
    drop sites, both derived rather than incremented by hand, because the bug this
    replaced dropped at the earlier one and a counter placed only at the later one
    reads 0 while it happens:

      parse_dropped  — matched by TS_DECL_RE, returned in neither decls nor
                       unparsed (ts_types' `dropped`). This is where
                       first-declaration-wins threw declarations away.
      classify_gap   — API-facing but neither field-compared nor skipped with a
                       printed reason (api_facing - checked - skipped).

    displaced == 0 therefore means: every declaration the regex found either had
    its fields compared, or is listed with a reason why it could not be.
    """

    def __init__(self):
        self.read = 0           # every parsed declaration in frontend/src
        self.api_facing = 0
        self.checked = 0        # declarations whose fields were compared one by one
        self.skipped = 0        # API-facing but not comparable, with a printed reason
        self.unparsed = 0       # body could not be read as a field list
        self.parse_dropped = 0  # discarded before classification — must stay 0
        self.fields_checked = 0
        self.fields_skipped = 0

    @property
    def classify_gap(self):
        return self.api_facing - self.checked - self.skipped

    @property
    def displaced(self):
        return self.parse_dropped + self.classify_gap


def classify(structs, known, decls, api_named, tally=None):
    """The gate's decision, shared by analyse() and the self-test.

    Returns (findings, skipped_details). One pass per DECLARATION, not per name.
    """
    findings = []   # (key, kind, detail)
    skipped = []
    tally = tally if tally is not None else Tally()

    for rel, name, fields in sorted(decls):
        tally.read += 1
        if not is_api_facing(name, rel, api_named):
            continue
        tally.api_facing += 1
        go_fields = structs.get(name)
        # A Go struct with NO json tags at all serialises its Go field names
        # (CamelCase). Comparing snake_case frontend fields against that would
        # flag every field of every such type, so it is unjudgeable rather than a
        # finding — counted as a skip so "OK" never overstates coverage.
        if go_fields is not None and not go_fields:
            tally.skipped += 1
            tally.fields_skipped += len(fields)
            skipped.extend(
                f"{rel}: {name}.{f} — Go `type {name} struct` carries no json tags "
                f"(serialises Go field names); cannot compare" for f in sorted(fields))
            continue
        tally.checked += 1
        for field in sorted(fields):
            key = finding_key(rel, name, field)
            if go_fields is not None:
                tally.fields_checked += 1
                if field not in go_fields:
                    findings.append((
                        key, "paired",
                        f"{rel}: {name}.{field} is not a json tag of Go `type {name} struct` "
                        f"(it emits: {', '.join(sorted(go_fields)) or '<none>'})",
                    ))
            elif field not in known:
                tally.fields_checked += 1
                findings.append((
                    key, "orphan",
                    f"{rel}: {name}.{field} — no Go struct emits or binds `{field}` anywhere "
                    f"in backend/, it is no openapi.yaml key and no migration column",
                ))
            else:
                tally.fields_skipped += 1
                skipped.append(f"{rel}: {name}.{field} — no Go struct named `{name}`; "
                               f"`{field}` exists elsewhere in the backend, cannot attribute")

    return findings, skipped


def analyse():
    structs, all_tags = go_structs()
    known = all_tags | spec_keys() | migration_columns()
    decls, api_named, unparsed, collisions, dropped = ts_types()

    tally = Tally()
    findings, skipped = classify(structs, known, decls, api_named, tally)
    tally.unparsed = len(unparsed)
    tally.parse_dropped = dropped
    return findings, skipped, unparsed, collisions, tally


# ── Baseline ─────────────────────────────────────────────────────────────────

def read_baseline():
    """Return {key: reason}. A line is `path::Type.field  # reason`."""
    entries = {}
    if not BASELINE.exists():
        return entries
    for line in BASELINE.read_text(encoding="utf-8").splitlines():
        line = line.strip()
        if not line or line.startswith("#"):
            continue
        key, _, reason = line.partition("#")
        entries[key.strip()] = reason.strip()
    return entries




# Every baseline line must start its reason with one of these tags. The gate
# enforces it, because "we wrote it down" is not the same as "we understood it".
#   mispair:  the namesake Go struct is a DIFFERENT type than the endpoint
#             returns (the K5 pass found 42 of these among 58 candidates)
#   fe-only:  a view model or hook wrapper that never crosses the wire
#   renamed:  the concept exists in Go under another name, the hook maps it
#   request:  a request-body field bound off a different input struct
#   carried:  NOT UNDERSTOOD. A live candidate of exactly the class this gate
#             exists for, carried so the gate can be switched on today instead of
#             never. Counted and printed as `unverified:` on every run — this
#             number is a debt, not a pass. Drive it to zero.
REASON_TAGS = ("mispair:", "fe-only:", "renamed:", "request:", "carried:")

BASELINE_HEADER = """\
# FE↔BE field-name findings that are NOT (or are not yet confirmed as) defects.
# K5-17 / G15 — see scripts/check_fe_be_fields.py for the whole argument.
# Regenerate:  python3 scripts/check_fe_be_fields.py --update-baseline
#
# ONE LINE PER TOLERATED FINDING:  path/to/decl.ts::Type.field  # <tag>: <why>
# The declaring file is part of the key: 25 type names in this tree are declared
# more than once, and every declaration is checked, so a bare `Type.field` line
# would silence a second declaration nobody wrote a reason about.
# This list may only ever get SHORTER. A line whose finding no longer reproduces
# fails the gate, so an improvement cannot be made without being recorded.
#
# Every reason MUST start with one of:
#   mispair:  the namesake Go struct is a DIFFERENT type than the endpoint returns
#   fe-only:  a view model / hook wrapper that never crosses the wire
#   renamed:  the concept exists in Go under another name, the hook maps it
#   request:  a request-body field bound off a different input struct
#   carried:  NOT UNDERSTOOD — a live candidate of the very class this gate
#             exists for. Reported as `unverified:` on every run. This is debt.
"""


NEW_ON_REGEN = "carried: new on regeneration — NOT investigated. Classify or fix."

REGEN_RULE = "# ── new on the last --update-baseline — classify these ──────────────────────"


def write_baseline(findings):
    """Rewrite the baseline IN PLACE: touch only the entry lines.

    Every non-entry line — the header, the section paragraphs that carry the
    argument for a whole group — is copied through byte for byte, and a surviving
    entry keeps both its reason and its position. Regeneration used to re-emit
    BASELINE_HEADER plus a globally sorted entry list, which silently deleted the
    prose: the paragraph explaining WHY thirteen cloud *Config types may report a
    field from the namesake *Status struct is the only thing that makes those
    thirteen `mispair:` tags checkable, and one `--update-baseline` removed it
    while keeping the tags. Reasons without their argument are decoration.

    A finding that is new gets `carried:` — deliberately the loudest tag, so a
    regeneration can never quietly launder a fresh defect into an exception.
    """
    current = {key for key, _kind, _detail in findings}
    kept: set[str] = set()
    out: list[str] = []
    dropped = 0

    if BASELINE.exists():
        for line in BASELINE.read_text(encoding="utf-8").splitlines():
            s = line.strip()
            if not s or s.startswith("#"):
                if s == REGEN_RULE:
                    continue      # re-added below only if there is something new
                out.append(line)
                continue
            key = s.partition("#")[0].strip()
            if key in current:
                out.append(line)
                kept.add(key)
            else:
                dropped += 1      # no longer reproduces: the ratchet closing
    else:
        out.append(BASELINE_HEADER.rstrip("\n"))

    while out and not out[-1].strip():
        out.pop()   # so removing the last regenerated block leaves no trailing gap

    fresh = sorted(current - kept)
    if fresh:
        out.append("")
        out.append(REGEN_RULE)
        out.extend(f"{key}  # {NEW_ON_REGEN}" for key in fresh)

    BASELINE.write_text("\n".join(out) + "\n", encoding="utf-8")
    try:
        shown = BASELINE.relative_to(ROOT)
    except ValueError:
        shown = BASELINE          # self-test points this at a temp file
    print(f"wrote {shown} — {len(current)} "
          f"entr{'y' if len(current) == 1 else 'ies'} "
          f"({len(fresh)} new, {dropped} no longer reproducing and removed)")


NOT_CHECKED = """\
not checked by construction (see script header for the full list):
  - VALUE domains (`oneof`, CHECK constraints, units, time zones) — names only
  - the other direction: backend fields no frontend reads
  - inline/anonymous response types: apiFetch<{ a: string }>(…)
  - WHICH ROUTE a type belongs to: pairing is by type name, not by endpoint
  - map[string]…/additionalProperties shapes"""


def main():
    ap = argparse.ArgumentParser(description=__doc__)
    ap.add_argument("--update-baseline", action="store_true",
                    help="rewrite the baseline from the current findings (reasons are preserved)")
    args = ap.parse_args()

    findings, skipped, unparsed, collisions, tally = analyse()

    if args.update_baseline:
        write_baseline(findings)
        return

    baseline = read_baseline()
    current = {key: (kind, detail) for key, kind, detail in findings}

    new = [(k, *current[k]) for k in sorted(current) if k not in baseline]
    stale = sorted(k for k in baseline if k not in current)
    unreasoned = sorted(k for k, r in baseline.items()
                        if not r.startswith(REASON_TAGS))
    unverified = sorted(k for k, r in baseline.items() if r.startswith("carried:"))

    print(f"FE↔BE field-name reconciliation — {tally.read} handwritten TS type "
          f"declaration(s) read, {tally.api_facing} API-facing.")
    # The denominator, spelled out. `displaced` is the number that used to be
    # invisible: declarations dropped in favour of a same-named one and counted as
    # covered anyway. It is derived from checked+skipped vs api_facing, so it can
    # only be 0 if every API-facing declaration really was looked at.
    print(f"declarations — checked:{tally.checked} displaced:{tally.displaced} "
          f"skipped:{tally.skipped + tally.unparsed}")
    print(f"fields       — checked:{tally.fields_checked} skipped:{tally.fields_skipped}")
    print(f"findings:{len(findings)} baselined:{len(baseline)}")
    print(f"unverified:{len(unverified)}")
    print(f"name-collisions:{len(collisions)}")

    failed = False

    if tally.displaced:
        # Unreachable while ts_types() returns every match and classify() makes one
        # pass per declaration. Kept as a hard failure because the only way to get
        # here is a regression to a rule that hides declarations — the exact defect
        # this accounting replaced.
        failed = True
        print(f"\nFAIL — {tally.displaced} declaration(s) were neither checked nor skipped "
              f"with a reason ({tally.parse_dropped} discarded before classification, "
              f"{tally.classify_gap} unaccounted for after). Coverage would be reported for "
              f"declarations nobody looked at; see the `WHY EVERY DECLARATION` note in "
              f"ts_types().")

    if new:
        failed = True
        print(f"\nFAIL — {len(new)} frontend field(s) the backend does not send or bind:")
        for _, kind, detail in new:
            print(f"  [{kind}] {detail}")
        print("\nFix: use the Go `json:` tag as the field name (it IS the wire format), or — if\n"
              "this is genuinely not a defect — add it to scripts/fe_be_fields_baseline.txt\n"
              "WITH A REASON. Regenerate with --update-baseline, then write the reason.")

    if stale:
        failed = True
        print(f"\nFAIL — {len(stale)} baseline entr{'y' if len(stale) == 1 else 'ies'} no longer "
              f"reproduce. An improvement was made and not recorded, so the gate would let the\n"
              f"regression back in silently. Remove them (--update-baseline):")
        for key in stale:
            print(f"  - {key}  # was: {baseline[key]}")

    if unreasoned:
        failed = True
        print(f"\nFAIL — {len(unreasoned)} baseline entr{'y' if len(unreasoned) == 1 else 'ies'} "
              f"carry a reason that does not start with one of {', '.join(REASON_TAGS)}:")
        for key in unreasoned:
            print(f"  - {key}  # {baseline[key]!r}")

    if unverified:
        # NOT a failure — but never silent either. These are live candidates of
        # exactly the class the gate exists for, accepted only so the gate could be
        # switched on at all. A green run that hides them would be the "reports
        # success for work it did not do" failure this gate was built against.
        print(f"\n::warning::check_fe_be_fields.py — {len(unverified)} baseline entr"
              f"{'y is' if len(unverified) == 1 else 'ies are'} tagged `carried:` "
              f"(UNVERIFIED candidate of this very defect class, not cleared).")
        print(f"\nunverified:{len(unverified)} — carried, not understood. This is debt:")
        for key in unverified:
            print(f"  - {key}  # {baseline[key]}")

    if collisions:
        # Not a failure (see ts_types()): every one of these declarations IS
        # checked, each against the Go struct of that name. Printed because the
        # ambiguity is real — pairing is by name, so two shapes under one name are
        # judged against one oracle, and the reader should know which files share.
        print(f"\n::warning::check_fe_be_fields.py — {len(collisions)} type name(s) are "
              f"declared more than once with different shapes. ALL declarations are checked "
              f"(no longer first-declaration-wins), each against the Go struct of that name.")
        for name, wheres in sorted(collisions.items()):
            print(f"  collision: {name} — {', '.join(wheres)}")

    if skipped or unparsed:
        # A skip is not a pass. Kept as a GitHub warning annotation (yellow, not
        # red) so the number stays visible in the checks UI rather than only in
        # the log — same convention as check_routes.py.
        print(f"\n::warning::check_fe_be_fields.py — {len(skipped) + len(unparsed)} frontend "
              f"field(s)/type(s) could NOT be attributed to a Go struct and were NOT checked; "
              f"coverage is a subset.")
        for s in sorted(set(unparsed))[:20]:
            print(f"  unparsed: {s}")
        if len(set(unparsed)) > 20:
            print(f"  … and {len(set(unparsed)) - 20} more unparsed declaration(s)")
        for s in sorted(set(skipped))[:20]:
            print(f"  unpaired: {s}")
        if len(set(skipped)) > 20:
            print(f"  … and {len(set(skipped)) - 20} more unpaired field(s)")

    print()
    print(NOT_CHECKED)

    if failed:
        sys.exit(1)
    print("\nOK — every API-facing frontend field is either a json tag of its namesake Go "
          "struct or a documented baseline exception.")


if __name__ == "__main__":
    main()
