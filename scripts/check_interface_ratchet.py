#!/usr/bin/env python3
# S121-F6 (A1): untyped-interface ratchet.
#
# CLAUDE.md bans `interface{}` in favour of typed structs, but the backend
# carries a large legacy population of `any`/`interface{}` — much of it the
# idiomatic `map[string]any` used for JSON responses. Rewriting all of it at
# once is out of scope and risky, so this gate does the next best thing: it
# freezes the count and only lets it go DOWN. Any change that adds a net new
# untyped interface fails CI; removing them is always allowed.
#
# When a new `any` is genuinely justified (e.g. a new JSON response map),
# reduce one elsewhere in the same PR, or lower the BASELINE below with a note.
# The number must never be raised to accommodate new debt.
#
# Scope: production Go under backend/ — excludes tests and generated code
# (sqlc *.sql.go, querier.go, the db package), where untyped interfaces are
# unavoidable and not authored by hand.

import re
import sys
import pathlib

ROOT = pathlib.Path(__file__).resolve().parent.parent
BACKEND = ROOT / "backend"

# Freeze at the count measured on 2026-07-11 (S121-F6). Only ever lower this.
BASELINE = 518

# `interface{}` (old syntax) and the `any` keyword used as a type. We match
# `any` only where it is a type token (word-boundary), which is what the Go
# compiler treats as the interface alias.
INTERFACE_RE = re.compile(r"interface\s*\{\s*\}")
ANY_RE = re.compile(r"\bany\b")


def is_excluded(path: pathlib.Path) -> bool:
    name = path.name
    if name.endswith("_test.go"):
        return True
    if name.endswith(".sql.go") or name == "querier.go" or name == "models.go" and "/db/" in str(path):
        return True
    parts = path.parts
    if "db" in parts and "internal" in parts:
        # internal/db is sqlc-generated
        idx = parts.index("internal")
        if idx + 1 < len(parts) and parts[idx + 1] == "db":
            return True
    return False


def count() -> int:
    total = 0
    for f in BACKEND.rglob("*.go"):
        if is_excluded(f):
            continue
        text = f.read_text(encoding="utf-8", errors="ignore")
        total += len(INTERFACE_RE.findall(text))
        total += len(ANY_RE.findall(text))
    return total


def main() -> int:
    n = count()
    if n > BASELINE:
        print(
            f"Untyped-interface ratchet FAILED: {n} occurrences of `interface{{}}`/`any` "
            f"in production Go, baseline is {BASELINE}.\n"
            f"You added {n - BASELINE} new one(s). Prefer a typed struct; if a JSON map is\n"
            f"genuinely required, remove an existing `any` elsewhere in this change, or lower\n"
            f"BASELINE in scripts/check_interface_ratchet.py with a note (never raise it)."
        )
        return 1
    if n < BASELINE:
        print(
            f"Untyped-interface ratchet: {n} < baseline {BASELINE} — nice, debt went down. "
            f"Please lower BASELINE to {n} in scripts/check_interface_ratchet.py to lock in the win."
        )
        return 0
    print(f"Untyped-interface ratchet OK: {n} occurrences (== baseline {BASELINE}).")
    return 0


if __name__ == "__main__":
    sys.exit(main())
