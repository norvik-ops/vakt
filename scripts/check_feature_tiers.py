#!/usr/bin/env python3
"""Stufenzusagen-Guard: beworbene Lizenzstufe == tatsaechliches Gate im Code.

Eine Aussage wie „PDF audit exports | Community | Pro" ist eine Tatsachenbehauptung
ueber das Produkt. Steht sie schief, ist das derselbe UWG-Fall wie ein erfundener
Wettbewerberpreis (§§ 5, 6 UWG) — entweder wird etwas als kostenlos beworben, das
402 antwortet, oder etwas als bezahlt verkauft, das jeder umsonst bekommt.

Am 2026-08-08 fand eine vollstaendige Inventur genau das, in beide Richtungen:
sechs Frameworks (DSGVO-TOM, CIS, KRITIS, C5, ISO 27017/27018) standen als Community
im README und haengen an FeatureMultiFramework; Webhooks und die Evidence-Historie
wurden als Pro verkauft und sind ungegatet; „Custom controls" wurde als Pro-Merkmal
verkauft, obwohl es dafuer nicht einmal eine Route gibt.

QUELLE DER WAHRHEIT IST AUSSCHLIESSLICH DER GO-CODE. Dieser Check haelt NIRGENDWO
eine Tier-Zuordnung von Hand — er liest sie. Die Registry unten sagt nur, WO im Code
eine Faehigkeit verankert ist (Feature-Konstante oder Route-Literal), nie WELCHE Stufe
sie hat. Eine handgepflegte Sollstufe waere genau die Drift, die der Check verhindern
soll: sie wuerde beim naechsten Tier-Umzug im Code stillschweigend falsch.

Geprueft werden vier Flaechen, jede mit einem eigenen, mechanischen Extraktor:

  E1  Markdown-Tier-Tabellen   (README, Wiki, Marketing) — zeilengenau
  E2  sites/vakt Pricing.astro (COMMUNITY_FEATURES / PRO_FEATURES)
  E3  frontend Layout.tsx      (Navigations-Badges `pro:` / `tier:`)
  E4  OpenAPI-Spec             (x-license-tier / x-pro-feature) — die gespiegelte
                                 oeffentliche Fassung liegt im Mirror unter
                                 docs/api/openapi.yaml und wurde von KEINEM
                                 Doku-Guard angesehen (die filtern auf *.md).

WAS DIESER CHECK NICHT ANSIEHT — und das gehoert hierher, nicht in eine Fussnote:
  * Fliesstext. „Pro schaltet spezialisierte Frameworks frei" ist keine mechanisch
    pruefbare Zuordnung; Prosa zu parsen wuerde Zusagen halluzinieren, und ein Gate,
    das eine Zusage erfindet, ist schlimmer als eines, das zugibt, sie nicht zu sehen.
  * Historie (Sprints, Stories, ADRs, CHANGELOG, Reports) haelt absichtlich alte
    Staende fest. Eine Korrektur dort waere eine Faelschung des Protokolls.
  * i18n-Kataloge des Frontends: dort steht die Zusage als Fliesstext in einem
    JSON-String, siehe erster Punkt.
  * MENGENBEGRENZTE Faehigkeiten. Der KI-Berater ist in Community enthalten und dort
    auf 25 Anfragen/Monat begrenzt (ai/usage.go: CEMonthlyLimit), in Pro unbegrenzt —
    er gehoert also zu Recht in BEIDE Spalten. Ein Listen-Extraktor kann „in beiden"
    nicht ausdruecken und wuerde die korrekte Seite rot faerben. Bewusste Luecke: die
    Behauptung „KI-Berater nur in Pro" faengt dieser Check NICHT. Wer eine Quote
    einfuehrt, prueft sie von Hand gegen ai/usage.go.
Alles, was ein Extraktor sieht, aber nicht zuordnen kann, wird GEZAEHLT UND BENANNT
(`skipped`) — nie stillschweigend uebersprungen.

    python3 scripts/check_feature_tiers.py            # Bericht + Exit-Code
    python3 scripts/check_feature_tiers.py --selftest # Nicht-Vakuitaets-Nachweis
"""
from __future__ import annotations

