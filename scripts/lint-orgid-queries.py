#!/usr/bin/env python3
"""
lint-orgid-queries.py — Check sqlc query files for missing org_id filters.

Since ADR-0042 removed row-level security, app-layer org_id scoping is the
only tenant isolation mechanism. Every SELECT/UPDATE/DELETE that touches a
multi-tenant table must include an org_id filter.

Usage: python3 scripts/lint-orgid-queries.py [--query-dir backend/db/queries]
Exit code 1 if violations are found; 0 otherwise.
"""
import sys
import re
import argparse
from pathlib import Path

# Queries that are intentionally global (no org_id filter needed).
# Key: "<filename>:<query_name>" — add a comment explaining why it's safe.
ALLOWLIST = {
    # HINWEIS (R1-SA03-05, 2026-08-06): Hier standen 20 Einträge für
    # `user_permissions.sql:{GetUserByEmail,GetUserByID,CreateUser,ListUsers,
    # GetOrgBySlug,CreateOrg,ListOrgs,GetOrgMember,GetRoleByName,ListRoles,
    # CreateRole,…}`. Keine dieser Queries existiert in backend/db/queries —
    # `user_permissions.sql` enthält seit dem initialen Monorepo-Commit (3f0a661)
    # genau drei Queries, alle zu user_module_permissions. Die Einträge waren also
    # nie greifend: sie prüften nichts, standen aber als vorab erteilte Ausnahme
    # bereit, falls jemand später einen dieser Namen anlegt — und hätten ihn dann
    # ungeprüft durchgelassen. Entfernt; der neue Stale-Check unten hält die Liste
    # ab jetzt gegenstandsbezogen. (Die Go-Seite dieser Zugriffe ist hand-gepflegtes
    # sqlc in internal/db und wird von Pass 1 ohnehin nie gelesen.)

    # ── vaktaware ─────────────────────────────────────────────────────────────
    # sr_targets.group_id is a FK into sr_target_groups (which has org_id);
    # org isolation is maintained via the caller owning the group.
    "vaktaware.sql:CountSRTargetsInGroup": "scoped by group_id FK; caller verifies group org ownership",
    # sr_campaigns has org_id; group_id is a safe surrogate here — caller verifies.
    "vaktaware.sql:GetSRCampaignGroupID": "SELECT by campaign PK; caller holds org-verified campaign reference",
    # COUNT events by campaign_id — same pattern; campaign is org-scoped.
    "vaktaware.sql:CountSREventsByType": "scoped by campaign_id FK; caller verifies campaign org ownership",
    # Public phishing-report webhook — lookup org by unique token (no auth, no tenant context yet).
    "vaktaware.sql:GetOrgByPhishReportToken": "public webhook lookup by unique phish_report_token; returns org id for subsequent scoped queries",
    # UPDATE organizations WHERE id = $2 — $2 is the org_id (primary key); $1 is the token value.
    "vaktaware.sql:SetOrgPhishReportToken": "UPDATE organizations by PK (org_id is param $2)",
    # SELECT name FROM organizations WHERE id = $1 — $1 is org_id passed by caller.
    "vaktaware.sql:GetSROrganizationName": "SELECT org name by PK (org_id is param $1)",
    # SELECT email FROM sr_targets WHERE id = $1 — used internally in the phishing flow after
    # org ownership is already verified via the campaign/group chain.
    "vaktaware.sql:GetSRTargetEmail": "SELECT target email by PK; org ownership verified earlier in call chain",

    # ── vaktcomply ────────────────────────────────────────────────────────────
    # Background job passes pre-fetched evidence IDs belonging to a single org.
    "vaktcomply.sql:MarkCKEvidenceExpiryNotified": "background worker passes org-filtered evidence IDs from a prior scoped query",
    # Junction-table DELETE by (supplier_id, risk_id) — handler verifies supplier and risk
    # org ownership before calling this.
    "vaktcomply.sql:UnlinkCKSupplierRisk": "junction-table DELETE by composite FK; handler verifies supplier/risk org ownership",
    # ck_questionnaire_questions.questionnaire_id FK → ck_questionnaires (has org_id);
    # all four question CRUD queries use questionnaire_id as implicit org scope.
    "vaktcomply.sql:NextCKQuestionOrderIdx": "scoped by questionnaire_id FK into org-scoped ck_questionnaires",
    "vaktcomply.sql:GetCKQuestion": "scoped by (id, questionnaire_id); questionnaire is org-scoped",
    "vaktcomply.sql:UpdateCKQuestion": "scoped by (id, questionnaire_id); questionnaire is org-scoped",
    "vaktcomply.sql:DeleteCKQuestion": "scoped by (id, questionnaire_id); questionnaire is org-scoped",
    "vaktcomply.sql:ListCKQuestions": "scoped by questionnaire_id FK into org-scoped ck_questionnaires",
    "vaktcomply.sql:ReorderCKQuestion": "scoped by (id, questionnaire_id); questionnaire is org-scoped",
    # ck_ccm_checks UPDATE by PK — handler verifies check ownership (has org_id in table).
    "vaktcomply.sql:UpdateCKCCMCheckEnabled": "UPDATE by PK; ck_ccm_checks has org_id, handler verifies before this call",
    # Background worker updates its own check row (system-internal, no user-supplied id).
    "vaktcomply.sql:UpdateCKCCMCheckLastRun": "background worker updates own check row by PK (system-internal, no user input)",
    # SELECT results by check_id — check is system-internal, org verified at check level.
    "vaktcomply.sql:ListCKCCMResults": "scoped by check_id; org isolation handled at ck_ccm_checks level",
    # SELECT/UPDATE from organizations by PK (org_id passed directly as param $1).
    "vaktcomply.sql:GetCKOrgApprovalRequired": "SELECT from organizations WHERE id = $1 (org_id passed as param)",
    "vaktcomply.sql:SetCKOrgApprovalRequired": "UPDATE organizations WHERE id = $1 (org_id passed as param)",
    # Public assessor portal: token-authenticated external user fills in assessment.
    # The assessment row was created by the org; the public endpoint has no org context.
    "vaktcomply.sql:UpdateCKAssessmentStatus": "public assessor portal: assessment UUID is the auth token; no separate org param possible",
    # Auditor portal: access counter on ck_auditor_links, authenticated via link token.
    "vaktcomply.sql:UpdateCKAuditorLinkAccess": "auditor portal counter update by PK; link token is the auth mechanism",
    "vaktcomply.sql:IncrementCKAuditorLinkUsage": "auditor portal usage counter by PK; link token is the auth mechanism",
    # Background jobs that intentionally iterate all orgs.
    "vaktcomply.sql:ListAllOrgIDs": "background worker job: intentionally iterates all orgs for daily snapshot",
    "vaktcomply.sql:ListActiveOrgIDs": "background worker job: intentionally iterates all non-deleted orgs",
    # SELECT org metadata by PK; $1 is org_id in all callers.
    "vaktcomply.sql:GetCKOrgSector": "SELECT from organizations WHERE id = $1 (org_id passed as param)",
    "vaktcomply.sql:UpdateCKOrgSector": "UPDATE organizations WHERE id = $1 (org_id passed as param)",
    "vaktcomply.sql:GetCKOrgName": "SELECT org name from organizations WHERE id = $1 (org_id passed as param)",
    # users is a global table (no org_id column); membership is in org_members.
    "vaktcomply.sql:GetUserDisplayName": "global users table; no org_id column — user is identified by user_id PK",
    # ck_policy_templates is a global catalogue (not per-org); templates are shared across all orgs.
    "vaktcomply.sql:ListCKPolicyTemplates": "global policy template catalogue; not per-org",
    "vaktcomply.sql:GetCKPolicyTemplateByID": "global policy template catalogue; not per-org",
    # Policy acceptance flow: UPDATE by PK after system sends email (system-internal step).
    "vaktcomply.sql:MarkCKPolicyAcceptanceRequestSent": "system marks email-sent status by PK; called immediately after sending — no user input",
    # NOTE: GetCKPolicyAcceptanceCampaignStats + ListCKPolicyAcceptanceRequests were
    # removed from this allowlist — the "caller verifies campaign org ownership"
    # rationale had NO such caller (R1-16-V1 / R1-24-RT02, CROSS_ORG_LEAK). Both
    # queries now carry an explicit `AND org_id = $2::uuid` predicate.
    # Public accept-portal: employee clicks token link, no org context; request UUID is the auth.
    "vaktcomply.sql:RecordCKPolicyAcceptance": "public token-based accept endpoint; request PK is the auth token",

    # ── vaktprivacy ───────────────────────────────────────────────────────────
    # Background job that globally expires AVVs past their review_date — intentionally all orgs.
    "vaktprivacy.sql:MarkExpiredPPAVVs": "background job: intentionally marks expired AVVs across all orgs",
    # Public DSR portal: visitor looks up org by slug before any auth exists.
    "vaktprivacy.sql:GetOrgByDSRSlug": "public DSR portal: org lookup by unique slug before any auth",
    # SELECT/UPDATE org DSR settings by PK; $1 is org_id passed by authenticated handler.
    "vaktprivacy.sql:GetDSRPortalSettings": "SELECT from organizations WHERE id = $1 (org_id passed as param)",
    "vaktprivacy.sql:UpdateDSRPortalSettings": "UPDATE organizations WHERE id = $1 (org_id passed as param)",

    # ── vaktscan ──────────────────────────────────────────────────────────────
    # Background scanner worker updates its own scan/report rows by PK (UUIDs issued by the system).
    "vaktscan.sql:UpdateSPScanStatus": "background worker updates own scan row by PK (system-issued UUID, no user input)",
    "vaktscan.sql:UpdateSPReport": "background worker updates own report row by PK (system-issued UUID)",
    "vaktscan.sql:StoreSPReportContent": "background worker stores report content by PK (system-issued UUID)",
    # SELECT components by sbom_id — vb_sboms has org_id; handler verifies sbom org before calling.
    "vaktscan.sql:ListSPComponentsBySBOM": "scoped by sbom_id FK into org-scoped vb_sboms; caller verifies sbom org ownership",
    "vaktscan.sql:ListSPComponentsBySBOMFull": "scoped by sbom_id FK into org-scoped vb_sboms; caller verifies sbom org ownership",
    # EOL-check background job: updates individual component rows by PK after global EOL API call.
    "vaktscan.sql:UpdateSPComponentEOL": "background EOL-check job: component UUID from prior org-scoped query",
    "vaktscan.sql:BatchUpdateSPComponentEOL": "background EOL-check job: component UUIDs from prior org-scoped query",
    # vb_eol_cache is a global product/cycle cache — not per-org (no org data stored).
    "vaktscan.sql:GetSPEOLCache": "vb_eol_cache is a global product lifecycle cache; contains no org data",

    # ── vaktvault ─────────────────────────────────────────────────────────────
    # UPDATE access counter by PK — handler verifies org ownership before decrypting/counting.
    "vaktvault.sql:UpdateSVSecretAccess": "access counter UPDATE by PK; handler verifies org ownership before this call",
    # Public share-link endpoint: token_hash is the auth credential — no org context yet.
    "vaktvault.sql:GetSVShareLink": "public share link: token_hash is globally unique auth; handler verifies org after",
    # Used internally to resolve secret → project → org_id for authorization check.
    "vaktvault.sql:GetSVSecretProjectID": "helper to resolve secret PK → project_id for org ownership check (result used for auth)",
}

