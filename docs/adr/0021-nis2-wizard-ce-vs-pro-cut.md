# ADR-0021: NIS2-Wizard — CE vs Pro Cut

**Status:** Accepted
**Datum:** 2026-05-22
**Entscheider:** Stefan (Maintainer)

## Kontext

Sprint 19 baut den NIS2-Self-Assessment-Wizard als Top-of-Funnel-Akquise-Asset. Der Bericht-§12.2 markierte ihn als „größte Chance #2" — Conversion-Hebel für den DACH-Markt 2026.

Die strategische Frage: was wird Community-Edition (CE, frei, public-zugänglich), was wird Pro-Feature (License-gated)?

Vergleichbare Plattformen:
- Vanta: Self-Assessment ist CE-Lead-Magnet, eigene Dashboards + Trend-Analyse sind Business-Tier.
- Drata: ähnlich.
- Tugboat Logic: kompletter Wizard hinter Sign-up + Trial.

Vakt-Positionierung (ADR-0008 + Memory `project_roadmap_decisions`): Wir sind self-hosted, kein zentrales Portal. Der Wizard muss anonym laufen (kein Sign-up-Zwang), sonst verlieren wir den Akquise-Hebel.

## Entscheidung

**Der Wizard selbst, Live-Score, Result-Screen und JSON-Export sind CE. Pro-Lock erst bei der Pflege-/Persistenz-Schicht.**

Konkret:

| Schicht | Tier | Wo |
|---|---|---|
| Wizard-Page `/nis2-check` (anonym, ohne Auth) | **CE** | Top-of-Funnel-Akquise |
| 30-Fragen + Live-Score-Berechnung | **CE** | Self-Bedienung |
| Result-Screen mit Top-3-Gaps | **CE** | Wertanker im Ergebnis |
| JSON-Export des Ergebnisses (Browser-Download) | **CE** | DSGVO-Auskunftsrecht-Analog |
| Magic-Link-Token (7 Tage Lebensdauer) | **CE** | Wiederbesuch ohne Account |
| Sign-up + Auto-Migration der Antworten in die Org | **CE** | Wertanker im Sign-up-Flow |
| Auto-Mapping auf bestehende Vakt-NIS2-Controls bei Sign-up | **CE** | Spart Customer ~30 min Setup |
| Branded-PDF-Export (Org-Logo, Auditor-Block) | **Pro** | Analog Audit-PDF in Vakt Comply |
| Re-Assessment-History + Trend-View über mehrere Runs | **Pro** | Premium-Compliance-Use-Case |
| Multi-Framework-Wizard (NIS2 + ISO27001 + DSGVO-TOM, ~80 Fragen) | **Pro** | Plattform-Tiefe |

## Alternativen

- **Wizard komplett hinter Sign-up** — verworfen. Senkt Conversion drastisch. Andere DACH-Compliance-Tools machen das genauso (Self-Assessment hinter Sign-up), die Vakt-Story „self-hosted + transparent" ist genau die, die das ändern darf.
- **Wizard CE, Result hinter Sign-up** — verworfen. Wer 10 min Fragen beantwortet, will sofort das Ergebnis. „Sign up to see your score" ist Conversion-Killer.
- **Wizard komplett CE inkl. Branded-PDF** — verworfen. Branded-PDF ist konsistent ein Pro-Feature in Vakt Comply (Audit-PDF, Board-Report). Den NIS2-Wizard hier auszunehmen wäre Tier-Inkonsistenz.
- **Pro-Lock pro Frage-Anzahl ("30 frei, weitere 50 als Pro")** — verworfen. Zu schmierig, schlecht zu kommunizieren. Klarer Cut: Wizard CE, Persistenz/Premium-Output Pro.

## Konsequenzen

### Positive

- Klare Lead-Magnet-Mechanik: anonymer Prospect → 10 Min Fragen → Ergebnis → CTA „Sign up, übernehme das in Vakt".
- Konsistent mit ADR-0014 (AI-Copilot ist CE) — beide adressieren das gleiche Strategie-Argument: Top-of-Funnel-Hebel mit Pro-Lock erst beim Power-User-Output.
- Marketing-Story: „Probier NIS2-Reife ohne Kreditkarte. Wenn dir das Ergebnis hilft, hol dir Vakt für die Umsetzung."
- Embedded-Mode möglich: Partner / Berater können den Wizard auf ihrer Website einbetten — bringt indirekte Akquise.

### Negative

- Public-Endpoints brauchen Rate-Limit + DDOS-Schutz. Im Sprint 19 S19-1 mit 5 req/min pro IP gestartet — ausreichend für seriöse Nutzung, blockt Mass-Abuse.
- Anonyme Daten in `nis2_anonymous_runs`-Tabelle: 7 Tage Lebensdauer, IP-Hash statt Klartext (DSGVO-konform), aber Asynq-Cleanup-Job ist Pflicht (Sprint 19 Roadmap-Restitem).
- Pro-Schicht muss klar erkennbar sein im UI — sonst empfindet Customer den CE-Wizard als „unfertig". Frontend zeigt explizite „Pro-Upgrade"-Cards für Branded-PDF + Trend-View.

### Neutrale

- Anonyme Antworten sind keine personenbezogenen Daten i.S.v. DSGVO Art. 4 — kein VVT-Eintrag, kein Cookie-Consent für die anonymen Runs (nur für die Magic-Link-Token in localStorage, aber das ist functional, kein Tracking).
- Sign-up-Migration übernimmt Antworten als initialer `manual_status` auf NIS2-Controls. Das ist eine CE-Funktion, weil sie zu Vakt-Comply gehört (das selbst CE ist) — Pro-Lock wäre dort inkonsequent.

## Referenzen

- [ADR-0008 — Kein MSP-Portal](0008-kein-msp-portal.md): self-hosted-per-customer-Prinzip
- [ADR-0014 — AI-Copilot Community](0014-ai-copilot-community-feature.md): vergleichbares CE-Argument
- Sprint 19 Backlog: S19-1 bis S19-13
- `backend/internal/shared/nis2wizard/` — Implementierung
- `frontend/src/pages/NIS2WizardPage.tsx` — Public Wizard UI
- Bericht §12.2 — „NIS2-Self-Assessment-Wizard als Top-of-Funnel: einer der größten Conversion-Hebel im DACH-Markt 2026"
