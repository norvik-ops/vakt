#!/usr/bin/env python3
"""
Self-test for check_fe_be_fields.py (Gate G15 / K5-17).

Why this file exists: CLAUDE.md requires three acceptances per gate, not one —
green on the baseline, RED on a real regression NAMING the offender, and RED when
an improvement is lost. A gate whose red path was never exercised proves nothing
by being green, and this repo has been bitten by exactly that (check_routes.py
silently skipping a quarter of its input, check_price_tax_marking's universally
true exemption, the interface ratchet counting prose).

The units under test are the gate's two decision functions:
  go_structs()  — does it read the wire format correctly, INCLUDING embedded
                  structs, whose fields encoding/json promotes? (Without that,
                  ApprovalWithDetails alone produced eleven false findings.)
  ts_types()    — does it read a handwritten TS declaration's top-level fields,
                  and does it refuse to guess where it cannot parse?
plus the ratchet arithmetic that turns findings + baseline into an exit code.

The cases marked "K5-nn" reproduce a real finding from the KONV-K5 pass in
miniature, so the gate's red path is exercised against the actual defect shape
rather than a synthetic one.

Exit non-zero on any failed case.
"""

import pathlib
import sys
import tempfile

sys.path.insert(0, str(pathlib.Path(__file__).resolve().parent))

from check_fe_be_fields import (  # noqa: E402
    REASON_TAGS,
    Tally,
    classify,
    finding_key,
    go_structs,
    migration_columns,
    spec_keys,
    ts_types,
)

FAILURES = []
RUN = 0


def _tree(files: dict) -> pathlib.Path:
    """Materialise {relpath: content} in a temp dir and return its root."""
    td = tempfile.mkdtemp()
    root = pathlib.Path(td)
    for rel, content in files.items():
        p = root / rel
        p.parent.mkdir(parents=True, exist_ok=True)
        p.write_text(content, encoding="utf-8")
    return root


def case(name, ok, detail=""):
    global RUN
    RUN += 1
    if ok:
        print(f"PASS: {name}")
    else:
        FAILURES.append(f"{name}: {detail}")


# ── 1. The Go side reads the wire format, not the Go field names ─────────────

structs, all_tags = go_structs(_tree({
    "m/models.go": '''package m

type GitScan struct {
	ID           string `json:"id"`
	FindingCount int    `json:"finding_count"`
	ErrorMessage string `json:"error_message,omitempty"`
	Internal     string `json:"-"`
	Untagged     string
}
''',
}))
case("json tags are the field names, omitempty stripped",
     structs.get("GitScan") == {"id", "finding_count", "error_message"},
     f"got {structs.get('GitScan')}")
case("`json:\"-\"` and untagged fields are not wire fields",
     "-" not in all_tags and "Untagged" not in all_tags,
     f"got {all_tags}")

# ── 2. Embedded structs: encoding/json promotes them, so the gate must too ───
#      This is the ApprovalWithDetails shape. Without promotion the gate reports
#      every inherited field as a defect — eleven false positives from one type,
#      which is how a gate gets switched off instead of fixed.

structs, _ = go_structs(_tree({
    "m/a.go": '''package m

type Approval struct {
	ID     string `json:"id"`
	Status string `json:"status"`
}

type ApprovalWithDetails struct {
	Approval
	ControlRef string `json:"control_ref"`
}

type Deep struct {
	ApprovalWithDetails
	Extra string `json:"extra"`
}
''',
}))
case("embedded struct fields are promoted",
     structs.get("ApprovalWithDetails") == {"id", "status", "control_ref"},
     f"got {structs.get('ApprovalWithDetails')}")
case("embedding is resolved transitively",
     structs.get("Deep") == {"id", "status", "control_ref", "extra"},
     f"got {structs.get('Deep')}")

# ── 3. The TS side reads top-level fields only ───────────────────────────────

TREE1 = _tree({
    "modules/v/types.ts": '''export interface GitScan {
  id: string
  /** doc comment must not be read as a field */
  finding_count: number
  scanned_at?: string
  nested: {
    inner_should_not_count: string
  }
  // line comment: also_not_a_field: string
}

export type Alias = Omit<GitScan, 'id'>
''',
    "modules/v/hooks/useGitScans.ts": '''import { apiFetch } from '../../../api/client'
export function useGitScans() {
  return apiFetch<GitScan[]>('/vaktvault/git-scans')
}
''',
    "modules/v/marker.ts": '''export interface Marker {
    only_indented_four_spaces: string
}
''',
})
decls, api_named, unparsed, collisions, dropped = ts_types(TREE1, root=TREE1)


