#!/usr/bin/env python3
"""G-CI — the workflows must not silently lose a job or a step.

Why this gate exists (2026-07-30)
---------------------------------
While wiring seven previously-unwired gates into ci.yml, a single Edit that
replaced a two-step block with a one-step block **deleted the frontend `Build`
step**. The YAML stayed valid, every remaining step stayed green, and a diff
stat showed only insertions because the same edit added more lines than it
removed. Nothing in the repo would have caught it.

That is the same class this repo keeps paying for: `check_routes.py` reporting
OK over a quarter of the calls it never parsed, the `ValidateUUIDParams` guard
mounted on a subset of the router, a gate that counted prose instead of code.
A green run over a subset is a false signal, and a *missing* step produces
exactly that — every remaining step passes.

It also freezes the merge-gate contract. The `protected-flow` ruleset on `main`
requires five status checks **by name**: four from ci.yml plus
"Doc consistency (drift + links)" from docs.yml. A renamed job silently stops
being required (the ruleset keeps waiting for a context that no longer exists);
a *new* job creates a context the ruleset does not know and therefore never
enforces. Both are invisible in a passing CI run.

What it checks
--------------
A. The job id -> display-name mapping of ci.yml and docs.yml matches EXPECTED
   exactly, in both directions. Renaming or adding a job is a deliberate act
   that must also update the ruleset, so it must fail here first.
B. Every step named in REQUIRED_STEPS still exists in its job. This is the
   half that needs no git history and therefore works identically locally,
   in the pre-push hook and on a shallow CI checkout.
C. Every step is structurally a step: `run` xor `uses`, no unknown keys, and no
   empty `run` body — an empty body is what a block scalar with the wrong
   indentation collapses to, which has already broken this file once.
D. If a base revision can be resolved, no step name present there may have
   disappeared. This catches the deletion of steps that are NOT in
   REQUIRED_STEPS. It needs git history, so on a shallow checkout it is
   reported as skipped — never silently passed, and never the only check.
E. No job or step is NEUTRALIZED in place — still there under its own name, but
   structurally incapable of failing. Added 2026-07-30 for K2-05.

Why E exists (K2-05)
--------------------
A-D freeze names and shapes. They all pass for a step that is still listed,
still required, still structurally valid — and switched off. The forms all use
*allowed* GitHub keys, so even C's "no unknown keys" waved them through:

    continue-on-error: true      # step runs, fails, job stays green
    if: false                    # step never runs at all
    run: <everything> || true    # step runs, cannot report failure
    run: <gate> || echo "skip"   # same thing, spelled differently   (R-01)
    run: |                       # errexit off + a forced zero exit  (R-01)
      set +e
      <gate>
      exit 0
    run: echo "TODO"             # nothing left that could fail      (R-01)
    run: <gate> | tee gate.log   # `tee`'s exit code wins, no pipefail (N-01)
    run: <gate> &                # backgrounded: the launcher returns 0 (N-01)
    run: if false; then …; fi    # constant-false guard, never runs   (N-01)

Three of these were added on 2026-07-30 for R-01: the first version matched
three literal tails (`|| true`, `|| :`, `|| exit 0`) while the commit message
claimed it grabbed "the class". It did not, and `|| echo …` plus `set +e`/
`exit 0` are not exotic — they are what someone reaches for when a gate is in
the way. The matcher works by mechanism (see the R-01 block at
_SUPPRESSED_TAIL), not by spelling.

The last three were added the same day for N-01, after the widened version was
still measured green on them AT GATE LEVEL. `| tee` is the one that matters:
it is not even sabotage-shaped. Someone adds it to keep a log, GitHub runs
`run:` as `bash -e {0}` WITHOUT `pipefail`, `tee` decides the exit code, and a
required merge gate silently stops being able to go red. The sister gate
check_workflow_exit_capture.py records the same `bash -e {0}` assumption and
explicitly puts pipefail questions outside its own scope — so before this
change NO gate in this repo saw that class.

Measured before the fix: neutralizing `Route reconciliation (FE apiFetch ↔ BE
routes)` with `continue-on-error: true` and `users.role explicit at every insert
(D24-1)` with `if: false` left this gate at rc=0, printing "required steps: 25
... OK — every required job and step present".

The denominator section below already named JOB-LEVEL `if:` as a known blind
spot — the mechanism was understood one level up and missed one level down.
E covers both levels, which is the I4 move: grep the class, not the finding.

E is deliberately narrow. A step with a real condition (`if: always()`,
`if: github.event_name == 'push'`) is normal engineering and is COUNTED and
PRINTED, not failed — a conditional step is visible, a neutralized one is not.
Only constant-false conditions and blanket failure-suppression are errors.

REQUIRED_STEPS is deliberately a curated list of load-bearing steps rather than
every step: a full mirror would turn every cosmetic rename into a gate failure
and would be deleted within a month. A legitimate rename is expected to update
this list in the same commit.

Its denominator — what this gate does NOT look at
-------------------------------------------------
This repo's own house rule is that a gate must name what it skipped, because a
green run over an unnamed subset is a false signal. So, explicitly:

* SCOPE: it reads **2 of the 11 workflows** — ci.yml and docs.yml, the two that
  carry the five required status checks. The other nine (release.yml,
  deploy-sites.yml, sync-public-repo.yml, promote.yml, smoke-schedule.yml,
  db-ops.yml, issue-*.yml, jira-close.yml) are NOT read. Deleting the only
  caller of a gate from release.yml is therefore invisible here — verified:
  removing the `backup_gpg_test.sh` step from release.yml leaves this gate at
  rc=0. Those workflows gate a release or a deploy, not a merge, which is why
  the line is drawn here and not that they do not matter.
* MATRIX: a `strategy.matrix` added to a job changes GitHub's *context* name to
  `Frontend (TypeScript) (24)` while `name:` stays literal. The ruleset then
  waits forever for a context that never reports — the exact failure mode
  described above, reached by a mechanism this gate cannot see, because the
  name in the file is still correct.
* JOB-LEVEL `if:`: covered by check E since K2-05 for the CONSTANT-FALSE case
  (`if: false`), which is the neutralization form — the comment in ci.yml
  records this repo's empirical finding that a SKIPPED job still leaves the PR
  CLEAN. A job with a REAL condition is counted and printed, not failed: this
  gate cannot evaluate a GitHub expression, so `if: github.ref ==
  'refs/heads/never'` remains a way to disable a job that only a human reading
  the printed `cond` list will notice.
* WHERE CHECK E LOOKS AWAY — spelled out rather than summarized, and the same
  lines are PRINTED on every run. This list is NOT called exhaustive, and that
  word was removed on purpose (N-01): E is a text matcher over a `run` body, it
  is not a shell parser, so "every way a shell body can end up unable to fail"
  is not a set it can close. What it CAN do is name the mechanisms it knows it
  misses, so a reader is never told a limit does not exist:
  1. PARTIAL suppression. E flags a `run` body only when EVERY effective line
     is failure-suppressed. A ten-line script with one suppressed line in the
     middle stays green — legitimate (`rm -f x || true`) and neutralizing
     (`the-actual-gate || true`) look identical to a text matcher, and a gate
     that cries wolf on `rm -f` gets switched off. Same for `set +e` without a
     forced zero exit at the end: the last command still decides, so only the
     commands BEFORE it are swallowed. Same for a pipe: `npm ci` followed by
     `gate | tee log` is not flagged, because `npm ci` can still fail the step.
  2. RUNTIME suppression. `bash -c "$CMD || true"`, a wrapper script that eats
     the exit code, a `trap … EXIT` that calls `exit 0`. E reads the literal
     `run` body; it does not execute it and does not follow it into another
     file.
  3. GITHUB EXPRESSIONS. Only CONSTANT-false `if:` values are neutralization
     (see the job-level bullet above). `if: github.ref == 'refs/heads/never'`
     is counted, printed as `cond`, and left to a human.
  4. STRUCTURAL WRAPPERS WITH A REAL CONDITION (N-01). `if <gate>; then echo
     ok; fi` swallows the gate's exit code just as completely as `|| true` —
     the `if` construct returns 0 when its condition is false and there is no
     `else`. It is NOT flagged, and this is the same judgement call as (1):
     `if command -v shellcheck; then shellcheck …; fi` is ordinary engineering
     and lives in this repo's own ci.yml today, and a matcher cannot tell the
     two apart without evaluating the condition. Only a CONSTANT-false guard
     (`if false`, `while false`, `if [ 1 = 0 ]`) is unambiguous, and that one
     IS flagged. The same goes for a `for`/`while` over a list that happens to
     be empty and for a `case` that falls through: the work never runs, the
     step reports 0, and only a reader can tell whether that was intended.
  5. THE FORMS NOBODY HAS THOUGHT OF YET. (1)-(4) are the mechanisms that have
     actually been measured against this gate. The honest general statement is
     the one above: a text matcher cannot close this class, so this list will
     grow again, and a run that prints it is not claiming otherwise.
* It does not verify the ruleset itself. That the five names here are the five
  names GitHub requires was checked by hand against the live ruleset; nothing
  re-checks it, so a ruleset edit can drift away from this file.
"""