# Patterns that count as org_id filtering in a query body.
ORG_ID_PATTERNS = [
    re.compile(r'\borg_id\b', re.IGNORECASE),
]

# DML types that require an org_id filter.
NEEDS_FILTER = {"SELECT", "UPDATE", "DELETE"}

# sqlc query name comment: -- name: QueryName :type
QUERY_HEADER = re.compile(r'--\s*name:\s*(\w+)\s*:\s*(\w+)')


def parse_queries(sql_content: str):
    """Yield (query_name, query_type, query_body) triples."""
    lines = sql_content.splitlines()
    current_name = None
    current_type = None
    body_lines = []

    for line in lines:
        m = QUERY_HEADER.match(line.strip())
        if m:
            if current_name:
                yield current_name, current_type, "\n".join(body_lines)
            current_name = m.group(1)
            current_type = m.group(2).lower()  # one, many, exec, execresult
            body_lines = []
        elif current_name:
            body_lines.append(line)

    if current_name:
        yield current_name, current_type, "\n".join(body_lines)


def leading_dml(body: str) -> str | None:
    """Return the first DML keyword in the query body, or None."""
    for token in body.split():
        upper = token.upper().strip("(")
        if upper in ("SELECT", "INSERT", "UPDATE", "DELETE", "WITH"):
            # WITH can wrap a SELECT/UPDATE/DELETE — check further
            if upper == "WITH":
                rest = body[body.upper().index("WITH") + 4:]
                return leading_dml(rest)
            return upper
    return None


