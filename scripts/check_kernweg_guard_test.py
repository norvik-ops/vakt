#!/usr/bin/env python3
"""Selbsttest fuer check_kernweg_guard.py — das Gate muss rot werden koennen."""
import pathlib
import subprocess
import sys

sys.path.insert(0, str(pathlib.Path(__file__).resolve().parent))
from check_kernweg_guard import LABEL, verdict  # noqa: E402

CASES = [
    # (Beschreibung, Dateien, Labels, erwarteter rc)
    ("nur Produkt", ["backend/internal/auth/service.go"], set(), 0),
    ("nur Kernweg-Test", ["frontend/kernwege/20-anmeldung-rechte.spec.ts"], set(), 0),
    ("nur Doku", ["docs/launch-gate.md", "CLAUDE.md"], set(), 0),
    ("beides ohne Label", ["backend/internal/auth/service.go", "frontend/kernwege/20-anmeldung-rechte.spec.ts"], set(), 1),
    ("beides mit fremdem Label", ["frontend/src/pages/Login.tsx", "scripts/kernwege/kernwege.sh"], {"bug"}, 1),
    ("beides mit Label", ["frontend/src/pages/Login.tsx", "scripts/kernwege/kernwege.sh"], {LABEL}, 0),
    ("Install-Anleitung zaehlt als Produkt", [".env.example", "frontend/kernwege/10-installation.spec.ts"], set(), 1),
    ("Config des Pruefers zaehlt als Test", ["frontend/playwright.kernwege.config.ts", "docker-compose.yml"], set(), 1),
    # Nachbarn, die NICHT dazugehoeren: das alte, gemockte e2e/ ist kein Kernweg-Pruefer.
    ("e2e/ ist kein Kernweg-Test", ["frontend/e2e/auth.spec.ts", "frontend/src/pages/Login.tsx"], set(), 0),
]


def main() -> int:
    bad = 0
    for desc, files, labels, want in CASES:
        rc, _ = verdict(files, labels)
        ok = rc == want
        bad += not ok
        print(f"{'ok ' if ok else 'BAD'} {desc}: rc={rc} (erwartet {want})")
    # Fail-closed: ein kaputter git-Aufruf ist kein Gruen.
    r = subprocess.run(
        [sys.executable, str(pathlib.Path(__file__).with_name("check_kernweg_guard.py")),
         "--base", "0000000000000000000000000000000000000000", "--head", "HEAD"],
        capture_output=True, text=True,
    )
    ok = r.returncode == 2
    bad += not ok
    print(f"{'ok ' if ok else 'BAD'} unbekannter Basis-SHA -> rc=2: rc={r.returncode}")
    print(f"{len(CASES) + 1 - bad}/{len(CASES) + 1} Faelle bestanden")
    return 1 if bad else 0


if __name__ == "__main__":
    sys.exit(main())
