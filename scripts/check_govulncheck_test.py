#!/usr/bin/env python3
# Copyright (c) 2026 NorvikOps. All rights reserved.
# SPDX-License-Identifier: Elastic-2.0
"""
Selbsttest fuer check_govulncheck.py (PROCESS.md P9: drei Abnahmen).

  1. gruen auf der Baseline — die echte Allowlist gegen die Funde, die CI am
     2026-11-09 meldete (GO-2026-6872/6875/6909, alle erreichbar);
  2. rot bei einem echten neuen Fund, der namentlich genannt wird;
  3. rot, sobald ein Eintrag abgelaufen ist.

Dazu die Raender, an denen ein Allowlist-Gate typischerweise still durchlaesst:
leere Eingabe, Frist > 31 Tage, falsches Modul, nicht erreichbare Funde,
Altlast-Eintraege (nur Warnung).

Exit ungleich 0 bei jedem fehlgeschlagenen Fall.
"""

import copy
import json
import pathlib
import subprocess
import sys
import tempfile
import datetime

sys.path.insert(0, str(pathlib.Path(__file__).resolve().parent))

from check_govulncheck import DEFAULT_ALLOWLIST, evaluate, parse_stream  # noqa: E402

FAILURES = []
D = datetime.date.fromisoformat
CONFIG = {"config": {"protocol_version": "v1.0.0", "scanner_name": "govulncheck", "scan_level": "symbol"}}


def finding(osv, module, function=None):
    frame = {"module": module, "version": "v0.0.0"}
    if function:
        frame.update({"package": module, "function": function})
    return {"finding": {"osv": osv, "trace": [frame]}}


BASELINE = [
    CONFIG,
    finding("GO-2026-6872", "github.com/labstack/echo/v4", "Echo.GET"),
    finding("GO-2026-6872", "github.com/labstack/echo/v4"),
    finding("GO-2026-6875", "github.com/labstack/echo/v4", "Group.POST"),
    finding("GO-2026-6909", "github.com/crewjam/saml", "ServiceProvider.ParseResponse"),
    # nur Modulebene, nicht erreichbar: darf nicht rot machen
    finding("GO-2026-5543", "github.com/Azure/go-ntlmssp"),
]

REAL_ENTRIES = json.loads(DEFAULT_ALLOWLIST.read_text(encoding="utf-8"))["entries"]


def case(name, objs, entries, today, want_code, must_contain=()):
    code, lines = evaluate(objs, entries, D(today))
    out = "\n".join(lines)
    if code != want_code:
        FAILURES.append(f"{name}: exit {code}, erwartet {want_code}\n{out}")
        return
    for needle in must_contain:
        if needle not in out:
            FAILURES.append(f"{name}: Ausgabe nennt {needle!r} nicht\n{out}")
            return
    print(f"ok  {name}")


# 1. Baseline gruen, Nenner stimmt
case("Baseline gruen", BASELINE, REAL_ENTRIES, "2026-11-09", 0,
     ["geprueft 3 · erlaubt 3 · uebersprungen 1", "govulncheck-Gate: OK"])

# 2. Echter neuer Fund -> rot, namentlich
case("neuer erreichbarer Fund wird rot",
     BASELINE + [finding("GO-2026-9999", "golang.org/x/net", "html.Parse")],
     REAL_ENTRIES, "2026-11-09", 1, ["GO-2026-9999", "nicht auf der Allowlist"])

# 3. Abgelaufener Eintrag -> rot
case("abgelaufener Eintrag wird rot", BASELINE, REAL_ENTRIES, "2026-12-10", 1,
     ["GO-2026-6872", "abgelaufen"])
case("letzter Tag der Frist ist noch gruen", BASELINE, REAL_ENTRIES, "2026-12-09", 0)

# Raender
case("leere Eingabe ist kein Erfolg", [], REAL_ENTRIES, "2026-11-09", 1, ["config-Block fehlt"])

too_long = copy.deepcopy(REAL_ENTRIES)
too_long[0]["expires"] = "2099-01-01"
case("Frist ueber 31 Tage wird rot", BASELINE, too_long, "2026-11-09", 1, ["> 31 Tage"])