import os
import re
import sys

ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
os.chdir(ROOT)

LICENSE_GO = "backend/internal/license/license.go"
TIERS_GO = "backend/internal/shared/platform/features/tiers.go"
COMPLY_HANDLER = "backend/internal/modules/vaktcomply/handler.go"
OPENAPI = "backend/internal/shared/apidocs/openapi.yaml"

# UNSOLD ist keine Stufe, sondern ihr Gegenteil: das Merkmal existiert, gatet echte
# Routen und liegt in KEINEM verkaeuflichen Tarif (features.UnsoldFeatures). Der Wert
# heisst deshalb wie die Kategorie im Code und NICHT "enterprise" — den Tarif gibt es
# seit dem 2026-08-08 nicht mehr, und ein Marker, der ihn in der oeffentlich
# gespiegelten Spec weiterfuehrt, verkauft eine Stufe, die niemand kaufen kann.
COMMUNITY, PRO, UNSOLD = "community", "pro", "unsold"

errors: list[str] = []
skipped: list[str] = []


def err(msg: str) -> None:
    errors.append(msg)


# ══ 1. Wahrheit aus dem Go-Code lesen ════════════════════════════════════════


def feature_strings() -> dict[str, str]:
    """Go-Konstantenname -> Feature-String, aus license.go."""
    text = open(LICENSE_GO, encoding="utf-8").read()
    return dict(re.findall(r"\b(Feature\w+)\s*=\s*\"([a-z0-9_]+)\"", text))


def tier_of_feature() -> dict[str, str]:
    """Feature-String -> Stufe, aus tiers.go. Kein Wert steht hier von Hand."""
    consts = feature_strings()
    text = open(TIERS_GO, encoding="utf-8").read()

    pro_block = re.search(r"var ProTier = \[\]string\{(.*?)\n\}", text, re.S)
    unsold_block = re.search(r"var UnsoldFeatures = \[\]string\{(.*?)\n\}", text, re.S)
    if not pro_block or not unsold_block:
        err(f"{TIERS_GO}: ProTier/UnsoldFeatures nicht gefunden — Guard kann nichts pruefen")
        return {}

    def names(block: str) -> list[str]:
        # Kommentare entfernen, sonst zaehlt eine im Fliesstext erwaehnte Konstante mit.
        stripped = re.sub(r"//.*", "", block)
        return re.findall(r"\bFeature\w+", stripped)

    out: dict[str, str] = {}
    for n in names(pro_block.group(1)):
        if n in consts:
            out[consts[n]] = PRO
    for n in names(unsold_block.group(1)):
        if n in consts:
            out[consts[n]] = UNSOLD
    return out


def framework_tiers() -> dict[str, str]:
    """Framework-Name -> Stufe, aus frameworkFeatureGate in vaktcomply/handler.go.

    Ein Framework, das NICHT in der Map steht, ist ungegatet = Community. Genau so
    herum, nicht als Allowlist: sonst faellt jedes neu hinzugefuegte Framework durch.
    """
    consts = feature_strings()
    ftier = tier_of_feature()
    text = open(COMPLY_HANDLER, encoding="utf-8").read()
    m = re.search(r"var frameworkFeatureGate = map\[string\]features\.Feature\{(.*?)\n\}", text, re.S)
    if not m:
        err(f"{COMPLY_HANDLER}: frameworkFeatureGate nicht gefunden")
        return {}
    body = re.sub(r"//.*", "", m.group(1))
    out: dict[str, str] = {}
    for name, const in re.findall(r"\"([A-Z0-9\-]+)\":\s*features\.(Feature\w+)", body):
        out[name] = ftier.get(consts.get(const, ""), PRO)
    return out


_GATE_ALIAS_RE = re.compile(r"(\w+)\s*:=\s*features\.Require\(features\.(Feature\w+)\)")