def has_org_id(body: str) -> bool:
    return any(p.search(body) for p in ORG_ID_PATTERNS)


# ── G-01: schema-driven tenant-table detection ──────────────────────────────
#
# The previous version of this gate recognised a "multi-tenant table" only by
# a hardcoded (ck_|vb_|so_|sr_|po_|hr_) prefix regex. That is a GATE_BLIND
# bug, not a design choice: ~50 org-scoped tables outside those 6 module
# prefixes (notifications, api_keys, webhooks, sessions, license_keys,
# cloud_integrations, refresh_sessions, org_members, ...) carry a real org_id
# column and were silently exempt from the raw-SQL org_id check below — not
# flagged, not counted as skipped, simply invisible to TENANT_TABLE_RE.search().
#
# Fix: derive the tenant-table set from the schema itself. Every CREATE TABLE
# in db/migrations/*.up.sql that declares an org_id column (directly, or via
# a later ALTER TABLE ... RENAME TO that carries the org_id-ness of a renamed
# table, e.g. migration 122's pg_* -> sr_* rename) is a tenant table — no
# prefix list to keep in sync by hand.
_CREATE_TABLE_RE = re.compile(
    r'CREATE TABLE\s+(?:IF NOT EXISTS\s+)?"?(\w+)"?\s*\(',
    re.IGNORECASE,
)
_ALTER_RENAME_TABLE_RE = re.compile(
    r'ALTER TABLE\s+"?(\w+)"?\s+RENAME TO\s+"?(\w+)"?',
    re.IGNORECASE,
)


def _table_bodies(sql: str):
    """Yield (table_name, balanced-paren body) for every CREATE TABLE statement."""
    for m in _CREATE_TABLE_RE.finditer(sql):
        name = m.group(1)
        start = m.end() - 1  # position of the opening '('
        depth = 0
        i = start
        while i < len(sql):
            if sql[i] == '(':
                depth += 1
            elif sql[i] == ')':
                depth -= 1
                if depth == 0:
                    i += 1
                    break
            i += 1
        yield name, sql[start:i]


def derive_tenant_tables(migrations_dir: Path) -> set[str]:
    """Return every table name that carries an org_id column, per the schema."""
    tables: set[str] = set()
    for f in sorted(migrations_dir.glob("*.up.sql")):
        content = f.read_text()
        for name, body in _table_bodies(content):
            if re.search(r'\borg_id\b', body, re.IGNORECASE):
                tables.add(name)
        for old, new in _ALTER_RENAME_TABLE_RE.findall(content):
            if old in tables:
                tables.discard(old)
                tables.add(new)
    return tables


def build_tenant_table_re(table_names) -> re.Pattern:
    alts = "|".join(re.escape(t) for t in sorted(table_names))
    return re.compile(rf'\b(?:{alts})\b', re.IGNORECASE)


def build_join_tenant_re(table_names) -> re.Pattern:
    """
    Matches: [LEFT|RIGHT|INNER|CROSS]? JOIN <tenant_table> [alias] ON ...
    Captures the ON clause up to the next SQL keyword or end-of-string.
    """
    alts = "|".join(re.escape(t) for t in sorted(table_names))
    return re.compile(
        rf'\bJOIN\s+(?:{alts})\b\s*(?:\w+\s+)?ON\s+([^;]+?)(?=\b(?:LEFT|RIGHT|INNER|CROSS|FULL|JOIN|WHERE|GROUP|ORDER|HAVING|LIMIT|UNION|EXCEPT|INTERSECT|$))',
        re.IGNORECASE | re.DOTALL,
    )


# Inline opt-out comment for raw Go SQL: // orgid-lint: global — <reason>
GO_LINT_SKIP_RE = re.compile(r'orgid-lint:\s*global', re.IGNORECASE)
# Inline opt-out for an unscoped JOIN: // orgid-lint: join-ok — <reason>
GO_LINT_JOIN_OK_RE = re.compile(r'orgid-lint:\s*join-ok', re.IGNORECASE)
# Both kinds in one pass, so a comment can be located and classified once.
GO_LINT_ANY_RE = re.compile(r'orgid-lint:\s*(global|join-ok)', re.IGNORECASE)