def fields_of(decls, name):
    """All field sets declared under `name`, one entry per declaration."""
    return [fl for _rel, n, fl in decls if n == name]


case("top-level TS fields are read, nested object keys are not",
     fields_of(decls, "GitScan") == [{"id", "finding_count", "scanned_at", "nested"}],
     f"got {fields_of(decls, 'GitScan')}")
case("comments are not read as fields",
     not any({"also_not_a_field", "inner_should_not_count"} & fl
             for fl in fields_of(decls, "GitScan")),
     f"got {fields_of(decls, 'GitScan')}")
case("a type used in an apiFetch generic is recognised as API-facing",
     "GitScan" in api_named, f"got {sorted(api_named)}")
# Two different "we cannot read this" paths, kept apart because conflating them
# made the original case vacuously true (an `or` whose second half always held):
#   `type Alias = Omit<…>` never matches TS_DECL_RE — no braces, so there is no
#     field list to misread and the name never enters `decls` at all;
#   `interface Marker { <nothing at two spaces> }` DOES match, yields no field and
#     is therefore recorded in `unparsed` — counted, never guessed at.
case("an aliased/mapped type is not treated as a declaration with fields",
     not fields_of(decls, "Alias") and not any("Alias" in u for u in unparsed),
     f"decls={fields_of(decls, 'Alias')} unparsed={unparsed}")
case("a matched declaration with no readable field list is counted, not guessed",
     any("Marker" in u for u in unparsed) and not fields_of(decls, "Marker"),
     f"unparsed={unparsed}")
case("a tree with no duplicate names reports no collisions",
     collisions == {}, f"got {collisions}")
case("every matched declaration is returned: nothing discarded before classification",
     dropped == 0, f"dropped={dropped}")

# ── 4. useQuery/useMutation generics also name wire types ────────────────────

TREE2 = _tree({
    "modules/v/hooks/useAssets.ts": '''import { useMutation, useQuery } from '@tanstack/react-query'
export function useAssets() { return useQuery<PaginatedResponse<Asset>>({}) }
export function useCreateAsset() {
  return useMutation<Asset, Error, CreateAssetInput>({})
}
''',
})
_, api_named, _, _, _ = ts_types(TREE2, root=TREE2)
case("useMutation's request type is API-facing (K5-16: CreateAssetInput.target)",
     {"Asset", "CreateAssetInput"} <= api_named, f"got {sorted(api_named)}")

# ── 5. The spec/migration nets that suppress orphan false positives ─────────

root = _tree({
    "backend/internal/shared/apidocs/openapi.yaml": """
components:
  schemas:
    Foo:
      properties:
        spec_only_field: { type: string }
""",
    "backend/db/migrations/001_x.up.sql": """
CREATE TABLE ck_things (
    id            UUID PRIMARY KEY,
    column_only   TEXT NOT NULL
);
ALTER TABLE ck_things ADD COLUMN IF NOT EXISTS added_later TEXT;
""",
})
case("openapi.yaml keys are collected",
     "spec_only_field" in spec_keys(root / "backend/internal/shared/apidocs/openapi.yaml"))
cols = migration_columns(root / "backend/db/migrations")
case("migration columns are collected (CREATE TABLE and ADD COLUMN)",
     {"column_only", "added_later"} <= cols, f"got {sorted(cols)}")


# ── 6. The ratchet arithmetic — the three acceptances, as decisions ──────────
#
# Modelled directly, because this is the logic that turns findings into an exit
# code and it is the part a refactor can silently invert.

def ratchet(findings: set, baseline: dict):
    """Mirror of main()'s decision: (new, stale, unreasoned, unverified)."""
    new = sorted(f for f in findings if f not in baseline)
    stale = sorted(k for k in baseline if k not in findings)
    unreasoned = sorted(k for k, r in baseline.items() if not r.startswith(REASON_TAGS))
    unverified = sorted(k for k, r in baseline.items() if r.startswith("carried:"))
    return new, stale, unreasoned, unverified


