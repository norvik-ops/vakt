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
# K2-07 (2026-07-30) — WHAT COUNTED AS A PRODUCER, AND WHY THAT WAS TOO LITTLE
# ---------------------------------------------------------------------------
# `has_producer` used to be true as soon as ANY line under backend/**.go
# contained the constant identifier or the task string literal, with only two
# exclusions: the definition line itself and lines containing `mux.HandleFunc(`.
# A `switch` case, a label/metric map, a `return TaskX` — every one of them
# counted as "something enqueues this task". Measured: a fully dead chain
# (constant + mux.HandleFunc, no enqueue anywhere) goes RED correctly, but ADD
# ONE LINE to a label map
#
#     var queueLabels = map[string]string{ taskDead: "Dead chain" }
#
# and the same dead chain reports OK. One line of mention was the whole
# difference. The docstring named its limits (dynamic task names, cron-vs-API)
# but not this one — it claimed to close the class "nothing ever enqueues it".
#
# A producer is now an ENQUEUE-SHAPED reference: the task must appear as the
# first argument of an `asynq.NewTask(...)` call, which is the only way this
# codebase creates a task. 55 of 58 tasks satisfy that directly.
#
# The remaining 3 (TaskScanTrivy / TaskScanNuclei / TaskScanOpenVAS) are really
# wired, but indirectly: `taskTypeForScanner()` returns the constant, its result
# is assigned to `taskType`, and `taskType` is passed to `asynq.NewTask(...)` —
# all inside internal/modules/vaktscan/service.go. The constant never appears at
# the call site, so they are RECORDED in INDIRECT_PRODUCERS with the resolver
# that makes them reachable, and the gate RE-WALKS that chain on every run:
# resolver exists, still returns this task, its result is still assigned in the
# recorded file, and that variable is still a NewTask first argument. A recorded
# task that gains a direct producer fails the gate so the exception is removed.
#
# Note what the recording deliberately does NOT accept: "the recorded file
# contains an asynq.NewTask somewhere". That was the first version of this check
# and it is the same reasoning as the bug — proximity mistaken for wiring.
#
# That is the difference the finding is about: these three were previously held
# to be wired FOR THE WRONG REASON, and the same wrong reason would have
# accepted a dead chain.
#
# Curated ALLOWLIST entries are task constants with a legitimate reason the
# static reference count can't see their producer or consumer.
#
# R-05 (2026-07-30) — the gate printed a claim it did not measure
# ---------------------------------------------------------------
# The dead-chain finding used to end in a FIXED sentence: "it IS mentioned {n}x
# (switch case, label map, return statement)". Only {n} was measured; the three
# forms were typed in once and printed for every task forever after. For the
# three scanner tasks the list happened to be right; on the very first probe
# with a different shape it named a switch case that was not there and missed
# the const definition and the mux registration that were. A gate that writes an
# unverified formulation into its own finding message is the class
# check_user_role_insert.py clears out one file over — prose counted as fact.
# The forms are now READ OFF THE LINES (classify_mention) and the sites are
# printed with file:line so the reader can go and look.
#
# WHERE THIS GATE LOOKS AWAY — printed on every run, not just recorded here:
#   * A fully DYNAMIC task name (`asynq.NewTask("scan:"+kind, …)`): the enqueue
#     surface is read as text, so a name assembled at runtime is invisible. None
#     exist today; the `enqueue sites` counter is what would show it changing.
#   * An enqueue OUTSIDE backend/**.go (a migration, an ops script, another
#     service) — not scanned.
#   * Whether the handler actually DOES anything: this is a wiring gate, not a
#     behaviour gate. A registered handler with an empty body passes.
#   * `classify_mention` names only shapes it recognizes; everything else comes
#     back as `reference` rather than being given a label it did not earn.

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

# The first argument of an asynq.NewTask(...) call — the only shape that
# actually enqueues in this codebase. Non-greedy up to the first comma or close
# paren; a task name never contains either, and the negated class spans newlines
# so a wrapped call is still read whole.
NEWTASK_ARG_RE = re.compile(r"asynq\.NewTask\(\s*([^,)]+)")