## GATE INVENTORY (2026-07-30) ###############################################
#
# Written down because the number was previously only asserted in a report: a
# count nobody can recount is a claim, not a measurement. Regenerate with
#   grep -ohE '(python3|bash) +(scripts|infra/server/scripts)/[^ ]+\.(py|sh)' \
#     .github/workflows/*.yml | sort -u
#
# RULE. A "gate" here is a script that (a) is invoked by a workflow, (b) fails
# that workflow with a non-zero exit when the invariant it measures is violated,
# and (c) judges the CONTENTS OF THIS REPOSITORY. Clause (c) is what separates a
# gate from an ops tool: backup-verify.sh, rotate-key.sh, restore.sh and
# ci-status.sh also exit non-zero, but they act on a live system or a real
# archive, and they are correctly absent from every workflow.
#
# SCOPE — and this is a real limit, not a formality: the inventory is
# PATH-BASED (scripts/, infra/server/scripts/). At least one gate lives outside
# it and is therefore not listed or counted here:
#   frontend/scripts/verify-sri.cjs  — process.exit(1) on an SRI mismatch, wired
#   via `npm run build` in frontend/package.json, so it does run in CI and in
#   `make check`. Wired, but out of scope of the count below.
#
# NACHTRAG (2026-08-07, Codeaudit v5c): zwei Selbsttests sind dazugekommen,
# lint_orgid_queries_test.py und ci_status_test.sh. Der zweite sieht auf den
# ersten Blick wie ein Verstoss gegen Klausel (c) aus, weil ci-status.sh selbst
# ein Ops-Werkzeug ist und deshalb zu Recht in keinem Workflow steht. Sein
# SELBSTTEST beurteilt aber den Inhalt dieses Repositorys — er weist nach, dass
# ci-status.sh seinen eigenen Nenner prueft. Er faellt damit unter (c) und
# gehoert in die Liste; das Werkzeug selbst weiterhin nicht.
#
# ZWEITER NACHTRAG, derselbe Tag: beim Nachzaehlen mit der Vorschrift oben kam
# 33 heraus, nicht 31 — die Liste war schon VOR dieser Aenderung um zwei zu
# kurz. check_workflow_exit_capture.py (ci.yml:410) und check_mirror_make_test.py
# (ci.yml:470-471, laeuft zusaetzlich mit --selftest) laufen im Backend-Job und
# standen hier nie. Der Kommentar oben sagt, eine Zahl, die niemand nachzaehlen
# kann, sei eine Behauptung — sie war es hier bereits, obwohl die Vorschrift
# danebenstand. Beide sind jetzt aufgenommen; die Zahlen unten sind mit der
# Vorschrift nachgemessen, nicht fortgeschrieben.
#
# 33 scripts are invoked by a workflow; 31 of them gate a PR (ci.yml, docs.yml):
#
#   ci.yml / backend (28)
#     lint-orgid-queries.py            lint-sql-shapes.py
#     check_routes.py                  check_interface_ratchet.py
#     check_sqlc_frozen.py             check_sqlc_seam.py
#     check_sqlc_seam_test.py          check_module_isolation.py
#     check_module_isolation_test.py   check_openapi_coverage.py
#     check_outbound_security.py       check_image_tags.py
#     check_image_tags_test.py         check_user_role_insert.py
#     check_response_shape.py          check_ci_steps.py  (this file)
#     check_worker_wiring.py           check_evidence_flow.py
#     check_backup_hardening.py        check_backup_hardening_test.py
#     restore_test.sh                  backup_cron_test.sh
#     infra/server/scripts/vakt-deploy_test.sh
#     infra/server/scripts/backup-offsite_test.sh
#     lint_orgid_queries_test.py       ci_status_test.sh
#     check_workflow_exit_capture.py   check_mirror_make_test.py
#   ci.yml / frontend (1)
#     check_fe_csrf.py
#   docs.yml / doc-consistency (2)
#     check-docs.py                    check-i18n-drift.py
#   release.yml only — gates a release, not a merge (2)
#     backup_gpg_test.sh               restore_drill.sh
#
# IN KEINEM WORKFLOW, ABER IN `make gates` (1) — N-02, offener Rest:
#   scripts/check_ci_steps_test.py — der Selbsttest fuer Pruefung E (dieser
#   Datei). Er stand nach seiner Erstellung in keinem Make-Ziel und in keinem
#   Workflow; `make gates` faehrt ihn seit 2026-07-30, ci.yml noch nicht. Damit
#   haelt ihn der pre-push-Hook, aber kein required Status-Check. Die fehlende
#   Zeile gehoert in ci.yml/backend, unmittelbar hinter den check_ci_steps.py-
#   Schritt, und das Wort "Selbsttest" im Namen ist Absicht (die anderen beiden
#   Selbsttests heissen genauso):
#
#       - name: CI-Steps-Gate Selbsttest
#         run: python3 scripts/check_ci_steps_test.py
#
#   Wer die Zeile setzt, aendert im selben Commit zwei Zahlen mit: den Zaehler
#   "ci.yml / backend (24)" unten auf 25 und "29 scripts ... 27 of them" auf
#   30/28 — und traegt den Stepnamen in REQUIRED_STEPS ein, sonst haelt nur
#   Pruefung D ihn fest, und die geht auf push-Laeufen leer aus. Bis dahin ist
#   dieser Block die Stelle, an der die Luecke steht statt in einem Report.
#
# ZWEI WEITERE FEHLENDE ZEILEN (2026-08-02, REV-W1B N1 und N3). Beide liegen in
# .github/, also ausserhalb des Dateieigentums der Spur, die sie gefunden hat.
# Sie stehen hier, damit sie an einer Stelle stehen, die ein Gate liest, und
# nicht nur in einem Report:
#
#   (1) check_helm_image_tags.py gehoert in den Job `helm-chart` von ci.yml —
#       NICHT in `gates`. Nur `helm-chart` hat `azure/setup-helm` und
#       `helm dependency build`, und nur dort laeuft Teil B (helm template)
#       ueberhaupt. In `make gates` laeuft das Gate mit --teil-a-genuegt und
#       meldet ohne helm ausdruecklich "NICHT MESSBAR" statt "OK"; ohne diesen
#       CI-Schritt wird Teil B NIRGENDS automatisch gefahren:
#
#           - name: Helm-Image-Tag-Gate (Teil A + B)
#             run: python3 scripts/check_helm_image_tags.py
#
#       Ohne Flag. Ein rc=2 ist dort ein Fehlschlag und soll es sein: in einem
#       Job mit helm heisst "nicht messbar" nicht "geht schon", sondern dass
#       setup-helm oder dependency build ausgefallen ist.
#
#   (2) release.yml baut das Backend-Image ohne den strengen Billing-Schalter.
#       `backend/Dockerfile` kennt seit 2026-08-02 `ARG VAKT_BILLING`; auf
#       `required` bricht der Bau ab, wenn cmd/billing fehlt (REV-W1B N3 — der
#       bewachte Bau hat auf der privaten Seite eine Bau-Zeit-Pruefung
#       verloren). Das Release-Image traegt den Billing-Dienst; der Bau in
#       release.yml:214-220 sollte das sagen:
#
#           docker build \
#             --build-arg APP_VERSION=${{ github.ref_name }} \
#             --build-arg VAKT_BILLING=required \
#             ...
#
#       Statisch haelt scripts/check_billing_binary.py die Zusage schon jetzt
#       fest (Container ausgeliefert -> Quelltext da -> Dockerfile baut ihn),
#       und dieses Gate laeuft in `make gates`. Was nur der Bau zeigen kann —
#       dass das fertige Image /billing wirklich traegt —, haengt an dieser
#       Zeile.
#
# A GATE THAT RUNS NOWHERE (1) — open finding, not an accepted state:
#   scripts/backup_restore_wiring_test.sh (PR #76) — 13 real assertions against a
#   live compose stack, measured rc=0 in 85 s (13/13). It is invoked ONLY by
#   `make test` and `make test-backup-wiring`; no workflow runs it. It needs
#   Docker, so it cannot join `make gates` (see the Makefile rule), and its
#   natural home is the `integration` job, which already has Docker. NOT wired
#   here on purpose: its 90 s budget waits on a real `* * * * *` cron tick, so
#   adding it to a REQUIRED status context trades a wiring gap for a flakiness
#   risk on every PR — a merge-gate decision, not a wiring fix.
#
# NOT a gate, deliberately unwired (1):
#   bundle-check.sh — prints "WARNING: Main bundle > 600 KB" and exits 0. It
#   cannot fail on the condition it measures, so a CI step would be a green tick
#   asserting nothing. Making the 600 KB budget hard is a product decision.
#
# Everything else under those two paths is ops/runtime/tooling:
# server cron (backup-*.sh, container-health-watcher.sh), provisioning
# (setup-deploy-ssh.sh, provision-zabbix-billing.py), the deploy forced-command
# wrappers (vakt-deploy.sh, vakt-deploy-gate.sh), and human utilities
# (ci-status.sh, support-bundle.sh, install.sh, update.sh, release-prep.sh).
##############################################################################

