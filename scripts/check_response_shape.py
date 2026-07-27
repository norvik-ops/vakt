#!/usr/bin/env python3
# S14 (G6 — Response-Shape, sprint-132-135-codeaudit-v4-remediation-plan.md).
#
# SHAPE_MISMATCH class: a handler declares an XLSX response (Content-Type
# "spreadsheetml" or a Content-Disposition filename ending in .xlsx) but the
# bytes it actually writes are CSV (encoding/csv), not real XLSX magic bytes
# (`PK\x03\x04`). Excel opens the CSV fine (it sniffs by content), but any
# consumer that trusts the declared type — an API client, a virus scanner
# unpacking the zip, an audit tool asserting file format — sees a lie. Found
# live in vaktcomply.ExportControlsXLSX/ExportPoliciesXLSX and
# vaktscan.ExportFindingsXLSX (A20-1); fixed by routing through
# internal/shared/xlsxexport (real excelize workbook, magic bytes correct).
#
# Static heuristic, no live server needed: for every Go function in
# backend/internal/modules/**/*.go, if the body claims an xlsx response
# (content type or filename) AND constructs its own CSV writer
# (`csv.NewWriter`) instead of delegating to xlsxexport/docxexport, that's a
# SHAPE_MISMATCH. A function that calls into xlsxexport.Render* is fine even
# if a *different* function in the same file uses csv.NewWriter for an
# actual CSV export (Content-Type text/csv — never flagged).
#
# Exit non-zero on any violation.

import re
import sys
import pathlib

ROOT = pathlib.Path(__file__).resolve().parent.parent
BACKEND = ROOT / "backend"

XLSX_MARKERS = (
    "spreadsheetml",
    ".xlsx",
)

FUNC_RE = re.compile(r"^func\s+(?:\([^)]*\)\s+)?([A-Za-z_][A-Za-z0-9_]*)\s*\(", re.MULTILINE)


def split_functions(text):
    """Yield (name, body) for each top-level func in a Go source file."""
    starts = [(m.start(), m.group(1)) for m in FUNC_RE.finditer(text)]
    for i, (start, name) in enumerate(starts):
        end = starts[i + 1][0] if i + 1 < len(starts) else len(text)
        yield name, text[start:end]


def check_file(path):
    violations = []
    text = path.read_text(encoding="utf-8")
    for name, body in split_functions(text):
        claims_xlsx = any(marker in body for marker in XLSX_MARKERS)
        if not claims_xlsx:
            continue
        writes_raw_csv = "csv.NewWriter" in body
        delegates_to_render = "xlsxexport." in body or "docxexport." in body
        if writes_raw_csv and not delegates_to_render:
            violations.append(
                f"{path.relative_to(ROOT)}: {name} declares an xlsx response "
                f"(spreadsheetml/.xlsx) but writes raw CSV via csv.NewWriter "
                f"instead of internal/shared/xlsxexport — SHAPE_MISMATCH"
            )
    return violations


def main():
    modules_dir = BACKEND / "internal" / "modules"
    violations = []
    skipped = 0
    for path in sorted(modules_dir.rglob("*.go")):
        if path.name.endswith("_test.go"):
            skipped += 1
            continue
        violations.extend(check_file(path))

    if violations:
        print(f"FAIL — {len(violations)} response-shape mismatch(es):")
        for v in violations:
            print(f"  - {v}")
        sys.exit(1)

    print(f"OK — no XLSX handler writes raw CSV bytes (skipped {skipped} test file(s))")


if __name__ == "__main__":
    main()