BASE = {"AWSConfig.is_configured": "mispair: GET returns the *Status struct",
        "Webhook.secret": "carried: emits has_secret"}

new, stale, unreasoned, unverified = ratchet(set(BASE), BASE)
case("ACCEPTANCE 1 — green on the baseline", not new and not stale and not unreasoned,
     f"new={new} stale={stale} unreasoned={unreasoned}")
case("ACCEPTANCE 1 — but `carried:` entries stay visible", unverified == ["Webhook.secret"],
     f"got {unverified}")

# A real drift, re-introduced: the K5-11 shape.
new, stale, _, _ = ratchet(set(BASE) | {"GitScan.result_count"}, BASE)
case("ACCEPTANCE 2 — RED on a re-introduced drift, naming it",
     new == ["GitScan.result_count"] and not stale, f"new={new}")

# An improvement made but not written down: Webhook.secret was fixed, the
# baseline still lists it. If that were tolerated, the gate would let the
# regression back in silently forever.
new, stale, _, _ = ratchet({"AWSConfig.is_configured"}, BASE)
case("ACCEPTANCE 3 — RED when a fixed finding is left in the baseline",
     stale == ["Webhook.secret"] and not new, f"stale={stale}")

# A baseline line with no tag is not an explanation.
_, _, unreasoned, _ = ratchet({"X.y"}, {"X.y": "because"})
case("a reason without one of the required tags fails", unreasoned == ["X.y"],
     f"got {unreasoned}")
_, _, unreasoned, _ = ratchet({"X.y"}, {"X.y": ""})
case("an empty reason fails", unreasoned == ["X.y"], f"got {unreasoned}")


# ── 7. End to end on a miniature repo: the actual K5-01 defect ──────────────

def run_gate(tree_root):
    """Run the SHIPPED classifier over a miniature repo.

    Calls classify() rather than re-implementing it: a copy of the decision logic
    in the test can drift from the one in CI and then prove nothing, which is the
    same failure mode as a gate that does not run.
    Returns (findings, skipped, tally, collisions); findings are (key, kind).
    """
    import check_fe_be_fields as g
    structs, all_tags = g.go_structs(tree_root / "backend")
    known = all_tags | g.spec_keys(tree_root / "spec.yaml") | g.migration_columns(tree_root / "mig")
    decls, api_named, unparsed, collisions, dropped = g.ts_types(
        tree_root / "frontend" / "src", root=tree_root)
    tally = Tally()
    findings, skipped = classify(structs, known, decls, api_named, tally)
    tally.unparsed = len(unparsed)
    tally.parse_dropped = dropped
    return [(k, kind) for k, kind, _detail in findings], skipped, tally, collisions


def findings_for(tree_root):
    """Bare (Type.field, kind) pairs — the declaring file dropped, for the cases
    that are about the classification and not about the key."""
    findings, _, _, _ = run_gate(tree_root)
    return [(k.split("::", 1)[1], kind) for k, kind in findings]


GO = '''package policy

type SoADedicatedEntry struct {
	ControlRef   string `json:"control_ref"`
	ControlGroup string `json:"control_group"`
}
'''
HOOK = '''import { apiFetch } from '../api/client'
export const load = () => apiFetch<SoADedicatedEntry[]>('/vaktcomply/soa/entries')
'''

broken = findings_for(_tree({
    "backend/m/models.go": GO,
    "frontend/src/modules/c/types.ts": "export interface SoADedicatedEntry {\n"
                                       "  control_ref: string\n  group: string\n}\n",
    "frontend/src/modules/c/hooks/use.ts": HOOK,
}))
case("K5-01 end to end: `group` instead of `control_group` is reported",
     ("SoADedicatedEntry.group", "paired") in broken, f"got {broken}")

fixed = findings_for(_tree({
    "backend/m/models.go": GO,
    "frontend/src/modules/c/types.ts": "export interface SoADedicatedEntry {\n"
                                       "  control_ref: string\n  control_group: string\n}\n",
    "frontend/src/modules/c/hooks/use.ts": HOOK,
}))
case("K5-01 end to end: the repaired shape is clean (no false positive)",
     fixed == [], f"got {fixed}")

