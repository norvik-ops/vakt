#!/usr/bin/env python3
# S5/G12 (S134): cross-module evidence-flow wiring gate ("Evidence-Flow").
#
# The defect class this closes is one level deeper than G7 (worker task
# wiring): a function can be perfectly correct — it builds a
# `crossevidence.NewRecordEvidenceTask(...)` and enqueues it — and still never
# run, because nothing in the codebase ever calls that function. That is
# exactly what happened to `RunORP3EvidenceSync` in S5/W3: the enqueue code
# was fine, the function itself was dead. G7 cannot see this (the Asynq task
# type it enqueues, vaktcomply:record_evidence, has a real producer *and*
# consumer elsewhere) — only tracing the Go call graph one level up from the
# enqueue site does.
#
# This is a static, best-effort check: it maps each occurrence of
# `crossevidence.NewRecordEvidenceTask(` to its enclosing Go function, then
# requires at least one call site for that function name elsewhere in
# production (non-test) code. It also re-confirms the base invariant that
# `crossevidence.TaskRecordEvidence` itself has a registered worker consumer.
#
# False positives are possible if two unrelated types both define a method
# with the same name (the check does not do real type resolution) — such a
# case masks a real EVENT_UNWIRED finding as green, so treat "OK" as strong
# evidence, not proof, and re-check manually before trusting a large refactor.
# False negatives (flagging a wired function as dead) are not expected since
# evidence-producing function names in this codebase are distinctive; if one
# ever occurs, allowlist it here with a comment, don't loosen the regex.

import re
import sys
import pathlib
import subprocess

ROOT = pathlib.Path(__file__).resolve().parent.parent

FUNC_DECL_RE = re.compile(r'^func\s+(?:\([^)]*\)\s*)?([A-Za-z_][A-Za-z0-9_]*)\s*\(')
ENQUEUE_CALL_RE = re.compile(r'crossevidence\.NewRecordEvidenceTask\(')

# function name -> reason it legitimately has no static caller (e.g. only
# invoked via reflection/dispatch table, or intentionally exported SDK API).
ALLOWLIST = {}


def go_files(include_tests):
    out = subprocess.run(
        ["git", "-C", str(ROOT), "ls-files", "-z", "--others", "--exclude-standard", "--cached", "backend"],
        capture_output=True, check=True,
    ).stdout
    paths = [p for p in out.decode("utf-8", "replace").split("\0") if p]
    return sorted(
        ROOT / p for p in paths
        if p.endswith(".go") and (include_tests or not p.endswith("_test.go"))
    )


def enclosing_function(lines, occurrence_idx):
    """Returns the name of the last `func ...(` declaration at or before
    occurrence_idx (0-based line index), or None if the file has none."""
    name = None
    for i in range(occurrence_idx, -1, -1):
        m = FUNC_DECL_RE.match(lines[i])
        if m:
            name = m.group(1)
            break
    return name


def main():
    prod_files = go_files(include_tests=False)
    all_files = go_files(include_tests=True)
    if not prod_files:
        print("check_evidence_flow: FAILED — found 0 Go files under backend/, "
              "the file collector is broken.")
        sys.exit(2)

    prod_text = {f: f.read_text(encoding="utf-8", errors="replace") for f in prod_files}
    all_text = dict(prod_text)
    for f in all_files:
        if f not in all_text:
            all_text[f] = f.read_text(encoding="utf-8", errors="replace")

    # 1. Base invariant: crossevidence.TaskRecordEvidence has a registered consumer.
    worker_files = [f for f in prod_files if "cmd/worker" in str(f.relative_to(ROOT))]
    consumer_wired = any(
        "mux.HandleFunc(" in line and "TaskRecordEvidence" in line
        for f in worker_files
        for line in prod_text[f].splitlines()
    )
    if not consumer_wired:
        print("check_evidence_flow: FAILED — crossevidence.TaskRecordEvidence has "
              "no registered mux.HandleFunc consumer in cmd/worker.")
        sys.exit(1)

    # 2. Map every NewRecordEvidenceTask(...) call site to its enclosing function.
    producers = set()  # (function_name, file, line_no)
    for f, text in prod_text.items():
        lines = text.splitlines()
        for i, line in enumerate(lines):
            if ENQUEUE_CALL_RE.search(line):
                fn = enclosing_function(lines, i)
                if fn is None:
                    print(f"check_evidence_flow: FAILED — {f.relative_to(ROOT)}:{i + 1} "
                          f"calls NewRecordEvidenceTask outside any function (unparseable).")
                    sys.exit(2)
                producers.add((fn, f, i + 1))

    if not producers:
        print("check_evidence_flow: FAILED — found 0 crossevidence.NewRecordEvidenceTask "
              "call sites, the collector regex is broken (non-vacuity check).")
        sys.exit(2)

    # 3. For each enclosing function, require a call site somewhere else in the
    # codebase (test files count too — a function only ever unit-tested but
    # never invoked from real code is still worth flagging, so we search
    # prod+test, but a hit in prod code alone is what actually clears it).
    unwired = []
    for fn, def_file, def_line in sorted(producers):
        if fn in ALLOWLIST:
            continue
        called = False
        call_re = re.compile(rf'[.\s]{re.escape(fn)}\(')
        for f, text in prod_text.items():
            for line_no, line in enumerate(text.splitlines(), start=1):
                if f == def_file and line_no == def_line:
                    continue
                if FUNC_DECL_RE.match(line) and FUNC_DECL_RE.match(line).group(1) == fn:
                    continue  # the `func Foo(...) {` line itself, not a call
                if call_re.search(line):
                    called = True
                    break
            if called:
                break
        if not called:
            unwired.append((fn, def_file, def_line))

    if unwired:
        print("check_evidence_flow: FAILED — evidence-producing function(s) with no caller:")
        for fn, def_file, def_line in unwired:
            print(f"  EVENT_UNWIRED  {fn}() — {def_file.relative_to(ROOT)}:{def_line} builds/enqueues "
                  f"a crossevidence task but nothing calls this function")
        print(
            "\nFix: wire a real caller (API handler, cron, or event trigger), or "
            "delete the dead producer function and document the decision — see "
            "docs/dev/dead-worker-chains.md."
        )
        sys.exit(1)

    print(f"OK — {len(producers)} crossevidence.NewRecordEvidenceTask call site(s) "
          f"across {len({fn for fn, _, _ in producers})} function(s), all reachable, "
          f"and TaskRecordEvidence has a registered consumer.")
    if ALLOWLIST:
        print(f"note: {len(ALLOWLIST)} function(s) allowlisted (see ALLOWLIST in this script).")


if __name__ == "__main__":
    main()
