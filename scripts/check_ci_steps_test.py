#!/usr/bin/env python3
"""Self-test for check_ci_steps.py — check E (neutralization), R-01.

Why this file exists
--------------------
check_ci_steps.py had no self-test at all. Check E was added for K2-05 with the
claim that it "grabs the class, not the finding"; the review (R-01) showed it
grabbed three literal spellings — `|| echo "skipped"` and `set +e` + `exit 0`
both left two required gates neutralized and the gate at rc=0. A claim about a
CLASS that only a hand-run can check is exactly the shape this repo keeps
paying for, so the class is pinned here, in both directions:

  * FORMS THAT MUST BE CAUGHT — every mechanism by which a `run` body can end up
    unable to report a failure.
  * FORMS THAT MUST STAY GREEN — the reason E is not simply `grep '|| '`. A gate
    that cries wolf on `rm -f x || true` gets switched off within a month, and
    then the K2-05 hole is back with a nicer commit message.

Both halves matter: without the second half, widening the matcher would look
like progress right up until someone deletes the gate.

Exit non-zero on any failed case.
"""

import pathlib
import sys

sys.path.insert(0, str(pathlib.Path(__file__).resolve().parent))

from check_ci_steps import (  # noqa: E402
    RENAMED_STEPS,
    is_const_false,
    neutralization_of,
    run_body_cannot_fail,
)

ROOT = pathlib.Path(__file__).resolve().parent.parent

FAILURES = []


def check(label, body, want_neutral):
    got = run_body_cannot_fail(body) is not None
    if got != want_neutral:
        FAILURES.append(
            f"{label}: expected neutralized={want_neutral}, got {got} for body:\n"
            + "\n".join("      | " + ln for ln in body.splitlines())
        )
        return False
    return True


# ── 1. Forms that MUST be caught ────────────────────────────────────────────
NEUTRALIZING = {
    "|| true (the K2-05 original)": "python3 scripts/check_routes.py || true",
    "|| : (the no-op builtin)": "python3 scripts/check_routes.py || :",
    "|| exit 0": "python3 scripts/check_routes.py || exit 0",
    # R-01: `echo` reports success just as reliably as `true`.
    "|| echo (R-01)": 'python3 scripts/check_routes.py || echo "route check skipped"',
    "|| <any other succeeding command> (R-01)":
        "python3 scripts/check_routes.py || logger -t ci gate-skipped",
    "|| true behind a line continuation": "go build ./... \\\n  || true",
    # R-01: errexit off means the last line decides; a forced zero exit makes
    # everything before it unreachable from the step's status.
    "set +e + exit 0 (R-01)":
        "set +e\npython3 scripts/check_user_role_insert.py\nexit 0",
    "set +o errexit + exit 0 (R-01)":
        "set +o errexit\npython3 scripts/check_user_role_insert.py\nexit 0",
    "set +ex + a trailing echo (R-01)":
        "set +ex\npython3 scripts/check_user_role_insert.py\necho fertig",
    "a body that is only an echo (R-01)": 'echo "TODO: re-enable this gate"',
    "every line suppressed, several lines":
        "python3 a.py || true\npython3 b.py || echo skip",
    # N-01: measured GREEN at gate level before this round — a required step
    # written this way could not go red and G-CI reported OK.
    "| tee without pipefail (N-01)":
        "python3 scripts/check_routes.py | tee gate.log",
    "| any succeeding sink without pipefail (N-01)":
        "python3 scripts/check_routes.py | cat",
    "backgrounded with & (N-01)": "python3 scripts/check_routes.py &",
    "if false; then … fi, one line (N-01)":
        "if false; then python3 scripts/check_routes.py; fi",
    "if false over several lines (N-01)":
        "if false; then\n  python3 scripts/check_routes.py\nfi",
    "while false; do … done (N-01)":
        "while false; do\n  python3 scripts/check_routes.py\ndone",
    "if [ 1 = 0 ] — a constant-false test (N-01)":
        "if [ 1 = 0 ]; then\n  python3 scripts/check_routes.py\nfi",
    "|| (exit 0) — the code hid behind a paren (N-01)":
        "python3 scripts/check_routes.py || (exit 0)",
}
ok = all([check(label, body, True) for label, body in NEUTRALIZING.items()])
print(("PASS: " if ok else "FAIL: ")
      + f"{len(NEUTRALIZING)} neutralization forms are caught (K2-05 + R-01)")