import os
import pathlib
import re
import subprocess
import sys

ROOT = pathlib.Path(__file__).resolve().parent.parent

# ── job id -> display name. The display name IS the status-check context that
# the `protected-flow` ruleset on main requires. Changing it here without
# changing the ruleset silently un-gates the branch.
EXPECTED_JOBS = {
    ".github/workflows/ci.yml": {
        "backend": "Backend (Go)",
        "integration": "Integration Tests",
        "frontend": "Frontend (TypeScript)",
        "helm-chart": "Helm chart lint & template",
    },
    ".github/workflows/docs.yml": {
        "doc-consistency": "Doc consistency (drift + links)",
    },
}

# ── steps whose silent disappearance would be expensive. Not a full mirror.
REQUIRED_STEPS = {
    (".github/workflows/ci.yml", "backend"): [
        "Build",
        "Test",
        "Lint",
        "Secret scanning (gitleaks)",
        "Vulnerability scan (govulncheck)",
        "Route reconciliation (FE apiFetch ↔ BE routes)",
        "Worker-Wiring-Gate (G7 — Asynq Producer/Consumer)",
        "Evidence-Flow-Gate (G12 — Cross-Modul-Evidence erreichbar)",
        "Image-Referenz-Gate Selbsttest",
        "restore.sh hardening (S89-1 — Key-Leak + Tamper)",
        "backup-cron.sh retention + Failure-Hook (S89-4)",
        "vakt-deploy.sh SSH-Command-Gate (R1-29-O01)",
        "org_id lint self-test",
        "ci-status.sh self-test",
    ],
    # The exact bug this gate was written for: `Build` and `Test (Vitest)` both
    # live here, and `Build` is the one that was deleted.
    (".github/workflows/ci.yml", "frontend"): [
        "Type check",
        "Lint",
        "Test (Vitest)",
        "Build",
        "CSRF-Gate (jeder FE-Write durch apiFetch)",
    ],
    # integration and helm-chart used to have NO entries at all, so their steps
    # hung on check D alone — the one check that goes vacuous on exactly the runs
    # that advance main (base == HEAD). Two jobs, eleven steps, guarded by the
    # weakest check in the file.
    (".github/workflows/ci.yml", "integration"): [
        "Gate — rotate_key_real_test must not be skipped (SEC-H08 / S99-4)",
        "Gate — jeder Integrations-Test liegt im Selektor",
        "Integration tests (testcontainers)",
        "Coverage floor — vaktaware / vaktscan (S126)",
    ],
    (".github/workflows/ci.yml", "helm-chart"): [
        "helm lint",
        "helm template (render all manifests)",
    ],
    (".github/workflows/docs.yml", "doc-consistency"): [
        "Doc consistency checks",
        "i18n drift check (missing keys + Du-forms)",
    ],
}

