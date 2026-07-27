#!/usr/bin/env python3
# S5/G7 (S134): Asynq worker producer/consumer wiring gate ("Worker-Smoke").
#
# The recurring defect class this closes: a Task-name constant gets defined
# (`TaskXxx = "..."`), a worker handler is written and registered on the mux —
# but nothing in the codebase ever enqueues it (WORKER_UNWIRED_PRODUCER, e.g.
# the training-reminder/ORP.3-sync chains found dead in S5/W2/W3). Or the
# reverse: something enqueues a task type that no mux.HandleFunc ever consumes
# (WORKER_UNWIRED_CONSUMER) — the job vanishes silently at dequeue time.
#
# This is a static, dependency-free set check: no running stack, no Redis. It
# cannot see fully dynamic task names (none exist in this codebase today — all
# Task* constants are string literals referenced by identifier) and does not
# attempt to distinguish "producer via cron" from "producer via API request" —
# both count as wired, which matches the AK ("Producer" is either).
#
# Curated ALLOWLIST entries are task constants with a legitimate reason the
# static reference count can't see their producer or consumer.

import re
import sys
import pathlib
import subprocess

ROOT = pathlib.Path(__file__).resolve().parent.parent
BACKEND = ROOT / "backend"

# Task-name constant definitions, e.g.:
#   TaskFoo = "pkg:foo"
#   const TaskBar = "pkg:bar"
#   taskBaz = "pkg:baz"                (unexported, cmd/worker-local)
TASK_CONST_RE = re.compile(
    r'^\s*(?:const\s+)?([a-zA-Z0-9_]*[Tt]ask[A-Za-z0-9_]*)\s*=\s*"([^"]+)"',
    re.MULTILINE,
)

# name -> reason. Each entry MUST have a comment explaining why the static
# reference count is wrong for it, not why the feature is fine to leave dead.
ALLOWLIST = {
    # none yet — keep this empty unless a real static-analysis blind spot shows up.
}


def go_files():
    """All non-test .go files tracked OR untracked-but-present under backend/,
    via git so generated/vendored trees outside the repo are never walked."""
    out = subprocess.run(
        ["git", "-C", str(ROOT), "ls-files", "-z", "--others", "--exclude-standard", "--cached", "backend"],
        capture_output=True, check=True,
    ).stdout
    paths = [p for p in out.decode("utf-8", "replace").split("\0") if p]
    return sorted(
        ROOT / p for p in paths
        if p.endswith(".go") and not p.endswith("_test.go")
    )


def strip_line_comment(line: str) -> str:
    # Good enough here: none of the lines we scan contain `//` inside a string
    # literal (task names and mux registrations don't).
    idx = line.find("//")
    return line if idx == -1 else line[:idx]


def main():
    files = go_files()
    if not files:
        print("check_worker_wiring: FAILED — found 0 Go files under backend/, "
              "the file collector is broken.")
        sys.exit(2)

    file_text = {f: f.read_text(encoding="utf-8", errors="replace") for f in files}

    # 1. Collect every Task*/task* constant definition.
    tasks = {}  # name -> (file, value)
    for f, text in file_text.items():
        for m in TASK_CONST_RE.finditer(text):
            name, value = m.group(1), m.group(2)
            tasks.setdefault(name, (f, value))

    if not tasks:
        print("check_worker_wiring: FAILED — found 0 Task constants, "
              "the collector regex is broken (non-vacuity check).")
        sys.exit(2)

    worker_files = [f for f in files if "cmd/worker" in str(f.relative_to(ROOT))]

    unwired_consumer = []  # defined + (maybe) produced, but no mux.HandleFunc
    unwired_producer = []  # has a mux.HandleFunc, but nobody ever enqueues it
    considered = 0

    for name, (def_file, value) in sorted(tasks.items()):
        if name in ALLOWLIST:
            continue
        considered += 1

        value_literal = f'"{value}"'
        has_consumer = False
        for f in worker_files:
            for line in file_text[f].splitlines():
                line = strip_line_comment(line)
                if "mux.HandleFunc(" in line and (
                    re.search(rf"\b{re.escape(name)}\b", line) or value_literal in line
                ):
                    has_consumer = True
                    break
            if has_consumer:
                break

        has_producer = False
        for f, text in file_text.items():
            for line in text.splitlines():
                line = strip_line_comment(line)
                # A producer reference is either the constant identifier, OR the
                # task-name string literal itself — module isolation (CLAUDE.md)
                # forbids a module importing another module's package, so a
                # cross-module producer (e.g. vaktscan enqueueing a vaktcomply
                # task) legitimately hardcodes the literal instead of importing
                # the constant. Both count as "wired".
                if not (re.search(rf"\b{re.escape(name)}\b", line) or value_literal in line):
                    continue
                # Skip the constant's own definition line.
                if f == def_file and TASK_CONST_RE.match(line):
                    continue
                # Skip the mux registration line itself — that is consumer
                # wiring, not evidence that anything enqueues the task.
                if "mux.HandleFunc(" in line:
                    continue
                has_producer = True
                break
            if has_producer:
                break

        if not has_consumer:
            unwired_consumer.append((name, value, def_file))
        if not has_producer:
            unwired_producer.append((name, value, def_file))

    if unwired_consumer or unwired_producer:
        print("check_worker_wiring: FAILED — unwired Asynq task(s) found:")
        for name, value, def_file in unwired_consumer:
            print(f"  WORKER_UNWIRED_CONSUMER  {name} ({value!r}) — "
                  f"defined in {def_file.relative_to(ROOT)}, no mux.HandleFunc registers it")
        for name, value, def_file in unwired_producer:
            print(f"  WORKER_UNWIRED_PRODUCER  {name} ({value!r}) — "
                  f"defined in {def_file.relative_to(ROOT)}, nothing ever enqueues it")
        print(
            "\nFix: either wire a real producer/consumer, or remove the dead half "
            "of the chain (task constant, payload type, handler, mux registration) "
            "and document the decision — see docs/dev/dead-worker-chains.md."
        )
        sys.exit(1)

    print(f"OK — {considered} Asynq task type(s) each have at least one producer "
          f"and one registered consumer.")
    if ALLOWLIST:
        print(f"note: {len(ALLOWLIST)} task(s) allowlisted (see ALLOWLIST in this script).")


if __name__ == "__main__":
    main()
