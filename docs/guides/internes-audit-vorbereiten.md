# Internes Audit vorbereiten

**Ziel:** Ein internes ISMS-Audit nach ISO 27001 Klausel 9.2 planen, durchführen,
Findings in Korrekturmaßnahmen (CAPA) überführen und ein Audit-Paket für den externen
Prüfer exportieren.

**Zielnutzer:** Informationssicherheitsbeauftragte:r (ISB), interne:r Auditor:in
**Dauer:** Programmplanung ~30 Minuten; Durchführung je nach Umfang
**Modul:** Vakt Comply → Audits

---

## Schritt 1 — Audit-Programm anlegen

1. Öffnen Sie **Vakt Comply → Audit-Programm** (`/vaktcomply/audit-program`).
2. Legen Sie das Jahres-Auditprogramm an (ISO 27001 Klausel 9.2 verlangt geplante,
   wiederkehrende interne Audits). Definieren Sie Umfang, Kriterien und Termine.

## Schritt 2 — Einzel-Audit durchführen

1. Erstellen Sie unter **Audits** (`/vaktcomply/audits`) ein Einzel-Audit mit Geltungsbereich
   (z. B. „Zugriffskontrolle A.5.15–A.5.18").
2. Arbeiten Sie die Checkliste im Audit-Detail (`/vaktcomply/audits/:id`) ab und erfassen
   Sie je geprüfter Control die Feststellung (konform / Abweichung / Beobachtung) mit
   Nachweisbezug.

## Schritt 3 — Findings → CAPA

1. Jede Abweichung (Nonkonformität) lässt sich direkt in eine **Korrektur-/
   Vorbeugemaßnahme (CAPA)** überführen.
2. Verfolgen Sie offene CAPAs unter **Vakt Comply → CAPAs** (`/vaktcomply/capas`):
   Verantwortlicher, Fälligkeit, Wirksamkeitsprüfung.

> **Audit-Trail:** Findings und ihre CAPA-Verknüpfung sind versioniert — der externe
> Prüfer sieht, dass Abweichungen nicht nur erkannt, sondern nachverfolgt wurden.

## Schritt 4 — Audit-Paket für den externen Prüfer exportieren

1. Exportieren Sie aus dem Audit das **Audit-Paket** (PDF): Programm, Feststellungen,
   CAPAs, verknüpfte Evidence — ein in sich geschlossener Nachweis.
2. Optional: gewähren Sie dem externen Auditor über das Auditor-Portal lesenden Zugriff auf
   den abgegrenzten Geltungsbereich.

## Ergebnis

- Ein dokumentiertes, wiederkehrendes internes Auditprogramm (ISO 27001 Klausel 9.2 erfüllt).
- Abweichungen sind in nachverfolgbare CAPAs überführt.
- Ein exportierbares Audit-Paket als Nachweis für die externe Zertifizierung.

## Nächste Schritte

- [Vom Risiko zur Maßnahme](risiko-zu-massnahme.md) — aus Findings abgeleitete Risiken behandeln.
- Management-Review vorbereiten (ISO 27001 Klausel 9.3) — Audit-Ergebnisse als Input.
