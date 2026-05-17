# FAQ — Häufige Fragen

---

## Allgemein

### Kostet Vakt etwas?

Vakt gibt es in zwei Editionen:

**Community Edition (CE)** — kostenlos, für immer. Alle 5 Module vollständig nutzbar. Kein Ablaufdatum, keine Kreditkarte, keine Registrierung. Einzige Voraussetzung: eigener Server.

**Pro** — 199 €/Monat oder 1.990 €/Jahr (zzgl. MwSt.). Schaltet spezialisierte Frameworks und erweiterte Features frei: TISAX, DORA, EU AI Act, CRA, KI-Berater, Audit-PDF-Export, SSO (OIDC/SAML) und API-Zugang. Nach dem Kauf erhältst du automatisch einen Lizenzschlüssel per E-Mail.

**Enterprise** — individuelle Konditionen, dediziertes Onboarding, custom SLA, White-Label. Auf Anfrage unter hello@norvikops.de.

Nicht erlaubt: Vakt als gehosteten oder verwalteten Service an Dritte verkaufen (mehr dazu unter [Was ist ELv2?](#was-ist-elv2)).

---

### Was ist ELv2?

ELv2 steht für die [Elastic License 2.0](https://www.elastic.co/licensing/elastic-license). Das ist eine Source-Available-Lizenz — der Quellcode ist öffentlich lesbar und auditierbar, aber mit einer Einschränkung:

**Was erlaubt ist:**
- Selbst hosten für die eigene Organisation
- Quellcode lesen, prüfen, forken
- Eigene Anpassungen für den internen Einsatz

**Was nicht erlaubt ist:**
- Vakt als Managed Service oder SaaS an Kunden anbieten
- Vakt als Hosted Compliance Tool für Dritte betreiben

Kurz: Wenn du Vakt für dein Unternehmen einsetzt, ist alles erlaubt. Wenn du planst, Vakt anderen Unternehmen als Service anzubieten, brauchst du eine kommerzielle Lizenz.

---

### Welche Daten verlassen meinen Server?

Keine. Vakt arbeitet vollständig lokal:

- Kein Phone-home, kein Telemetry, keine Usage-Tracking
- Keine Daten werden an Dritte übermittelt
- Der KI-Berater läuft standardmäßig über Ollama auf dem eigenen Server

Die einzige externe Verbindung, die Vakt standardmäßig aufbaut, ist eine Prüfung auf neue Releases bei GitHub (`/api/v1/version/check`) — rein lesend, sendet keine Daten. Wenn du auch das unterbinden willst, kannst du den API-Container netzwerktechnisch isolieren.

Wenn du einen Cloud-KI-Provider (z. B. Mistral AI) konfigurierst, gehen die Compliance-Abfragen an diesen Provider. Das ist optional und explizit konfiguriert — nichts passiert ohne dein Wissen.

---

### Unterschied zu Vanta, Drata oder ähnlichen Tools?

Die wichtigsten Unterschiede:

| | Vakt | Vanta / Drata |
|---|---|---|
| **Hosting** | Selbst gehostet, eigene Infrastruktur | Cloud, Daten beim Anbieter |
| **Preis** | CE kostenlos · Pro ab 199 €/Monat | ~€10.000–30.000/Jahr |
| **Datenhoheit** | Vollständig bei dir | Beim Cloud-Anbieter |
| **DSGVO** | Keine Drittübermittlung | Datenexport in US-Cloud |
| **Anpassbarkeit** | Quellcode zugänglich | Closed Source |
| **Multi-Tenancy** | Nein (eine Instanz = eine Org) | Ja |
| **Zielgruppe** | KMU, IT-Admins, DACH-Markt | Enterprise, US-Markt |

Vanta und Drata sind ausgereifte Tools mit großen Teams. Vakt ist für Unternehmen, die keine Compliance-Daten in eine US-Cloud geben wollen oder können, und die keinen fünfstelligen Jahresbeitrag zahlen möchten.

---

### Kann ich Vakt als Managed Service für meine Kunden anbieten?

Nein — das ist durch die ELv2-Lizenz explizit ausgeschlossen. Jeder Kunde muss eine eigene Vakt-Instanz auf seiner eigenen Infrastruktur betreiben.

Als MSP kannst du:
- Vakt für jeden Kunden separat deployen (eine Instanz pro Kunde, je auf der Kunden-Infrastruktur)
- Die initiale Einrichtung, Konfiguration und den Betrieb als Dienstleistung verkaufen
- Support und Updates als Service anbieten

Was nicht erlaubt ist: Eine zentrale Vakt-Installation betreiben und mehreren Kunden Zugang dazu verkaufen (Multi-Tenancy as a Service).

---

## Betrieb

### Wie erfahre ich, wenn eine neue Version verfügbar ist?

Setze `VAKT_UPDATE_CHECK=true` in deiner `.env`. Vakt prüft dann einmal täglich ob eine neue Version auf GitHub verfügbar ist und zeigt Administratoren ein Banner in der Benutzeroberfläche. Alternativ kannst du [Watchtower](https://containrrr.dev/watchtower/) für automatische Updates verwenden oder das [GitHub-Repository](https://github.com/norvik-ops/vakt) beobachten (Watch → Releases only).

---

### Wie update ich Vakt?

```bash
git pull
docker compose pull
docker compose up -d
```

Datenbankmigrationen laufen automatisch beim Start (wenn `AUTO_MIGRATE=true` gesetzt ist). Bei kritischen Produktionssystemen empfiehlt sich vorher ein Backup.

Vakt prüft automatisch, ob eine neue Version verfügbar ist, und zeigt es im Dashboard an (via GitHub Releases API, keine Daten werden übermittelt).

---

### Kann ich Benutzerrechte auf bestimmte Module einschränken?

In der **Community Edition** haben Benutzer feste Rollen (Admin, Analyst, Viewer, Auditor), die für alle Module gelten.

Mit **Vakt Pro** lassen sich Berechtigungen granular pro Modul vergeben: Jeder Benutzer erhält separat `can_read` und `can_write` für jedes der fünf Module (Vakt Scan, Vakt Comply, Vakt Vault, Vakt Aware, Vakt Privacy). Damit kann z. B. ein Analyst Lesezugriff auf Vakt Comply erhalten, ohne Schreibrechte in Vakt Vault zu bekommen.

Verwaltung: **Einstellungen → Benutzerverwaltung → Shield-Icon** neben dem jeweiligen Benutzer.

---

### Kann ich einzelne Module deaktivieren?

Ja. Jedes Modul kann unabhängig über die Umgebungsvariable `VAKT_MODULES_ENABLED` deaktiviert werden:

```env
# Nur Vakt Comply und Vakt Vault aktiv
VAKT_MODULES_ENABLED=secvitals,secvault
```

Deaktivierte Module registrieren keine API-Routen und verbrauchen keine Ressourcen.

---

### Was passiert, wenn ich den VAKT_SECRET_KEY ändere?

Alle verschlüsselten Daten in der Datenbank werden unlesbar. Das betrifft:

- Alle Secrets in Vakt Vault
- Gespeicherte SMTP-Passwörter und API-Keys

Den Key vor dem ersten Start generieren (`openssl rand -hex 32`), sicher aufbewahren und danach nie mehr ändern. Wenn der Key verloren geht oder versehentlich geändert wurde, gibt es keinen Weg, die verschlüsselten Daten wiederherzustellen.

---

### Wie sichere ich meine Vakt-Instanz?

Vakt speichert alle Daten in PostgreSQL. Backup-Strategie:

```bash
# Datenbankdump
docker compose exec postgres pg_dump -U sechealth sechealth > backup-$(date +%Y%m%d).sql

# Evidence-Anhänge (hochgeladene Dateien)
tar -czf uploads-$(date +%Y%m%d).tar.gz ./data/uploads
```

Beide Backups sicher aufbewahren — idealerweise verschlüsselt und auf einem separaten System.

---

### Wie viele Benutzer kann ich anlegen?

Es gibt kein Limit. Vakt hat keine Benutzer-basierte Lizenzierung.

---

### Unterstützt Vakt Single Sign-On?

Ja, über [Casdoor](https://casdoor.org) als OIDC/SAML-Proxy. Damit lassen sich Azure AD, Okta, Keycloak und Google Workspace einbinden. Casdoor muss separat deployt werden. Außerdem unterstützt Vakt LDAP/Active Directory-Synchronisierung.

**SSO ist ein Pro-Feature** und erfordert einen aktiven Pro-Lizenzschlüssel.

Ohne SSO funktioniert Vakt mit lokalen Benutzerkonten und optionaler 2-Faktor-Authentifizierung via TOTP.

---

### Kann Vakt ohne Internetverbindung betrieben werden?

Ja, vollständig. Alle Funktionen arbeiten lokal. Die einzigen optionalen Verbindungen nach außen:

- Version-Check via GitHub API (nur lesend, lässt sich durch Netzwerktrennung abschalten)
- KI-Provider, wenn ein Cloud-Dienst konfiguriert ist — mit Ollama lokal kein Problem

---

## Compliance

### Für welche Frameworks ist Vakt geeignet?

Vakt Comply unterstützt: NIS2, ISO 27001:2022, BSI IT-Grundschutz, DORA, TISAX, EU AI Act, DSGVO Art. 32 TOM, ISO 42001, CRA (Cyber Resilience Act).

Mehrere Frameworks können gleichzeitig aktiv sein.

---

### Reicht Vakt für eine ISO 27001-Zertifizierung?

Vakt ist ein Dokumentationstool, kein Zertifizierungspartner. Es hilft dir, alle erforderlichen Controls zu tracken, Nachweise zu verwalten und Audit-Pakete zu exportieren — aber die eigentliche Zertifizierung macht ein akkreditierter Auditor.

Was Vakt liefert: Strukturierte, auditreife Dokumentation, die einen externen Auditor zufriedenstellt. Was Vakt nicht ersetzt: den Auditor selbst.

---

### Ist Vakt DSGVO-konform?

Da Vakt selbst gehostet wird und keine Daten an Dritte übermittelt, ist die Datenschutz-Situation einfach: Du bist der alleinige Verantwortliche für die Daten in deiner Instanz. Es gibt keinen Auftragsverarbeiter.

Für die eigene DSGVO-Dokumentation (VVT, DPIA, AVV) stellt Vakt das Privacy-Modul zur Verfügung.

---

### Funktioniert Vakt Aware ohne Betriebsvereinbarung?

Im Betriebsrat-Modus (Standard) werden keine individuellen Klickdaten gespeichert — nur Abteilungs-Aggregationen. In dieser Konfiguration ist in der Regel keine gesonderte Betriebsvereinbarung erforderlich.

Wenn du individuelles Tracking aktivieren möchtest, ist eine Betriebsvereinbarung mit dem Betriebsrat notwendig. Vakt selbst erzwingt das nicht technisch — das liegt in der Verantwortung des Betreibers.