no_reason = copy.deepcopy(REAL_ENTRIES)
no_reason[1]["reason"] = "  "
case("Eintrag ohne Begruendung wird rot", BASELINE, no_reason, "2026-11-09", 1, ["reason"])

wrong_mod = copy.deepcopy(REAL_ENTRIES)
wrong_mod[2]["module"] = "github.com/other/saml"
case("falsches Modul deckt den Fund nicht", BASELINE, wrong_mod, "2026-11-09", 1, ["GO-2026-6909"])

case("nicht erreichbarer Fund ohne Eintrag bleibt gruen",
     BASELINE + [finding("GO-2026-8888", "golang.org/x/text")],
     REAL_ENTRIES, "2026-11-09", 0, ["uebersprungen 2"])

stale = [f for f in BASELINE if "GO-2026-6909" not in json.dumps(f)]
case("Altlast-Eintrag nur Warnung", stale, REAL_ENTRIES, "2026-11-09", 0,
     ["WARNUNG: GO-2026-6909", "geprueft 2 · erlaubt 2"])

future = copy.deepcopy(REAL_ENTRIES)
future[0]["added"] = "2026-12-01"
future[0]["expires"] = "2026-12-31"
case("vordatiertes added wird rot (umgeht sonst die 31 Tage)", BASELINE, future, "2026-11-09", 1,
     ["in der Zukunft"])

module_level = [{"config": dict(CONFIG["config"], scan_level="module")}] + BASELINE[1:]
case("scan_level ungleich symbol wird rot", module_level, REAL_ENTRIES, "2026-11-09", 1,
     ["scan_level"])

drift = [CONFIG, {"vuln": {"osv": "GO-2026-6872"}}]
case("kein finding-Block bei gepflegter Allowlist wird rot", drift, REAL_ENTRIES, "2026-11-09", 1,
     ["Format-Drift"])
case("kein finding-Block bei leerer Allowlist bleibt gruen", [CONFIG], [], "2026-11-09", 0,
     ["geprueft 0"])

second_module = BASELINE + [finding("GO-2026-6909", "github.com/russellhaering/goxmldsig", "Validate")]
case("gleiche ID in zweitem, nicht gelistetem Modul wird rot", second_module, REAL_ENTRIES,
     "2026-11-09", 1, ["GO-2026-6909", "goxmldsig"])

# Stream-Parser: govulncheck schreibt mehrzeilige Objekte hintereinander
stream = "\n".join(json.dumps(o, indent=2) for o in BASELINE)
if len(parse_stream(stream)) != len(BASELINE):
    FAILURES.append("parse_stream: Objektzahl stimmt nicht")
else:
    print("ok  Stream-Parser liest mehrzeilige Objekte")

# CLI-Ende-zu-Ende: exit-Code kommt am Prozess an, kaputte Eingabe ist rot
with tempfile.TemporaryDirectory() as td:
    good = pathlib.Path(td) / "good.json"
    good.write_text(stream, encoding="utf-8")
    bad = pathlib.Path(td) / "bad.json"
    bad.write_text(stream[: len(stream) // 2], encoding="utf-8")
    script = str(pathlib.Path(__file__).resolve().parent / "check_govulncheck.py")
    for label, path, today, want in (("CLI gruen", good, "2026-11-09", 0),
                                     ("CLI rot nach Ablauf", good, "2026-12-10", 1),
                                     ("CLI rot bei abgeschnittener Eingabe", bad, "2026-11-09", 1)):
        rc = subprocess.run([sys.executable, script, "--input", str(path), "--today", today],
                            capture_output=True, text=True).returncode
        if rc != want:
            FAILURES.append(f"{label}: exit {rc}, erwartet {want}")
        else:
            print(f"ok  {label}")

if FAILURES:
    print("\n".join(["", "FEHLGESCHLAGEN:"] + FAILURES))
    sys.exit(1)
print("check_govulncheck Selbsttest: alle Faelle gruen")