ALLOWED_STEP_KEYS = {
    "name", "run", "uses", "with", "env", "if", "id", "shell",
    "working-directory", "continue-on-error", "timeout-minutes",
}

# ── check E (K2-05) ─────────────────────────────────────────────────────────
# Jobs/steps allowed to be neutralized, with a reason. Empty on purpose: in
# these two workflows every job and step is a merge gate, so "present but
# cannot fail" has no legitimate use. An entry here is a hole and must argue for
# itself; a stale entry fails the gate (see the stale check in main).
NEUTRALIZED_ALLOWLIST: dict[tuple[str, str, str], str] = {}

# DEKLARIERTE UMBENENNUNGEN fuer Pruefung D.
#
# D paart Schritte ueber den Namen und kann eine Umbenennung nicht von einer
# Loeschung unterscheiden. Das ist Absicht — ein still verschwundener Schritt
# ist in einem gruenen Lauf unsichtbar. Der Preis: jede legitime Umbenennung
# schlaegt an, sobald die Basis sie noch nicht kennt. Bei einem PR in einen
# Integrations-Branch faellt das nicht auf (die Basis kennt sie schon); bei
# einem PR nach `main` sehr wohl, denn dort ist der Merge genau der Moment,
# in dem `main` sie lernt.
#
# Statt den Fehlalarm hinzunehmen (ein Gate, das bei korrekter Arbeit rot wird,
# wird abgeschaltet statt gefixt) wird die Umbenennung ERKLAERT. Wichtig ist
# die Gegenprobe darunter: der neue Schritt muss DENSELBEN Befehl fahren. Eine
# "Umbenennung", die auch den Befehl aendert, ist eine Loeschung im Kostuem
# und bleibt ein Befund.
#
#   (workflow, job, alter Name) -> (neuer Name, Begruendung)
RENAMED_STEPS: dict[tuple[str, str, str], tuple[str, str]] = {
    (".github/workflows/ci.yml", "backend", "Image-Tags existieren (compose + helm)"): (
        "Image-Referenzen (compose + helm + testcontainers + CI-Services)",
        "R1-INT-02: das Gate deckt seit 2026-07-30 auch testcontainers-Images und "
        "CI-Service-Container ab; der alte Name log ueber seinen Umfang. Gleicher "
        "Befehl (scripts/check_image_tags.py).",
    ),
}

# `if:` values that are constant false. GitHub accepts the bare word, the quoted
# word and the wrapped expression; all three mean "never runs". PyYAML parses
# the bare word to the Python bool, hence the `is False` branch below.
CONST_FALSE_IF = {"false", "${{ false }}", "${{false}}", "off", "no"}