def route_tier(anchor: str) -> tuple[str, str]:
    """Stufe einer Route, gelesen an der Route-Registrierung selbst.

    anchor: "<datei>::<literal>", z.B.
        backend/internal/modules/vaktcomply/routes.go::"/evidence/:id/history"

    Rueckgabe: (stufe, beleg). Keine Gate-Middleware auf der Zeile == Community.
    Das ist der Punkt: „ungegatet" wird nicht behauptet, sondern am Code abgelesen.
    """
    path, literal = anchor.split("::", 1)
    if not os.path.exists(path):
        return "", f"{path}: Datei fehlt"
    text = open(path, encoding="utf-8").read()
    aliases = {a: b for a, b in _GATE_ALIAS_RE.findall(text)}
    consts = feature_strings()
    ftier = tier_of_feature()

    for lineno, line in enumerate(text.split("\n"), 1):
        if literal not in line or not re.search(r"\.(GET|POST|PUT|PATCH|DELETE)\(", line):
            continue
        beleg = f"{path}:{lineno}"
        m = re.search(r"features\.Require\(features\.(Feature\w+)\)", line)
        if m:
            return ftier.get(consts.get(m.group(1), ""), PRO), beleg
        for alias, const in aliases.items():
            if re.search(rf",\s*{alias}\b", line):
                return ftier.get(consts.get(const, ""), PRO), beleg
        return COMMUNITY, beleg
    return "", f"{path}: Route-Literal {literal} nicht gefunden"


# ══ 2. Registry: WO eine Faehigkeit im Code haengt — nie WELCHE Stufe ════════
#
# `labels` sind die Schreibweisen, unter denen die Faehigkeit in Verkaufstexten
# auftaucht (Regex, case-insensitiv, auf die Zeile/den Listeneintrag angewandt).
# `feature` ODER `route` liefert die Stufe; genau eins von beiden ist gesetzt.
# `nav` sind Frontend-Routen, `api` OpenAPI-Pfadpraefixe.

