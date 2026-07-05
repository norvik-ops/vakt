# Vom Risiko zur Maßnahme

**Ziel:** Ein identifiziertes Risiko bewerten, eine Behandlungsstrategie wählen und es über
eine konkrete Maßnahme/Control auf ein akzeptables Restrisiko senken — der Kern des
ISO-27001-Risikomanagements (Klausel 6.1 / 8.2 / 8.3).

**Zielnutzer:** Informationssicherheitsbeauftragte:r (ISB), Risk-Owner
**Dauer:** ~15 Minuten pro Risiko
**Modul:** Vakt Comply → Risikoregister

---

## Schritt 1 — Risiko erfassen

1. Öffnen Sie **Vakt Comply → Risiken** (`/vaktcomply/risks`) und legen Sie ein neues
   Risiko an.
2. Beschreiben Sie Gefährdung und betroffenes Zielobjekt. Optional: aus dem
   **Gefährdungskatalog** ein Standardszenario übernehmen („Risiko aus Katalog").
3. Bewerten Sie **Eintrittswahrscheinlichkeit** (1–5) und **Schadensauswirkung** (1–5).
   Der Risiko-Score (Wahrscheinlichkeit × Auswirkung) und die Heatmap-Einordnung werden
   automatisch berechnet.

> **Methodik:** Die 5×5-Matrix (ISO 27005) ist über den **„Methodik"-Button** auf der
> Risiko-Seite einsehbar (Skalen + Score-Kategorien Niedrig/Mittel/Hoch/Kritisch).

## Schritt 2 — Behandlungsstrategie wählen

Wählen Sie je Risiko eine der vier ISO-Strategien:

| Strategie | Wann | Folge in Vakt |
|-----------|------|---------------|
| **Reduzieren** (mitigate) | Risiko zu hoch, technisch/organisatorisch senkbar | Maßnahme/Control verknüpfen (Schritt 3) |
| **Akzeptieren** | Restrisiko innerhalb des Risikoappetits | Begründung + Freigabe durch Risk-Owner |
| **Übertragen** | z. B. Versicherung, Dienstleister | Maßnahme „Transfer" + Vertrag als Evidence |
| **Vermeiden** | Aktivität/Asset wird eingestellt | Zielobjekt deaktivieren |

## Schritt 3 — Maßnahme verknüpfen und zuweisen

1. Öffnen Sie das Risiko-Detail (`/vaktcomply/risks/:id`).
2. Verknüpfen Sie eine **Control** (ISO-27001-Annex-A-Maßnahme oder BSI-Anforderung) als
   risikomindernde Maßnahme.
3. Weisen Sie einen **Verantwortlichen** und ein **Zieldatum** zu.
4. Erfassen Sie nach Umsetzung das **Restrisiko** (erneute Wahrscheinlichkeit × Auswirkung).
   Vakt zeigt Brutto- vs. Netto-Risiko (Residualrisiko, ISO 27001 Klausel 8.3).

## Ergebnis

- Das Risiko hat eine dokumentierte Behandlungsstrategie, eine verknüpfte Maßnahme, einen
  Verantwortlichen und ein nachvollziehbares Restrisiko.
- Die Verknüpfung erscheint beidseitig (Risiko ↔ Control) und fließt in die
  **Erklärung zur Anwendbarkeit (SoA)** und das Risikoregister-PDF.

## Nächste Schritte

- [Internes Audit vorbereiten](internes-audit-vorbereiten.md) — Wirksamkeit der Maßnahmen prüfen.
- Überfällige Reviews (`/vaktcomply/overdue-reviews`) — fällige Risiko-Reassessments verfolgen.