# ── R1-SA03-05: die Unterdrückung hängt am STATEMENT, nicht an Zeilennähe ────
#
# Vorher stand hier:
#     preceding = lines[start_line - 4:start_line]
#     skip_global = ... or GO_LINT_SKIP_RE.search(preceding)
# Ein `// orgid-lint: global` exemptierte damit JEDES SQL-Literal, das in den
# folgenden drei Zeilen begann — also auch das org-pflichtige Statement, das
# zufällig neben dem berechtigt globalen stand. Reproduziert: ein Kommentar,
# zwei unterdrückte Queries. Gemessen am Stand 0389470 deckten 3 von 91
# Unterdrückungen mehr als ein Statement (2× cmd/rotate-key/rotate.go, 1× ein
# Test) — heute alle drei zu Recht, der Defekt war also latent. Latent ist auf
# der einzigen Mandanten-Isolationsprüfung nach ADR-0042 trotzdem zu viel.
#
# Neue Bindung, zwei Fälle:
#
#   * TRAILING-Kommentar (links davon steht Code) gilt NUR für Literale auf
#     derselben Zeile — die übliche `//nolint`-Semantik. Muster im Repo:
#     usermgmt/role_update_boundary_test.go, wo jede Tabellenzeile ihr eigenes
#     Opt-out trägt und die Zeile darunter es NICHT erben darf.
#
#   * STANDALONE-Kommentar (allein auf seiner Zeile) gilt für genau EIN
#     Statement: das erste ihm folgende, das überhaupt ein prüfpflichtiges
#     SQL-Literal enthält. Statementgrenzen kommen aus Gos automatischer
#     Semikolon-Einfügung (ASI), ausgewertet auf der Klammertiefe des
#     Kommentars. Die Suche endet spätestens, wenn die Tiefe unter die des
#     Kommentars fällt (der umschließende Block ist verlassen).
#
# Bewusst NICHT enger: ein Standalone-Kommentar innerhalb eines Composite-
# Literals deckt dessen Elemente mit, weil sie EIN Go-Statement sind —
# `rotateColumnByServiceKey(..., columnRotation{SelectSQL: `…`, UpdateSQL: `…`})`
# in cmd/rotate-key/rotate.go ist genau das. Diese Mehrfachdeckung ist damit
# syntaktisch begrenzt statt an einer Zeilenzahl, und sie wird ausgewiesen
# (`multi_stmt_suppressions` in der NENNER-Zeile) statt still zu bleiben.
#
# Zwischenschritte: zwischen Kommentar und Statement dürfen Statements ohne
# prüfpflichtiges SQL-Literal liegen (`var tokenHash string` vor dem QueryRow —
# im Repo mehrfach so geschrieben). Höchstens MAX_SKIPPED_STMTS Stück, damit
# ein verwaister Kommentar nicht beliebig weit nach unten greift und dort eine
# fremde Query exemptiert. Gemessener Bedarf auf der Baseline: genau 4
# (shared/account/account.go:97 — Kommentar, dann Struct-Kopf, zwei Feldzeilen
# und der Kopf des Composite-Literals, bevor die erste Query kommt). Bewusst
# ohne Reserve: greift die Grenze einmal zu früh, meldet das Gate den Kommentar
# NAMENTLICH als stale, und der Fix ist, ihn an sein Statement zu setzen — ein
# lautes Rot mit Fundstelle ist hier besser als stille Reichweite.
MAX_SKIPPED_STMTS = 4

# Ein Zeilenumbruch beendet in Go ein Statement, wenn das letzte Zeichen der
# Zeile ein Bezeichner/Literal/schließende Klammer ist (Spec: "Semicolons").
_ASI_TERMINALS = set(")]}\"'`+-")


def _asi_terminates(ch: str) -> bool:
    return ch.isalnum() or ch == "_" or ch in _ASI_TERMINALS