# task constant -> (resolver function, file that enqueues the resolver's result)
#
# Tasks whose enqueue goes through a function instead of naming the constant at
# the call site. Each entry is VERIFIED on every run (see verify_indirect): the
# resolver must exist, must still return THIS task, the recorded file must still
# assign the resolver's result to a variable, and that variable must still be the
# first argument of an asynq.NewTask. An entry whose task gains a direct producer
# fails the gate — an exception that outlives its reason is a hole nobody can see
# any more.
INDIRECT_PRODUCERS = {
    "TaskScanTrivy": ("taskTypeForScanner", "backend/internal/modules/vaktscan/service.go"),
    "TaskScanNuclei": ("taskTypeForScanner", "backend/internal/modules/vaktscan/service.go"),
    "TaskScanOpenVAS": ("taskTypeForScanner", "backend/internal/modules/vaktscan/service.go"),
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


def classify_mention(line: str, name: str) -> str:
    """What KIND of mention is this line? (R-05)

    Measured, not assumed. The gate used to append a fixed "switch case, label
    map, return statement" to every dead-chain finding regardless of what was
    actually there. Anything this function cannot place comes back as
    `reference`, which is honest — an unrecognized shape must not be given a
    name it did not earn."""
    stripped = line.strip()
    n = re.escape(name)
    if re.match(rf"^(?:const\s+)?{n}\s*(?:[A-Za-z_][\w.]*\s*)?=\s*\"", stripped):
        return "const definition"
    if re.match(rf"^case\b.*\b{n}\b", stripped):
        return "switch case"
    if re.match(rf"^{n}\s*:", stripped):
        return "map key"
    if re.search(rf"\breturn\b[^/]*\b{n}\b", stripped):
        return "return statement"
    if re.match(r"^func\b", stripped):
        return "function signature"
    if re.search(rf"\bmux\.HandleFunc\(\s*{n}\b", stripped):
        return "mux.HandleFunc (consumer)"
    return "reference"


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

    unwired_consumer = []   # defined + (maybe) produced, but no mux.HandleFunc
    unwired_producer = []   # has a mux.HandleFunc, but nobody ever enqueues it
    considered = 0
    direct_producers = 0
    indirect_producers = []  # recorded in INDIRECT_PRODUCERS and re-verified
    stale_indirect = []      # a recording that no longer holds — always an error

    # Comment-free view, used for every reference decision below. A gate that
    # counts prose lies — and after K2-07 the producer test is a reference
    # decision, so it has to be made on code.
    clean_text = {
        f: "\n".join(strip_line_comment(l) for l in t.splitlines())
        for f, t in file_text.items()
    }

    # Every asynq.NewTask(...) first argument in the tree: the enqueue surface.
    newtask_args = []
    for f, t in clean_text.items():
        for m in NEWTASK_ARG_RE.finditer(t):
            newtask_args.append(m.group(1).strip())
    if not newtask_args:
        print("check_worker_wiring: FAILED — found 0 `asynq.NewTask(` call sites, "
              "the producer matcher is broken (non-vacuity check).")
        sys.exit(2)
    enqueue_surface = " | ".join(newtask_args)

    def verify_indirect(name):
        """Does the recorded indirect wiring still hold? None = yes, else why not.

        The whole chain is re-walked, because "it was true once" is not a check:
            1. the resolver still exists,
            2. it still returns THIS task,
            3. the recorded file still assigns the resolver's result to a variable,
            4. and THAT variable is still the first argument of an asynq.NewTask.
        Step 3+4 is the part that makes this a verification rather than an
        amnesty: without it, "the file happens to contain a NewTask somewhere"
        would count — which is the very shape of reasoning K2-07 is about."""
        resolver, enqueue_file = INDIRECT_PRODUCERS[name]
        body = None
        for t in clean_text.values():
            m = re.search(rf"^func\s+(?:\([^)]*\)\s*)?{re.escape(resolver)}\s*\(", t, re.M)
            if m:
                # Bound the body at the closing brace in column 0, so a `return
                # TaskX` in a LATER function is never mistaken for this one's.
                rest = t[m.start():]
                close = re.search(r"^\}", rest[1:], re.M)
                body = rest[: close.end() + 1] if close else rest
                break
        if body is None:
            return f"resolver `{resolver}()` no longer exists"
        if not re.search(rf"return\s+{re.escape(name)}\b", body):
            return f"resolver `{resolver}()` no longer returns {name}"

        ef = ROOT / enqueue_file
        if ef not in clean_text:
            return f"recorded enqueue site {enqueue_file} no longer exists"
        et = clean_text[ef]
        vars_from_resolver = set(
            re.findall(rf"(\w+)\s*:?=\s*{re.escape(resolver)}\s*\(", et)
        )
        if not vars_from_resolver:
            return (f"{enqueue_file} no longer assigns the result of `{resolver}()` "
                    f"to anything — the resolver is not on an enqueue path there")
        enqueued = [
            v for v in vars_from_resolver
            if re.search(rf"asynq\.NewTask\(\s*{re.escape(v)}\b", et)
        ]
        if not enqueued:
            return (f"{enqueue_file} assigns `{resolver}()` to "
                    f"{sorted(vars_from_resolver)}, but none of those is passed to "
                    f"asynq.NewTask — the chain no longer reaches an enqueue")
        return None

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

        # A producer reference is the task as the FIRST ARGUMENT of an
        # asynq.NewTask call — either by identifier or by the string literal
        # itself (module isolation forbids a module importing another module's
        # package, so a cross-module producer legitimately hardcodes the
        # literal). Anything else is a mention, not an enqueue.
        has_producer = bool(
            re.search(rf"\b{re.escape(name)}\b", enqueue_surface)
            or value_literal in enqueue_surface
        )
        if has_producer:
            direct_producers += 1
            if name in INDIRECT_PRODUCERS:
                stale_indirect.append(
                    f"{name} now has a DIRECT producer — remove it from "
                    "INDIRECT_PRODUCERS so the exception cannot outlive its reason."
                )
        elif name in INDIRECT_PRODUCERS:
            reason = verify_indirect(name)
            if reason is None:
                has_producer = True
                indirect_producers.append((name, INDIRECT_PRODUCERS[name][0]))
            else:
                stale_indirect.append(
                    f"{name}: recorded as indirectly produced, but {reason}. Either "
                    "the chain is dead or the recording is wrong — both need a human."
                )

        if not has_consumer:
            unwired_consumer.append((name, value, def_file))
        if not has_producer:
            # How often is it mentioned at all? A task with mentions but no
            # enqueue is the K2-07 shape: it LOOKS wired to a text search.
            # R-05 (2026-07-30): this used to count the mentions and then print
            # a FIXED sentence naming "switch case, label map, return statement"
            # as if that had been measured. It had not — only the number was.
            # For the three scanner tasks the list happened to be right; for any
            # other dead chain it was a guess in the gate's own finding message,
            # which is the class check_user_role_insert.py clears out one file
            # over. Now the sites are collected and classified from the line
            # they stand on, and the message prints what was read.
            mentions = 0
            mention_sites = []
            word = rf"\b{re.escape(name)}\b"
            for f, t in clean_text.items():
                for i, line in enumerate(t.splitlines(), 1):
                    hits = len(re.findall(word, line))
                    if hits:
                        mentions += hits
                        mention_sites.append(
                            (str(f.relative_to(ROOT)), i, classify_mention(line, name))
                        )
            unwired_producer.append((name, value, def_file, mentions, mention_sites))

    for name in INDIRECT_PRODUCERS:
        if name not in tasks:
            stale_indirect.append(
                f"{name}: recorded in INDIRECT_PRODUCERS but no such task constant "
                "exists any more — remove the entry."
            )

    if unwired_consumer or unwired_producer or stale_indirect:
        print("check_worker_wiring: FAILED — unwired Asynq task(s) found:")
        for name, value, def_file in unwired_consumer:
            print(f"  WORKER_UNWIRED_CONSUMER  {name} ({value!r}) — "
                  f"defined in {def_file.relative_to(ROOT)}, no mux.HandleFunc registers it")
        for name, value, def_file, mentions, mention_sites in unwired_producer:
            extra = ""
            if mentions > 1:
                # R-05: the forms are READ OFF THE LINES, not asserted. Sites are
                # printed so the reader can go and look instead of trusting the
                # gate's prose — up to 6, with the remainder counted, never
                # silently cut.
                forms = sorted({form for _, _, form in mention_sites})
                extra = (f" — it IS mentioned {mentions}x as: {', '.join(forms)}; "
                         f"a mention is not an enqueue (K2-07)")
            print(f"  WORKER_UNWIRED_PRODUCER  {name} ({value!r}) — "
                  f"defined in {def_file.relative_to(ROOT)}, no `asynq.NewTask({name}, …)` "
                  f"anywhere{extra}")
            for rel, lineno, form in mention_sites[:6]:
                print(f"      mention  {rel}:{lineno}  [{form}]")
            if len(mention_sites) > 6:
                print(f"      mention  … and {len(mention_sites) - 6} more line(s)")
        for msg in stale_indirect:
            print(f"  INDIRECT_PRODUCERS stale  {msg}")
        print(
            "\nFix: either wire a real producer/consumer, or remove the dead half "
            "of the chain (task constant, payload type, handler, mux registration) "
            "and document the decision — see docs/dev/dead-worker-chains.md."
        )
        sys.exit(1)

    print(f"OK — {considered} Asynq task type(s) each have at least one producer "
          f"and one registered consumer.")
    print(f"  enqueue sites  : {len(newtask_args)} `asynq.NewTask(` call(s)")
    print(f"  producers      : {direct_producers} direct, "
          f"{len(indirect_producers)} indirect (recorded + verified), "
          f"0 mention-only")
    for name, resolver in indirect_producers:
        print(f"      indirect   : {name} via {resolver}() — chain re-walked: resolver "
              f"returns it, its result is assigned and passed to asynq.NewTask")
    if ALLOWLIST:
        print(f"note: {len(ALLOWLIST)} task(s) allowlisted (see ALLOWLIST in this script).")
    # Wo dieses Gate wegschaut — gedruckt, nicht nur im Kopf (R-05-Lehre: was
    # nur im Quelltext steht, liest niemand, der die Ausgabe fuer bare Muenze
    # nimmt).
    print("  blind by design: dynamisch zusammengesetzte Task-Namen "
          "(`asynq.NewTask(\"scan:\"+kind, …)`) · Enqueues ausserhalb von "
          "backend/**.go · ob der Handler etwas TUT (Verdrahtung, nicht "
          "Verhalten) — ein registrierter Handler mit leerem Rumpf ist hier gruen.")


if __name__ == "__main__":
    main()
