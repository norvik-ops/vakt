# ADR-0022: Auth-Tier-Cut — SAML CE, SCIM/SIEM/IP-Allowlist Pro

**Status:** Akzeptiert
**Datum:** 2026-05-22
**Autoren:** Matharnica / KI-Assist

## Kontext

Vakt wird primär von KMUs (20–200 Mitarbeitende) in der DACH-Region eingesetzt,
die NIS2 oder ISO 27001 Compliance anstreben. Diese Zielgruppe betreibt in
der überwältigenden Mehrheit Microsoft 365 / Azure Entra ID oder Google Workspace
— beides enthält SAML 2.0 im Basis-Tarif.

Zugang per Mitarbeiter-Verzeichnis, zentrales MFA-Enforcement und sauberes
Offboarding sind NIS2-Anforderungen (Art. 21 Abs. 2 i), keine Enterprise-
Exklusivfeatures. Ein Pro-Gate auf SAML wäre eine Marketing-Limitierung, die
dem Kern-Nutzenversprechen widerspricht.

SCIM und SIEM setzen hingegen eine IT-Infrastruktur voraus (IdP-Connector,
SOC/SIEM-Plattform), die erst ab ~100+ MA Sinn ergibt und typischerweise
Enterprise-Kontext impliziert.

## Entscheidung

### Tier-Zuordnung

| Feature | Tier | Begründung |
|---------|------|------------|
| SAML 2.0 SP (AzureAD, Okta, Google, OneLogin) | **CE** | KMU-Hygiene, M365/Google-Standard |
| OIDC / Social Login (via Casdoor) | CE | Gleiche Logik wie SAML |
| SCIM 2.0 User+Group Auto-Provisioning | **Pro** | Enterprise IdP-Connector, ab ~100 MA |
| SIEM Audit-Forwarder (Splunk/Elastic/Webhook) | **Pro** | Setzt SOC-Infra voraus |
| IP-Allowlist für Admin-Endpoints | **Pro** | Enterprise-Netzwerk-Segmentierung |
| MFA-Pflicht für sensitive API-Calls | **Pro** | Erweiterte Sicherheits-Policy |

### Änderungen gegenüber Status quo

**Entfernt** (kein Pro-Gate mehr):
- `features.Require(features.FeatureSSO)` auf SAML-Routen (`/saml/metadata`, `/saml/callback`, `/saml/acs`)
- `FeatureSSO` bleibt als Konstante für OIDC-Provider-spezifische Flows (nicht SAML)

**Neu Pro-gated:**
- SCIM-Endpoints (`/scim/v2/*`) → `FeatureSCIMProvisioning`
- SIEM-Forwarder-Config (`/admin/org/siem`) → `FeatureSIEM`
- Org-IP-Allowlist-Mgmt (`/admin/org/ip-allowlist`) → `FeatureAPI` (wiederverwendet)
- MFA-Enforcement per Call → Org-Setting, Pro-only

## Konsequenzen

- **Positiv**: SAML-Setup ist für alle Vakt-Instanzen ohne Lizenz-Upgrade nutzbar.
  Das senkt die Einstiegshürde für NIS2-konforme Zugangsverwaltung in KMUs.
- **Positiv**: Klare Enterprise-Grenze bei SCIM/SIEM schützt den Pro-Wert.
- **Risiko**: Casdoor-Dependency für SAML wird durch direkte `crewjam/saml`-
  Implementierung ersetzt — sorgfältiger Migrationstest für bestehende
  Casdoor-SAML-Kunden notwendig (Backward-Compat via Feature-Flag-Period).
- **Wartung**: SAML-Zertifikat-Rotation muss in Org-Settings dokumentiert werden.

## Referenzen

- Sprint 21 Stories S21-1 bis S21-12
- [ADR-0024](0024-model-selection-policy.md) — für Vergleich Tier-Entscheidungsmuster
- `backend/internal/auth/routes.go` — Routing-Änderungen
- `backend/internal/shared/platform/features/flags.go` — Feature-Flag-Definitionen