class GoSource:
    """
    Ein Durchlauf über eine .go-Datei: Backtick-Literale, Klammertiefe und
    Statementgrenzen. Bewusst kein Go-Parser — nur so viel Zustandsautomat, wie
    für die Frage „gehören diese beiden Literale zum selben Statement?" nötig
    ist. Was er nicht lesen kann, zählt er (siehe `unparsed`), statt es still
    zu überspringen.
    """

    def __init__(self, text: str):
        self.text = text
        self.line_starts = [0] + [m.end() for m in re.finditer(r'\n', text)]
        self.raw_spans: list[tuple[int, int]] = []   # (offset of `, offset after closing `)
        self._ev_off: list[int] = []                 # Klammer-Ereignisse
        self._ev_depth: list[int] = []
        self.breaks: list[tuple[int, int]] = []      # (offset des \n, Tiefe dort)
        self.block_opens: list[tuple[int, int]] = []  # (offset des {, Tiefe davor)
        self._scan()
        # Ein `{`, nach dem auf der Zeile nur noch Weißraum/Kommentar steht,
        # öffnet einen Block (`if … {`, `func … {`, `for … {`) — dort ist der
        # Statement-KOPF zu Ende. Ein `{` mit Inhalt dahinter gehört zu einem
        # einzeiligen Composite-Literal und trennt nichts.
        eol = re.compile(r'^[ \t]*(//.*)?$')
        self.block_opens = [
            (off, d) for off, d in self.block_opens
            if eol.match(self.text[off + 1:self._line_end(off)])
        ]

    def _scan(self) -> None:
        text = self.text
        n = len(text)
        i = 0
        depth = 0
        last_code = None   # letztes Code-Zeichen der laufenden Zeile

        def newline(off: int) -> None:
            nonlocal last_code
            if last_code is not None and _asi_terminates(last_code):
                self.breaks.append((off, depth))
            last_code = None

        while i < n:
            c = text[i]
            if c == '/' and i + 1 < n and text[i + 1] == '/':
                i += 2
                while i < n and text[i] != '\n':
                    i += 1
                continue                      # ASI wertet den Zeilenumbruch aus
            if c == '/' and i + 1 < n and text[i + 1] == '*':
                i += 2
                while i + 1 < n and not (text[i] == '*' and text[i + 1] == '/'):
                    if text[i] == '\n':
                        newline(i)
                    i += 1
                i += 2
                continue
            if c == '`':
                start = i
                i += 1
                while i < n and text[i] != '`':
                    i += 1
                i += 1                        # schließenden Backtick überspringen
                self.raw_spans.append((start, i))
                last_code = '`'
                continue
            if c in '"\'':
                quote = c
                i += 1
                while i < n and text[i] != quote:
                    if text[i] == '\\':
                        i += 1
                    if text[i] == '\n':       # unterminiert — Zeile beenden
                        break
                    i += 1
                i += 1
                last_code = quote
                continue
            if c in '([{':
                if c == '{':
                    self.block_opens.append((i, depth))
                depth += 1
                self._ev_off.append(i)
                self._ev_depth.append(depth)
                last_code = c
                i += 1
                continue
            if c in ')]}':
                depth -= 1
                self._ev_off.append(i)
                self._ev_depth.append(depth)
                last_code = c
                i += 1
                continue
            if c == '\n':
                newline(i)
                i += 1
                continue
            if not c.isspace():
                last_code = c
            i += 1

    def line_of(self, off: int) -> int:
        from bisect import bisect_right
        return bisect_right(self.line_starts, off)

    def _line_end(self, off: int) -> int:
        nl = self.text.find('\n', off)
        return len(self.text) if nl < 0 else nl

    def line_text(self, lineno: int) -> str:
        from bisect import bisect_right  # noqa: F401  (Symmetrie zu line_of)
        start = self.line_starts[lineno - 1]
        end = self.line_starts[lineno] if lineno < len(self.line_starts) else len(self.text)
        return self.text[start:end].rstrip('\n')

    def depth_at(self, off: int) -> int:
        from bisect import bisect_left
        idx = bisect_left(self._ev_off, off)
        return self._ev_depth[idx - 1] if idx > 0 else 0

    def block_end(self, off: int, d0: int) -> int:
        """Erster Offset nach `off`, an dem die Tiefe unter `d0` fällt."""
        from bisect import bisect_right
        idx = bisect_right(self._ev_off, off)
        while idx < len(self._ev_off):
            if self._ev_depth[idx] < d0:
                return self._ev_off[idx]
            idx += 1
        return len(self.text)

    def in_raw_string(self, off: int) -> bool:
        return any(s < off < e for s, e in self.raw_spans)


def find_suppressions(src: GoSource):
    """
    Alle `orgid-lint:`-Opt-outs einer Datei.

    Rückgabe: (Liste von Dicts, Zahl der nicht zuordenbaren Treffer).
    Ein Treffer, der nicht in einem `//`-Kommentar steht (Block-Kommentar,
    String, generierter Text), wird gezählt statt still verworfen.
    """
    found = []
    unparsed = 0
    for m in GO_LINT_ANY_RE.finditer(src.text):
        off = m.start()
        if src.in_raw_string(off):
            continue                       # Fließtext in einem SQL-/Doku-Literal
        lineno = src.line_of(off)
        line_start = src.line_starts[lineno - 1]
        marker = src.text.rfind('//', line_start, off)
        if marker < 0:
            unparsed += 1
            continue
        found.append({
            "kind": m.group(1).lower(),
            "offset": marker,
            "line": lineno,
            "standalone": src.text[line_start:marker].strip() == "",
            "text": src.line_text(lineno).strip(),
            "used": False,
            "covers": 0,
        })
    return found, unparsed


def governed_literals(src: GoSource, comment, sql_spans: list[tuple[int, int]]) -> list[int]:
    """
    Die SQL-Literale, für die dieses Opt-out gilt — an das Statement gebunden.

    `sql_spans` sind die (start, end) aller Backtick-Literale der Datei, die
    überhaupt nach SQL aussehen (leading_dml != None), aufsteigend sortiert.
    Die Zuordnung läuft bewusst über ALLE SQL-Literale, nicht nur die
    prüfpflichtigen: sonst spränge ein Kommentar über die Query, für die er
    geschrieben wurde (etwa auf eine Katalogtabelle ohne org_id), hinweg zur
    nächsten org-pflichtigen Query — und exemptierte ausgerechnet die.
    Der Aufrufer schneidet das Ergebnis mit seiner Prüfmenge.
    """
    if not comment["standalone"]:
        # Trailing-Kommentar (`//nolint`-Semantik): nur das Literal, das auf
        # seiner Zeile steht — ein mehrzeiliges Literal zählt mit, solange die
        # Kommentarzeile in seiner Spanne liegt. Die Zeile DARUNTER erbt nichts;
        # genau das war R1-SA03-05 (role_update_boundary_test.go:157 → :158).
        line = comment["line"]
        return [s for s, e in sql_spans
                if src.line_of(s) <= line <= src.line_of(max(s, e - 1))]

    off = comment["offset"]
    d0 = src.depth_at(off)
    stop = src.block_end(off, d0)

    # Statementgrenzen aus Gos ASI, auf der Tiefe des Kommentars UND eine
    # Ebene tiefer. Das `+ 1` ist nicht kosmetisch: steht der Kommentar vor
    # einer Deklaration oder einem `if`, öffnet die nächste Zeile einen Block,
    # und ohne die tiefere Ebene liefe die Region bis zum Blockende — ein
    # Kommentar über einer Funktion deckte dann deren GANZEN Rumpf
    # (vaktaware/repository.go:1105 deckte so das Statement in Zeile 1143 mit).
    # Innerhalb eines Composite-Literals gibt es diese Grenzen nicht, weil die
    # Zeilen dort auf `,` enden und Go dort kein Semikolon einfügt — genau
    # deshalb bleiben dessen Elemente EIN Statement.
    bounds = [b for b, d in src.breaks if off < b < stop and d <= d0 + 1]
    # Ein Block-`{` auf der Tiefe des Kommentars beendet den Statement-KOPF:
    # ohne diese Grenze deckte ein Kommentar vor `if _, err := db.Exec(ctx,
    # `…`); err != nil {` auch noch das erste Statement IM Rumpf (live gegen
    # eine Kopie des Baums nachgestellt, cmd/rotate-key/rotate.go:113).
    bounds += [b for b, d in src.block_opens if off < b < stop and d <= d0]
    bounds = sorted(set(bounds))
    bounds.append(stop)

    lower = off
    for skipped, upper in enumerate(bounds):
        hits = [s for s, _e in sql_spans if lower < s < upper]
        if hits:
            return hits          # erstes Statement mit SQL — und nur dieses
        if skipped >= MAX_SKIPPED_STMTS:
            return []
        lower = upper
    return []


