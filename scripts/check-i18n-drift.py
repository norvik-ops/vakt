#!/usr/bin/env python3
"""
check-i18n-drift.py — i18n Drift-Guard for Vakt frontend.

Checks:
1. All keys in de.json exist in en.json, fr.json, nl.json (no missing translations)
2. No hardcoded German strings (containing Umlauts or German-specific patterns) in
   JSX files that are NOT wrapped in t() calls — heuristic detection.
3. No Du-form strings in de.json (dein/deine/deinen/deiner/du )

Exit code 1 if violations found, 0 if clean.
CI: python3 scripts/check-i18n-drift.py
"""

import json
import os
import re
import sys
from pathlib import Path

ROOT = Path(__file__).parent.parent
LOCALES_DIR = ROOT / "frontend" / "src" / "i18n" / "locales"
SRC_DIR = ROOT / "frontend" / "src"

REFERENCE_LOCALE = "de"
OTHER_LOCALES = ["en", "fr", "nl"]

errors: list[str] = []
warnings: list[str] = []


def load_json(path: Path) -> dict:
    with open(path, encoding="utf-8") as f:
        return json.load(f)


def flatten(d: dict, prefix: str = "") -> set[str]:
    """Flatten nested dict to set of dotted key paths."""
    keys = set()
    for k, v in d.items():
        full = f"{prefix}.{k}" if prefix else k
        if isinstance(v, dict):
            keys |= flatten(v, full)
        else:
            keys.add(full)
    return keys


def check_missing_keys():
    ref_path = LOCALES_DIR / f"{REFERENCE_LOCALE}.json"
    if not ref_path.exists():
        errors.append(f"Reference locale file not found: {ref_path}")
        return

    ref = load_json(ref_path)
    ref_keys = flatten(ref)

    for locale in OTHER_LOCALES:
        path = LOCALES_DIR / f"{locale}.json"
        if not path.exists():
            errors.append(f"Locale file missing: {path}")
            continue
        data = load_json(path)
        locale_keys = flatten(data)
        missing = ref_keys - locale_keys
        if missing:
            for key in sorted(missing)[:20]:  # cap output
                errors.append(f"[{locale}] Missing key: {key}")
            if len(missing) > 20:
                errors.append(f"[{locale}] ... and {len(missing) - 20} more missing keys")


def check_du_forms():
    """Detect Sie/Du inconsistencies in de.json."""
    ref_path = LOCALES_DIR / f"{REFERENCE_LOCALE}.json"
    if not ref_path.exists():
        return
    with open(ref_path, encoding="utf-8") as f:
        content = f.read()

    # Match Du-form patterns in string values
    du_pattern = re.compile(
        r'"(?:[^"]*\b(?:dein|deine|deinen|deiner|deines|deinem)\b[^"]*|'
        r'[^"]*\b du \b[^"]*)"',
        re.IGNORECASE
    )
    matches = list(du_pattern.finditer(content))
    for m in matches:
        errors.append(f"[de.json] Du-form detected: {m.group()[:80]}")


def check_hardcoded_strings():
    """Heuristic: find JSX text nodes with German umlauts not wrapped in t()."""
    UMLAUT_RE = re.compile(r'>[^<{]*[äöüßÄÖÜ][^<{]*</')
    T_CALL_RE = re.compile(r'\{t\(')

    tsx_files = list(SRC_DIR.rglob("*.tsx"))
    hit_count = 0

    for fpath in tsx_files:
        # Skip test files
        if ".test." in fpath.name or "__tests__" in str(fpath):
            continue
        with open(fpath, encoding="utf-8") as f:
            lines = f.readlines()
        for lineno, line in enumerate(lines, 1):
            # Skip lines that already use t()
            if T_CALL_RE.search(line):
                continue
            # Skip comments
            stripped = line.strip()
            if stripped.startswith("//") or stripped.startswith("*"):
                continue
            if UMLAUT_RE.search(line):
                rel = fpath.relative_to(ROOT)
                warnings.append(f"{rel}:{lineno}: potential hardcoded DE string: {stripped[:80]}")
                hit_count += 1
                if hit_count >= 50:
                    warnings.append("... (truncated at 50 hardcoded string warnings)")
                    return



def check_orphan_keys():
    """Top-level i18n namespaces that no source file references any more.

    check_missing_keys catches keys present in de but absent elsewhere. It cannot
    see the opposite: a whole namespace whose only consumer was DELETED. Those
    keys stay in perfect parity across all four locales, so every existing check
    reports green while the strings ship in the bundle — which is how a removed
    Managed-Hosting UI left 132 translated keys (admin.tenants.*, including
    "impersonateError") behind and made a deliberately-not-built feature look
    half-present to the next reader.

    Namespace granularity on purpose: per-key would be noisy (keys are built
    dynamically), while a whole top-level namespace with zero references is
    unambiguous dead weight.
    """
    ref = json.loads((LOCALES_DIR / f"{REFERENCE_LOCALE}.json").read_text(encoding="utf-8"))
    sources = "\n".join(
        f.read_text(encoding="utf-8", errors="ignore")
        for f in list(SRC_DIR.rglob("*.tsx")) + list(SRC_DIR.rglob("*.ts"))
    )
    checked = 0
    for ns, val in ref.items():
        if not isinstance(val, dict):
            continue
        checked += 1
        # `t('admin.tenants...')`, `t("admin.tenants` or a dynamic `admin.tenants.${x}`
        if re.search(r"[\'\"`]" + re.escape(ns) + r"[.\'\"`]", sources):
            continue
        errors.append(
            f"i18n namespace '{ns}' has no reference in {SRC_DIR} — its consumer was "
            f"probably deleted; remove the block from all locales or wire it up"
        )
    return checked


def main():
    # G-07 non-vacuity guard: no locale files / no sources means the paths are
    # wrong (bad cwd, moved dir), not that the translations are perfect. Without
    # this, every check below iterates over nothing and the gate reports a clean
    # pass for work it never did.
    n_locales = len(list(LOCALES_DIR.glob("*.json"))) if LOCALES_DIR.is_dir() else 0
    n_sources = len(list(SRC_DIR.rglob("*.tsx"))) if SRC_DIR.is_dir() else 0
    if n_locales == 0 or n_sources == 0:
        print(f"i18n drift check FAILED — found {n_locales} locale file(s) and "
              f"{n_sources} .tsx source(s) (non-vacuity guard, G-07). "
              f"Check the working directory: {LOCALES_DIR} / {SRC_DIR}")
        sys.exit(2)

    check_missing_keys()
    check_du_forms()
    check_hardcoded_strings()
    n_ns = check_orphan_keys()

    if warnings:
        print("\n=== i18n WARNINGS (hardcoded strings — not blocking) ===")
        for w in warnings:
            print(f"  WARN  {w}")

    if errors:
        print("\n=== i18n ERRORS ===")
        for e in errors:
            print(f"  ERR   {e}")
        print(f"\n{len(errors)} error(s) found. Fix before merging.")
        sys.exit(1)
    else:
        print(f"i18n drift check passed. {len(warnings)} warning(s); "
              f"checked {n_locales} locale file(s) against {n_sources} .tsx source(s).")
        sys.exit(0)


if __name__ == "__main__":
    main()