# ── 2. Forms that MUST stay green ───────────────────────────────────────────
#      This half is what stops the fix for R-01 from becoming a new defect.
LEGITIMATE = {
    "a plain gate call": "python3 scripts/check_routes.py",
    "a real script with set -euo pipefail": "set -euo pipefail\nnpm test",
    "|| exit 1 — the tail itself fails": "python3 x.py || exit 1",
    "|| false — the tail itself fails": "python3 x.py || false",
    "|| { …; exit 2; } — the tail itself fails":
        'python3 x.py || { echo "gate failed"; exit 2; }',
    "|| exit — inherits the failing status": "python3 x.py || exit",
    "|| exit $? — inherits the failing status": "python3 x.py || exit $?",
    # The named limit (blind spot 1): partial suppression stays green ON PURPOSE.
    "one legitimate `rm -f x || true` inside real work":
        "rm -f /tmp/x || true\npython3 scripts/check_routes.py",
    "set +e without a forced zero exit — the last command still decides":
        "set +e\npython3 scripts/check_routes.py",
    "a trailing exit 0 WITHOUT set +e — bash -e aborts before reaching it":
        "python3 scripts/check_routes.py\nexit 0",
    "set +e with a real exit code at the end":
        "set +e\npython3 x.py\nrc=$?\nexit $rc",
    "an empty body": "",
    # ── N-01: the counter-half of the three new mechanisms. Each of these is a
    #    shape that exists in this repo's real ci.yml today; a matcher that goes
    #    red here gets switched off, and then the whole class is back.
    "a pipe WITH set -o pipefail — the pipeline's failure survives":
        "set -euo pipefail\npython3 scripts/check_routes.py | tee gate.log",
    "a pipe consumed by `if` — the status goes to the construct, not the step":
        "if ! head -1 \"$f\" | grep -q 'migrate: no transaction'; then\n"
        "  echo bad\n  exit 1\nfi",
    "a pipe inside $( ) — the status stays in the substitution":
        "TOTAL=$(go tool cover -func=coverage.out | tail -1 | awk '{print $3}')\n"
        "echo \"$TOTAL\" | grep -qE '^[0-9]+\\.[0-9]+%$' || { echo parse; exit 1; }",
    "a `|` inside quotes is not a pipe":
        "grep -E 'NOW\\(\\)|CURRENT_DATE' db/migrations/*.sql && exit 1",
    "one piped line inside real work (blind spot 1, on purpose)":
        "npm ci\npython3 scripts/check_routes.py | tee gate.log",
    "2>&1 is not a background &": "python3 x.py 2>&1",
    "&& is not a background &": "python3 a.py && python3 b.py",
    # ── The NAMED blind spot 4: this one IS a real neutralization and stays
    #    green ON PURPOSE. Pinned here so that the printed limit is a tested
    #    claim rather than a sentence, and so that a future widening has to
    #    delete this case deliberately instead of by accident.
    "if <gate>; then …; fi — blind spot 4, green by decision not by oversight":
        "if python3 scripts/check_routes.py; then\n  echo ok\nfi",
    "if with a REAL condition around real work (ci.yml has this verbatim)":
        "if command -v shellcheck &>/dev/null; then\n"
        "  find scripts/ -name '*.sh' -print0 | xargs -0 shellcheck\n"
        "else\n  echo \"shellcheck not installed\"\n  exit 1\nfi",
}
ok = all([check(label, body, False) for label, body in LEGITIMATE.items()])
print(("PASS: " if ok else "FAIL: ")
      + f"{len(LEGITIMATE)} legitimate forms stay green (no cry-wolf)")

