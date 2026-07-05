# Security Policy

## Supported Versions

| Version | Supported |
|---------|-----------|
| Latest  | ✅        |
| < 6 months old | ✅ |
| Older   | ❌        |

## Reporting a Vulnerability

**Please do not report security vulnerabilities through public GitHub issues.**

Report security vulnerabilities to: **security@norvikops.de**

Include:
- Description of the vulnerability
- Steps to reproduce
- Potential impact
- (Optional) Suggested fix

We will acknowledge receipt within 48 hours and provide an initial assessment within 5 business days.

## Scope

### In scope

The following vulnerability classes are in scope for responsible disclosure:

- Authentication bypass (login, session management, token validation)
- Data leakage (cross-organisation data access, information disclosure)
- Remote Code Execution (RCE) — in any component
- CSRF — on state-mutating endpoints
- SQL injection or other injection vulnerabilities
- Privilege escalation (horizontal or vertical)
- Cryptographic weaknesses in secrets storage or token handling
- Server-Side Request Forgery (SSRF) bypasses

### Out of scope

The following are **not** in scope:

- Physical attacks against self-hosted infrastructure
- Denial of Service (DoS/DDoS) attacks
- Social engineering of Norvik staff or customers
- Vulnerabilities requiring physical access to the server
- Issues in third-party dependencies where a fix is not yet available (report upstream)
- Rate limiting on non-sensitive endpoints
- Self-XSS (requires the attacker to already have account access)
- Findings from automated scanners without manual validation and demonstrated impact

## Response SLA

| Severity | Acknowledge | Fix Target |
|----------|-------------|------------|
| Critical (auth bypass, RCE, cross-org data) | 48 hours | 7 days |
| High (data leak, privilege escalation) | 48 hours | 30 days |
| Medium | 48 hours | 90 days |
| Low / Informational | 5 business days | Best effort |

**No bug bounty program is offered.** We credit researchers in release notes unless they prefer to remain anonymous.

## Disclosure Policy

We follow coordinated disclosure. We ask that you:

1. Give us time to fix the issue before public disclosure
2. Avoid accessing or modifying data that does not belong to you
3. Not perform DoS attacks or automated scanning at scale against public infrastructure

We aim to issue a fix and a public advisory (GitHub Security Advisory) within the SLA above.

### Actively exploited vulnerabilities (EU Cyber Resilience Act)

If a vulnerability in Vakt is **actively exploited in the wild**, an additional
regulatory reporting duty applies from **11.09.2026**: as the product's
manufacturer, NorvikOps must report to ENISA / the national CSIRT (early warning
within 24 h, notification within 72 h, final report within 14 days) and inform
affected users. A merely *reported* (not yet exploited) vulnerability follows the
coordinated-disclosure flow above and does not start that clock. The operational
procedure lives in the internal incident-response runbook (CRA Art. 14 section).

## Security Architecture

Vakt is a self-hosted platform. Key security properties:

- All data remains on your infrastructure — no telemetry, no usage tracking. Complete list of optional outbound connections:

| Destination | Purpose | Opt-in variable | Data sent |
|---|---|---|---|
| `api.norvikops.de` | License auto-renewal | `VAKT_LICENSE_TOKEN=<token>` | License token only |
| `www.bsi.bund.de` | BSI CERT-Bund advisory RSS feed | default-on; disable with `VAKT_BSI_FEED_ENABLED=false` | None (GET request) |
| `api.first.org` | EPSS vulnerability scores | `VAKT_EPSS_ENABLED=true` | CVE IDs |
| `api.github.com` | Update availability check | `VAKT_UPDATE_CHECK=true` | None (GET request) |
| Operator-configured LLM provider | AI features | `VAKT_AI_PROVIDER=openai` + `VAKT_AI_BASE_URL` | Compliance document excerpts — **data leaves your instance!** Ensure a DSGVO-compliant AVV with the provider. |
- **Paseto v4 tokens** (not JWT — no algorithm confusion attacks); PASETO signing key domain-separated from AES-256-GCM encryption keys via HKDF-SHA256
- **AES-256-GCM** encryption for stored secrets; HKDF key derivation per project and per service
- **bcrypt cost 12** for password hashing
- **httpOnly cookies** for session tokens — not localStorage
- SSRF protection on all outgoing URLs (AI base URL, webhook URLs)
- Redis-backed rate limiting and brute-force lockout: account lockout after **5 failed logins** (15 min), IP-level lockout after **10 failed logins** from one IP (15 min)
- Structured audit log (zerolog) for all mutations
- CSP: no `unsafe-inline` in `script-src`
- Password minimum: 10 characters

## Security Assessment

The current security posture is documented in [`docs/SECURITY-ASSESSMENT.md`](docs/SECURITY-ASSESSMENT.md).

## Security Documentation

| Document | Description |
|----------|-------------|
| [`docs/security/tom.md`](docs/security/tom.md) | Technical and Organizational Measures (Art. 32 GDPR) |
| [`docs/security/vvt.md`](docs/security/vvt.md) | Records of Processing Activities template (Art. 30 GDPR) |
| [`docs/security/subprocessors.md`](docs/security/subprocessors.md) | Third-party components used in self-hosted deployments |
| [`docs/security/pentest-intern.md`](docs/security/pentest-intern.md) | Internal self-pentest checklist (OWASP Top 10 + Vakt-specific) |
| [`docs/security/pentest-rfp.md`](docs/security/pentest-rfp.md) | External pentest RFP — planned Q3 2026 |
| [`docs/security/scim-verification.md`](docs/security/scim-verification.md) | SCIM 2.0 verification checklist (Users, Groups, Okta) |
| [`docs/security/pentest-scope.md`](docs/security/pentest-scope.md) | External pentest scope definition for security testing firms |
| [`docs/security/responsible-disclosure.md`](docs/security/responsible-disclosure.md) | Full responsible disclosure policy with timelines and scope |