# K5-15: a field that exists nowhere in the backend at all, on a type with no
# namesake struct — the orphan tier, which needs no pairing and cannot mispair.
orphan = findings_for(_tree({
    "backend/m/models.go": "package p\n\ntype Other struct {\n\tID string `json:\"id\"`\n}\n",
    "frontend/src/modules/c/types.ts": "export interface Framework {\n"
                                       "  id: string\n  control_count?: number\n}\n",
}))
case("K5-15 end to end: an orphan field with no namesake struct is reported",
     ("Framework.control_count", "orphan") in orphan, f"got {orphan}")

# And the counterpart: a field that DOES exist somewhere in the backend but on an
# unpairable type must NOT be reported — that is the 42-mispairings trap.
quiet = findings_for(_tree({
    "backend/m/models.go": "package p\n\ntype Other struct {\n\tName string `json:\"asset_name\"`\n}\n",
    "frontend/src/modules/c/types.ts": "export interface SomethingElse {\n  asset_name: string\n}\n",
}))
case("an unpairable field that exists elsewhere in the backend is NOT reported",
     quiet == [], f"got {quiet}")


# ── 8. Duplicate type names — the displacement bug (REV-K5 R2) ───────────────
#
# The shape that forced this: a page-local 3-field view model and the real 21-field
# wire type both named `Control`. Sorted by path the page file comes first, so the
# old first-declaration-wins rule dropped the WIRE type unchecked while the name
# still counted toward the API-facing total — and real drift injected into the
# dropped declaration came out exit 0 (measured, REV-K5 §4). No self-test case
# covered duplicate names, which is why the hole survived 21 green cases.
#
# The three properties that close it, each asserted below:
#   every declaration of a duplicated name is checked (nothing displaced),
#   the finding KEY names the declaring file (so one baseline line cannot silence
#   a declaration nobody wrote a reason about),
#   and the collision itself is reported rather than swallowed.

DUP_TREE = _tree({
    "backend/m/models.go": '''package policy

type Control struct {
	ID           string `json:"id"`
	Title        string `json:"title"`
	ControlGroup string `json:"control_group"`
}
''',
    # Sorts FIRST — the old winner. A narrow page-local view model.
    "frontend/src/modules/c/pages/ChecklistPage.tsx": '''import { apiFetch } from '../../../api/client'
interface Control {
  id: string
  title: string
  module: string
}
export const load = () => apiFetch<Control[]>('/vaktcomply/controls')
''',
    # Sorts SECOND — the real wire type, dropped unchecked by the old rule.
    # `group` is the K5-01 drift; `module` is the KONV-K5 §4.6 candidate.
    "frontend/src/modules/c/types.ts": '''export interface Control {
  id: string
  group: string
  module: string
}
''',
})
dup_findings, _dup_skipped, dup_tally, dup_collisions = run_gate(DUP_TREE)
dup_keys = {k for k, _ in dup_findings}

case("duplicate name: BOTH declarations are checked, none displaced",
     (dup_tally.checked, dup_tally.displaced) == (2, 0),
     f"checked={dup_tally.checked} displaced={dup_tally.displaced} "
     f"api_facing={dup_tally.api_facing} skipped={dup_tally.skipped}")
case("duplicate name: drift in the LATER declaration is reported "
     "(exit 0 under first-declaration-wins)",
     finding_key("frontend/src/modules/c/types.ts", "Control", "group") in dup_keys,
     f"got {sorted(dup_keys)}")
case("duplicate name: the earlier declaration is still reported (coverage is monotone)",
     finding_key("frontend/src/modules/c/pages/ChecklistPage.tsx", "Control", "module")
     in dup_keys, f"got {sorted(dup_keys)}")
case("duplicate name: the same field on two declarations yields two DISTINCT keys, "
     "so one baseline line cannot silence both",
     len({finding_key("frontend/src/modules/c/types.ts", "Control", "module"),
          finding_key("frontend/src/modules/c/pages/ChecklistPage.tsx", "Control", "module")}
         & dup_keys) == 2,
     f"got {sorted(dup_keys)}")
case("duplicate name: nothing else is invented (exactly the three drifted fields)",
     len(dup_findings) == 3, f"got {sorted(dup_keys)}")
case("duplicate name with differing shapes is reported as a collision, not skipped",
     dup_collisions.get("Control") == ["frontend/src/modules/c/pages/ChecklistPage.tsx",
                                       "frontend/src/modules/c/types.ts"],
     f"got {dup_collisions}")

