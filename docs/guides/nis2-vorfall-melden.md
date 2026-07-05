# NIS2-Vorfall melden

**Ziel:** Einen meldepflichtigen Sicherheitsvorfall nach NIS2 (§ 32 NIS2UmsuCG / Art. 23
NIS2-Richtlinie) fristgerecht erfassen und die gestaffelten Meldepflichten gegenüber der
zuständigen Behörde dokumentieren.

**Zielnutzer:** Informationssicherheitsbeauftragte:r (ISB), Incident-Verantwortliche:r
**Dauer:** Erstmeldung in < 30 Minuten (die 24h-Frist ist knapp — Vorbereitung zählt)
**Modul:** Vakt Comply → Vorfälle (Incident Register)

> ⚖️ **Hinweis:** Vakt unterstützt die *Dokumentation* der Meldepflicht. Die rechtliche
> Beurteilung, ob ein Vorfall meldepflichtig ist, und die tatsächliche Behördenmeldung
> liegen beim Betreiber. Vakt ersetzt keine Rechtsberatung.

---

## Die NIS2-Meldefristen im Überblick

| Frist | Was | Inhalt |
|-------|-----|--------|
| **T + 24 h** | Frühwarnung | Erstmeldung: Verdacht auf erhebliche Auswirkung, mögliche grenzüberschreitende Folgen |
| **T + 72 h** | Vorfallmeldung | Bewertung von Schwere, Auswirkung, ggf. Kompromittierungsindikatoren |
| **T + 1 Monat** | Abschlussmeldung | Ursachenanalyse, ergriffene Maßnahmen, Auswirkungen |

`T` = Zeitpunkt der Kenntnisnahme des Vorfalls.

## Schritt 1 — Vorfall erfassen

1. Öffnen Sie **Vakt Comply → Vorfälle** (`/vaktcomply/incidents`) und legen Sie einen
   neuen Vorfall an.
2. Erfassen Sie Zeitpunkt der Kenntnisnahme, betroffene Systeme/Prozesse und eine
   Erstbeschreibung. Der **Kenntnis-Zeitstempel** startet die Fristberechnung.

## Schritt 2 — Meldepflicht beurteilen

1. Nutzen Sie den **NIS2-Assistenten** (`/vaktcomply/nis2-assistant`), um zu prüfen, ob
   Ihr Sektor und die Schwere des Vorfalls eine Meldepflicht auslösen.
2. Ist der Vorfall meldepflichtig, zeigt das Vorfall-Detail die drei Fristen
   (T+24h / T+72h / T+30d) mit Countdown.

## Schritt 3 — Behörde wählen und Meldungen dokumentieren

1. Hinterlegen Sie unter **Behörden** (`/vaktcomply/authorities`) die zuständige Stelle
   (in DE i. d. R. das BSI über die Meldeplattform).
2. Dokumentieren Sie je Stufe (Frühwarnung / Vorfallmeldung / Abschlussmeldung) den
   gemeldeten Inhalt und das Meldedatum am Vorfall.
3. Verknüpfen Sie ergriffene **Maßnahmen/CAPAs** — daraus entsteht automatisch der
   Abschlussbericht-Entwurf.

## Schritt 4 — Verknüpfung zum Datenschutz (falls PII betroffen)

War der Vorfall auch eine Verletzung des Schutzes personenbezogener Daten, verlinkt Vakt
Privacy den Breach (Art. 33/34 DSGVO) mit diesem Incident-Eintrag — die DSGVO-72h-Frist
läuft parallel und ist separat zu erfüllen.

## Ergebnis

- Ein lückenlos dokumentierter Vorfall mit Fristnachweis für alle drei NIS2-Meldestufen.
- Audit-fester Nachweis (Zeitstempel, Behörde, Meldeinhalte, Maßnahmen) im Incident-PDF.

## Nächste Schritte

- [Internes Audit vorbereiten](internes-audit-vorbereiten.md) — Lessons Learned in CAPAs überführen.
- Vakt Privacy → Breach (`/vaktprivacy/breach`) — bei PII-Bezug die DSGVO-Meldung dokumentieren.