def unscoped_joins(sql: str, join_re: re.Pattern) -> list[str]:
    """
    Return a list of JOIN ON clauses that reference a tenant-prefixed table
    and are dangerous: they do not scope org_id AND they join on a non-UUID
    business key (i.e. the ON clause does not use a `.id` UUID PK field).

    This catches the S78-2 pattern:
        LEFT JOIN ck_controls c ON c.control_id = cr.anforderung_id
    where control_id is a shared business key (e.g. "BSI-ORP.1.A1") that
    exists in every org's ck_controls table — causing cross-org row
    multiplication even though the WHERE clause scopes the primary table.

    FK joins on UUID PKs (e.g. `JOIN so_envs e ON e.id = sk.env_id`) are
    safe because UUID PKs are globally unique across orgs — no annotation needed.
    Use `// orgid-lint: join-ok — <reason>` to suppress a remaining flag.
    """
    bad = []
    for m in join_re.finditer(sql):
        on_clause = m.group(1)
        if re.search(r'\borg_id\b', on_clause, re.IGNORECASE):
            continue  # explicitly org-scoped in ON clause — safe
        # UUID PK joins (.id field) are globally unique — no cross-org leak.
        if re.search(r'\.\bid\b', on_clause, re.IGNORECASE):
            continue  # FK join on UUID PK — safe
        bad.append(m.group(0).strip().splitlines()[0][:120])
    return bad


def scan_go_raw_sql(go_dir: Path, tenant_re: re.Pattern, join_re: re.Pattern):
    """
    Scan backtick SQL strings in .go files for missing org_id filters.

    Two checks are performed:
    1. The query body must contain org_id somewhere (existing check).
    2. Every JOIN on a tenant table must have org_id in its ON clause (catches
       the S78-2 cross-org JOIN pattern even when the WHERE clause does scope
       to org_id).

    tenant_re/join_re are schema-derived (see derive_tenant_tables, G-01) —
    every table with an org_id column, not just the 6 module prefixes.

    Opt-outs (`// orgid-lint: global|join-ok`) sind an das Statement gebunden,
    nicht an eine Zeilendistanz — siehe governed_literals (R1-SA03-05). Ein
    Opt-out, das kein prüfpflichtiges Statement mehr trifft, ist STALE und wird
    als Verletzung gemeldet: eine Ausnahme ohne Gegenstand ist eine offene Tür,
    die niemand mehr sieht (K2-08).

    Returns (violations, checked, suppressed, stats).
    """
    violations = []
    suppressed = 0
    total = 0
    stats = {
        "comments": 0,          # gefundene orgid-lint-Kommentare
        "stale": [],            # Kommentare, die auf gar kein SQL-Statement zeigen
        "redundant": 0,         # zeigen auf SQL, das ohnehin nicht prüfpflichtig ist
        "multi": [],            # Kommentare, die mehr als ein Literal decken
        "unparsed": 0,          # orgid-lint-Treffer außerhalb eines //-Kommentars
        "unreadable": [],       # .go-Dateien, die nicht gelesen werden konnten
    }

    def rel(p: Path) -> str:
        return str(p.relative_to(go_dir.parent.parent
                                 if go_dir.name != "backend" else go_dir.parent))

    for go_file in sorted(go_dir.rglob("*.go")):
        # Skip generated sqlc files — they are covered by the SQL scanner.
        if go_file.name.endswith(".sql.go") or "internal/db/" in str(go_file):
            continue

        try:
            text = go_file.read_text(errors="replace")
        except OSError as exc:
            stats["unreadable"].append(f"{rel(go_file)} ({exc.strerror})")
            continue

        src = GoSource(text)

        # Alle SQL-artigen Backtick-Literale — Zuordnungsziel für die Opt-outs.
        sql_spans = []
        eligible = {}                      # offset -> (start_line, snippet, dml)
        for start, end in src.raw_spans:
            snippet = text[start + 1:end - 1]
            dml = leading_dml(snippet)
            if dml is None:
                continue                   # kein SQL: Regex, JSON, Template …
            sql_spans.append((start, end))
            # Prüfpflichtig: Tenant-Tabelle + SELECT/UPDATE/DELETE.
            if dml in NEEDS_FILTER and tenant_re.search(snippet):
                eligible[start] = (src.line_of(start), snippet, dml)

        comments, unparsed = find_suppressions(src)
        stats["comments"] += len(comments)
        stats["unparsed"] += unparsed

        skip_global = set()
        skip_join = set()
        for c in comments:
            covered = governed_literals(src, c, sql_spans)
            hit = [o for o in covered if o in eligible]
            c["used"] = bool(covered)
            c["covers"] = len(hit)
            target = skip_global if c["kind"] == "global" else skip_join
            target.update(hit)
            if not covered:
                # Kein einziges SQL-Statement in Reichweite: die Query, für die
                # dieses Opt-out geschrieben wurde, gibt es nicht mehr.
                stats["stale"].append(f"{rel(go_file)}:{c['line']}  {c['text'][:110]}")
            elif not hit:
                # Zeigt auf SQL, das ohnehin nicht prüfpflichtig ist (Katalog-
                # tabelle ohne org_id, INSERT). Überflüssig, aber nicht blind —
                # deshalb gezählt, nicht rot.
                stats["redundant"] += 1
            elif len(hit) > 1:
                stats["multi"].append(
                    f"{rel(go_file)}:{c['line']} deckt {len(hit)} Literale "
                    f"(Zeilen {', '.join(str(eligible[o][0]) for o in hit)}) — "
                    f"ein Go-Statement")

        for start, (start_line, snippet, dml) in sorted(eligible.items()):
            if start in skip_global:
                # A suppressed statement was NOT checked. Counting it in `total`
                # and reporting skipped=0 is the class this gate exists to kill
                # (a gate that reports OK for work it did not do), so it moves
                # from the checked column into the skipped one.
                suppressed += 1
                continue

            total += 1
            short = snippet.strip().splitlines()[0][:120]

            # Check 1: query body must contain org_id.
            if not has_org_id(snippet):
                violations.append((rel(go_file), start_line, short, dml,
                                   "missing org_id in query body"))
                continue  # don't double-report

            # Check 2: every JOIN on a tenant table must scope org_id in ON clause.
            if start not in skip_join:
                for join_clause in unscoped_joins(snippet, join_re):
                    violations.append((rel(go_file), start_line, join_clause, dml,
                                       "JOIN on tenant table without org_id in ON clause (S78-2 pattern)"))

    return violations, total, suppressed, stats


