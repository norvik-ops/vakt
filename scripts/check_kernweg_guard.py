#!/usr/bin/env python3
"""Macher != Pruefer — ein PR aendert Kernweg-Tests ODER Produktcode, nicht beides.

Regel (PROCESS.md P7c, docs/launch-gate.md): Wer einen Fehler fixt, aendert die
Kernweg-Tests nicht im selben PR. Wer den Test anpassen darf, damit er gruen wird,
macht den Test leichter, nicht die Arbeit besser. Ein PR, der beides anfasst,
braucht das Label `kernweg-aenderung` (und im PR-Text eine Begruendung).

Das gilt auch fuer den haeufigsten legitimen Fall: ein Fix macht einen bekannten
Befund gruen, sein `test.fail()` muss raus. Dann traegt der PR das Label — sichtbar,
statt still.

Aufruf (CI, .github/workflows/kernweg-guard.yml):
    python3 scripts/check_kernweg_guard.py --base <sha> --head <sha> --labels "a,b"
Zum Testen ohne git:
    python3 scripts/check_kernweg_guard.py --files a.go frontend/kernwege/x.ts --labels ""

Exit: 0 = erlaubt, 1 = beides ohne Label, 2 = Aufruf-/git-Fehler (fail-closed).
"""
from __future__ import annotations

import argparse
import subprocess
import sys

LABEL = "kernweg-aenderung"

# Der Pruefer: Tests, ihre Konfiguration und die Umgebung, in der sie laufen.
TEST_PREFIXES = (
    "frontend/kernwege/",
    "frontend/playwright.kernwege.config.ts",
    "scripts/kernwege/",
)

# Das Gepruefte: alles, was in die Instanz eines Kunden eingeht. README und
# .env.example gehoeren dazu — Kernweg 2 installiert genau danach.
PRODUCT_PREFIXES = (
    "backend/",
    "frontend/src/",
    "frontend/public/",
    "frontend/index.html",
    "frontend/package.json",
    "frontend/package-lock.json",
    "frontend/vite.config.ts",
    "frontend/Dockerfile",
    "frontend/nginx-prod.conf",
    "frontend/security-headers.inc",
    "docker-compose.yml",
    "docker-compose.backup.yml",
    "Caddyfile",
    ".env.example",
    "README.md",
    "helm/",
    "scripts/backup.sh",
    "scripts/backup-cron.sh",
    "scripts/backup-pg-target.sh",
    "scripts/restore.sh",
    "scripts/update.sh",
    "scripts/install.sh",
)


def classify(files: list[str]) -> tuple[list[str], list[str]]:
    tests = [f for f in files if f.startswith(TEST_PREFIXES)]
    product = [f for f in files if f.startswith(PRODUCT_PREFIXES) and f not in tests]
    return tests, product


def verdict(files: list[str], labels: set[str]) -> tuple[int, str]:
    tests, product = classify(files)
    head = f"Macher != Pruefer: {len(files)} Datei(en) geprueft — {len(tests)} Kernweg-Test, {len(product)} Produkt."
    if not tests or not product:
        return 0, head + " OK (nur eine Seite geaendert)."
    listing = "\n".join(
        ["  Kernweg-Tests:"] + [f"    {f}" for f in tests[:15]]
        + ["  Produktcode:"] + [f"    {f}" for f in product[:15]]
    )
    if LABEL in labels:
        return 0, (
            f"{head}\n{listing}\n"
            f"ERLAUBT durch Label `{LABEL}` — die Begruendung gehoert in den PR-Text."
        )
    return 1, (
        f"{head}\n{listing}\n"
        f"FEHLER: Dieser PR aendert Kernweg-Tests UND Produktcode. Entweder trennen\n"
        f"(Fix in einem PR, Test-Anpassung in einem eigenen) oder das Label `{LABEL}`\n"
        f"setzen und im PR-Text begruenden, warum der Test sich mitaendern muss\n"
        f"(z. B. ein Befund ist behoben und sein test.fail() entfaellt)."
    )


def changed_files(base: str, head: str) -> list[str]:
    out = subprocess.run(
        ["git", "diff", "--name-only", f"{base}...{head}"],
        check=True, capture_output=True, text=True,
    ).stdout
    return [l for l in out.splitlines() if l.strip()]


def main(argv: list[str]) -> int:
    ap = argparse.ArgumentParser()
    ap.add_argument("--base")
    ap.add_argument("--head")
    ap.add_argument("--files", nargs="*")
    ap.add_argument("--labels", default="")
    args = ap.parse_args(argv)
    labels = {l.strip() for l in args.labels.split(",") if l.strip()}
    if args.files is not None:
        files = args.files
    elif args.base and args.head:
        try:
            files = changed_files(args.base, args.head)
        except subprocess.CalledProcessError as e:
            print(f"Macher != Pruefer: git diff fehlgeschlagen — kein Urteil moeglich:\n{e.stderr}", file=sys.stderr)
            return 2
        if not files:
            # Ein PR ohne Diff ist moeglich (leerer Merge), aber fast immer ein
            # falscher Basis-SHA. Leise gruen waere genau die "OK ueber nichts"-Klasse.
            print("Macher != Pruefer: git diff lieferte keine Datei — Basis/Head pruefen.", file=sys.stderr)
            return 2
    else:
        print("Aufruf: --base/--head oder --files", file=sys.stderr)
        return 2
    rc, msg = verdict(files, labels)
    print(msg)
    return rc


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