# ── R-01 (2026-07-30) ───────────────────────────────────────────────────────
# The first version of E matched `\|\|\s*(true|:|exit\s+0)\s*$` — THREE SPELLINGS,
# while the commit message claimed it grabbed "the class". Two equally cheap
# forms stayed green, both of them the literal K2-05 finding in other words:
#
#     run: python3 scripts/check_routes.py || echo "route check skipped"
#     run: |
#       set +e
#       python3 scripts/check_user_role_insert.py
#       exit 0
#
# Measured: both together left this gate at rc=0 with "OK — every required job
# and step present". So the class is now grabbed by MECHANISM, not by spelling:
#
#   * `cmd || <tail>` swallows the exit code UNLESS the tail can itself fail.
#     `|| echo x`, `|| logger …`, `|| cp a b` all report success just as
#     reliably as `|| true`; `|| exit 1`, `|| false`, `|| { …; exit 2; }` do not.
#   * With errexit switched off (`set +e` / `set +o errexit`) a failing command
#     no longer aborts the body, so whatever stands LAST decides the exit code.
#     If that last line always returns 0 (`exit 0`, `true`, `:`, a bare `echo`),
#     nothing in between can fail the step. GitHub runs `run` bodies under
#     `bash -e`, so WITHOUT `set +e` a trailing `exit 0` is NOT neutralizing —
#     the body aborts at the first failure and never reaches it. Both halves are
#     required, which is why this does not fire on ordinary scripts.
#
# ── N-01 (2026-07-30) ───────────────────────────────────────────────────────
# Three further mechanisms, measured green at GATE level before this change —
# each one replaces a required step's exit code with a different command's:
#
#   * `cmd | <sink>` WITHOUT `set -o pipefail`. GitHub runs `run:` as
#     `bash -e {0}` (no `shell:`/`defaults:` in either workflow — checked), and
#     bash without pipefail reports the LAST segment's status. `gate | tee
#     gate.log` therefore reports tee's. Same rule as `||`: the pipe swallows
#     unless the tail can itself fail, and it does not swallow at all once the
#     body sets pipefail. Deliberately NOT applied to a pipe that some other
#     construct consumes — `if a | b; then` hands the status to `if`, and
#     `X=$(a | b)` keeps it inside the substitution.
#   * `cmd &`. The shell backgrounds it and the launcher returns 0; nothing the
#     command does afterwards can reach the step's exit code.
#   * A CONSTANT-FALSE guard around the whole body (`if false; then …; fi`,
#     `while false; do … done`, `if [ 1 = 0 ]`). The work never runs and an
#     empty construct returns 0. Only CONSTANT-false, for the same reason the
#     `if:` key is only flagged when it is constant-false: a real condition is
#     ordinary engineering (blind spot 4 in the docstring).
#
# A command line that cannot report failure: `|| <tail>` …
_SUPPRESSED_TAIL = re.compile(r"\|\|\s*(?P<tail>\S.*)$")
# … unless the tail itself fails. `exit`/`return` without an argument inherits
# the failing status, so only a literal 0 counts as suppression. The strip is
# what makes `|| (exit 0)` a finding: the code group reads `0)` there, and the
# first version therefore judged the tail fallible and stayed green (N-01).
_TAIL_EXIT = re.compile(r"\b(?:exit|return)\b(?:[ \t]+(?P<code>\S+))?")
_TAIL_FALSE = re.compile(r"(?:^|[|;&(\s])false(?:[ \t;)&|]|$)")
# A line that always returns 0 all by itself.
_ALWAYS_ZERO = re.compile(r"^(?:exit[ \t]+0|true|:|echo\b[^|&;]*)$")
# errexit switched off — a failing command stops aborting the body.
_ERREXIT_OFF = re.compile(r"^set\s+(?:\+[a-df-zA-Z]*e[a-zA-Z]*|\+o\s+errexit)\b")
# Scaffolding, not the work the step claims to do.
_SCAFFOLDING = re.compile(r"^(set\s|cd\s|export\s|shopt\s|echo\s|source\s|\.\s)")
# N-01: pipefail restores the pipeline's failing status, so a pipe is only a
# swallow while it is OFF. Any `set -...o pipefail` anywhere in the body counts.
_PIPEFAIL_ON = re.compile(r"\bset\s+-[a-zA-Z]*o\s+pipefail\b")
# N-01: a trailing single `&` backgrounds the command. `&&` and `2>&1` are not it.
_BACKGROUNDED = re.compile(r"(?<![&>])&[ \t]*$")
# N-01: a line whose exit status is consumed by an enclosing construct rather
# than by the step. `if a | b; then`, `while …; do`, `foo() {`.
_COMPOUND_HEAD = re.compile(r"^(?:if|elif|while|until|for|case)\b|(?:;[ \t]*(?:then|do)|\{)[ \t]*$")
# N-01: a guard that is constant false — the body it wraps never runs.
_CONST_FALSE_GUARD = re.compile(
    r"^(?:if|while)[ \t]+(?:false\b|\[\[?[ \t]*(?:1[ \t]*=[ \t]*0|0[ \t]*=[ \t]*1"
    r"|0[ \t]+-eq[ \t]+1|1[ \t]+-eq[ \t]+0)[ \t]*\]\]?)"
)
_BLOCK_END = re.compile(r"(?:^|[;\s])(?:fi|done)[ \t]*$")


def is_const_false(cond) -> bool:
    if cond is False:
        return True
    return isinstance(cond, str) and cond.strip().lower() in CONST_FALSE_IF


def is_truthy(val) -> bool:
    if val is True:
        return True
    return isinstance(val, str) and val.strip().lower() in {"true", "${{ true }}", "yes", "on"}


def tail_can_fail(tail: str) -> bool:
    """True when the right-hand side of a `||` or a `|` can itself report a
    failure."""
    if _TAIL_FALSE.search(tail):
        return True
    for m in _TAIL_EXIT.finditer(tail):
        code = m.group("code")
        # `(exit 0)`, `exit 0;`, `exit 0)` — the punctuation is the construct,
        # not the code. Without the strip, `|| (exit 0)` reads as fallible.
        if code is None or code.strip("\"'();&|") != "0":
            return True
    return False


def last_top_level_pipe(line: str) -> int:
    """Index of the last `|` that is really this line's pipe (N-01).

    Not a shell parser — it tracks exactly the three things that decide whether
    a `|` belongs to this command line: quotes (`grep 'a|b'`), `$(…)`/backtick
    substitution (`X=$(a | b)` keeps the status inside), and `||`. Anything
    subtler is blind spot 5. Returns -1 when there is no such pipe.
    """
    idx, quote, depth, i = -1, None, 0, 0
    while i < len(line):
        c = line[i]
        if quote:
            if c == quote:
                quote = None
        elif c in "\"'`":
            quote = c
        elif line.startswith("$(", i):
            depth += 1
            i += 2
            continue
        elif c == "(":
            depth += 1
        elif c == ")":
            depth = max(0, depth - 1)
        elif c == "|":
            if line.startswith("||", i):
                i += 2
                continue
            if depth == 0:
                idx = i
        i += 1
    return idx


def line_cannot_fail(line: str, pipefail: bool = False) -> bool:
    if _ALWAYS_ZERO.match(line):
        return True
    m = _SUPPRESSED_TAIL.search(line)
    if m and not tail_can_fail(m.group("tail")):
        return True
    if _BACKGROUNDED.search(line):
        return True
    # A pipe only swallows while pipefail is off, and only when the step is the
    # one reading the status — not when `if`/`while`/`{` consumes it.
    if not pipefail and not _COMPOUND_HEAD.search(line):
        i = last_top_level_pipe(line)
        if i >= 0 and not tail_can_fail(line[i + 1:]):
            return True
    return False