# What makes `displaced` trustworthy: it sums BOTH drop sites. The downstream
# identity alone is not enough — a rule that discards during parsing shrinks
# api_facing along with checked, so the identity still holds while declarations
# vanish. That is measured: simulating first-declaration-wins leaves
# api_facing == checked == 1 and only parse_dropped moves.
case("the denominator adds up: checked + skipped == API-facing (no classify gap)",
     dup_tally.classify_gap == 0,
     f"{dup_tally.checked}+{dup_tally.skipped} != {dup_tally.api_facing}")
case("displaced counts BOTH drop sites, so a parse-time discard cannot hide",
     dup_tally.displaced == dup_tally.parse_dropped + dup_tally.classify_gap
     and dup_tally.displaced == 0,
     f"displaced={dup_tally.displaced} parse_dropped={dup_tally.parse_dropped} "
     f"classify_gap={dup_tally.classify_gap}")

# Same shape twice under one name is not a collision (nothing ambiguous to warn
# about) but both declarations are still checked.
SAME_TREE = _tree({
    "backend/m/models.go": "package p\n\ntype Gap struct {\n\tID string `json:\"id\"`\n}\n",
    "frontend/src/modules/a/types.ts": "export interface Gap {\n  id: string\n  note: string\n}\n",
    "frontend/src/modules/b/types.ts": "export interface Gap {\n  id: string\n  note: string\n}\n",
})
same_findings, _s, same_tally, same_collisions = run_gate(SAME_TREE)
case("identical shapes under one name: not a collision, both still checked",
     same_collisions == {} and same_tally.checked == 2 and len(same_findings) == 2,
     f"collisions={same_collisions} checked={same_tally.checked} findings={same_findings}")


# ── 9. --update-baseline must not delete the argument ────────────────────────
#
# The reason tags are only checkable because of the prose above them: thirteen
# `mispair:` lines are defensible only via the paragraph explaining that the GET
# returns the namesake *Status struct. Regeneration used to re-emit a fixed header
# plus a sorted entry list, so one --update-baseline deleted every section
# paragraph while keeping the tags — reasons without their argument.

import check_fe_be_fields as gate  # noqa: E402

_bl_dir = _tree({"baseline.txt": """\
# HEADER LINE THAT MUST SURVIVE
#
# ── section paragraph carrying the argument for the group below ──
# second line of that argument
a.ts::Kept.field  # mispair: this reason and this position must survive
b.ts::Fixed.field  # mispair: no longer reproduces, must be removed
"""})
_bl = _bl_dir / "baseline.txt"
_saved_baseline = gate.BASELINE
gate.BASELINE = _bl
try:
    gate.write_baseline([
        ("a.ts::Kept.field", "paired", "…"),
        ("c.ts::Brand.new", "paired", "…"),
    ])
    written = _bl.read_text()
finally:
    gate.BASELINE = _saved_baseline

case("--update-baseline keeps the header and the section argument verbatim",
     "# HEADER LINE THAT MUST SURVIVE" in written
     and "# ── section paragraph carrying the argument for the group below ──" in written
     and "# second line of that argument" in written, f"got:\n{written}")
case("--update-baseline keeps a surviving entry's reason and position",
     "a.ts::Kept.field  # mispair: this reason and this position must survive" in written
     # find() and not index(): a missing substring must FAIL the case, not raise
     # and abort the remaining cases.
     and -1 < written.find("HEADER LINE") < written.find("a.ts::Kept.field"),
     f"got:\n{written}")
case("--update-baseline removes an entry that no longer reproduces (ratchet closes)",
     "b.ts::Fixed.field" not in written, f"got:\n{written}")
case("--update-baseline files a brand-new finding as `carried:`, never as cleared",
     "c.ts::Brand.new" in written
     and written.split("c.ts::Brand.new")[1].lstrip().startswith("# carried:"),
     f"got:\n{written}")


if FAILURES:
    print("\nFAILED:")
    for f in FAILURES:
        print(f"  - {f}")
    print(f"\n{len(FAILURES)} of {RUN} case(s) FAILED")
    sys.exit(1)

print(f"\nALL TESTS PASSED ({RUN} cases)")