REGISTRY: list[dict] = [
    # ── Frameworks (Stufe aus frameworkFeatureGate) ──────────────────────────
    {"key": "fw:NIS2", "framework": "NIS2", "labels": [r"^NIS2$", r"^NIS2 \(EU"]},
    {"key": "fw:ISO27001", "framework": "ISO27001", "labels": [r"^ISO ?27001"]},
    {"key": "fw:DSGVO-TOM", "framework": "DSGVO-TOM",
     "labels": [r"GDPR Art\.? 32", r"DSGVO[- ]TOM", r"GDPR TOM"]},
    {"key": "fw:CIS", "framework": "CIS", "labels": [r"CIS Controls"]},
    {"key": "fw:KRITIS", "framework": "KRITIS", "labels": [r"KRITIS"]},
    {"key": "fw:C5", "framework": "C5", "labels": [r"BSI C5"]},
    {"key": "fw:ISO27017", "framework": "ISO27017", "labels": [r"ISO ?27017"]},
    {"key": "fw:ISO27018", "framework": "ISO27018", "labels": [r"ISO ?27018"]},
    {"key": "fw:BSI", "framework": "BSI", "labels": [r"BSI IT-Grundschutz"],
     "nav": ["/vaktcomply/bsi/target-objects", "/vaktcomply/bsi/cockpit",
             "/vaktcomply/bsi/reports", "/vaktcomply/bcm",
             # BCM haengt komplett an bsiPro := features.Require(FeatureBSIGrundschutz)
             "/vaktcomply/bcm/bia", "/vaktcomply/bcm/recovery-plans",
             "/vaktcomply/bcm/emergency-contacts"]},
    {"key": "fw:EUAIACT", "framework": "EUAIACT", "labels": [r"EU AI Act"],
     "nav": ["/vaktcomply/eu-ai-act/dashboard", "/vaktcomply/ai-systems"],
     "api": ["/vaktcomply/ai-systems"]},
    {"key": "fw:CRA", "framework": "CRA", "labels": [r"EU CRA", r"^CRA\b"]},
    {"key": "fw:TISAX", "framework": "TISAX", "labels": [r"TISAX"],
     "api": ["/vaktcomply/frameworks/tisax", "/vaktcomply/frameworks/{id}/tisax",
             "/vaktcomply/frameworks/TISAX"]},
    {"key": "fw:DORA", "framework": "DORA", "labels": [r"^DORA\b"],
     "nav": ["/vaktcomply/dora/dashboard", "/vaktcomply/resilience-tests"],
     "api": ["/vaktcomply/dora", "/vaktcomply/frameworks/dora",
             "/vaktcomply/frameworks/DORA", "/vaktcomply/resilience-tests"]},
    {"key": "fw:ISO42001", "framework": "ISO42001", "labels": [r"ISO ?42001"],
     "api": ["/vaktcomply/frameworks/ISO42001"]},

    # ── Faehigkeiten mit eigener Feature-Konstante ───────────────────────────
    {"key": "audit_pdf", "feature": "FeatureAuditPDF",
     "labels": [r"PDF audit export", r"Audit-PDF", r"Audit PDF"]},
    {"key": "sso", "feature": "FeatureSSO", "labels": [r"SSO ?/ ?OIDC", r"OIDC ?/ ?OAuth2? SSO"],
     "api": ["/admin/org/oidc-config"]},
    {"key": "saml", "feature": "FeatureSAMLAuth", "labels": [r"SAML ?2\.0"],
     "api": ["/admin/org/saml-config"]},
    {"key": "scim", "feature": "FeatureSCIMProvisioning",
     "labels": [r"SCIM[- ]?(2\.0 )?[Pp]rovision"]},
    {"key": "siem", "feature": "FeatureSIEM", "labels": [r"SIEM[- ]export"]},
    {"key": "api", "feature": "FeatureAPI", "labels": [r"API[- ]Zugang", r"API access"]},
    {"key": "nis2_reporting", "feature": "FeatureNIS2Reporting",
     "labels": [r"NIS2 reporting assistant", r"NIS2-Meldungsassistent"]},
    {"key": "supplier", "feature": "FeatureSupplierPortal",
     "labels": [r"Supplier portal", r"Lieferantenportal"],
     "nav": ["/vaktcomply/suppliers"]},
    {"key": "scan_adv", "feature": "FeatureSecPulse",
     "labels": [r"SBOM scanning", r"EOL tracking"],
     "nav": ["/vaktscan/reports", "/vaktscan/eol"]},
    {"key": "vault_adv", "feature": "FeatureSecVault",
     "labels": [r"Git repo secret scanning", r"Git-Leak-Scans"],
     "nav": ["/vaktvault/git-scans", "/vaktvault/access-reviews"]},
    {"key": "aware_adv", "feature": "FeatureSecReflex",
     "labels": [r"phishing campaigns", r"Phishing-Kampagnen"],
     "nav": ["/vaktaware/campaigns", "/vaktaware/templates", "/vaktaware/target-groups"]},
    {"key": "privacy_adv", "feature": "FeatureSecPrivacy",
     "labels": [r"DPIA workflows", r"DPIA[- ]Workflows"],
     "nav": ["/vaktprivacy/dpia", "/vaktprivacy/transfers",
             "/vaktprivacy/deletion-reminders", "/vaktprivacy/privacy-design"]},

    # ── Faehigkeiten OHNE Feature-Konstante: Stufe an der Route abgelesen ────
    # Genau hier sassen die Falschzusagen in die andere Richtung: als Pro verkauft,
    # im Code fuer jeden offen. Die Route ist der Beleg, nicht diese Zeile.
    {"key": "webhooks",
     "route": 'backend/internal/shared/platform/webhooks/handler.go::g.POST("", h.Create',
     "labels": [r"Webhook integrations", r"Webhook-Integrationen"]},
    {"key": "evidence_versioning",
     "route": 'backend/internal/modules/vaktcomply/routes.go::"/evidence/:id/history"',
     "labels": [r"Evidence versioning", r"Evidence-Versionierung"]},
    {"key": "ccm",
     "route": 'backend/internal/modules/vaktcomply/routes.go::"/ccm/checks"',
     "labels": [], "nav": ["/vaktcomply/ccm"]},
    {"key": "avv",
     "route": 'backend/internal/modules/vaktprivacy/routes.go::"/avvs"',
     "labels": [], "nav": ["/vaktprivacy/avv"]},
    {"key": "ldap",
     "route": 'backend/internal/admin/routes.go::"/org/ldap"',
     "labels": [r"LDAP ?/ ?Active Directory", r"LDAP[- ]Sync"]},
]

