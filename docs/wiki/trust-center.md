# Trust Center

Das Vakt **Trust Center** ist eine öffentliche Seite, auf der du potenziellen Kunden, Auditoren und der eigenen Geschäftsführung zeigen kannst, wie es um die Compliance deiner Organisation steht — ohne dass die Empfänger einen Account brauchen.

URL-Schema: `https://<dein-vakt-host>/trust/<slug>` — z.B. `https://vakt.acme.de/trust/acme`.

---

## Was wird angezeigt?

Konfigurierbar pro Organisation:

| Bereich | Inhalt | Toggle |
|---------|--------|--------|
| Organisations-Info | Name, Beschreibung, Kontakt-E-Mail, Logo | — (immer sichtbar) |
| Framework-Status | NIS2 / ISO 27001 / BSI / TISAX Compliance-Score in % | `show_frameworks` |
| Zertifikate | hochgeladene Zertifikate (ISO 27001-Urkunde, TISAX-Label, etc.) | `show_certs` |
| Veröffentlichte Policies | Markdown-Policies die du explizit freigegeben hast | `show_policies` |
| Sub-Processor-Liste | Markdown-Liste deiner Auftragsverarbeiter | `subprocessors_md` |

Nicht angezeigt werden **niemals**:

- Konkrete Findings, Risiken, Incidents
- Mitarbeiterdaten
- Audit-Log
- DSR-Anträge
- API-Keys

---

## Aktivierung (Admin)

1. Einstellungen → **Trust Center**
2. „Trust Center aktivieren" anschalten
3. Beschreibung + Kontakt-E-Mail ausfüllen
4. Toggle-Switches für Frameworks/Policies/Zertifikate setzen
5. Sub-Processor-Liste in Markdown hinterlegen (siehe Vorlage unten)
6. Optional: Logo hochladen
7. „Speichern"

Die Seite ist sofort unter `/trust/<slug>` erreichbar. `slug` ist der URL-Identifier deiner Organisation (Einstellungen → Organisation → Slug).

---

## Sub-Processor-Liste — Vorlage

Diese Vorlage ist DSGVO-Art.-28-konform und kann als Startpunkt dienen. Anpassen an deine konkreten Auftragsverarbeiter:

```markdown
# Sub-Processor-Übersicht

Stand: <YYYY-MM-DD>

Im Rahmen unserer Geschäftstätigkeit setzen wir die folgenden Auftragsverarbeiter
ein. Alle aufgeführten Anbieter haben mit uns einen Auftragsverarbeitungsvertrag
gemäß DSGVO Art. 28 abgeschlossen.

| Anbieter | Zweck | Sitz | Datenschutz-Mechanismus |
|----------|-------|------|--------------------------|
| Hetzner Online GmbH | Hosting (Server, DB) | DE | AVV, ISO 27001 |
| sendinblue / Brevo | Transaktions-E-Mail | FR | AVV, DSGVO-konform |
| ... | ... | ... | ... |

## Drittland-Übermittlungen

Aktuell verarbeiten wir keine personenbezogenen Daten außerhalb der EU/des EWR.

## Bei Änderungen

Diese Liste wird bei jeder Änderung des Sub-Processor-Sets aktualisiert.
Materielle Änderungen (neue Anbieter, neue Datenkategorien) werden den
Kunden mit 30 Tagen Vorlauf per E-Mail an die hinterlegte
Datenschutz-Kontaktadresse angekündigt.
```

---

## Customer-Use-Cases

**Vertrieb / RFI-Antwort.** Ein Interessent fragt im RFI „Wie sieht es bei euch mit Compliance aus?". Du verlinkst die Trust-Center-URL — fertig.

**Auditor-Vorprüfung.** Bevor ein ISO-27001-Auditor zum Termin kommt, schickst du den Link. Er sieht den groben Score und die hochgeladenen Zertifikate und kann das Auditgespräch fokussieren.

**Vorstand / Aufsichtsrat-Update.** Quartalsweise zeigt die Geschäftsführung das Trust Center im Quartals-Update — Compliance-Status auf einen Blick.

---

## Was ist es ausdrücklich **nicht**?

- **Kein Marketing-Funnel.** Es enthält keine Tracker, keine Lead-Capture-Formulare, keine Re-Targeting-Pixel.
- **Kein Compliance-Beweis.** Es ist eine Selbst-Auskunft, kein Zertifikat. Wer auditiert werden will, braucht eine echte Zertifizierung (ISO 27001 etc.).
- **Kein externer Service.** Die Seite läuft auf deinem self-hosted Vakt — keine Daten verlassen deine Infrastruktur (ADR-0001).

---

## Zugriff einschränken

Wenn du das Trust Center nicht öffentlich, sondern nur für bestimmte IPs sichtbar machen willst:

- nginx/Caddy-Layer: `location /trust/` mit IP-Allowlist oder HTTP-Basic-Auth davor
- Vakt selbst kennt keine Geo-/IP-Restriktionen auf öffentliche Endpoints

---

## API

Für SDK-/Automatisierungs-Zugriff:

```bash
# Public read (no auth):
curl https://vakt.acme.de/trust/acme

# Admin Settings (auth required):
curl -H "Authorization: Bearer $TOKEN" \
     https://vakt.acme.de/api/v1/trust-center/settings
```

Vollständige Endpoint-Doku: `GET /api/docs` → Tags `Trust Center` und `Trust Center Admin`.