def main():
    parser = argparse.ArgumentParser(
        description="Check SQL queries for missing org_id filters (multi-tenancy guard)."
    )
    parser.add_argument("--query-dir", default="backend/db/queries",
                        help="Directory containing sqlc *.sql files")
    parser.add_argument("--raw-sql", action="store_true",
                        help="Also scan backtick SQL literals in Go source files")
    parser.add_argument("--go-dir", default="backend",
                        help="Root directory to scan for Go files (used with --raw-sql)")
    parser.add_argument("--migrations-dir", default="backend/db/migrations",
                        help="Directory containing golang-migrate *.up.sql files "
                             "(source of truth for the schema-derived tenant-table set, G-01)")
    args = parser.parse_args()

    query_dir = Path(args.query_dir)
    if not query_dir.exists():
        print(f"ERROR: query dir {query_dir} not found", file=sys.stderr)
        sys.exit(1)

    # G-01: schema-derived tenant-table set, used by Pass 2 (raw Go SQL) below.
    # A gate whose own detection input is empty is not "0 violations", it is
    # blind — same SA-03 rule as every other gate in this repo.
    migrations_dir = Path(args.migrations_dir)
    tenant_tables = set()
    if args.raw_sql:
        if not migrations_dir.exists():
            print(f"ERROR: migrations dir {migrations_dir} not found", file=sys.stderr)
            sys.exit(1)
        tenant_tables = derive_tenant_tables(migrations_dir)
        if not tenant_tables:
            print("org_id query lint: VAKUAER — 0 org_id-bearing tables derived from "
                  f"{migrations_dir} (schema parser broken, or migrations dir empty)")
            sys.exit(2)

    violations = []
    total = 0
    seen_keys = set()
    allowlist_used = set()

    # ── Pass 1: sqlc query files ─────────────────────────────────────────────
    for sql_file in sorted(query_dir.glob("*.sql")):
        content = sql_file.read_text()
        for qname, _qtype, body in parse_queries(content):
            total += 1
            key = f"{sql_file.name}:{qname}"
            seen_keys.add(key)
            dml = leading_dml(body)
            if dml not in NEEDS_FILTER:
                continue

            if key in ALLOWLIST:
                allowlist_used.add(key)
                continue

            if not has_org_id(body):
                violations.append((sql_file.name, qname, dml, None))

    # Stale-Prüfung Pass 1 (R1-SA03-05): ein ALLOWLIST-Eintrag, dessen Query es
    # in --query-dir nicht (mehr) gibt, ist eine vorab erteilte Ausnahme ohne
    # Gegenstand. Sie prüft nichts, sie steht nur bereit, falls jemand später
    # denselben Namen wieder anlegt — und dann greift sie ungeprüft. Dieselbe
    # Klasse wie ein verwaister // orgid-lint-Kommentar, deshalb hier genauso rot.
    #
    # Geprüft wird nur gegen Dateien, die es in --query-dir tatsächlich gibt:
    # zeigt der Lauf auf einen fremden Baum (Test-Fixture, Teil-Checkout), ist
    # ein Eintrag nicht stale, sondern UNPRÜFBAR. Der Unterschied wird gezählt
    # und ausgewiesen — ein Gate, das bei gesundem Repo rot wird, wird
    # abgeschaltet statt gefixt.
    seen_files = {p.name for p in query_dir.glob("*.sql")}
    stale_allowlist = sorted(k for k in ALLOWLIST
                             if k not in seen_keys and k.split(":", 1)[0] in seen_files)
    unverifiable_allowlist = sorted(k for k in ALLOWLIST
                                    if k.split(":", 1)[0] not in seen_files)

    if violations:
        print(f"\norg_id query lint: {len(violations)} violation(s) found\n")
        print("  These queries filter multi-tenant tables without an org_id check.")
        print("  Either add `org_id = $N` to the WHERE clause, or add to ALLOWLIST")
        print("  in scripts/lint-orgid-queries.py with a justification comment.\n")
        for item in violations:
            fname, qname, dml, _ = item
            print(f"  FAIL  {fname}:{qname}  ({dml})")
        print()
        sys.exit(1)

    # G-07: zero queries parsed is not a passing result — it means --query-dir
    # pointed nowhere useful or the *.sql glob stopped matching, not that every
    # query is safe. A gate that reports "OK (0 queries checked)" is a silent
    # no-op wearing a green checkmark.
    if total == 0:
        print(f"FAIL — parsed ZERO queries from {query_dir} (non-vacuity guard, G-07). "
              "Check --query-dir and the *.sql glob.", file=sys.stderr)
        sys.exit(2)

    print(f"NENNER (sqlc): queries={total} | allowlist={len(ALLOWLIST)} "
          f"(greifend={len(allowlist_used)}, "
          f"gegenstandslos={len(ALLOWLIST) - len(allowlist_used) - len(stale_allowlist) - len(unverifiable_allowlist)}, "
          f"unpruefbar={len(unverifiable_allowlist)}) "
          f"| allowlist_stale={len(stale_allowlist)}")
    if stale_allowlist:
        print(f"\norg_id query lint: {len(stale_allowlist)} STALE ALLOWLIST-Eintrag/Einträge\n")
        print("  Diese Einträge nennen eine Query, die es in "
              f"{query_dir} nicht gibt. Eine Ausnahme ohne Gegenstand prüft nichts")
        print("  und greift ungeprüft, sobald jemand denselben Namen wieder anlegt.")
        print("  Fix: Eintrag aus ALLOWLIST in scripts/lint-orgid-queries.py entfernen.\n")
        for key in stale_allowlist:
            print(f"  STALE  {key}")
        print()
        sys.exit(1)

    # ── Pass 2: raw Go backtick SQL (opt-in via --raw-sql) ───────────────────
    if args.raw_sql:
        go_dir = Path(args.go_dir)
        if not go_dir.exists():
            print(f"ERROR: go dir {go_dir} not found", file=sys.stderr)
            sys.exit(1)
        tenant_re = build_tenant_table_re(tenant_tables)
        join_re = build_join_tenant_re(tenant_tables)
        raw_violations, raw_total, raw_skipped, stats = scan_go_raw_sql(go_dir, tenant_re, join_re)
        total += raw_total
        print(f"NENNER: tenant_tables={len(tenant_tables)} (schema-derived from "
              f"{migrations_dir}, org_id column) | raw_sql_checked={raw_total} | skipped={raw_skipped}"
              f" | suppressions={raw_skipped} aus {stats['comments']} Kommentar(en)"
              f" | multi_stmt_suppressions={len(stats['multi'])}"
              f" | redundant={stats['redundant']} | stale={len(stats['stale'])}"
              f" | unparsed={stats['unparsed']} | unreadable={len(stats['unreadable'])}")
        for line in stats["multi"]:
            # Mehrfachdeckung ist erlaubt (ein Go-Statement), aber nie still:
            # genau ihre Unsichtbarkeit war der Defekt R1-SA03-05.
            print(f"  MULTI  {line}")
        for line in stats["unreadable"]:
            print(f"  UNREADABLE  {line}")
        if stats["unparsed"]:
            print(f"  HINWEIS: {stats['unparsed']} orgid-lint-Treffer stehen nicht in einem "
                  "//-Kommentar und wurden keinem Statement zugeordnet.")
        if stats["stale"]:
            print(f"\norg_id query lint (raw SQL): {len(stats['stale'])} STALE Opt-out(s)\n")
            print("  Diese `// orgid-lint:`-Kommentare treffen kein prüfpflichtiges SQL-Statement")
            print("  mehr — die Query, für die sie einmal geschrieben wurden, ist weg oder")
            print("  umgebaut. Eine Ausnahme ohne Gegenstand ist eine offene Tür, die niemand")
            print("  mehr sieht: der nächste, der dort eine Query einfügt, ist ungeprüft.")
            print("  Fix: Kommentar entfernen oder direkt an sein Statement setzen.\n")
            for line in stats["stale"]:
                print(f"  STALE  {line}")
            print()
            sys.exit(1)
        if raw_violations:
            print(f"\norg_id query lint (raw SQL): {len(raw_violations)} violation(s) found\n")
            print("  Backtick SQL in .go files references multi-tenant tables without org_id.")
            print("  Fix: add org_id filter, or annotate with:")
            print("    // orgid-lint: global — <reason>  (whole query is intentionally unscoped)")
            print("    // orgid-lint: join-ok — <reason>  (JOIN is safely scoped via other means)\n")
            for fpath, lineno, snippet, dml, detail in raw_violations:
                print(f"  FAIL  {fpath}:{lineno}  ({dml})  [{detail}]  {snippet!r:.80}")
            print()
            sys.exit(1)
        # G-07: --raw-sql is opt-in specifically to scan backend/**/*.go — zero
        # tenant-table hits there means the walk found no .go files at all
        # (wrong --go-dir, empty checkout), not that the whole backend is clean.
        if raw_total == 0:
            print(f"FAIL — --raw-sql scanned {go_dir} and found ZERO tenant-table SQL "
                  "literals (non-vacuity guard, G-07). Check --go-dir.", file=sys.stderr)
            sys.exit(2)
        print(f"org_id query lint: OK ({total} queries checked, 0 violations; raw-SQL pass included)")
    else:
        print(f"org_id query lint: OK ({total} queries checked, 0 violations)")


if __name__ == "__main__":
    main()