# Merkmale, die in Verkaufstexten standen und im Code NICHT existieren. Sie duerfen
# nirgends mehr auftauchen — eine falsche Tatsachenbehauptung ist kein Stufenfehler,
# sondern eine Erfindung, und wird deshalb hart abgelehnt statt umgestuft.
PHANTOM_CLAIMS = {
    r"Custom controls": "es gibt keine Route, mit der ein eigenes Control angelegt wird "
                        "(kein POST /controls in vaktcomply/routes.go)",
    # Absichtlich OHNE Ausnahme fuer erklaerende Erwaehnungen: eine Bedingung wie
    # „Zeile ist ein Zitat" waere trivial erfuellbar und machte die Regel lautlos
    # wirkungslos. Wer die Abwesenheit dokumentieren will, umschreibt sie (siehe die
    # datierten Hinweise in msp-onboarding.md und pricing-governance.md).
    r"White-Label-(PDF|Report)": "kein PDF-Renderer liest organizations.logo_url",
    r"Advanced PDF \(branding": "im PDF-Layout existiert kein Branding-Feld",
}


def resolved_registry() -> list[dict]:
    """Registry + aufgeloeste Stufe je Eintrag (aus dem Code, mit Beleg)."""
    ftier = tier_of_feature()
    fwtier = framework_tiers()
    consts = feature_strings()
    out = []
    for entry in REGISTRY:
        e = dict(entry)
        if "framework" in entry:
            fw = entry["framework"]
            e["tier"] = fwtier.get(fw, COMMUNITY)
            e["beleg"] = f"frameworkFeatureGate[{fw}]" if fw in fwtier else \
                         f"{fw} steht nicht in frameworkFeatureGate = ungegatet"
        elif "feature" in entry:
            fs = consts.get(entry["feature"], "")
            e["tier"] = ftier.get(fs, COMMUNITY)
            e["beleg"] = f"tiers.go: {entry['feature']}"
        else:
            tier, beleg = route_tier(entry["route"])
            if not tier:
                err(f"Registry-Anker nicht aufloesbar ({entry['key']}): {beleg}")
                continue
            e["tier"] = tier
            e["beleg"] = beleg
        e["res"] = [re.compile(p, re.I) for p in entry.get("labels", [])]
        out.append(e)
    return out


def match_label(reg: list[dict], text: str) -> dict | None:
    for e in reg:
        for rx in e["res"]:
            if rx.search(text):
                return e
    return None


# ══ 3. Extraktoren ═══════════════════════════════════════════════════════════

# Nur aktuelle, kundenwirksame Flaechen. Historie bleibt bewusst aussen vor.
MD_SURFACES = [
    "README.md",
    "OVERVIEW.md",
    "docs/wiki/modules/comply.md",
    "docs/wiki/faq.md",
    "docs/wiki/configuration.md",
    "docs/wiki/enterprise-sso.md",
    "docs/wiki/api-reference.md",
    "docs/wiki/msp-onboarding.md",
    "docs/marketing/positioning.md",
    "docs/marketing/battlecards.md",
    "docs/marketing/roundup-targets.md",
    "docs/business/pricing-governance.md",
]

_YES = re.compile(r"[✅✓]|\bJa\b|\bYes\b|Unlimited|Unbegrenzt|\d+\s*req")
_COMMUNITY_COL = re.compile(r"community|kostenlos|\bfree\b", re.I)
_PAID_COL = re.compile(r"\bpro\b", re.I)


