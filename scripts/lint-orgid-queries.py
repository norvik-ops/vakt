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
    # User lookup by token/email during auth — users are not org-scoped in the
    # users table; membership is stored in org_members.
    "user_permissions.sql:GetUserByEmail": "global users table, no org_id column",
    "user_permissions.sql:GetUserByID": "global users table, no org_id column",
    "user_permissions.sql:GetUserByExternalID": "global users table, no org_id column",
    "user_permissions.sql:CreateUser": "INSERT sets org via org_members, not users",
    "user_permissions.sql:UpdateUser": "global users update (password/profile)",
    "user_permissions.sql:UpdateUserPassword": "global user password update",
    "user_permissions.sql:ListUsers": "admin-only global listing with RBAC guard in handler",
    "user_permissions.sql:GetOrgBySlug": "org lookup by slug is always global",
    "user_permissions.sql:GetOrgByID": "org lookup by id is always global",
    "user_permissions.sql:CreateOrg": "INSERT creates the org — no pre-existing org_id",
    "user_permissions.sql:UpdateOrg": "org update by primary key only",
    "user_permissions.sql:ListOrgs": "super-admin only: lists all orgs",
    "user_permissions.sql:GetOrgMember": "lookup by composite key (org_id passed as param)",
    "user_permissions.sql:CreateOrgMember": "INSERT sets org_id",
    "user_permissions.sql:DeleteOrgMember": "composite key includes org_id as param",
    "user_permissions.sql:ListOrgMembers": "org_id passed as param $1",
    "user_permissions.sql:GetRoleByName": "roles are global (not per-org)",
    "user_permissions.sql:GetRoleByID": "roles are global (not per-org)",
    "user_permissions.sql:ListRoles": "roles are global (not per-org)",
    "user_permissions.sql:CreateRole": "roles are global INSERT",

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
    # SELECT/list policy acceptance requests by campaign_id; caller holds org-verified campaign.
    "vaktcomply.sql:GetCKPolicyAcceptanceCampaignStats": "scoped by campaign_id FK; caller verifies campaign org ownership",
    "vaktcomply.sql:ListCKPolicyAcceptanceRequests": "scoped by campaign_id FK; caller verifies campaign org ownership",
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

    Returns list of (file, approx_line, snippet, dml, detail) violation tuples.
    """
    violations = []
    skipped = 0
    total = 0

    for go_file in sorted(go_dir.rglob("*.go")):
        # Skip generated sqlc files — they are covered by the SQL scanner.
        if go_file.name.endswith(".sql.go") or "internal/db/" in str(go_file):
            continue

        text = go_file.read_text(errors="replace")
        lines = text.splitlines()

        # Extract backtick string spans with their starting line number.
        i = 0
        while i < len(text):
            if text[i] != '`':
                i += 1
                continue
            start = i
            start_line = text[:start].count('\n') + 1
            i += 1
            while i < len(text) and text[i] != '`':
                i += 1
            snippet = text[start + 1:i]
            i += 1  # skip closing backtick

            # Only care about strings that look like SQL touching tenant tables.
            if not tenant_re.search(snippet):
                continue
            dml = leading_dml(snippet)
            if dml not in NEEDS_FILTER:
                continue

            total += 1

            # Check for inline skip comment on the line that opens the backtick.
            context_line = lines[start_line - 1] if start_line <= len(lines) else ""
            # Also check a few lines before the backtick for a preceding comment.
            preceding = "\n".join(lines[max(0, start_line - 4):start_line])
            skip_global = GO_LINT_SKIP_RE.search(context_line) or GO_LINT_SKIP_RE.search(preceding)
            skip_join = GO_LINT_JOIN_OK_RE.search(context_line) or GO_LINT_JOIN_OK_RE.search(preceding)

            if skip_global:
                # A suppressed statement was NOT checked. Counting it in `total`
                # and reporting skipped=0 is the class this gate exists to kill
                # (a gate that reports OK for work it did not do), so it moves
                # from the checked column into the skipped one.
                total -= 1
                skipped += 1
                continue

            short = snippet.strip().splitlines()[0][:120]

            # Check 1: query body must contain org_id.
            if not has_org_id(snippet):
                violations.append((str(go_file.relative_to(go_dir.parent.parent
                                       if go_dir.name != "backend" else go_dir.parent)),
                                    start_line, short, dml, "missing org_id in query body"))
                continue  # don't double-report

            # Check 2: every JOIN on a tenant table must scope org_id in ON clause.
            if not skip_join:
                bad_joins = unscoped_joins(snippet, join_re)
                for join_clause in bad_joins:
                    violations.append((str(go_file.relative_to(go_dir.parent.parent
                                           if go_dir.name != "backend" else go_dir.parent)),
                                        start_line, join_clause, dml,
                                        "JOIN on tenant table without org_id in ON clause (S78-2 pattern)"))

    return violations, total, skipped


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

    # ── Pass 1: sqlc query files ─────────────────────────────────────────────
    for sql_file in sorted(query_dir.glob("*.sql")):
        content = sql_file.read_text()
        for qname, _qtype, body in parse_queries(content):
            total += 1
            dml = leading_dml(body)
            if dml not in NEEDS_FILTER:
                continue

            key = f"{sql_file.name}:{qname}"
            if key in ALLOWLIST:
                continue

            if not has_org_id(body):
                violations.append((sql_file.name, qname, dml, None))

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

    # ── Pass 2: raw Go backtick SQL (opt-in via --raw-sql) ───────────────────
    if args.raw_sql:
        go_dir = Path(args.go_dir)
        if not go_dir.exists():
            print(f"ERROR: go dir {go_dir} not found", file=sys.stderr)
            sys.exit(1)
        tenant_re = build_tenant_table_re(tenant_tables)
        join_re = build_join_tenant_re(tenant_tables)
        raw_violations, raw_total, raw_skipped = scan_go_raw_sql(go_dir, tenant_re, join_re)
        total += raw_total
        print(f"NENNER: tenant_tables={len(tenant_tables)} (schema-derived from "
              f"{migrations_dir}, org_id column) | raw_sql_checked={raw_total} | skipped={raw_skipped}")
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
