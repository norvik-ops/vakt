#!/usr/bin/env python3
"""
Self-test for check_image_tags.py (Test-/CI-Image-Flaeche, R1-INT-02).

Why this file exists: the Go side of that gate is a HAND-WRITTEN parser — a
comment stripper, a literal scanner, and a balanced-paren argument splitter.
That makes it the most fragile part of the gate, and CLAUDE.md is explicit that
a gate proves nothing by being green: it needs the red path exercised too. This
repo has been bitten repeatedly by gates that silently skipped their input
(check_routes.py dropping a quarter of the frontend calls), that counted prose
instead of code (the interface ratchet), or that INVENTED findings out of
runtime-built strings (check_openapi_coverage hallucinating routes).

Every case below is a real bypass or false positive that this gate actually had:

  · plain `ctx` first argument                   — caught from the start
  · `context.Background()` / `t.Context()`       — silently invisible (F3 review)
  · a `"/*"` literal earlier in the file         — swallowed the rest of the file
  · example code inside a raw string             — INVENTED a reference (delta review)
  · an unrelated struct field named `Image`      — false positive (delta review)

The units under test are the two functions the gate is actually made of:
`scan_go_source()` (one Go source -> refs/skipped/occurrences) and the parser
primitives it stands on.

Exit non-zero on any failed case.
"""

import pathlib
import sys

sys.path.insert(0, str(pathlib.Path(__file__).resolve().parent))

from check_image_tags import (  # noqa: E402
    call_args,
    scan_go_source,
    strip_go_comments,
)

FAILURES: list[str] = []
COUNT = 0

# Every source needs this import, otherwise the file is out of scope by design.
IMPORTS = '''package p

import (
\t"github.com/testcontainers/testcontainers-go"
\t"github.com/testcontainers/testcontainers-go/modules/postgres"
)
'''

PINNED = "postgres:16.14-alpine@sha256:" + "5" * 64


def case(name, body, expect_refs, expect_skipped, consts=None):
    """expect_refs: set of image strings. expect_skipped: number of skips."""
    global COUNT
    COUNT += 1
    refs, skipped, _occ = scan_go_source("x_test.go", IMPORTS + body, consts or {})
    got = set(refs)
    if got != set(expect_refs):
        FAILURES.append(f"{name}: refs {sorted(got)} != {sorted(expect_refs)}")
        return
    if len(skipped) != expect_skipped:
        FAILURES.append(
            f"{name}: {len(skipped)} skips != {expect_skipped} — {skipped}")


# ── The four bypasses: each MUST yield the image, so the digest rule can fire ──

case("plain ctx",
     'func f() { postgres.Run(ctx, "postgres:15-alpine") }',
     {"postgres:15-alpine"}, 0)

case("context.Background() as first arg",
     'func f() { postgres.Run(context.Background(), "postgres:15-alpine") }',
     {"postgres:15-alpine"}, 0)

case("t.Context() as first arg",
     'func f() { postgres.Run(t.Context(), "postgres:15-alpine") }',
     {"postgres:15-alpine"}, 0)

case("block-comment opener in a string literal does not blind the file",
     'var g = "/*"\nfunc f() { postgres.Run(ctx, "postgres:15-alpine") }',
     {"postgres:15-alpine"}, 0)

# ── The two false-positive vectors: MUST NOT invent a finding ──

case("example code in a raw string is not a reference",
     'var doc = `postgres.Run(ctx, "postgres:15-alpine")`',
     set(), 0)

case("example code in an interpreted string is not a reference",
     'func f() { t.Log("call postgres.Run(ctx, \\"postgres:15-alpine\\") first") }',
     set(), 0)

# Covers the literal-span guard on the OTHER matcher. Without it this raw string
# yields a phantom reference: brace_blocks happily finds the `ContainerRequest{`
# inside the string, so the `Image:` within it looks like a real field.
case("a ContainerRequest inside a raw string is not a reference",
     'var doc = `testcontainers.ContainerRequest{Image: "postgres:15-alpine"}`',
     set(), 0)