def extract_md_tables(reg: list[dict]) -> int:
    """E1: Markdown-Tabellen mit einer Community- UND einer Pro/Enterprise-Spalte.

    Zeilengenau, weil eine Markdown-Tabellenzeile eine abgeschlossene Aussage ist.
    """
    n = 0
    for f in MD_SURFACES:
        if not os.path.exists(f):
            continue
        lines = open(f, encoding="utf-8", errors="ignore").read().split("\n")
        cols: dict[str, int] | None = None
        for lineno, line in enumerate(lines, 1):
            if not line.lstrip().startswith("|"):
                cols = None
                continue
            cells = [c.strip() for c in line.strip().strip("|").split("|")]
            if cols is None:
                comm = [i for i, c in enumerate(cells) if _COMMUNITY_COL.search(c)]
                paid = [i for i, c in enumerate(cells) if _PAID_COL.search(c)]
                if comm and paid:
                    cols = {"community": comm[0], "paid": paid[0], "paid_name": PRO}
                continue
            if set(line.replace("|", "").replace(" ", "")) <= set(":-"):
                continue  # Trennzeile
            if max(cols["community"], cols["paid"]) >= len(cells):
                continue
            label = re.sub(r"[*`\[\]]", "", cells[0]).strip()
            if not label:
                continue
            e = match_label(reg, label)
            if e is None:
                skipped.append(f"{f}:{lineno}: Tabellenzeile '{label}' keiner Faehigkeit zugeordnet")
                continue
            n += 1
            c_cell, p_cell = cells[cols["community"]], cells[cols["paid"]]
            claimed = (COMMUNITY if _YES.search(c_cell)
                       else cols["paid_name"] if _YES.search(p_cell)
                       else None)
            if claimed is None:
                skipped.append(f"{f}:{lineno}: '{label}' — Zellen ohne erkennbare Zusage")
                continue
            if claimed != e["tier"]:
                err(f"{f}:{lineno}: '{label}' wird als {claimed.upper()} beworben, "
                    f"im Code ist es {e['tier'].upper()} ({e['beleg']})")
    return n


PRICING_ASTRO = "sites/vakt/src/components/Pricing.astro"


def extract_pricing_astro(reg: list[dict]) -> int:
    """E2: die lebende Preisseite — COMMUNITY_FEATURES / PRO_FEATURES."""
    if not os.path.exists(PRICING_ASTRO):
        return 0
    text = open(PRICING_ASTRO, encoding="utf-8").read()
    lines = text.split("\n")
    n = 0
    for var, tier in (("COMMUNITY_FEATURES", COMMUNITY), ("PRO_FEATURES", PRO)):
        m = re.search(rf"const {var}[^=]*= \[(.*?)\n\]", text, re.S)
        if not m:
            skipped.append(f"{PRICING_ASTRO}: {var} nicht gefunden")
            continue
        start = text[:m.start()].count("\n")
        for off, raw in enumerate(m.group(1).split("\n")):
            item = re.search(r"'([^']+)'", raw)
            if not item:
                continue
            lineno = start + off + 1
            label = item.group(1)
            e = match_label(reg, label)
            if e is None:
                skipped.append(f"{PRICING_ASTRO}:{lineno}: '{label[:48]}' keiner Faehigkeit zugeordnet")
                continue
            n += 1
            if e["tier"] != tier:
                err(f"{PRICING_ASTRO}:{lineno}: '{label[:48]}' steht in {var} "
                    f"(= {tier.upper()}), im Code ist es {e['tier'].upper()} ({e['beleg']})")
    del lines
    return n


LAYOUT_TSX = "frontend/src/shared/components/Layout.tsx"


def extract_nav_badges(reg: list[dict]) -> int:
    """E3: App-Navigation — `pro: true` / `tier: 'unsold'` je Route.

    Beide Richtungen sind Fehler: ein Badge auf einer freien Seite verkauft etwas,
    das jeder hat; eine fehlende Markierung schickt Community-Nutzer ungewarnt in
    ein 402.
    """
    if not os.path.exists(LAYOUT_TSX):
        return 0
    nav_of = {}
    for e in reg:
        for path in e.get("nav", []):
            nav_of[path] = e
    n = 0
    for lineno, line in enumerate(open(LAYOUT_TSX, encoding="utf-8"), 1):
        m = re.search(r"path:\s*'([^']+)'", line)
        if not m or "label:" not in line:
            continue
        path = m.group(1)
        e = nav_of.get(path)
        if e is None:
            # Ohne Badge steht dort keine Zusage — nichts zu pruefen, nichts zu melden.
            # MIT Badge steht dort sehr wohl eine, die dieser Guard nicht belegen kann.
            # Die still zu ueberspringen waere genau die Teilmengen-Falle.
            if re.search(r"\bpro:\s*true", line):
                skipped.append(f"{LAYOUT_TSX}:{lineno}: '{path}' traegt ein Pro-Badge, "
                               f"ist aber keiner Faehigkeit zugeordnet")
            continue
        n += 1
        has_badge = bool(re.search(r"\bpro:\s*true", line))
        tm = re.search(r"tier:\s*'(\w+)'", line)
        claimed = (tm.group(1) if tm else PRO) if has_badge else COMMUNITY
        if claimed != e["tier"]:
            err(f"{LAYOUT_TSX}:{lineno}: Navigationseintrag '{path}' ist als "
                f"{claimed.upper()} markiert, im Code ist er {e['tier'].upper()} ({e['beleg']})")
    return n