def const_false_guard(effective: list) -> bool:
    """The WHOLE body sits inside a constant-false guard, so none of it runs.

    Deliberately all-or-nothing, like every other branch of E: a constant-false
    guard around one line of a real script is dead code, a constant-false guard
    around the entire body is a step that cannot go red. Reads the scaffolding-
    stripped view so a leading `set -euo pipefail` does not hide the guard.
    """
    if not effective or not _CONST_FALSE_GUARD.match(effective[0]):
        return False
    if _BLOCK_END.search(effective[0]):    # one-liner: `if false; then …; fi`
        return len(effective) == 1
    return bool(_BLOCK_END.search(effective[-1]))


def run_body_cannot_fail(body: str):
    """(reason, detail) when a shell body structurally cannot report a failure.

    Three mechanisms, see the R-01 and N-01 blocks above:
      1. EVERY effective line swallows its exit code — `|| <tail that cannot
         fail>`, `| <sink>` while pipefail is off, a trailing `&`, or a line
         that always returns 0.
      2. errexit is switched off AND the last line always returns 0 — then no
         failure in between can reach the step's exit code.
      3. The whole body sits inside a CONSTANT-false guard, so it never runs.

    (1) is deliberately all-or-nothing: one `rm -f x || true` inside a real
    script is ordinary, a body where NOTHING can fail is a step that only looks
    like one. Line continuations are folded first so `foo \\\n  || true` is seen
    as the single command it is.

    What this does NOT reach is the docstring's blind-spot list, and the fourth
    entry there is the important one: a wrapper with a REAL condition
    (`if <gate>; then echo ok; fi`) swallows just as thoroughly and is left
    green on purpose, because it is indistinguishable from the legitimate
    `if command -v shellcheck; then …; fi` that ci.yml already contains."""
    folded, buf = [], ""
    for raw in body.splitlines():
        line = raw.strip()
        if not line or line.startswith("#"):
            continue
        buf = (buf + " " + line).strip() if buf else line
        if buf.endswith("\\"):
            buf = buf[:-1].rstrip()
            continue
        folded.append(buf)
        buf = ""
    if buf:
        folded.append(buf)
    if not folded:
        return None

    pipefail = any(_PIPEFAIL_ON.search(ln) for ln in folded)
    effective = [ln for ln in folded if not _SCAFFOLDING.match(ln)]

    if const_false_guard(effective):
        return ("run cannot fail (constant-false guard)",
                f"the whole body sits inside `{effective[0]}` — a constant-false "
                "guard, so nothing in it ever runs and the empty construct "
                "returns 0")

    if _ERREXIT_OFF.match(folded[0]) or any(_ERREXIT_OFF.match(ln) for ln in folded):
        if _ALWAYS_ZERO.match(folded[-1]):
            return ("run cannot fail (set +e)",
                    f"the body switches errexit off (`set +e`) and ends in "
                    f"`{folded[-1]}`, which always returns 0 — every failure in "
                    f"between is unreachable from the step's exit code")

    if all(_ALWAYS_ZERO.match(ln) for ln in folded):
        return ("run does nothing that can fail",
                "every line of the `run` body is `echo` / `true` / `:` / `exit 0` "
                "— the step has no work left that could report a failure")

    if effective and all(line_cannot_fail(ln, pipefail) for ln in effective):
        return ("run swallows failure",
                "every effective line of the `run` body is failure-suppressed "
                "(`|| <command that cannot fail>` / `| <sink>` without "
                "`set -o pipefail` / a trailing `&` / `exit 0` / `true`) — the "
                "step cannot report a failure")
    return None


def neutralization_of(node: dict):
    """(kind, detail) if this job/step is present-but-incapable-of-failing."""
    if is_const_false(node.get("if")):
        return ("if: false",
                f"`if: {node.get('if')!r}` — never runs, and a skipped check leaves "
                "the PR CLEAN")
    if is_truthy(node.get("continue-on-error")):
        return ("continue-on-error",
                "`continue-on-error: true` — runs, may fail, and the job stays green "
                "anyway")
    body = node.get("run")
    if isinstance(body, str):
        hit = run_body_cannot_fail(body)
        if hit:
            return hit
    return None

# Check D's base is NOT a hardcoded branch list. An earlier version tried
# ["origin/audit/v5-fixes", "origin/main", "main"] in order, which is wrong twice:
# a branch name baked into a gate outlives the branch, and a *stale* local `main`
# is still a valid ref, so D silently compared against a months-old tree and
# reported every legitimate rename since then as a deleted step (observed: the
# `Image-Tags existieren` -> `Image-Referenzen` rename from merged PR #74 came
# back as "step GONE"). A gate that cries wolf on correct work gets switched off.
#
# The only meaningful base is the one the change is measured against: the PR's
# target branch in CI, the previous commit locally.


def load_yaml(text: str, label: str) -> dict:
    """Parse with PyYAML. One parser, deliberately — no fallback.

    An earlier version fell back to `yq` when PyYAML was missing. That was a
    second gate with its own truth: PyYAML implements YAML 1.1 and parses this
    file's `on:` key as the boolean True (top-level keys come out as
    ['name', True, 'concurrency', 'env', 'jobs']), while mikefarah-yq yields the
    string "on". Only `jobs` is read here, so the divergence is harmless today —
    but "harmless today, under the keys we happen to read" is not a property a
    gate should rest on, and the yq branch was never once executed.

    So: one parser, installed explicitly in the CI step. If it is missing we stop
    with an actionable message rather than guessing or skipping — a gate that
    cannot read its input must not report success.
    """
    try:
        import yaml  # noqa: PLC0415
    except ImportError:
        sys.exit(
            "check_ci_steps: PyYAML is required and not installed.\n"
            "  pip install pyyaml   (the CI step installs it; ubuntu-latest does "
            "not ship it)\n"
            "  Deliberately NOT skipped and deliberately no `yq` fallback: two "
            "parsers with different YAML versions are two gates."
        )
    try:
        return yaml.safe_load(text)
    except yaml.YAMLError as exc:
        # A clean message, not a traceback: unparsable YAML is the single most
        # likely way this file breaks (a block scalar at the wrong indentation),
        # and a traceback buries the location the parser already knows.
        sys.exit(f"check_ci_steps: {label} is not valid YAML —\n{exc}")


def step_label(st: dict) -> str:
    if st.get("name"):
        return st["name"]
    return f"uses:{str(st.get('uses', '?')).split('@')[0]}"


def steps_of(job: dict) -> list:
    return job.get("steps") or []


