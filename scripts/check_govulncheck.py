#!/usr/bin/env python3
# Copyright (c) 2026 NorvikOps. All rights reserved.
# SPDX-License-Identifier: Elastic-2.0
"""
govulncheck-Gate mit befristeter Allowlist (ADR-0093).

Liest die JSON-Ausgabe von `govulncheck -format json` und entscheidet selbst,
ob CI rot wird. Im JSON-Modus endet govulncheck immer mit exit 0 — ohne dieses
Gate waere der Schritt „Vulnerability scan (govulncheck)" wirkungslos.

ROT (exit 1), wenn:
  · ein erreichbarer Fund (Trace bis auf Funktionsebene, genau das, woran der
    Textmodus scheitert) nicht auf der Allowlist steht — oder dort mit einem
    anderen Modul steht;
  · ein Allowlist-Eintrag abgelaufen ist (heute > expires) und der Fund noch
    gemeldet wird;
  · dieselbe ID zusaetzlich in einem Modul gemeldet wird, das der Eintrag
    nicht nennt — eine Ausnahme gilt fuer genau ein Modul;
  · ein Eintrag unvollstaendig ist (id, module, reason, evidence, added,
    expires), ein Datum nicht lesbar ist, added in der Zukunft liegt, expires
    vor added liegt oder mehr als 31 Tage nach added — so kann niemand eine
    Ausnahme „bis 2099" eintragen, auch nicht ueber ein vordatiertes added;
  · die Eingabe nicht vollstaendig lesbar ist oder keinen config-Block hat
    (dann hat govulncheck nicht gelaufen, und Stille waere kein Befund);
  · config.scan_level nicht „symbol" ist — auf Modul- oder Paketebene gibt es
    keine erreichbaren Funde, das Gate waere still gruen;
  · die Ausgabe keinen einzigen finding-Block enthaelt, die Allowlist aber
    Eintraege hat. Schutz vor Format-Drift: Benennt govulncheck seine Bloecke
    um, saehe das Gate null Funde und meldete OK. Wer wirklich alles behoben
    hat, leert die Allowlist im selben PR.

WARNUNG (exit bleibt 0), wenn ein Eintrag nicht mehr gemeldet wird. Bewusst
nicht rot: Die Ablauffrist (max. 31 Tage) raeumt die Liste ohnehin auf, und ein
roter Altlast-Eintrag liesse eine Aenderung in der externen Vuln-DB (Advisory
zurueckgezogen, Symbole nachgetragen) jeden fremden PR blockieren. Abgelaufene
Eintraege ohne Fund sind ebenfalls nur eine Warnung.

Was dieses Gate NICHT prueft:
  · Funde auf Modul- oder Paketebene ohne erreichbaren Aufruf — sie zaehlen
    als „nicht erreichbar" und werden nur gezaehlt (wie im Textmodus).
  · ob die Begruendung im Eintrag stimmt. Das bleibt Review-Arbeit; das Gate
    erzwingt nur, dass es eine gibt und dass sie nach 31 Tagen neu faellig ist.
  · ob govulncheck die richtige Go-Version oder alle Pakete gescannt hat —
    das legt der CI-Schritt fest (go-package, work-dir).
  · Trivy-Funde (eigener Schritt, eigene .trivyignore).

Nenner: „geprueft N · erlaubt M · uebersprungen K" — N = erreichbare Funde
(je OSV-ID), M = davon durch die Allowlist gedeckt, K = nicht erreichbare
Funde, die das Gate bewusst nicht bewertet.

Aufruf:
  python3 scripts/check_govulncheck.py --input govulncheck.json \
      [--allowlist .github/govulncheck-allowlist.json] [--today YYYY-MM-DD]
"""

import argparse
import datetime
import json
import pathlib
import sys

ROOT = pathlib.Path(__file__).resolve().parent.parent
DEFAULT_ALLOWLIST = ROOT / ".github" / "govulncheck-allowlist.json"
MAX_DAYS = 31
REQUIRED_FIELDS = ("id", "module", "reason", "evidence", "added", "expires")


def parse_stream(text):
    """govulncheck -format json schreibt hintereinander gehaengte JSON-Objekte,
    kein Array. Wirft ValueError, wenn ein Teil nicht lesbar ist."""
    dec = json.JSONDecoder()
    i, objs = 0, []
    while True:
        while i < len(text) and text[i].isspace():
            i += 1
        if i >= len(text):
            return objs
        obj, i = dec.raw_decode(text, i)
        objs.append(obj)


def collect_findings(objs):
    """Gibt ({osv_id: {module, ...}} erreichbar, {osv_id} nicht erreichbar) zurueck."""
    reachable, unreachable = {}, set()
    for o in objs:
        f = o.get("finding")
        if not f:
            continue
        osv = f.get("osv", "")
        trace = f.get("trace") or []
        if not osv or not trace:
            continue
        top = trace[0]
        if top.get("function"):
            reachable.setdefault(osv, set()).add(top.get("module", ""))
        else:
            unreachable.add(osv)
    return reachable, unreachable - set(reachable)