def extract_openapi(reg: list[dict]) -> int:
    """E4: die OpenAPI-Spec — im Public Mirror liegt sie als docs/api/openapi.yaml.

    Diese Flaeche ist der Grund fuer diesen Extraktor: sie geht bei jedem Push nach
    main oeffentlich raus und in generierte SDKs, und KEIN Doku-Guard hat sie je
    angesehen, weil alle auf *.md filtern.
    """
    if not os.path.exists(OPENAPI):
        return 0
    api_of: list[tuple[str, dict]] = []
    for e in reg:
        for prefix in e.get("api", []):
            api_of.append((prefix, e))
    api_of.sort(key=lambda t: -len(t[0]))

    n = 0
    cur_path = None
    cur_op_line = None
    for lineno, line in enumerate(open(OPENAPI, encoding="utf-8"), 1):
        pm = re.match(r"^  (/\S+):\s*$", line)
        if pm:
            cur_path = pm.group(1)
            continue
        if re.match(r"^    (get|post|put|patch|delete):", line):
            cur_op_line = lineno
            continue
        if cur_path is None:
            continue
        marker = None
        mm = re.match(r"^\s+x-license-tier:\s*(\w+)", line)
        if mm:
            marker = mm.group(1)
        elif re.match(r"^\s+x-pro-feature:\s*true", line):
            marker = PRO
        if marker is None:
            continue
        e = next((e for p, e in api_of if cur_path.startswith(p)), None)
        if e is None:
            skipped.append(f"{OPENAPI}:{lineno}: Pfad {cur_path} traegt Stufenmarker "
                           f"'{marker}', ist aber keiner Faehigkeit zugeordnet")
            continue
        n += 1
        if marker != e["tier"]:
            err(f"{OPENAPI}:{lineno}: Operation unter {cur_path} (ab Zeile {cur_op_line}) "
                f"ist als {marker.upper()} markiert, im Code ist sie "
                f"{e['tier'].upper()} ({e['beleg']})")
    return n


PHANTOM_SURFACES = MD_SURFACES + [PRICING_ASTRO, LAYOUT_TSX]


def check_phantoms() -> int:
    """Merkmale, die es im Code nicht gibt, duerfen in keinem Verkaufstext stehen."""
    n = 0
    for f in PHANTOM_SURFACES:
        if not os.path.exists(f):
            continue
        for lineno, line in enumerate(open(f, encoding="utf-8", errors="ignore"), 1):
            for pat, why in PHANTOM_CLAIMS.items():
                if re.search(pat, line, re.I):
                    n += 1
                    err(f"{f}:{lineno}: bewirbt '{pat}' — {why}")
    return n


# ══ 4. Lauf ══════════════════════════════════════════════════════════════════