# ── 3. The other two mechanisms of check E, at node level ───────────────────
CONST_FALSE = [False, "false", "'false'".strip("'"), "${{ false }}", "${{false}}",
               "off", "no"]
ok = all(is_const_false(v) for v in CONST_FALSE)
if not ok:
    FAILURES.append(f"constant-false `if:` values not all recognized: {CONST_FALSE}")
print(("PASS: " if ok else "FAIL: ")
      + f"{len(CONST_FALSE)} constant-false `if:` spellings are neutralization")

REAL_CONDITIONS = ["always()", "github.event_name == 'push'", "${{ true }}", "true"]
ok = not any(is_const_false(v) for v in REAL_CONDITIONS)
if not ok:
    FAILURES.append(f"a real `if:` condition was read as constant-false: {REAL_CONDITIONS}")
print(("PASS: " if ok else "FAIL: ")
      + "real `if:` conditions are counted and printed, not failed")

NODE_CASES = [
    ("continue-on-error: true", {"continue-on-error": True,
                                 "run": "python3 x.py"}, True),
    ("continue-on-error: 'true'", {"continue-on-error": "true",
                                   "run": "python3 x.py"}, True),
    ("if: false", {"if": False, "run": "python3 x.py"}, True),
    ("continue-on-error: false", {"continue-on-error": False,
                                  "run": "python3 x.py"}, False),
    ("a healthy step", {"run": "python3 x.py"}, False),
    ("if: always()", {"if": "always()", "run": "python3 x.py"}, False),
]
ok = True
for label, node, want in NODE_CASES:
    got = neutralization_of(node) is not None
    if got != want:
        FAILURES.append(f"node case {label}: expected {want}, got {got}")
        ok = False
print(("PASS: " if ok else "FAIL: ")
      + f"{len(NODE_CASES)} job/step-level neutralization cases")

# ── 4. Deklarierte Umbenennungen (Pruefung D) ──────────────────────────────
# D paart Schritte ueber den Namen und kann eine Umbenennung nicht von einer
# Loeschung unterscheiden. RENAMED_STEPS erklaert sie — aber nur, wenn der
# Befehl derselbe bleibt. Eine "Umbenennung", die auch den Befehl aendert, ist
# eine Loeschung im Kostuem und MUSS rot bleiben. Genau dieser Fall trat beim
# PR nach main am 2026-08-26 auf (Image-Tags -> Image-Referenzen).
RENAME_CASES = []
for (wf, jid, old), (new, why) in RENAMED_STEPS.items():
    RENAME_CASES.append((f"{old!r} -> {new!r}", wf, jid, old, new, why))

ok = True
for label, wf, jid, old, new, why in RENAME_CASES:
    if not why.strip():
        FAILURES.append(f"Umbenennung {label} hat keine Begruendung"); ok = False
    if old == new:
        FAILURES.append(f"Umbenennung {label} benennt nichts um"); ok = False
    # Der neue Name MUSS im echten Workflow stehen, sonst ist der Eintrag Fiktion.
    text = (ROOT / wf).read_text(encoding="utf-8")
    if f"name: {new}" not in text:
        FAILURES.append(f"Umbenennung {label}: {new!r} steht nicht in {wf}"); ok = False
print(("PASS: " if ok else "FAIL: ")
      + f"{len(RENAME_CASES)} deklarierte Umbenennung(en): begruendet, nicht leer, Ziel existiert")

if FAILURES:
    print("\nFAILED:")
    for f in FAILURES:
        print(f"  - {f}")
    sys.exit(1)

print(f"\nALL TESTS PASSED ({len(NEUTRALIZING) + len(LEGITIMATE) + len(NODE_CASES) + len(RENAME_CASES) + 2} cases)")