def _date(value):
    return datetime.date.fromisoformat(value)


def validate_entries(entries, today):
    """Formale Pruefung der Allowlist. Gibt (gueltige Eintraege je ID, Fehler) zurueck."""
    errors, valid = [], {}
    for n, e in enumerate(entries):
        label = e.get("id") or f"Eintrag #{n}"
        missing = [k for k in REQUIRED_FIELDS if not str(e.get(k, "")).strip()]
        if missing:
            errors.append(f"{label}: Pflichtfeld(er) fehlen: {', '.join(missing)}")
            continue
        try:
            added, expires = _date(e["added"]), _date(e["expires"])
        except ValueError:
            errors.append(f"{label}: Datum nicht lesbar (added={e['added']}, expires={e['expires']})")
            continue
        if added > today:
            errors.append(f"{label}: added {added} liegt in der Zukunft (heute {today})")
            continue
        if expires < added:
            errors.append(f"{label}: expires {expires} liegt vor added {added}")
            continue
        if (expires - added).days > MAX_DAYS:
            errors.append(f"{label}: Frist {(expires - added).days} Tage > {MAX_DAYS} Tage erlaubt")
            continue
        if e["id"] in valid:
            errors.append(f"{label}: doppelter Eintrag")
            continue
        valid[e["id"]] = dict(e, _expires=expires)
    return valid, errors


def evaluate(objs, entries, today):
    """Kern des Gates. Gibt (exit_code, Zeilen) zurueck."""
    lines, errors, warnings = [], [], []
    configs = [o["config"] for o in objs if isinstance(o.get("config"), dict)]
    if not configs:
        return 1, ["FEHLER: keine govulncheck-Ausgabe (config-Block fehlt) — nichts geprueft"]
    scan_level = configs[0].get("scan_level", "")
    if scan_level != "symbol":
        return 1, [f"FEHLER: scan_level ist {scan_level!r}, erwartet 'symbol' — ohne Symbolebene gibt es keine erreichbaren Funde"]
    if entries and not any("finding" in o for o in objs):
        return 1, ["FEHLER: Ausgabe enthaelt keinen einzigen finding-Block, die Allowlist aber "
                   f"{len(entries)} Eintrag/Eintraege — Format-Drift? Bei echter Fundfreiheit Allowlist leeren"]

    allow, entry_errors = validate_entries(entries, today)
    errors.extend(entry_errors)
    reachable, unreachable = collect_findings(objs)

    allowed = 0
    for osv in sorted(reachable):
        modules = sorted(reachable[osv])
        e = allow.get(osv)
        if e is None:
            errors.append(f"{osv} ({', '.join(modules)}): erreichbar und nicht auf der Allowlist")
        elif modules != [e["module"]]:
            errors.append(f"{osv}: Allowlist nennt Modul {e['module']}, gemeldet wird {', '.join(modules)}")
        elif today > e["_expires"]:
            errors.append(f"{osv} ({e['module']}): Ausnahme am {e['_expires']} abgelaufen — beheben oder neu begruenden")
        else:
            allowed += 1
            lines.append(f"  erlaubt: {osv} ({e['module']}) bis {e['_expires']} — {e['evidence']}")

    for osv in sorted(set(allow) - set(reachable)):
        warnings.append(f"{osv}: steht auf der Allowlist, wird aber nicht mehr gemeldet — Eintrag entfernen")

    for w in warnings:
        lines.append(f"WARNUNG: {w}")
    for err in errors:
        lines.append(f"FEHLER: {err}")
    lines.append(
        f"govulncheck-Gate: geprueft {len(reachable)} · erlaubt {allowed} · "
        f"uebersprungen {len(unreachable)} (nicht erreichbar) · Allowlist {len(entries)} Eintrag/Eintraege")
    if errors:
        lines.append("govulncheck-Gate: ROT")
        return 1, lines
    lines.append("govulncheck-Gate: OK")
    return 0, lines


def main(argv=None):
    ap = argparse.ArgumentParser(description=__doc__.split("\n")[1])
    ap.add_argument("--input", required=True, help="Datei mit govulncheck -format json")
    ap.add_argument("--allowlist", default=str(DEFAULT_ALLOWLIST))
    ap.add_argument("--today", default=None, help="YYYY-MM-DD (nur fuer Tests)")
    args = ap.parse_args(argv)

    today = _date(args.today) if args.today else datetime.date.today()
    try:
        objs = parse_stream(pathlib.Path(args.input).read_text(encoding="utf-8"))
    except (OSError, ValueError) as exc:
        print(f"FEHLER: govulncheck-Ausgabe nicht lesbar: {exc}")
        return 1
    try:
        entries = json.loads(pathlib.Path(args.allowlist).read_text(encoding="utf-8"))["entries"]
    except (OSError, ValueError, KeyError, TypeError) as exc:
        print(f"FEHLER: Allowlist nicht lesbar: {exc}")
        return 1

    code, lines = evaluate(objs, entries, today)
    print("\n".join(lines))
    return code


if __name__ == "__main__":
    sys.exit(main())