def run() -> int:
    reg = resolved_registry()
    if errors:  # Wahrheitsquelle unlesbar: nichts vortaeuschen
        for e in errors:
            print(f"  ✗ {e}")
        return 1

    counts = {
        "Markdown-Tier-Tabellen": extract_md_tables(reg),
        "Pricing.astro": extract_pricing_astro(reg),
        "Navigations-Badges": extract_nav_badges(reg),
        "OpenAPI-Stufenmarker": extract_openapi(reg),
    }
    check_phantoms()
    total = sum(counts.values())

    print("Stufenzusagen-Guard — Zusagen gegen die Gates im Go-Code geprueft:")
    for k, v in counts.items():
        print(f"  {v:4d}  {k}")
    print(f"  {total:4d}  gesamt, {len(skipped)} nicht zuordenbar")

    leer = [k for k, v in counts.items() if v == 0]
    if leer:
        print(f"  ✗ Extraktor ohne einen einzigen Fund: {', '.join(leer)}. "
              f"Das ist ein Guard-Defekt, kein sauberes Repo — eine Flaeche, die "
              f"nichts mehr liefert, meldet dasselbe Gruen wie eine fehlerfreie.")
        return 1

    if skipped:
        print("\nNicht zuordenbar (ungeprueft — kein Urteil, aber auch kein Freispruch):")
        for s in skipped:
            print(f"  ~ {s}")

    if errors:
        print(f"\n{len(errors)} Widerspruch/Widersprueche zwischen Verkaufstext und Code:")
        for e in errors:
            print(f"  ✗ {e}")
        print("\nQuelle der Wahrheit ist der Code. Ist der Text falsch, Text korrigieren.")
        print("Ist das Gate falsch, ist das eine Produktentscheidung — nicht hier loesen.")
        print("Regeln: docs/dev/marketing-claims-guide.md → 'Stufenzusagen'")
        return 1

    print("\nOK — jede zugeordnete Stufenzusage deckt sich mit dem Gate im Code.")
    return 0


def selftest() -> int:
    """Nicht-Vakuitaet: der Guard MUSS bei einer wiedereingefuegten Falschzusage
    rot werden — und die Datei samt Zeile benennen.

    Ohne diesen Nachweis ist eine gruene Meldung wertlos: ein Extraktor, der nichts
    findet, meldet dasselbe Gruen wie ein sauberes Repo.
    """
    import tempfile, shutil, subprocess

    cases = [
        ("README.md", "| GDPR Art. 32 (TOM) | — | ✅ |", "| GDPR Art. 32 (TOM) | ✅ | ✅ |",
         "Framework als Community beworben, das Pro-gegatet ist"),
        ("README.md", "| Webhook integrations | ✅ | ✅ |", "| Webhook integrations | — | ✅ |",
         "ungegatetes Merkmal als Pro verkauft"),
        ("README.md", "| SCIM provisioning | — | ✅ |", "| Custom controls | — | ✅ |",
         "Merkmal beworben, das im Code nicht existiert"),
        (LAYOUT_TSX, "icon: Handshake },", "icon: Handshake, pro: true },",
         "Pro-Badge auf einer ungegateten Seite"),
        (OPENAPI, "x-license-tier: unsold", "x-pro-feature: true",
         "unverkaeufliche Operation in der oeffentlichen Spec als Pro markiert"),
    ]
    tmp = tempfile.mkdtemp(prefix="feature-tiers-selftest-")
    ok = True
    try:
        for path, old, new, was in cases:
            backup = os.path.join(tmp, os.path.basename(path))
            shutil.copy(path, backup)
            text = open(path, encoding="utf-8").read()
            if old not in text:
                print(f"  ✗ Selftest-Anker fehlt in {path}: {old[:50]}")
                ok = False
                continue
            open(path, "w", encoding="utf-8").write(text.replace(old, new, 1))
            r = subprocess.run([sys.executable, __file__], capture_output=True, text=True)
            shutil.copy(backup, path)
            named = any(path in ln and "✗" in ln for ln in r.stdout.split("\n"))
            if r.returncode == 0 or not named:
                print(f"  ✗ NICHT erkannt ({was}) — Guard blieb gruen oder nannte "
                      f"{path} nicht beim Namen")
                ok = False
            else:
                hit = next(ln.strip() for ln in r.stdout.split("\n")
                           if path in ln and "✗" in ln)
                print(f"  ✓ erkannt ({was}):\n      {hit}")
    finally:
        shutil.rmtree(tmp, ignore_errors=True)

    r = subprocess.run([sys.executable, __file__], capture_output=True, text=True)
    if r.returncode != 0:
        print("  ✗ Baseline ist nach dem Selftest nicht mehr gruen — Datei nicht sauber "
              "wiederhergestellt")
        ok = False
    else:
        print("  ✓ Baseline nach Wiederherstellung wieder gruen")
    return 0 if ok else 1


if __name__ == "__main__":
    sys.exit(selftest() if "--selftest" in sys.argv else run())
