# Schutzbedarfsfeststellung durchführen

**Ziel:** Für jedes relevante Zielobjekt (Server, Anwendung, Geschäftsprozess) den
Schutzbedarf in den Kategorien Vertraulichkeit, Integrität und Verfügbarkeit (CIA)
bestimmen — die Grundlage jeder BSI-IT-Grundschutz-Modellierung und ISO-Risikoanalyse.

**Zielnutzer:** Informationssicherheitsbeauftragte:r (ISB)
**Dauer:** ~30–60 Minuten für ein Erst-Set von Zielobjekten
**Modul:** Vakt Comply → BSI-IT-Grundschutz

---

## Schritt 1 — Zielobjekte erfassen

1. Öffnen Sie **Vakt Comply → BSI → Zielobjekte** (`/vaktcomply/bsi/target-objects`).
2. Legen Sie pro Asset ein Zielobjekt an (z. B. „Fileserver FS-01", „CRM-Anwendung",
   „Personalabrechnung"). Wählen Sie den passenden Zielobjekt-Typ (IT-System,
   Anwendung, Geschäftsprozess, Raum).

> **Tipp:** Beginnen Sie mit den geschäftskritischen Prozessen, nicht mit der Technik —
> der Schutzbedarf vererbt sich von oben nach unten (siehe Schritt 3).

## Schritt 2 — CIA-Schutzbedarf bewerten

1. Öffnen Sie **Vakt Comply → Schutzbedarf** (`/vaktcomply/protection-needs`).
2. Bewerten Sie je Zielobjekt die drei Grundwerte mit einer Schutzbedarfsklasse:
   - **normal** — Schäden begrenzt und überschaubar
   - **hoch** — Schäden können beträchtlich sein
   - **sehr hoch** — Schäden können existenziell bedrohlich sein
3. Hinterlegen Sie je Bewertung eine kurze **Begründung** (Audit-Nachweis: der Prüfer
   will sehen, *warum* eine Einstufung gewählt wurde).

## Schritt 3 — Vererbung nach dem Maximumprinzip

Vakt wendet das **BSI-Maximumprinzip** an: Ein IT-System erbt den höchsten Schutzbedarf
aller Geschäftsprozesse, die es unterstützt.

1. Verknüpfen Sie in der Zielobjekt-Ansicht abhängige Objekte (Prozess → Anwendung →
   Server).
2. Prüfen Sie die berechnete Vererbung. Weicht der tatsächliche Schutzbedarf ab
   (Kumulations- oder Verteilungseffekt), übersteuern Sie die Einstufung manuell **mit
   dokumentierter Begründung**.

## Ergebnis

- Jedes Zielobjekt trägt eine begründete CIA-Einstufung.
- Die Einstufung steuert in der **BSI-Modellierung** (`/vaktcomply/bsi-modeling`), welche
  Bausteine und Anforderungen (Basis / Standard / erhöht) anzuwenden sind.
- Die Bewertungen fließen als Evidence in den Grundschutz-Check und das Audit-Paket.

## Nächste Schritte

- [Risiko zu Maßnahme](risiko-zu-massnahme.md) — Risiken zu den hoch eingestuften Objekten behandeln.
- BSI-Cockpit (`/vaktcomply/bsi/cockpit`) — Umsetzungsgrad je Baustein verfolgen.