case("Image field on an unrelated struct is named, not flagged",
     'type scanTarget struct{ Image string }\n'
     'func f() { _ = scanTarget{Image: "postgres:15-alpine"} }',
     set(), 1)

# ── Real ContainerRequest forms still resolve ──

case("Image inside ContainerRequest resolves",
     'func f() { _ = testcontainers.ContainerRequest{Image: "postgres:15-alpine"} }',
     {"postgres:15-alpine"}, 0)

case("Image inside GenericContainerRequest resolves",
     'func f() { _ = testcontainers.GenericContainerRequest{\n'
     '\tContainerRequest: testcontainers.ContainerRequest{Image: "postgres:15-alpine"},\n'
     '} }',
     {"postgres:15-alpine"}, 0)

case("identifier resolves through a package const",
     'func f() { postgres.Run(ctx, imagePostgres) }',
     {PINNED}, 0, consts={"imagePostgres": PINNED})

case("unresolvable image argument is counted, not dropped",
     'func f() { postgres.Run(ctx, pgImage()) }',
     set(), 1)

case("two containers in one file are both counted",
     'func f() {\n'
     '\tpostgres.Run(ctx, "postgres:15-alpine")\n'
     '\tpostgres.Run(ctx, "postgres:14-alpine")\n'
     '}',
     {"postgres:15-alpine", "postgres:14-alpine"}, 0)

# Out-of-scope really means out of scope: no import, no scan. Written without
# the `case()` helper because that one prepends the import that must be absent.
COUNT += 1
_r, _s, _o = scan_go_source(
    "x_test.go",
    'package p\nfunc f() { postgres.Run(ctx, "postgres:15-alpine") }',
    {})
if _r or _s:
    FAILURES.append(f"no-testcontainers-import: expected nothing, got {_r} / {_s}")

# ── Parser primitives ──

STRIP_CASES = [
    ('a := "/*"\nImage: "postgres:15"', 'postgres:15', None),
    ('// Image: "postgres:99"\nx := 1', None, 'postgres:99'),
    ('/* Image: "postgres:98" */\ny := 2', None, 'postgres:98'),
    ('s := "// not a comment"\nz := 3', '// not a comment', None),
    ('s := `raw /* still */ text`\nw := 4', 'raw /* still */ text', None),
    ("c := '\\''\nImage: \"postgres:97\"", 'postgres:97', None),
]
for src, must, mustnot in STRIP_CASES:
    COUNT += 1
    out = strip_go_comments(src)
    if must is not None and must not in out:
        FAILURES.append(f"strip: lost {must!r} from {src!r} -> {out!r}")
    if mustnot is not None and mustnot in out:
        FAILURES.append(f"strip: kept {mustnot!r} from {src!r} -> {out!r}")

ARG_CASES = [
    ('postgres.Run(ctx, "img")', ["ctx", '"img"']),
    ('postgres.Run(context.Background(), "img")', ["context.Background()", '"img"']),
    ('postgres.Run(t.Context(), "img", postgres.WithDatabase("d"))',
     ["t.Context()", '"img"', 'postgres.WithDatabase("d")']),
    ('postgres.Run(ctx,\n\t"img",\n\topt(),\n)', ["ctx", '"img"', "opt()", ""]),
    ('postgres.Run(f(a, b), "img")', ["f(a, b)", '"img"']),
    ('postgres.Run(ctx, "has,comma")', ["ctx", '"has,comma"']),
]
for src, want in ARG_CASES:
    COUNT += 1
    got = call_args(src, src.index("("))
    if got != want:
        FAILURES.append(f"call_args({src!r}): {got} != {want}")

COUNT += 1
if call_args('postgres.Run(ctx, "img"', 12) is not None:
    FAILURES.append("call_args: unterminated call must return None")

if FAILURES:
    print("FAILED:")
    for f in FAILURES:
        print(f"  - {f}")
    sys.exit(1)

print(f"ALL TESTS PASSED ({COUNT} cases)")