def main() -> int:
    errors: list[str] = []
    notes: list[str] = []
    checked_jobs = checked_steps = 0

    parsed: dict[str, dict] = {}
    for wf in EXPECTED_JOBS:
        p = ROOT / wf
        if not p.exists():
            errors.append(f"{wf}: workflow file is gone")
            continue
        doc = load_yaml(p.read_text(), wf)
        if not isinstance(doc, dict) or "jobs" not in doc:
            errors.append(f"{wf}: no top-level `jobs` mapping")
            continue
        parsed[wf] = doc

    # ── A. job id -> name, both directions
    for wf, expected in EXPECTED_JOBS.items():
        doc = parsed.get(wf)
        if doc is None:
            continue
        jobs = doc["jobs"]
        for jid, want in expected.items():
            if jid not in jobs:
                errors.append(
                    f"{wf}: job `{jid}` is gone — the required status check "
                    f"{want!r} would never report again"
                )
                continue
            got = jobs[jid].get("name")
            if got != want:
                errors.append(
                    f"{wf}: job `{jid}` is named {got!r}, expected {want!r} — "
                    "renaming it un-gates main (the ruleset waits for the old name)"
                )
            checked_jobs += 1
        for jid in jobs:
            if jid not in expected:
                errors.append(
                    f"{wf}: NEW job `{jid}` (name={jobs[jid].get('name')!r}) — a new "
                    "status context the ruleset on main does not require. Add it to "
                    "EXPECTED_JOBS here and to the ruleset, or fold the steps into an "
                    "existing job."
                )

    # ── B. required steps still present
    for (wf, jid), required in REQUIRED_STEPS.items():
        doc = parsed.get(wf)
        if doc is None:
            continue  # already reported by A (missing/unparsable workflow)
        if jid not in doc.get("jobs", {}):
            # NOT a silent `continue`. A REQUIRED_STEPS key that names a job which
            # does not exist would otherwise check nothing and still report OK —
            # exactly the "green over a subset" class. Found by a probe: the docs
            # entry was keyed `docs` while the real job id is `doc-consistency`,
            # so all its required steps were skipped without a word.
            errors.append(
                f"{wf}: REQUIRED_STEPS names job `{jid}`, which does not exist in "
                f"this workflow — {len(required)} required step(s) were NOT checked. "
                "Fix the key (typo?) or remove the entry."
            )
            continue
        present = {step_label(s) for s in steps_of(doc["jobs"][jid]) if isinstance(s, dict)}
        for name in required:
            if name not in present:
                errors.append(f"{wf} job `{jid}`: required step {name!r} is MISSING")
            checked_steps += 1

    # ── C. structural validity of every step
    for wf, doc in parsed.items():
        for jid, job in doc["jobs"].items():
            for i, st in enumerate(steps_of(job)):
                where = f"{wf} job `{jid}` step[{i}]"
                if not isinstance(st, dict):
                    errors.append(f"{where}: not a mapping ({st!r})")
                    continue
                has_run, has_uses = "run" in st, "uses" in st
                if has_run == has_uses:
                    errors.append(
                        f"{where} ({step_label(st)}): must have exactly one of "
                        "`run`/`uses`"
                    )
                if has_run and not str(st["run"]).strip():
                    errors.append(
                        f"{where} ({step_label(st)}): empty `run` body — a block "
                        "scalar with the wrong indentation collapses to this"
                    )
                unknown = set(st) - ALLOWED_STEP_KEYS
                if unknown:
                    errors.append(f"{where} ({step_label(st)}): unknown keys {sorted(unknown)}")

    # ── E. no job or step is neutralized in place (K2-05)
    neutralized_seen = set()
    conditional = []  # real conditions: visible, not an error
    checked_neutralization = 0
    for wf, doc in parsed.items():
        for jid, job in doc["jobs"].items():
            targets = [("<job>", job)] + [
                (step_label(st), st) for st in steps_of(job) if isinstance(st, dict)
            ]
            for label, node in targets:
                checked_neutralization += 1
                hit = neutralization_of(node)
                where = f"{wf} job `{jid}`" + ("" if label == "<job>" else f" step {label!r}")
                if hit:
                    key = (wf, jid, label)
                    if key in NEUTRALIZED_ALLOWLIST:
                        neutralized_seen.add(key)
                        notes.append(f"neutralization allowlisted: {where} — "
                                     f"{NEUTRALIZED_ALLOWLIST[key]}")
                    else:
                        kind, detail = hit
                        errors.append(
                            f"{where}: NEUTRALIZED ({kind}) — {detail}. It is still "
                            "present under its own name, so checks A-D all pass. Delete "
                            "it if it is obsolete, or fix it — do not leave a required "
                            "gate that cannot go red."
                        )
                elif node.get("if") is not None:
                    conditional.append(f"{where} runs only `if: {node['if']}`")
    for key in sorted(NEUTRALIZED_ALLOWLIST):
        if key not in neutralized_seen:
            errors.append(
                f"NEUTRALIZED_ALLOWLIST entry {key} no longer applies — the job/step is "
                "either gone or no longer neutralized. Remove the entry so the exception "
                "cannot outlive its reason."
            )

    # ── D. nothing that existed on the base may have vanished
    def rev(ref: str) -> str | None:
        r = subprocess.run(["git", "-C", str(ROOT), "rev-parse", "--verify", "-q", ref],
                           capture_output=True, text=True)
        return r.stdout.strip() if r.returncode == 0 else None

    head = rev("HEAD")
    base = None
    tried: list[str] = []
    # In a PR run GitHub names the target branch; locally the previous commit is
    # the thing the working change is measured against.
    gh_base = os.environ.get("GITHUB_BASE_REF") or ""
    for cand in ([f"origin/{gh_base}", gh_base] if gh_base else []) + ["HEAD~1"]:
        tried.append(cand)
        cand_sha = rev(cand)
        if cand_sha is None:
            continue
        # A base that IS HEAD compares the file with itself: every step matches,
        # D can never go red, and it would still print a step count that reads
        # like coverage. That is exactly the state of a push run on main — the
        # runs that advance the branch this gate protects. Verified: with
        # origin/main == HEAD, deleting an `integration` step left rc=0 while D
        # happily reported "64 step(s) compared".
        if head is not None and cand_sha == head:
            notes.append(
                f"base-diff: `{cand}` resolves to HEAD itself — not compared "
                "(a self-comparison cannot fail)."
            )
            continue
        base = cand
        break

    if base is None:
        notes.append(
            f"base-diff SKIPPED: no usable base (tried {tried}; shallow checkout, "
            "root commit, or base == HEAD). Checks A-C still ran, so this run is "
            "NOT vacuous — but deletion of a step outside REQUIRED_STEPS was not "
            "verified. That is why integration/helm-chart have explicit entries."
        )
    else:
        compared = 0
        renamed = 0
        for wf, doc in parsed.items():
            r = subprocess.run(["git", "-C", str(ROOT), "show", f"{base}:{wf}"],
                               capture_output=True, text=True)
            if r.returncode != 0:
                notes.append(f"base-diff: {wf} does not exist at {base} (new file) — not compared")
                continue
            bdoc = load_yaml(r.stdout, f"{base}:{wf}")
            if not isinstance(bdoc, dict) or "jobs" not in bdoc:
                notes.append(f"base-diff: {base}:{wf} unparsable — not compared")
                continue
            for jid, bjob in bdoc["jobs"].items():
                if jid not in doc["jobs"]:
                    continue  # reported by A
                was = [step_label(s) for s in steps_of(bjob) if isinstance(s, dict)]
                cur = [s for s in steps_of(doc["jobs"][jid]) if isinstance(s, dict)]
                now = {step_label(s) for s in cur}
                by_name = {step_label(s): s for s in cur}
                base_by_name = {step_label(s): s for s in steps_of(bjob) if isinstance(s, dict)}
                for name in was:
                    compared += 1
                    if name in now:
                        continue
                    ren = RENAMED_STEPS.get((wf, jid, name))
                    if ren is None:
                        errors.append(
                            f"{wf} job `{jid}`: step {name!r} existed at {base} and is "
                            "GONE. Adding steps is fine; losing one is invisible in a "
                            "green run and in a diff stat. War das eine Umbenennung? "
                            "Dann in RENAMED_STEPS eintragen, mit Begruendung."
                        )
                        continue
                    new_name, _why = ren
                    if new_name not in now:
                        errors.append(
                            f"{wf} job `{jid}`: step {name!r} is declared in "
                            f"RENAMED_STEPS as {new_name!r}, but no step of that name "
                            "exists either. Beide sind weg — das ist eine Loeschung."
                        )
                        continue
                    # Gegenprobe: gleiche Arbeit, nur anderer Name?
                    old_run = (base_by_name[name].get("run") or "").strip()
                    new_run = (by_name[new_name].get("run") or "").strip()
                    if old_run != new_run:
                        errors.append(
                            f"{wf} job `{jid}`: {name!r} -> {new_name!r} ist als "
                            "Umbenennung deklariert, faehrt aber einen ANDEREN Befehl "
                            f"({old_run!r} -> {new_run!r}). Eine Umbenennung, die den "
                            "Befehl aendert, ist eine Loeschung im Kostuem."
                        )
                        continue
                    renamed += 1
        notes.append(f"base-diff against {base}: {compared} step(s) compared"
                     + (f", {renamed} declared rename(s) accepted (same command)" if renamed else ""))

    print("G-CI — workflow job/step integrity")
    print(f"  workflows      : {len(parsed)}/{len(EXPECTED_JOBS)}")
    print(f"  jobs verified  : {checked_jobs}")
    print(f"  required steps : {checked_steps}")
    print(f"  can-fail check : {checked_neutralization} job(s)+step(s) inspected, "
          f"{len(neutralized_seen)} neutralization(s) allowlisted, "
          f"{len(conditional)} conditional")
    # Wo E wegschaut — gedruckt, nicht nur im Kopf. R-01 entstand daraus, dass
    # der Commit "die Klasse" behauptete und der Nenner die Grenze nicht nannte;
    # N-01 daraus, dass die gedruckte Grenze sich "erschöpfend" nannte. Das Wort
    # steht hier nicht mehr, und der Grund dafür steht in (5).
    print("      erkannt      : `|| <Tail, der nicht scheitern kann>` · `set +e` + "
          "Schlusszeile-immer-0 · ein Body, in dem nur echo/true/: steht · "
          "`| <Senke>` OHNE `set -o pipefail` (GitHub fährt `run:` als `bash -e {0}`, "
          "ohne pipefail) · abschließendes `&` · konstant-falscher Guard "
          "(`if false`, `while false`, `if [ 1 = 0 ]`) · `if:`/`continue-on-error`.")
    print("      blind spot   : (1) TEILWEISE Unterdrückung — nur ein `run`-Body, "
          "in dem JEDE wirksame Zeile schluckt, ist ein Befund; ein `rm -f x || true` "
          "oder ein `gate | tee log` mitten in echter Arbeit nicht. "
          "(2) Unterdrückung zur LAUFZEIT — Wrapper-Skript, `trap … EXIT`, "
          "`bash -c \"$CMD || true\"`: E liest den Body, es führt ihn nicht aus. "
          "(3) GitHub-AUSDRÜCKE — nur konstant-falsche `if:` sind Neutralisierung, "
          "echte Bedingungen stehen oben als `cond`. (4) STRUKTUR-WRAPPER MIT ECHTER "
          "BEDINGUNG — `if <gate>; then echo ok; fi` schluckt den Exit-Code "
          "vollständig und bleibt trotzdem GRÜN: von `if command -v shellcheck; "
          "then …; fi` (steht in dieser ci.yml) ist es ohne Auswertung der Bedingung "
          "nicht zu unterscheiden. Ebenso eine Schleife über eine leere Liste. "
          "(5) E ist ein TEXT-MATCHER, kein Shell-Parser — diese Liste ist deshalb "
          "ausdrücklich NICHT erschöpfend, sie nennt die gemessenen Mechanismen.")
    for c in conditional:
        # Not an error, but never silent: this gate cannot evaluate a GitHub
        # expression, so a condition is a place where the step may not run and
        # only a reader can judge whether that is intended.
        print(f"      cond         : {c}")
    for n in notes:
        print(f"  note           : {n}")

    if errors:
        print(f"\nFAIL — {len(errors)} problem(s):\n")
        for e in errors:
            print(f"  · {e}")
        return 1

    print("\nOK — every required job and step present, every step structurally valid.")
    return 0


if __name__ == "__main__":
    sys.exit(main())
