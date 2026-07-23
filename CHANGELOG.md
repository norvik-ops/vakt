# Changelog

All notable user-facing changes to Vakt are documented here.
Format follows [Keep a Changelog](https://keepachangelog.com/en/1.0.0/).

---
## [0.42.48] — 2026-07-23

Sprint 131 Phase 3: die zuletzt offenen Zugriffs-/2FA-Härtungen aus dem Codeaudit v4.

### Added
- **Schrittweise 2FA-Bestätigung für sicherheitskritische Aktionen.** Organisationen können in den Einstellungen verlangen, dass sensible Schreibaktionen (API-Schlüssel anlegen/widerrufen, Nutzerrollen ändern, Sicherheitseinstellungen) zusätzlich mit einem aktuellen Authenticator-Code bestätigt werden. Lesen und normale Nutzung bleiben unberührt; das Abschalten des Schalters bleibt jederzeit möglich (kein Selbst-Aussperren).
- **MFA-Zurücksetzen für Admins (Break-Glass).** Ein Admin kann die 2FA eines ausgesperrten Mitglieds zurücksetzen, damit es sich neu einrichten kann — mit Audit-Eintrag und sofortiger Sitzungs-Invalidierung.
- **Weitere Frameworks im Katalog** (Pro): DSGVO Art. 32 TOM, CIS Controls v8, KRITIS-DachG, BSI C5:2020.

### Fixed
- Der Framework-Aktivierungs-Endpoint lehnt unbekannte Framework-Namen jetzt sauber mit `404` ab, statt einen leeren Datensatz anzulegen.

---
## [0.42.47] — 2026-07-23

Härtungs-Release aus dem Codeaudit v4 (Sprint 131): born-broken Kernfunktionen, der Neukunden-Pfad und die Zugriffsentzug-Kette geschlossen. Der Großteil der Änderungen sind interne Korrekturen und Tests — hier stehen die user-sichtbaren.

### Fixed

- **Drei Seiten crashten in eine Fehlerseite und sind wieder benutzbar.** Die Audit-Programm- und die Trainings-Report-Seite crashten im **Leerzustand** — also für jeden neuen Kunden, bis der erste Datensatz existierte; die Fund-Detailseite crashte bei jedem Öffnen. Ursache war jeweils eine leere Liste, die als `null` statt `[]` ankam. Die Handler liefern jetzt `[]`, und die Seiten sind zusätzlich gegen den Fall gehärtet.

- **Der Audit-Paket-Export lädt keine getarnte Fehlermeldung mehr herunter.** Bei einem Serverfehler wurde die Fehlerantwort als `audit-paket-<datum>.zip` gespeichert — eine als ZIP getarnte JSON-Fehlermeldung, ohne jede Warnung; ein Nutzer hätte Müll beim Auditor einreichen können. Der Download prüft jetzt die Antwort und zeigt bei einem Fehler eine Meldung, statt eine Datei zu speichern.

- **Trust Center und Lieferanten-Portal sind wieder erreichbar.** Beide öffentlichen Seiten zeigten hinter dem Reverse Proxy immer „nicht gefunden" bzw. konnten den Fragebogen nicht laden, weil ihre Daten-Route am Proxy vorbeilief. Beide sind jetzt korrekt unter `/api/v1` geroutet.

- **`docker compose up --wait` kehrt bei einer frischen Installation wieder zurück.** Der Caddy-Container meldete sich bei voll funktionierendem Proxy dauerhaft als „unhealthy" (der Healthcheck fragte eine nicht aktivierte Admin-Schnittstelle ab), sodass der Start-Befehl nie zurückkam. Der Healthcheck prüft jetzt die tatsächliche Proxy-Fähigkeit.

- **Der lokale KI-Berater funktioniert auf einer frischen Installation.** Das Feature „Local AI" war auf jeder Neuinstallation tot — das Modell konnte mangels ausgehender Verbindung nicht geladen werden, und der Fehler wurde als „gesund" gemeldet. Jetzt gibt es einen kontrollierten Egress für den Modell-Pull, und `/health` spiegelt die echte KI-Verfügbarkeit.

- **NC-/Wirksamkeits-Felder an Korrekturmaßnahmen und „bearbeitet von"/„Fristverlängerung" an Betroffenenanfragen bleiben nach dem Speichern erhalten.** Sie wurden geschrieben, aber von keinem Read zurückgelesen — nach dem Speichern waren sie in der Oberfläche wieder leer.

- **ISO 27017 und ISO 27018 sind im Framework-Katalog aktivierbar.** Beide (Pro) existierten serverseitig, hatten aber keinen Aktivieren-Knopf.

- **Zwei ins Leere führende UI-Aktionen korrigiert.** Der „Board-Bericht"-Eintrag der globalen Suche und der „Vorlage verwenden"-Knopf für DPIA/AVV-Vorlagen leiteten still auf eine Übersichtsseite um und verwarfen die Absicht.

- **Weitere Korrekturen:** Cloud-Collector melden echten statt Phantom-Erfolg, Kampagnen-Versand-Vorlagen und OpenVAS-Scan-Status korrigiert, eine fehlerhaft geformte ID in der URL gibt jetzt `400` statt `500`.

### Changed / Security

- **Zugriffsentzug greift sofort.** Ein Rollen-Downgrade, das Entfernen eines Nutzers, eine SCIM-Deprovisionierung oder eine HR-Kündigung invalidiert das Access-Token des Nutzers jetzt **sofort** — vorher galt das zustandslose Token bis zu einer Stunde weiter. Zusätzlich können Administratoren die API-Keys jedes Nutzers ihrer Organisation sehen und widerrufen.

- **Die globale Suche respektiert die organisationsweite MFA-Pflicht** — sie lief bisher an der MFA-Durchsetzung vorbei, die jede andere authentifizierte Route erzwingt.

- **Acht Endpunkte melden einen Fehler ehrlich statt still einen Erfolg.** Sie gaben `2xx` für eine Aktion ohne Wirkung zurück — u. a. lieferte die NIS2-Meldepflicht-Bewertung eine vollständige Bewertung für einen nicht existierenden Vorfall. Sie geben jetzt `404`.

- **Rechnungen: die `gross`-Backfill-Absicherung aus Migration 244 nachgezogen** (Migration 246, idempotent).

---
## [0.42.46] — 2026-07-19

### Added

- **Umsatzsteuer für DACH: Inland, EU-Ausland mit Reverse Charge, Drittland.** Bis hierhin standen `taxType: "vatfree"` und Steuersatz 0 **fest im Code** — es gab keine Verzweigung nach Land, weil es unter § 19 UStG keine brauchte. Die Zuordnung liegt jetzt in **einer reinen Funktion**, die Steuerart, Satz und Pflichthinweis **gemeinsam** liefert: Damit ist der teuerste Fehler nicht mehr formulierbar — Steuerart `net` bei 0 % bedeutet Umsatzsteuer, die geschuldet, aber nicht ausgewiesen wird (§ 14c UStG), und zwar ohne Fehler und ohne Log. Der Wechsel selbst ist eine **Konfigurationsänderung** (`VAKT_BILLING_SMALL_BUSINESS`, Default `true` = bisheriges Verhalten), keine Code-Änderung; ein Regressionstest hält fest, dass der ausgehende Payload in der Vorgabestellung **byte-identisch** bleibt. `VerifyTaxStatus()` gleicht die Einstellung beim Start gegen Lexware ab, weil eine Abweichung sonst in **beide** Richtungen still ist. Live gegen den echten Mandanten belegt: Mandant § 19 + `net` → HTTP 406, Mandant Regelbesteuerung + `vatfree` → HTTP 406. Ein Auseinanderlaufen erzeugt also **keine falsche Rechnung, sondern gar keine**.

- **USt-IdNr.-Prüfung gegen VIES, mit Nachweis.** Bei Reverse Charge geht die Steuerschuld nur bei nachgewiesener Unternehmereigenschaft über — ist die Nummer des Kunden ungültig, schuldet **der Verkäufer** die Umsatzsteuer, die er nie berechnet hat. Ein Freitextfeld trägt das nicht. Die Prüfung ist deshalb **blockierend** für EU-Auslandsverkäufe und **fail-closed**: Auch „Prüfdienst nicht erreichbar" führt zum Abbruch, nicht zum Durchwinken. Jede Prüfung wird mit Zeitstempel und Ergebnis gespeichert — eine Zeile je Prüfung, nicht je Kunde, weil Reverse Charge eine zum Zeitpunkt **des Umsatzes** gültige Nummer verlangt und jede Verlängerung eine eigene braucht. Auch das negative Ergebnis wird festgehalten: Es belegt, dass geprüft und deshalb *nicht* mit Reverse Charge abgerechnet wurde.

- **Land im Bestellformular — und der Rechnungsbetrag vor dem Klick.** Das Formular hatte **kein Land-Feld**; das Backend nahm bei leerem Wert stumm `DE` an. Jeder darüber bestellende Kunde lag damit als deutscher Kunde in der Buchhaltung, auch der aus Wien oder Zürich — unter § 19 folgenlos, mit Regelbesteuerung ein falscher Beleg. Jetzt Pflichtfeld mit Auswahlliste (DACH vorangestellt, dann EU-27 plus LI, NO, GB), und darunter die steuerliche Vorschau: Netto, gegebenenfalls Umsatzsteuer, Rechnungsbetrag — abhängig vom gewählten Land. Der Kunde sieht **vor** dem Klick, was er überweisen wird.

- **Steuerübersicht im Billing-Panel** (`/steuern`) — je Quartal die Summen nach Steuerart und darunter jede Rechnung mit Land, USt-IdNr., Steuerart und Satz. Erzeugt ausdrücklich **keine** Meldung: Umsatzsteuer-Voranmeldung und Zusammenfassende Meldung macht Lexware Office selbst. Der Zweck ist die **Anomalie-Spalte** — Steuerart `net` bei 0 %, EU-Ausland ohne gültige Prüfung, Inlandsrechnung ohne Steuer. Bewusst **keine** Warnung bei Drittland ohne USt-IdNr.: VIES kennt die Schweiz nicht, ein Hinweis dort wäre ein Fehlalarm, und Fehlalarme machen so eine Seite unbrauchbar.

### Changed

- **Freigabe-Knopf und Benachrichtigungen nennen den Rechnungsbetrag statt des Nettobetrags.** Bestätigt wird eine **unumkehrbare** Rechnung — eine finalisierte Lexware-Rechnung lässt sich über die API nicht zurücknehmen. Unter § 19 UStG waren beide Zahlen identisch; ab der Regelbesteuerung ist es der Unterschied zwischen 2.392 € und 2.846,48 €. Beträge kommen aus **derselben** Funktion, die auch die Rechnung baut; der Bruttobetrag wird als Netto + Steuer **gebildet**, nie unabhängig gerechnet.

- **Rechnungen tragen Brutto, Steuerbetrag, Satz und Steuerart** (Migration 244). Bisher stand nur ein Betrag in den Büchern. Damit fehlte, was der Kunde tatsächlich überweist — die einzige Zahl, die zu einem Kontoeingang passt — und der Nachweis, unter welchem Steuerregime eine Rechnung entstand. Letzterer ist nachträglich **nicht rekonstruierbar**. Ein `CHECK` erzwingt `brutto = netto + steuer`.

- **AGB § 4.1 korrigiert.** Die Klausel sagte „Gesamtpreise" und „Umsatzsteuer fällt nicht an, da der Unternehmer als Kleinunternehmer umsatzsteuerbefreit ist" — beides seit dem Verzicht nach § 19 Abs. 3 UStG unwahr, und „Gesamtpreise" heißt brutto, während netto zuzüglich Steuer verkauft wird. Bewusst eine **Faktenkorrektur, keine Neufassung**: Der Text ist ein eingefrorener Kanzlei-Snapshot, Struktur und Sprache bleiben.

### Fixed

- **🔴 Die USt-IdNr.-Prüfung hätte jede gültige Nummer abgewiesen.** Der VIES-Client las das JSON-Feld `valid`, die EU-Kommission liefert **`isValid`** — der Wert blieb damit konstant `false`. Fail-closed, also nicht gefährlich, aber vollständig funktionslos. Die Unit-Tests fingen es nicht, weil sie mit **derselben erfundenen JSON-Form** gefüttert waren wie der Code: Ein Test, der den eigenen Wunschvertrag prüft, bestätigt den Irrtum, statt ihn zu finden. Die Fixtures sind jetzt wörtliche Antworten der echten API. Aufgefallen an einem konkreten Fall — die **Wirtschafts-Identifikationsnummer** (§ 139c AO) hat exakt die Form einer USt-IdNr., steht aber nicht in VIES und trägt kein Reverse Charge.

- **🔴 Die Verlängerung hätte die Steuerbehandlung nie bestimmen können.** Der Datensatz, aus dem der Verlängerungslauf seine Rechnung baut, kannte weder Land noch USt-IdNr. — die Erstrechnung an einen österreichischen Kunden wäre korrekt gewesen, **jede Folgerechnung still falsch**. Keine Migration nötig: Die Spalten standen längst auf derselben Zeile, die Abfrage hat sie nur nicht gelesen.

- **Jeder reine Doku- oder Website-PR war unmergebar.** Das Branch-Ruleset fordert fünf Status-Checks; vier davon liefern ein Workflow, der wegen seines Pfadfilters bei Änderungen außerhalb von `backend/`, `frontend/` und `scripts/` **gar nicht startete**. Ein Check, der nie startet, meldet auch nie etwas — der PR blieb blockiert, obwohl nichts fehlgeschlagen war. Der Pfadfilter ist vom `pull_request`-Trigger entfernt; ein *übersprungener* Job innerhalb eines gelaufenen Workflows blockiert nicht.

### Removed

- **Ein überholter AGB-Entwurf im Repository.** Er blieb beim Umzug in den Seitenordner liegen — die eigene README des Ordners führte ihn seit dem 03.07. als „live" und den Ordner als „aktuell leer". Kein harmloses Duplikat: Er trug noch die Klauseln aus der Vorlage für **proprietäre** Software („der Quellcode ist nicht Teil der bereitgestellten Software", „zeitlich unbegrenztes, übertragbares Recht") — das Gegenteil dessen, was die Elastic License 2.0 einräumt, und genau das, was in der Live-Fassung bereits bereinigt war. Ein Rechtstext-Zwilling mit widerrufenen Klauseln ist eine Falle, kein Backup.

### Fixed

- **🔴 Läuft die automatische Verlängerung ins Leere, verlor die Instanz den Schreibzugriff ohne jede Vorwarnung.** Banner und Einstellungs-Warnung unterdrückten sich, sobald Auto-Erneuerung aktiv war — und die ist seit v0.42.43 der Default. Die Begründung im Code lautete „auto-renewal handles key rotation automatically", was nur stimmt, solange die Erneuerung **klappt**. Lehnt der Server sie ab (offene Rechnung, gekündigter Platz), passiert laut `autorefresh.go` ausdrücklich „nothing: the current key simply runs out". Genau dann schwieg die Oberfläche: Das Renew-Fenster ist ein Viertel der Key-Laufzeit (bei 90 Tagen ~22 Tage), und in dieser ganzen Zeit sah der Admin nur ein neutrales „gültig bis …" — danach schlug jeder Speichern-Versuch mit `402` fehl. Die Logik war exakt invertiert: gewarnt wurde nur, wenn Auto-Erneuerung **aus** war, geschwiegen genau dann, wenn der automatische Weg **versagte**. Fix: `/license` liefert neu `renewal_failing`; der `AutoRefresher` schreibt nach jedem Versuch mit, ob ein neuerer Key herauskam. Alle vier Fehlerpfade sind erfasst (Abruf scheitert, Antwort ist kein gültiger Key, Server liefert keinen neueren Key, Key trägt gar keinen Renewal-Token) — das Flag ist selbstheilend, denn eine geglückte Erneuerung schiebt den Ablauf aus dem Fenster und räumt es von allein ab. Banner und Einstellungen warnen jetzt über das **gesamte** Restfenster mit eigenem Text („Die automatische Verlängerung greift nicht — häufigste Ursache: eine offene Rechnung"), statt erst in den letzten 30 Tagen. Die Blockierung selbst (`402 license_expired` bei Writes, Lesezugriff bleibt) war nie betroffen. Der Fehlerfall ist zusätzlich gegen den **echten** Billing-Server abgenommen (`autorefresh_live_test.go`, Build-Tag `billinglive`): Da `GetLicense` *revoked, cancelled, unpaid und unbekannter Token* bewusst alle mit demselben `404` beantwortet — jede Unterscheidung wäre ein Orakel zum Token-Raten —, genügt dafür ein erfundener Token; eine echte Nullrechnung im Mandanten erzeugte denselben `404`, aber einen unlöschbaren Buchungsbeleg. Siehe [ADR-0052](docs/adr/0052-license-expiry-als-revokation-fuer-selfhosted.md).

### Removed

- **Die Widerrufs-Sperrliste im Lizenz-Check ist entfernt — sie hat seit dem 12.07. bei jedem Request einen Datenbankfehler ausgelöst und konnte niemanden sperren.** `DBMiddleware` fragte pro Cache-Miss `ls_revoked_subscriptions` ab; Migration 235 hatte die Tabelle beim Polar/LemonSqueezy-Ausbau gedroppt. Ergebnis: `SQLSTATE 42P01`, eine Warnung im Log, und der Code lief **fail-open** weiter — `revoked` konnte nie mehr `true` werden. [ADR-0052](docs/adr/0052-license-expiry-als-revokation-fuer-selfhosted.md) verwirft den Push in die Kunden-Datenbank ohnehin als *technisch unmöglich* (Norvik hat keinen Zugriff auf eine Self-Hosted-DB); die Sperrliste war dessen Rest aus der Polar-Ära. Widerruf funktioniert unverändert über die **Key-Laufzeit**: kein Renewal → der Schlüssel läuft binnen 90 Tagen ab. Diese Verzögerung steht in ADR-0052 seit jeher als bewusst akzeptierter Nachteil — es ging also **kein Schutz verloren, nur die Behauptung von Schutz**. Mitentfernt: `License.Revoked`, das Redis-Cache-Feld, das `revoked`-Feld aus der `/license`-Antwort (**API-Änderung**: das Feld war konstant `false`) und die Warnbox in den Einstellungen, die nie erscheinen konnte. ADR-0052 ist entsprechend fortgeschrieben (die Referenzen zeigten noch auf gelöschten Polar-Code, und die Key-Laufzeit stand dort mit 35/395 statt 90 Tagen).

### Fixed

- **Vier Endpunkte antworteten auf eine kaputte UUID im Pfad mit `500` statt `400` — der Guard dagegen war über einer Teilmenge grün.** `/admin/users/{user_id}/permissions`, `/alerting/channels/{id}/deliveries` und `/integrations/github/{id}/checks` hängen direkt an der `protected`-Gruppe und nie an einer Modul-Gruppe — dort, wo der UUID-Guard montiert war. Die frühere Entwarnung für Shared/Admin beruhte auf einer Stichprobe von sechs Routen, nicht auf einem Beweis; sie war falsch. `/vaktcomply/incident-reports/{reportId}/pdf` benutzte zusätzlich einen Param-Namen, den der Guard nicht kannte. Zwei Verstärker: Die Probe sah damals **80** Routen, inzwischen **133** — die OpenAPI-Coverage war auf 100 % gestiegen, und eine Probe ist nur so vollständig wie die Spec, aus der sie ihre Kandidaten zieht. Und der Guard war auf **elf** Mount-Punkte dupliziert worden: Wiederholung sieht nach Abdeckung aus, ist aber die Drift-Quelle. Fix: **ein** Mount an `protected` (deckt Module, admin, alerting, account, integrations, webhooks), zwölf Wiederholungen entfernt, `user_id`/`policyId`/`reportId` ergänzt. Gegenprobe über alle 133 Routen: **0× 500** (126× 400, 6× 401, 1× 404); Browser-Sweep über 76 Routen weiterhin ohne Befund; Nicht-UUID-Params (`:name`, `:control_ref`, `:type`) bleiben unberührt. Nur über einen von Hand gebauten URL erreichbar — das Frontend sendet ausschließlich echte UUIDs.

- **🔴 Vakt Scan konnte pro Organisation genau EINEN Fund speichern — jeder Scan mit zwei Funden speicherte gar nichts (Migration 243).** `vb_findings` hat drei **partielle** Unique-Indexe, und die Migration, die sie anlegt, schreibt selbst dazu: „partiell, weil die Spalten NULL sein dürfen und mehrere NULL-Werte erlaubt sein müssen". Der Code schrieb aber nie `NULL`, sondern den **Leerstring** — und `''` ist in PostgreSQL **NOT NULL**. Die Indexe griffen damit für jeden Fund mit demselben Schlüssel: Zwei Trivy-Funde auf demselben Asset kollidierten (`org, asset, 'trivy', ''`), und der `raw_id`-Index läuft sogar **ohne das Asset** — eine Organisation konnte über alle Assets hinweg genau einen Trivy-Fund halten. Weil pgx einen Batch in **eine** implizite Transaktion legt, riss die kollidierende Zeile den ganzen Batch mit: Der Scan meldete „abgeschlossen, 1 Fund" und speicherte **null**. Der gemeldete Zähler war eine Behauptung, kein Ergebnis. Aufgefallen, als der Scan-Weg zum ersten Mal von einem Test durchlaufen wurde — vorher konnte ihn keiner aufrufen (Unterprozess + Datenbank), und der Demo-Seed füllt die Fund-Tabelle direkt, sodass Vakt Scan in der Demo funktionierte.

- **🔴 Das EOL-Tracking hat nie eine einzige veraltete Komponente gemeldet.** `fetchEOL` serialisierte den geparsten Zyklus zurück, um ihn zwischenzuspeichern — aber der Typ für das `eol`-Feld hatte nur ein `UnmarshalJSON` und **kein** `MarshalJSON`. Aus dem API-Wert `"2022-05-24"` wurde damit die interne Struktur `{"IsEOL":true,…}`, die die Zeile direkt darunter wieder einzulesen versuchte — und daran scheiterte. Der Aufrufer behandelt diesen Fehler mit einer Log-Warnung und **verwirft die Komponente**: nicht einmal als „unbekannt". Jede Komponente, die endoflife.date tatsächlich kennt, fiel also still heraus. Ein Typ mit eigenem `UnmarshalJSON` braucht ein passendes `MarshalJSON` — sonst ist er nur in eine Richtung ein Typ, und die andere ist eine Falle ohne Compiler-Fehler.

- **🟠 Eine unvollständige Nuclei-Ausgabezeile hängte den Worker dauerhaft auf.** Die Parse-Schleife nutzte `json.Decoder.More()` und übersprang Decode-Fehler mit `continue` — aber `More()` bleibt nach einem gescheiterten `Decode` auf `true`: Der Decoder kommt über das kaputte Byte nicht hinweg. Das ist eine **Endlosschleife** bei 100 % CPU. Eine halb geschriebene Zeile (abgebrochener Prozess, volle Platte) genügte. Jetzt zeilenweise geparst: Eine kaputte Zeile kostet eine Zeile.

- **🟠 Ein abgelehnter Scan blieb für immer auf „läuft".** Der Status wird zu Beginn auf `running` gesetzt; lehnte der SSRF-Guard das Ziel danach ab, kehrte die Funktion zurück, **ohne** den Status wieder zu verlassen. In der Oberfläche drehte sich ein Spinner ohne Ende, und der Grund stand nur im Log. Ein abgelehnter Scan wird jetzt als gescheitert abgeschlossen — mit dem Grund am Scan.

### Security

- **Der Outbound-Guard hatte einen blinden Fleck: `internal/modules/**`.** Das Gate, das ungeguardete HTTP-Clients einfriert, schaute nur in `services/` und `platform/`. In den Modulen lebten dadurch vier ungeguardete Clients unbemerkt — OpenVAS/GVM und die EPSS-API in Vakt Scan, endoflife.date, dazu einer in Vakt Comply. Genau deshalb ist bei der letzten SSRF-Runde niemandem aufgefallen, dass es sie gibt: Der Ratchet hat sie nie gezählt. Ein Gate, das einen Teil des Codes gar nicht anschaut, friert nicht den Zustand ein, sondern den Ausschnitt, den es sieht — und meldet dafür „OK". Die drei Clients in Vakt Scan laufen jetzt über den `GuardedClient` (DNS-Rebinding-Schutz), das Gate sieht alle Module.


## [0.42.45] — 2026-07-14

### Security

- **Vakt Aware ist erstmals end-to-end abgenommen.** Eine echte Kampagne geht über einen echten SMTP-Server raus, die Mail wird aus dem Postfach zurückgelesen, ihr Link von einem Client geklickt, der **keinen Token und keine Session** hat — genau wie der Browser einer Zielperson — und der Klick landet als Zahl in der Vakt-Comply-KPI. Zwei der drei Brüche dieser Kette waren in Produktion und für jeden bestehenden Test unsichtbar; beide sind jetzt durch einen Test festgenagelt, der den Link tatsächlich anklickt.

- **Der KI-Client hängt jetzt am SSRF-Guard (Sprint 129).** Die AI-Basis-URL ist admin-konfigurierbar — eine Organisation kann sie aus der Datenbank heraus überschreiben — und war damit die letzte ausgehende Verbindung, die sich auf eine interne Adresse zeigen und **nach der Prüfung umbiegen** ließ (DNS-Rebinding). Alle drei HTTP-Clients des Pakets laufen jetzt über `httputil.GuardedClient`, der den Namen auflöst und die **aufgelöste IP** wählt, statt ein zweites Mal nachzuschlagen. Private Ziele bleiben dabei ausdrücklich erlaubt: Der Standard-KI-Provider ist ein **lokaler Ollama-Container**, und private Adressen zu verbieten hätte nichts gehärtet, sondern die KI-Funktionen in jeder Standardinstallation lautlos abgeschaltet. Ein Test hält das fest.

### Changed

- **Die OpenAPI-Spec beschreibt jetzt die ganze API — vorher knapp 60 % davon (Sprint 128).** `openapi.yaml` ist keine Dokumentation: Die TypeScript-Typen des Frontends werden **daraus generiert**, externe SDK-Konsumenten lesen sie, und der Contract-Test validiert echte Antworten dagegen. Eine fehlende Route ist deshalb eine Route, **deren Form niemand prüft**. 355 live, aber undokumentierte Routen sind ergänzt (jede Response-Form aus dem Handler-Code abgeleitet, nicht geraten), die Coverage steigt von **59,9 % auf 100 %** (883 von 883 Operationen), und ein neues CI-Gate friert das ein: Eine neue Route ohne Spec-Eintrag macht den Build rot und **nennt die Route beim Namen**.

### Fixed

- **Die Kampagnen-Statistik zeigte Unsinn, sobald sie echte Zahlen bekam.** Der Tracking-Fix machte zwei Fehler sichtbar, die seit Einführung der Seite hinter lauter Nullen lagen: Die Kachel rechnete `Klickrate × gesendete Mails` und behandelte die Rate damit als **Bruch** (0–1), obwohl die API **Prozent** (0–100) liefert; und sie rendert eine Rate von 50 % als „5000,0 %". Solange „gesendete Mails" bei 0 feststeckte, kam überall 0 heraus und niemand sah es — mit echten Zahlen hätte die Seite bei einer von zwei Zielpersonen „50 geklickt" von **einer** gesendeten Mail behauptet. Die Kacheln nehmen jetzt die echten Zähler, die das Backend längst liefert. Dabei log auch das OpenAPI-Schema für diese Antwort: Es beschrieb `sent`, `opened`, `clicked`, `submitted`, `reported` — Felder, die dieser Endpunkt noch nie zurückgegeben hat, und aus denen die Frontend-Typen generiert wurden. Gegen den Handler korrigiert. **Altdaten:** Kampagnen aus der Zeit vor dem Tracking-Fix haben keine Sende-Nachweise; ihre Nullen sind die **Abwesenheit** einer Messung und sehen in einem Balkendiagramm genauso aus wie eine. Sie tragen jetzt `tracking_measured: false`, die Kampagnenseite schreibt es hin, und die Comply-KPI lässt sie aus dem Nenner — eine ungemessene Kampagne mit 100 Empfängern hätte die Klickrate der Organisation sonst mit 100 Menschen verdünnt, deren Verhalten nie jemand beobachtet hat.

- **Elf Spec-Einträge entfernt, hinter denen nie eine Route stand.** Sie trugen „spec-ahead / TODO align" und klangen nach Endpunkten, die noch gebaut werden — tatsächlich beschrieben sie Endpunkte, die es unter diesen Namen nie gab: sechs `protection-needs`-Pfade (der Code nutzt seit jeher `/assessments/`), `PUT /ccm/checks/{id}` (der Code hat `PATCH /{id}/toggle`), `DELETE /policies/{id}`, `PUT /controls/{controlId}/measures/{mid}` und `GET /dsr/{id}`. Solange die Spec nur 60 % der API beschrieb, waren das Randnotizen; seit sie vollständig ist, tippt jeder Client dagegen und bekommt 404. **Hinweis für SDK-Konsumenten:** Wer gegen diese Pfade generiert hat, muss auf die echten wechseln — sie sind alle dokumentiert.

- **Vier Endpunkte waren in der Spec als Liste dokumentiert und haben nie eine geliefert (Sprint 128).** `/vaktcomply/policies`, `/vaktcomply/capas`, `/vaktprivacy/vvt` und `/vaktprivacy/breaches` geben seit Einführung der Pagination eine Hülle (`{data, pagination}`) zurück — die Spec behauptete weiterhin ein nacktes Array, und die daraus **generierten Frontend-Typen** behaupteten es mit. Gefunden hat es der neue **authentifizierte** Contract-Test: Der bisherige rief ohne Token auf und konnte damit fast nur Fehler-Antworten prüfen, also gerade nicht die Form der Daten, aus der das Frontend seine Typen zieht.

- **Vakt Aware hat nie einen einzigen Klick gemessen — und der dritte Fix derselben Funktion ist der erste, nach dem sie funktioniert (Sprint 126, Migration 242).** Der Tracking-Token wird pro Empfänger geprägt, in den Link gebaut, verschickt — und **nirgendwo gespeichert**. Aufgelöst wird er über `JOIN sr_events ON tracking_token`, ist also nur auffindbar, wenn zu ihm **schon ein Event existiert**; Events schreibt aber ausschließlich der Klick-Handler, der genau diese Auflösung vorher braucht. Henne und Ei: Der erste Klick einer Kampagne konnte per Konstruktion nie ankommen, jede Reaktion endete auf `invalid tracking token`. S127 hatte zwei andere Brüche derselben Kette geschlossen (alle Tracking-Routen hingen hinter Auth und gaben dem Empfänger ohne Token 401; der Klick-Link zeigte auf den Open-Pixel-Pfad) — beide Fixes waren nötig, beide zusammen nicht hinreichend. Aufgefallen ist es nie, weil der Demo-Seed `sr_events` direkt befüllt: In der Demo sah das Feature lebendig aus. **Ein vierter Bruch lag direkt dahinter:** Die Comply-KPI `PhishingClickRate` zählte `DISTINCT (Kampagne, Zielperson)` und übersprang Zeilen ohne Zielperson — der Klick-Handler schrieb aber **immer** ohne. Sie hätte auch mit funktionierendem Token 0 % gemeldet. Jede Kampagne meldete damit strukturell **0 % Klickrate** — kein fehlender Wert, sondern ein falscher: nicht unterscheidbar von „niemand ist auf die Phishing-Mail hereingefallen", und genau so floss er als Nachweis nach Vakt Comply (ISO 27001 A.6.3, NIS2 Art. 21(2)(g)). Jetzt schreibt der Versand pro Empfänger ein `sent`-Event mit dem Token, **bevor** die Mail rausgeht; die KPI dedupliziert über den Token statt über die Person und funktioniert dadurch **auch im Betriebsrat-Modus**, in dem die Zielperson bewusst nicht gespeichert wird (§ 87 BetrVG, DSGVO Art. 22). `emails_sent` in den Kampagnen-Statistiken ist damit zum ersten Mal überhaupt eine echte Zahl — das Feld wurde im Code nie gesetzt.

- **Eine CSV mit einem einzigen fehlerhaften Eintrag importierte gar keine Assets (Sprint 126).** Der Asset-Import sammelt Fehler pro Zeile und meldet Teilerfolge — lief aber in **einer** Transaktion. In PostgreSQL bricht das erste fehlerhafte Statement die ganze Transaktion ab: Jede Folgezeile scheitert, und der Commit kommt als Rollback zurück. 500 gute Zeilen und ein Tippfehler ergaben **null** importierte Assets, „alle 501 fehlgeschlagen" und eine Rollback-Meldung statt der Zeile, die zu reparieren ist. Jede Zeile bekommt jetzt einen eigenen Savepoint: Die fehlerhafte fällt heraus und benennt sich, der Rest wird importiert.

- **Der Release-Workflow committete auf `main` — und dieser Commit lief durch kein einziges Gate.** `release.yml` bumpte Helm-/OpenAPI-Version **nach** dem Tag und pushte das selbst. Ein Push, den ein Workflow mit dem `GITHUB_TOKEN` macht, löst per GitHub-Loop-Schutz **keine weiteren Workflows** aus — der Commit lag also ungeprüft auf `main`, bis zufällig jemand anderes pushte. Ausgerechnet dieser `sed` hat in [0.42.42] den **Ollama-Tag mit der Vakt-Version überschrieben**, wodurch `docker compose up` 22 Releases lang für jeden Neukunden tot war. **Die Richtung ist jetzt umgedreht:** Der Bump ist ein normaler Commit **vor** dem Tag (`scripts/release-prep.sh vX.Y.Z`), läuft durch CI wie jeder andere, und der Tag sitzt auf ihm. Der Workflow committet nichts mehr, er **prüft** nur noch — im `test`-Job, also **bevor** ein einziges Image gebaut ist. Die Prüfung fängt ausdrücklich auch den Ollama-Fall (Fremd-Image mit Vakt-Version); verifiziert, indem der Tag im Test vergiftet wurde. Damit ist auch **S123-G9** („Release-Tag auf den Bump-Commit") erstmals wirklich erfüllt — vorher war es strukturell unmöglich und stand trotzdem als erledigt im Ledger.

## [0.42.44] — 2026-07-13

### Added

- **Freilizenz — eine Lizenz ohne Rechnung** (Billing-Panel, Haken beim Anlegen). Für Design-Partner, Referenzkunden, Verbände, Beta-Tester: kein Lexware-Kontakt, keine Rechnung, sofort ein **voller** Schlüssel (kein 45-Tage-Test — es wartet ja keine Zahlung). Verlängert sich automatisch bis zur Kündigung, zählt nicht in die MRR. **Vorher gab es dafür schlicht keinen Weg:** Die gesamte Lizenz-Kette hängt an einer *bezahlten* Rechnung (`settle()` stellt den Vollschlüssel aus, `Entitlement()` liest bezahlte Perioden, `Seats.State` verlangt `status='paid'`) — „Platz vergeben" war ohne Zahlung nicht einmal erreichbar. Migration 241.
- **Freilizenz → zahlendes Abo umwandeln** (Abo-Detailseite). Das Abo bleibt **dasselbe**, damit der Kunde seinen Schlüssel **und seinen Renewal-Token** behält: Der naheliegende Weg (kündigen + neu anlegen) hätte sein `VAKT_LICENSE_TOKEN` genau in dem Moment abgeschaltet, in dem er zusagt zu zahlen (`GetLicense` prüft `cancelled_at`). Ohne „Sofort abrechnen" entsteht **nichts Unumkehrbares** — die erste Rechnung geht automatisch raus, wenn der geschenkte Zeitraum ausläuft.
- **Der Verlängerungs-Sweep meldet jetzt, was er stillschweigend ausschließt.** Seine Bedingung `(is_free OR lexware_contact_id IS NOT NULL)` lässt ein Abo, das weder das eine noch das andere hat, wortlos herausfallen: nie abgerechnet, nie verlängert, Schlüssel läuft lautlos aus. Genau dorthin führt ein `is_free = false` von Hand in der DB. Solche Waisen werden nun gezählt und als `CRITICAL` geloggt — eine Bedingung, die still ausschließt, muss zählen, was sie ausschließt.
- **Rabatt pro Kunde** (Billing-Panel). Prozentual, dauerhaft — er gilt für die erste Rechnung **und für jede Verlängerung** — und wird von Hand vergeben, nicht per Code eingelöst: Der Verkauf läuft ohnehin über eine persönliche Freigabe, ein Code-Feld am öffentlichen Formular wäre eine Angriffsfläche ohne Gegenwert. Setzbar beim Anlegen und jederzeit am laufenden Abo (greift dann ab der nächsten Verlängerung); bereits **finalisierte** Rechnungen bleiben unangetastet, die ändert nur ein Storno in Lexware von Hand. Der Kunde sieht auf der Rechnung den **Listenpreis und den Abzug** (Lexware `totalDiscountPercentage`), nicht bloß eine kleinere Zahl. Migration 240.
- **Das Billing-Panel hat erstmals Tests** (`internal/billing/admin/render_test.go`): Jede Seite wird mit einem befüllten Modell gerendert. `html/template` löst Feldnamen erst beim Ausführen auf — eine Seite, die `{{.Sub.Discount}}` gegen ein Struct ohne dieses Feld liest, parst sauber und wirft dann einen 500 bei der ersten Ansicht. Das ist das Panel, mit dem Rechnungen freigegeben werden.

### Fixed

- **Der Preis wurde an vier Stellen unabhängig voneinander aus dem Plan-Katalog abgeleitet** (Erstrechnung, Verlängerung, MRR-Metrik für Zabbix, Panel-Übersicht) — und der Betrag für Lexware kam aus einer anderen Funktion (`TotalEUR`, float) als der für die Datenbank (`TotalCents`, int). Solange beide nur multiplizierten, stimmten sie zufällig überein; mit einem Prozentsatz dazwischen können sie um einen Cent auseinanderlaufen, und **nichts im System hätte das als Fehler betrachtet** — der Kunde überweist den einen Betrag, die Bücher nennen den anderen. Jetzt rechnet genau eine Funktion (`Plan.Charge`), in Cent, und der Euro-Wert für Lexware wird daraus abgeleitet.
- **`docs/dev/billing-admin.md` behauptete, Rechnungsfreigabe sei „bewusst **nicht**" im Panel** — seit `6c520da` („Freigeben im Panel") ist sie es. Die Zeile war seit dem Panel-Umbau falsch.

## [0.42.43] — 2026-07-12

### Fixed

- **Der Public-Mirror-Sync war seit dem Billing-Panel-Commit rot — der `docker compose up`-Fix aus [0.42.42] erreichte damit keinen einzigen Kunden.** Der Leak-Guard in `scripts/build-public-mirror.sh` bricht den Sync ab, wenn ein interner Infra-Name in einer gespiegelten Datei auftaucht; genau das war passiert (`norvikserver` im `Makefile` und in einem Kommentar in `internal/billing/admin/auth.go`). Release und Deploy waren grün, der Mirror stand still. `BILLING_HOST` kommt jetzt aus einer gitignorierten `Makefile.local`; ohne sie bricht `make billing` mit klarer Anleitung ab, statt mit einem eingebackenen Hostnamen zu laufen.
- **Der Release-Tag-Sync überschrieb Fremd-Image-Tags mit der Vakt-Version — und bumpte die Chart-Version nie.** `release.yml`s `sed` traf jede `tag:`-Zeile in `helm/vakt/values.yaml`, auch Ollamas, deren Kommentar (`# pinned; update manually`) genau das ausschließen sollte. Jetzt bekommt nur ein Tag die Release-Version, dessen `repository:` auf `ghcr.io/norvik-ops` zeigt; findet der Schritt gar keine eigene Zeile, bricht er ab. Der Chart-Version-`sed` ankerte auf zwei Leerzeichen, während `Chart.yaml` die Felder auf Spalte 0 hat — er traf **nie**, und ein `|| true` verschluckte es: Das Chart meldete sich seit Ewigkeiten als `0.29.0`, während die Images auf `0.42.x` liefen. Anker korrigiert, `|| true` entfernt, jede Ersetzung wird per `grep` gegengeprüft.
- **Helm: `VAKT_AI_PROVIDER` stand fest auf `ollama`, auch bei `ollama.enabled=false`** — die API rief dann einen Service, den es nicht gab. Provider und Modell leitet die ConfigMap jetzt aus dem `ollama:`-Block ab.

### Added

- **CI-Gate `scripts/check_image_tags.py`** — prüft jeden Fremd-Image-Tag aus `docker-compose*.yml`, `infra/server/docker-compose.yml` und `helm/vakt/values.yaml` gegen die Registry und stellt offline sicher, dass **kein** Fremd-Image die Vakt-Version trägt. Bewusst über die Docker-Hub-Tags-API statt `docker manifest inspect`: letzteres zählt gegen das anonyme Pull-Limit, und ein rate-limitetes Gate meldet „existiert nicht“ für ein Image, das es gibt.
- **Billing-Metriken in Zabbix** (8 Items, 5 Trigger; siehe [`docs/dev/billing-admin.md`](docs/dev/billing-admin.md)). Der wichtigste alarmiert bei einem Lexware-Storno für eine Rechnung, deren Lizenzschlüssel schon beim Kunden liegt — das Teuerste, was dieses System entdecken kann.

## [0.42.42] — 2026-07-12

### Fixed

- **`docker compose up` startete seit v0.42.20 überhaupt nicht mehr.** Der Ollama-Dienst zeigte auf `ollama/ollama:${OLLAMA_TAG:-0.6}` — einen Tag, den Ollama **nie veröffentlicht hat** (es gibt `0.6.8`, aber kein blankes `0.6`). Der Wert stand seit dem initialen Monorepo-Merge im Compose und fiel nie auf, weil Ollama hinter `--profile ai` hing. Als die KI mit v0.42.20 default-on wurde, fiel das Profil weg — und damit brach docker compose den **gesamten** `up` ab (Exit 1, **kein einziger Container startet**), also genau der beworbene „in unter 5 Minuten startbereit"-Weg. Jetzt auf `0.31.2` gepinnt. Ein neues CI-Gate (`scripts/check_image_tags.py`) prüft jeden Fremd-Image-Tag gegen die Registry.
- **Helm-Chart: das KI-Deployment hat noch nie funktioniert.** Der Ollama-Image-Tag trug die **Vakt**-Version (zuletzt `0.42.41`), weil das Release-`sed` blind jede `tag:`-Zeile mitzog — `ImagePullBackOff` für jeden, der mit `ai.enabled` deployt. Außerdem gab es **zwei unabhängige Knöpfe** für dasselbe Modell (der Init-Job zog `ollama.model`, die API las `api.env.VAKT_AI_MODEL`): Wer nur einen umstellte, ließ ein Modell ziehen, nach dem die API nie fragte. Jetzt ein Wert; Modell auf `qwen2.5:7b` (wie überall sonst), Ressourcen von 2 Gi/4 Gi auf 5 Gi/8 Gi angehoben (7b hätte den Pod sonst beim ersten Prompt OOM-gekillt), und `VAKT_AI_PROVIDER` folgt `ollama.enabled` statt bei `false` einen Service zu rufen, den es nicht gibt.
- **Die Lizenz-Mail versprach jedem Kunden eine Jahreslaufzeit** — auch dem Monatskunden, der 299 € zahlt und einen 30-Tage-Schlüssel bekommt. Sie nennt jetzt das echte Ablaufdatum des mitgeschickten Schlüssels.
- **Eine Abrechnungsperiode war 30 Tage, kein Kalendermonat.** 12 × 30 = 360 Tage, also 12,17 Rechnungen im Jahr (3.639 € statt der ausgewiesenen 3.588 €), und das Rechnungsdatum wanderte durch den Monat (1.3. → 31.3. → 30.4.). Jetzt Kalendermonate, geklemmt auf den letzten Monatstag (31.01. + 1 Monat = 28.02., nicht 03.03.).

### Changed

- **Bestellseite und Preiskarte sagen jetzt die Wahrheit.** Laufzeit und Zahlungsziel unter dem Formular folgen der getroffenen Auswahl (vorher standen dort fest die Jahreswerte — 14 Tage statt der 10 des Monatsplans). Der Button heißt **„Kostenpflichtig bestellen"** statt „Angebot anfordern": Wir verschicken kein Angebot, sondern eine Rechnung — und nach den eigenen AGB (§ 3.1/3.2) ist **die Bestellung des Kunden** das verbindliche Angebot. Entfernt: „30 Tage kostenlos testen · jederzeit kündbar" auf der Preiskarte und „Polar-Checkout, 30 Tage kostenlose Testphase" in `docs/setup.md` — es gibt **keine** Testphase; man bestellt und wird sofort in Rechnung gestellt, der 45-Tage-Schlüssel liegt der Rechnung **bei**.

## [0.42.41] — 2026-07-12

### Added

- **Billing-Admin-Panel im Vakt-Look mit echtem Lexware-Abgleich** — Navigation, Übersicht, Abos, Rechnungen, Lizenzen und eine Abgleich-Seite, die Vakts Rechnungen gegen das stellt, was Lexware wirklich sagt (Storno, nur-in-Lexware, Betragsabweichung). `billing.norvikops.de` und `lizenz.norvikops.de` bekommen eigene Hosts; das Panel steht hinter **Cloudflare Access** (Edge-Auth plus JWT-Prüfung im Prozess selbst).

## [0.42.40] — 2026-07-12

### Added

- **Eigener Billing-Dienst (`cmd/billing`) — der Lizenz-Signaturschlüssel lag neben 919 Routen.** Der Verkaufsweg lief bisher im ISMS-Prozess mit; jetzt ein eigener Prozess mit sechs öffentlichen Routen, getrenntem Admin-Listener und eigenen Metriken. `api.norvikops.de` ist damit auf `/api/v1/billing/*` verengt — die ISMS-App ist dort nicht mehr erreichbar.
- **Token pro Lizenz, Lizenz-Heartbeat und MSP-Self-Service-Portal** — jede Lizenz trägt ihren eigenen Erneuerungs-Token. Vorher hing er am **Abo**: Alle zehn Endkunden eines MSP hätten denselben Schlüssel bekommen, nämlich den des MSP. Auto-Erneuerung ist jetzt **Default**, ruft aber nur an, wenn der Schlüssel tatsächlich abläuft (Jahreslizenz: einmal im Jahr, nicht 365-mal).
- **Schlüssel-Laufzeit an den bezahlten Anspruch gekoppelt statt an eine feste Frist.** Ein kurzlebiger Schlüssel mit Dauererneuerung hätte die Folgen **unseres** Ausfalls dem Kunden aufgebürdet, der ein Jahr im Voraus bezahlt hat: Fällt unser Lizenzserver drei Monate aus, geht sein ISMS dunkel — möglicherweise mitten im Audit. Ein Schlüssel ist jetzt bis zum Ende der zuletzt **bezahlten** Periode plus Karenz gültig: nicht länger, aber eben auch nicht kürzer. Kontrolle und Robustheit fallen aus derselben Regel.
- **Neues Marken-Logo: Monogramm-Badges statt Schild-Logos ([ADR-0070](docs/adr/0070-marken-logo-system.md)).** Alle vier Marken tragen jetzt denselben Badge — gleicher Container, Eckenradius und Strichstärke — mit eigenem Buchstaben und eigener Farbe: **N** teal für NorvikOps (Dachmarke), **V** indigo für Vakt, **D** sky für DirHealth, **F** amber für ForgeHive. Die vier Schilde vorher unterschieden sich praktisch nur im Farbton und waren nebeneinander kaum auseinanderzuhalten; ein Schild ist außerdem die Aussage eines *Produkts*, nicht die eines Hauses, das mehrere trägt. Ein einziges Logo für alles wurde bewusst verworfen — das Vakt-Icon trüge dann im Browser-Tab ein „N".
  - **Die Wortmarke besteht jetzt aus Pfaden, nicht aus `<text>`.** Die gelieferten SVGs setzten `font-family="Space Grotesk"` — aber ein SVG in einem `<img>`-Tag erbt die `@font-face`-Regeln der Host-Seite **nicht**, das Logo wäre also bei jedem Besucher in der Fallback-Schrift gerendert worden. Ausgezeichnet aus `@fontsource/space-grotesk` (600). Damit entfallen zugleich die Subpixel-Farbsäume der PNG-Vorlagen (2.652 farbige Kantenpixel im Lockup → 0).
  - **DirHealth wechselt von Blau (`#3B82F6`) auf sky-400 (`#38BDF8`).** Das alte Blau lag unter Rot-Grün-Schwäche bei nur ΔE 4,6 zu Vakt-Indigo — die beiden Produktmarken waren für farbfehlsichtige Nutzer nicht unterscheidbar (jetzt ΔE 19,0). Flächen bekommen `sky-700`, nicht `sky-400`: weißer Text auf sky-400 erreicht nur 2,14 Kontrast (WCAG-Minimum 4,5).
  - Favicons in Produktfarbe inkl. 16/32 px (mit verstärkter Strichstärke, sonst säuft die Diagonale bei 16 px ab), echtes `apple-touch-icon.png` (Safari lädt kein SVG dafür — der bisherige Link zeigte ins Leere) und PWA-Icons 192/512 plus ein eigenes maskable mit Glyph in der 80-%-Safe-Zone.
- **Hero-Effekte auf allen Sites vereinheitlicht.** Die laufenden Lichtbalken, das Partikelnetz und der atmende Glow gab es nur auf vakt.norvikops.de — norvikops.de hatte ein statisches Raster, dirhealth.app gar nichts, und beide trugen einen Glow in *Vakts* Indigo. Jetzt eine gemeinsame Komponente (`HeroFX.astro`) mit der Akzentfarbe als Prop; `prefers-reduced-motion` schaltet Bewegung und Partikelschleife weiterhin ab.
- **dirhealth.app:** „★ Star on GitHub" im Vakt-Cross-Sell-Block zeigt jetzt auf `releases/latest` („↡ Download DirHealth") — eine Stern-Bitte war dort die schwächere CTA.

### Fixed

- **Produktionsstörung: `api.norvikops.de` gab 502.** Die Caddy-Konfiguration zeigte auf ein Reverse-Proxy-Ziel, das es noch nicht gab — und `deploy-sites.yml` rollte sie sofort aus. Regel seither: Ein Reverse-Proxy-Ziel wird erst eingetragen, wenn der Container **existiert**.
- **Bewegung lief trotz `prefers-reduced-motion` weiter (WCAG 2.2.2)** — Der Hero von `vakt.norvikops.de` animierte endlos: Lichtstrahlen, pulsierende Glows und ein Partikel-Canvas mit `requestAnimationFrame`. Die einzige Reduced-Motion-Regel deckte nur die Scroll-Einblendung (`.reveal`) ab; **17 Animationen liefen weiter**, auch wenn der Nutzer reduzierte Bewegung verlangt. `sites/main` hatte gar keine Regel. Jetzt: Hero wird statisch (Inhalt bleibt vollständig sichtbar, nur die Bewegung entfällt), die rAF-Schleife des Canvas startet erst gar nicht (sonst liefe sie unsichtbar weiter und kostete CPU/Akku), und die endlos laufenden Tailwind-Utilities (`animate-pulse`/`animate-ping`) sind site-weit abgeschaltet. Gemessen: vakt 17 → **0** laufende Animationen unter `prefers-reduced-motion: reduce`, ohne Inhaltsverlust.

### Changed

- **„Kein Phone-Home“ ist so nicht mehr wahr — in acht öffentlichen Flächen korrigiert.** Die opt-in Lizenz-Auto-Erneuerung kontaktiert `api.norvikops.de`. README, Landingpage, Wiki, Launch-Texte, PRD und ADR-0001 sagen jetzt „keine Telemetrie, keine Nutzungsdaten“ statt eines Absolutheitsanspruchs, den das Produkt nicht mehr einlöst.

## [0.42.39] — 2026-07-12

### Fixed

- **Lizenzversand: Fallback-Poller und eine Race Condition geschlossen.** Bleibt der Lexware-Zahlungs-Webhook aus (Netz, Key-Rotation, Ausfall), holt ein Poller die Zahlungen nach — sonst bezahlt jemand und bekommt still nichts. Die Race zwischen Webhook und Poller kann keinen zweiten Schlüssel mehr ausstellen.

## [0.42.38] — 2026-07-12

### Security

- **Der Rechnungs-Freigabe-Token stand im Klartext im Access-Log (CWE-532).** In der Datenbank liegt er nur als SHA-256-Hash, damit ein geleaktes Backup niemandem erlaubt, Rechnungen freizugeben — und dann schrieb der Request-Logger die volle URI samt Token in ein Log, das per Promtail an **Loki auf einem anderen Server** geht. Wer Loki lesen konnte, konnte Rechnungen freigeben. Der Token **muss** in der URL stehen (ein Ein-Klick-Link aus einer E-Mail kann nicht POSTen), also ist Maskieren beim Hinausschreiben die einzige Lösung: `redactQuery()` ersetzt die Werte von `token`, `access_token`, `api_key`, `key`, `secret`, `password` sowie OAuth-`code`/`state`; harmlose Parameter (`page`, `limit`) bleiben lesbar, sonst wären die Logs zum Debuggen wertlos.

## [0.42.37] — 2026-07-12

### Fixed

- **Drei Fehler aus dem ersten echten Rechnungslauf.** Lexware lehnte das Datumsformat ohne Millisekunden ab; die HEAD-Probe auf den Webhook-Endpunkt wurde mit 401 beantwortet (Lexware prüft die Erreichbarkeit, bevor es registriert); und die Mail nach Zahlungseingang versprach einen Anhang, den sie gar nicht hat — die Rechnung ging bereits mit der ersten Mail raus.

## [0.42.36] — 2026-07-12

### Added

- **Direktverkauf auf Rechnung (Lexware Office) — Polar (US-Merchant-of-Record) abgelöst.** Vakt verkauft ein Produkt, dessen Kernversprechen „keine Daten in US-Clouds" ist, wickelte den Kauf aber über **Polar Software, Inc.** ab — eine Delaware Corporation nach Delaware-Recht, ohne EU-Entität und (gegen die offizielle Liste geprüft) **ohne DPF-Zertifizierung**. Name, E-Mail, Rechnungsanschrift, USt-IdNr. und Zahlungsdaten jedes Kunden lagen damit bei einer US-Gesellschaft. Dazu ein Vertragsproblem: Als Merchant of Record verkaufte *Polar* im eigenen Namen — die anwaltlichen B2B-AGB von NorvikOps beschrieben damit einen Vertrag, der gar nicht geschlossen wurde; beim Direktverkauf passen sie exakt. Und ein Marktargument: Im deutschen B2B ist Kauf auf Rechnung mit 43 % das meistgenutzte Zahlungsmittel (HHL-Studie, n=830) — ein reiner Kreditkarten-Checkout ist für IT-Leiter im KMU eher Hürde als Komfort.
  - **Ablauf:** Angebotsformular (`/angebot`) → Mail an den Verkäufer mit signiertem Freigabe-Link → *[1 Klick]* Lexware-Kontakt + finalisierte Rechnung + PDF + 45-Tage-Lizenzschlüssel an den Kunden → Überweisung → *[1 Klick]* Zahlung in Lexware zuordnen → Webhook → Vollschlüssel (395 Tage) automatisch signiert und gemailt.
  - **Der Zahlungs-Webhook ist ein unvertrauter Hinweis, nicht die Wahrheit.** Er trägt nur eine Resource-ID; der Code fragt die Lexware-API und stellt **nur bei `paymentStatus == "balanced"`** aus. Damit laufen drei Fälle ins Leere: ein gefälschter Webhook, ein Replay — und der realistischste, eine **Teilzahlung**: `payment.changed` feuert auch dafür. Eine 100-€-Überweisung darf keinen 2.990-€-Schlüssel auslösen.
  - **Beide Klicks sind Absicht, nicht Bequemlichkeit.** Ein öffentliches Formular, das ohne Kontrolle finalisierte Rechnungen unter unserer Steuernummer erzeugt, wäre eine offene Flanke — und eine finalisierte Rechnung lässt sich per API nicht zurückdrehen. Der Freigabe-Link trägt ein 32-Byte-Token (gespeichert wird nur der SHA-256-Hash, verglichen in konstanter Zeit) und ist idempotent, weil Mail-Clients Links prefetchen. Lexware verbucht Zahlungen ohnehin nicht ohne Klick („ohne Ihre aktive Übernahme findet keine endgültige Verbuchung statt") — der Klick ist der Moment, in dem jemand Betrag und Absender prüft.
  - `features.ProTier` ist jetzt die einzige Quelle für den Pro-Funktionsumfang, `license.KeyExpiry` die einzige für die Laufzeit — beide lagen unexportiert im Polar-Handler. Ein per Rechnung gekaufter Schlüssel muss exakt dasselbe freischalten wie ein per Karte gekaufter; zwei Listen wären zwei Produkte.
  - Der Lexware-Webhook wird bei **jedem Boot** neu registriert: Das Rotieren des API-Keys löscht alle Event-Subscriptions, und der Key läuft nach 24 Monaten ab — sonst verstummen bezahlte Rechnungen still, und niemand merkt es, bis ein Kunde schreibt.
  - Rechnungen tragen zwingend `taxType: "vatfree"` (§ 19 UStG, gegen die echte Lexware-API verifiziert: `smallBusiness: true`); `taxTypeNote` wird bewusst weggelassen, damit Lexware den dort hinterlegten § 19-Text einsetzt — eine Stelle, nicht zwei. Neue Seite `/angebot` mit **AGB-Pflichtcheckbox**: Nach den AGB ist die Bestellung des Kunden das Angebot, die AGB müssen also **vor** dem Absenden vorliegen, nicht erst auf der Rechnung. Migration 233. Auf Kunden-Instanzen bleibt alles dunkel (kein `VAKT_LEXWARE_API_KEY`, kein Signaturschlüssel).

### Fixed

- **Wettbewerber-Preisangaben in der öffentlichen Kommunikation waren frei erfunden (UWG-Risiko)** — Die Vergleichstabelle auf `vakt.norvikops.de` berief sich auf „Preise laut öffentlichen Preislisten der Anbieter". **Vanta, Drata und DataGuard veröffentlichen keine Preise** (alle drei reines Contact-Sales, geprüft 2026-07-11) — die Seite schrieb damit drei namentlich genannten Mitbewerbern konkrete Euro-Preise zu und erfand die Quelle dazu: vergleichende Werbung mit unzutreffenden Preisangaben, angreifbar nach **§§ 5, 6 UWG**. Die Zahlen waren zusätzlich inhaltlich falsch (Vanta als teurer als Drata dargestellt — aggregierte Ist-Vertragsdaten zeigen das Gegenteil) und gaben USD-Werte als Euro aus. Dieselbe Behauptung stand mit **fünf verschiedenen Zahlen in sechs öffentlichen Flächen**: Landingpage, SEO-Blogpost (dort zusätzlich im **FAQ-Structured-Data**, wo Google den falschen Claim als Rich Snippet verstärkt), `README.md` (Public Mirror), `docs/wiki/faq.md` (öffentliches GitHub-Wiki), `docs/launch-producthunt.md` und `sites/main` (DE + EN). Alle korrigiert auf belegte Spannen mit Quellenangabe (Vendr-Ist-Vertragsdaten). Neue Quelle der Wahrheit: `docs/marketing/competitor-pricing.md` — jede künftige Wettbewerber-Preisaussage stammt daraus oder wird nicht getroffen. Die Vergleichsspalte „Setup: Wochen" (nicht objektiv nachprüfbar) wurde durch „Zugang: Sales-Gespräch" ersetzt (nachprüfbar: alle drei Pricing-Pages erzwingen einen Demo-Termin).

## [0.42.35] — 2026-07-12

### Security

- **Echtes MFA statt Enrollment-only (Sprint 124, SA14-01)** — MFA war bisher rein enrollment-basiert: ein korrektes Passwort stellte sofort ein volles Token-Paar aus, und die Enforce-Middleware prüfte nur, ob TOTP *eingerichtet* ist — nie, ob die aktuelle Session den zweiten Faktor *bewiesen* hat. Ein gestohlenes Passwort authentifizierte damit voll, auch bei `require_mfa=true`. Jetzt zweistufiger Login: Passwort → kurzlebiger `mfa_pending`-Token (5 min, rollenlos, wird als Access-Token abgelehnt) → nach TOTP-/Backup-Code am neuen `POST /auth/2fa/login-verify` das echte, `mfa=true`-markierte Token-Paar. `MFAEnforce` verlangt den bewiesenen Faktor. SSO/Recovery-Code zählen als starker Faktor. Zwei-Stufen-UI im Frontend (de/en/fr/nl). Verifiziert per testcontainers-Integrationstest.
- **Sprint 122 (P0-Audit-Nachlauf) — Rechteausweitung geschlossen: `SecurityAnalyst` konnte Cloud-Credentials, GitHub-PATs und externe Auditor-Magic-Links schreiben.** Drei Route-Gruppen (`/integrations/cloud`, `/integrations/github`, `/auditor`) hingen ohne Rollen-Gate unter `protected` — jeder `SecurityAnalyst` konnte AWS-Zugangsdaten speichern (13 Provider × `PUT /config`), eine GitHub-Integration inkl. Personal-Access-Token anlegen und einen org-weiten Auditor-Magic-Link ausstellen (live mit einem Analyst-Account demonstriert). Fix: `auth.RequireRole("Admin")` an allen drei Group-Mounts (`cmd/api/routes.go`) — **Admin-only**, bewusst nicht `Admin,SecurityAnalyst`, da der PoC genau ein SecurityAnalyst war. Regressionstest `internal/rbaccov/s122_integrations_rbac_test.go` probt **SecurityAnalyst *und* Viewer → 403** (nicht nur Viewer — eine reine Viewer-Probe hätte den Angriff durchgelassen). Sollen Analysts Integrationen fachlich verwalten, kommt dafür eine feinere Rolle in einem Folge-Sprint; das Gate wird nicht aufgeweicht.
- **SMTP-Header-Injection (CWE-93) in 10 E-Mail-Versandpfaden geschlossen** — dieselbe CRLF-Header-Injection-Klasse, die im `form-handler` (S120-3) bereits gefixt war, stand unangetastet im Alerting-Service (`subject` trägt user-/event-kontrollierten Text) und in neun weiteren hand-gebauten Header-Buildern (Scheduled-Reports, Policy-Acceptance, User-Invites, Password-Reset, E-Mail-Digest, Notifications-Mailer, SecReflex-Worker, Polar-/LemonSqueezy-Webhooks). Ein `\r\n` in `From`/`To`/`Subject` konnte zusätzliche Header (`Bcc:`) oder einen gefälschten Body injizieren. Fix: ein zentraler Guard `internal/shared/mailhdr.Sanitize` strippt CR/LF an jeder Header-Interpolation; Unit-Test deckt die Injektionsfälle ab. Bodies bleiben unberührt (Newlines dort sind legitim).

### Fixed

- **Vakt Aware war im Kern funktionslos: Tracking-Routen hingen hinter Auth (Sprint 127)** — Der Tracking-Pixel, der Klick-Link, das Landing-Page-Formular und der Phish-Report-Webhook waren als „public" kommentiert, aber unter `protected` gemountet. Sie werden per Definition von Empfängern *ohne* Vakt-Session aufgerufen (Mail-Client, Ziel-Browser) → jede Öffnung/jeder Klick endete auf 401. Damit meldete `PhishingClickRate` (Evidence für ISO 27001 A.6.3 / NIS2 Art. 21(2)(g)) strukturell 0 %. Zusätzlich zeigte der Klick-Link auf den Öffnungs-Pixel-Handler (Klicks wurden als Öffnungen verbucht). Auch Vakt-Vault-Share-Links (`/share/:token`) gaben externen Empfängern 401. Alle betroffenen Routen laufen jetzt öffentlich (Token-validiert, Redis-IP-Rate-Limit statt In-Memory), der Klick-Link zeigt auf den richtigen Handler, und ein neues CI-Gate erzwingt die Erreichbarkeit-ohne-Token dieser Routen.
- **Policies-Export-Button lief in einen 404 (Sprint 124, CB-01)** — Der XLSX-Export auf der Richtlinien-Seite rief einen Endpoint auf, den es im Backend nicht gab. Route gebaut (`GET /vaktcomply/policies/export/xlsx`), Muster wie der Controls-Export.
- **Managementbewertung-Approve zeigte nach dem Genehmigen weiter „Entwurf" (Sprint 124, N2)** — `UpdateManagementReviewInputs/Outputs` und `ApproveManagementReview` nutzten dasselbe read-after-write-Muster (`WITH upd AS (UPDATE … RETURNING) SELECT … JOIN`), das den Vor-Zustand las: die Änderung wurde persistiert, aber die Antwort zeigte die alten Werte. Auf direktes `UPDATE … RETURNING` umgestellt.
- **Asset-Klassifizierung wurde nie angezeigt (Sprint 124, DB-01)** — `classification` wurde gespeichert, aber in keiner Read-Query gelesen → das Badge blieb leer. In die Read-Queries + das Modell aufgenommen.
- **Sprint 122: DSR-Export-Button lieferte PDF statt der versprochenen CSV** — der „CSV"-Button auf der Vakt-Privacy-DSR-Seite rief `GET /vaktprivacy/dsr/export?format=csv`, der Handler ignorierte den `format`-Parameter aber vollständig und lieferte immer PDF (als `.csv` benannt). Jetzt honoriert der Handler `format=csv|pdf`; die CSV trägt denselben audit-log-Umfang wie das PDF (bewusst **keine** Antragsteller-Personendaten, Art. 5 DSGVO Datensparsamkeit). PDF- und CSV-Export teilen sich jetzt eine gemeinsame Zeilen-Fetch-Funktion, damit die zwei Formate in Spalten/Umfang nicht driften können.
- **norvikops.de: englische Seiten fehlten in der Sitemap** — `/en/`, `/en/about` und `/en/contact` waren live erreichbar, standen aber in keiner Sitemap; die handgepflegte `public/sitemap.xml` listete nur die 5 deutschen Seiten. Suchmaschinen konnten den englischen Teil nur über interne Links finden. Ersetzt durch `@astrojs/sitemap` (dieselbe Integration wie auf `sites/vakt`) mit i18n-Konfiguration → alle 8 Seiten inkl. `hreflang`-Alternates für `de-DE`/`en-US`, `robots.txt` zeigt auf `sitemap-index.xml`. Eine handgepflegte Liste driftet — eine generierte nicht.

### Added

- **Landingpage-Überarbeitung nach CRO-Audit** (`sites/vakt`, alle 9 Findings aus `docs/vakt-landingpage-findings.md`) — Neue Headline („Der Auditor kommt. / Deine Compliance-Daten bleiben trotzdem im Haus."), Persona-Block „Für wen ist Vakt?" **vor** der Modulsektion (die 6 Module lasen sich zuvor wie ein Produktkatalog), Install-Sektion trennt jetzt „Plattform < 5 Min." vom optionalen KI-Modell-Download (3–30 Min.) statt beides zu vermischen, Pro-CTA erklärt in drei Schritten was nach dem Klick passiert (sprang zuvor kontextlos in den Polar-Checkout), Community-CTA zeigt auf die Install-Anleitung statt auf GitHub (Sackgasse für nicht-technische Entscheider), neue Abschluss-Sektion mit der **NIS2-Checkliste als PDF-Download**. Grundsatzentscheidungen dazu in [ADR-0069](docs/adr/0069-marketing-integritaet-keine-erfundenen-signale.md): keine erfundenen Trust-Signale (das Repo hatte 0 Stars, Vakt hat keinen Kunden — Testimonials und Nutzerzahlen wären erfunden gewesen), keine erfundene Preis-Urgency, **Lead-Magnet ungegatet** (kein E-Mail-Gate — eine Seite, die mit Datensouveränität argumentiert, verlangt keine E-Mail-Adresse für ein PDF). Stattdessen drei nachprüfbare Trust-Signale: letztes Release (Build-Time-Fetch der GitHub-API — bewusst **nicht** im Browser, sonst ginge die IP jedes Besuchers an GitHub), Dogfooding, Quelloffenheit.
- **Cookiefreie Conversion-Events (Umami)** auf `sites/vakt` — bisher wurden nur Seitenaufrufe erfasst, jede CRO-Iteration war damit Bauchgefühl. Sechs Events (Hero-CTA, GitHub, Header, Community, Pro-Checkout, Copy-Command, Lead-Magnet-Download). Die Datenschutzerklärung wurde im selben Zug nachgezogen: Sie sagte, es würden „**ausschließlich**" Seitenaufrufe/Referrer/Herkunft/Browser/Gerätetyp erfasst — Klick-Ereignisse sind eine neue Kategorie, die Aufzählung wäre sonst unwahr geworden. Weiterhin keine Cookies, keine personenbezogenen Daten (erfasst wird nur der Name der Schaltfläche).
- **Zwei Lead-Magneten als ungegatete PDF-Downloads** — „NIS2-Compliance-Checkliste für KMU" (3 Seiten) und „ISO 27001 in 90 Tagen" (4 Seiten), verlinkt auf der Landingpage (`FinalCTA.astro`) und kontextuell in den passenden SEO-Artikeln. Kein E-Mail-Gate, keine Anmeldung (ADR-0069). Erzeugt aus `docs/marketing/lead-magnets/*.md` via **`scripts/build-lead-magnet-pdf.mjs`** (Playwright); die PDFs sind eingecheckt, nicht CI-gebaut (Playwright im Sites-Build wäre unverhältnismäßig für Assets, die sich selten ändern). Das Skript verwirft alles vor dem ersten `---` der Quell-Markdown — dort stehen die internen Redaktionsnotizen, die nicht ins Kunden-PDF dürfen. Bewusst **strukturell** abgegrenzt statt über eine Stichwortliste: Eine Stichwortliste bricht still, sobald jemand die Notiz umformuliert.

## [0.42.34] — 2026-07-11

### Fixed

- **`POST /vaktcomply/management-reviews` (Managementbewertung anlegen) war seit Einführung born-broken — `500` + Duplikat bei jedem Retry.** Die Query war `WITH ins AS (INSERT INTO ck_management_reviews (...) RETURNING id) SELECT ... FROM ck_management_reviews mr JOIN ins ON mr.id = ins.id`. In PostgreSQL laufen alle `WITH`-Sub-Statements plus die Hauptabfrage unter EINEM Snapshot — der äußere Scan von `ck_management_reviews` sieht die von der `ins`-CTE gerade eingefügte Zeile nicht, der JOIN liefert 0 Zeilen → `pgx.ErrNoRows` → Handler-`500`. Die Zeile wurde trotzdem eingefügt (live per `SELECT count(*)` bestätigt), also erzeugte jeder Retry ein Duplikat. Der vorherige Empty-Body-Route-Sweep verpasste es, weil `review_date` `required` ist (`422` vor dem INSERT); erst ein Happy-Path-Create mit validem Body fährt bis zum INSERT durch. Fix: `INSERT ... RETURNING <alle Spalten>` direkt (RETURNING sieht die eingefügte Zeile inkl. Defaults). Regressionstest `internal/integration_test/mgmt_review_create_real_test.go` (testcontainers). Gefunden über einen Happy-Path-Smoke mit echten Bodies + Cookie-Jar (Double-Submit-CSRF — ohne zurückgesendeten `csrf_token`-Cookie maskiert `403` jeden dahinterliegenden Fehler).
- **34 von 80 GET-`{id}`-Routen lieferten `500` statt `400` bei einer syntaktisch ungültigen UUID im Pfad** — ein Nicht-UUID-Segment (`/vaktcomply/controls/not-a-uuid/measures`) erreicht eine Query, die es nach `::uuid` castet, Postgres wirft SQLSTATE `22P02`, und jeder Handler, der nur `ErrNoRows`/Not-found mappt, fällt auf `500` durch. **Nicht über die UI erreichbar** (das Frontend sendet immer echte UUIDs aus List-Responses — der Browser-Sweep sah 0×500; alle `:id`-Position-Variablen wurden als UUIDs verifiziert), aber ein gebastelter URL trifft es. Nur eine Probe mit einem *ungültigen* Segment findet die Klasse — ein Sweep mit wohlgeformten-aber-nicht-existenten Dummy-UUIDs trifft den Not-found-Pfad (`404`) und übersieht sie komplett. Fix: `ValidateUUIDParams`-Middleware (siehe „Added") an den 6 Modul-Groups → Re-Probe `74× 400, 0× 500`. `isNotFound` bleibt unverändert (22P02 ist Client-Input, nicht Not-found — hielte sonst den `badparam_test`-Invariant nicht).
- **Sprint 122: Asynq-Enqueue schlug auf der Standard-Compose (`--requirepass` Redis) still fehl (NOAUTH)** — vier Enqueue-Clients (`cmd/api/routes.go`) und zwei Admin-Queue-Inspektoren (`admin/jobs_handler.go`, `admin/health_handler.go`) wurden mit `RedisClientOpt{Addr}` **ohne `Password`** gebaut. Auf jeder auth-geschützten Redis (also dem ausgelieferten Default) scheiterte damit jede asynchrone Einreihung — Breach-Bridge, DSR-Jobs, Scan-Trigger, Kampagnen —, während der Handler weiter `201` gab und die Queue-Gauge flach blieb. Fix: `Password` an allen sechs Clients durchgereicht (Muster wie der Metrics-Client aus S121). **Neuer Sichtbarkeits-Mechanismus:** In-Process-Counter `vakt_asynq_enqueue_errors_total{queue}` auf dem `/metrics`-Endpoint (in-process, weil ein Enqueue-Fehler oft „Redis nicht erreichbar" heißt und daher nicht *in* Redis protokolliert werden kann), an ~20 Enqueue-Fehlerpfaden über alle Module instrumentiert — ein Zabbix-Trigger `>0` fängt die Klasse künftig sofort.

### Added

- **KPI-Dashboard-PDF-Export und Management-Review-PDF-Export echt implementiert** (waren `501 Not Implemented`-Stubs mit clientseitigem „PDF-Export demnächst verfügbar"-Toast) — beide Seiten hatten einen sichtbaren Export-Button, der nur einen Toast zeigte statt einer Datei. Jetzt echte, audit-fertige PDFs via `fpdf` (`internal/modules/vaktcomply/pdf_reports.go`, Muster aus `pdf.go`/`bsi/pdf.go`): `GET /vaktcomply/kpi-dashboard/export-pdf` rendert die 12 ISMS-Kennzahlen des aktuellen Snapshots (fehlende Werte als „n/a"), `GET /vaktcomply/management-reviews/:id/export-pdf` rendert eine ISO-27001-Kap.-9.3-Managementbewertung mit allen Eingabe-/Ergebnis-Abschnitten. FE-Buttons auf echten Blob-Download umgestellt (`KPIDashboardPage.tsx`, `ManagementReviewsPage.tsx`), OpenAPI von `501` auf `200 application/pdf` korrigiert + `generated.ts` nachgezogen. Happy-Path gegen echte DB verifiziert (gültige `%PDF-`-Bytes). Die übrigen 3 `501`-Stubs (isms-scope-PDF, bsi-modeling-PDF/XLSX, pentest-upload/link) haben keinen FE-Aufrufer → bewusst deferred, kein user-sichtbarer Bug.
- **`ValidateUUIDParams`-Middleware** (`internal/shared/middleware/uuid_param.go`) — validiert UUID-typisierte Pfad-Params (`id`/`cid`/`fid`/`eid`) an den 6 Business-Modul-Route-Groups vorab und liefert `400` für ein syntaktisch ungültiges Segment, bevor es eine Query erreicht. Behebt eine systemische `500`-Klasse (siehe „Fixed"). Bewusst nicht-UUID-Params (`:name`, `:control_ref`, `:type`, `:token`, `:slug`) heißen anders und bleiben unberührt; Unit-Test deckt Reject und Pass-Through ab.

## [0.42.33] — 2026-07-10

### Fixed

- **GitHub-Release-Notes waren bei jedem Release seit dem initialen Monorepo-Merge leer** — `release.yml`s „Create GitHub Release"-Schritt setzte sowohl `body:` (Docker-Pull/Verify-Anleitung) als auch `body_path: .github/RELEASE_TEMPLATE.md`; `softprops/action-gh-release` lässt `body_path`, wenn gesetzt, `body` vollständig ersetzen statt zu ergänzen — und `RELEASE_TEMPLATE.md` bestand seit Sprint S45-7 nur aus unausgefüllten Kommentar-Platzhaltern (`<!-- Neue Features in dieser Version -->` etc.), die nie an CHANGELOG.md angebunden wurden, obwohl genau das die ursprüngliche Akzeptanzbedingung war. Aufgefallen beim Nachprüfen von v0.42.32s Release-Seite auf Nachfrage „ist alles dokumentiert" — v0.42.31 hatte exakt denselben leeren Text. Fix: `body_path` entfernt, totes `RELEASE_TEMPLATE.md` gelöscht; `generate_release_notes: true` hängt jetzt GitHubs automatische Commit-Liste unter den bestehenden `body`-Text. v0.42.32s bereits veröffentlichte leere Release-Notes nachträglich per `gh release edit` befüllt.

## [0.42.32] — 2026-07-10

### Fixed

- **Fünf vom Nutzer per Klick-Test gemeldete Bugs untersucht, gefixt und mit echtem Playwright/Chromium gegen den lokalen Stack verifiziert (nicht nur curl/Code-Review):**
  - **BSI-Cockpit crashte mit „Cannot read properties of undefined"** — `bsi/models.go`s `BSICockpit`/`HeatmapCell`/`BSIGapEntry` nutzten deutsche JSON-Feldnamen (`gesamt_fortschritt_pct`, `fortschritt_pct`, `betroffene_zielobjekte`), während `openapi.yaml` und der Frontend-Typ bereits übereinstimmend `overall_pct`/`pct`/`affected_objects` erwarteten — `cockpit.overall_pct` war `undefined`, `pct.toFixed()` warf. Fix: JSON-Tags an den bereits bestehenden Contract angeglichen; `getTopGaps` füllte zudem `anforderung_title` nie und verwarf den bereits berechneten `affected`-Zähler — beides nachgezogen (Join auf `ck_controls` wie in `GetBSIGapReport`).
  - **Löschfristen-Seite (Vakt Privacy) crashte mit „Cannot read properties of null"** — `ListDeletionReminders`/`ListRetentionTemplates` gaben `var reminders []T` zurück, das Go bei leerem Ergebnis als JSON `null` serialisiert; der Frontend-Destructuring-Default `= []` fängt nur `undefined`, nicht `null` ab, `reminders.filter(...)` warf. Fix: beide Repository-Funktionen geben jetzt explizit `[]T{}` statt `nil` zurück (etabliertes Muster im Codebase, siehe `vaktprivacy/service.go`s `ListDPIAs`-Kommentar „Always returns a non-nil slice").
  - **Audit-Log verweigerte Zugriff trotz Admin-Login** — `AuditLogPage.tsx` prüfte `user?.roles?.includes('admin') || includes('owner')` (kleingeschrieben; „owner" ist zudem keine existierende Rolle — die App kennt nur `Admin`/`SecurityAnalyst`/`InternalAuditor`/`AuditorReadOnly`/`Viewer`, großgeschrieben). Der Backend-Endpoint `GET /audit-log` hatte nie ein Rollen-Gate — der Bug blockierte ausnahmslos jeden Nutzer rein clientseitig. Fix: Vergleich auf `'Admin'` korrigiert; identischer Bug in `LicenseExpiryBanner.tsx` gefunden und mitgefixt.
  - **Zwei fehlende Übersetzungen** — `TargetGroupsPage.tsx` (Vakt Aware) hatte den Button-Text „New Group" hartcodiert statt über `t()`; neuer Key `vaktaware.targetGroups.newGroup` in allen 4 Locales (de/en/fr/nl). Settings-„KI"-Tab: Code generiert den i18n-Key generisch als `tab${id.charAt(0).toUpperCase()}${id.slice(1)}` → `tabAi`, alle 4 Locale-Dateien hatten aber `tabAI`/`tabAIDesc` (Akronym-Großschreibung) — Tab-Label und -Tooltip blieben leer/zeigten den rohen Key. Fix: Keys auf `tabAi`/`tabAiDesc` umbenannt, passend zum generischen Muster aller anderen Tabs.
  - **Framework-Aktivierung (BSI/DORA/EUAIACT/TISAX) — kein Code-Bug.** Live gegen den aktuellen Stack verifiziert: BSI/EUAIACT aktivieren korrekt (`201`), DORA/TISAX liefern korrekt `403` (draft, aus dem Angebot genommen). Die zugrunde liegende Route-Fix existiert bereits seit [0.42.23](#historie); vermutlich ein veralteter Container/Browser-Cache beim ursprünglichen Test.
  - Stichprobenartig geprüft (ausgelöst durch einen Fund in `frontend/e2e/fixtures.ts`, das 8 weitere Endpunkte gegen exakt dasselbe nil-slice-Muster mockt): `dpias`, `avvs`, `my-tasks`, `score-history`, `sla-dashboard`, `notifications` — alle bereits mit explizitem nil→`[]`-Guard abgesichert, keine weiteren offenen Fälle gefunden. Kein vollständiger Sweep über alle ~100 Backend-Dateien mit `var x []T`-Deklarationen — das wäre ein eigenständiges Audit jenseits der 5 gemeldeten Bugs.

## [0.42.31] — 2026-07-10

### Fixed

- **CI brach nach der OpenAPI-Ergänzung** — `frontend/src/api/generated.ts` war nicht neu generiert. Regel seither: `openapi.yaml` ändern heißt `npm run api-types` im selben Commit, sonst bricht CI erst beim nächsten Push.

## [0.42.30] — 2026-07-10

### Added

- **OpenAPI-Spec für die fünf in [0.42.29] ergänzten Endpunkte** — auf die Nachfrage, ob wirklich „alles“ dokumentiert sei, war das nicht der Fall. Neues Schema `AssignmentDetail` statt Wiederverwendung des bereits vorhandenen, aber veralteten `TrainingAssignment` (nie von einem echten Pfad referenziert, andere Feldnamen).

## [0.42.29] — 2026-07-10

### Added

- **Drei zuvor als „gefunden, bewusst nicht gebaut" dokumentierte Feature-Lücken auf explizite Anweisung vollständig implementiert:**
  - **`DELETE /vaktscan/findings/:id`** — neue sqlc-Query + Repository/Service/Handler/Route, Audit-Log-Eintrag, Dashboard-Cache-Invalidierung wie bei `UpdateFinding`. Der bereits verdrahtete Einzel-/Bulk-Löschen-Button auf `FindingsPage.tsx` funktioniert jetzt.
  - **vaktaware `DELETE /groups/:id` + `POST /groups/:id/targets`** — `DeleteTargetGroup` (cascadet auf `sr_targets`, setzt `sr_campaigns.group_id` auf NULL) und `AddTarget` (nutzt das bereits vorhandene `Repository.CreateTarget`) neu gebaut; `TargetGroupsPage.tsx`s Lösch- und Einzelziel-Hinzufügen-Buttons funktionieren jetzt. `useDeleteTargetGroup`/`useAddTarget` riefen zudem noch den alten `/target-groups`-statt-`/groups`-Pfad — mitkorrigiert.
  - **vaktaware `POST /training-modules/:id/assign` + `GET /training-modules/:id/assignments`** — Email→Target-Auflösung: sucht zuerst org-weit nach einem existierenden Target mit dieser E-Mail, legt sonst eines in einer neuen, reservierten Gruppe „Manuelle Zuweisungen" an (Targets brauchen zwingend eine Gruppe, `group_id` ist `NOT NULL`). Neue gejointe Query `ListSRAssignmentsByModule` (LEFT JOIN `sr_targets` + `sr_completions`) liefert die vom Frontend erwartete `{user_email, status, score, completed_at}`-Form — die alte `sr_assignments`-Rohzeile hatte keins dieser Felder. Standard-Fälligkeit 14 Tage (kein anderer Default existierte im Code, den man hätte spiegeln können). `useAssignments`/`useAssignModule` riefen zudem noch den alten `/training/...`-statt-`/training-modules/...`-Pfad — mitkorrigiert.
  - **Dabei gefunden: ein zweiter, unabhängiger, vorbestehender Bug** — die (bis dahin nie aufgerufene) `UpsertSRAssignment`-Query nutzte `ON CONFLICT (module_id, target_id)`, aber `sr_assignments.UNIQUE(module_id, target_id)` ist `DEFERRABLE INITIALLY DEFERRED` (Migration 009) — Postgres erlaubt `ON CONFLICT` grundsätzlich nicht gegen einen deferrable Constraint als Arbiter (`ERROR: ON CONFLICT does not support deferrable unique constraints`). Da diese Query zuvor nirgends aufgerufen wurde, ist der Fehler nie aufgefallen. Fix: explizites Find-then-Insert-or-Update (`FindSRAssignmentByTarget` → `UpdateSRAssignmentDueDate` oder `InsertSRAssignment`) statt eines einzelnen `ON CONFLICT`-Statements — keine Migration nötig, nur Query-Ebene.
  - **Alle fünf neuen Endpunkte zusätzlich mit einem echten Browser-Klick-Test verifiziert** (nicht nur API-Aufrufe) — auf Nachfrage, ob die vorherige Verifikation auch den literalen Button-Klick abdeckte (tat sie nicht). Playwright klickt Löschen-Button (inkl. 5s-Undo-Toast-Fenster von `useDeferredDelete`), "New Group"/"Add Target"/"Assign"-Dialoge aus, füllt Formulare, prüft UI-Ergebnis. Ein Fund unterwegs war ein Test-Skript-Bug, kein App-Bug: der Demo-Seed legt standardmäßig eine Zielgruppe „Alle Mitarbeiter" an, die alphabetisch vor der Test-Gruppe sortiert — ein naiver `.first()`-Selektor löschte die falsche Gruppe. Per Server-Log verifiziert (gelöschte UUID war exakt die der Test-Gruppe), Selektor auf die eigene Zeile gescopet, danach 7/7 sauber.
  - **OpenAPI-Spec für alle fünf neuen Endpunkte nachgezogen** (`openapi.yaml`) — auf direkte Nachfrage, ob wirklich „alles" dokumentiert ist, war das nicht der Fall: `DELETE /vaktscan/findings/{id}`, `DELETE /vaktaware/groups/{id}`, `POST /vaktaware/groups/{id}/targets`, `POST /vaktaware/training-modules/{id}/assign` und `GET /vaktaware/training-modules/{id}/assignments` fehlten komplett. Neues Schema `AssignmentDetail` ergänzt statt das bereits vorhandene, aber veraltete `TrainingAssignment`-Schema (nie von einem echten Pfad referenziert, andere Feldnamen: `user_id` statt `user_email`, kein `score`) wiederzuverwenden.

## [0.42.28] — 2026-07-10

### Fixed

- **Auf explizite Nachfrage ("teste alles was du testen kannst") erweiterter Check: mechanischer Frontend↔Backend-Abgleich für vaktscan/vaktvault/vakthr/shared+admin (bisher nicht in dieser Tiefe geprüft) plus Live-Sweep aller parametrisierten Detail-Seiten (`:id`-Routen) mit echten Datensatz-IDs statt nur der 104 ID-losen Routen.** Vier weitere echte Bugs gefunden und gefixt:
  - **`ControlDetailPage.tsx` crashte beim Öffnen des „Änderungsverlauf"-Tabs jeder Control-Detailseite** — `GET /vaktcomply/controls/:id/changelog` liefert serverseitig `{"changelog": [...]}` (objekt-gewrappt wie andere Endpoints in derselben `handler.go`), das Frontend erwartete ein nacktes Array. Da das Ergebnis kein `null`/`undefined` ist, griff der `changes ?? []`-Fallback nicht — `.map()` auf dem Wrapper-Objekt warf `TypeError: (o ?? []).map is not a function`, von der ErrorBoundary abgefangen. Reproduziert über einen Live-Playwright-Sweep aller parametrisierten Detailseiten mit echten Control-IDs (nicht nur die 104 ID-losen Routen der vorherigen Sweeps). Fix: Frontend entpackt jetzt `res.changelog`.
  - **`GET /vaktvault/projects/:id` (ProjectDetailPage.tsx) 404te für jedes Projekt** — `Service.GetProject`/`Repository.GetProject` waren vollständig implementiert, aber nie über einen `Handler`/`routes.go`-Eintrag erreichbar; nur `ListProjects` (ohne ID) und `DeleteProject` existierten für `/projects`. Derselbe „Handler/Datenschicht fertig, Route fehlt"-Fall wie mehrfach zuvor in diesem Abschnitt — diesmal aber nicht über einen Frontend↔Backend-Pfadabgleich gefunden (der hätte den Treffer übersehen, da beide Seiten „/projects/:id" verwenden), sondern erst durch das tatsächliche Live-Aufrufen der Detailseite mit einer echten Projekt-ID. Fix: `Handler.GetProject` ergänzt (gleiches Muster wie `SetSecret`: `errors.Is(err, pgx.ErrNoRows)` → `404`), Route registriert.
  - **Nach dem Route-Fix: `ProjectDetailPage.tsx` crashte stattdessen mit `TypeError: Cannot read properties of undefined (reading 'length')`** — `GET /projects/:project_id/health` gab serverseitig ein Array aus Pro-Secret-Health-Einträgen zurück (`[]SecretHealth`, eine Zeile pro Secret), während der Frontend-Typ `ProjectHealth` explizit als „Aggregated health summary for a single SecVault project" dokumentiert ist — ein einzelnes `{score, issues}`-Objekt. `health.issues.length` griff auf ein Array-Objekt zu, das kein `issues`-Feld hat → `undefined.length`. Kein reiner Wiring-Bug, sondern eine echte Formabweichung von der im Frontend-Typ dokumentierten Spezifikation; da nichts anderes im Code von der Array-Form abhing, wurde die Aggregation serverseitig nachgezogen statt das Frontend auf eine Pro-Secret-Liste umzubauen. Fix: `Service.GetProjectHealth` liefert jetzt `*ProjectHealth{Score, Issues}` (Score = Durchschnitt aller Secret-Scores, Issues = flache, mit Secret-Key präfixierte Liste); reine Aggregationslogik in `aggregateProjectHealth()` ausgelagert und unit-getestet (kein DB-Zugriff nötig).
  - **vaktscan „Findings als CSV exportieren"-Button 404te immer** — `exportFindingsCsv()` rief `GET /vaktscan/findings/export.csv` (literaler Pfad-Suffix), das Backend erwartet `GET /vaktscan/findings/export?format=csv` (Query-Parameter-basierte Formatwahl, `ExportFindings`-Handler). Fix: Frontend-Pfad korrigiert.
  - **Gefunden, bewusst nicht gebaut:** `useDeleteFinding` in vaktscan ruft `DELETE /vaktscan/findings/:id` — verdrahtet an einen echten Lösch-Button (Einzel- und Bulk-Löschung) auf `FindingsPage.tsx` —, aber es existiert serverseitig weder Repository- noch Service- noch Handler-Code dafür (anders als bei Assets, wo `SoftDeleteAsset` als Vorbild existiert). Echte Feature-Lücke; nicht spekulativ nachgebaut, da unklar ist, ob ein Hard-Delete oder ein Soft-Delete-Pattern analog zu Assets gewünscht ist.
  - **Gefunden, kein aktiver Bug, aber eine Falle für künftige Refactorings:** `internal/admin` und `internal/shared/usermgmt` registrieren beide `GET /admin/users` und `PATCH /admin/users/:id/role` auf demselben Pfad — Echo lässt bei exakter Pfad-Kollision die zuletzt registrierte Route gewinnen (empirisch mit einem isolierten Echo-Testfall bestätigt), hier `usermgmt`s spätere, vollständigere Implementierung (Schutz vor Entfernen des letzten Admins, Session-Revoke bei Rollen-Downgrade). `internal/admin`s ältere Variante (kein Last-Admin-Schutz, kein Session-Revoke, anderes Rollen-Datenmodell über `org_members.role_id`/`roles`-Tabelle statt `users.role`) ist dadurch tot, aber nicht entfernt — würde bei einer Umsortierung der Registrierung in `cmd/api/routes.go` stillschweigend wieder aktiv und die Sicherheitslücke zurückbringen. Nicht angefasst, da aktuell nicht symptomatisch und das Entfernen eine bewusste Entscheidung über security-relevanten Code ist.

## [0.42.27] — 2026-07-10

### Fixed

- **Vollständiger Funktions-Check auf ausdrücklichen Wunsch** ("mach einen kompletten Check ob alles so funktioniert wie es soll") — automatisierter Playwright-Sweep über alle 104 parameterlosen Frontend-Routen (Login, Frameworks aktivieren, jede Route besuchen, Konsolen-Fehler + fehlgeschlagene API-Requests protokollieren), jeder Fund einzeln gegen den tatsächlichen Frontend-Call-Site und Backend-Handler/Route-Code trianagiert. Vier echte Bugs gefunden und gefixt:
  - **`GET /vaktcomply/isms-scope` lieferte immer `500 CK_GET_ISMS_SCOPE_FAILED`** statt eines graceful `200 null` für Orgs ohne ISMS-Scope — `GetCurrentISMSScope`/`ApproveISMSScope` im Repository gaben ein unverbundenes `fmt.Errorf("isms scope not found")` zurück, das `isNotFound()` (prüft `errors.Is(err, ErrNotFound)` u.a.) niemals matchen konnte. Fix: beide Stellen wrappen jetzt `ErrNotFound` (`repository_isms_scope.go`). Live gegen den lokalen Stack verifiziert (`200`).
  - **BSI-Grundschutz Schutzbedarfsfeststellung (`ProtectionNeedsPage.tsx`) faktisch komplett unbenutzbar** — 5 von 6 Funktionen in `useProtectionNeeds.ts` riefen `/vaktcomply/protection-needs` (List/Create/Update/Finalize/Delete) statt des tatsächlich registrierten `/vaktcomply/protection-needs/assessments`; `useUpdateProtectionNeed` nutzte zusätzlich `PUT` statt des vom Backend erwarteten `PATCH`. Nur der Asset-Link-Endpoint (einzige Funktion mit korrektem Pfad) funktionierte. Kernworkflow von BSI-200-2 Phase 2 war seit Einführung nicht benutzbar — Liste blieb leer, Anlegen/Bearbeiten/Abschließen/Löschen schlugen fehl. Fix: alle Pfade + Methode korrigiert.
  - **`PolicyTemplatesPage.tsx` (Richtlinien/DPIA/AVV-Vorlagenauswahl) — `GET /vaktcomply/templates?category=X` war nie registriert**, obwohl der vollständig implementierte Handler (`ListDBPolicyTemplates`/`GetDBPolicyTemplate`, fragt `ck_policy_templates` via sqlc ab) seit Einführung im Code existierte. Reiner „Handler ohne Route"-Fall wie schon mehrfach in diesem Abschnitt. Fix: `GET /templates` + `GET /templates/:id` ergänzt (`routes.go`). Live verifiziert (`200`).
  - **vaktaware Zielgruppen- und Training-Seiten riefen einen anderen Pfadpräfix als das Backend registriert** — `useTargetGroups.ts` rief durchgängig `/vaktaware/target-groups`, das Backend registriert `/vaktaware/groups`; `useTrainingModules` rief `/vaktaware/training`, das Backend registriert `/vaktaware/training-modules`. Beide Seiten zeigten dadurch dauerhaft ihren Empty-State (keine Zielgruppen/keine Trainingsmodule sichtbar), obwohl Daten vorhanden waren. Fix: Frontend-Pfade an die tatsächlichen Backend-Routen angeglichen.
  - **Bewusst nicht (mit-)gefixt, da über reine Verdrahtung hinausgehend:** `useDeleteTargetGroup`, `useAddTarget` (einzelnes Ziel statt CSV-Import) und der komplette „Modul an E-Mail-Liste zuweisen"-Flow (`useAssignments`/`useAssignModule` in `useTraining.ts`) rufen Endpoints auf, für die es serverseitig **keinen** Handler/Service/Route gibt (nicht nur falsch benannt) — `DeleteTargetGroup` existiert nirgends, `CreateTarget` (Repository-Methode für Einzel-Ziel) hat keinen Service-/Handler-/Route-Aufrufer, `UpsertAssignment` (Repository) ebenso. Das sind fehlende Features, keine Wiring-Bugs — Fertigstellung braucht eine Produktentscheidung (u.a. wie E-Mails ohne existierendes Ziel aufgelöst werden), daher hier bewusst nicht implementiert statt spekulativ gebaut.

## [0.42.26] — 2026-07-10

### Fixed

- **Vollständiger, sauberer Frontend↔Backend-Routenabgleich (statt Stichprobe) fand vier weitere echte Lücken:**
  - **Dashboard-Widget „Quick Wins" hat noch nie etwas angezeigt** — `DashboardWidgets.tsx` rief `GET /vaktcomply/controls?status=missing&limit=20` auf; es gab nur den framework-gebundenen `GET /frameworks/:id/controls`, keinen org-weiten Endpunkt über alle aktivierten Frameworks hinweg. 404 wurde von React Query verschluckt, das Widget rendert bei leeren Daten einfach `null` — komplett unsichtbarer Ausfall, kein Fehler irgendwo sichtbar. Neuer Handler `ListControlsAcrossFrameworks` (wiederverwendet das etablierte „ListFrameworks + ListControls pro Framework"-Muster aus dem Auditor-Export), Route `GET /vaktcomply/controls`. Live verifiziert (`200`, korrekt gefiltert nach `status=missing`).
  - **`VersionBanner.tsx` rief einen nie existierenden `/version/check`-Endpoint auf** — kein Backend-Bug: eine bereits fertige, korrekt verdrahtete „Update verfügbar"-Anzeige (`useUpdateCheck()` → `/system/update`) stand direkt daneben in `Layout.tsx`. Toter, redundanter Code — gelöscht statt einen zweiten Endpoint zu bauen.
  - **`vakthr/access-concepts/:id/snapshot` (POST) existierte serverseitig nie** — Tippfehler/Namensdrift: der Handler ist unter `/versions` registriert (dieselbe URL wie die bereits funktionierende GET-Liste, nur mit POST). Frontend-Aufruf korrigiert, kein Backend-Change nötig.
  - **`/settings/team/members` (@-Mention-Picker in Kommentaren) existierte nie** — der Frontend-Hook hat den 404 mit einem stillen `try/catch → []` abgefangen, das Mention-Feld war seit Einführung leer, ohne dass es je auffiel. Neuer, minimal-scoped Endpoint (`id`/`name`/`email`, kein Rollen-Gate nötig — Team-Namen sind für alle Org-Mitglieder sichtbar) in `internal/shared/comments`, wiederverwendet dieselbe `org_members`-JOIN-Query wie die bestehende @-Mention-Benachrichtigung. Live verifiziert.
  - **Separat gefunden, nicht gefixt:** `AdminTenantsPage.tsx` (`/admin/tenants`) ist eine vollständige Multi-Org-Verwaltungs-UI (Anlegen, Löschen, Impersonation) für das geplante Managed-Hosting-Angebot — der Backend-Handler (`CreateManagedOrg`) existiert nur als Doku-Kommentar, keine Implementierung. Absichtlich nicht gebaut: Impersonation (Access-Token-Ausstellung für fremde Orgs) ist ein eigenständiges Sicherheits- und Produktentscheidungsthema, an ein bereits in der Sprint-Planung dokumentiertes rechtliches Gate gebunden (EULA-Prüfung, AVV — siehe Sprint 104/111/118), kein Verdrahtungsfehler.

## [0.42.25] — 2026-07-08

### Fixed

- **`DSRPortalSettingsPage.tsx` (Konfiguration des DSGVO-Selbstbedienungsportals) rief eine Route auf, die nie registriert war** — Handler (`GetDSRPortalSettings`/`UpdateDSRPortalSettings`) existierte, aber `vaktprivacy/routes.go` hatte keinen `g.GET`/`g.PATCH` dafür — reiner `404`, unabhängig vom CSRF-Bug. Gefunden bei einer gezielten Durchsuche des kompletten Frontend↔Backend-Routenabgleichs (ausgelöst durch die Häufung der obigen Bugs); dieselbe Suche über alle anderen feature-gated Routen (Groß-/Kleinschreibungs-Kollision) und alle sonstigen Frontend-Aufrufe fand keine weiteren Treffer dieser Art. Fix: Route ergänzt (`GET`/`PATCH /vaktprivacy/dsr-portal-settings`, `PATCH` Admin-only), RBAC-Regressionstest ergänzt.

## [0.42.24] — 2026-07-08

### Security

- **Paywall-Bypass bei Framework-Aktivierung über Groß-/Kleinschreibung (gefunden beim Fix der Zeile direkt oben)** — `POST /frameworks/CRA/enable` ist als literale Route mit `features.Require(features.FeatureCRA)` gegated; Echos Router ist case-sensitive, jede andere Schreibweise (`cra`, `Cra`, …) trifft die literale Route nicht und landet auf der generischen `/frameworks/:name/enable`-Route — die nur die Rolle (Admin/SecurityAnalyst), nicht die Lizenz prüft. Jeder Nutzer mit Admin-/SecurityAnalyst-Rolle auf einer Community-Instanz konnte damit jedes Pro-/Enterprise-Framework (CRA, EUAIACT, BSI, TISAX, DORA, ISO42001, ISO27017, ISO27018) aktivieren, unabhängig von der Lizenz — live gegen den lokalen Stack mit einer Community-Lizenz verifiziert (`201 Created` für `cra` ohne CRA-Feature). Fix: Feature-Gate wird jetzt zusätzlich in `EnableFramework` selbst geprüft, keyed auf den case-normalisierten Namen (`frameworkFeatureGate`-Map in `handler.go`) — greift unabhängig davon, welche Route getroffen wurde. Regressionstest deckt alle drei Schreibweisen ab (`TestEnableFrameworkCasingCannotBypassFeatureGate`). Bestand vermutlich schon seit Einführung der feature-gated Routen, war aber praktisch nie ausnutzbar, solange der CSRF-Bug (oben) jede Mutation blockierte.
- **Totes, unsichereres Duplikat von `GET /admin/users` + `PATCH /admin/users/:id/role` entfernt** — `internal/admin` und `internal/shared/usermgmt` registrierten beide dieselben Pfade; Echo lässt bei exakter Kollision die zuletzt registrierte Route (`usermgmt`, mit Last-Admin-Schutz + Session-Revoke bei Rollen-Downgrade) gewinnen, sodass `internal/admin`s ältere, unsicherere Implementierung (kein Last-Admin-Schutz, kein Session-Revoke) 100 % unerreichbar, aber vollständig kompiliert war — eine stille Falle, die bei einer künftigen Umsortierung der Registrierung in `cmd/api/routes.go` unbemerkt wieder aktiv geworden wäre. Route-Registrierungen, Handler, Service-Methoden und die nur dafür genutzten Typen (`OrgMember`, `RoleUpdateInput`) entfernt; keine Verhaltensänderung, da nur der ohnehin nie erreichte Pfad verschwindet.

## [0.42.23] — 2026-07-08

### Fixed

- **Feature-gated Framework-Enable-Routen (`CRA`, `EUAIACT`, `BSI`, `TISAX`, `DORA`, `ISO42001`, `ISO27017`, `ISO27018`) schlugen immer mit `400 "framework name is required"` fehl** — die statischen Routen (`POST /frameworks/CRA/enable` etc., registriert vor dem generischen `/frameworks/:name/enable` fürs Feature-Gating) deklarieren kein `:name`-Path-Segment, wodurch `c.Param("name")` im gemeinsamen `EnableFramework`-Handler immer leer war. Betraf praktisch jedes Pro-/Enterprise-Framework — vorher durch den CSRF-Bug oben nie testbar. Fix: `enableFrameworkNamed(name)`-Wrapper setzt den Param explizit vor dem Delegieren (`routes.go`, `handler.go`). Regressionstest über alle 8 Routen (`routes_enable_test.go`), live gegen den lokalen Stack verifiziert (`201 Created` für BSI/CRA/EUAIACT/ISO42001/ISO27017/ISO27018).
- **Draft-Status-Ablehnung (TISAX, DORA) lieferte `500 "failed to enable framework"` statt eines sprechenden Fehlers** — `EnableFramework` mappte jeden Service-Fehler pauschal auf 500. Neuer Sentinel `policy.ErrFrameworkDraft`, Handler unterscheidet jetzt und liefert `403 CK_FRAMEWORK_DRAFT`. Zusätzlich: TISAX und DORA im Frontend-Framework-Katalog auf `draft: true` gesetzt (wie `prEN18286`) — beide sind laut [0.42.20](#04220--2026-07-06) aus dem Angebot genommen, der Katalog-Eintrag hat das bisher nicht gespiegelt und zeigte einen aktiven „Aktivieren"-Button, der serverseitig immer abgelehnt wurde.

## [0.42.22] — 2026-07-08

### Fixed

- **`apiFetch` verwarf den `X-CSRF-Token`-Header auf jeder Mutation, die eigene `headers` übergibt (echte Ursache von `403 CSRF_HEADER_MISSING`)** — `fetch()` wurde mit `{ headers: {...}, ...options }` aufgerufen; da `...options` NACH `headers` gespreadet wird und jeder Mutation-Hook (`useEnableFramework`, `useSwitchDORAVariant`, …) selbst `headers: { 'Content-Type': 'application/json' }` mitgibt, überschrieb dieses `options.headers` das sorgfältig gebaute Headers-Objekt komplett — `X-CSRF-Token` und `X-Vakt-Session-Id` verschwanden spurlos, unabhängig davon, ob der Cookie lesbar war. Deterministischer, umgebungsunabhängiger Bug — reproduzierbar in jedem Browser, jedem Gerät, jedem Netzwerk, mit isoliertem Node-Testfall bestätigt. Fix: `...options` wird jetzt VOR `headers` gespreadet, sodass das konstruierte Headers-Objekt gewinnt (`client.ts`). Regressionstest ergänzt (`client.test.ts`: "survives a caller-supplied options.headers on POST"), End-to-End mit echtem Chromium gegen den vollständigen lokalen Stack verifiziert (Login → Enable → `201 Created`).

## [0.42.21] — 2026-07-07

### Added

- **RSS-Feed für den Blog (`sites/vakt`)** — `/rss.xml` über `@astrojs/rss`, mit `<link rel="alternate">` in `Layout.astro` für Feed-Reader-Autodiscovery. Artikel-Metadaten (Titel, Beschreibung, Tags, Datum) aus `blog/index.astro` nach `src/data/blog-posts.ts` ausgelagert — eine Quelle für Übersichtsseite und Feed statt zweier gepflegter Listen.

### Fixed

- **CSRF-Token zusätzlich im Response-Body** (`/auth/login`, `/auth/refresh`, `/auth/oidc/callback`, `/auth/saml/callback`, `/demo/login`, `/auth/me` → `csrf_token`-Feld) — Defense-in-Depth, falls ein Reverse Proxy/CDN vor einer Instanz den `csrf_token`-Cookie doch einmal für JS unlesbar macht; das Frontend cached den Wert in-memory (`client.ts` `setCsrfToken`) als Fallback zu `document.cookie`. War nicht die Ursache des oben beschriebenen Bugs, bleibt aber als zusätzliche Absicherung bestehen.

## [0.42.20] — 2026-07-06

### Removed

- **DORA + TISAX aus dem Angebot genommen** — beide Frameworks werden nicht mehr angeboten. Aus allen kundenseitigen Docs entfernt (README-Headline, Wiki-Framework-Listen, comply-Modul-Doc inkl. DORA-Meldepflichten-/TISAX-Ansichten-Abschnitte, ai-features, trust-center, api-reference, encryption-at-rest, UPGRADE) und im Code auf `draft`-Status gegatet (`plugins.go` `builtinAvailable`) — der `EnableFramework`-Guard lehnt draft-Frameworks ab (in der Community-Edition zusätzlich durch das Pro-Lizenz-Gate → `402`). Ein CI-Guard in `build-public-mirror.sh` failt Build + Sync, falls DORA/TISAX wieder in eine gemirrorte `*.md` geraten. Handler/Migrationen bleiben latent im Source (kein Rückbau, damit bestehende Aktivierungen nicht brechen). Runtime-verifiziert: DORA-Enable → 402, NIS2-Enable → 200.

### Security

- **EU AI Act Art. 50(2) — maschinenlesbare KI-Kennzeichnung auf Streams** — die SSE-Endpoints (`ai/chat/stream`, `controls/:id/explain`) senden jetzt den Header `X-AI-Generated: true`, konsistent zum bestehenden `"ai_generated": true`-Flag der JSON-Antworten. Damit ist jeder KI-Output maschinenlesbar als künstlich erzeugt markiert (UI-Kennzeichnung via `AIDisclaimer` bestand bereits). Rechts-Einordnung (AI Act + CRA) intern dokumentiert (`docs/legal/ai-act-cra-einordnung.md`); CRA-Art.-14-Meldeprozess ins Incident-Runbook aufgenommen.
- **Polar-Webhook-Signatur auf Standard-Webhooks-Spec korrigiert (kritisch)** — die Signaturprüfung verifizierte simples `HMAC-SHA256(body)` als Hex mit `v1=`-Prefix; Polar signiert aber nach [Standard Webhooks](https://www.standardwebhooks.com): `HMAC-SHA256("{webhook-id}.{webhook-timestamp}.{body}")`, base64, Prefix `v1,`, Secret als rohe UTF-8-Bytes. Auf vier Punkten inkompatibel — jeder echte Polar-Webhook wäre mit 401 abgewiesen worden (kein License-Key ausgestellt, obwohl gezahlt). Zusätzlich Replay-Schutz per ±5-min-Timestamp-Freshness-Check (Dedup bleibt zweite Linie).
- **API-Key-Verwaltung RBAC-gegated (S120-4)** — `POST/DELETE /api-keys` und Key-Rotation erfordern jetzt die Rolle Admin oder SecurityAnalyst. Zusätzlich erbt jeder API-Key höchstens die aktuelle Rolle seines Ausstellers (kein pauschales `SecurityAnalyst`-Grant mehr für Personal-Keys) — ein Viewer-/Auditor-Konto kann sich über einen API-Key keine Schreibrechte mehr verschaffen; Downgrade/Offboarding des Ausstellers wirkt auf den Key durch.
- **form-handler: Slowloris-/Memory-Flood-Schutz (S120-3)** — `http.Server` mit Read-/ReadHeader-/Write-/Idle-Timeouts und 64-KiB-`MaxBytesReader` auf beiden POST-Endpunkten. `realIP()` akzeptiert `X-Real-IP`/`X-Forwarded-For` nur noch vom vertrauenswürdigen Proxy (Caddy setzt `X-Real-IP` explizit) — der Pro-IP-Rate-Limiter kollabiert hinter dem Proxy nicht mehr zu einem globalen Limiter.
- **IP-Rate-Limits hinter Reverse-Proxy korrekt (S120-5)** — das Root-Compose setzt `VAKT_TRUSTED_PROXIES` (Default `172.16.0.0/12`), sodass Login-Lockouts, IP-Rate-Limits und die Admin-IP-Allowlist die echte Client-IP sehen statt der nginx-Container-IP. Dokumentiert in `docs/wiki/configuration.md`.
- **DNS-Rebinding-TOCTOU geschlossen (S120-12)** — CCM-HTTP-Checks und Outgoing Webhooks validieren die Ziel-IP jetzt beim Dial (resolve+validate+dial in einem Schritt, wie SAML-Metadata) statt nur per Pre-Flight-Lookup. App-Container laufen mit `cap_drop: ALL` + `no-new-privileges`.
- **CI-Supply-Chain (S120-10)** — alle GitHub-Actions in sämtlichen 8 verbleibenden Workflows (inkl. `release.yml`, Kunden-Image-Build + Signing) sind auf Commit-SHAs gepinnt; Backup-GPG-Roundtrip und der End-to-End-Restore-Drill laufen als Gate vor jedem Release-Build.
- **form-handler: Header-Injection-Schutz** — CRLF-Zeichen (`\r`, `\n`) in Name, E-Mail und Betreff werden jetzt abgelehnt (400 Bad Request). IP-Ermittlung verwendet `RemoteAddr` statt `X-Forwarded-For` (XFF-Spoofing-Schutz). E-Mail-Validierung via `mail.ParseAddress` (RFC 5322).
- **AI Goal Sanitizing** — `AgentRunRequest.Goal` wird auf 2000 Zeichen begrenzt; ANSI-Escape-Codes und Steuerzeichen werden via `logsafe.SanitizeField` entfernt bevor der Prompt an das LLM weitergegeben wird.

### Fixed

- **`docker compose up` für Self-Hoster überhaupt erst lauffähig (Launch-Blocker, kritisch)** — zwei unabhängige Fehler machten eine frische Installation aus dem öffentlichen Repo unmöglich: (1) Die Kunden-Images lagen **privat** unter `ghcr.io/matharnica/*` — jeder anonyme Pull (also jeder Self-Hoster) schlug mit `denied` fehl. Images nach `ghcr.io/norvik-ops/*` verschoben und public gestellt; Compose/Helm ziehen von dort. (2) Der `nginx`-Frontdoor startete **nie**: das Frontend-Release-Image ist self-serving (eigener Webserver + `/api`-Proxy), erfüllte aber die `service_completed_successfully`-Bedingung des separaten `nginx`-Service nie → Host-Port 80 blieb tot. Ersetzt durch einen Caddy-Frontdoor (siehe „Changed"). End-to-end verifiziert (anonymer Pull → Boot → `/health`/Login/Setup).
- **Polar-Trial-Abos stellten keinen License-Key aus** — der Webhook reagierte nur auf `status == "active"`; ein Abo im 30-Tage-Trial (`status "trialing"`) lief ins Leere (kein Key, keine Mail). Jetzt wird bei `trialing` ein auf die Trial-Laufzeit begrenzter Key ausgestellt (30 Tage + 15 Tage Puffer für die manuelle Aktivierung), der volle Interval-Key folgt bei Umwandlung in ein bezahltes Abo. Eigene Trial-Bestätigungsmail.

- **Frische Installationen: pgBouncer-Image + Auth** — der Docker-Hub-Tag `edoburu/pgbouncer:1.22.1` wurde upstream entfernt, `docker compose up` schlug bei Neuinstallationen mit einem Pull-Fehler fehl → Pin auf `1.22.1-p0` (gleiche pgBouncer-Version, neu gebautes Image). Zusätzlich `AUTH_TYPE: scram-sha-256` gesetzt: das neue Image default't auf md5, Postgres 16 authentifiziert mit SCRAM — ohne den Fix startete die API mit „DB unavailable — all routes disabled". Bestandsinstallationen mit lokal gecachtem altem Image sind nicht betroffen, übernehmen den Fix aber beim nächsten `docker compose pull`.
- **Art. 17 DSGVO Erasure — sr_campaign_enrollments** — Löschung von `sr_campaign_enrollments` (Aware-Kampagnen) war nicht in `ExecuteErasure()` enthalten. `employee_id` ist TEXT ohne FK-Cascade auf `hr_employees`, daher musste die Löschung explizit ergänzt werden. Evidence-Note wird um `sr_campaign_enrollments deleted: N` erweitert.
- **Impressum §5 DDG** — Vollständiger Name „Stefan Moseler" in beiden Sites (`sites/vakt/`, `sites/main/`) ergänzt. Steuernummer-Abschnitt als Pflichtangabe vorbereitet (⚠️ Steuernummer muss manuell eingetragen werden).
- **Broken Navigation — „Lizenz aktivieren" im Multi-Framework-Wizard** — Schaltfläche verlinkte auf `/settings/license` (nicht existent). Link korrigiert auf `/settings`. Zusätzlich: `/settings/license` im Router als Redirect → `/settings` eingetragen, damit direkte URL-Eingabe nicht zu 404 führt.
- **Broken Navigation — Verknüpfter Datenschutzvorfall in Incident-Detailseite** — Link „DSGVO-Vorfall öffnen" verlinkte auf `/vaktprivacy/breaches/:id` (keine Detail-Route). Korrigiert auf `/vaktprivacy/breach` (Vorfalls-Übersicht).

### Changed

- **Frontdoor: Caddy statt nginx, automatisches HTTPS** — der App-Stack nutzt jetzt `caddy:2-alpine` als Reverse-Proxy. `VAKT_DOMAIN` auf die öffentliche Domain setzen → Caddy holt und erneuert das Let's-Encrypt-Zertifikat vollautomatisch (Ports 80+443), routet `/api`+`/health`→api (SSE-tauglich via `flush_interval -1`) und alles andere→frontend. Eigene nicht-interne `edge`-Netzebene nur für Caddy (ACME-Egress); api/db bleiben `internal` (ISO 27001 A.8.22 unangetastet). Entfernt: `docker-compose.tls.yml`, `nginx/`, `scripts/gen-local-cert.sh`; TLS-Doku auf „`VAKT_DOMAIN` setzen" vereinfacht.
- **KI-Berater standardmäßig aktiv (lokales Ollama)** — `VAKT_AI_PROVIDER` default't auf `ollama`, der Ollama-Container läuft ohne Compose-Profil mit (kein `COMPOSE_PROFILES=ai` mehr nötig). Deaktivieren via `VAKT_AI_PROVIDER=disabled`; ohne KI läuft die Plattform in 2 GB RAM. Worker-Memory-Limit 256m→768m mit Scan-Semaphore (`VAKT_SCAN_CONCURRENCY`, Default 2) gegen Scanner-OOM; Redis `--maxmemory 400mb` mit 512m-cgroup-Limit (Eviction greift vor dem Kernel-OOM-Kill).
- **Findings-Export: echte Keyset-Pagination (S120-9)** — der Export nutzt jetzt `ListFindingsCursor` statt eines OFFSET-Loops (war O(n²) bei großen Exports); Integrationstest über 1203 Findings.
- **AI-Agent als Beta gekennzeichnet (S120-8)** — `AIAgentPage` zeigt KI-Disclaimer + Beta-Badge (EU-AI-Act-Transparenz), reflektiert `X-Vakt-Status: experimental` und ist vollständig übersetzt (de/en/fr/nl), ebenso die SecVitals-KPIs und Export-Buttons (S120-11).
- **setup.md repariert + Mirror vollständig (S120-7)** — Schnellstart erzeugt jetzt gültige Secrets (Secret-Key, Postgres-/Redis-Passwort), `VAKT_REDIS_URL` wird im Compose aus `REDIS_PASSWORD` abgeleitet, und `docs/operations/` (13 Runbooks) wird in den Public Mirror gesynct; `check-docs.py` prüft Mirror-Links ab jetzt automatisch.
- **Code-Hygiene: Refactoring** — `vaktcomply/repository.go` (war 2333 Zeilen, 120 Funktionen) wurde in 9 Domain-Dateien aufgeteilt (`repository_milestones.go`, `repository_access_review.go`, `repository_interested_parties.go`, `repository_isms_scope.go`, `repository_tasks_comments.go`, `repository_resilience.go`, `repository_capa.go`, `repository_reporting.go`, `repository_incidents.go`). `admin/handler.go` (war 1376 Zeilen) wurde in `handler_org.go`, `handler_sso.go` und `handler_settings.go` aufgeteilt. Kein Behavior-Change.
- **Findings-Export: alle Seiten** — `ExportFindings` (CSV/JSON) und `ExportFindingsXLSX` paginieren jetzt alle Findings in 500er-Batches statt bei 500/25 abzuschneiden. Orgs mit > 500 Findings bekommen vollständige Exports.
- **AI-Report-Timeout konfigurierbar** — `VAKT_AI_REPORT_TIMEOUT` (Sekunden, Standard 120) steuert den HTTP-Timeout für KI-Report-Generierung. Nützlich bei langsamen CPU-only-Modellen auf kleinen VMs.
- **Backup: db.pgdump GPG-verschlüsselt** — `backup.sh` verschlüsselt den PostgreSQL-Dump nach der Erstellung symmetrisch mit GPG (AES256). Klartext-Dump wird nach Verschlüsselung gelöscht. `backup-verify.sh` entschlüsselt automatisch wenn `VAKT_BACKUP_PASSPHRASE`/`VAKT_BACKUP_PASSPHRASE_FILE` gesetzt. gpg (GnuPG) ist jetzt Pflicht-Dependency für `backup.sh`.
- **VAKT_AI_PROVIDER Default → `disabled`** — KI-Berater ist standardmäßig deaktiviert. Vorher war `openai` der Default, was bei Instanzen ohne `.env`-Konfiguration ungewollt Verbindungsversuche zu OpenAI ausgelöst hat. Aktivierung explizit via `VAKT_AI_PROVIDER=openai` + `VAKT_AI_BASE_URL`.
- **Redis maxmemory** — Redis-Container startet jetzt mit `--maxmemory 512mb --maxmemory-policy allkeys-lru`. Verhindert OOM-Kills auf kleinen VMs; älteste Cache-Keys werden bei Speicherdruck verdrängt.
- **Public Mirror: docs/guides/ enthalten** — `docs/guides/` (Getting-Started-Guides, Tutorials) wird jetzt in den Public Mirror gespiegelt. `docs/modules/` (veraltete Modulbeschreibungen) wird nicht mehr gespiegelt — `docs/wiki/` ist die kanonische Quelle.
- **README: First-Login-Hinweis** — Quick-Start-Sektion erklärt den ersten Login ohne Demo-Modus (`/setup`).

---

**Auth & User Provisioning — Sprint 105.**

### Added

- **Direktes User-Anlegen ohne SMTP (S105-1, CE)** — `POST /api/v1/admin/users` legt Nutzer direkt mit E-Mail, Passwort (min. 10 Zeichen) und Rolle an — kein SMTP erforderlich. Nutzer ist sofort aktiv. Settings → Team: zweite Schaltfläche „Direkt anlegen" neben „Einladen".
- **OIDC/Casdoor-Konfiguration in der Settings-UI (S105-2, Pro)** — OIDC-Verbindungsdaten (Provider-URL, Client-ID, Client-Secret) können in der Settings-UI gespeichert werden, ohne Container-Neustart. Migration 227: Tabelle `org_oidc_configs`. Client-Secret wird AES-256-GCM-verschlüsselt. `/health`-Endpoint liest `sso_enabled` jetzt zur Laufzeit aus der DB; Env-Vars (`CASDOOR_URL` etc.) bleiben als Fallback aktiv.
- **SAML JIT-Provisioning (S105-3, Pro)** — Nutzer werden bei erfolgreichem SAML-Login automatisch angelegt, wenn sie noch nicht in Vakt existieren. Toggle in Settings → Zugang → SAML 2.0 (Standard: an). Migration 228: Spalte `jit_provisioning` in `org_saml_configs`.
- **SAML Metadaten-Import via URL (S105-3, Pro)** — IdP-Metadaten können per URL importiert werden statt XML manuell einzufügen. Schaltfläche „URL laden" in den SAML-Settings.
- **SAML CE → Pro (S105-4, Pro)** — SAML 2.0 SP ist jetzt ein Pro-Feature. SAML-Settings-Sektion zeigt CE-Nutzern einen Upgrade-Prompt.

### Fixed

- **SAML-ACS-Fehlermeldungen (S105-3)** — SAML-Fehler beim Login gaben bisher generische 500er zurück. Jetzt: Browser-Redirect auf `/login?error=saml_*` mit i18n-Fehlermeldung (de/en/fr/nl). Fehlercodes: `saml_assertion_invalid`, `saml_missing_email`, `saml_user_not_provisioned`, `saml_provision_failed`.

---

**Security — SSRF DNS-Rebinding-Fix (S105-3 SAML Metadata-Fetch).**

### Security

- **SSRF DNS-Rebinding TOCTOU geschlossen (`internal/admin/saml_metadata.go`)** — Pre-flight-DNS-Check für den SAML-Metadaten-URL-Import wurde durch einen custom `DialContext` im HTTP-Transport ersetzt. Resolve, Validate (`isPublicIP`) und Dial passieren jetzt atomar in einem Schritt; eine zwischen Validation und `client.Do` geänderte DNS-Antwort (DNS-Rebinding) wird damit unmöglich. Redirects nutzen denselben Transport — keine zweite Validate-Lücke. Maximale Redirect-Tiefe auf 3 begrenzt. Verhalten für legitime IdP-URLs identisch.

---

**Code-Architektur — vaktcomply Phase 2 (Sprint 102).**
Rein internes Refactoring, keine User-Facing-Änderungen.

### Internal

- **`vaktcomply/bsi/` Sub-Package (S102-1)** — BSI-Grundschutz-Domäne aus dem God-Module extrahiert: 27 Typen, BSI-Check-Workflow, Strukturanalyse, Abhängigkeitsgraph, Risikobewertung (BSI 200-3), Referenzberichte A1–A6, KompendiumScorer, DER.4-Crossmappings. Root-Handlers delegieren via `service.BSI.*`.
- **`vaktcomply/audit/` Sub-Package (S102-2)** — Interne Audits, Audit-Programm, Management-Review und Approval-Workflow extrahiert. Root-Handlers delegieren via `service.Audit.*`.
- Evidence-Extraktion (S102-3) auf S103 verschoben — 18 Aufrufer im Root-Package erfordern Interface-Injection-Design vor der Extraktion.

---

**Bugfix — SAML-Direct-Private-Key-Speicherung (Migration 226).**

### Fixed

- **`org_saml_configs.key_pem` TEXT → BYTEA (Migration 226)** — Die Spalte war als `TEXT` deklariert, der gesamte SAML-Code (`auth/saml_direct.go`, `cmd/rotate-key`) speichert dort aber rohe AES-GCM-Ciphertext-Bytes. Roher Ciphertext enthält Nicht-UTF8-Bytes → Postgres lehnte den Insert ab (SQLSTATE 22021); SAML-Direct-Private-Key-Speicherung war für echte Keys defekt (latent, da Enterprise-Feature). Schema-only-Fix, kein Code-Change (Code nutzt bereits `[]byte`). Aufgedeckt durch die re-aktivierte Key-Rotation-E2E.
- **CI-Härtung** — gofmt-Drift (Sprints 99/100), OpenAPI-Type-Drift (`generated.ts` regeneriert), pprof-semgrep (dedizierter Mux statt DefaultServeMux), und der S99-4-Key-Rotation-Gate-False-Positive (traf legitime `testing.Short()`/Docker-Guards) behoben.

---

**Dokumentations-Audit-Remediation — Sprint 93.**
Schließt die Doku-Korrektheitsfehler für Self-Hoster und ergänzt aufgabenorientierte ISMS-Guides.

### Added

- **ISMS-Workflow-Guides (S93-5)** — 4 aufgabenorientierte Schritt-für-Schritt-Guides unter `docs/guides/`: Schutzbedarfsfeststellung, Vom Risiko zur Maßnahme, Internes Audit vorbereiten, NIS2-Vorfall melden. Mit realen Vakt-UI-Pfaden, verlinkt aus getting-started + wiki/README.
- **`/.well-known/security.txt` (S93-7)** — RFC-9116-Sicherheitskontakt unter `frontend/public/.well-known/`.
- **Operations-Index (S93-9)** — `docs/operations/README.md` als Einstieg in alle Betriebs-Runbooks mit klarer Hierarchie zu Installation/Konfig/DR.

### Fixed

- **Migrations-Doku-Widerspruch (S93-3)** — `faq.md` implizierte fälschlich, Migrationen bräuchten `AUTO_MIGRATE=true`. Jetzt konsistent mit `installation.md`: der `migrate`-Container ist der Prod-Default, `AUTO_MIGRATE` ist dev-only.
- **Falsche Paseto-Version (S93-14)** — `api-reference.md` nannte „Paseto v2", der Code nutzt v4 (`V4SymmetricKey`). Korrigiert auf v4 (SECURITY.md war bereits korrekt).
- **Sprint-Status-Drift (S93-13)** — `docs/sprints/overview.md` Sprints 91/93/94/98 auf ✅ nachgezogen.

---

**Performance- & Skalierungs-Härtung — Sprint 98.**
Initial-Bundle entlastet (Recharts + Routen lazy), Slowloris-Härtung, opt-in pprof, SSE-Push statt DB-Polling, AI-Stream-Deadline.

### Changed

- **Frontend-Initial-Bundle −129 KiB gzip (S98-1/S98-2)** — Recharts (`ForecastChart` im Dashboard) und alle Modul-Route-Pages laden jetzt via `React.lazy` + `<Suspense>`; zusätzlich wurde **framer-motion vollständig durch leichte CSS-Keyframes ersetzt** (`PageTransition`/`EmptyState`/`SkeletonLoaders`/`SlideOver`, `prefers-reduced-motion`-aware) und aus den Dependencies entfernt. `vendor-charts` (106 KiB) und `vendor-motion` (41 KiB) sind nicht mehr im Initial-Pfad; Initial-Paint-JS sank von **452 → 323 KiB gzip** (< 330-KiB-Ziel), Entry-Chunk 192 KiB.
- **Notifications-SSE: Push statt 2-s-Poll (S98-5)** — Der Notification-Stream nutzt jetzt Redis Pub/Sub (`notify.SetPublisher`) mit 30-s-Safety-Poll-Fallback statt eines 2-s-DB-Polls pro offenem Tab. DB-Grundlast ist damit O(Events) statt O(Nutzer). Fällt Redis aus, fällt der Stream automatisch auf den alten Poll zurück. Migration 225 (Deckindex `idx_user_notifications_org_cursor`).
- **DB-Pool-Default 25 → 15 (PERF-M01)** — `VAKT_DB_MAX_CONNS` default gesenkt mit Kommentar zum pgBouncer-Zusammenspiel; verhindert Connection-Sättigung bei mehreren Instanzen.

### Added

- **HTTP-Server-Timeouts (S98-3)** — `ReadHeaderTimeout` (5 s), `ReadTimeout` (15 s), `IdleTimeout` (120 s) am API-Server gegen Slowloris; `WriteTimeout=0` damit SSE-Streams nicht gekappt werden.
- **pprof opt-in (S98-4)** — `VAKT_PPROF_ENABLED=true` startet einen Go-pprof-Server auf `127.0.0.1:6060` (nur localhost). Anleitung in `docs/operations/runbook.md#pprof`.
- **AI-Stream-Deadline (PERF-M03)** — Der AI-Streaming-Client erzwingt jetzt eine 90-s-Context-Deadline, damit ein hängender Provider keine Goroutine dauerhaft blockiert.
- **k6-ISMS-Last-Test (S98-11)** — `loadtest/vakt-isms-load.js` (Ramp 10→50→100 VU, p95-Gates) für private Staging-Instanzen + optionaler `workflow_dispatch`-Job.
- `docs/operations/scaling.md` — Skalierungs-/Sizing-Doku (Statelessness-Checkliste, Sizing-Tabelle, SSE-Push als Multi-Instance-Voraussetzung).
- ADR-0065 — Recharts-Bundle-Strategie (lazy statt Lib-Wechsel).

---

**UX & Onboarding-Polish — Sprint 94.**
Durchgängige „Sie"-Form in allen deutschen Texten, i18n-Drift-Guard in CI, Risikobewertungs-Methodik-Dialog, i18n-Styleguide.

### Changed

- **Sie-Form durchgängig (S94-3)** — 30 Du-Form-Strings in `de.json` auf „Sie" umgestellt (account settings, notifications, trust center, scanner hints, error messages, AI tooltips, app tour). Kein `du`/`dein` mehr in deutschen UI-Texten.
- **i18n-Drift-Guard in CI (S94-4)** — `scripts/check-i18n-drift.py` prüft fehlende Keys in en/fr/nl vs. de, Du-Formen in de.json, und warnt bei hardkodierten Umlauten in JSX. Eingebunden in `.github/workflows/docs.yml` (blockiert bei Fehler, warnt bei hardkodierten Strings).
- **Risiko-Methodik-Dialog (S94-5)** — RisksPage hat jetzt einen „Methodik"-Button (HelpCircle) der die 5×5-ISO-27005-Matrix-Legende öffnet: Wahrscheinlichkeits- und Auswirkungsskala 1–5, Score-Kategorien Niedrig/Mittel/Hoch/Kritisch mit Farbkodierung. Alle 4 Sprachen (de/en/fr/nl).

### Added

- `docs/dev/i18n-style-guide.md` — Styleguide für i18n-Konventionen (Sie-Form, Key-Struktur, Plural, Drift-Guard-Befehl).

---

**Compliance-Lücken & Datenqualität — Sprint 100 (Phase-A-Audit-Nachgang).**
Schließt DSGVO-Löschpipeline-Lücke, NIS2-KPI-Blindstellen, Docker-Netzwerksegmentierung und legt erste Benchmarks für kritische Pfade an.

### Security

- **NIS2 Art. 21f — KPIs aus Echtdaten befüllt** — `kpi_calculator.go` gibt für `FindingSLACompliance` (% Findings innerhalb SLA aus `vb_findings`) und `OpenMajorNCs` (offene Major-NCs aus `ck_capas` ISO 27001 Cl. 10.1) jetzt echte DB-Werte zurück. `SuppliersOverduePct` und `PhishingClickRate` sind bewusst `nil` mit `TODO(data-source)`-Kommentar (Datenquellen Q4 2026). 4 neue Unit-Tests (S100-1, COMP-C01).
- **DSGVO Art. 17 — Erasure-Pipeline vervollständigt** — `ExecuteErasure()` löscht jetzt `sr_events` (IP-Adressen + User-Agents aus Phishing-Simulationen) **vor** `sr_targets`, damit der FK-Constraint nicht dazwischenfunkt. Evidenz-Notiz dokumentiert alle 4 betroffenen Tabellen. Unit-Tests prüfen Reihenfolge, Evidenz-Note und Idempotenz (S100-2, COMP-M01).
- **EU AI Act Art. 52 Disclaimer auf alle AI-Ausgabekanäle ausgeweitet** — E-Mail-Digests (`emaildigest/service.go`) und Policy-Draft-Exports (`handler_policies.go`) tragen jetzt denselben Art.-52-Transparenzhinweis wie die `AIReportPage` (S100-3, COMP-M02).

### Infrastructure

- **Docker-Netzwerksegmentierung (ISO A.8.22)** — `docker-compose.yml` verwendet jetzt zwei interne Netzwerke: `db-net` (nur Postgres + pgBouncer) und `app-net` (API, Worker, Redis, Nginx, Ollama). API und Worker joinen beide Netze; Nginx und Redis erreichen Postgres damit **nicht** direkt (S100-4, COMP-M03).
- **Legacy Evidence Upload deprecated** — `POST /controls/:id/evidence/upload` setzt `Deprecation: true`-Header und verweist auf `POST /controls/:id/evidence-files` (EvidenceFileService). `file_path` (Server-Pfad) fehlt in allen Evidence-API-Responses (`json:"-"`). Entfernung im nächsten Minor (S100-5, ARCH-M03).
- **Migration-Lücke 203–209 dokumentiert** — [ADR-0064](docs/adr/0064-migration-gap-203-209.md) erklärt Ursache (verworfene Sprint-84/85-Branches), bestätigt dass golang-migrate Lücken toleriert und legt fest, dass künftige Migrationen ab 225 lückenlos vergeben werden (S100-6, OPS-H01).

### Added

- **Benchmarks für kritische Pfade** — `crypto_bench_test.go`: 8 Benchmarks für AES-256-GCM Encrypt/Decrypt/EncryptWithAAD/DecryptWithAAD (1 KB, Größenreihe 64 B–64 KB) und HKDF-Derivation. `kpi_calculator_bench_test.go`: 3 Benchmarks für den nil-DB-Pfad und `numericToFloat64Ptr`. Dazu 4 tabellengetriebene Property-Invariant-Tests (Round-Trip für 10 Payload-Größen, AAD-Varianten, Wrong-AAD-rejection, Legacy-Backward-Compatibility). Upgrade auf `pgregory.net/rapid` als Follow-up geplant (S100-7, ARCH-L01).

---

**Security-Härtung III — Sprint 99 (Phase-A- und v3-Audit-Nachgang).**
Schließt alle neuen CRITICAL/HIGH-Security-Findings, die S87/S90 nicht abgedeckt haben: zwei CRITICALs (SMTP-Credential-Leak, E-Mail-Header-Injection) und zwei HIGHs (SSRF-Scanner-Bypass, Key-Rotation-CI-Gate).

### Security

- **E-Mail-Header-Injection verhindert (CWE-93)** — `fromName`, `fromEmail` und `subject` in `vaktaware/service.go` werden jetzt durch `sanitizeHeader()` von CR/LF-Zeichen bereinigt, bevor sie in MIME-Header eingebaut werden. Verhindert SMTP-Header-Injection durch Kampagnen-Absendernamen (S99-2).
- **SSRF-Bypass in Scanner durch Hostname-Auflösung geschlossen** — `isPrivateOrLoopback()` prüfte bisher nur IP-Adressen; Hostnamen (z. B. `internal.corp`) wurden nie aufgelöst und passierten den Check. Jetzt werden Hostnamen via `net.LookupHost` aufgelöst und **alle** resultierenden IPs gegen private CIDR-Ranges geprüft. DNS-zu-Private-IP-Redirect-Angriffe werden damit abgeblockt (S99-3, CWE-918).
- **Key-Rotation-Integrationstest reaktiviert** — `rotate_key_real_test.go` war seit Migration 152 mit `t.Skip()` auskommentiert und gab kein Signal mehr über gebrochene Rotation. Test aktualisiert, `t.Skip()` entfernt; CI blockiert jetzt, wenn diese Datei wieder einen Skip enthält (S99-4).
- **Alle CI-Actions auf SHA-Commit-Hashes gepinnt** — mutable Tags wie `@master` und `@v4` wurden durch verifizierte SHA-Pins ersetzt; `aquasecurity/trivy-action`, `github/codeql-action` und alle anderen Actions sind damit Supply-Chain-sicher (S99-5, SEC-H01/H02).
- **SAML-Parsing: XML-Entity-Expansion begrenzt** — `saml_direct.go` nutzt jetzt einen Decoder mit `Entity`-Limit, um Billion-Laughs-Angriffe auf den SAML-Response-Parser abzuwehren (S99-7, SEC-M03).
- **Rate-Limiter fail-closed bei Redis-Ausfall** — öffentliche Endpunkte (Login, Demo-Start) fallen bei Redis-Ausfall jetzt auf `503 Service Unavailable` zurück statt alle Requests durchzulassen. Interne Endpunkte (bereits authentifiziert) bleiben fail-open (S99-8, SEC-M08).
- **`RequireRole`-Kommentar korrigiert** — der Middleware-Kommentar beschrieb eine Rollenhierarchie, die Implementierung prüft exakt eine Rolle. Kommentar klargestellt; parametrisierte Rollenkombinations-Tests hinzugefügt (S99-12, ARCH-L02).
- **System-Worker-Actor-ID für Audit-Log** — Hintergrund-Jobs schreiben jetzt `actor_id = "system/worker"` in den Audit-Log statt einen leeren String zu hinterlassen; zentrales `SystemWorkerActorID`-Constant in `shared/audit` (S99-11, SEC-M01).

### Docs / CI

- **Dependency-License-Gate (go-licenses + license-checker)** — neuer CI-Job prüft bei jedem Push, ob alle Go- und npm-Abhängigkeiten unter erlaubten OSS-Lizenzen stehen (MIT, Apache-2.0, BSD-*). AGPL und GPL-only-Lizenzen schlagen den Build fehl; Ausnahmen in `.license-exceptions.json`. `NOTICE`-Datei automatisch aus `go-licenses csv` aktualisiert (S91-9).
- **Paseto v4 in OpenAPI und Security-Doku korrigiert** — zwei Stellen in `openapi.yaml` (API-Beschreibung + `RefreshResponse`) und `SECURITY.md` nannten „Paseto v2"; tatsächlich wird Paseto v4 (local) verwendet. Beide Stellen korrigiert (S93-14).

---

**Dokumentations-Audit-Remediation — Sprint 93 (P0-Blocker).**
Beseitigt drei datenverlust- oder erststartblockierende Fehler in der Self-Hoster-Dokumentation.

### Fixed (Docs)

- **Backup-Doku: `uploads_data`-Volume ergänzt** — `docs/operations/backup-restore.md` und `scripts/backup.sh`/`restore.sh` sicherten bisher nur die Postgres-Datenbank. Das `uploads_data`-Volume (Nachweis-Dateianhänge) wurde komplett übersehen. Backup-Script erzeugt jetzt `uploads.tar.gz` via `docker run alpine tar`; Restore-Script stellt das Volume wieder her; FAQ und Backup-Doku korrigiert (S93-1).
- **Phantom-Installer-Link entfernt** — `docs/guides/getting-started.md` verwies auf `curl -sSL https://get.vakt.app | sh`; die Domain löst nicht auf. Abschnitt entfernt; einzig dokumentierter Installationspfad ist `git clone` + `docker compose up` (S93-2).
- **`AUTO_MIGRATE`-Widerspruch aufgelöst** — README, `docs/wiki/installation.md` und FAQ beschrieben `AUTO_MIGRATE` inkonsistent. Klargestellt: der `migrate`-Container läuft bei `docker compose up` automatisch; `AUTO_MIGRATE=true` ist ausschließlich ein Dev-Convenience-Flag und kein empfohlener Produktionspfad (S93-3).
- **Modul-Dokumentation auf aktuelle Namen umbenannt** — `docs/modules/` enthielt noch die Pre-Rebrand-Dateinamen (`secvitals.md`, `secpulse.md`, `secvault.md`, `secreflex.md`, `secprivacy.md`). Alle Dateien auf `vaktcomply.md`, `vaktscan.md`, `vaktvault.md`, `vaktaware.md`, `vaktprivacy.md` umbenannt; `docs/modules/index.md` entsprechend aktualisiert (S93-4).
- **`check-docs.py` erkennt jetzt Volume-Backup-Drift** — neuer `check_volume_backup()`-Check stellt sicher, dass jedes Docker-Named-Volume (das kein ephemeres Artefakt ist) in `docs/operations/backup-restore.md` erwähnt wird; meldet außerdem veraltete `./data/uploads`-Pfade in benutzersichtbaren Docs (S93-11).

---

**UX- & Onboarding-Polish — Sprint 94 (P0-Beta-Blocker).**
Behebt die drei ersten Fehler, die ein neuer Nutzer nach dem Login sieht: hartkodiertes Deutsch, zwei konkurrierende Onboarding-Systeme und fehlende Passwort-Mindestlängen-Konsistenz.

### Fixed

- **Dashboard vollständig i18n** — `WidgetGrid.tsx` enthielt ~30 hartkodierte deutsche Strings (Sektionsüberschriften, KPI-Labels, Modul-Beschreibungen, Badge-Texte, Fehler-Banner, Einstellungslinks). Alle durch `t()`-Aufrufe ersetzt; Übersetzungskeys in allen 4 Locales (de/en/fr/nl) ergänzt (S94-1).
- **Onboarding-Doppelsystem konsolidiert** — `OnboardingWizard`/`OnboardingBanner` (veraltetes Komponent) und `GettingStartedChecklist` entfernt; einziger Onboarding-Einstieg ist jetzt die `OnboardingChecklist` (S89-5, 7 datenabgeleitete Schritte). `data-tour="getting-started"`-Attribut auf das konsolidierte Komponent übertragen, damit AppTour weiterhin funktioniert (S94-2).
- **Passwort-Mindestlänge überall 10 Zeichen** — `Setup.tsx` und `InviteAcceptPage.tsx` prüften bisher auf `>= 8`; auf `>= 10` (Backend-Minimum) angeglichen (S94-7).

---

### Fixed

- **Worker: SQLSTATE 23503 FK-Race bei Demo-Cleanup** — alle 10 Batch-Cron-Handler (`score_snapshot`, `risk_trend`, `kpi_snapshot`, `bsi_kpi`, `evidence_staleness`, `backup_freshness_check`, `bcm_evidence_sync`, alle 6 `secvitals`-Handler, `epss_enrich`, `cert_scan`, `sla_check`, `github_ci_sync`) schlossen zuvor ephemere Demo-Orgs (`slug LIKE 'demo-%'`) nicht aus. Der stündliche Demo-Cleanup löscht die Org hart aus `organizations`; ein parallel laufender Batch-Job, der kurz zuvor die Org-ID gelesen hat, schreibt dann in eine Tabelle mit FK-Constraint auf `organizations(id)` → SQLSTATE 23503. Zabbix-Alert ausgelöst 2026-06-17 01:02 CEST. Fix: neues `cmd/worker/shared.go` mit `nonDemoOrgIDs()`/`nonDemoOrgs()` (WHERE slug NOT LIKE 'demo-%'), einheitlich in allen betroffenen Handlern angewendet. Demo-Orgs benötigen keine persistente KPI-/Snapshot-/Evidence-History und werden ohnehin nach 4 h gelöscht.
- **Schema-Drift-Test jetzt vollständig** — `TestWorkerRawSQLAgainstSchema` prüfte bisher nur `handlers_*.go`; nach dem Hinzufügen von `shared.go` und `handlers_shared.go` wurden deren Raw-SQL-Queries nicht validiert. Test-Glob auf alle `*.go` (ohne `_test.go`) erweitert.

---

**Code-Review-Hardening (Sprint 90).**
Schließt die Härtungs-/Skalierungs-/Wartbarkeits-Findings des Architektur-Reviews — keine CRITICAL/HIGH, Codebasis als „ungewöhnlich reif" bewertet. Krypto-Kontextbindung, Read-Only-API-Keys, Permission-Cache, Repository-Refactor, Multi-Replica-Doku und ein End-to-End-Middleware-Test.

### Added

- **Read-Only-API-Keys (`:ro`-Scope)** — ein API-Key kann jetzt rein lesend ausgestellt werden (Scope-Suffix `:ro`, z. B. `vaktcomply:ro`). Solche Keys erhalten die Rolle `Viewer` und werden auf jeder schreibenden HTTP-Methode (POST/PUT/PATCH/DELETE) mit `403 AUTH_READONLY_KEY` abgewiesen — ideal für Dashboards, Monitoring oder Auditor-Export-Jobs. „Nur-Lesen"-Checkbox im API-Key-Dialog (i18n 4 Sprachen), Scope-Syntax in der [API-Referenz](docs/wiki/api-reference.md) dokumentiert (S90-5).

### Security

- **AES-256-GCM mit Associated Data (Kontextbindung)** — gespeicherte Vault-Secrets werden jetzt an ihren Kontext (`org_id` + `secret_id`) gebunden (`EncryptWithAAD`/`DecryptWithAAD`, `enc:v2:`-Format-Marker). Ein gültiger Ciphertext kann nicht mehr unbemerkt zwischen Zeilen oder Organisationen umkopiert werden (Confused-Deputy-/Ciphertext-Reuse-Schutz, CWE-345). Vollständig abwärtskompatibel: bestehende marker-lose Werte bleiben lesbar und werden beim nächsten Schreibzugriff lazy auf `enc:v2:` migriert ([ADR-0059](docs/adr/0059-aes-gcm-associated-data.md), S90-3).
- **Modul-Permission-Check mit Fail-Closed-Redis-Cache** — die Per-Request-Berechtigungsprüfung wird 45 s in Redis gecacht (weniger DB-Last), invalidiert sofort bei Permission-Änderungen und bleibt fail-closed: ein DB-Fehler liefert `503` statt Zugriff, eine Redis-Störung degradiert nur auf den ungecachten DB-Pfad (S90-4).

### Changed

- **`client_errors` aus `main.go` in ein Repository ausgelagert** — das Frontend-Error-Logging nutzt jetzt das `clienterrors`-Paket (Repository + Handler) statt Inline-Raw-SQL im API-Entrypoint; Sanitisierung in `shared/logsafe` zentralisiert (S90-2).
- **Schlüsselableitungs-Doku ↔ Code-Widerspruch aufgelöst** — der irreführende Kommentar in `main.go` (vermeintliche „geplante Re-Encryption-Migration") wurde durch die tatsächliche, verifizierte Realität ersetzt ([ADR-0058](docs/adr/0058-key-derivation-raw-vs-derived.md), S90-1).
- **Mehrere kleine Polish-Punkte als bewusste Entscheidung kommentiert** — synchrone Login-Writes, doppelte `/health`-Registrierung (No-DB-Fallback + Upgrade) und der 401-Hard-Redirect im Frontend (vollständiger State-Reset) sind jetzt im Code begründet, kein Verhaltens-Change (S90-8).

### Docs / CI

- **DB-Pool-Sizing & Multi-Replica (PgBouncer)** — Doku in [Configuration](docs/wiki/configuration.md) + Helm-Werte: `VAKT_DB_MAX_CONNS × Replicas` muss unter Postgres `max_connections` bleiben; ab 2 Replicas PgBouncer (Transaction-Mode) (S90-6).
- **MegaLinter `GO_REVIVE` deaktiviert** — lief im falschen Workspace (Falsch-Negative + reines Style-Rauschen); Go-Linting ist autoritativ am CI-`golangci-lint`-Job delegiert (S90-7).
- **End-to-End-Middleware-Ketten-Test** — neuer Integrationstest (Testcontainer Postgres + Redis) durchläuft den vollständigen `protected`-Stack (Auth → CSRF → MFA → License → Rate-Limit → Module-Permission) mit 5 Szenarien inkl. Fail-Closed-`503`; Integration-Job-Timeout 300 s → 600 s (S90-9).

---

**Marktreife-Auflagen — Private-Beta-Readiness.**
Schließt die Beta-Launch-Auflagen: vollständiger DSGVO-Export, gehärteter Restore-Pfad, transparenter Beta-Status, automatisierte Backups, geführtes ISB-Onboarding und Word/DOCX-Export.

### Added

- **DSGVO-Org-Export um HR- und Awareness-PII vervollständigt** — der Daten-Export (Art. 20) enthält jetzt das Mitarbeiterverzeichnis (`hr_employees`, Checklisten-Läufe, Contractors, Mover-Events) sowie das Awareness-Zielverzeichnis (`sr_targets`). Phishing-/Trainings-**Ergebnisse** werden pseudonymisiert exportiert (gesalzener SHA-256, Salt verlässt nie den Prozess), damit die §87-BetrVG-Zusage „der Admin sieht nicht, wer geklickt hat" auch im Org-Takeout gewahrt bleibt. HR-/Aware-Dateien nur bei aktivem Modul. Dokumentiert in [Daten-Export](docs/wiki/data-export.md) (S89-2).
- **Geführtes ISB-Onboarding „Erste 30 Tage"** — ein 7-Schritte-Pfad auf dem Dashboard (Scope → Assets → Schutzbedarf → Framework → Risiken → Controls/Nachweise → Policy). Jeder Schritt verlinkt direkt auf die echte Funktion und zeigt „erledigt" anhand realer Org-Daten; Fortschritt + Ausblenden bleiben über Sessions erhalten. Community-Feature, baut auf der bestehenden Onboarding-Infrastruktur auf (keine Doppelung). i18n in 4 Sprachen (S89-5).
- **Automatische Backups (Scheduler + Off-Site)** — neuer `scripts/backup-cron.sh`-Wrapper (erstellen → verifizieren → optional off-site pushen → nach Retention rotieren → bei Fehlschlag benachrichtigen) plus optionaler `docker-compose.backup.yml`-Scheduler-Service. Off-Site-Push ist opt-in und zielt auf ein **kundenkonfiguriertes** Ziel (S3/rsync/SFTP), niemals auf Norvik. Konfigurierbar via `VAKT_BACKUP_SCHEDULE/DIR/RETENTION_DAYS/OFFSITE_CMD/NOTIFY_*` (S89-4).
- **Word/DOCX-Export für Auditoren** — Statement of Applicability und Risikoregister lassen sich jetzt zusätzlich zu PDF/XLSX als editierbares `.docx` exportieren. Reiner-Go-Generator (nur Standardbibliothek, kein externer Dienst, kein CGO), Pro-gated und SHA-256-Audit-Log-Eintrag beim Export (S89-6).

### Security

- **Restore-Pfad gehärtet** — `restore.sh` schreibt den entschlüsselten Master-Key nicht mehr ungeschützt nach `/tmp`: `umask 077`, `0600`-Tempdatei, `shred`-Löschung bei **jedem** Exit-Pfad, und der Schlüssel erscheint nie in stdout/Logs (auch nicht im Dry-Run). Neuer Shell-Test prüft Schlüssel-Leak + HMAC-Ablehnung manipulierter Archive; Disaster-Recovery-Runbook um Drill-Protokoll, Drill-Prozedur und das gehärtete Schlüssel-Handling ergänzt (S89-1).
- **Beta-Status & Support-Erwartungen transparent** — diskreter „Private Beta"-Hinweis in der App (verlinkt auf den [Beta-Disclaimer](docs/wiki/beta-disclaimer.md): Best-Effort-Support ohne 24/7-SLA, Backup-Verantwortung des Betreibers, Bus-Faktor-Hinweis). README + Status-Badge entsprechend aktualisiert (S89-3).

---

**Feature-Gap-Closure — Backup-Nachweis, Risk-Catalog, verinice-Import & mehr.**
Schließt die Wettbewerbs-Gaps der ISMS-Gap-Analyse rund um das Notfallmanagement (Sprint 86) und senkt die Time-to-Value für ISB und verinice-Wechsler.

### Added

- **Backup-/Restore-Nachweis (ISO 27001 A.8.13)** — leichtgewichtige Registry für Backup-Jobs und Restore-Tests (RTO-Soll/Ist), mit Staleness-Erkennung (überfälliges Backup/überfälliger Restore-Test → „at risk"/„überfällig"), täglichem Reminder-Job und automatischem Evidence-Nachweis an A.8.13/DER.4. Neue Seite „Backup-Nachweis" unter Vakt Comply (S88-2).
- **Gefährdungs-/Maßnahmen-Katalog (Risk-Catalog)** — vorbefüllte Bibliothek mit 61 generischen Gefährdungen/Szenarien (ISO/BSI/NIS2/DSGVO), filterbar nach Framework/Asset-Typ/Schutzziel. „Risiko aus Katalog erstellen" befüllt ein Risiko inkl. Maßnahmenvorschlag und Control-Verknüpfung vor — senkt die Erfassung von Tagen auf Stunden (S88-3, ADR-0061).
- **verinice-(.vna)-Import** — Migrationsbrücke für verinice-Wechsler: Upload → Dry-Run-Vorschau (Assets/Controls/Risiken/unmapped) → Bestätigen. Defensiver, XXE-sicherer, fuzz-getesteter SNCA-Parser; strukturierter Audit-Log-Eintrag pro Import. Import-Wizard unter Einstellungen (S88-4, ADR-0062).
- **Physische-Maßnahmen-Checklisten (ISO 27001 A.7.1–A.7.14)** — 14 geführte Checklisten-Templates mit DACH-typischen Prüfpunkten; „Checkliste anwenden" auf A.7-Controls erzeugt strukturierte Evidence statt Freitext (S88-5).
- **Microsoft Intune (MDM) Integration** — Pull-Collector für Geräte-Compliance (Verschlüsselung, Patch-Stand, Conformität) aus Microsoft Graph (`managedDevices`), als Endpoint-Evidence für ISO A.8.1/A.8.9 und NIS2-Cyberhygiene. Read-only, SSRF-geschützt, AES-256-GCM-verschlüsselte Credentials (S88-7).
- **Scan→Comply-Evidence-Brücke** — kritische/hohe Scanner-Findings fließen automatisch als Evidence an die Schwachstellen-/Konfigurations-Controls (A.8.8/A.8.9). Idempotent: ein Re-Scan dupliziert keine Evidence. Modul-isoliert über das Shared-Event-Interface (S88-8, ADR-0063).
- **DPIA-Trigger (Art. 35 DSGVO)** — eine Verarbeitungstätigkeit mit Hochrisiko-Indikator (besondere Kategorien Art. 9, Drittlandübermittlung, Profiling/großflächig) erzeugt automatisch einen DPIA-Entwurf mit Begründung (S88-8).
- **VVT→Control-Verknüpfung** — Verarbeitungstätigkeiten (Art. 30) lassen sich mit ISO-27001-/DSGVO-TOM-Controls verknüpfen; beidseitig sichtbar („Nachweis aus VVT" am Control). Modul-isoliert (S88-9).
- **Audit-Log Syslog/SIEM-Forwarding (opt-in)** — Audit-Ereignisse können an einen kunden-eigenen Syslog/SIEM-Server (RFC 5424 oder CEF, TCP/TLS) ausgeleitet werden. Default aus; SSRF-geschütztes Ziel; asynchron mit Drop-Zähler (Audit-Schreibpfad wird nie blockiert); Prometheus-Counter `vakt_audit_forward_{sent,dropped,failed}`. **Kunden-konfiguriert, kein Norvik-Relay, kein Phone-Home** (S88-6).

### Infrastructure

- **Migrationen 220–224** — `ck_backup_jobs`/`ck_backup_restore_tests` (S88-2), `ck_threat_library_links` (S88-3), `cloud_integrations`-Provider-Enum um `intune` (S88-7), `ck_scan_evidence_map` (S88-8), `ck_vvt_control_links` (S88-9).
- **OpenAPI-Spec** — neue Endpunkte für Backup-Nachweis, Physische-Templates, Threat-Catalog, verinice-Import, Intune und VVT-Verknüpfung; Frontend-Typen + i18n (de/en/fr/nl) durchgehend nachgezogen.

---

**Security-Hardening — residuale Härtungen aus dem AppSec-Assessment.**
Schließt die verbliebenen LOW/MEDIUM-Findings des AppSec-Assessments (2026-06-13) — keine kritischen Lücken, sondern Tiefenverteidigung für den Beta-Launch.

### Security

- **CORS Fail-Closed in Produktion** — eine Nicht-Demo-Instanz startet nicht mehr, wenn `VAKT_CORS_ORIGINS` auf `*` (alle Origins) steht, solange Session-Cookies erlaubt sind. Demo-Instanzen dürfen `*` weiterhin nutzen. Schützt vor versehentlich offener Cross-Origin-Konfiguration (S87-2).
- **Login-Timing-Oracle geschlossen** — der Login führt jetzt auch bei unbekannter E-Mail die volle bcrypt-Prüfung (gegen einen vorab berechneten Dummy-Hash) aus. Damit lässt sich aus der Antwortzeit nicht mehr ableiten, ob ein Konto existiert (S87-3, CWE-208).
- **`/health/ready` leakt keine Infrastruktur-Details mehr** — bei DB-/Redis-Ausfall liefert der unauthentifizierte Readiness-Endpunkt generische Statusmeldungen (`database unavailable` / `redis unavailable`); der Detailfehler steht nur noch im Server-Log (S87-4).
- **`VAKT_FORCE_SECURE_COOKIES`** — neuer Schalter (default `false`), der das `Secure`-Attribut auf allen Session-/CSRF-Cookies erzwingt, unabhängig von TLS/`X-Forwarded-Proto`. Empfohlen in Produktion hinter einem TLS-terminierenden Proxy als Sicherheitsnetz gegen fehlkonfigurierte Reverse-Proxies (S87-5, CWE-614).
- **`pw_version` fail-closed bei Redis-Ausfall** — die Token-Invalidierung nach Passwortwechsel/Offboarding greift jetzt auch während eines Redis-Ausfalls: Der Versionszähler wird zusätzlich durabel in PostgreSQL gehalten und bei Redis-Ausfall von dort gelesen, statt die Prüfung zu überspringen. Veraltete Tokens bleiben dadurch abgelehnt; legitime Nutzer werden nicht ausgesperrt (S87-6, CWE-636, ADR-0060).

### Infrastructure

- **Migration 219** — neue Spalte `users.pw_version BIGINT NOT NULL DEFAULT 0` als durable Source of Truth für die Passwort-Versionierung.
- **CI-Vuln-Gates dokumentiert** — `govulncheck` und `npm audit --audit-level=high --omit=dev` failen den Build bei reachable High-Vulns. Der Runtime-Dependency-Tree ist frei von Vulnerabilities; die verbliebenen 3 High betreffen ausschließlich Build-/Dev-Tools (Vite/esbuild) und landen nie im Produktions-Image (S87-1, dokumentiert in `SECURITY_REVIEW.md`).

---

**BCM Notfallmanagement — BSI 200-4, ISO 22301, NIS2 Art. 21 c.**
Business Continuity Management vollständig in Vakt Comply integriert: Business Impact Analysis, Wiederanlaufpläne, Alarmierungsplan und BCM-Bereitschaftsscore mit PDF-Notfallhandbuch-Export.

### Added

- **Business Impact Analysis (BIA)** — Prozesse mit Schutzbedarfsklasse 1–3, RTO/RPO/MBCO-Kennzahlen, Kritikalitätsstufen (low/medium/high/critical) und Abhängigkeiten. Vollständige CRUD-API + Frontend-Seite.
- **Wiederanlaufpläne (WAP)** — Strukturierte Notfallpläne mit Aktivierungskriterien, verantwortlicher Stelle, RTO-Ziel, Status (Entwurf/Aktiv/Getestet) und Schritt-für-Schritt-Massnahmenblöcken (JSONB). Zuordnung zu BIA-Prozessen optional.
- **Alarmierungsplan (Notfallkontakte)** — Kontaktverzeichnis mit drei Eskalationsstufen, 24/7-Verfügbarkeit und Rolle. In BCMDashboard nach Eskalationsstufen gegliedert.
- **BCM-Bereitschaftsscore** — 0–100 Punkte (5 Kriterien à 20 Punkte): BIA vorhanden, Wiederanlaufpläne vorhanden, Kontakte gepflegt, kritische Prozesse als „high" klassifiziert, WAP getestet. Warnung-Banner bei Score < 60.
- **Notfallhandbuch-PDF-Export** — Sieben Sektionen (Deckblatt, Schutzzieldefinition, BIA-Übersicht, Wiederanlaufpläne, Alarmierungsplan, Test-Nachweise, BSI-Mapping), SHA-256-Hash in `ck_bsi_report_exports` gespeichert. Pro-Feature-Gate (`audit_pdf`).
- **DER.4-BSI-Baustein** — 11 Anforderungen A1–A11 vollständig abgedeckt; 12 Cross-Mappings zu ISO 27001:2022 (A.5.29, A.5.30, A.8.13, A.8.14), NIS2 Art. 21 (c) und DORA Art. 11.
- **BCM-Asynq-Job** — `comply:bcm_evidence_sync` läuft täglich 07:00 UTC und schreibt BIA-Prozesse als Evidence in `ck_evidence`.
- **Demo-Seed** — 3 BIA-Prozesse (IT-Infrastruktur/E-Mail-System/ERP), 1 Wiederanlaufplan mit 5 Schritten, 3 Notfallkontakte auf Eskalationsstufen 1–3.
- **BCM-Dashboard** — Übersichtsseite mit Score-Gauge, KPI-Kacheln und Schnelllinks zu BIA, WAP und Alarmierungsplan.
- **i18n** — ~75 neue Keys in DE/EN/FR/NL für alle BCM-Seiten und Navigationspunkte.

### Infrastructure

- **Migrationen 216–218** — `ck_bia_processes`, `ck_recovery_plans` (JSONB Steps), `ck_emergency_contacts`.
- **OpenAPI-Spec** — 13 neue Endpunkte (`/vaktcomply/bia/processes`, `/vaktcomply/bcm/recovery-plans`, `/vaktcomply/bcm/emergency-contacts`, `/vaktcomply/bcm/readiness-score`, `/vaktcomply/bcm/report.pdf`) mit vollständigen Schemas und `required`-Listen.

---

**Identity & Access Automation — Entra ID, Keycloak, LDAP/Active Directory.**
Drei neue Evidence-Collector für Identity-Provider und Verzeichnisdienste — automatisch, lokal, kein Datenabfluss.

### Added

- **Microsoft Entra ID / Graph API-Integration** — MFA-Enrollment-Quote, Conditional-Access-Policies, Risky Users (Identitätsrisiko), Admin-Rollenmitglieder und inaktive Accounts täglich als Compliance-Evidence. OAuth2 Client Credentials (client_id/client_secret), AES-256-GCM verschlüsselt. `@odata.nextLink`-Pagination für große Tenants.
- **Keycloak REST-Integration** — MFA-Status pro User (OTP/TOTP), Passwort-Policy-Stärke (length()-Extraktion), inaktive Accounts, Admin-Rollenmitglieder und Session-Timeout-Compliance täglich als Evidence. Service Account Client Credentials. Warnung bei Passwortlänge <8 oder SSO-Session >12 Stunden.
- **LDAP / Active Directory-Integration** — Inaktive Accounts (>90 Tage nicht eingeloggt), Accounts mit „Passwort läuft nie ab", Mitglieder privilegierter Gruppen (Domain Admins, Administrators), deaktivierte Accounts und aktive Account-Gesamtzahl als Evidence. Unterstützt AD (userAccountControl-Flags, Windows FILETIME) und OpenLDAP (shadowLastChange, shadowMax). LDAPS (TLS) unterstützt.
- **Support-Diagnose-Bundle** — `make support-bundle` (bzw. `scripts/support-bundle.sh`) sammelt Versionsinfos, Container-Status, Health und die Logs aller Services in ein `vakt-support-<datum>.tar.gz` für Support-Tickets. Optionen `TAIL=` (Zeilen/Service) und `SINCE=` (Zeitfenster); erkennt `docker compose` v2 und v1. Kein Datenabfluss — schreibt nur lokal, Logs sind PII-redigiert. Neue Wiki-Seite [Support & Diagnose](docs/wiki/support.md) mit Hinweis zu `VAKT_LOG_LEVEL=debug`.

### Infrastructure

- **Log-Rotation als Default** — `docker-compose.yml` setzt für alle langlebigen Services (`api`, `worker`, `nginx`, `postgres`, `pgbouncer`, `redis`, `ollama`) den `json-file`-Logdriver mit `max-size: 10m` / `max-file: 5` (max. ~50 MB pro Service). Verhindert volllaufende Disks und stellt sicher, dass aktuelle Logs für ein Support-Bundle vorhanden sind. Kein manueller Eingriff nötig.

- **Migration 169** — `cloud_integrations.provider` CHECK-Constraint erweitert um `ldap`.
- **OpenAPI-Spec** — 15 neue Endpunkte für die drei neuen Identity-Provider (config GET/PUT, sync POST, status GET, evidence GET) sowie zugehörige Component-Schemas.
- **Doku-Konsistenz-Guard** — `scripts/check-docs.py` (+ `Docs`-Workflow `.github/workflows/docs.yml`, GitHub-hosted Runner) prüft bei jedem Doku-/`go.mod`/`config.go`-Push: Go-Version in den Stack-Docs == `backend/go.mod`, AI-Default-Modell == `config.go`, keine kaputten internen `.md`-Links, und die Env-Var-Coverage in drei Invarianten: (A) jede `.env.example`-Variable steht in `docs/wiki/configuration.md`; (B) jede in `backend/**` gelesene Env-Var ist in *irgendeiner* echten Referenz-Doku oder `.env.example` dokumentiert; (C) jede `import.meta.env.VITE_*`-Lesestelle in `frontend/src` ebenso; (D) jede `${VAR}`-Referenz in `docker-compose*.yml` ebenso; (E) jede in `helm/` deklarierte `VAKT_*`-Var wird auch vom Backend gelesen (fängt tote Config). Implementierte dabei das bis dahin ignorierte `VAKT_LOG_LEVEL` (zerolog-Global-Level) und deckte 2 weitere undokumentierte Vars auf (`VAKT_OLD_/NEW_SECRET_KEY`, via `mustEnv` gelesen). Deckte insgesamt 11 bis dahin undokumentierte Config-Vars auf (EPSS, AI-Cost/Cache/Limits/Fail-Open, License-Refresh, Metrics, Sentry, SLO-Targets, Admin-IP-Allowlist, Worker-Concurrency) — jetzt alle dokumentiert. Quelle der Wahrheit ist immer der Code.
- **Doku-Drift behoben** — Go-Version 1.22 → 1.26 (README-Badge, `docs/wiki/README.md`, `docs/architecture.md`, `docs/security/pentest-rfp.md`, `backend/internal/integration_test/README.md`), `operator/go.mod` `go 1.22.0` → `1.26.0`; README/Wiki auf vollständige Framework-Liste (14) + Modul-Basis/Pro-Aufteilung; redundante `docs/public/README.md` entfernt (Root-`README.md` ist die via `sync-public-repo.yml` gespiegelte Public-README).

### Removed

- **`VAKT_SENTRY_DSN` (dead config)** — Config-Feld und env-Read entfernt. Das Struct-Feld `SentryDSN` wurde zwar seit Sprint 12 gesetzt, aber nie ausgelesen — kein Sentry-SDK in `go.mod`, null Effekt. Wer Sentry-Integration benötigt, kann das als eigenständige Middleware hinzufügen.

### Changed

- **`VAKT_SCAN_ALLOW_PRIVATE` jetzt in der Operator-Referenz** (`docs/wiki/configuration.md`) — war nur in der internen Security-Assessment-Doku erwähnt, fehlte im kanonischen Wiki.

- **Lizenz-Keys haben jetzt ein Ablaufdatum** — Monatsabo 35 Tage, Jahresabo 395 Tage. Bei Kündigung läuft der Key am nächsten Renewal-Datum automatisch aus; die Instanz fällt dann auf Community zurück.
- **License Auto-Renewal** — Mit `VAKT_LICENSE_TOKEN` (aus der Kauf-E-Mail) holt sich die Instanz den aktuellen Key täglich selbst — kein manueller Eingriff bei Verlängerungen nötig. Opt-in; ohne Token läuft alles wie bisher. Einzige ausgehende Verbindung: `api.norvikops.de` (nur Lizenzdaten, keine Geschäftsdaten). Sichtbar in Einstellungen → Lizenz als „Auto-Renewal aktiv"-Badge.
- **AI-Default-Modell `qwen2.5:3b` → `qwen2.5:7b`** — bessere DE-Compliance-Qualität. Durchgezogen über `config.go`, `docker-compose.yml` (ollama-init Pull + Ollama-RAM-Limit 6→8 GB), `.env.example` und alle Docs. **Mindest-RAM für den lokalen KI-Berater steigt dadurch von ~4 GB auf 8 GB** (Modell ~4.5 GB). Auf VMs mit < 8 GB RAM weiter `qwen2.5:3b` nutzen: `VAKT_AI_MODEL=qwen2.5:3b`. ADR-0024 aktualisiert.

---

## [0.40.0] — 2026-06-09

**DACH-Integrations-Welle — Hetzner, IONOS, Wazuh, GitHub GHAS, Prometheus.**
Fünf neue Evidence-Collector für DACH-typische Infrastruktur — alle Pull-basiert, alle lokal in der Kunden-Infrastruktur, kein Datenabfluss an externe SaaS.

### Added

- **Hetzner Cloud-Integration** — Server-Inventar, Firewall-Regeln, SSH-Keys und Snapshot-Nachweis täglich als Compliance-Evidence. Warnung wenn ein Server seit >7 Tagen kein Snapshot hat. API-Token read-only, AES-256-GCM verschlüsselt. Standort-Filter optional (nbg1, fsn1, hel1, …).
- **IONOS Cloud-Integration** — Server-Inventar, SSH-Keys und Snapshot-Compliance aus IONOS Cloud API v6. Unterstützt Basic Auth (Benutzername/Passwort) oder API-Token. Warnung bei fehlendem Snapshot in den letzten 7 Tagen.
- **Wazuh Pull-Integration** — Vulnerability-Scans (CVE), SCA-Compliance-Scores und FIM-Events täglich aus dem Wazuh-Manager (REST-API v4, JWT). Warnung bei offline-Agents >24h oder kritischen CVEs. TLS-Verifizierung deaktivierbar für on-prem-Deployments mit selbstsignierten Zertifikaten.
- **GitHub GHAS-Integration** — Dependabot Alerts, Secret Scanning Alerts und Code Scanning Alerts (high+critical) werden bei jedem GitHub-Sync automatisch als Compliance-Evidence erfasst. GHAS nicht aktiviert → stiller Skip (kein Fehler). Deduplication über `auto_source_ref`, schreibt in `ck_evidence` (kein Cross-Modul-Import).
- **Prometheus / Alertmanager-Integration** — Uptime-Metriken (PromQL `avg_over_time(up[24h])`), Scrape-Target-Health und aktive Alerts (critical) täglich als Monitoring-Evidence. Alertmanager-URL optional. Bearer Token optional, AES-256-GCM verschlüsselt.

### Infrastructure

- **Migration 168** — `cloud_integrations.provider` CHECK-Constraint erweitert um `hetzner`, `ionos`, `wazuh`, `prometheus`, `entra_id`, `keycloak`, `gitlab`, `sonarqube`.
- **OpenAPI-Spec** — 20 neue Endpunkte für die vier neuen Cloud-Provider (config GET/PUT, sync POST, status GET, evidence GET) sowie zugehörige Component-Schemas.

---

## [0.38.0] — 2026-06-09

**ISB-Vollständigkeit — Notfallhandbuch (BCP), Schutzbedarfsfeststellung, Berechtigungskonzept.**
Drei neue Feature-Bereiche runden die ISB-Checkliste ab. Alle drei sind vollständig versioniert und erzeugen audit-fähige Nachweise in Vakt Comply.

### Added

- **Notfallhandbuch / BCP** (`Vakt Comply`) — Verwaltung von Business-Continuity-Plänen mit Status-Workflow (draft → active → archived), versionierten Plänen und zugeordneten Wiederanlauftests. Jeder Test dokumentiert Datum, Typ (tabletop / walkthrough / fulltest) und Ergebnis (passed / failed / partial). Pläne ohne Test in den letzten 12 Monaten werden mit einem Amber-Banner hervorgehoben. Pläne können direkt als Compliance-Nachweis in Vakt Comply verlinkt werden.
- **Schutzbedarfsfeststellung** (`Vakt Comply`) — CIA-Triade-Bewertung (Vertraulichkeit, Integrität, Verfügbarkeit) nach BSI-Maximumprinzip. Schutzklassen: `normal`, `hoch`, `sehr_hoch`. Gesamtbedarf wird automatisch als Maximum der drei Dimensionen berechnet. Einträge können finalisiert (eingefroren) werden — danach keine Änderungen mehr möglich. Unterstützte Objekttypen: Prozess, System, Information, Standort.
- **Berechtigungskonzept** (`Vakt HR`) — Verwaltung von Berechtigungskonzepten mit Rollenmatrix pro Konzept. Zugriffsrollen dokumentieren System, Zugriffsebene (`read / write / admin / no_access`), Begründung und Wiederprüfungsintervall. Konzepte können per „Version einfrieren" als unveränderlicher Schnappschuss gesichert werden; die Versionshistorie ist vollständig einsehbar.

### Infrastructure

- **`promote.yml` mit automatischem Deploy** — Der promote-Workflow kopiert Images jetzt auf `:latest` **und** `:demo` (Server nutzt `APP_VERSION=demo`) und fährt danach den Demo-Server direkt auf dem Self-Hosted Runner hoch (`docker compose pull` → migrate → worker → api → health-check → frontend). Kein manueller SSH-Schritt mehr nötig.

---

## [0.37.0] — 2026-05-29

**Mega-Audit-Welle — VPS-Hardening, Code-Security-Fixes, CI-Hygiene.** Zweites Agent-Audit (2026-05-29) mit 5 VPS-Findings + 7 Code-Findings + 3 Hardening-Items. Alle Wave A/B/C-Items adressiert; CI durchgehend grün (Backend, Frontend, Integration, Deploy, E2E).

> **Operative Hinweise:** `VAKT_SECRET_KEY` auf dem VPS rotiert — bestehende verschlüsselte DB-Einträge bleiben lesbar (HKDF-Migration ist idempotent; `cmd/rotate-key` war in 0.36.0 abgesichert). UFW aktiv auf dem VPS; Zabbix-Agent (Port 10050) und -Proxy (Port 10051) sind in den Allow-Rules explizit gesichert. `VAKT_PROMOTE_SECRET` aus der systemd-Unit in `/etc/vakt-promote.env` (chmod 600) ausgelagert.

### Security

- **VPS Secret-Key rotiert** — neuer kryptografisch zufälliger 32-Byte-Key; `docker compose up -d` propagiert den neuen Key ohne Downtime.
- **Firewall aktiviert (UFW)** — Default deny-incoming, explizite Allows für SSH (22), HTTP/S (80/443), Zabbix-Agent (10050 von dirserver), Zabbix-Proxy (10051 von dirserver), Prometheus-Scrape.
- **VAKT_PROMOTE_SECRET rotiert + gehärtet** — Secret aus systemd-Unit-inline in `EnvironmentFile=/etc/vakt-promote.env` (chmod 600) verschoben; kein Klartext mehr in `systemctl show`.
- **`.env` Berechtigungen** — chmod 600 auf `.env`; war zuvor world-readable.
- **Schwacher-Key-Guard** (`B1`) — `config.Validate()` verwirft Keys bei denen alle Bytes identisch sind (z.B. `0000…`). Fehler enthält Regenerierungshinweis.
- **Scanner-Image-Pinning** (`B3`) — Trivy (`0.62.0`) und Nuclei (`v3.4.4`) im Dockerfile per SHA-256-Digest gepinnt; verhindert stilles Tag-Overschreiben.
- **`err.Error()`-Leaks reduziert** (`B4`) — interne Fehlermeldungen in `cloud/handler.go`, `jobs_handler.go`, `vaktscan/handler.go`, `ai/handler.go`, `nis2wizard/handler.go` durch generische Meldungen ersetzt; Stack-Details nur im strukturierten Log.
- **`html/template` für E-Mail-Templates** (`B5`) — `vaktaware/service.go` und `vaktcomply/policy_acceptance.go` nutzen jetzt `html/template` statt `text/template`; Auto-Escaping verhindert XSS in kampagnen-generierten E-Mails.
- **TRUSTED_PROXIES-Warning** (`C3`) — Startup-Log-Warn wenn `VAKT_TRUSTED_PROXIES` nicht gesetzt; verhindert stilles IP-Spoofing hinter Reverse-Proxys.
- **In-Memory-Ratelimit-Warning** (`C7`) — Startup-Log-Warn wenn Redis nicht konfiguriert und In-Memory-Fallback aktiv ist; Multi-Replica-Deployment mit gespiegelten Limits ist damit erkennbar.

### CI / Tooling

- **Trivy-Image-Scan im Deploy-Step** (`C2`) — nach `docker build` scannt Trivy das frisch gebaute API-Image auf CRITICAL/HIGH; nicht-blockierend (exit-code 0), Report im Summary.
- **Fuzz `-parallel=1`** — Go 1.22+ gibt `context deadline exceeded` zurück wenn parallele Fuzz-Worker beim Budget-Ablauf nicht sauber stoppen. Einzel-Worker behebt das false-positive.
- **Vollständiges Paket-Rename** (`secX → vaktX`) — alle verbleibenden Handler, Query-Dateien, SQL-Go-Dateien, Worker-Handler und Test-Fixtures auf die neuen Modul-Namen umgestellt.

### Tests

- **`config/validate_test.go`** (neu) — 5 Tests für Weak-Key-Guard: Zero-Key, Repeat-Byte, valid Key, zu kurzer Key, fehlende DB-URL.
- **E2E-Fixes** — 3 Playwright-Tests repariert: `compliance` navigiert auf `/vaktcomply/frameworks` (Accordion versteckte Nav-Labels); `ExpiringEvidenceWidget`-Crash bei paginated Mock-Response durch Fixture-Mock behoben; Keyboard-Shortcut-Tests warten auf Layout-Mount vor Tastendruck.

---

## [0.36.0] — 2026-05-27

**Marktreife-Programm — Sprint 56–59 Sammel-Release.** Schließt die 11 Top-Findings aus dem Auditos-Singularity-9-Agent-Audit + alle daraus hervorgegangenen P1-Items und Content-Drifts. 15 neue ADRs (0033–0047), 3 Migrationen (149–151), Backend 33 Pakete + Frontend 482 Tests durchgehend grün.

> **Operative Hinweise:** Migrationen 149 (`audit_log`-Hash-Chain), 150 (RLS-Theater zurückgenommen) und 151 (`audit_log` Range-Partitioning auf `created_at`) sind additiv bzw. data-preserving. Migration 151 ändert den `PRIMARY KEY` von `(id)` auf `(id, created_at)` — anwendungsseitig transparent. Operator: optional `VAKT_AUTH_FAIL_OPEN_ON_REDIS_OUTAGE=true` setzen, falls die strengere Default-Behandlung (503 bei Redis-Outage) für ein Deployment unpassend ist.

### Security (Audit-Findings F1, F2, F4, F5, F6, F7, F8, F9, F10, F11 + XFF/Cross-Org)

- **OIDC `email_verified`-Gate beim Account-Linking** (F4, ADR-0033) — fremde OIDC-Subjects werden nicht mehr blind an Lokal-Accounts mit gleicher Email gelinkt, solange der IdP die Email nicht als verifiziert ausweist.
- **License-Activate Role-Case-Fix** (F10) — `license/routes.go` checkt jetzt `"Admin"` (PascalCase, DB-Seed-konform) statt des nirgendwo gesetzten `"admin"`. Pro-Aktivierung funktioniert wieder.
- **LocalLLMBadge zeigt Provider ehrlich** (F2, ADR-0034) — Backend liefert `provider_host` in `/ai/status`, Frontend reicht es in den Badge durch. Kein "Lokal"-Badge mehr bei OpenAI-Cloud.
- **XFF-Spoofing-Schutz** — `VAKT_TRUSTED_PROXIES` wird als CIDR-Liste in echo-`TrustOption`s übersetzt; XFF-Header von außerhalb des Trust-Sets werden ignoriert.
- **SAML `InResponseTo`-Binding** (F5, ADR-0036) — HMAC-signiertes Single-Use-Cookie bindet AuthnRequest-ID an die Browser-Session; ACS akzeptiert nur Assertions mit passendem `InResponseTo`.
- **Operator-Rebrand abgeschlossen** (F11, ADR-0035) — Helm/CRD/RBAC auf `secrets.vakt.io / VaktSecret` migriert; Group-Konsistenz per Unit-Test gepinnt.
- **Cross-Org Approve-Hijack geschlossen** — `AgentRunManager.Decide` prüft Caller-Org und User-Owner; fremde `run_id`-Approvals geben 404.
- **`cmd/rotate-key` repariert + erweitert** (F1, ADR-0038) — HKDF-Coverage auf alle 8 verschlüsselten Spalten (`so_secrets`, `totp_secrets`, `notification_channels` ×2, `integrations_github`, `org_saml_configs`, `webhooks.secret`, `cloud_integrations.config`). SAML-Legacy-Rows (raw-master-encrypted) werden im Lauf migriert.
- **`audit_log` tamper-evident** (F8, ADR-0040, Migration 149) — Per-Org SHA-256 Hash-Chain mit `prev_hash` und `entry_hash`. Neues Tool `cmd/audit-verify` lokalisiert Tamper auf die exakte Row. ISO 27001 A.12.4.3 / NIS2 / DORA Art. 11 Audit-Trail-Anforderungen erfüllt.
- **AI-Counter zentralisiert** (F3, ADR-0041) — Echo-Middleware `RequireAILimit` ersetzt inline-Gates; alle 8 LLM-erzeugenden Routes durch das Gate. Statischer Route-Coverage-Test verhindert künftige Drift.
- **PII-Log-Redaktion** (F7, ADR-0039) — Helper `logsafe.RedactEmail` (Format `***@domain`) ersetzt Volltextexposures in 38 Call-Sites über 13 Dateien.
- **Auth-Lockout fail closed** (ADR-0044) — `checkAccountLocked` / `checkIPLocked` geben 503 `AUTH_LOCKOUT_UNAVAILABLE` bei Redis-Outage statt fail-open. Opt-out via `VAKT_AUTH_FAIL_OPEN_ON_REDIS_OUTAGE=true`.
- **RLS-Theater zurückgenommen** (F6, ADR-0042, Migration 150) — Migration 012 hatte `ENABLE ROW LEVEL SECURITY` aktiviert, ohne dass die App `app.current_org_id` setzte. Ehrlich-Rückbau auf reine App-Layer-Isolation.
- **`shieldstack` Build-Artefakt aus Working-Tree entfernt** (F9, ADR-0037) — Datei war seit `b83890c` aus HEAD entfernt; lokal aufgeräumt, History-Rewrite-Plan dokumentiert.
- **`webhooks.secret` Legacy-Migration** (ADR-0043) — Boot-Hook `MigrateLegacyPlaintextSecrets` konvertiert historische Plaintext-Secrets idempotent auf das `enc:v1:`-Format.

### Operations & Releases (P1-1, P1-2, P1-5)

- **Worker-Health/Readiness** (P1-5) — `/health` (Liveness), `/health/ready` (DB + Asynq-Queue-Probe), `/health/queue` (per-Queue Counts) statt einzelnem DB-Ping.
- **`audit_log` Range-Partitioning** (P1-2, ADR-0045, Migration 151) — Yearly Partitions (2025–2028) + DEFAULT, `audit_logs`-Backcompat-View neu erstellt.
- **SBOM + SLSA-Provenance pro Release** (P1-1, ADR-0046) — `release.yml` generiert SPDX-2.3 + CycloneDX SBOMs via syft, attestiert via `cosign attest --type spdxjson`. Release-Body enthält SBOMs als Assets. Compliance für EU CRA Art. 13(15).

### Content

- **BSI IT-Grundschutz von 7 Stub-Controls auf 34 Bausteine** (ADR-0047) — vollständige Abdeckung aller 10 Schichten (ISMS, ORP, CON, OPS, DER, APP, SYS, IND, NET, INF), jeder Control mit deutscher Description, Domain, Evidence-Type und Weight nach CRA/DORA-Pattern.
- **i18n-Sweep P0+P1 (79 neue Keys × 4 Locales = 316 Strings)** — `AccessReviewsPage`, `AISystemsPage`, `ResilienceTestsPage`, `ExceptionsPage`, `EvidenceAutoPage`, `TISAXMappingPage`, `DSGVOTOMPage` von hardcoded-Deutsch auf `useTranslation`. 240 i18n-Contract-Tests pinnen alle 60 Keys × 4 Locales gegen Drift.

### Migrations

- **149** — `audit_log` Hash-Chain (`prev_hash`, `entry_hash` BYTEA-Spalten + Index).
- **150** — RLS-Policies aus Migration 012 zurückgenommen.
- **151** — `audit_log` zu `PARTITION BY RANGE (created_at)`, Yearly + DEFAULT.

### Tools

- **`cmd/audit-verify`** — neuer Verifier für die Audit-Log-Hash-Chain.
- **`cmd/rotate-key`** — komplett umgebaut zu einer Pipeline aus 8 Stages mit unit-testbaren Stage-Funktionen.

### Tests

- Backend: **33 Pakete grün** (Unit + neue Integration-Tests via testcontainers-postgres in `internal/integration_test/`).
- Frontend: **482 Tests grün** (vorher 242 + 240 neue i18n-Contract-Tests).

---

## [0.35.0] — 2026-05-25

> Tag-Note: dieser Release-Eintrag wurde nachträglich im Zuge von v0.36.0 ergänzt. v0.34.0 + v0.35.0 enthielten zwei Commits zur Pro-Tier-UX (`feat(ux): ProGate "Demnächst" + DemoTierHint für Pro-Module`) und Billing-Korrektur (`fix(billing): Polar.sh Checkout-URL auf tatsächliche Product-ID aktualisiert`).

---

## [0.33.0] — 2026-05-25

Monetarisierung Phase 4 — Pricing-Dokumentation + Public README

### Changed

- **README Pricing-Section** — Vollständige CE/Pro/Enterprise Tier-Tabelle mit Framework-Matrix (NIS2/ISO 27001 ✅ Community; BSI/EU AI Act/CRA ✅ Pro; DORA/TISAX/ISO 42001 ✅ Enterprise), Modul-Verfügbarkeit und Feature-Vergleich (AI: 25 req/month CE vs. Unlimited Pro/Enterprise). Checkout-Links auf Polar.sh aktualisiert.

---

## [0.32.0] — 2026-05-25

Monetarisierung Phase 3 — In-App UX vollständig

### Added

- **CE AI-Counter-Anzeige** — `CEAICounter`-Component zeigt "18 / 25 KI-Anfragen diesen Monat" mit Fortschrittsbalken im KI-Berater-Widget. Warnung bei ≤5 verbleibenden Anfragen (Amber), Erschöpft-State mit Upgrade-Link (Rot).
- **`useAIUsage` Hook** — ruft `GET /api/v1/vaktcomply/ai/usage` ab, liefert `{used, limit, is_pro}`. Pro-Orgs: `is_pro=true`, Counter ausgeblendet.
- **`AI_CE_MONTHLY_LIMIT` Error-Handling** — KI-Berater zeigt deutschen Hinweis statt generischem Fehler wenn das CE-Monatslimit erreicht ist.

### Changed

- **Checkout/Portal-URLs auf Polar.sh migriert** — `frontend/src/lib/constants.ts`: `VAKT_PRO_CHECKOUT_URL` → `buy.polar.sh/norvik-ops/vakt-pro-monthly`, `VAKT_POLAR_PORTAL_URL` neu. `VAKT_LS_PORTAL_URL` als Alias erhalten.

### Notes

Folgende Phase-3-Elemente waren bereits implementiert: License-Key-Eingabe (Settings → Lizenz), ProGate-Upgrade-Prompt, 30-Tage-Ablauf-Banner (LicenseExpiryBanner), Post-Expiry-Hint mit Renewal-Link.

---

## [0.31.0] — 2026-05-25

Monetarisierung Phase 2 — Gate Enforcement vollständig, CE AI-Counter

### Added

- **CE AI-Monatslimit (25 Anfragen)** — Community-Edition-Orgs können AI-Features (Gap-Analyse, Policy-Draft, Incident-Guide, Chat, GapExplain, RiskNarrative) bis zu 25-mal pro Monat verwenden. Ab Anfrage 26 folgt HTTP 402 mit `AI_CE_MONTHLY_LIMIT`. Pro/Enterprise: unbegrenzt.
- **`GET /api/v1/vaktcomply/ai/usage`** — gibt `{used, limit, is_pro}` zurück. Frontend nutzt das zum Anzeigen von "18/25 Anfragen diesen Monat".

### Notes

Feature-Gates für alle Module und Frameworks (TISAX, DORA, CRA, EU AI Act, SCIM, SSO) waren bereits vollständig implementiert (106 aktive `features.Require()`-Gates). Phase 2 war deshalb auf den fehlenden CE-AI-Counter reduziert.

---

## [0.30.0] — 2026-05-25

Monetarisierung Phase 1 — Polar.sh Webhook, Demo-Tier Enterprise, License-Infrastruktur vollständig

### Added

- **Polar.sh Webhook** — `POST /api/v1/billing/webhook` empfängt Polar.sh-Subscription-Events und stellt automatisch Pro-Lizenzschlüssel aus. HMAC-SHA256-Signaturverifikation, Replay-Schutz via `polar_webhook_events`, idempotente Subscription-Speicherung in `polar_subscriptions`. Migration 148.
- **Demo → Enterprise-Tier** — `VAKT_DEMO=true` erteilt jetzt Enterprise-Tier statt Pro. Alle Features inkl. SCIM, TISAX, DORA sichtbar für Interessenten auf der Demo-Instanz.
- **`IsEnterprise()` auf License** — neue Hilfsmethode für Enterprise-Gate-Checks. `IsPro()` gibt auch für Enterprise `true` zurück (Enterprise ⊇ Pro).
- **`VAKT_POLAR_WEBHOOK_SECRET`** — neue Umgebungsvariable für Polar-Webhook-Signaturprüfung, dokumentiert in `.env.example`.

---

## [0.29.0] — 2026-05-25

Pre-v1.0 Sprint D — HKDF-Schlüsseltrennung, SCIM-Token-Ablauf, Pentest-Dokumentation

### Security

- **HKDF domain-separated keys** — `VAKT_SECRET_KEY` leitet jetzt via HKDF-SHA256 separate Sub-Keys für jede Komponente ab (`vakt-paseto-v1`, `vakt-vault-v1`, `vakt-totp-v1`, `vakt-alert-v1`, `vakt-github-v1`, `vakt-cloud-v1`, `vakt-webhook-v1`). Algorithmus-Isolation: ein kompromittierter Token-Key gibt keinen Zugriff auf verschlüsselte Vault-Secrets und umgekehrt. **Breaking:** alle aktiven Sessions werden beim Rollout ungültig (Paseto-Signing-Key geändert).
- **Pentest-Scope-Dokument** — `docs/security/pentest-scope.md`: vollständige Scope-Definition für externe Pentester (In-Scope-Klassen, Test-Accounts, Out-of-Scope, Timeline, erwartete Deliverables).
- **Responsible-Disclosure-Policy** — `docs/security/responsible-disclosure.md`: öffentlich zugängliche Policy mit Timelines, sicheren Kommunikationskanälen, Safe-Harbour-Erklärung.

### Added

- **SCIM Token-Ablauf** — `POST /api/v1/admin/scim/tokens` akzeptiert jetzt `expires_in_days` (0 = unbegrenzt). Abgelaufene Tokens werden täglich automatisch durch einen Worker-Job revoked. Migration 147: `expires_at`-Spalte auf `scim_tokens`.

---

## [0.28.0] — 2026-05-25

Pre-v1.0 Sprint C — Datenbankperformance, unbegrenzte Queries gecappt

### Performance

- **Audit-Log-Composite-Index** — neuer Index `idx_audit_log_org_time ON audit_log (org_id, created_at DESC)`. Audit-Trail-Queries im Compliance-Dashboard sind ab 10.000+ Einträgen deutlich schneller. Migration 145.
- **Risk-Trend-Snapshots** — täglicher Worker-Job berechnet Risiko-Snapshot pro Organisation und schreibt in `vb_risk_trend_snapshots`. Dashboard-Queries laufen jetzt in O(Tage) statt O(Findings × Tage). Migration 146. Fallback auf Live-Berechnung für frische Instanzen ohne Snapshots.

### Fixed

- **Unbegrenzte Datenbankqueries** — 7 interne `:many`-Queries hatten kein `LIMIT` und konnten bei großen Datensätzen den DB-Pool blockieren. Alle gecappt: Risiken/Policies/Suppressions/SBOM-Komponenten (10.000), Scan-Schedules/Control-Tasks (500), Kommentare (200). Interne Aufrufer (PDF-Export, Audit, XLSX) nutzen explizit `limit=10_000`.

---

## [0.27.0] — 2026-05-25

Pre-v1.0 Sprint B — Command Palette, HR Toast-Undo

### Added

- **Command Palette** (`GlobalSearch`) — `Cmd+K` / `Ctrl+K` öffnet eine globale Suchpalette. Schnellnavigation zu Dashboard, Controls, Risiken, Vorfälle, Richtlinien, Findings und Board-Bericht. Freitext-Suche über alle Entitäten (Controls, Risks, Policies, Incidents, Assets, Findings, DSR, Breaches). Recent-Items-Gedächtnis, Keyboard-Navigation (↑↓ + Enter), Focus-Trap.
- **Toast-Undo für HR** — das Undo-Pattern (5-Sekunden-Countdown, Löschung erst nach Ablauf) ist jetzt auf HR-Checklisten-Items (`ChecklistsPage`) und Mitarbeiter-Verwaltung (`EmployeesPage`) ausgerollt. Bereits seit v0.24.0 aktiv für Risiken und Ausnahmen in Vakt Comply.

---

## [0.26.0] — 2026-05-25

Pre-v1.0 Sprint A — Infrastruktur-Hygiene

### Added

- **Helm Migration-Job** — `helm/vakt/templates/migrate-job.yaml` führt Datenbankmigrationen als Helm Pre-Upgrade-Hook aus. Keine manuellen Schritte mehr vor `helm upgrade`.
- **Konfigurierbare DB-Connection-Pool-Größe** — `VAKT_DB_MAX_CONNS` (Default: 25) ermöglicht Tuning für größere Deployments. Dokumentiert in `.env.example`.
- **Webhook-Secrets verschlüsselt** — Webhook-Secrets werden jetzt mit AES-256-GCM at rest verschlüsselt. Secrets sind nach der Erstellung nicht mehr über List/Get-Endpoints abrufbar (write-once). Bestehende Plaintext-Secrets werden beim Lesen transparent entschlüsselt (lazy migration).

### Changed

- **Vakt Operator** — Kubernetes-Operator umbenannt: Go-Modul `github.com/matharnica/vakt-operator`, CRD-Group `secrets.vakt.io/v1alpha1`. **Breaking** für bestehende Operator-Deployments (als experimental markiert, kein Bestand).
- **Modul-Isolation** — `vaktcomply` importiert `hr` nicht mehr direkt. HR-Onboarding/Offboarding-Evidence läuft über einen geteilten Interface-Typ in `internal/shared/platform/evidence`.

---

## [0.25.0] — 2026-05-25

Pre-v1.0 Phase 1 — Kritische Sicherheits- und Zuverlässigkeitsfixes

### Security

- **Offene Registrierung geschlossen** — `POST /api/v1/auth/register` liefert 403, sobald eine Organisation existiert. Nur der Bootstrap-Fall (leere DB) erlaubt die erste Registrierung. Migration 144 (`open_registration`-Spalte, Default `false`).
- **API-Key-Rotation IDOR** — `RotateKey` prüft jetzt `created_by = current_user`. SecurityAnalysts konnten bisher beliebige Keys der Organisation rotieren; das ist behoben.
- **MFA-Bypass via API-Keys dokumentiert** — die MFA-Middleware exemptiert API-Key-Sessions explizit (Automation-Pfad, kein interaktives TOTP möglich). Kommentar im Code erklärt das bewusste Design.

### Fixed

- **Redis-URL-Bug im Worker** — `buildServer()` und `buildScheduler()` haben die Redis-URL bisher direkt als `host:port` interpretiert. Bei URLs mit Passwort (`redis://:pw@redis:6379`) lief der Worker ohne Authentifizierung. Behoben via `redis.ParseURL()` — identisch zum API-Container. Background-Jobs (Demo-Cleanup, Token-Cleanup, Scan-Fortschritt) funktionieren jetzt zuverlässig.
- **BSI-Grundschutz-Controls stummes Abschneiden** — interne Aufrufer nutzten `ListCKControls` (LIMIT 1000). BSI-Grundschutz hat 800+ Controls; eigene Controls kommen hinzu. Alle internen Caller nutzen jetzt `ListCKControlsPaged` mit 10.000-Limit.

---

## [0.24.0] — 2026-05-24

Pre-v1.0 Consolidation Wave — Module Depth, AI-Native v2, Security Docs, UX Polish, Architecture Hygiene

### Added

#### Vakt Aware — Module Depth (S55)
- **8 Phishing Templates** — ready to use in every fresh instance: credential harvesting, invoice fraud, IT helpdesk, parcel notification, CEO fraud, MS 365, bank alert, software update.
- **5 Training Modules** — Phishing Awareness, Password Hygiene, Clean Desk Policy, MFA & 2-Factor, Social Engineering. Completions automatically flow as evidence into Vakt Comply.
- **Comply Evidence Banner** — resolving a finding shows "Finding resolution saved as evidence in Vakt Comply" + link. Training completions show "Saved automatically as evidence."
- **Extended Getting-Started Guide** — Step 6 (First Scan) and Step 7 (First Campaign), each with prerequisites, expected duration, and a direct action link.
- **Demo seed enrichment** — campaign click events pre-populated in demo instances for realistic campaign analytics.

#### Vakt Comply & Scan — Module Depth (S54)
- **Scanner status endpoint** — `GET /api/v1/vaktscan/scanner-status` returns `{trivy, nuclei, openvas}` availability; admin dashboard shows scanner health.
- **HR → Comply evidence flow** — completing an HR onboarding/offboarding checklist emits an evidence event in Vakt Comply (`/vaktcomply/evidence/auto`) with ISO 27001 A.6.1/A.6.5 control-mapping suggestion.
- **Control suggestion for HR evidence** — unassigned HR evidence shows a rule-based control suggestion, reducing manual mapping overhead.

#### AI-Native v2 (S52)
- **Evidence Freshness Check** — daily job flags controls with evidence older than 90 days as `evidence_stale` insight cards (24h dedup per control).
- **Gap-Explain (SSE)** — `POST /api/v1/vaktcomply/ai/controls/:id/explain` streams a German-language gap explanation into the control detail page. Local AI advisor, no external API.
- **Risk Narrative** — `POST /api/v1/vaktcomply/ai/risks/:id/narrative` generates and persists a risk narrative; displayed in Risk Detail with a "Regenerate" option.
- **AI Weekly Digest** — opt-in in Settings → AI Advisor. Every Monday 08:00 UTC: digest of open gaps, stale evidence, and unresolved high-severity findings.
- **Evidence Suggestion Banner** — Finding Detail shows `evidence_suggestion` insight cards for the current finding with one-click navigation to the suggested control.
- **AI Insights Widget** — Vakt Comply dashboard shows up to 5 dismissable AI insight cards sourced from `ck_ai_insights`.

#### UX Polish (S58)
- **Inline-Edit Controls** — Control title and status editable directly in the table row (double-click → field, Enter saves, Escape cancels). No modal for these fields.
- **Inline-Edit Findings & Risks** — Status and severity inline-editable. Bulk status-change via BulkActionBar + "Change status to…" dropdown for selected findings.
- **Optimistic UI for toggle states** — all boolean status PATCH calls update the UI immediately; on HTTP error: automatic rollback + error toast. No spinner wait.
- **Toast-Undo for delete actions** — all DELETE calls show a 5-second countdown toast with "Undo". DELETE executes only after the countdown expires.
- **AI Source Attribution** — AI responses include structured `sources` chips (e.g. "NIS2 Art. 21", "ISO 27001 A.6.1") extracted from the response. Chips navigate to the corresponding control or framework page.

#### Enterprise Trust & Security Docs (S60)
- **TOM (Art. 32 DSGVO)** — `docs/security/tom.md`: Technical and Organisational Measures document, verified against Go implementation (16/16 claims confirmed).
- **VVT Template (Art. 30 DSGVO)** — `docs/security/vvt.md`: Records of Processing Activities template with 9 pre-filled processing activities.
- **Internal Self-Pentest Guide** — `docs/security/pentest-intern.md`: OWASP Top 10 checklist with curl commands for IDOR, privilege escalation, SQL/prompt injection, brute-force, token revocation, and Vakt-specific attack surfaces (SSRF, mass assignment).
- **External Pentest RFP** — `docs/security/pentest-rfp.md`: ready-to-send RFP targeting Q3 2026 with scope, deliverables, timeline, budget (€3–8k), and 5-vendor shortlist.
- **SCIM 2.0 Verification Checklist** — `docs/security/scim-verification.md`: 10-point manual verification checklist with curl commands and Okta integration reference.

### Changed

#### Architecture Hygiene (S59)
- **Audit package consolidated** — `auditexport` + `auditreport` merged into `shared/audit` with `ExportHandler` / `ReportHandler`.
- **Worker handlers split** — 1,443-line `handlers.go` split into 5 domain files: `auth_handlers.go`, `scan_handlers.go`, `comply_handlers.go`, `aware_handlers.go`, `privacy_handlers.go`.
- **vaktcomply repository split** — 4,724-line `repository.go` split into 9 domain files < 600 lines each.
- **Integration test CI job** — new GitHub Actions job runs Go integration tests (`//go:build integration`) against a real PostgreSQL container on every push to `main`.

### Security

#### Security Hardening (S57)
- **Silent SQL error logging** — raw SQL errors no longer surface to API consumers; structured logging with context in `mfa_sensitive`, `org_ip_allowlist`, `audit`, `dataexport`, `license`, `auth`, `ai/service`.
- **MFA middleware hardened** — 8 unit tests added; fail-closed on org-DB error (503) and TOTP-DB error (403).
- **AI streaming hardened** — SSE endpoints validate content type and connection state; panics caught and logged.
- **TOM correction** — SCIM Bearer tokens are SHA-256 hashed (not bcrypt) — deterministic lookup required for API tokens. Documented in `docs/security/tom.md`.

### Fixed
- `no-unnecessary-type-arguments` ESLint rule — removed redundant `Error` type argument from TanStack Query mutation hooks.
- TypeScript strict mode — `useMutation` context generic added for optimistic rollback hooks.

---

## [0.23.0] — 2026-05-23

Security Hardening Wave 2 + Release Readiness Phase 1

#### Phase 1 — Release Readiness

- **feat(auth): Enterprise-Auth Frontend vollständig** — Confirm-Dialog für Session-Widerruf in `SessionsPage` (inkl. Panic-Button „Alle anderen abmelden"), Audit-Trail-Link pro API-Key in `ApiKeysPage`, Login-History-Section in `AccountSettingsPage` (letzte 50 Versuche, Failed-Logins fett markiert) (S20-3, S20-5, S20-7)
- **refactor(i18n): 62 raw date-Calls auf `useFormatDate` migriert** — alle Datumsangaben in Audit-Trail, Finding-Listen, Session-Tabellen, Compliance-Reports und Supplier-Portal respektieren jetzt die gewählte Sprache (DE/EN/FR/NL); kein hardcoded `de-DE` mehr in React-Komponenten (S13-27)
- **fix(i18n): `shared/utils/date.ts` auf `navigator.language` umgestellt** — Fallback-Locale in Utility-Funktionen war hardcoded `de-DE`; liest jetzt Browser-Locale dynamisch; betrifft Chart-Label-Formatter und CSV-Export-Datumsspalten

#### Sicherheit
- **Per-Email Password-Reset-Throttle** — max. 3 Reset-Mails pro Stunde pro Adresse via Redis-INCR; verhindert Inbox-Spam-Angriffe ohne Enumeration-Leak (Antwort bleibt immer HTTP 200)
- **HR API-Key-Scope** — `/api/v1/hr/`-Endpoints werden jetzt in der Scope-Path-Map geprüft; scoped API-Keys mit `"hr"`-Scope können gezielt auf HR-Endpoints zugreifen, andere Scopes werden abgewiesen

#### Bugfixes
- **EOL-Version-Parsing: Großbuchstaben-V-Prefix** — `normaliseCycle("V3.9")` lieferte `"v3.9"` statt `"3.9"`, weil `TrimPrefix` case-sensitiv ist und vor `ToLower` aufgerufen wurde. Fix: erst lowercase, dann trim. Betraf SBOM-Komponenten mit Großbuchstaben-V-Versionspräfix (z.B. aus Syft), die silently als "unknown" EOL-Status bewertet wurden.

#### Tests
- **MFAEnforceMiddleware vollständig getestet** — 8 neue Unit-Tests ohne Real-DB via `mfaDB`-Interface-Fake: exempt paths, missing context, fail-closed bei org-DB-Fehler (503), fail-closed bei TOTP-DB-Fehler (403), MFA required/not required, TOTP enabled/disabled
- **Password-Reset-Throttle-Invarianten** — 5 reine Logik-Tests: Konstanten-Grenzen, Zähler-Bedingung, Redis-Key-Format
- **vaktscan Domain-Invarianten** — 15 neue Tests: SLA-Severity-Mapping (BSI-90-Tage-Fallback), EOL-Versionsparsing (`majorCycle`, `normaliseCycle`), EOL-Payload-Deserialisierung (bool/string/date polymorph), `eolValue.UnmarshalJSON` alle 6 Varianten

#### Infrastruktur
- **`StartBackgroundRefresh` Lifecycle-Context** — Update-Check-Goroutine läuft jetzt mit Server-Lifecycle-Context statt `context.Background()`; wird bei SIGTERM sauber gestoppt bevor Echo shutdown

### v0.22.0 — Supplier Portal + Vakt Scan (2026-05-22)

#### Added
- Supplier Portal Phase 1 — Lieferanten-Register, Fragebogen-Builder (4 Frage-Typen, 3 Templates), externes Portal via Token-Link ohne Login
- Supplier Portal Phase 2 — Auswertungsansicht, Zertifikat-Ablauf-Alert (30 Tage), Assessment-Report PDF
- Asset Inventory — `environment` (prod/staging/dev), Kritikalitätsstufen, Ownership; Migration 139
- CVE-Enrichment-Service — NVD API v2.0, Redis-Cache 24h, 429-Retry-Backoff
- Finding-Deduplizierung cross-scanner — CVE+Asset-Key, Severity-Max-Merge, `sources`-JSONB
- SLA-Overdue-Badge in Findings-Liste — zeigt "SLA überfällig" wenn `sla_due_at` überschritten

---

### v0.21.0 — EU AI Act (2026-05-22)

#### Added
- KI-System-Inventar — `ai_systems`, `ai_classifications`; CRUD + Filter nach Risikoklasse + Status
- Risiko-Klassifizierungs-Wizard — JSON-konfigurierter Entscheidungsbaum nach Annex III (Verbots-Prüfung → Hochrisiko → Transparenzpflicht)
- Technische Dokumentation Hochrisiko-KI (Art. 11) — Template nach Annex IV, Versionierung, PDF-Export
- EU AI Act Dashboard — Kachel mit Systemen pro Risikoklasse, Countdown August 2026

---

### v0.20.0 — TISAX (2026-05-22)

#### Added
- TISAX® / VDA ISA-Framework — alle 15 Kapitel als Controls, Reifegrad 0–3, Schutzbedarf Normal/Hoch/Sehr hoch (Kapitel 15 Prototypenschutz optional)
- TISAX ↔ ISO27001 Mapping — ~60–70% Controls als vorgefüllt bei aktivem ISO27001
- TISAX Bereitschaftsbericht PDF — Reifegrad pro Kapitel, offene Controls, Deckblatt mit Assessment-Level

---

### v0.19.0 — BSI-Meldungsassistent + i18n (2026-05-22)

#### Added
- BSI-Meldungsassistent — Meldepflicht-Klassifizierung (3-Fragen-Wizard, obligation probably/unclear/none), Behörden-Empfehlung (BSI/BaFin+BSI/BNetzA/LDA), Migration 140
- Behörden-Verzeichnis (`authorities.yaml`) + Sektor-Konfiguration in Org-Settings
- Täglicher NIS2-Deadline-Check-Worker (24h/72h/30d-Fristen ab `first_detected_at`)
- Gemeinsamer `compliance_reporting`-Service — `DeadlineTracker`, `ComputeDeadlines()`, `AmpelStatus()`, `DORADeadlines`, `NIS2Deadlines`, `DSGVODeadlines`
- DORA TLPT-Dokumentation — Resilience-Test als DORA-Evidenz verknüpfbar; `POST /resilience-tests/:id/link-evidence`
- i18n-Infrastruktur Phase 1 — `i18next` vollständig verdrahtet, Locales DE/EN/FR/NL, Locale-Umschalter in User-Settings

---

### v0.18.0 — DORA Phase 1+2 (2026-05-22)

#### Added
- DORA-Kontrollkatalog als Framework-Seed (Art. II–VI, alle Artikel als Controls)
- DORA ↔ ISO27001 Mapping — geteilte Evidenz, „DORA-Lücken nach ISO27001-Abzug"
- IKT-Incident-Register — Typ `ikt_dora`, Felder `first_detected_at`, `reported_24h/72h/30d_at`, `severity_dora`, DORA-Klassifizierungs-JSONB; Migration 136
- Frist-Berechnung + Ampel (Worker-Cron alle 5 min, grün/gelb/rot pro Frist)
- IKT-Drittanbieter-Register — `dora_third_parties`, Kritikalitätsstufen, Ausstiegsstrategie, Vertragsparameter; Migration 138
- DORA Dashboard-Kachel — Drittanbieter-Zähler, fehlende Ausstiegsstrategien
- DORA PDF-Report — Abschnitt IKT-Drittanbieter + Resilienz-Tests

#### Changed
- `internal/shared/` → `platform/` Welle 4 (auditor, integrations, ldap, trustcenter, webhooks)

---

### v0.17.0 — Auth-Welle (2026-05-22)

#### Added
- SAML 2.0 Direct SP (CE) — AzureAD, Okta, OneLogin, Google Workspace; Metadata-XML-Endpoint
- SCIM 2.0 User+Group Provisioning (Pro) — `/scim/v2/Users`, `/scim/v2/Groups`, Filter-DSL
- IP-Allowlist für Admin-Endpoints (Pro) — CIDR-Konfiguration in Org-Settings
- MFA für sensitive API-Calls (Pro) — TOTP-Validation via `X-MFA-Token`-Header
- SIEM-Audit-Forwarder (Pro) — Splunk HEC, Elastic Bulk API, Generic Webhook; Asynq-Job mit Retry
- ADR-0022 Auth-Tier-Cut (SAML CE / SCIM+SIEM Pro)

---

### v0.16.0 — Foundation-Welle (2026-05-22)

#### Added
- Feature-Flag-Infrastruktur (`platform/features`) — alle Pro-Features über `IsEnabled()` steuerbar
- AgentRunPanel Approve-Cards — Write-Tool-Freigabe-Flow mit Audit-Log
- Cursor-basierte Pagination für Findings, Controls, Risks, Secrets, DSRs, Employees, Campaigns
- Typisierte Cross-Module Event-Contracts (`platform/events`) — `FindingCreated`, `BreachNotified`, `EvidenceCollected`, `IncidentCreated`

#### Changed
- `internal/shared/` → `platform/` Welle 3 (crypto, db, cache, telemetry, middleware, metrics, alerting, notify, scheduledreports, retention)
- Worker-Queue-Namespaces pro Modul (vaktscan concurrency 8, vaktprivacy 5, ai_agent 3, vaktcomply 5)
- Redis-Auth-Fallback auf PostgreSQL bei Redis-Ausfall

#### Fixed
- Dashboard.tsx von 1448 auf 144 Zeilen dekomponiert (5 Komponenten)
- SQL-Injection-Risiko in `admin/service.go` (dynamisches WHERE → fixe NULL-Safe-Placeholder)
- `interface{}` vollständig aus `internal/` eliminiert (Go 1.18 `any`)
- CI Frontend-Lint ist jetzt explizit blockend (`continue-on-error: false`)

---

### v0.15.0 — NIS2 Pro-Layer (Tag-Kandidat, Sprint 28)

Schließt die Pro-Schicht aus Sprint 19 vollständig ab. Kein Breaking-Change — alle neuen Features sind additiv und hinter `FeatureNIS2Reporting` Pro-gated. CE-Features des NIS2-Wizards bleiben unverändert.

**S28-1 Embedded-Mode:**
- NIS2-Self-Assessment-Wizard via `<iframe>` einbettbar auf Partner- und Berater-Sites.
- CORS `Access-Control-Allow-Origin: *` auf öffentlichen Wizard-Endpoints (`/api/v1/public/nis2-assessment/*`).
- `X-Frame-Options`-Header wird auf `/nis2-check*`-Routen entfernt; CSP `frame-ancestors *` gesetzt.
- Resize-Helper `public/nis2-embed.js` (PostMessage-basiert, 26 Zeilen, kein Tracking, kein Cookie).

**S28-2 Branded PDF-Export (Pro, `FeatureNIS2Reporting`):**
- `GET /api/v1/public/nis2-assessment/:token/export-pdf` — generiert mehrseitiges PDF: Cover mit Gesamtscore, Bereichs-Tabelle, Top-Gaps, Detailantworten.
- Footer „Erstellt mit Vakt · vakt.io". Rückgabe als `application/pdf` Blob (filename `nis2-assessment.pdf`).
- Frontend-Download-Button im Result-Screen — sichtbar nur wenn authentifiziert. Bei `402 Payment Required`: Upgrade-CTA.

**S28-3 Re-Assessment-History (Pro, `FeatureNIS2Reporting`):**
- Neue Tabelle `ck_nis2_assessment_runs` (Migration 127): speichert vollständige Assessment-Runs mit Scores + Top-Gaps.
- 90-Tage-Cooldown zwischen Re-Assessments — `429 Too Many Requests` mit `Retry-After`-Header bei Verletzung.
- Endpoint `GET /api/v1/vaktcomply/nis2-assessment/history` liefert alle Runs sortiert nach Datum.
- Frontend-Seite `/vaktcomply/nis2-history`: Trend-Pfeile (TrendingUp / TrendingDown) pro Bereich, Delta-Spalte zum Vorrun, Cooldown-Restanzeige, Leer-State mit CTA.

**S28-4 Multi-Framework-Wizard (Pro, `FeatureNIS2Reporting`):**
- 80 kombinierte Fragen: NIS2 (~30), ISO 27001 (~25), DSGVO-TOM (~25). Stabile IDs mit `mf.`-Prefix.
- 23 Cross-Mapping-Fragen, die mehreren Frameworks angerechnet werden (Ref-Feld pro Frage).
- Score-Engine `MultiFrameworkScore`: `NIS2`, `ISO27001`, `DSGVO`, `Overall`, `TopGaps`, `ByFramework`.
- Neue Route `/nis2-check/multi` — eigene Frontend-Page mit drei Fortschrittsbalken (NIS2 indigo, ISO27001 emerald, DSGVO violet) + Cross-Mapping-Hinweis im Result.

**S28-5 Landing-Page SEO:**
- `docs/marketing/nis2-check-landing.md` — deutschsprachige SEO-Vorlage für `vakt.io/nis2-check`.
- Meta-Block (title, description, canonical), Hero, NIS2-Bereichs-Tabelle, 3-Schritt-Flow, Zielgruppen-Blöcke, FAQ (5 Fragen inkl. DSGVO-Hinweis), Legal-Disclaimer. Optimiert auf „NIS2 Self-Assessment", „NIS2 Umsetzungsgesetz", „BSI NIS2 Compliance Check".

---

### v0.14.3 — Interne Qualitätswelle (Sprints 24-27, kein User-Impact)

Keine neuen User-facing-Features. Keine DB-Migrations. Kein Upgrade-Eingriff nötig.

**S24 — UX-Polish + Security-Hardening:**
- **`Spinner`-Komponente** als zentrale Ladeanimation eingeführt; Inline-`div`-Spinner in Frontend entfernt.
- **`StatusMapping`-Bibliothek** — zentralisierte `Record`-Typen für Status/Severity-Farb- und Label-Mappings; keine gestreuten `switch`-Blöcke mehr.
- **Toast-Migration** — verbleibende Inline-`fixed-bottom`-Toast-Blöcke auf globalen `toast()`-Hook umgestellt.
- **Settings-Modul** — 6 Settings-Pages nach `modules/settings/pages/` migriert (saubere Modul-Struktur).
- **IP-Lockout** — per-IP Redis-Failure-Counter: nach 10 fehlgeschlagenen Logins wird die IP für 15 Minuten gesperrt. Brute-Force-Schutz auf Login-Endpoint.
- **Backup-HMAC** — Backup-Archive werden mit HMAC-SHA256 signiert; Integritätsprüfung beim Restore.

**S25 — sqlc-Welle 1 (SecPulse + SecVitals) + E2E:**
- **SecPulse sqlc-Abschluss** — 3 verbleibende Raw-SQL-Stellen in `vaktscan/` auf sqlc migriert.
- **SecVitals sqlc Wellen 1+2** — `service_soa`, `approvals_handler`, `handler_my_tasks`, `milestones_repository` auf sqlc.
- **Playwright E2E V22-1** — Sessions-Panic-2-Step-Confirm, ApiKeys-Rotate-Modal, AgentRunPanel-Visualisierung. Schließt V22-1 aus dem Verifizierungs-Backlog ab.

**S26 — sqlc-Welle 2 (SecVitals + SecReflex + HR):**
- **SecVitals sqlc Wellen 3+4+5** — `handler_ical`, `handler_templates`, `service_policies`, `service_frameworks`, `handler_boardreport`, `service_reporting`, `policy_acceptance` auf sqlc.
- **SecReflex + Vakt HR sqlc-Abschluss** — alle verbleibenden Raw-SQL-Stellen in beiden Modulen migriert.

**S27 — sqlc-Abschluss Vakt Vault + E2E Verification:**
- **Vakt Vault sqlc komplett** — 29 neue sqlc-Queries (Shares, API-Tokens, Git-Scans, Scan-Results, Rotation-Policies, Access-Log, Secrets-Metadata). Drei dokumentierte Ausnahmen bleiben Embedded-SQL: `UpsertSecret` (ON CONFLICT + Crypto-Bytes), `GetSecretRaw`, `GetSecretByID` — beide geben `[]byte`-Encrypted-Value zurück, das sqlc-Code-Gen nicht abbilden kann.
- **SecPulse CI-Evidence** — `INSERT INTO ck_evidence` in `handler_ci_evidence.go` auf `r.q.InsertCKCIEvidence` migriert.
- **E2E Grace-Period-Badge** — Playwright-Test für `API_KEYS_IN_GRACE`-Fixture (rotated_at = jetzt → `text=Grace 24h aktiv` sichtbar). Schließt V22-1 vollständig ab.

---

### v0.14.2 — Build-Hotfix (2026-05-23)

Pure Build-Fix. Funktional identisch zu v0.14.1 für den Runtime-Pfad.

- **OpenAPI-Drift gefixt:** `HealthResponse` und `DemoStartResponse` Schemas waren in `backend/internal/shared/apidocs/openapi.yaml` nie definiert, wurden aber in `frontend/src/pages/Login.tsx` per `components['schemas']` referenziert. `npm run build` (tsc -b) ist deshalb seit v0.14.0 rot. Schemas nachgezogen, Types regeneriert. ADR-0017-Honesty-Audit-Miss.
- **`Setup.tsx` dead state entfernt:** `migratedMsg`-useState wurde gesetzt, dann `navigate('/')` — gerendert wurde es nie. Auf `toast()` umgestellt, damit der User die NIS2-Migrations-Bestätigung nach dem Sign-up auch tatsächlich sieht.
- **Verifizierung:** `go test ./...` + `npm run build` + `npm run test` alle grün.

### Sprint 22 Tail — Verbleibende Frontend-Komponenten + Tests (Tag-Kandidat v0.14.1)

Schließt die 4 in v0.14.0 zurückgestellten Items aus Sprint 22 ab. Damit ist der Sprint-22-Honesty-Audit vollständig abgearbeitet.

**S22-8 AgentRunPanel-Frontend:**
- Neuer Hook `useAgentRun` (`frontend/src/shared/hooks/useAgentRun.ts`) konsumiert den SSE-Stream von `POST /api/v1/vaktcomply/ai/agent/run`, parsed strukturierte `AgentEvent`-Frames (plan / tool_call / tool_result / reflect / final / error) und liefert `events[]`, `isRunning`, `error`, `durationMs`, `start()`, `stop()`.
- Neue Komponente `AgentRunPanel` (`frontend/src/shared/components/AgentRunPanel.tsx`): Goal-Input, Start/Stop-Button, Event-Cards mit farbcodierten Typen, JSON-Expand/Collapse pro Event für Arguments + Result.
- Neue Page `AIAgentPage` unter `vaktcomply/ai/agent` — mountet das Panel, listet erlaubte Tools/Approve-Skelett.

**S22-9 ApiKeysPage-Refactor:**
- **Scope-Picker im Create-Dialog**: Checkbox-Liste pro Modul (`vaktcomply.*`, `vaktscan.*`, `vaktvault.*`, `vaktaware.*`, `vaktprivacy.*`, `hr.*`) mit Beschreibungstexten. Leer = Personal-Key (Full Access, ambers gekennzeichnet).
- **Rotate-Button pro Key** mit eigenem Modal: Erklärt die 24h Grace-Period explizit, zeigt den neuen Raw-Key nach Rotation einmalig im New-Key-Dialog.
- **Scope-Tags und Grace-Indicator** pro Row: code-style-Pills mit dem Scope-String, oder „Personal (Full Access)"-Badge wenn leer. Während aktiver Grace-Period zusätzlich „Grace 24h aktiv"-Marker.
- **last_used_ip-Anzeige** unterhalb von last_used_at (klein, monospace).

**Backend-Begleitänderungen:**
- `apikeys.APIKey` Struct um `LastUsedIP` + `RotatedAt` erweitert; `List` selectiert beide Felder mit. Middleware-Hook für API-Key-Auth-Erfolg updated jetzt zusätzlich `last_used_ip` aus `c.RealIP()`.

**S22-10 Session-Management — Current-Session-Marker + Panic-Button:**
- `auth.AuthResponse` um `session_id` (UUID der `refresh_sessions`-Row) erweitert. `issueTokenPair` nutzt `RETURNING id::text`, damit Login/Register/Refresh die ID mitliefern.
- Frontend `api/client.ts` um `getSessionId()`/`setSessionId()`-Helpers erweitert; `apiFetch` sendet die ID als `X-Vakt-Session-Id` Header automatisch mit. `Login.tsx` persistiert die ID in localStorage; `setAuthToken(null)` löscht sie wieder.
- `auth.SessionHandler.ListSessions` markiert die zur Header-ID passende Row mit `is_current: true`. `RevokeAllOtherSessions` nutzt die Header-ID statt einer nicht-funktionierenden Token-Hash-Vergleichslogik.
- `SessionsPage` zeigt „Diese hier"-Badge + last_used pro Session, separiert „Andere abmelden" und einen 2-Step-confirm Panic-Button („inkl. dieser") mit auto-redirect auf `/login` nach Revoke.
- OpenAPI-Spec entsprechend nachgezogen: `LoginResponse` um `session_id`, `SessionInfo` an Backend-Form angepasst (`device_hint`, `last_used`, `is_current`) — gem. ADR-0017.

**S22-14 Integration-Tests für Cleanup-Jobs:**
- Neue Test-Datei `internal/integration_test/cleanup_jobs_real_test.go` (build-tag `integration`):
  - `TestCleanupAnonymousRuns_DeletesExpiredRows` — seedet 1 expired + 1 fresh Row in `nis2_anonymous_runs`, ruft `nis2wizard.CleanupAnonymousRuns`, asserted nur expired ist weg.
  - `TestCleanupLoginHistory_DeletesOldEntries` — seedet 1 Eintrag vor 100 Tagen + 1 frischer Eintrag in `login_history`, ruft `auth.CleanupLoginHistory`, asserted Retention-Grenze 90d sauber.
- Beide Tests bootstrap Postgres via testcontainers-go (analog zu `hr_evidence_real_test.go`), skippen sauber wenn Docker nicht verfügbar.

**Operations-Doku:**
- `docs/operations/maintenance-window-server-upgrade.md` — Wartungsfenster-Plan für Strato VC-2-4 → VC-6-12 Upgrade: Pre-Flight (T-24h, T-1h), Live-Migration vs. Backup-Restore-Variante, Post-Flight-Validierung (Health-Smoke aus ADR-0017 Checklist), Rollback-Strategie, Kommunikations-Schema.

### Sprint 22 — Fertigstellungs-Welle für Sprints 17-20 (Tag-Kandidat v0.14.0)

Schließt die Skeleton-Lücken aus 17-20 nach dem Honesty-Audit vom 2026-05-22. Kein neues Feature-Versprechen, sondern Einlösung alter. 12 Items voll-implementiert, 4 größere Frontend-Komponenten als [~] in nachfolgende Welle verschoben.

**22.1 Backend-Bugs (echte Defekte):**
- **S22-1 Auth-Lookup mit Grace-Period:** API-Key-Auth-Middleware akzeptiert jetzt `previous_key_hash` während `previous_key_grace_expires_at > NOW()`. Beim Match über alten Hash: Response-Header `X-Vakt-Key-Deprecated: true` + `Sunset: <RFC1123>` als Migrations-Signal. **Bug aus Sprint 20 effektiv broken Rotation** ist gefixt.
- **S22-2 RequireScope-Kontext-Plumbing:** Auth-Middleware setzt jetzt `auth_method=api_key`, `api_key_scopes`, `api_key_id` im Echo-Context. `apikeys.RequireScope(scope)`-Middleware kann das nun nutzen — manuelles Mounten auf Routen ist möglich. Volle 200-Route-Annotation ist noch eigener Sprint, aber das Plumbing steht.
- **S22-3 OIDC + SAML + Register schreiben login_history:** `auth.OIDCLogin`, `auth.SAMLLogin`, `auth.Register` rufen jetzt `recordLogin` mit source=`oidc`/`saml`/`register`. Failed-OIDC-Provisioning auch als `oidc_failed`. Sprint 20 hatte nur Password-Pfad — Audit-Gap geschlossen.

**22.2 Sign-up-Integration (NIS2-Akquise-Loop schließen):**
- **S22-4 Setup.tsx liest `?nis2_token=` + localStorage** und ruft nach erfolgreichem Setup `POST /vaktcomply/nis2-assessment/migrate-from-anonymous` auf. CTA aus dem Public-Wizard läuft jetzt nicht mehr ins Leere.
- **S22-5 Auto-Mapping auf NIS2-Controls** in `nis2wizard.AutoMapToControls`: value 0-1 → `not_implemented`, 2 → `partial`, 3-4 → `implemented`. Mapping via NIS2-Ref-Substring auf `ck_controls.description`/`control_id`. Nur Controls ohne aktiven manual_status werden überschrieben.
- **S22-6 Authentifizierter Endpoint** `POST /api/v1/vaktcomply/nis2-assessment/migrate-from-anonymous`. Service-Methode `MigrateAndAutoMap` kombiniert Migration + Auto-Mapping in einem atomaren Schritt.

**22.3 Frontend-UI (3 von 5, größere Komponenten als [~]):**
- **S22-7 `ScanProgressIndicator`-Komponente** unter `modules/vaktscan/components/`. Konsumiert SSE-Stream, zeigt Live-Phase + Percent-Bar + Heartbeat-Filter. Auto-Cleanup beim Unmount via AbortController.
- **S22-11 `LoginHistorySection`-Komponente** unter `shared/components/`. Tabelle mit TS / Quelle / Browser-Excerpt / IP / Result-Badge. Failed-Logins fett markiert. UA-Mini-Parser (Firefox/Edge/Chrome/Safari-Detection). In `AccountSettingsPage` eingebaut.

**22.4 Cleanup-Jobs:**
- **S22-12 `TaskCleanupAnonymousRuns`** (täglich 03:15 UTC): `DELETE FROM nis2_anonymous_runs WHERE expires_at < NOW()`. Im Worker-Scheduler verdrahtet.
- **S22-13 `TaskCleanupLoginHistory`** (wöchentlich Sonntag 04:00 UTC): `DELETE FROM login_history WHERE ts < NOW() - INTERVAL '90 days'`. Worker-Handler + Scheduler-Cron.

**22.5 Doku:**
- **S22-15 `docs/reviews/2026-05-22-honesty-audit.md`** dokumentiert den Skeleton-Status-Audit der zu Sprint 22 führte. Methodik, Item-Klassifikation, Lessons-Learned.
- **S22-16 CHANGELOG + UPGRADE** für v0.14.0 mit klarer Bugfix-Kennzeichnung der S22-1-Rotation-Defekts.

**Verschoben (S22-8, S22-9, S22-10, S22-14 [~]) → Folge-Welle:**
- S22-8 `AgentRunPanel`-Frontend (groß, Streaming-UI mit Approve-Cards).
- S22-9 `ApiKeysPage`-Refactor (Scope-Checkbox-Wizard, Rotation-Button-UI mit Modal).
- S22-10 Session-Mgmt-Backend-Endpoint (`/auth/sessions{,/:id/revoke,/revoke-all}`) + SessionsPage-Ausbau.
- S22-14 Integration-Tests für Cleanup-Jobs (brauchen testcontainers-Setup, separater Test-Hardening-Sprint).

### Sprint 20 — Enterprise-Auth CE-Tier (Tag-Kandidat v0.13.0)

CE-Schicht der Enterprise-Auth-Welle: feingranulare API-Key-Scopes mit Wildcard-Logik, zerstörungsfreie Rotation mit 24-h-Grace-Period, Login-Historie pro User. Pro-Schicht (SAML, SCIM, IP-Allowlist, MFA-API, SIEM) bleibt explizit Sprint 21 — on-demand bei konkretem Enterprise-Sales-Trigger.

**Backend (S20-1, S20-2, S20-6, S20-8):**
- Migration 126: `api_keys.previous_key_hash` + `previous_key_grace_expires_at` + `last_used_ip` + `rotated_at` für Rotation. Neue Tabelle `login_history` (user/email/ip/UA/source/result) mit 90-Tage-Retention-Plan.
- `internal/shared/apikeys/rotation_and_scopes.go`:
  - `RequireScope(scope)` Echo-Middleware mit Wildcard-Logik (`*`, `vaktvault.*`, `vaktvault.secrets.read`).
  - `ScopeAllows([]string, string) bool` als exportierter Helper für den Auth-Lookup-Pfad.
  - `Service.RotateKey(orgID, keyID) (*CreateResult, error)` — generiert neuen Hash, alter Hash wandert in Grace-Period (24h), beide werden vom Auth-Middleware akzeptiert. Endpoint `POST /api/v1/api-keys/:id/rotate`.
  - `RecordLoginAttempt` + `ListLoginHistoryForUser` Helpers.
- `auth/service.go`: Login-Pfad schreibt `login_history`-Entry bei `bad_password` + `ok`. Best-Effort, blockiert Login nie. Failed-Login ohne user_id (Account-Enumeration-Schutz).

**Docs (S20-8):**
- `docs/concepts/api-key-scopes.md` — Scope-Format, Wildcards, CI-Pipeline-Workflow, Rotation mit Grace-Period, Migration für Bestands-Keys, Backend-Implementation-Verweise, Skeleton-Status zu Auth-Middleware-Integration.
- `docs/concepts/README.md` Index aktualisiert.

**Verschoben (S20-3/4/5/7 [~] Frontend-Iteration):**
- S20-3 ApiKeysPage-Refactor (Scopes-Checkbox-Liste, Rotation-Button, Last-Used-IP) — Backend ist da, Frontend Cosmetic-Iteration.
- S20-4 Session-Mgmt-Endpoint + S20-5 SessionsPage — bestehende Skelette aus Sprint 2 reichen aktuell; Vollausbau in Folge-Welle.
- S20-7 Login-History-Section in AccountSettingsPage — Backend-Service-Methode `ListLoginHistoryForUser` ist da, UI ist iterativ.

### Sprint 19 — NIS2-Self-Assessment-Wizard CE (Tag-Kandidat v0.12.0)

Top-of-Funnel-Akquise-Asset für DACH-Markt 2026. Anonymer Wizard mit 30 NIS2-Fragen, Live-Score, Top-3-Gaps. Pro-Schicht (Branded PDF, Trend-View, Multi-Framework) als Folge-Welle vorbereitet.

**Backend:**
- Migration 125: `nis2_anonymous_runs` (7d-Lebensdauer, IP-Hash für DSGVO) + `ck_nis2_assessments` (Org-Migration bei Sign-up).
- `internal/shared/nis2wizard/` mit 30 Fragen über 8 Themenbereiche (NIS2 Art. 21 + BSI NIS2-UmsG §30). Gewichtete Score-Engine 0-4 mit Per-Area-Aufschlüsselung.
- Public-Endpoints (kein Auth, Rate-Limit 5/min/IP): `POST /public/nis2-assessment/{start,answer}`, `GET /public/nis2-assessment/{result,questions}`.
- `Service.MigrateToOrg(token, orgID, userID)` für Sign-up-Flow.
- 9 Score-Engine-Tests.

**Frontend:**
- `pages/NIS2WizardPage.tsx` unter `/nis2-check` (kein Layout, mobile-first). Multi-Step-Flow, Progress-Bar, Live-Score, Token in localStorage für Wiederbesuch.
- Result-Screen mit Ampel-Bewertung, Top-3-Gaps, CTA „Account erstellen + Ergebnis übernehmen".

**Docs:**
- **ADR-0021** Accepted: CE vs Pro Cut. Wizard + Sign-up-Migration sind CE; Branded-PDF + Trend + Multi-Framework sind Pro.

**Verschoben (S19-7..12 [~] Folge-Welle):**
- Embedded-Mode (iframe), Branded-PDF, Re-Assessment-History, Multi-Framework-Wizard, Auto-Mapping bei Sign-up, Landing-Page-Marketing.

### Sprint 18 — Agentic-AI v2 (Tag-Kandidat v0.11.0)

Vakts erste agentische AI-Workflows mit Plan/Execute/Reflect-Loop, Tool-Registry und RBAC-Enforcement. Adressiert den Bericht-§8-„AI-Native"-Hebel.

**Backend:**
- `AgentRunner` (`services/ai/agent.go`) mit MaxIterations (Default 5, Cap 10), OnEvent-Callback, Rate-Limit + Quota wie AI-Chat-Stream.
- `AgentTool`-Interface + drei Read-Only-Tools: `list_open_findings`, `list_stale_evidence`, `list_controls_without_evidence`. Jedes Tool deklariert `RequireScope` (z.B. `vaktscan.findings.read`).
- `POST /api/v1/vaktcomply/ai/agent/run` als SSE-Endpoint. Frame-Types: `plan`, `tool_call`, `tool_result`, `final`, `error`. Terminiert mit `[DONE]`.

**RBAC + Audit:**
- Tools werden im Plan-Prompt NUR gelistet, wenn der User den Scope hat. Defensiver zweiter Check vor jedem Execute. Audit-Log-Entry pro Agent-Run-Start (`action=agent_run_start, actor=ai_agent`).
- **ADR-0020** Accepted: keine Privilege-Escalation via AI; Pre-Approval-Pattern für mutierende Tools vorbereitet.

**Drei initiale Workflows:** Triage offener Findings, Wochen-Compliance-Plan, Evidence-Re-Collection.

**Docs:**
- `docs/concepts/ai-agents.md` — Architektur-Diagramm, Komponenten, SSE-Format, drei Workflows, Skeleton-Grenzen.
- ADR-0020 in `docs/adr/README.md`-Index.

**Verschoben (S18-4 [~]):**
- `AgentRunPanel`-Frontend mit Live-Plan-Steps + Approve-Cards. Backend-SSE-Endpoint ist produktiv; Frontend ist Cosmetic-Iteration für eine Folge-Welle.

**Skeleton-Grenzen (bewusst):**
- Plan-zu-Tool-Mapping via Substring-Heuristik statt echtem OpenAI-Function-Calling-Schema.
- Reflect ist Single-Pass-Final-Event statt iterativer LLM-Roundtrip pro Tool-Result.
- Beide Punkte sind Folge-Wellen-Themen; das Skeleton beweist das Pattern + die RBAC-Architektur.

### Sprint 17 — Realtime-Welle (Tag-Kandidat v0.10.0)

Erste produktive SSE-Endpoints nach dem ADR-0019-Pattern aus Sprint 16. Notifications und Scan-Progress werden jetzt live gepushed statt gepollt.

**Backend (S17-1, S17-2, S17-7):**
- `GET /api/v1/dashboard/notifications/stream` — server-side-poll-and-push, 2 s Cursor-Tick, 30 s Heartbeat-Pongs (`event: ping`). Skaliert besser als Postgres-LISTEN-per-Connection.
- `GET /api/v1/vaktscan/scans/:id/progress/stream` — subscribed Redis Pub/Sub auf `scan:progress:<id>`-Channel. Worker publiziert `started` und `finished`/`failed`; Stream beendet sich mit `data: [DONE]`. Org-Isolation enforced (Cross-Org-Stream → 404).
- `internal/modules/vaktscan/progress_stream.go` mit `PublishProgress(rdb, evt)`-Helper; im Worker (`handleScanJob`) verdrahtet vor + nach jedem Scan-Run.
- OpenTelemetry-Spans pro Stream-Lifecycle.

**Frontend (S17-3, S17-4):**
- `useNotificationStream`-Hook — fetch-SSE-Reader, Auto-Reconnect mit 1-s-Backoff, Heartbeat-Filter, Unmount-Cleanup.
- `NotificationBell` invalidiert React-Query-Cache bei jedem Stream-Event statt 60-s-Polling. `useNotifications.refetchInterval` entfernt.

**Docs (S17-6):**
- `docs/wiki/reverse-proxy.md` — nginx-Konfig für SSE-Endpoints (`proxy_buffering off`, `proxy_read_timeout 1h`, `location ~ ^/api/v1/.+/stream$`-Block). Caddy/Traefik/HAProxy/Cloudflare-Hinweise. Liste aller aktiven SSE-Endpoints.

**Tests (S17-8):**
- `parseSSEFrames`-Helper in `notifications_stream_test.go` — testbarer SSE-Frame-Parser mit 5 Unit-Tests (single-frame, ping-heartbeat, mixed-stream, empty, DONE-marker).

**Verschoben (S17-5 [~]):**
- `ScanProgressIndicator`-Frontend-UI als Cosmetic-Polish nach Sprint 18 verschoben. Backend-Pub/Sub-Infra produktiv, Hook-Pattern aus S17-3 wiederverwendbar.

### Sprint 16 — Frontend-Polish + Doku-Reife (Tag-Kandidat v0.9.0)

Sprint 16 schließt die Reife-Sanierung-Welle 2 strukturell ab. Schwerpunkt: Frontend-Hygiene + Doku-Vollständigkeit, keine API-Breaking-Changes.

**Doku-Wave (S16-5..9):**
- `docs/GLOSSARY.md` neu — Compliance-Vokabular (Control, Evidence, Framework, Finding, Risk, Incident, Cross-Module-Evidence, SoA, TOM, VVT, DPIA, AVV, DSR) + Vakt-Architektur-Begriffe (Modul, Service, Shared, Demo-Flow, safego.Run, Public Mirror).
- `docs/concepts/` Subdir mit `module-isolation.md`, `evidence-collection.md`, `rbac-model.md`, `demo-flow.md`. Narrative Erklärungen zur Architektur, komplementär zu den ADRs.
- `docs/api-versioning-policy.md` — Breaking-Change-Definition, 6-Monats-Deprecation-Window, CI-Enforcement-Plan, Sonderfälle für Security-/Legal-Pflichten.
- `docs/wiki/admin-cli.md` — vollständige Doku zu `vakt-admin` CLI (`health-check`, `list-orgs`, `list-users`, `reset-password`).
- `docs/adr/0019-sse-statt-websocket-fuer-realtime.md` Accepted — Server-Sent Events als Pflicht-Transport für alle Realtime-Pfade, WebSockets bewusst ausgeschlossen.

**Frontend-Polish (S16-1, S16-3, S16-10, S16-2):**
- **Severity-Farben als Design-Tokens** — Tailwind `theme.colors.severity.{critical,high,medium,low,info}` + `*-bg`-Varianten. Alle hardcoded `bg-[#hexhex]`-Bracket-Notations bereinigt (0 verbleibend). Whitelabel-Theme-Vorbereitung.
- **Code-Splitting** — alle Settings-/Admin-Pages auf `React.lazy()` umgestellt; Layout wrapped Outlet in Suspense. Eager bleiben Login/Setup/Dashboard + Token-Magic-Link-Pages (Auditor/Policy/Invite/DSR). Größter einzelner Chunk: `SecVitalsRoutes 452 kB` (gzip 105 kB) — unter Warning-Threshold.
- **`useFormatDate`-Bulk-Migration** — 60 Files mit hardcoded `toLocaleDateString('de-DE', ...)` / `toLocaleString('de-DE')` auf `formatLocale()` (neuer non-Hook-Helper) migriert. Hook-Variante `useFormatDate` (Sprint 13) bleibt für reaktive Komponenten verfügbar. 0 verbleibende Stellen.
- **openapi-typescript Client-Generierung** — `npm run api-types` generiert `frontend/src/api/generated.ts` (7018 LOC) aus `openapi.yaml`. CI-Step `api-types:check` enforced Drift (ADR-0017). `Login.tsx` als Demo-Migration nutzt jetzt `components['schemas']['LoginResponse']` statt Manual-Interface.

**Skip-Item:**
- S16-4 Bundle-Audit verschoben — `vite build` Chunk-Size-Warning erfüllt den Monitoring-Zweck; echte Tree-Shake-Optimierung lohnt sich erst nach Recharts/framer-motion-Bereinigung in einer Q3-Polish-Welle.

### Sprint 15 — AI-Härtung + Observability + Welle 2 (Tag-Kandidat v0.8.0)

Sprint 15 schließt die Backend-Stabilität (Sprint 14) ab und liefert produktreife AI-UX + Observability-Default-On.

**AI-Härtung (S15-1 bis S15-5):**
- Neue Tabelle `ai_usage` (Migration 124) trackt Tokens, Kosten (micro-EUR), Dauer und Status pro AI-Call. Konfigurierbare Tagesquota via `VAKT_AI_DAILY_TOKEN_LIMIT_PER_ORG`.
- Redis-basiertes Rate-Limit per Org (Default 30 req/min, `VAKT_AI_RATE_LIMIT_RPM`). Bei Verstoß `429 AI_RATE_LIMITED`.
- Response-Cache mit sha256(model+messages)-Key, TTL via `VAKT_AI_CACHE_TTL_SECONDS` (Default 1h). Cache-Hits werden als `cache_hit`-Status persistiert.
- Prompt-Injection-Schutz: strikte System/User-Role-Trennung in `buildMessages` — User-Input landet niemals im System-Prompt-Concat. Unit-Test deckt den Pfad ab.
- Neuer Endpoint `POST /api/v1/vaktcomply/ai/chat/stream` mit Server-Sent-Events: OpenAI-konforme `data: {"content":"..."}` Frames, `data: [DONE]`-Terminator, X-Accel-Buffering-Off für nginx.

**AI-UX Frontend (S15-6 bis S15-9):**
- `useAIStream` Hook konsumiert SSE-Frames inkrementell; bietet `text`, `isStreaming`, `error`, `durationMs`, `start(req)`, `stop()`. AbortController + Unmount-Cleanup.
- `LocalLLMBadge` zeigt sichtbar "Lokal · qwen2.5:3b" (No-Phone-Home-Differential) vs "Cloud · gpt-4o-mini" je nach Provider.
- `TokenCostIndicator` mit kompakter `1.2k Tk · 0.02 € · 4.3 s`-Anzeige nach Streamende.
- `AIAdvisor.tsx` als Demo-Migration: Live-Streaming-Rendering mit blinkendem Cursor, Stop-Button, Badge im Header, Cost-Indikator nach Abschluss. Rate-Limit/Quota-Errors bekommen spezifische i18n-Hints.
- i18n-Keys `ai.{localBadge,cost,stream}.*` in de/en/fr/nl.

**Observability default-on (S15-11 bis S15-15):**
- `MetricsEnabled` default `true` (opt-out via `VAKT_METRICS_DISABLED=true`); `/metrics` bleibt IP-allowlisted (Loopback + Docker-Netz).
- Prometheus + AlertManager im `docker-compose.observability.yml` Profil. `observability/prometheus.yaml` scrapt api + worker; `observability/alert-rules.yaml` mit 7 konservativen Default-Alerts (5xx-Rate, P95-Latency, Queue-Backlog, AI-Latency, …).
- 4 Grafana-Dashboards committed (`observability/dashboards/{api,worker,ai,demo}.json`) + Provisioning-Manifest. Beim Start automatisch unter dem Folder „Vakt" verfügbar.
- `alertmanager.example.yml` mit severity-basiertem Routing (critical→pager, warning→webhook, info→email-digest), Customer konfiguriert eigene Receiver — kein Phone-Home zu Norvik.
- `safego.SetPanicHandler` callback-Hook für optionale Sentry/3rd-party-Integration ohne externe Pflicht-Dependency.
- `docs/operations.md` Sektion 0 mit SLA-Matrix (RTO/RPO) für Container-Crash, Redis-Loss, DB-Korruption, Server-Verlust, K8s-Pod-Eviction, Region-Outage + PITR-/Hot-Standby-Empfehlungen.

**`internal/shared/` Konsolidierung Welle 2 (S15-10):**
- `internal/shared/{ai,alerting,evidence_auto,crossevidence}/` → `internal/services/*`. 17 Import-Call-Sites in 16 Files migriert, History via `git mv` erhalten.
- Neues `internal/services/README.md` dokumentiert die Boundary: `shared/` für Cross-Cutting-Concerns, `services/` für Cross-Module-Services mit eigener Domain-Logik. Welle-3-Kandidaten (scheduledreports, emaildigest, notifications) explizit als zukünftige Iteration markiert.

**Neue Env-Vars (Sprint 15):**

| Variable | Default | Bedeutung |
|---|---|---|
| `VAKT_AI_RATE_LIMIT_RPM` | 30 | Max AI-Calls pro Minute pro Org |
| `VAKT_AI_DAILY_TOKEN_LIMIT_PER_ORG` | 0 (aus) | Tages-Token-Quota pro Org |
| `VAKT_AI_CACHE_TTL_SECONDS` | 3600 | Response-Cache-TTL |
| `VAKT_AI_COST_PER_MTOKEN_IN_MICRO_EUR` | 0 | Kosten pro 1M Input-Tokens (0 = lokal) |
| `VAKT_AI_COST_PER_MTOKEN_OUT_MICRO_EUR` | 0 | Kosten pro 1M Output-Tokens |
| `VAKT_SENTRY_DSN` | leer | Optional Sentry-DSN; aktiviert PanicHandler-Hook |
| `VAKT_METRICS_DISABLED` | false | Opt-Out für /metrics (vorher: opt-in via VAKT_METRICS_ENABLED) |

### Sprint 13 — Reife-Sanierung Welle 2 abgeschlossen (Tag-Kandidat v0.7.0)

Befunde aus der zweiten Elite-Review (Mai 2026, archiviert unter `docs/reviews/2026-05-elite-review/`, Verify-Pass `docs/reviews/2026-05-bericht-verify.md`). 28/29 P0-Items erledigt; ein Bulk-Migration-Item (`useFormatDate`-Roll-out) verschoben in Sprint 16 (S16-10).

#### Sicherheit

- **SSRF-Guard für `VAKT_AI_BASE_URL`** — neue URL-Validierung beim Startup blockt IMDS (169.254.169.254), Loopback (127.0.0.0/8, ::1), Link-Local (169.254.x, fe80::/10) und `localhost` als Hostname, wenn `VAKT_AI_PROVIDER != "disabled"`. Allowlist für Container-Service-Discovery (`ollama`, `ai-llm`, `llm-proxy`, `lm-studio`) + alle Public-DNS-Hostnames. 22 Testfälle in `backend/internal/config/ai_base_url_test.go`.
- **LemonSqueezy Webhook-Replay-Schutz** — neue Migration `123_lemonsqueezy_webhook_events.{up,down}.sql` deduped Webhooks auf sha256(body). Doppelter Body → 200 OK ohne erneute Verarbeitung. Vorher konnte ein wiederholter `subscription_created`-Event prinzipiell mehrfach E-Mails / License-Operationen triggern.
- **LemonSqueezy Startup-Warning** — `NewHandler` logt `Warn` wenn `VAKT_LS_WEBHOOK_SECRET=""`; ohne Secret weist jede Signaturprüfung den Request ab.
- **bcrypt Cost-Upgrade-on-Login** — Login-Pfad prüft `bcrypt.Cost(hash)` und re-hasht transparent auf cost 12, wenn ein Legacy-Wert kleiner war. Update ist Best-Effort (Fehler nur Warn-Log), Login bleibt funktional.
- **Audit-Redaction erweitert** — `sensitiveKeys` in `audit/audit.go` enthält jetzt `recovery`, `backup`, `otp`, `mfa` zusätzlich zu `password`, `secret`, `token`, `key`. Felder wie `recovery_code` / `backup_code` / `totp_code` landen nicht mehr im Klartext im Audit-Log.
- **Trivy `ignore-unfixed: false`** im CI-Workflow (`backend` + `frontend` Scans). Unfixed-Akzeptanzen wandern in `.trivyignore` mit Begründung + Re-Check-Datum (Template enthalten).
- **gitleaks Per-Secret-Allowlist** — `.gitleaks.toml` nutzt jetzt `regexes` für konkrete Test-Konstanten (CI-Test-Hex, `admin1234demo`, `analyst1234demo`) statt pauschaler Pfad-Allowlist. Pfad-Liste auf wenige kontrollierte Dummy-Files reduziert (`.github/workflows/*.yml` und `docs/`, `Makefile` rausgeflogen).
- **Helm-Defaults verschärft** — `postgresql.auth.password` darf nicht mehr `"changeme"` sein UND muss ≥ 16 Zeichen lang sein (Honeypot-Default `MUST_BE_OVERRIDDEN` + `fail`-Hook in `_helpers.tpl`). `redis.auth.enabled` default `true` (vorher `false`). Siehe [UPGRADE.md v0.7.0](docs/UPGRADE.md) für Migrations-Hinweise.

#### Rebrand-Cleanup End-to-End

- **`helm/sechealth/` → `helm/vakt/`** — Verzeichnis umbenannt; alle 70 template-namespace-Definitionen (`define "sechealth.fullname"`, …) zu `vakt.*` migriert. Externe Konsumenten von `helm install ./helm/sechealth` müssen den Pfad anpassen — siehe UPGRADE.md.
- **`backend/cmd/sechealth/` entfernt** — legacy CLI-Binary, nicht in Makefile/Dockerfile referenziert, war Naming-Drift nach Rebrand.
- **`website/README.md`, `integrations/github-action/action.yml`, `integrations/gitlab-template.yml`** rebranded SecHealth → Vakt.
- **Frontend-Banner-Links** (`VersionBanner.tsx`, `TrustPage.tsx`) zeigen jetzt auf `github.com/norvik-ops/vatk` (Public Mirror).
- **`CLAUDE.md` Repo-Tree** aktualisiert (`sechealth/` → `vakt-app/`, `helm/sechealth/` → `helm/vakt/`).
- **`backend/cmd/admin/`** CLI `Use`-String + Beispiel-Outputs auf `vakt-admin` umgestellt.
- **Codekommentare + Default-Werte** in `vaktscan/handler.go` (PDF-Dateiname), `vaktcomply/policy_acceptance.go` (Default-From-Adresse), `vaktvault/git_scanner.go` (tmp-Dir-Prefix), `shared/notify`, `shared/dashboard/notifications.go`, `setup/handler_test.go`, `cmd/seed/main.go`, `frontend/src/hooks/useDashboard.ts`, `pkg/sdk/nodejs/{index.ts,package.json}` von `sechealth`/`SecHealth` auf `vakt`/`Vakt` umgestellt.
- **`docker-compose.demo.yml`** Header rebranded; statische Demo-Credentials-Kommentare entfernt (irreführend nach v0.6.2-Ephemeral-Refactor, Memory-Violation).
- **`.gitignore`** legacy-Patterns für gelöschtes Binary entfernt.

Bewusst belassen (Memory `project_rebrand` + ADR-0004): DB-Schema-Präfixe (`vb_`, `ck_`, `so_`, …), Docker-Image-`LEGACY_PREFIX`-Aliase (`ghcr.io/matharnica/sechealth/*`) für Watchtower-Backward-Compat, ADR-Historien-Texte, Memory-Dateien, Operator-CRD-Name `SecHealthSecret` (Kubernetes-API-Breaking-Change, separate Welle).

#### Stabilität

- **Silent SQL-Errors in `vaktcomply`** — alle 14 Stellen mit `_ = s.db.QueryRow(...).Scan(...)` durch sichtbare `err`-Pfade ersetzt. Neuer Helper `fetchOrgName(ctx, db, orgID)` in `vaktcomply/orgname.go` mit Warn-Log statt stillem Drop. Composite-Queries (`service_frameworks` Milestone-Dedup, `service_reporting` 30-Tage-Counter, `handler_boardreport` Score-History + Incidents-30d) loggen jetzt explizit; Milestone-Dedup bricht bei DB-Fehler defensiv ab statt Doppelversand.

#### PRD & Doku-Wahrheit

- **PRD aktualisiert** (`docs/prd.md`): Jira-FR-VB06 entfernt (v0.5.2-Realität), Success-Metric "first paying managed-cloud customer" → ADR-0008-konform formuliert ("First 10 self-hosted Pro customers"), Setup-Zeit "< 3 min" → "≤ 5 min Plattform + 3–30 min Ollama-Pull". MSP-Tertiary-Audience neu beschrieben (per-customer-instance, kein zentrales Portal). Epic E16 "MSP Multi-tenancy" gestrichen.
- **`CONTRIBUTING.md`** neu — Branch-/Commit-Stil, Test-Erwartung gemäß ADR-0012 (kein 80%-Quoten-Diktat), ADR-Prozess, PR-Workflow, Pre-Release-Smoke-Test gemäß ADR-0017, Security-Disclosure-Adresse, explizite "NICHT-Annahme"-Liste (MSP-Portal, Phone-Home, Cloud-SaaS-Integrationen).
- **`.github/ISSUE_TEMPLATE/{bug,feature,security}.yml`** + **`.github/PULL_REQUEST_TEMPLATE.md`** + **`CODEOWNERS`** neu.
- **`frontend/README.md`** komplett neu — Stack, Modul-Struktur, Dev-Befehle, wichtige Hooks/Patterns, Frontend↔Backend-Vertrag.
- **CHANGELOG-Fragment-Konsolidierung** — `docs/CHANGELOG-{sprint3,sprint4,sprint5,launch-readiness,security-wave-may26,session-2026-05-20}.md` nach `docs/history/` verschoben mit Index-README. Root-`CHANGELOG.md` bleibt Single-Source-of-Truth.
- **`CLAUDE.md`** 80%-Coverage-Satz zu ADR-0012 (risikobasiert statt Quote) konsistent gemacht.

#### Frontend-Quick-Polish

- **Demo-Login-Fail-Toast** (`Login.tsx`) — `/api/v1/demo/start`-Fehler → sichtbarer Error-Toast statt stillem UI-Zerfall. i18n-Schlüssel `auth.demoUnavailable` in allen 4 Locales.
- **`useFormatDate`-Hook** (`shared/hooks/useFormatDate.ts`) liefert `formatDate`, `formatDateTime`, `formatTime`, `formatRelative` für aktive i18n-Locale (BCP47-Mapping `de/en/fr/nl`). Demo-Migration in `AdminSecurityPage` + `SecVitalsOverviewPage`. Bulk-Migration der verbleibenden ~60 Treffer in Sprint 16 (S16-10).
- **Hardcoded deutsche Microcopy** `"Demo wird vorbereitet…"` → i18n-Schlüssel `auth.demoPreparing` in allen 4 Locales.
- **`useErrorMessage`-Hook** (`shared/hooks/useErrorMessage.ts`) — i18n-bewusster Wrapper um `humanizeError`. Bevorzugt `errors.<CODE>`-Lookup über die Locales, fällt auf bestehende Substring-Map zurück. Locale-Keys für `AUTH_INVALID_CREDENTIALS`, `AUTH_BAD_REQUEST`, `AUTH_VALIDATION_ERROR`, `AUTH_INVALID_STATE`, `AUTH_TOKEN_REVOKED`, `AUTH_OIDC_NOT_CONFIGURED`, `AUTH_OIDC_FAILED`, `ACCOUNT_LOCKED`, `RATE_LIMITED`, `GENERIC` in `de/en/fr/nl`.

### Geändert

- **[ADR-0018](docs/adr/0018-goroutine-lifecycle-und-panic-eskalation.md)** (Accepted) — Goroutine-Lifecycle (Parent-Context-Pflicht) und Panic-Eskalation via `safego.Run`. Pflicht-Pattern für alle `backend/internal/`-Goroutinen ab Sprint-14-Migration; golangci-lint-Regel blockt neue Verstöße.

### Behoben

- **`/health` enthält jetzt `demo`, `sso_enabled`, `version`** — Frontend (`useDemoMode`) las diese Felder, Backend lieferte sie nicht. Effekt: `isDemo` war auf `secdemo.norvikops.de` immer `false`, die Demo-Credentials-UI wurde nie eingeblendet.
- **`POST /auth/login` enthält jetzt das `user`-Objekt** (`id`, `email`, `display_name`, `roles[]`) — Frontend (`Login.tsx → setAuth(data.user)`) crashte mit `can't access property "id"` direkt nach erfolgreichem Login, weil das Feld fehlte.
- **OpenAPI-Spec auf realen Stand gebracht** — `LoginResponse`-Schema hatte `token`/`name`/`role` während Code längst `access_token`/`display_name`/`roles[]` nutzte. `/health` hatte gar kein Response-Schema. Beides angepasst.
- **Demo-Banner zeigt keine fake Credentials mehr** — `Layout.tsx` und i18n-Locales (de/en/fr/nl) hatten weiterhin `admin@vakt.local / admin1234` im Demo-Banner, was nach dem Ephemeral-Refactor irreführend war.

### Geändert

- **[ADR-0017](docs/adr/0017-api-contract-tests.md)** — Strategie gegen Backend/Frontend-Drift: OpenAPI-Schemas für alle Frontend-konsumierten Endpoints sind verbindlich, Contract-Tests + Type-Generation als Ziel-Architektur, Maintainer-Checkliste in `docs/dev/api-contract-checklist.md` als Übergang.
- **[ADR-0016](docs/adr/0016-public-mirror-via-script.md)** — Public Mirror per Script (`scripts/build-public-mirror.sh` + `make public-mirror`) statt inline rsync im CI. Eingebauter `go build ./...`-Check verhindert Bugs wie den v0.6.1-Excludes-Bug.

---

## [v0.6.2] — 2026-05-20

### Behoben

- **Demo-Login funktioniert wieder** — Backend `/api/v1/demo/start` gibt jetzt die generierten ephemeren Random-Passwörter (16 hex chars, admin + analyst) im Response zurück. Frontend `Login.tsx` nimmt sie und füllt die Login-Form vor. Vorher hatte das Frontend ein hardcodiertes `admin1234` als Default-Passwort, das (a) nicht den tatsächlich erzeugten Random-Hashes entsprach und (b) seit Erhöhung der Mindestpasswortlänge auf 10 Zeichen nicht mehr durch die Auth-Validierung kommt. Demo war dadurch unbenutzbar.
- **Statischer Demo-Seed nutzt 10+ Zeichen-Passwörter** — `demoseed.Run()` (für lokale Dev-Setups) setzt jetzt `admin1234demo` / `analyst1234demo`. Der frühere 9-Zeichen-Default (`admin1234`) wurde von der Auth-Validierung (min 10) abgelehnt.
- **Public Repo `norvik-ops/vatk` kompiliert wieder** — der Sync-Workflow hatte `internal/shared/demo/`, `demoseed/`, `feedback/` exkludiert, aber `cmd/api/main.go` importierte sie weiterhin. Wer die Codebase aus dem Public Repo baute, erhielt `no required module provides package …`-Fehler. Die drei Packages sind jetzt im Public Repo enthalten — sie sind hinter `if cfg.DemoSeed` gegated und ändern bei Customer-Default-Installs (VAKT_DEMO=false) das Verhalten nicht.

### Geändert

- **Doku zum Demo-Modus richtiggestellt** — `CLAUDE.md`, `docs/wiki/demo-mode.md`, `docs/setup.md`, `docs/configuration.md`, `docs/public/README.md`, `docs/launch-producthunt.md` und CI-Sync-Workflow dokumentieren jetzt einheitlich: Demo-Logins sind ephemer pro Visitor (Random-Slug, Random-Passwort, 4 h Lebensdauer), niemals statisches `admin@vakt.local / admin1234`.

### Lint / Hygiene

- **golangci-lint v2.12.2** statt v1.x — neuer config-Schema (`linters.settings`, `linters.exclusions.rules`), passend zu Go 1.25 build-toolchain
- **105 vorbestehende Lint-Verstöße bereinigt** — errcheck-Exclusions für idiomatische `defer X.Close()` Patterns, sinnvolle staticcheck-Ausnahmen für deutschsprachige Codebase, echte Bugfixes in `vaktcomply/reportpdf.go` (ungenutzte status-Variable in SoA-PDF jetzt im richtigen Feld dargestellt) und `alerting/service.go` (labeled `break` für korrekten Abbruch der Retry-Schleife bei ctx-cancel)

### Branding

- **Landing-Pages aktualisiert** — `vakt.norvikops.de`: Pro-Features auf v0.6.1-Stand (KI-Berater raus, AI Copilot Community rein, 6 Module statt 5, NIS2-Meldungsassistent + Lieferantenportal als Pro ergänzt), Enterprise-Sales-Block entfernt, Datenschutz „SecHealth" → „Vakt"; `norvikops.de`: Meta-Description + Form-Placeholder rebranded

---

## [v0.6.1] — 2026-05-20

> **⚠️ Upgrade-Hinweis für Bestandskunden:** Diese Version startet Ollama (AI Copilot)
> automatisch mit `docker compose up` (vorher hinter `--profile ai` versteckt). Der
> Ollama-Container lädt beim ersten Start einmalig das Modell `qwen2.5:3b` (~1.9 GB
> Download, ~2 GB RAM-Live-Footprint, 4 GB Limit). Auf VMs mit weniger als 8 GB
> Gesamt-RAM bitte VOR dem Upgrade `VAKT_AI_PROVIDER=disabled` in `.env` setzen
> und in einer Compose-Override-Datei den `ollama`/`ollama-init`-Service entfernen.
> Plattform-Startup-Zeit unverändert (<5 Min); AI-Funktionen sind 3–30 Min später
> verfügbar, abhängig von Internet-Bandbreite (1.9 GB Modell-Download).

### Geändert

- **AI-Copilot ist Community** — Die fünf AI-Endpunkte (`/vaktcomply/ai/status`, `/ai/report`, `/ai/advice`, `/ai/draft-policy`, `/ai/incident-guide` sowie `/vaktcomply/policies/generate-draft`) sind ab sofort in jeder Vakt-Instanz nutzbar — kein `FeatureAIAdvisor`-Pro-Gate mehr. Mit qwen2.5:3b als Default-Modell (Apache 2.0, ~1.9 GB RAM, CPU-tauglich) läuft die AI lokal auf jeder VM; ein Lizenz-Gate hatte daher nur Marketing-Charakter ohne echten Schutz. Premium-Compliance-Features (TISAX, DORA, NIS2-Reporting, EU-AI-Act, AuditPDF, SSO, API-Access, SecReflex/SecPulse-Advanced, Granular-Permissions, Supplier-Portal) bleiben Pro. `FeatureAIAdvisor`-Konstante bleibt für Lizenz-Validierung erhalten, wird aber nicht mehr im Routing geprüft.
- **Ollama default-on, Auto-Model-Pull** — `ollama` Service ist nicht mehr hinter `profiles: ["ai"]` versteckt; startet automatisch mit `docker compose up`. Neuer Init-Container `ollama-init` zieht das Default-Modell `qwen2.5:3b` einmalig beim ersten Start (idempotent — bei vorhandenem Modell No-Op). Damit ist AI nach einem einzigen `docker compose up` lauffähig — kein `--profile ai`, kein manueller `ollama pull` mehr. Resource-Limit auf Ollama: 4 GB RAM / 2 vCPU. Customers auf VMs mit < 8 GB Gesamt-RAM können via `VAKT_AI_PROVIDER=disabled` + compose-override deaktivieren.
- **Helm-Chart Ollama-Integration** — Neue Templates in `helm/sechealth/templates/ollama/`: StatefulSet mit PersistentVolumeClaim (10 Gi default), ClusterIP-Service, Helm-Hook-Job für das einmalige Modell-Pull. Default-on via `ollama.enabled: true` in `values.yaml`. Die ConfigMap setzt `VAKT_AI_BASE_URL` automatisch auf den Cluster-internen Ollama-Endpoint, oder erlaubt Override für externe LLM-Quellen (z.B. Mistral EU). Resource-Defaults: 500m CPU / 2 GiB Memory request, 2 / 4 GiB limit.
- **Vakt Aware vollständig sqlc-migriert** — Tabellen-Präfix `pg_*` → `sr_*` (Migration 122, reine Metadaten-Operation in Postgres). Damit konnte sqlc die Tabellen parsen und alle 35 Repository-Methoden auf den generierten Code umgestellt werden. Vakt Aware war das letzte Modul mit embedded SQL. **ADR-0005 schließt damit ab — alle Module nutzen sqlc.**

### Sicherheit

- **CSRF Double-Submit-Cookie** — alle state-ändernden Endpoints unter `/api/v1` sind jetzt zusätzlich zu SameSite=Strict per expliziten Token gegen CSRF geschützt; Backend setzt `csrf_token` Cookie bei Login/Refresh/OIDC/SAML, Frontend echot ihn als `X-CSRF-Token` Header
- **Helm Pod-Security** — `podSecurityContext` mit `runAsNonRoot: true`, UID 65532, fsGroup 65532; `containerSecurityContext` mit `readOnlyRootFilesystem: true`, `allowPrivilegeEscalation: false`, alle Capabilities gedroppt, seccomp `RuntimeDefault` für API und Worker; Frontend mit minimal nötigen Anpassungen für nginx
- **Verschlüsselung at-Rest dokumentiert** — neue `docs/encryption-at-rest.md` mit drei Pfaden (LUKS, Cloud-Provider, pgcrypto) und Installations-Checklist für DSGVO Art. 32
- **Redis-backed Org-Rate-Limiting** — fixed-window INCR/EXPIRE statt in-memory token-bucket; multi-replica-sicher für HA-Deployments
- **OIDC/SSO CSRF-Schutz** — OAuth2 `state`-Parameter wird jetzt serverseitig validiert (One-Time-Use via Redis, 10 min TTL); verhindert Login-CSRF-Angriffe
- **TOTP Deny-List** — ausgeloggte Paseto-Tokens waren auf 2FA-Endpunkten weiterhin gültig; Redis-Deny-List greift jetzt auch auf `/auth/2fa/*`-Routen
- **TOTP Replay-Schutz** — derselbe 6-stellige Code konnte innerhalb des 90-Sekunden-Fensters mehrfach eingesetzt werden; jetzt per Redis SetNX gesperrt
- **`RevokeAllOtherSessions`** — widerrief fälschlicherweise auch die eigene Session; eigene Session wird jetzt via `token_hash` ausgeschlossen
- **MFA-Enforcement Fail-Closed** — ein DB-Fehler beim MFA-Pflicht-Check ließ Requests kommentarlos durch; gibt jetzt HTTP 503 zurück
- **DSR-Portal** — öffentlicher Status-Endpunkt gab interne DPO-Notizen und org_id zurück; gibt jetzt nur noch `id`, `status`, `type` und Timestamps zurück
- **Setup-Handler Passwortvalidierung** — initiales Admin-Passwort konnte kürzer als 10 Zeichen sein; jetzt identisch mit der regulären Passwort-Policy
- **SMTP** — Port 465: implizites TLS (`tls.Dial`); Port 587: STARTTLS; keine Klartext-Credentials mehr
- **Webhook-RBAC** — Webhook-Endpunkte hatten keine Rollenprüfung; `List`/`Test` → `SecurityAnalyst+`, `Create`/`Update`/`Delete` → `Admin`
- **SSRF-Schutz** — Scanner-Targets (Trivy, Nuclei) werden gegen RFC-1918, Loopback und Link-Local geprüft; opt-out via `VAKT_SCAN_ALLOW_PRIVATE=true`
- **CSP** — `style-src` in `style-src-elem 'self'` (blockiert `<style>`-Injection) und `style-src-attr 'unsafe-inline'` (nur Inline-Attribute, nötig für UI-Framework) aufgeteilt
- **IP-Forwarding** — `X-Forwarded-For` wird nur noch ausgewertet wenn `VAKT_TRUSTED_PROXIES` gesetzt ist; verhindert IP-Spoofing bei direkter Installation

### Hinzugefügt

- **Session-Verwaltung pro Gerät** — neue Seite „Aktive Sitzungen" unter Einstellungen: alle angemeldeten Geräte einsehen und einzeln abmelden (`GET /auth/sessions`, `DELETE /auth/sessions/:id`)
- **Startup-Warnungen** — strukturierte Warn-Logs beim Start wenn HTTP statt HTTPS (`VAKT_FRONTEND_URL`) oder Demo-Modus aktiv (`VAKT_DEMO=true`)

### Infrastruktur

- **Nicht-Root-Container** — API, Worker und Migrate laufen jetzt als `nonroot` (UID 65532, distroless/static); kein Root-Prozess im Container
- **Go Healthcheck-Binary** — statisch kompiliertes `/healthcheck`-Binary ersetzt busybox-Abhängigkeit im distroless-Image; Docker-Healthcheck funktioniert ohne Shell
- **`VAKT_CORS_ORIGINS`** — CORS-Origins sind jetzt konfigurierbar (kommasepariert); Default `*`, Dokumentation in `.env.example` ergänzt

### Dokumentation & Architektur

- **Architecture Decision Records** — neuer `docs/adr/` Verzeichnis mit 12 retrospektiven ADRs: Self-Hosted-Prinzip, ELv2-Lizenz, Paseto-Wahl, Modul-Isolation, sqlc-Strategie, Anonymisierung statt Hard-Delete, Betriebsrat-Modus, MSP-Verzicht, OpenAPI-Single-Source-of-Truth, AES-256-GCM, OTel-Opt-in, Test-Coverage-Pragmatik

### Observability (opt-in)

- **OpenTelemetry-Instrumentation** — `internal/shared/telemetry/` initialisiert OTel beim Start, aktiviert sich aber nur bei explizit gesetztem `OTEL_EXPORTER_OTLP_ENDPOINT` (keine versteckten Telemetrie-Pfade, siehe ADR-0011)
- **Observability-Stack** — neue `docker-compose.observability.yml` Profile mit Loki + Promtail + Tempo + Grafana; aktivieren via `docker compose --profile observability up`; `docs/observability.md` mit Volumen-Schätzungen und Sicherheits-Hinweisen

### AI-Copilot

- **Default-Modell auf `qwen2.5:3b` umgestellt** — Apache-2.0-Lizenz statt Llama-Community, ~10 % weniger RAM-Footprint, schneller auf CPU, bessere Deutsch-Performance; alternative Modelle dokumentiert (`llama3.2:1b`, `phi3.5:mini`, `gemma2:2b`, `qwen2.5:7b`)
- **Policy-Drafting** — `POST /vaktcomply/ai/draft-policy` generiert einen Richtlinien-Entwurf in Markdown für ein Thema; Admin reviewt und veröffentlicht
- **Incident-Response-Guide** — `POST /vaktcomply/ai/incident-guide` erstellt aus einer Vorfalls-Beschreibung eine nummerierte Sofort-Checkliste mit gesetzlichen Fristen (NIS2, DSGVO Art. 33, DORA); im Frontend per „KI-Sofortmaßnahmen"-Button in der Vorfalls-Detailansicht direkt anwendbar
- **Wiki + Landingpage-Briefing** — neue `docs/wiki/ai-features.md` mit System-Requirements-Tabelle, Modell-Vergleich, DSGVO-Statement und Mistral-EU-Konfiguration; `docs/landingpage-ai-briefing.md` mit Headlines, Use-Cases und Vergleichstabelle gegen Vanta/Drata für die Marketing-Seite

### Refactor & Tests

- **HR-Service Pattern-Migration** — Audit-Logging vom Handler in den Service verlagert (P2-19/P2-20-Pattern); HR-Service ist jetzt vollständig SDK-fähig — Audit-Trail bleibt intakt auch bei Aufrufen aus Worker-Jobs oder künftigen CLI-Tools
- **sqlc Start für Vakt Vault** — Projects/Environments/AccessLog als sqlc-Queries (`db/queries/vaktvault.sql`); Secrets-Tabelle bleibt embedded SQL wegen Crypto-Spezifika
- **sqlc VVT (Vakt Privacy)** — Verzeichnis von Verarbeitungstätigkeiten (DSGVO Art. 30) komplett auf sqlc umgestellt; DPIA / AVV / Breach / DSR folgen in Folge-Sitzungen
- **Frontend-Test-Coverage erhöht** — 16 neue Unit-Tests: apiFetch (CSRF + Retry + Error-Mapping), useFirstAction (Persistenz + Idempotenz), useMilestoneToast (Schwellen + Jump-Detection); 2 vorbestehende Test-Fails behoben
- **Bugfix MilestoneToast** — Score-Jump-Baseline wurde nicht aktualisiert wenn ein Schwellen-Toast feuerte, führte zu Phantom-Toasts beim Remount; durch Test entdeckt und behoben
- **Integration-Test mit testcontainers-go** — echter End-to-End-Test für Vakt HR → Vakt Comply Evidence-Flow (`internal/integration_test/hr_evidence_real_test.go`); läuft in CI mit Docker-Daemon, skippt sauber wenn nicht verfügbar

### Datenschutz (DSGVO)

- **Recht auf Datenübertragbarkeit** (Art. 20) — neuer Endpoint `GET /api/v1/account/data-export` liefert ein ZIP-Archiv mit allen persönlichen Daten des Nutzers (Profil, Sessions, API-Keys-Metadaten, eigene Audit-Log-Einträge, eigene Kommentare, Benachrichtigungseinstellungen) als maschinenlesbare JSON-Dateien
- **Recht auf Löschung** (Art. 17) — neuer Endpoint `POST /api/v1/account/delete` mit Passwort-Re-Auth und expliziter „LÖSCHEN"-Bestätigung; Konto wird in der Datenbank anonymisiert (E-Mail, Name, Avatar geleert; Sessions + API-Keys widerrufen) statt hart gelöscht, um die Audit-Trail-Integrität gemäß ISO 27001 A.5.28 / BSI ORP.2 zu wahren; verhindert versehentliches Orphaning einer Organisation (letzter Admin → 409)

### UX-Verbesserungen

- **SlideOver-Komponente** — neue `SlideOver` für Linear-Style Detail-Panels mit framer-motion-Animation, Focus-Trap und Escape-Handling; nutzbar für Control-, Risiko- und Finding-Details ohne Kontextverlust
- **Micro-Guidance** — beim ersten Anlegen eines Risikos, Vorfalls, einer Richtlinie oder eines Assets erscheint ein einmaliger Hinweis mit Folge-Aktion-Empfehlung (z.B. „Control angelegt — als Nächstes Evidenz hochladen")
- **Role-basiertes Onboarding** — der Setup-Wizard zeigt nur die Schritte, die für die Rolle des angemeldeten Nutzers relevant sind: Admins sehen alle 4 Schritte, SecurityAnalysts nur die 2 Arbeits-Schritte (Control + Risiko), Viewer/Auditor sehen den Wizard gar nicht
- **Formular-Validierung erweitert** — `useFormValidation` unterstützt jetzt Cross-Field-Validation (`custom`-Callback) und scrollt + fokussiert automatisch das erste fehlerhafte Feld

### Hinzugefügt

- **OpenAPI 3.0 Spec — Single Source of Truth** — `backend/internal/shared/apidocs/openapi.yaml` wird zur Build-Zeit in den API-Server embedded; vorher lieferte der Server eine separate hardcoded Go-Spec mit nur 10 Endpoints, jetzt 75+. CI-Gate (`spec_test.go`) prüft YAML-Validität und blockiert PRs, die Pflicht-Endpoints aus der Doku entfernen. Spec ist über `GET /api/v1/openapi.yaml` und Swagger-UI unter `/api/docs` erreichbar. Kunden können daraus eigene SDKs generieren oder Automatisierungs-Skripte schreiben.
- **Frontend-Error-Tracking** — JS-Errors aus dem ErrorBoundary werden in der Tabelle `client_errors` persistiert; Admins sehen die letzten 200 Errors unter `GET /admin/client-errors` (org-scoped, self-hosted, kein externer Dienst)
- **Vakt Aware Content-Library** — 10 DACH-spezifische Phishing-Templates (CEO-Fraud, IT-Helpdesk, DHL, Microsoft-MFA, Mahnung, OneDrive, Sparkasse-SMS, USB-Köder, ...) + 5 vorgefertigte Trainings-Module abrufbar über `GET /api/v1/vaktaware/templates/presets` und `GET /api/v1/vaktaware/training-modules/presets`
- **Vakt Aware Anonymisierungs-Garantie** — Bei `betriebsrat_mode=true` werden IP-Adresse und User-Agent **gar nicht erst** in die DB geschrieben (statt nur im PDF-Export ausgeblendet) — DSGVO Art. 5 (1c) Datenminimierung + §87 BetrVG-konform; Wiki dokumentiert die rechtliche Begründung

### Datenbank

- Migration `117`: `refresh_sessions` — Tabelle für Refresh-Tokens mit Device-Info und Widerruf pro Gerät
- Migration `118`: `ck_evidence.control_id` nullable + neue Tabelle `hr_run_events` für Vakt HR Step-Audit-Trail
- Migration `119`: `client_errors` — Tabelle für persistierte Frontend-Errors

---

## [0.38.0] — 2026-06-09

**ISB-Vollständigkeit — Notfallhandbuch (BCP), Schutzbedarfsfeststellung, Berechtigungskonzept.**
Drei neue Feature-Bereiche runden die ISB-Checkliste ab. Alle drei sind vollständig versioniert und erzeugen audit-fähige Nachweise in Vakt Comply.

### Added

- **Notfallhandbuch / BCP** (`Vakt Comply`) — Verwaltung von Business-Continuity-Plänen mit Status-Workflow (draft → active → archived), versionierten Plänen und zugeordneten Wiederanlauftests. Jeder Test dokumentiert Datum, Typ (tabletop / walkthrough / fulltest) und Ergebnis (passed / failed / partial). Pläne ohne Test in den letzten 12 Monaten werden mit einem Amber-Banner hervorgehoben. Pläne können direkt als Compliance-Nachweis in Vakt Comply verlinkt werden.
- **Schutzbedarfsfeststellung** (`Vakt Comply`) — CIA-Triade-Bewertung (Vertraulichkeit, Integrität, Verfügbarkeit) nach BSI-Maximumprinzip. Schutzklassen: `normal`, `hoch`, `sehr_hoch`. Gesamtbedarf wird automatisch als Maximum der drei Dimensionen berechnet. Einträge können finalisiert (eingefroren) werden — danach keine Änderungen mehr möglich. Unterstützte Objekttypen: Prozess, System, Information, Standort.
- **Berechtigungskonzept** (`Vakt HR`) — Verwaltung von Berechtigungskonzepten mit Rollenmatrix pro Konzept. Zugriffsrollen dokumentieren System, Zugriffsebene (`read / write / admin / no_access`), Begründung und Wiederprüfungsintervall. Konzepte können per „Version einfrieren" als unveränderlicher Schnappschuss gesichert werden; die Versionshistorie ist vollständig einsehbar.

### Infrastructure

- **`promote.yml` mit automatischem Deploy** — Der promote-Workflow kopiert Images jetzt auf `:latest` **und** `:demo` (Server nutzt `APP_VERSION=demo`) und fährt danach den Demo-Server direkt auf dem Self-Hosted Runner hoch (`docker compose pull` → migrate → worker → api → health-check → frontend). Kein manueller SSH-Schritt mehr nötig.

---

## [0.37.0] — 2026-05-29

**Mega-Audit-Welle — VPS-Hardening, Code-Security-Fixes, CI-Hygiene.** Zweites Agent-Audit (2026-05-29) mit 5 VPS-Findings + 7 Code-Findings + 3 Hardening-Items. Alle Wave A/B/C-Items adressiert; CI durchgehend grün (Backend, Frontend, Integration, Deploy, E2E).

> **Operative Hinweise:** `VAKT_SECRET_KEY` auf dem VPS rotiert — bestehende verschlüsselte DB-Einträge bleiben lesbar (HKDF-Migration ist idempotent; `cmd/rotate-key` war in 0.36.0 abgesichert). UFW aktiv auf dem VPS; Zabbix-Agent (Port 10050) und -Proxy (Port 10051) sind in den Allow-Rules explizit gesichert. `VAKT_PROMOTE_SECRET` aus der systemd-Unit in `/etc/vakt-promote.env` (chmod 600) ausgelagert.

### Security

- **VPS Secret-Key rotiert** — neuer kryptografisch zufälliger 32-Byte-Key; `docker compose up -d` propagiert den neuen Key ohne Downtime.
- **Firewall aktiviert (UFW)** — Default deny-incoming, explizite Allows für SSH (22), HTTP/S (80/443), Zabbix-Agent (10050 von dirserver), Zabbix-Proxy (10051 von dirserver), Prometheus-Scrape.
- **VAKT_PROMOTE_SECRET rotiert + gehärtet** — Secret aus systemd-Unit-inline in `EnvironmentFile=/etc/vakt-promote.env` (chmod 600) verschoben; kein Klartext mehr in `systemctl show`.
- **`.env` Berechtigungen** — chmod 600 auf `.env`; war zuvor world-readable.
- **Schwacher-Key-Guard** (`B1`) — `config.Validate()` verwirft Keys bei denen alle Bytes identisch sind (z.B. `0000…`). Fehler enthält Regenerierungshinweis.
- **Scanner-Image-Pinning** (`B3`) — Trivy (`0.62.0`) und Nuclei (`v3.4.4`) im Dockerfile per SHA-256-Digest gepinnt; verhindert stilles Tag-Overschreiben.
- **`err.Error()`-Leaks reduziert** (`B4`) — interne Fehlermeldungen in `cloud/handler.go`, `jobs_handler.go`, `vaktscan/handler.go`, `ai/handler.go`, `nis2wizard/handler.go` durch generische Meldungen ersetzt; Stack-Details nur im strukturierten Log.
- **`html/template` für E-Mail-Templates** (`B5`) — `vaktaware/service.go` und `vaktcomply/policy_acceptance.go` nutzen jetzt `html/template` statt `text/template`; Auto-Escaping verhindert XSS in kampagnen-generierten E-Mails.
- **TRUSTED_PROXIES-Warning** (`C3`) — Startup-Log-Warn wenn `VAKT_TRUSTED_PROXIES` nicht gesetzt; verhindert stilles IP-Spoofing hinter Reverse-Proxys.
- **In-Memory-Ratelimit-Warning** (`C7`) — Startup-Log-Warn wenn Redis nicht konfiguriert und In-Memory-Fallback aktiv ist; Multi-Replica-Deployment mit gespiegelten Limits ist damit erkennbar.

### CI / Tooling

- **Trivy-Image-Scan im Deploy-Step** (`C2`) — nach `docker build` scannt Trivy das frisch gebaute API-Image auf CRITICAL/HIGH; nicht-blockierend (exit-code 0), Report im Summary.
- **Fuzz `-parallel=1`** — Go 1.22+ gibt `context deadline exceeded` zurück wenn parallele Fuzz-Worker beim Budget-Ablauf nicht sauber stoppen. Einzel-Worker behebt das false-positive.
- **Vollständiges Paket-Rename** (`secX → vaktX`) — alle verbleibenden Handler, Query-Dateien, SQL-Go-Dateien, Worker-Handler und Test-Fixtures auf die neuen Modul-Namen umgestellt.

### Tests

- **`config/validate_test.go`** (neu) — 5 Tests für Weak-Key-Guard: Zero-Key, Repeat-Byte, valid Key, zu kurzer Key, fehlende DB-URL.
- **E2E-Fixes** — 3 Playwright-Tests repariert: `compliance` navigiert auf `/vaktcomply/frameworks` (Accordion versteckte Nav-Labels); `ExpiringEvidenceWidget`-Crash bei paginated Mock-Response durch Fixture-Mock behoben; Keyboard-Shortcut-Tests warten auf Layout-Mount vor Tastendruck.

---

## [0.36.0] — 2026-05-27

**Marktreife-Programm — Sprint 56–59 Sammel-Release.** Schließt die 11 Top-Findings aus dem Auditos-Singularity-9-Agent-Audit + alle daraus hervorgegangenen P1-Items und Content-Drifts. 15 neue ADRs (0033–0047), 3 Migrationen (149–151), Backend 33 Pakete + Frontend 482 Tests durchgehend grün.

> **Operative Hinweise:** Migrationen 149 (`audit_log`-Hash-Chain), 150 (RLS-Theater zurückgenommen) und 151 (`audit_log` Range-Partitioning auf `created_at`) sind additiv bzw. data-preserving. Migration 151 ändert den `PRIMARY KEY` von `(id)` auf `(id, created_at)` — anwendungsseitig transparent. Operator: optional `VAKT_AUTH_FAIL_OPEN_ON_REDIS_OUTAGE=true` setzen, falls die strengere Default-Behandlung (503 bei Redis-Outage) für ein Deployment unpassend ist.

### Security (Audit-Findings F1, F2, F4, F5, F6, F7, F8, F9, F10, F11 + XFF/Cross-Org)

- **OIDC `email_verified`-Gate beim Account-Linking** (F4, ADR-0033) — fremde OIDC-Subjects werden nicht mehr blind an Lokal-Accounts mit gleicher Email gelinkt, solange der IdP die Email nicht als verifiziert ausweist.
- **License-Activate Role-Case-Fix** (F10) — `license/routes.go` checkt jetzt `"Admin"` (PascalCase, DB-Seed-konform) statt des nirgendwo gesetzten `"admin"`. Pro-Aktivierung funktioniert wieder.
- **LocalLLMBadge zeigt Provider ehrlich** (F2, ADR-0034) — Backend liefert `provider_host` in `/ai/status`, Frontend reicht es in den Badge durch. Kein "Lokal"-Badge mehr bei OpenAI-Cloud.
- **XFF-Spoofing-Schutz** — `VAKT_TRUSTED_PROXIES` wird als CIDR-Liste in echo-`TrustOption`s übersetzt; XFF-Header von außerhalb des Trust-Sets werden ignoriert.
- **SAML `InResponseTo`-Binding** (F5, ADR-0036) — HMAC-signiertes Single-Use-Cookie bindet AuthnRequest-ID an die Browser-Session; ACS akzeptiert nur Assertions mit passendem `InResponseTo`.
- **Operator-Rebrand abgeschlossen** (F11, ADR-0035) — Helm/CRD/RBAC auf `secrets.vakt.io / VaktSecret` migriert; Group-Konsistenz per Unit-Test gepinnt.
- **Cross-Org Approve-Hijack geschlossen** — `AgentRunManager.Decide` prüft Caller-Org und User-Owner; fremde `run_id`-Approvals geben 404.
- **`cmd/rotate-key` repariert + erweitert** (F1, ADR-0038) — HKDF-Coverage auf alle 8 verschlüsselten Spalten (`so_secrets`, `totp_secrets`, `notification_channels` ×2, `integrations_github`, `org_saml_configs`, `webhooks.secret`, `cloud_integrations.config`). SAML-Legacy-Rows (raw-master-encrypted) werden im Lauf migriert.
- **`audit_log` tamper-evident** (F8, ADR-0040, Migration 149) — Per-Org SHA-256 Hash-Chain mit `prev_hash` und `entry_hash`. Neues Tool `cmd/audit-verify` lokalisiert Tamper auf die exakte Row. ISO 27001 A.12.4.3 / NIS2 / DORA Art. 11 Audit-Trail-Anforderungen erfüllt.
- **AI-Counter zentralisiert** (F3, ADR-0041) — Echo-Middleware `RequireAILimit` ersetzt inline-Gates; alle 8 LLM-erzeugenden Routes durch das Gate. Statischer Route-Coverage-Test verhindert künftige Drift.
- **PII-Log-Redaktion** (F7, ADR-0039) — Helper `logsafe.RedactEmail` (Format `***@domain`) ersetzt Volltextexposures in 38 Call-Sites über 13 Dateien.
- **Auth-Lockout fail closed** (ADR-0044) — `checkAccountLocked` / `checkIPLocked` geben 503 `AUTH_LOCKOUT_UNAVAILABLE` bei Redis-Outage statt fail-open. Opt-out via `VAKT_AUTH_FAIL_OPEN_ON_REDIS_OUTAGE=true`.
- **RLS-Theater zurückgenommen** (F6, ADR-0042, Migration 150) — Migration 012 hatte `ENABLE ROW LEVEL SECURITY` aktiviert, ohne dass die App `app.current_org_id` setzte. Ehrlich-Rückbau auf reine App-Layer-Isolation.
- **`shieldstack` Build-Artefakt aus Working-Tree entfernt** (F9, ADR-0037) — Datei war seit `b83890c` aus HEAD entfernt; lokal aufgeräumt, History-Rewrite-Plan dokumentiert.
- **`webhooks.secret` Legacy-Migration** (ADR-0043) — Boot-Hook `MigrateLegacyPlaintextSecrets` konvertiert historische Plaintext-Secrets idempotent auf das `enc:v1:`-Format.

### Operations & Releases (P1-1, P1-2, P1-5)

- **Worker-Health/Readiness** (P1-5) — `/health` (Liveness), `/health/ready` (DB + Asynq-Queue-Probe), `/health/queue` (per-Queue Counts) statt einzelnem DB-Ping.
- **`audit_log` Range-Partitioning** (P1-2, ADR-0045, Migration 151) — Yearly Partitions (2025–2028) + DEFAULT, `audit_logs`-Backcompat-View neu erstellt.
- **SBOM + SLSA-Provenance pro Release** (P1-1, ADR-0046) — `release.yml` generiert SPDX-2.3 + CycloneDX SBOMs via syft, attestiert via `cosign attest --type spdxjson`. Release-Body enthält SBOMs als Assets. Compliance für EU CRA Art. 13(15).

### Content

- **BSI IT-Grundschutz von 7 Stub-Controls auf 34 Bausteine** (ADR-0047) — vollständige Abdeckung aller 10 Schichten (ISMS, ORP, CON, OPS, DER, APP, SYS, IND, NET, INF), jeder Control mit deutscher Description, Domain, Evidence-Type und Weight nach CRA/DORA-Pattern.
- **i18n-Sweep P0+P1 (79 neue Keys × 4 Locales = 316 Strings)** — `AccessReviewsPage`, `AISystemsPage`, `ResilienceTestsPage`, `ExceptionsPage`, `EvidenceAutoPage`, `TISAXMappingPage`, `DSGVOTOMPage` von hardcoded-Deutsch auf `useTranslation`. 240 i18n-Contract-Tests pinnen alle 60 Keys × 4 Locales gegen Drift.

### Migrations

- **149** — `audit_log` Hash-Chain (`prev_hash`, `entry_hash` BYTEA-Spalten + Index).
- **150** — RLS-Policies aus Migration 012 zurückgenommen.
- **151** — `audit_log` zu `PARTITION BY RANGE (created_at)`, Yearly + DEFAULT.

### Tools

- **`cmd/audit-verify`** — neuer Verifier für die Audit-Log-Hash-Chain.
- **`cmd/rotate-key`** — komplett umgebaut zu einer Pipeline aus 8 Stages mit unit-testbaren Stage-Funktionen.

### Tests

- Backend: **33 Pakete grün** (Unit + neue Integration-Tests via testcontainers-postgres in `internal/integration_test/`).
- Frontend: **482 Tests grün** (vorher 242 + 240 neue i18n-Contract-Tests).

---

## [0.35.0] — 2026-05-25

> Tag-Note: dieser Release-Eintrag wurde nachträglich im Zuge von v0.36.0 ergänzt. v0.34.0 + v0.35.0 enthielten zwei Commits zur Pro-Tier-UX (`feat(ux): ProGate "Demnächst" + DemoTierHint für Pro-Module`) und Billing-Korrektur (`fix(billing): Polar.sh Checkout-URL auf tatsächliche Product-ID aktualisiert`).

---

## [0.33.0] — 2026-05-25

Monetarisierung Phase 4 — Pricing-Dokumentation + Public README

### Changed

- **README Pricing-Section** — Vollständige CE/Pro/Enterprise Tier-Tabelle mit Framework-Matrix (NIS2/ISO 27001 ✅ Community; BSI/EU AI Act/CRA ✅ Pro; DORA/TISAX/ISO 42001 ✅ Enterprise), Modul-Verfügbarkeit und Feature-Vergleich (AI: 25 req/month CE vs. Unlimited Pro/Enterprise). Checkout-Links auf Polar.sh aktualisiert.

---

## [0.32.0] — 2026-05-25

Monetarisierung Phase 3 — In-App UX vollständig

### Added

- **CE AI-Counter-Anzeige** — `CEAICounter`-Component zeigt "18 / 25 KI-Anfragen diesen Monat" mit Fortschrittsbalken im KI-Berater-Widget. Warnung bei ≤5 verbleibenden Anfragen (Amber), Erschöpft-State mit Upgrade-Link (Rot).
- **`useAIUsage` Hook** — ruft `GET /api/v1/vaktcomply/ai/usage` ab, liefert `{used, limit, is_pro}`. Pro-Orgs: `is_pro=true`, Counter ausgeblendet.
- **`AI_CE_MONTHLY_LIMIT` Error-Handling** — KI-Berater zeigt deutschen Hinweis statt generischem Fehler wenn das CE-Monatslimit erreicht ist.

### Changed

- **Checkout/Portal-URLs auf Polar.sh migriert** — `frontend/src/lib/constants.ts`: `VAKT_PRO_CHECKOUT_URL` → `buy.polar.sh/norvik-ops/vakt-pro-monthly`, `VAKT_POLAR_PORTAL_URL` neu. `VAKT_LS_PORTAL_URL` als Alias erhalten.

### Notes

Folgende Phase-3-Elemente waren bereits implementiert: License-Key-Eingabe (Settings → Lizenz), ProGate-Upgrade-Prompt, 30-Tage-Ablauf-Banner (LicenseExpiryBanner), Post-Expiry-Hint mit Renewal-Link.

---

## [0.31.0] — 2026-05-25

Monetarisierung Phase 2 — Gate Enforcement vollständig, CE AI-Counter

### Added

- **CE AI-Monatslimit (25 Anfragen)** — Community-Edition-Orgs können AI-Features (Gap-Analyse, Policy-Draft, Incident-Guide, Chat, GapExplain, RiskNarrative) bis zu 25-mal pro Monat verwenden. Ab Anfrage 26 folgt HTTP 402 mit `AI_CE_MONTHLY_LIMIT`. Pro/Enterprise: unbegrenzt.
- **`GET /api/v1/vaktcomply/ai/usage`** — gibt `{used, limit, is_pro}` zurück. Frontend nutzt das zum Anzeigen von "18/25 Anfragen diesen Monat".

### Notes

Feature-Gates für alle Module und Frameworks (TISAX, DORA, CRA, EU AI Act, SCIM, SSO) waren bereits vollständig implementiert (106 aktive `features.Require()`-Gates). Phase 2 war deshalb auf den fehlenden CE-AI-Counter reduziert.

---

## [0.30.0] — 2026-05-25

Monetarisierung Phase 1 — Polar.sh Webhook, Demo-Tier Enterprise, License-Infrastruktur vollständig

### Added

- **Polar.sh Webhook** — `POST /api/v1/billing/webhook` empfängt Polar.sh-Subscription-Events und stellt automatisch Pro-Lizenzschlüssel aus. HMAC-SHA256-Signaturverifikation, Replay-Schutz via `polar_webhook_events`, idempotente Subscription-Speicherung in `polar_subscriptions`. Migration 148.
- **Demo → Enterprise-Tier** — `VAKT_DEMO=true` erteilt jetzt Enterprise-Tier statt Pro. Alle Features inkl. SCIM, TISAX, DORA sichtbar für Interessenten auf der Demo-Instanz.
- **`IsEnterprise()` auf License** — neue Hilfsmethode für Enterprise-Gate-Checks. `IsPro()` gibt auch für Enterprise `true` zurück (Enterprise ⊇ Pro).
- **`VAKT_POLAR_WEBHOOK_SECRET`** — neue Umgebungsvariable für Polar-Webhook-Signaturprüfung, dokumentiert in `.env.example`.

---

## [0.29.0] — 2026-05-25

Pre-v1.0 Sprint D — HKDF-Schlüsseltrennung, SCIM-Token-Ablauf, Pentest-Dokumentation

### Security

- **HKDF domain-separated keys** — `VAKT_SECRET_KEY` leitet jetzt via HKDF-SHA256 separate Sub-Keys für jede Komponente ab (`vakt-paseto-v1`, `vakt-vault-v1`, `vakt-totp-v1`, `vakt-alert-v1`, `vakt-github-v1`, `vakt-cloud-v1`, `vakt-webhook-v1`). Algorithmus-Isolation: ein kompromittierter Token-Key gibt keinen Zugriff auf verschlüsselte Vault-Secrets und umgekehrt. **Breaking:** alle aktiven Sessions werden beim Rollout ungültig (Paseto-Signing-Key geändert).
- **Pentest-Scope-Dokument** — `docs/security/pentest-scope.md`: vollständige Scope-Definition für externe Pentester (In-Scope-Klassen, Test-Accounts, Out-of-Scope, Timeline, erwartete Deliverables).
- **Responsible-Disclosure-Policy** — `docs/security/responsible-disclosure.md`: öffentlich zugängliche Policy mit Timelines, sicheren Kommunikationskanälen, Safe-Harbour-Erklärung.

### Added

- **SCIM Token-Ablauf** — `POST /api/v1/admin/scim/tokens` akzeptiert jetzt `expires_in_days` (0 = unbegrenzt). Abgelaufene Tokens werden täglich automatisch durch einen Worker-Job revoked. Migration 147: `expires_at`-Spalte auf `scim_tokens`.

---

## [0.28.0] — 2026-05-25

Pre-v1.0 Sprint C — Datenbankperformance, unbegrenzte Queries gecappt

### Performance

- **Audit-Log-Composite-Index** — neuer Index `idx_audit_log_org_time ON audit_log (org_id, created_at DESC)`. Audit-Trail-Queries im Compliance-Dashboard sind ab 10.000+ Einträgen deutlich schneller. Migration 145.
- **Risk-Trend-Snapshots** — täglicher Worker-Job berechnet Risiko-Snapshot pro Organisation und schreibt in `vb_risk_trend_snapshots`. Dashboard-Queries laufen jetzt in O(Tage) statt O(Findings × Tage). Migration 146. Fallback auf Live-Berechnung für frische Instanzen ohne Snapshots.

### Fixed

- **Unbegrenzte Datenbankqueries** — 7 interne `:many`-Queries hatten kein `LIMIT` und konnten bei großen Datensätzen den DB-Pool blockieren. Alle gecappt: Risiken/Policies/Suppressions/SBOM-Komponenten (10.000), Scan-Schedules/Control-Tasks (500), Kommentare (200). Interne Aufrufer (PDF-Export, Audit, XLSX) nutzen explizit `limit=10_000`.

---

## [0.27.0] — 2026-05-25

Pre-v1.0 Sprint B — Command Palette, HR Toast-Undo

### Added

- **Command Palette** (`GlobalSearch`) — `Cmd+K` / `Ctrl+K` öffnet eine globale Suchpalette. Schnellnavigation zu Dashboard, Controls, Risiken, Vorfälle, Richtlinien, Findings und Board-Bericht. Freitext-Suche über alle Entitäten (Controls, Risks, Policies, Incidents, Assets, Findings, DSR, Breaches). Recent-Items-Gedächtnis, Keyboard-Navigation (↑↓ + Enter), Focus-Trap.
- **Toast-Undo für HR** — das Undo-Pattern (5-Sekunden-Countdown, Löschung erst nach Ablauf) ist jetzt auf HR-Checklisten-Items (`ChecklistsPage`) und Mitarbeiter-Verwaltung (`EmployeesPage`) ausgerollt. Bereits seit v0.24.0 aktiv für Risiken und Ausnahmen in Vakt Comply.

---

## [0.26.0] — 2026-05-25

Pre-v1.0 Sprint A — Infrastruktur-Hygiene

### Added

- **Helm Migration-Job** — `helm/vakt/templates/migrate-job.yaml` führt Datenbankmigrationen als Helm Pre-Upgrade-Hook aus. Keine manuellen Schritte mehr vor `helm upgrade`.
- **Konfigurierbare DB-Connection-Pool-Größe** — `VAKT_DB_MAX_CONNS` (Default: 25) ermöglicht Tuning für größere Deployments. Dokumentiert in `.env.example`.
- **Webhook-Secrets verschlüsselt** — Webhook-Secrets werden jetzt mit AES-256-GCM at rest verschlüsselt. Secrets sind nach der Erstellung nicht mehr über List/Get-Endpoints abrufbar (write-once). Bestehende Plaintext-Secrets werden beim Lesen transparent entschlüsselt (lazy migration).

### Changed

- **Vakt Operator** — Kubernetes-Operator umbenannt: Go-Modul `github.com/matharnica/vakt-operator`, CRD-Group `secrets.vakt.io/v1alpha1`. **Breaking** für bestehende Operator-Deployments (als experimental markiert, kein Bestand).
- **Modul-Isolation** — `vaktcomply` importiert `hr` nicht mehr direkt. HR-Onboarding/Offboarding-Evidence läuft über einen geteilten Interface-Typ in `internal/shared/platform/evidence`.

---

## [0.25.0] — 2026-05-25

Pre-v1.0 Phase 1 — Kritische Sicherheits- und Zuverlässigkeitsfixes

### Security

- **Offene Registrierung geschlossen** — `POST /api/v1/auth/register` liefert 403, sobald eine Organisation existiert. Nur der Bootstrap-Fall (leere DB) erlaubt die erste Registrierung. Migration 144 (`open_registration`-Spalte, Default `false`).
- **API-Key-Rotation IDOR** — `RotateKey` prüft jetzt `created_by = current_user`. SecurityAnalysts konnten bisher beliebige Keys der Organisation rotieren; das ist behoben.
- **MFA-Bypass via API-Keys dokumentiert** — die MFA-Middleware exemptiert API-Key-Sessions explizit (Automation-Pfad, kein interaktives TOTP möglich). Kommentar im Code erklärt das bewusste Design.

### Fixed

- **Redis-URL-Bug im Worker** — `buildServer()` und `buildScheduler()` haben die Redis-URL bisher direkt als `host:port` interpretiert. Bei URLs mit Passwort (`redis://:pw@redis:6379`) lief der Worker ohne Authentifizierung. Behoben via `redis.ParseURL()` — identisch zum API-Container. Background-Jobs (Demo-Cleanup, Token-Cleanup, Scan-Fortschritt) funktionieren jetzt zuverlässig.
- **BSI-Grundschutz-Controls stummes Abschneiden** — interne Aufrufer nutzten `ListCKControls` (LIMIT 1000). BSI-Grundschutz hat 800+ Controls; eigene Controls kommen hinzu. Alle internen Caller nutzen jetzt `ListCKControlsPaged` mit 10.000-Limit.

---

## [0.24.0] — 2026-05-24

Pre-v1.0 Consolidation Wave — Module Depth, AI-Native v2, Security Docs, UX Polish, Architecture Hygiene

### Added

#### Vakt Aware — Module Depth (S55)
- **8 Phishing Templates** — ready to use in every fresh instance: credential harvesting, invoice fraud, IT helpdesk, parcel notification, CEO fraud, MS 365, bank alert, software update.
- **5 Training Modules** — Phishing Awareness, Password Hygiene, Clean Desk Policy, MFA & 2-Factor, Social Engineering. Completions automatically flow as evidence into Vakt Comply.
- **Comply Evidence Banner** — resolving a finding shows "Finding resolution saved as evidence in Vakt Comply" + link. Training completions show "Saved automatically as evidence."
- **Extended Getting-Started Guide** — Step 6 (First Scan) and Step 7 (First Campaign), each with prerequisites, expected duration, and a direct action link.
- **Demo seed enrichment** — campaign click events pre-populated in demo instances for realistic campaign analytics.

#### Vakt Comply & Scan — Module Depth (S54)
- **Scanner status endpoint** — `GET /api/v1/vaktscan/scanner-status` returns `{trivy, nuclei, openvas}` availability; admin dashboard shows scanner health.
- **HR → Comply evidence flow** — completing an HR onboarding/offboarding checklist emits an evidence event in Vakt Comply (`/vaktcomply/evidence/auto`) with ISO 27001 A.6.1/A.6.5 control-mapping suggestion.
- **Control suggestion for HR evidence** — unassigned HR evidence shows a rule-based control suggestion, reducing manual mapping overhead.

#### AI-Native v2 (S52)
- **Evidence Freshness Check** — daily job flags controls with evidence older than 90 days as `evidence_stale` insight cards (24h dedup per control).
- **Gap-Explain (SSE)** — `POST /api/v1/vaktcomply/ai/controls/:id/explain` streams a German-language gap explanation into the control detail page. Local AI advisor, no external API.
- **Risk Narrative** — `POST /api/v1/vaktcomply/ai/risks/:id/narrative` generates and persists a risk narrative; displayed in Risk Detail with a "Regenerate" option.
- **AI Weekly Digest** — opt-in in Settings → AI Advisor. Every Monday 08:00 UTC: digest of open gaps, stale evidence, and unresolved high-severity findings.
- **Evidence Suggestion Banner** — Finding Detail shows `evidence_suggestion` insight cards for the current finding with one-click navigation to the suggested control.
- **AI Insights Widget** — Vakt Comply dashboard shows up to 5 dismissable AI insight cards sourced from `ck_ai_insights`.

#### UX Polish (S58)
- **Inline-Edit Controls** — Control title and status editable directly in the table row (double-click → field, Enter saves, Escape cancels). No modal for these fields.
- **Inline-Edit Findings & Risks** — Status and severity inline-editable. Bulk status-change via BulkActionBar + "Change status to…" dropdown for selected findings.
- **Optimistic UI for toggle states** — all boolean status PATCH calls update the UI immediately; on HTTP error: automatic rollback + error toast. No spinner wait.
- **Toast-Undo for delete actions** — all DELETE calls show a 5-second countdown toast with "Undo". DELETE executes only after the countdown expires.
- **AI Source Attribution** — AI responses include structured `sources` chips (e.g. "NIS2 Art. 21", "ISO 27001 A.6.1") extracted from the response. Chips navigate to the corresponding control or framework page.

#### Enterprise Trust & Security Docs (S60)
- **TOM (Art. 32 DSGVO)** — `docs/security/tom.md`: Technical and Organisational Measures document, verified against Go implementation (16/16 claims confirmed).
- **VVT Template (Art. 30 DSGVO)** — `docs/security/vvt.md`: Records of Processing Activities template with 9 pre-filled processing activities.
- **Internal Self-Pentest Guide** — `docs/security/pentest-intern.md`: OWASP Top 10 checklist with curl commands for IDOR, privilege escalation, SQL/prompt injection, brute-force, token revocation, and Vakt-specific attack surfaces (SSRF, mass assignment).
- **External Pentest RFP** — `docs/security/pentest-rfp.md`: ready-to-send RFP targeting Q3 2026 with scope, deliverables, timeline, budget (€3–8k), and 5-vendor shortlist.
- **SCIM 2.0 Verification Checklist** — `docs/security/scim-verification.md`: 10-point manual verification checklist with curl commands and Okta integration reference.

### Changed

#### Architecture Hygiene (S59)
- **Audit package consolidated** — `auditexport` + `auditreport` merged into `shared/audit` with `ExportHandler` / `ReportHandler`.
- **Worker handlers split** — 1,443-line `handlers.go` split into 5 domain files: `auth_handlers.go`, `scan_handlers.go`, `comply_handlers.go`, `aware_handlers.go`, `privacy_handlers.go`.
- **vaktcomply repository split** — 4,724-line `repository.go` split into 9 domain files < 600 lines each.
- **Integration test CI job** — new GitHub Actions job runs Go integration tests (`//go:build integration`) against a real PostgreSQL container on every push to `main`.

### Security

#### Security Hardening (S57)
- **Silent SQL error logging** — raw SQL errors no longer surface to API consumers; structured logging with context in `mfa_sensitive`, `org_ip_allowlist`, `audit`, `dataexport`, `license`, `auth`, `ai/service`.
- **MFA middleware hardened** — 8 unit tests added; fail-closed on org-DB error (503) and TOTP-DB error (403).
- **AI streaming hardened** — SSE endpoints validate content type and connection state; panics caught and logged.
- **TOM correction** — SCIM Bearer tokens are SHA-256 hashed (not bcrypt) — deterministic lookup required for API tokens. Documented in `docs/security/tom.md`.

### Fixed
- `no-unnecessary-type-arguments` ESLint rule — removed redundant `Error` type argument from TanStack Query mutation hooks.
- TypeScript strict mode — `useMutation` context generic added for optimistic rollback hooks.

---

## [0.23.0] — 2026-05-23

Security Hardening Wave 2 + Release Readiness Phase 1

#### Phase 1 — Release Readiness

- **feat(auth): Enterprise-Auth Frontend vollständig** — Confirm-Dialog für Session-Widerruf in `SessionsPage` (inkl. Panic-Button „Alle anderen abmelden"), Audit-Trail-Link pro API-Key in `ApiKeysPage`, Login-History-Section in `AccountSettingsPage` (letzte 50 Versuche, Failed-Logins fett markiert) (S20-3, S20-5, S20-7)
- **refactor(i18n): 62 raw date-Calls auf `useFormatDate` migriert** — alle Datumsangaben in Audit-Trail, Finding-Listen, Session-Tabellen, Compliance-Reports und Supplier-Portal respektieren jetzt die gewählte Sprache (DE/EN/FR/NL); kein hardcoded `de-DE` mehr in React-Komponenten (S13-27)
- **fix(i18n): `shared/utils/date.ts` auf `navigator.language` umgestellt** — Fallback-Locale in Utility-Funktionen war hardcoded `de-DE`; liest jetzt Browser-Locale dynamisch; betrifft Chart-Label-Formatter und CSV-Export-Datumsspalten

#### Sicherheit
- **Per-Email Password-Reset-Throttle** — max. 3 Reset-Mails pro Stunde pro Adresse via Redis-INCR; verhindert Inbox-Spam-Angriffe ohne Enumeration-Leak (Antwort bleibt immer HTTP 200)
- **HR API-Key-Scope** — `/api/v1/hr/`-Endpoints werden jetzt in der Scope-Path-Map geprüft; scoped API-Keys mit `"hr"`-Scope können gezielt auf HR-Endpoints zugreifen, andere Scopes werden abgewiesen

#### Bugfixes
- **EOL-Version-Parsing: Großbuchstaben-V-Prefix** — `normaliseCycle("V3.9")` lieferte `"v3.9"` statt `"3.9"`, weil `TrimPrefix` case-sensitiv ist und vor `ToLower` aufgerufen wurde. Fix: erst lowercase, dann trim. Betraf SBOM-Komponenten mit Großbuchstaben-V-Versionspräfix (z.B. aus Syft), die silently als "unknown" EOL-Status bewertet wurden.

#### Tests
- **MFAEnforceMiddleware vollständig getestet** — 8 neue Unit-Tests ohne Real-DB via `mfaDB`-Interface-Fake: exempt paths, missing context, fail-closed bei org-DB-Fehler (503), fail-closed bei TOTP-DB-Fehler (403), MFA required/not required, TOTP enabled/disabled
- **Password-Reset-Throttle-Invarianten** — 5 reine Logik-Tests: Konstanten-Grenzen, Zähler-Bedingung, Redis-Key-Format
- **vaktscan Domain-Invarianten** — 15 neue Tests: SLA-Severity-Mapping (BSI-90-Tage-Fallback), EOL-Versionsparsing (`majorCycle`, `normaliseCycle`), EOL-Payload-Deserialisierung (bool/string/date polymorph), `eolValue.UnmarshalJSON` alle 6 Varianten

#### Infrastruktur
- **`StartBackgroundRefresh` Lifecycle-Context** — Update-Check-Goroutine läuft jetzt mit Server-Lifecycle-Context statt `context.Background()`; wird bei SIGTERM sauber gestoppt bevor Echo shutdown

### v0.22.0 — Supplier Portal + Vakt Scan (2026-05-22)

#### Added
- Supplier Portal Phase 1 — Lieferanten-Register, Fragebogen-Builder (4 Frage-Typen, 3 Templates), externes Portal via Token-Link ohne Login
- Supplier Portal Phase 2 — Auswertungsansicht, Zertifikat-Ablauf-Alert (30 Tage), Assessment-Report PDF
- Asset Inventory — `environment` (prod/staging/dev), Kritikalitätsstufen, Ownership; Migration 139
- CVE-Enrichment-Service — NVD API v2.0, Redis-Cache 24h, 429-Retry-Backoff
- Finding-Deduplizierung cross-scanner — CVE+Asset-Key, Severity-Max-Merge, `sources`-JSONB
- SLA-Overdue-Badge in Findings-Liste — zeigt "SLA überfällig" wenn `sla_due_at` überschritten

---

### v0.21.0 — EU AI Act (2026-05-22)

#### Added
- KI-System-Inventar — `ai_systems`, `ai_classifications`; CRUD + Filter nach Risikoklasse + Status
- Risiko-Klassifizierungs-Wizard — JSON-konfigurierter Entscheidungsbaum nach Annex III (Verbots-Prüfung → Hochrisiko → Transparenzpflicht)
- Technische Dokumentation Hochrisiko-KI (Art. 11) — Template nach Annex IV, Versionierung, PDF-Export
- EU AI Act Dashboard — Kachel mit Systemen pro Risikoklasse, Countdown August 2026

---

### v0.20.0 — TISAX (2026-05-22)

#### Added
- TISAX® / VDA ISA-Framework — alle 15 Kapitel als Controls, Reifegrad 0–3, Schutzbedarf Normal/Hoch/Sehr hoch (Kapitel 15 Prototypenschutz optional)
- TISAX ↔ ISO27001 Mapping — ~60–70% Controls als vorgefüllt bei aktivem ISO27001
- TISAX Bereitschaftsbericht PDF — Reifegrad pro Kapitel, offene Controls, Deckblatt mit Assessment-Level

---

### v0.19.0 — BSI-Meldungsassistent + i18n (2026-05-22)

#### Added
- BSI-Meldungsassistent — Meldepflicht-Klassifizierung (3-Fragen-Wizard, obligation probably/unclear/none), Behörden-Empfehlung (BSI/BaFin+BSI/BNetzA/LDA), Migration 140
- Behörden-Verzeichnis (`authorities.yaml`) + Sektor-Konfiguration in Org-Settings
- Täglicher NIS2-Deadline-Check-Worker (24h/72h/30d-Fristen ab `first_detected_at`)
- Gemeinsamer `compliance_reporting`-Service — `DeadlineTracker`, `ComputeDeadlines()`, `AmpelStatus()`, `DORADeadlines`, `NIS2Deadlines`, `DSGVODeadlines`
- DORA TLPT-Dokumentation — Resilience-Test als DORA-Evidenz verknüpfbar; `POST /resilience-tests/:id/link-evidence`
- i18n-Infrastruktur Phase 1 — `i18next` vollständig verdrahtet, Locales DE/EN/FR/NL, Locale-Umschalter in User-Settings

---

### v0.18.0 — DORA Phase 1+2 (2026-05-22)

#### Added
- DORA-Kontrollkatalog als Framework-Seed (Art. II–VI, alle Artikel als Controls)
- DORA ↔ ISO27001 Mapping — geteilte Evidenz, „DORA-Lücken nach ISO27001-Abzug"
- IKT-Incident-Register — Typ `ikt_dora`, Felder `first_detected_at`, `reported_24h/72h/30d_at`, `severity_dora`, DORA-Klassifizierungs-JSONB; Migration 136
- Frist-Berechnung + Ampel (Worker-Cron alle 5 min, grün/gelb/rot pro Frist)
- IKT-Drittanbieter-Register — `dora_third_parties`, Kritikalitätsstufen, Ausstiegsstrategie, Vertragsparameter; Migration 138
- DORA Dashboard-Kachel — Drittanbieter-Zähler, fehlende Ausstiegsstrategien
- DORA PDF-Report — Abschnitt IKT-Drittanbieter + Resilienz-Tests

#### Changed
- `internal/shared/` → `platform/` Welle 4 (auditor, integrations, ldap, trustcenter, webhooks)

---

### v0.17.0 — Auth-Welle (2026-05-22)

#### Added
- SAML 2.0 Direct SP (CE) — AzureAD, Okta, OneLogin, Google Workspace; Metadata-XML-Endpoint
- SCIM 2.0 User+Group Provisioning (Pro) — `/scim/v2/Users`, `/scim/v2/Groups`, Filter-DSL
- IP-Allowlist für Admin-Endpoints (Pro) — CIDR-Konfiguration in Org-Settings
- MFA für sensitive API-Calls (Pro) — TOTP-Validation via `X-MFA-Token`-Header
- SIEM-Audit-Forwarder (Pro) — Splunk HEC, Elastic Bulk API, Generic Webhook; Asynq-Job mit Retry
- ADR-0022 Auth-Tier-Cut (SAML CE / SCIM+SIEM Pro)

---

### v0.16.0 — Foundation-Welle (2026-05-22)

#### Added
- Feature-Flag-Infrastruktur (`platform/features`) — alle Pro-Features über `IsEnabled()` steuerbar
- AgentRunPanel Approve-Cards — Write-Tool-Freigabe-Flow mit Audit-Log
- Cursor-basierte Pagination für Findings, Controls, Risks, Secrets, DSRs, Employees, Campaigns
- Typisierte Cross-Module Event-Contracts (`platform/events`) — `FindingCreated`, `BreachNotified`, `EvidenceCollected`, `IncidentCreated`

#### Changed
- `internal/shared/` → `platform/` Welle 3 (crypto, db, cache, telemetry, middleware, metrics, alerting, notify, scheduledreports, retention)
- Worker-Queue-Namespaces pro Modul (vaktscan concurrency 8, vaktprivacy 5, ai_agent 3, vaktcomply 5)
- Redis-Auth-Fallback auf PostgreSQL bei Redis-Ausfall

#### Fixed
- Dashboard.tsx von 1448 auf 144 Zeilen dekomponiert (5 Komponenten)
- SQL-Injection-Risiko in `admin/service.go` (dynamisches WHERE → fixe NULL-Safe-Placeholder)
- `interface{}` vollständig aus `internal/` eliminiert (Go 1.18 `any`)
- CI Frontend-Lint ist jetzt explizit blockend (`continue-on-error: false`)

---

### v0.15.0 — NIS2 Pro-Layer (Tag-Kandidat, Sprint 28)

Schließt die Pro-Schicht aus Sprint 19 vollständig ab. Kein Breaking-Change — alle neuen Features sind additiv und hinter `FeatureNIS2Reporting` Pro-gated. CE-Features des NIS2-Wizards bleiben unverändert.

**S28-1 Embedded-Mode:**
- NIS2-Self-Assessment-Wizard via `<iframe>` einbettbar auf Partner- und Berater-Sites.
- CORS `Access-Control-Allow-Origin: *` auf öffentlichen Wizard-Endpoints (`/api/v1/public/nis2-assessment/*`).
- `X-Frame-Options`-Header wird auf `/nis2-check*`-Routen entfernt; CSP `frame-ancestors *` gesetzt.
- Resize-Helper `public/nis2-embed.js` (PostMessage-basiert, 26 Zeilen, kein Tracking, kein Cookie).

**S28-2 Branded PDF-Export (Pro, `FeatureNIS2Reporting`):**
- `GET /api/v1/public/nis2-assessment/:token/export-pdf` — generiert mehrseitiges PDF: Cover mit Gesamtscore, Bereichs-Tabelle, Top-Gaps, Detailantworten.
- Footer „Erstellt mit Vakt · vakt.io". Rückgabe als `application/pdf` Blob (filename `nis2-assessment.pdf`).
- Frontend-Download-Button im Result-Screen — sichtbar nur wenn authentifiziert. Bei `402 Payment Required`: Upgrade-CTA.

**S28-3 Re-Assessment-History (Pro, `FeatureNIS2Reporting`):**
- Neue Tabelle `ck_nis2_assessment_runs` (Migration 127): speichert vollständige Assessment-Runs mit Scores + Top-Gaps.
- 90-Tage-Cooldown zwischen Re-Assessments — `429 Too Many Requests` mit `Retry-After`-Header bei Verletzung.
- Endpoint `GET /api/v1/vaktcomply/nis2-assessment/history` liefert alle Runs sortiert nach Datum.
- Frontend-Seite `/vaktcomply/nis2-history`: Trend-Pfeile (TrendingUp / TrendingDown) pro Bereich, Delta-Spalte zum Vorrun, Cooldown-Restanzeige, Leer-State mit CTA.

**S28-4 Multi-Framework-Wizard (Pro, `FeatureNIS2Reporting`):**
- 80 kombinierte Fragen: NIS2 (~30), ISO 27001 (~25), DSGVO-TOM (~25). Stabile IDs mit `mf.`-Prefix.
- 23 Cross-Mapping-Fragen, die mehreren Frameworks angerechnet werden (Ref-Feld pro Frage).
- Score-Engine `MultiFrameworkScore`: `NIS2`, `ISO27001`, `DSGVO`, `Overall`, `TopGaps`, `ByFramework`.
- Neue Route `/nis2-check/multi` — eigene Frontend-Page mit drei Fortschrittsbalken (NIS2 indigo, ISO27001 emerald, DSGVO violet) + Cross-Mapping-Hinweis im Result.

**S28-5 Landing-Page SEO:**
- `docs/marketing/nis2-check-landing.md` — deutschsprachige SEO-Vorlage für `vakt.io/nis2-check`.
- Meta-Block (title, description, canonical), Hero, NIS2-Bereichs-Tabelle, 3-Schritt-Flow, Zielgruppen-Blöcke, FAQ (5 Fragen inkl. DSGVO-Hinweis), Legal-Disclaimer. Optimiert auf „NIS2 Self-Assessment", „NIS2 Umsetzungsgesetz", „BSI NIS2 Compliance Check".

---

### v0.14.3 — Interne Qualitätswelle (Sprints 24-27, kein User-Impact)

Keine neuen User-facing-Features. Keine DB-Migrations. Kein Upgrade-Eingriff nötig.

**S24 — UX-Polish + Security-Hardening:**
- **`Spinner`-Komponente** als zentrale Ladeanimation eingeführt; Inline-`div`-Spinner in Frontend entfernt.
- **`StatusMapping`-Bibliothek** — zentralisierte `Record`-Typen für Status/Severity-Farb- und Label-Mappings; keine gestreuten `switch`-Blöcke mehr.
- **Toast-Migration** — verbleibende Inline-`fixed-bottom`-Toast-Blöcke auf globalen `toast()`-Hook umgestellt.
- **Settings-Modul** — 6 Settings-Pages nach `modules/settings/pages/` migriert (saubere Modul-Struktur).
- **IP-Lockout** — per-IP Redis-Failure-Counter: nach 10 fehlgeschlagenen Logins wird die IP für 15 Minuten gesperrt. Brute-Force-Schutz auf Login-Endpoint.
- **Backup-HMAC** — Backup-Archive werden mit HMAC-SHA256 signiert; Integritätsprüfung beim Restore.

**S25 — sqlc-Welle 1 (SecPulse + SecVitals) + E2E:**
- **SecPulse sqlc-Abschluss** — 3 verbleibende Raw-SQL-Stellen in `vaktscan/` auf sqlc migriert.
- **SecVitals sqlc Wellen 1+2** — `service_soa`, `approvals_handler`, `handler_my_tasks`, `milestones_repository` auf sqlc.
- **Playwright E2E V22-1** — Sessions-Panic-2-Step-Confirm, ApiKeys-Rotate-Modal, AgentRunPanel-Visualisierung. Schließt V22-1 aus dem Verifizierungs-Backlog ab.

**S26 — sqlc-Welle 2 (SecVitals + SecReflex + HR):**
- **SecVitals sqlc Wellen 3+4+5** — `handler_ical`, `handler_templates`, `service_policies`, `service_frameworks`, `handler_boardreport`, `service_reporting`, `policy_acceptance` auf sqlc.
- **SecReflex + Vakt HR sqlc-Abschluss** — alle verbleibenden Raw-SQL-Stellen in beiden Modulen migriert.

**S27 — sqlc-Abschluss Vakt Vault + E2E Verification:**
- **Vakt Vault sqlc komplett** — 29 neue sqlc-Queries (Shares, API-Tokens, Git-Scans, Scan-Results, Rotation-Policies, Access-Log, Secrets-Metadata). Drei dokumentierte Ausnahmen bleiben Embedded-SQL: `UpsertSecret` (ON CONFLICT + Crypto-Bytes), `GetSecretRaw`, `GetSecretByID` — beide geben `[]byte`-Encrypted-Value zurück, das sqlc-Code-Gen nicht abbilden kann.
- **SecPulse CI-Evidence** — `INSERT INTO ck_evidence` in `handler_ci_evidence.go` auf `r.q.InsertCKCIEvidence` migriert.
- **E2E Grace-Period-Badge** — Playwright-Test für `API_KEYS_IN_GRACE`-Fixture (rotated_at = jetzt → `text=Grace 24h aktiv` sichtbar). Schließt V22-1 vollständig ab.

---

### v0.14.2 — Build-Hotfix (2026-05-23)

Pure Build-Fix. Funktional identisch zu v0.14.1 für den Runtime-Pfad.

- **OpenAPI-Drift gefixt:** `HealthResponse` und `DemoStartResponse` Schemas waren in `backend/internal/shared/apidocs/openapi.yaml` nie definiert, wurden aber in `frontend/src/pages/Login.tsx` per `components['schemas']` referenziert. `npm run build` (tsc -b) ist deshalb seit v0.14.0 rot. Schemas nachgezogen, Types regeneriert. ADR-0017-Honesty-Audit-Miss.
- **`Setup.tsx` dead state entfernt:** `migratedMsg`-useState wurde gesetzt, dann `navigate('/')` — gerendert wurde es nie. Auf `toast()` umgestellt, damit der User die NIS2-Migrations-Bestätigung nach dem Sign-up auch tatsächlich sieht.
- **Verifizierung:** `go test ./...` + `npm run build` + `npm run test` alle grün.

### Sprint 22 Tail — Verbleibende Frontend-Komponenten + Tests (Tag-Kandidat v0.14.1)

Schließt die 4 in v0.14.0 zurückgestellten Items aus Sprint 22 ab. Damit ist der Sprint-22-Honesty-Audit vollständig abgearbeitet.

**S22-8 AgentRunPanel-Frontend:**
- Neuer Hook `useAgentRun` (`frontend/src/shared/hooks/useAgentRun.ts`) konsumiert den SSE-Stream von `POST /api/v1/vaktcomply/ai/agent/run`, parsed strukturierte `AgentEvent`-Frames (plan / tool_call / tool_result / reflect / final / error) und liefert `events[]`, `isRunning`, `error`, `durationMs`, `start()`, `stop()`.
- Neue Komponente `AgentRunPanel` (`frontend/src/shared/components/AgentRunPanel.tsx`): Goal-Input, Start/Stop-Button, Event-Cards mit farbcodierten Typen, JSON-Expand/Collapse pro Event für Arguments + Result.
- Neue Page `AIAgentPage` unter `vaktcomply/ai/agent` — mountet das Panel, listet erlaubte Tools/Approve-Skelett.

**S22-9 ApiKeysPage-Refactor:**
- **Scope-Picker im Create-Dialog**: Checkbox-Liste pro Modul (`vaktcomply.*`, `vaktscan.*`, `vaktvault.*`, `vaktaware.*`, `vaktprivacy.*`, `hr.*`) mit Beschreibungstexten. Leer = Personal-Key (Full Access, ambers gekennzeichnet).
- **Rotate-Button pro Key** mit eigenem Modal: Erklärt die 24h Grace-Period explizit, zeigt den neuen Raw-Key nach Rotation einmalig im New-Key-Dialog.
- **Scope-Tags und Grace-Indicator** pro Row: code-style-Pills mit dem Scope-String, oder „Personal (Full Access)"-Badge wenn leer. Während aktiver Grace-Period zusätzlich „Grace 24h aktiv"-Marker.
- **last_used_ip-Anzeige** unterhalb von last_used_at (klein, monospace).

**Backend-Begleitänderungen:**
- `apikeys.APIKey` Struct um `LastUsedIP` + `RotatedAt` erweitert; `List` selectiert beide Felder mit. Middleware-Hook für API-Key-Auth-Erfolg updated jetzt zusätzlich `last_used_ip` aus `c.RealIP()`.

**S22-10 Session-Management — Current-Session-Marker + Panic-Button:**
- `auth.AuthResponse` um `session_id` (UUID der `refresh_sessions`-Row) erweitert. `issueTokenPair` nutzt `RETURNING id::text`, damit Login/Register/Refresh die ID mitliefern.
- Frontend `api/client.ts` um `getSessionId()`/`setSessionId()`-Helpers erweitert; `apiFetch` sendet die ID als `X-Vakt-Session-Id` Header automatisch mit. `Login.tsx` persistiert die ID in localStorage; `setAuthToken(null)` löscht sie wieder.
- `auth.SessionHandler.ListSessions` markiert die zur Header-ID passende Row mit `is_current: true`. `RevokeAllOtherSessions` nutzt die Header-ID statt einer nicht-funktionierenden Token-Hash-Vergleichslogik.
- `SessionsPage` zeigt „Diese hier"-Badge + last_used pro Session, separiert „Andere abmelden" und einen 2-Step-confirm Panic-Button („inkl. dieser") mit auto-redirect auf `/login` nach Revoke.
- OpenAPI-Spec entsprechend nachgezogen: `LoginResponse` um `session_id`, `SessionInfo` an Backend-Form angepasst (`device_hint`, `last_used`, `is_current`) — gem. ADR-0017.

**S22-14 Integration-Tests für Cleanup-Jobs:**
- Neue Test-Datei `internal/integration_test/cleanup_jobs_real_test.go` (build-tag `integration`):
  - `TestCleanupAnonymousRuns_DeletesExpiredRows` — seedet 1 expired + 1 fresh Row in `nis2_anonymous_runs`, ruft `nis2wizard.CleanupAnonymousRuns`, asserted nur expired ist weg.
  - `TestCleanupLoginHistory_DeletesOldEntries` — seedet 1 Eintrag vor 100 Tagen + 1 frischer Eintrag in `login_history`, ruft `auth.CleanupLoginHistory`, asserted Retention-Grenze 90d sauber.
- Beide Tests bootstrap Postgres via testcontainers-go (analog zu `hr_evidence_real_test.go`), skippen sauber wenn Docker nicht verfügbar.

**Operations-Doku:**
- `docs/operations/maintenance-window-server-upgrade.md` — Wartungsfenster-Plan für Strato VC-2-4 → VC-6-12 Upgrade: Pre-Flight (T-24h, T-1h), Live-Migration vs. Backup-Restore-Variante, Post-Flight-Validierung (Health-Smoke aus ADR-0017 Checklist), Rollback-Strategie, Kommunikations-Schema.

### Sprint 22 — Fertigstellungs-Welle für Sprints 17-20 (Tag-Kandidat v0.14.0)

Schließt die Skeleton-Lücken aus 17-20 nach dem Honesty-Audit vom 2026-05-22. Kein neues Feature-Versprechen, sondern Einlösung alter. 12 Items voll-implementiert, 4 größere Frontend-Komponenten als [~] in nachfolgende Welle verschoben.

**22.1 Backend-Bugs (echte Defekte):**
- **S22-1 Auth-Lookup mit Grace-Period:** API-Key-Auth-Middleware akzeptiert jetzt `previous_key_hash` während `previous_key_grace_expires_at > NOW()`. Beim Match über alten Hash: Response-Header `X-Vakt-Key-Deprecated: true` + `Sunset: <RFC1123>` als Migrations-Signal. **Bug aus Sprint 20 effektiv broken Rotation** ist gefixt.
- **S22-2 RequireScope-Kontext-Plumbing:** Auth-Middleware setzt jetzt `auth_method=api_key`, `api_key_scopes`, `api_key_id` im Echo-Context. `apikeys.RequireScope(scope)`-Middleware kann das nun nutzen — manuelles Mounten auf Routen ist möglich. Volle 200-Route-Annotation ist noch eigener Sprint, aber das Plumbing steht.
- **S22-3 OIDC + SAML + Register schreiben login_history:** `auth.OIDCLogin`, `auth.SAMLLogin`, `auth.Register` rufen jetzt `recordLogin` mit source=`oidc`/`saml`/`register`. Failed-OIDC-Provisioning auch als `oidc_failed`. Sprint 20 hatte nur Password-Pfad — Audit-Gap geschlossen.

**22.2 Sign-up-Integration (NIS2-Akquise-Loop schließen):**
- **S22-4 Setup.tsx liest `?nis2_token=` + localStorage** und ruft nach erfolgreichem Setup `POST /vaktcomply/nis2-assessment/migrate-from-anonymous` auf. CTA aus dem Public-Wizard läuft jetzt nicht mehr ins Leere.
- **S22-5 Auto-Mapping auf NIS2-Controls** in `nis2wizard.AutoMapToControls`: value 0-1 → `not_implemented`, 2 → `partial`, 3-4 → `implemented`. Mapping via NIS2-Ref-Substring auf `ck_controls.description`/`control_id`. Nur Controls ohne aktiven manual_status werden überschrieben.
- **S22-6 Authentifizierter Endpoint** `POST /api/v1/vaktcomply/nis2-assessment/migrate-from-anonymous`. Service-Methode `MigrateAndAutoMap` kombiniert Migration + Auto-Mapping in einem atomaren Schritt.

**22.3 Frontend-UI (3 von 5, größere Komponenten als [~]):**
- **S22-7 `ScanProgressIndicator`-Komponente** unter `modules/vaktscan/components/`. Konsumiert SSE-Stream, zeigt Live-Phase + Percent-Bar + Heartbeat-Filter. Auto-Cleanup beim Unmount via AbortController.
- **S22-11 `LoginHistorySection`-Komponente** unter `shared/components/`. Tabelle mit TS / Quelle / Browser-Excerpt / IP / Result-Badge. Failed-Logins fett markiert. UA-Mini-Parser (Firefox/Edge/Chrome/Safari-Detection). In `AccountSettingsPage` eingebaut.

**22.4 Cleanup-Jobs:**
- **S22-12 `TaskCleanupAnonymousRuns`** (täglich 03:15 UTC): `DELETE FROM nis2_anonymous_runs WHERE expires_at < NOW()`. Im Worker-Scheduler verdrahtet.
- **S22-13 `TaskCleanupLoginHistory`** (wöchentlich Sonntag 04:00 UTC): `DELETE FROM login_history WHERE ts < NOW() - INTERVAL '90 days'`. Worker-Handler + Scheduler-Cron.

**22.5 Doku:**
- **S22-15 `docs/reviews/2026-05-22-honesty-audit.md`** dokumentiert den Skeleton-Status-Audit der zu Sprint 22 führte. Methodik, Item-Klassifikation, Lessons-Learned.
- **S22-16 CHANGELOG + UPGRADE** für v0.14.0 mit klarer Bugfix-Kennzeichnung der S22-1-Rotation-Defekts.

**Verschoben (S22-8, S22-9, S22-10, S22-14 [~]) → Folge-Welle:**
- S22-8 `AgentRunPanel`-Frontend (groß, Streaming-UI mit Approve-Cards).
- S22-9 `ApiKeysPage`-Refactor (Scope-Checkbox-Wizard, Rotation-Button-UI mit Modal).
- S22-10 Session-Mgmt-Backend-Endpoint (`/auth/sessions{,/:id/revoke,/revoke-all}`) + SessionsPage-Ausbau.
- S22-14 Integration-Tests für Cleanup-Jobs (brauchen testcontainers-Setup, separater Test-Hardening-Sprint).

### Sprint 20 — Enterprise-Auth CE-Tier (Tag-Kandidat v0.13.0)

CE-Schicht der Enterprise-Auth-Welle: feingranulare API-Key-Scopes mit Wildcard-Logik, zerstörungsfreie Rotation mit 24-h-Grace-Period, Login-Historie pro User. Pro-Schicht (SAML, SCIM, IP-Allowlist, MFA-API, SIEM) bleibt explizit Sprint 21 — on-demand bei konkretem Enterprise-Sales-Trigger.

**Backend (S20-1, S20-2, S20-6, S20-8):**
- Migration 126: `api_keys.previous_key_hash` + `previous_key_grace_expires_at` + `last_used_ip` + `rotated_at` für Rotation. Neue Tabelle `login_history` (user/email/ip/UA/source/result) mit 90-Tage-Retention-Plan.
- `internal/shared/apikeys/rotation_and_scopes.go`:
  - `RequireScope(scope)` Echo-Middleware mit Wildcard-Logik (`*`, `vaktvault.*`, `vaktvault.secrets.read`).
  - `ScopeAllows([]string, string) bool` als exportierter Helper für den Auth-Lookup-Pfad.
  - `Service.RotateKey(orgID, keyID) (*CreateResult, error)` — generiert neuen Hash, alter Hash wandert in Grace-Period (24h), beide werden vom Auth-Middleware akzeptiert. Endpoint `POST /api/v1/api-keys/:id/rotate`.
  - `RecordLoginAttempt` + `ListLoginHistoryForUser` Helpers.
- `auth/service.go`: Login-Pfad schreibt `login_history`-Entry bei `bad_password` + `ok`. Best-Effort, blockiert Login nie. Failed-Login ohne user_id (Account-Enumeration-Schutz).

**Docs (S20-8):**
- `docs/concepts/api-key-scopes.md` — Scope-Format, Wildcards, CI-Pipeline-Workflow, Rotation mit Grace-Period, Migration für Bestands-Keys, Backend-Implementation-Verweise, Skeleton-Status zu Auth-Middleware-Integration.
- `docs/concepts/README.md` Index aktualisiert.

**Verschoben (S20-3/4/5/7 [~] Frontend-Iteration):**
- S20-3 ApiKeysPage-Refactor (Scopes-Checkbox-Liste, Rotation-Button, Last-Used-IP) — Backend ist da, Frontend Cosmetic-Iteration.
- S20-4 Session-Mgmt-Endpoint + S20-5 SessionsPage — bestehende Skelette aus Sprint 2 reichen aktuell; Vollausbau in Folge-Welle.
- S20-7 Login-History-Section in AccountSettingsPage — Backend-Service-Methode `ListLoginHistoryForUser` ist da, UI ist iterativ.

### Sprint 19 — NIS2-Self-Assessment-Wizard CE (Tag-Kandidat v0.12.0)

Top-of-Funnel-Akquise-Asset für DACH-Markt 2026. Anonymer Wizard mit 30 NIS2-Fragen, Live-Score, Top-3-Gaps. Pro-Schicht (Branded PDF, Trend-View, Multi-Framework) als Folge-Welle vorbereitet.

**Backend:**
- Migration 125: `nis2_anonymous_runs` (7d-Lebensdauer, IP-Hash für DSGVO) + `ck_nis2_assessments` (Org-Migration bei Sign-up).
- `internal/shared/nis2wizard/` mit 30 Fragen über 8 Themenbereiche (NIS2 Art. 21 + BSI NIS2-UmsG §30). Gewichtete Score-Engine 0-4 mit Per-Area-Aufschlüsselung.
- Public-Endpoints (kein Auth, Rate-Limit 5/min/IP): `POST /public/nis2-assessment/{start,answer}`, `GET /public/nis2-assessment/{result,questions}`.
- `Service.MigrateToOrg(token, orgID, userID)` für Sign-up-Flow.
- 9 Score-Engine-Tests.

**Frontend:**
- `pages/NIS2WizardPage.tsx` unter `/nis2-check` (kein Layout, mobile-first). Multi-Step-Flow, Progress-Bar, Live-Score, Token in localStorage für Wiederbesuch.
- Result-Screen mit Ampel-Bewertung, Top-3-Gaps, CTA „Account erstellen + Ergebnis übernehmen".

**Docs:**
- **ADR-0021** Accepted: CE vs Pro Cut. Wizard + Sign-up-Migration sind CE; Branded-PDF + Trend + Multi-Framework sind Pro.

**Verschoben (S19-7..12 [~] Folge-Welle):**
- Embedded-Mode (iframe), Branded-PDF, Re-Assessment-History, Multi-Framework-Wizard, Auto-Mapping bei Sign-up, Landing-Page-Marketing.

### Sprint 18 — Agentic-AI v2 (Tag-Kandidat v0.11.0)

Vakts erste agentische AI-Workflows mit Plan/Execute/Reflect-Loop, Tool-Registry und RBAC-Enforcement. Adressiert den Bericht-§8-„AI-Native"-Hebel.

**Backend:**
- `AgentRunner` (`services/ai/agent.go`) mit MaxIterations (Default 5, Cap 10), OnEvent-Callback, Rate-Limit + Quota wie AI-Chat-Stream.
- `AgentTool`-Interface + drei Read-Only-Tools: `list_open_findings`, `list_stale_evidence`, `list_controls_without_evidence`. Jedes Tool deklariert `RequireScope` (z.B. `vaktscan.findings.read`).
- `POST /api/v1/vaktcomply/ai/agent/run` als SSE-Endpoint. Frame-Types: `plan`, `tool_call`, `tool_result`, `final`, `error`. Terminiert mit `[DONE]`.

**RBAC + Audit:**
- Tools werden im Plan-Prompt NUR gelistet, wenn der User den Scope hat. Defensiver zweiter Check vor jedem Execute. Audit-Log-Entry pro Agent-Run-Start (`action=agent_run_start, actor=ai_agent`).
- **ADR-0020** Accepted: keine Privilege-Escalation via AI; Pre-Approval-Pattern für mutierende Tools vorbereitet.

**Drei initiale Workflows:** Triage offener Findings, Wochen-Compliance-Plan, Evidence-Re-Collection.

**Docs:**
- `docs/concepts/ai-agents.md` — Architektur-Diagramm, Komponenten, SSE-Format, drei Workflows, Skeleton-Grenzen.
- ADR-0020 in `docs/adr/README.md`-Index.

**Verschoben (S18-4 [~]):**
- `AgentRunPanel`-Frontend mit Live-Plan-Steps + Approve-Cards. Backend-SSE-Endpoint ist produktiv; Frontend ist Cosmetic-Iteration für eine Folge-Welle.

**Skeleton-Grenzen (bewusst):**
- Plan-zu-Tool-Mapping via Substring-Heuristik statt echtem OpenAI-Function-Calling-Schema.
- Reflect ist Single-Pass-Final-Event statt iterativer LLM-Roundtrip pro Tool-Result.
- Beide Punkte sind Folge-Wellen-Themen; das Skeleton beweist das Pattern + die RBAC-Architektur.

### Sprint 17 — Realtime-Welle (Tag-Kandidat v0.10.0)

Erste produktive SSE-Endpoints nach dem ADR-0019-Pattern aus Sprint 16. Notifications und Scan-Progress werden jetzt live gepushed statt gepollt.

**Backend (S17-1, S17-2, S17-7):**
- `GET /api/v1/dashboard/notifications/stream` — server-side-poll-and-push, 2 s Cursor-Tick, 30 s Heartbeat-Pongs (`event: ping`). Skaliert besser als Postgres-LISTEN-per-Connection.
- `GET /api/v1/vaktscan/scans/:id/progress/stream` — subscribed Redis Pub/Sub auf `scan:progress:<id>`-Channel. Worker publiziert `started` und `finished`/`failed`; Stream beendet sich mit `data: [DONE]`. Org-Isolation enforced (Cross-Org-Stream → 404).
- `internal/modules/vaktscan/progress_stream.go` mit `PublishProgress(rdb, evt)`-Helper; im Worker (`handleScanJob`) verdrahtet vor + nach jedem Scan-Run.
- OpenTelemetry-Spans pro Stream-Lifecycle.

**Frontend (S17-3, S17-4):**
- `useNotificationStream`-Hook — fetch-SSE-Reader, Auto-Reconnect mit 1-s-Backoff, Heartbeat-Filter, Unmount-Cleanup.
- `NotificationBell` invalidiert React-Query-Cache bei jedem Stream-Event statt 60-s-Polling. `useNotifications.refetchInterval` entfernt.

**Docs (S17-6):**
- `docs/wiki/reverse-proxy.md` — nginx-Konfig für SSE-Endpoints (`proxy_buffering off`, `proxy_read_timeout 1h`, `location ~ ^/api/v1/.+/stream$`-Block). Caddy/Traefik/HAProxy/Cloudflare-Hinweise. Liste aller aktiven SSE-Endpoints.

**Tests (S17-8):**
- `parseSSEFrames`-Helper in `notifications_stream_test.go` — testbarer SSE-Frame-Parser mit 5 Unit-Tests (single-frame, ping-heartbeat, mixed-stream, empty, DONE-marker).

**Verschoben (S17-5 [~]):**
- `ScanProgressIndicator`-Frontend-UI als Cosmetic-Polish nach Sprint 18 verschoben. Backend-Pub/Sub-Infra produktiv, Hook-Pattern aus S17-3 wiederverwendbar.

### Sprint 16 — Frontend-Polish + Doku-Reife (Tag-Kandidat v0.9.0)

Sprint 16 schließt die Reife-Sanierung-Welle 2 strukturell ab. Schwerpunkt: Frontend-Hygiene + Doku-Vollständigkeit, keine API-Breaking-Changes.

**Doku-Wave (S16-5..9):**
- `docs/GLOSSARY.md` neu — Compliance-Vokabular (Control, Evidence, Framework, Finding, Risk, Incident, Cross-Module-Evidence, SoA, TOM, VVT, DPIA, AVV, DSR) + Vakt-Architektur-Begriffe (Modul, Service, Shared, Demo-Flow, safego.Run, Public Mirror).
- `docs/concepts/` Subdir mit `module-isolation.md`, `evidence-collection.md`, `rbac-model.md`, `demo-flow.md`. Narrative Erklärungen zur Architektur, komplementär zu den ADRs.
- `docs/api-versioning-policy.md` — Breaking-Change-Definition, 6-Monats-Deprecation-Window, CI-Enforcement-Plan, Sonderfälle für Security-/Legal-Pflichten.
- `docs/wiki/admin-cli.md` — vollständige Doku zu `vakt-admin` CLI (`health-check`, `list-orgs`, `list-users`, `reset-password`).
- `docs/adr/0019-sse-statt-websocket-fuer-realtime.md` Accepted — Server-Sent Events als Pflicht-Transport für alle Realtime-Pfade, WebSockets bewusst ausgeschlossen.

**Frontend-Polish (S16-1, S16-3, S16-10, S16-2):**
- **Severity-Farben als Design-Tokens** — Tailwind `theme.colors.severity.{critical,high,medium,low,info}` + `*-bg`-Varianten. Alle hardcoded `bg-[#hexhex]`-Bracket-Notations bereinigt (0 verbleibend). Whitelabel-Theme-Vorbereitung.
- **Code-Splitting** — alle Settings-/Admin-Pages auf `React.lazy()` umgestellt; Layout wrapped Outlet in Suspense. Eager bleiben Login/Setup/Dashboard + Token-Magic-Link-Pages (Auditor/Policy/Invite/DSR). Größter einzelner Chunk: `SecVitalsRoutes 452 kB` (gzip 105 kB) — unter Warning-Threshold.
- **`useFormatDate`-Bulk-Migration** — 60 Files mit hardcoded `toLocaleDateString('de-DE', ...)` / `toLocaleString('de-DE')` auf `formatLocale()` (neuer non-Hook-Helper) migriert. Hook-Variante `useFormatDate` (Sprint 13) bleibt für reaktive Komponenten verfügbar. 0 verbleibende Stellen.
- **openapi-typescript Client-Generierung** — `npm run api-types` generiert `frontend/src/api/generated.ts` (7018 LOC) aus `openapi.yaml`. CI-Step `api-types:check` enforced Drift (ADR-0017). `Login.tsx` als Demo-Migration nutzt jetzt `components['schemas']['LoginResponse']` statt Manual-Interface.

**Skip-Item:**
- S16-4 Bundle-Audit verschoben — `vite build` Chunk-Size-Warning erfüllt den Monitoring-Zweck; echte Tree-Shake-Optimierung lohnt sich erst nach Recharts/framer-motion-Bereinigung in einer Q3-Polish-Welle.

### Sprint 15 — AI-Härtung + Observability + Welle 2 (Tag-Kandidat v0.8.0)

Sprint 15 schließt die Backend-Stabilität (Sprint 14) ab und liefert produktreife AI-UX + Observability-Default-On.

**AI-Härtung (S15-1 bis S15-5):**
- Neue Tabelle `ai_usage` (Migration 124) trackt Tokens, Kosten (micro-EUR), Dauer und Status pro AI-Call. Konfigurierbare Tagesquota via `VAKT_AI_DAILY_TOKEN_LIMIT_PER_ORG`.
- Redis-basiertes Rate-Limit per Org (Default 30 req/min, `VAKT_AI_RATE_LIMIT_RPM`). Bei Verstoß `429 AI_RATE_LIMITED`.
- Response-Cache mit sha256(model+messages)-Key, TTL via `VAKT_AI_CACHE_TTL_SECONDS` (Default 1h). Cache-Hits werden als `cache_hit`-Status persistiert.
- Prompt-Injection-Schutz: strikte System/User-Role-Trennung in `buildMessages` — User-Input landet niemals im System-Prompt-Concat. Unit-Test deckt den Pfad ab.
- Neuer Endpoint `POST /api/v1/vaktcomply/ai/chat/stream` mit Server-Sent-Events: OpenAI-konforme `data: {"content":"..."}` Frames, `data: [DONE]`-Terminator, X-Accel-Buffering-Off für nginx.

**AI-UX Frontend (S15-6 bis S15-9):**
- `useAIStream` Hook konsumiert SSE-Frames inkrementell; bietet `text`, `isStreaming`, `error`, `durationMs`, `start(req)`, `stop()`. AbortController + Unmount-Cleanup.
- `LocalLLMBadge` zeigt sichtbar "Lokal · qwen2.5:3b" (No-Phone-Home-Differential) vs "Cloud · gpt-4o-mini" je nach Provider.
- `TokenCostIndicator` mit kompakter `1.2k Tk · 0.02 € · 4.3 s`-Anzeige nach Streamende.
- `AIAdvisor.tsx` als Demo-Migration: Live-Streaming-Rendering mit blinkendem Cursor, Stop-Button, Badge im Header, Cost-Indikator nach Abschluss. Rate-Limit/Quota-Errors bekommen spezifische i18n-Hints.
- i18n-Keys `ai.{localBadge,cost,stream}.*` in de/en/fr/nl.

**Observability default-on (S15-11 bis S15-15):**
- `MetricsEnabled` default `true` (opt-out via `VAKT_METRICS_DISABLED=true`); `/metrics` bleibt IP-allowlisted (Loopback + Docker-Netz).
- Prometheus + AlertManager im `docker-compose.observability.yml` Profil. `observability/prometheus.yaml` scrapt api + worker; `observability/alert-rules.yaml` mit 7 konservativen Default-Alerts (5xx-Rate, P95-Latency, Queue-Backlog, AI-Latency, …).
- 4 Grafana-Dashboards committed (`observability/dashboards/{api,worker,ai,demo}.json`) + Provisioning-Manifest. Beim Start automatisch unter dem Folder „Vakt" verfügbar.
- `alertmanager.example.yml` mit severity-basiertem Routing (critical→pager, warning→webhook, info→email-digest), Customer konfiguriert eigene Receiver — kein Phone-Home zu Norvik.
- `safego.SetPanicHandler` callback-Hook für optionale Sentry/3rd-party-Integration ohne externe Pflicht-Dependency.
- `docs/operations.md` Sektion 0 mit SLA-Matrix (RTO/RPO) für Container-Crash, Redis-Loss, DB-Korruption, Server-Verlust, K8s-Pod-Eviction, Region-Outage + PITR-/Hot-Standby-Empfehlungen.

**`internal/shared/` Konsolidierung Welle 2 (S15-10):**
- `internal/shared/{ai,alerting,evidence_auto,crossevidence}/` → `internal/services/*`. 17 Import-Call-Sites in 16 Files migriert, History via `git mv` erhalten.
- Neues `internal/services/README.md` dokumentiert die Boundary: `shared/` für Cross-Cutting-Concerns, `services/` für Cross-Module-Services mit eigener Domain-Logik. Welle-3-Kandidaten (scheduledreports, emaildigest, notifications) explizit als zukünftige Iteration markiert.

**Neue Env-Vars (Sprint 15):**

| Variable | Default | Bedeutung |
|---|---|---|
| `VAKT_AI_RATE_LIMIT_RPM` | 30 | Max AI-Calls pro Minute pro Org |
| `VAKT_AI_DAILY_TOKEN_LIMIT_PER_ORG` | 0 (aus) | Tages-Token-Quota pro Org |
| `VAKT_AI_CACHE_TTL_SECONDS` | 3600 | Response-Cache-TTL |
| `VAKT_AI_COST_PER_MTOKEN_IN_MICRO_EUR` | 0 | Kosten pro 1M Input-Tokens (0 = lokal) |
| `VAKT_AI_COST_PER_MTOKEN_OUT_MICRO_EUR` | 0 | Kosten pro 1M Output-Tokens |
| `VAKT_SENTRY_DSN` | leer | Optional Sentry-DSN; aktiviert PanicHandler-Hook |
| `VAKT_METRICS_DISABLED` | false | Opt-Out für /metrics (vorher: opt-in via VAKT_METRICS_ENABLED) |

### Sprint 13 — Reife-Sanierung Welle 2 abgeschlossen (Tag-Kandidat v0.7.0)

Befunde aus der zweiten Elite-Review (Mai 2026, archiviert unter `docs/reviews/2026-05-elite-review/`, Verify-Pass `docs/reviews/2026-05-bericht-verify.md`). 28/29 P0-Items erledigt; ein Bulk-Migration-Item (`useFormatDate`-Roll-out) verschoben in Sprint 16 (S16-10).

#### Sicherheit

- **SSRF-Guard für `VAKT_AI_BASE_URL`** — neue URL-Validierung beim Startup blockt IMDS (169.254.169.254), Loopback (127.0.0.0/8, ::1), Link-Local (169.254.x, fe80::/10) und `localhost` als Hostname, wenn `VAKT_AI_PROVIDER != "disabled"`. Allowlist für Container-Service-Discovery (`ollama`, `ai-llm`, `llm-proxy`, `lm-studio`) + alle Public-DNS-Hostnames. 22 Testfälle in `backend/internal/config/ai_base_url_test.go`.
- **LemonSqueezy Webhook-Replay-Schutz** — neue Migration `123_lemonsqueezy_webhook_events.{up,down}.sql` deduped Webhooks auf sha256(body). Doppelter Body → 200 OK ohne erneute Verarbeitung. Vorher konnte ein wiederholter `subscription_created`-Event prinzipiell mehrfach E-Mails / License-Operationen triggern.
- **LemonSqueezy Startup-Warning** — `NewHandler` logt `Warn` wenn `VAKT_LS_WEBHOOK_SECRET=""`; ohne Secret weist jede Signaturprüfung den Request ab.
- **bcrypt Cost-Upgrade-on-Login** — Login-Pfad prüft `bcrypt.Cost(hash)` und re-hasht transparent auf cost 12, wenn ein Legacy-Wert kleiner war. Update ist Best-Effort (Fehler nur Warn-Log), Login bleibt funktional.
- **Audit-Redaction erweitert** — `sensitiveKeys` in `audit/audit.go` enthält jetzt `recovery`, `backup`, `otp`, `mfa` zusätzlich zu `password`, `secret`, `token`, `key`. Felder wie `recovery_code` / `backup_code` / `totp_code` landen nicht mehr im Klartext im Audit-Log.
- **Trivy `ignore-unfixed: false`** im CI-Workflow (`backend` + `frontend` Scans). Unfixed-Akzeptanzen wandern in `.trivyignore` mit Begründung + Re-Check-Datum (Template enthalten).
- **gitleaks Per-Secret-Allowlist** — `.gitleaks.toml` nutzt jetzt `regexes` für konkrete Test-Konstanten (CI-Test-Hex, `admin1234demo`, `analyst1234demo`) statt pauschaler Pfad-Allowlist. Pfad-Liste auf wenige kontrollierte Dummy-Files reduziert (`.github/workflows/*.yml` und `docs/`, `Makefile` rausgeflogen).
- **Helm-Defaults verschärft** — `postgresql.auth.password` darf nicht mehr `"changeme"` sein UND muss ≥ 16 Zeichen lang sein (Honeypot-Default `MUST_BE_OVERRIDDEN` + `fail`-Hook in `_helpers.tpl`). `redis.auth.enabled` default `true` (vorher `false`). Siehe [UPGRADE.md v0.7.0](docs/UPGRADE.md) für Migrations-Hinweise.

#### Rebrand-Cleanup End-to-End

- **`helm/sechealth/` → `helm/vakt/`** — Verzeichnis umbenannt; alle 70 template-namespace-Definitionen (`define "sechealth.fullname"`, …) zu `vakt.*` migriert. Externe Konsumenten von `helm install ./helm/sechealth` müssen den Pfad anpassen — siehe UPGRADE.md.
- **`backend/cmd/sechealth/` entfernt** — legacy CLI-Binary, nicht in Makefile/Dockerfile referenziert, war Naming-Drift nach Rebrand.
- **`website/README.md`, `integrations/github-action/action.yml`, `integrations/gitlab-template.yml`** rebranded SecHealth → Vakt.
- **Frontend-Banner-Links** (`VersionBanner.tsx`, `TrustPage.tsx`) zeigen jetzt auf `github.com/norvik-ops/vatk` (Public Mirror).
- **`CLAUDE.md` Repo-Tree** aktualisiert (`sechealth/` → `vakt-app/`, `helm/sechealth/` → `helm/vakt/`).
- **`backend/cmd/admin/`** CLI `Use`-String + Beispiel-Outputs auf `vakt-admin` umgestellt.
- **Codekommentare + Default-Werte** in `vaktscan/handler.go` (PDF-Dateiname), `vaktcomply/policy_acceptance.go` (Default-From-Adresse), `vaktvault/git_scanner.go` (tmp-Dir-Prefix), `shared/notify`, `shared/dashboard/notifications.go`, `setup/handler_test.go`, `cmd/seed/main.go`, `frontend/src/hooks/useDashboard.ts`, `pkg/sdk/nodejs/{index.ts,package.json}` von `sechealth`/`SecHealth` auf `vakt`/`Vakt` umgestellt.
- **`docker-compose.demo.yml`** Header rebranded; statische Demo-Credentials-Kommentare entfernt (irreführend nach v0.6.2-Ephemeral-Refactor, Memory-Violation).
- **`.gitignore`** legacy-Patterns für gelöschtes Binary entfernt.

Bewusst belassen (Memory `project_rebrand` + ADR-0004): DB-Schema-Präfixe (`vb_`, `ck_`, `so_`, …), Docker-Image-`LEGACY_PREFIX`-Aliase (`ghcr.io/matharnica/sechealth/*`) für Watchtower-Backward-Compat, ADR-Historien-Texte, Memory-Dateien, Operator-CRD-Name `SecHealthSecret` (Kubernetes-API-Breaking-Change, separate Welle).

#### Stabilität

- **Silent SQL-Errors in `vaktcomply`** — alle 14 Stellen mit `_ = s.db.QueryRow(...).Scan(...)` durch sichtbare `err`-Pfade ersetzt. Neuer Helper `fetchOrgName(ctx, db, orgID)` in `vaktcomply/orgname.go` mit Warn-Log statt stillem Drop. Composite-Queries (`service_frameworks` Milestone-Dedup, `service_reporting` 30-Tage-Counter, `handler_boardreport` Score-History + Incidents-30d) loggen jetzt explizit; Milestone-Dedup bricht bei DB-Fehler defensiv ab statt Doppelversand.

#### PRD & Doku-Wahrheit

- **PRD aktualisiert** (`docs/prd.md`): Jira-FR-VB06 entfernt (v0.5.2-Realität), Success-Metric "first paying managed-cloud customer" → ADR-0008-konform formuliert ("First 10 self-hosted Pro customers"), Setup-Zeit "< 3 min" → "≤ 5 min Plattform + 3–30 min Ollama-Pull". MSP-Tertiary-Audience neu beschrieben (per-customer-instance, kein zentrales Portal). Epic E16 "MSP Multi-tenancy" gestrichen.
- **`CONTRIBUTING.md`** neu — Branch-/Commit-Stil, Test-Erwartung gemäß ADR-0012 (kein 80%-Quoten-Diktat), ADR-Prozess, PR-Workflow, Pre-Release-Smoke-Test gemäß ADR-0017, Security-Disclosure-Adresse, explizite "NICHT-Annahme"-Liste (MSP-Portal, Phone-Home, Cloud-SaaS-Integrationen).
- **`.github/ISSUE_TEMPLATE/{bug,feature,security}.yml`** + **`.github/PULL_REQUEST_TEMPLATE.md`** + **`CODEOWNERS`** neu.
- **`frontend/README.md`** komplett neu — Stack, Modul-Struktur, Dev-Befehle, wichtige Hooks/Patterns, Frontend↔Backend-Vertrag.
- **CHANGELOG-Fragment-Konsolidierung** — `docs/CHANGELOG-{sprint3,sprint4,sprint5,launch-readiness,security-wave-may26,session-2026-05-20}.md` nach `docs/history/` verschoben mit Index-README. Root-`CHANGELOG.md` bleibt Single-Source-of-Truth.
- **`CLAUDE.md`** 80%-Coverage-Satz zu ADR-0012 (risikobasiert statt Quote) konsistent gemacht.

#### Frontend-Quick-Polish

- **Demo-Login-Fail-Toast** (`Login.tsx`) — `/api/v1/demo/start`-Fehler → sichtbarer Error-Toast statt stillem UI-Zerfall. i18n-Schlüssel `auth.demoUnavailable` in allen 4 Locales.
- **`useFormatDate`-Hook** (`shared/hooks/useFormatDate.ts`) liefert `formatDate`, `formatDateTime`, `formatTime`, `formatRelative` für aktive i18n-Locale (BCP47-Mapping `de/en/fr/nl`). Demo-Migration in `AdminSecurityPage` + `SecVitalsOverviewPage`. Bulk-Migration der verbleibenden ~60 Treffer in Sprint 16 (S16-10).
- **Hardcoded deutsche Microcopy** `"Demo wird vorbereitet…"` → i18n-Schlüssel `auth.demoPreparing` in allen 4 Locales.
- **`useErrorMessage`-Hook** (`shared/hooks/useErrorMessage.ts`) — i18n-bewusster Wrapper um `humanizeError`. Bevorzugt `errors.<CODE>`-Lookup über die Locales, fällt auf bestehende Substring-Map zurück. Locale-Keys für `AUTH_INVALID_CREDENTIALS`, `AUTH_BAD_REQUEST`, `AUTH_VALIDATION_ERROR`, `AUTH_INVALID_STATE`, `AUTH_TOKEN_REVOKED`, `AUTH_OIDC_NOT_CONFIGURED`, `AUTH_OIDC_FAILED`, `ACCOUNT_LOCKED`, `RATE_LIMITED`, `GENERIC` in `de/en/fr/nl`.

### Geändert

- **[ADR-0018](docs/adr/0018-goroutine-lifecycle-und-panic-eskalation.md)** (Accepted) — Goroutine-Lifecycle (Parent-Context-Pflicht) und Panic-Eskalation via `safego.Run`. Pflicht-Pattern für alle `backend/internal/`-Goroutinen ab Sprint-14-Migration; golangci-lint-Regel blockt neue Verstöße.

### Behoben

- **`/health` enthält jetzt `demo`, `sso_enabled`, `version`** — Frontend (`useDemoMode`) las diese Felder, Backend lieferte sie nicht. Effekt: `isDemo` war auf `secdemo.norvikops.de` immer `false`, die Demo-Credentials-UI wurde nie eingeblendet.
- **`POST /auth/login` enthält jetzt das `user`-Objekt** (`id`, `email`, `display_name`, `roles[]`) — Frontend (`Login.tsx → setAuth(data.user)`) crashte mit `can't access property "id"` direkt nach erfolgreichem Login, weil das Feld fehlte.
- **OpenAPI-Spec auf realen Stand gebracht** — `LoginResponse`-Schema hatte `token`/`name`/`role` während Code längst `access_token`/`display_name`/`roles[]` nutzte. `/health` hatte gar kein Response-Schema. Beides angepasst.
- **Demo-Banner zeigt keine fake Credentials mehr** — `Layout.tsx` und i18n-Locales (de/en/fr/nl) hatten weiterhin `admin@vakt.local / admin1234` im Demo-Banner, was nach dem Ephemeral-Refactor irreführend war.

### Geändert

- **[ADR-0017](docs/adr/0017-api-contract-tests.md)** — Strategie gegen Backend/Frontend-Drift: OpenAPI-Schemas für alle Frontend-konsumierten Endpoints sind verbindlich, Contract-Tests + Type-Generation als Ziel-Architektur, Maintainer-Checkliste in `docs/dev/api-contract-checklist.md` als Übergang.
- **[ADR-0016](docs/adr/0016-public-mirror-via-script.md)** — Public Mirror per Script (`scripts/build-public-mirror.sh` + `make public-mirror`) statt inline rsync im CI. Eingebauter `go build ./...`-Check verhindert Bugs wie den v0.6.1-Excludes-Bug.

---

## [v0.6.2] — 2026-05-20

### Behoben

- **Demo-Login funktioniert wieder** — Backend `/api/v1/demo/start` gibt jetzt die generierten ephemeren Random-Passwörter (16 hex chars, admin + analyst) im Response zurück. Frontend `Login.tsx` nimmt sie und füllt die Login-Form vor. Vorher hatte das Frontend ein hardcodiertes `admin1234` als Default-Passwort, das (a) nicht den tatsächlich erzeugten Random-Hashes entsprach und (b) seit Erhöhung der Mindestpasswortlänge auf 10 Zeichen nicht mehr durch die Auth-Validierung kommt. Demo war dadurch unbenutzbar.
- **Statischer Demo-Seed nutzt 10+ Zeichen-Passwörter** — `demoseed.Run()` (für lokale Dev-Setups) setzt jetzt `admin1234demo` / `analyst1234demo`. Der frühere 9-Zeichen-Default (`admin1234`) wurde von der Auth-Validierung (min 10) abgelehnt.
- **Public Repo `norvik-ops/vatk` kompiliert wieder** — der Sync-Workflow hatte `internal/shared/demo/`, `demoseed/`, `feedback/` exkludiert, aber `cmd/api/main.go` importierte sie weiterhin. Wer die Codebase aus dem Public Repo baute, erhielt `no required module provides package …`-Fehler. Die drei Packages sind jetzt im Public Repo enthalten — sie sind hinter `if cfg.DemoSeed` gegated und ändern bei Customer-Default-Installs (VAKT_DEMO=false) das Verhalten nicht.

### Geändert

- **Doku zum Demo-Modus richtiggestellt** — `CLAUDE.md`, `docs/wiki/demo-mode.md`, `docs/setup.md`, `docs/configuration.md`, `docs/public/README.md`, `docs/launch-producthunt.md` und CI-Sync-Workflow dokumentieren jetzt einheitlich: Demo-Logins sind ephemer pro Visitor (Random-Slug, Random-Passwort, 4 h Lebensdauer), niemals statisches `admin@vakt.local / admin1234`.

### Lint / Hygiene

- **golangci-lint v2.12.2** statt v1.x — neuer config-Schema (`linters.settings`, `linters.exclusions.rules`), passend zu Go 1.25 build-toolchain
- **105 vorbestehende Lint-Verstöße bereinigt** — errcheck-Exclusions für idiomatische `defer X.Close()` Patterns, sinnvolle staticcheck-Ausnahmen für deutschsprachige Codebase, echte Bugfixes in `vaktcomply/reportpdf.go` (ungenutzte status-Variable in SoA-PDF jetzt im richtigen Feld dargestellt) und `alerting/service.go` (labeled `break` für korrekten Abbruch der Retry-Schleife bei ctx-cancel)

### Branding

- **Landing-Pages aktualisiert** — `vakt.norvikops.de`: Pro-Features auf v0.6.1-Stand (KI-Berater raus, AI Copilot Community rein, 6 Module statt 5, NIS2-Meldungsassistent + Lieferantenportal als Pro ergänzt), Enterprise-Sales-Block entfernt, Datenschutz „SecHealth" → „Vakt"; `norvikops.de`: Meta-Description + Form-Placeholder rebranded

---

## [v0.6.1] — 2026-05-20

> **⚠️ Upgrade-Hinweis für Bestandskunden:** Diese Version startet Ollama (AI Copilot)
> automatisch mit `docker compose up` (vorher hinter `--profile ai` versteckt). Der
> Ollama-Container lädt beim ersten Start einmalig das Modell `qwen2.5:3b` (~1.9 GB
> Download, ~2 GB RAM-Live-Footprint, 4 GB Limit). Auf VMs mit weniger als 8 GB
> Gesamt-RAM bitte VOR dem Upgrade `VAKT_AI_PROVIDER=disabled` in `.env` setzen
> und in einer Compose-Override-Datei den `ollama`/`ollama-init`-Service entfernen.
> Plattform-Startup-Zeit unverändert (<5 Min); AI-Funktionen sind 3–30 Min später
> verfügbar, abhängig von Internet-Bandbreite (1.9 GB Modell-Download).

### Geändert

- **AI-Copilot ist Community** — Die fünf AI-Endpunkte (`/vaktcomply/ai/status`, `/ai/report`, `/ai/advice`, `/ai/draft-policy`, `/ai/incident-guide` sowie `/vaktcomply/policies/generate-draft`) sind ab sofort in jeder Vakt-Instanz nutzbar — kein `FeatureAIAdvisor`-Pro-Gate mehr. Mit qwen2.5:3b als Default-Modell (Apache 2.0, ~1.9 GB RAM, CPU-tauglich) läuft die AI lokal auf jeder VM; ein Lizenz-Gate hatte daher nur Marketing-Charakter ohne echten Schutz. Premium-Compliance-Features (TISAX, DORA, NIS2-Reporting, EU-AI-Act, AuditPDF, SSO, API-Access, SecReflex/SecPulse-Advanced, Granular-Permissions, Supplier-Portal) bleiben Pro. `FeatureAIAdvisor`-Konstante bleibt für Lizenz-Validierung erhalten, wird aber nicht mehr im Routing geprüft.
- **Ollama default-on, Auto-Model-Pull** — `ollama` Service ist nicht mehr hinter `profiles: ["ai"]` versteckt; startet automatisch mit `docker compose up`. Neuer Init-Container `ollama-init` zieht das Default-Modell `qwen2.5:3b` einmalig beim ersten Start (idempotent — bei vorhandenem Modell No-Op). Damit ist AI nach einem einzigen `docker compose up` lauffähig — kein `--profile ai`, kein manueller `ollama pull` mehr. Resource-Limit auf Ollama: 4 GB RAM / 2 vCPU. Customers auf VMs mit < 8 GB Gesamt-RAM können via `VAKT_AI_PROVIDER=disabled` + compose-override deaktivieren.
- **Helm-Chart Ollama-Integration** — Neue Templates in `helm/sechealth/templates/ollama/`: StatefulSet mit PersistentVolumeClaim (10 Gi default), ClusterIP-Service, Helm-Hook-Job für das einmalige Modell-Pull. Default-on via `ollama.enabled: true` in `values.yaml`. Die ConfigMap setzt `VAKT_AI_BASE_URL` automatisch auf den Cluster-internen Ollama-Endpoint, oder erlaubt Override für externe LLM-Quellen (z.B. Mistral EU). Resource-Defaults: 500m CPU / 2 GiB Memory request, 2 / 4 GiB limit.
- **Vakt Aware vollständig sqlc-migriert** — Tabellen-Präfix `pg_*` → `sr_*` (Migration 122, reine Metadaten-Operation in Postgres). Damit konnte sqlc die Tabellen parsen und alle 35 Repository-Methoden auf den generierten Code umgestellt werden. Vakt Aware war das letzte Modul mit embedded SQL. **ADR-0005 schließt damit ab — alle Module nutzen sqlc.**

### Sicherheit

- **CSRF Double-Submit-Cookie** — alle state-ändernden Endpoints unter `/api/v1` sind jetzt zusätzlich zu SameSite=Strict per expliziten Token gegen CSRF geschützt; Backend setzt `csrf_token` Cookie bei Login/Refresh/OIDC/SAML, Frontend echot ihn als `X-CSRF-Token` Header
- **Helm Pod-Security** — `podSecurityContext` mit `runAsNonRoot: true`, UID 65532, fsGroup 65532; `containerSecurityContext` mit `readOnlyRootFilesystem: true`, `allowPrivilegeEscalation: false`, alle Capabilities gedroppt, seccomp `RuntimeDefault` für API und Worker; Frontend mit minimal nötigen Anpassungen für nginx
- **Verschlüsselung at-Rest dokumentiert** — neue `docs/encryption-at-rest.md` mit drei Pfaden (LUKS, Cloud-Provider, pgcrypto) und Installations-Checklist für DSGVO Art. 32
- **Redis-backed Org-Rate-Limiting** — fixed-window INCR/EXPIRE statt in-memory token-bucket; multi-replica-sicher für HA-Deployments
- **OIDC/SSO CSRF-Schutz** — OAuth2 `state`-Parameter wird jetzt serverseitig validiert (One-Time-Use via Redis, 10 min TTL); verhindert Login-CSRF-Angriffe
- **TOTP Deny-List** — ausgeloggte Paseto-Tokens waren auf 2FA-Endpunkten weiterhin gültig; Redis-Deny-List greift jetzt auch auf `/auth/2fa/*`-Routen
- **TOTP Replay-Schutz** — derselbe 6-stellige Code konnte innerhalb des 90-Sekunden-Fensters mehrfach eingesetzt werden; jetzt per Redis SetNX gesperrt
- **`RevokeAllOtherSessions`** — widerrief fälschlicherweise auch die eigene Session; eigene Session wird jetzt via `token_hash` ausgeschlossen
- **MFA-Enforcement Fail-Closed** — ein DB-Fehler beim MFA-Pflicht-Check ließ Requests kommentarlos durch; gibt jetzt HTTP 503 zurück
- **DSR-Portal** — öffentlicher Status-Endpunkt gab interne DPO-Notizen und org_id zurück; gibt jetzt nur noch `id`, `status`, `type` und Timestamps zurück
- **Setup-Handler Passwortvalidierung** — initiales Admin-Passwort konnte kürzer als 10 Zeichen sein; jetzt identisch mit der regulären Passwort-Policy
- **SMTP** — Port 465: implizites TLS (`tls.Dial`); Port 587: STARTTLS; keine Klartext-Credentials mehr
- **Webhook-RBAC** — Webhook-Endpunkte hatten keine Rollenprüfung; `List`/`Test` → `SecurityAnalyst+`, `Create`/`Update`/`Delete` → `Admin`
- **SSRF-Schutz** — Scanner-Targets (Trivy, Nuclei) werden gegen RFC-1918, Loopback und Link-Local geprüft; opt-out via `VAKT_SCAN_ALLOW_PRIVATE=true`
- **CSP** — `style-src` in `style-src-elem 'self'` (blockiert `<style>`-Injection) und `style-src-attr 'unsafe-inline'` (nur Inline-Attribute, nötig für UI-Framework) aufgeteilt
- **IP-Forwarding** — `X-Forwarded-For` wird nur noch ausgewertet wenn `VAKT_TRUSTED_PROXIES` gesetzt ist; verhindert IP-Spoofing bei direkter Installation

### Hinzugefügt

- **Session-Verwaltung pro Gerät** — neue Seite „Aktive Sitzungen" unter Einstellungen: alle angemeldeten Geräte einsehen und einzeln abmelden (`GET /auth/sessions`, `DELETE /auth/sessions/:id`)
- **Startup-Warnungen** — strukturierte Warn-Logs beim Start wenn HTTP statt HTTPS (`VAKT_FRONTEND_URL`) oder Demo-Modus aktiv (`VAKT_DEMO=true`)

### Infrastruktur

- **Nicht-Root-Container** — API, Worker und Migrate laufen jetzt als `nonroot` (UID 65532, distroless/static); kein Root-Prozess im Container
- **Go Healthcheck-Binary** — statisch kompiliertes `/healthcheck`-Binary ersetzt busybox-Abhängigkeit im distroless-Image; Docker-Healthcheck funktioniert ohne Shell
- **`VAKT_CORS_ORIGINS`** — CORS-Origins sind jetzt konfigurierbar (kommasepariert); Default `*`, Dokumentation in `.env.example` ergänzt

### Dokumentation & Architektur

- **Architecture Decision Records** — neuer `docs/adr/` Verzeichnis mit 12 retrospektiven ADRs: Self-Hosted-Prinzip, ELv2-Lizenz, Paseto-Wahl, Modul-Isolation, sqlc-Strategie, Anonymisierung statt Hard-Delete, Betriebsrat-Modus, MSP-Verzicht, OpenAPI-Single-Source-of-Truth, AES-256-GCM, OTel-Opt-in, Test-Coverage-Pragmatik

### Observability (opt-in)

- **OpenTelemetry-Instrumentation** — `internal/shared/telemetry/` initialisiert OTel beim Start, aktiviert sich aber nur bei explizit gesetztem `OTEL_EXPORTER_OTLP_ENDPOINT` (keine versteckten Telemetrie-Pfade, siehe ADR-0011)
- **Observability-Stack** — neue `docker-compose.observability.yml` Profile mit Loki + Promtail + Tempo + Grafana; aktivieren via `docker compose --profile observability up`; `docs/observability.md` mit Volumen-Schätzungen und Sicherheits-Hinweisen

### AI-Copilot

- **Default-Modell auf `qwen2.5:3b` umgestellt** — Apache-2.0-Lizenz statt Llama-Community, ~10 % weniger RAM-Footprint, schneller auf CPU, bessere Deutsch-Performance; alternative Modelle dokumentiert (`llama3.2:1b`, `phi3.5:mini`, `gemma2:2b`, `qwen2.5:7b`)
- **Policy-Drafting** — `POST /vaktcomply/ai/draft-policy` generiert einen Richtlinien-Entwurf in Markdown für ein Thema; Admin reviewt und veröffentlicht
- **Incident-Response-Guide** — `POST /vaktcomply/ai/incident-guide` erstellt aus einer Vorfalls-Beschreibung eine nummerierte Sofort-Checkliste mit gesetzlichen Fristen (NIS2, DSGVO Art. 33, DORA); im Frontend per „KI-Sofortmaßnahmen"-Button in der Vorfalls-Detailansicht direkt anwendbar
- **Wiki + Landingpage-Briefing** — neue `docs/wiki/ai-features.md` mit System-Requirements-Tabelle, Modell-Vergleich, DSGVO-Statement und Mistral-EU-Konfiguration; `docs/landingpage-ai-briefing.md` mit Headlines, Use-Cases und Vergleichstabelle gegen Vanta/Drata für die Marketing-Seite

### Refactor & Tests

- **HR-Service Pattern-Migration** — Audit-Logging vom Handler in den Service verlagert (P2-19/P2-20-Pattern); HR-Service ist jetzt vollständig SDK-fähig — Audit-Trail bleibt intakt auch bei Aufrufen aus Worker-Jobs oder künftigen CLI-Tools
- **sqlc Start für Vakt Vault** — Projects/Environments/AccessLog als sqlc-Queries (`db/queries/vaktvault.sql`); Secrets-Tabelle bleibt embedded SQL wegen Crypto-Spezifika
- **sqlc VVT (Vakt Privacy)** — Verzeichnis von Verarbeitungstätigkeiten (DSGVO Art. 30) komplett auf sqlc umgestellt; DPIA / AVV / Breach / DSR folgen in Folge-Sitzungen
- **Frontend-Test-Coverage erhöht** — 16 neue Unit-Tests: apiFetch (CSRF + Retry + Error-Mapping), useFirstAction (Persistenz + Idempotenz), useMilestoneToast (Schwellen + Jump-Detection); 2 vorbestehende Test-Fails behoben
- **Bugfix MilestoneToast** — Score-Jump-Baseline wurde nicht aktualisiert wenn ein Schwellen-Toast feuerte, führte zu Phantom-Toasts beim Remount; durch Test entdeckt und behoben
- **Integration-Test mit testcontainers-go** — echter End-to-End-Test für Vakt HR → Vakt Comply Evidence-Flow (`internal/integration_test/hr_evidence_real_test.go`); läuft in CI mit Docker-Daemon, skippt sauber wenn nicht verfügbar

### Datenschutz (DSGVO)

- **Recht auf Datenübertragbarkeit** (Art. 20) — neuer Endpoint `GET /api/v1/account/data-export` liefert ein ZIP-Archiv mit allen persönlichen Daten des Nutzers (Profil, Sessions, API-Keys-Metadaten, eigene Audit-Log-Einträge, eigene Kommentare, Benachrichtigungseinstellungen) als maschinenlesbare JSON-Dateien
- **Recht auf Löschung** (Art. 17) — neuer Endpoint `POST /api/v1/account/delete` mit Passwort-Re-Auth und expliziter „LÖSCHEN"-Bestätigung; Konto wird in der Datenbank anonymisiert (E-Mail, Name, Avatar geleert; Sessions + API-Keys widerrufen) statt hart gelöscht, um die Audit-Trail-Integrität gemäß ISO 27001 A.5.28 / BSI ORP.2 zu wahren; verhindert versehentliches Orphaning einer Organisation (letzter Admin → 409)

### UX-Verbesserungen

- **SlideOver-Komponente** — neue `SlideOver` für Linear-Style Detail-Panels mit framer-motion-Animation, Focus-Trap und Escape-Handling; nutzbar für Control-, Risiko- und Finding-Details ohne Kontextverlust
- **Micro-Guidance** — beim ersten Anlegen eines Risikos, Vorfalls, einer Richtlinie oder eines Assets erscheint ein einmaliger Hinweis mit Folge-Aktion-Empfehlung (z.B. „Control angelegt — als Nächstes Evidenz hochladen")
- **Role-basiertes Onboarding** — der Setup-Wizard zeigt nur die Schritte, die für die Rolle des angemeldeten Nutzers relevant sind: Admins sehen alle 4 Schritte, SecurityAnalysts nur die 2 Arbeits-Schritte (Control + Risiko), Viewer/Auditor sehen den Wizard gar nicht
- **Formular-Validierung erweitert** — `useFormValidation` unterstützt jetzt Cross-Field-Validation (`custom`-Callback) und scrollt + fokussiert automatisch das erste fehlerhafte Feld

### Hinzugefügt

- **OpenAPI 3.0 Spec — Single Source of Truth** — `backend/internal/shared/apidocs/openapi.yaml` wird zur Build-Zeit in den API-Server embedded; vorher lieferte der Server eine separate hardcoded Go-Spec mit nur 10 Endpoints, jetzt 75+. CI-Gate (`spec_test.go`) prüft YAML-Validität und blockiert PRs, die Pflicht-Endpoints aus der Doku entfernen. Spec ist über `GET /api/v1/openapi.yaml` und Swagger-UI unter `/api/docs` erreichbar. Kunden können daraus eigene SDKs generieren oder Automatisierungs-Skripte schreiben.
- **Frontend-Error-Tracking** — JS-Errors aus dem ErrorBoundary werden in der Tabelle `client_errors` persistiert; Admins sehen die letzten 200 Errors unter `GET /admin/client-errors` (org-scoped, self-hosted, kein externer Dienst)
- **Vakt Aware Content-Library** — 10 DACH-spezifische Phishing-Templates (CEO-Fraud, IT-Helpdesk, DHL, Microsoft-MFA, Mahnung, OneDrive, Sparkasse-SMS, USB-Köder, ...) + 5 vorgefertigte Trainings-Module abrufbar über `GET /api/v1/vaktaware/templates/presets` und `GET /api/v1/vaktaware/training-modules/presets`
- **Vakt Aware Anonymisierungs-Garantie** — Bei `betriebsrat_mode=true` werden IP-Adresse und User-Agent **gar nicht erst** in die DB geschrieben (statt nur im PDF-Export ausgeblendet) — DSGVO Art. 5 (1c) Datenminimierung + §87 BetrVG-konform; Wiki dokumentiert die rechtliche Begründung

### Datenbank

- Migration `117`: `refresh_sessions` — Tabelle für Refresh-Tokens mit Device-Info und Widerruf pro Gerät
- Migration `118`: `ck_evidence.control_id` nullable + neue Tabelle `hr_run_events` für Vakt HR Step-Audit-Trail
- Migration `119`: `client_errors` — Tabelle für persistierte Frontend-Errors

---

## [v0.5.5] — 2026-05-18

### Hinzugefügt

**Security**
- **CORS** — `CORSWithConfig` mit expliziten Methoden und exponierten Rate-Limit-Headern (statt Allow-All)
- **EPSS-Enrichment** — tägliche CVE-Exploit-Wahrscheinlichkeit via FIRST.org API (Batch 100 CVEs, Cron 01:00 UTC)
- **Control-Changelog (Vakt Comply)** — jede Status-, Owner- und Fälligkeitsänderung an Controls wird mit Zeitstempel und User-E-Mail in `ck_control_changelog` gespeichert; API: `GET /vaktcomply/controls/:id/changelog`

**UX & Interface**
- **Skeleton Loading** — alle Listenseiten (Incidents, Policies, Risks, Breaches, VVT) zeigen Skeleton-Platzhalter statt leere Fläche
- **Responsive Tables** — Desktop zeigt Tabellen, Mobile zeigt Cards (`useMediaQuery`-Hook)
- **Inline-Edit** — Finding-Status und Severity direkt in der Tabelle ändern (optimistisches Update + Rollback)
- **Empty States** — kontextspezifische Leerseiten mit direktem CTA (Frameworks, Assets, Risiken, Incidents)
- **Bulk-Aktionen Risiken** — mehrere Risks gleichzeitig auf einen Status setzen (`Promise.allSettled`)
- **`ConfirmDeleteDialog`** — Name-Eingabe-Bestätigung vor dem Löschen kritischer Objekte
- **`CopyButton`** — Kopieren-Button mit 2s-Feedback auf API Keys und Webhook Secrets
- **@-Mentions im Kommentarfeld** — Dropdown mit Teammitgliedern, Tab/Enter zum Einfügen, Escape schließt
- **Dark/Light/System-Toggle** — Drei-Stufen-Umschalter mit OS-Listener im Layout
- **Page Transitions** — 150ms Fade-Animation bei Navigation zwischen Seiten
- **Dashboard Drag & Drop** — Widget-Reihenfolge per HTML5 DnD anpassen, localStorage-persistiert
- **RTF-Export (Word)** — Framework-Controls als RTF-Dokument exportieren (Word-kompatibel, ohne npm-Dependency)
- **Vorfälle ↔ Datenpannen-Link** — `breach_id` wird in der Incident-Detailansicht als Link zu Vakt Privacy angezeigt; Breach-ID optional im Erstell-Dialog

**Platform**
- **Helm Chart** (K8s) — produktionsreifes Chart mit bitnami postgresql+redis Subcharts, HPA, Ingress, computed DSN helpers, liveness/readiness Probes
- **Queue Health Check** — Worker prüft alle 5 Minuten Redis-Queue-Tiefe und loggt Warnung bei >100 pending Jobs
- **EPSS Worker** — täglicher Cron-Job zur automatischen CVE-Anreicherung
- **Control-Owner-Reminder** — täglicher 09:00-Cron erinnert Verantwortliche an offene Controls
- **GitHub CI Evidence** — Worker sammelt GitHub Actions-Runs als Compliance-Evidenz (`ck_evidence`)
- **Playwright E2E** — 9 Spec-Dateien: Auth, Dashboard, Assets, Compliance, Navigation, Vakt Scan, Vakt Privacy, Vakt HR, Vakt Aware

**Dokumentation & API**
- **OpenAPI 3.0.3 v0.5.5** — 70 dokumentierte Pfade (+48 gegenüber v0.5.4): vollständige Vakt HR- und Vakt Aware-Endpunkte mit Schemas
- **Vakt HR Wiki** (`docs/wiki/modules/hr.md`) — vollständige Modul-Dokumentation mit API-Übersicht, curl-Beispielen und Compliance-Integration
- **api-reference.md** — Endpoint-Tabellen für Vakt HR und Vakt Aware ergänzt

### Entfernt
- **MSP-Layer** — `admin/organizations`-Endpunkte, MSPService, ImpersonateManagedOrg, Org-Branding-API vollständig entfernt. Vakt ist single-tenant self-hosted; MSPs deployen pro Kunde eine eigene Instanz.

### Datenbank
- Migration `102`: `ck_control_changelog` — Audit-Trail für Control-Änderungen
- Migration `103`: Entfernt MSP-Spalten aus `organizations` (`parent_org_id`, `msp_brand_logo`, `msp_brand_colors`, `scheduled_deletion_at`, Index)

### Upgrade
```bash
docker compose pull && docker compose down && docker compose run --rm migrate && docker compose up -d
```

---

## [v0.5.4] — 2026-05-18

### Hinzugefügt
- **Helm Chart** — `helm/sechealth/` mit bitnami postgresql+redis Subcharts, HPA, Ingress, NOTES.txt
- **OpenAPI 3.0.3** — vollständige Spec mit 45+ Endpunkten, BearerAuth, paginierten Responses, reuse-Schemas
- **Playwright E2E** — 5 Spec-Dateien (Auth, Dashboard, Assets, Compliance, Navigation) mit gemockter API
- **Queue Health Alert** — Worker loggt Warning wenn >100 pending Jobs in der Asynq-Queue

### Technisch
- EscalationChainSection (totes UI) entfernt
- CI: Node 24, FORCE_JAVASCRIPT_ACTIONS_TO_NODE24
- CI: E2E-Job mit chromium + Playwright-Report-Artifact

---

## [v0.5.3] — 2026-05-17

### Hinzugefügt
- **Notification Preferences** — Nutzer steuern welche E-Mails und In-App-Benachrichtigungen sie erhalten (`GET/PUT /notifications/preferences`)
- **Dependabot** — wöchentliche Dependency-Updates für Go, npm und GitHub Actions
- **Graceful Shutdown** — API und Worker beenden laufende Requests sauber (SIGTERM-Handler, 10s Timeout)

### Tests
- Webhook-Service: 5 Tests (HMAC-Berechnung, Event-Trigger mit und ohne Secret)
- Scheduled-Reports-Service: 13 Sub-Tests für Next-Run-Berechnung (wöchentlich/monatlich/vierteljährlich)
- Worker-Startup-Test

### CI
- GitHub Actions: Node 24 im Frontend- und E2E-Job
- `build-push-action@v6` in Staging-Deploy

---

## [v0.5.2] — 2026-05-17

### Entfernt
- **Jira-Integration** — entfernt wegen Datenabfluss zu Atlassian-Cloud (DSGVO Art. 28). Ersatz: Outgoing Webhooks für eigene Automatisierungen.

### Hinzugefügt
- **Webhooks aktiv** — `finding.created`, `finding.severity_changed`, `incident.created`, `incident.status_changed`, `control.status_changed` lösen jetzt tatsächlich Webhooks aus
- **Scheduled Reports** — Compliance-, Findings- und Risk-Berichte automatisch per E-Mail planen (wöchentlich/monatlich/vierteljährlich)
- **Excel-Export** — Findings, Risks und Controls als `.xlsx` aus der Toolbar exportieren
- **Risk Matrix interaktiv** — Klick auf Zelle zeigt Risiken der jeweiligen Kombination
- **Compliance-Score-Prognose** — Linearer Trend im Dashboard ("Bei aktuellem Tempo: 82% in 6 Wochen")
- **Notification Preferences** — Nutzer steuern welche E-Mails und In-App-Benachrichtigungen sie erhalten
- **In-App-Tour** — 5-Schritte-Tooltip-Guide für neue Nutzer
- **i18n vollständig** — alle Seiten auf Deutsch/Englisch (1.093 Keys)

### Sicherheit
- **Datenschutz-Grundsatz** in CLAUDE.md dokumentiert: keine Drittanbieter-SaaS-Integrationen die Vakt-Daten empfangen

### Upgrade
Neue Migrationen: `099_remove_jira`, `100_scheduled_reports`

---

## [v0.5.0] — 2026-05-17

### Added
- **AWS Evidence Collection** — automatische Sammlung von IAM-Passwortrichtlinie, MFA-Status, CloudTrail-Konfiguration und S3-Verschlüsselung als Compliance-Evidence
- **Azure Evidence Collection** — Secure Score, Security Center Assessments und Policy Compliance via Azure Management API
- **CIS Controls v8** — vollständiges Framework mit 61 IG1-Safeguards in 18 Kontrollgruppen, inkl. CIS ↔ ISO 27001 Mapping; Seeding in Vakt Comply
- **Progressive Web App (PWA)** — Vakt kann auf Mobilgeräten als App installiert werden (Offline-Unterstützung, Add-to-Home-Screen)
- **Englische Übersetzung** — vollständige UI-Übersetzung (277 Keys), automatische Spracherkennung, manueller Sprachwechsel in den Einstellungen
- **Jira-Integration** (Pro) — Findings und offene Controls direkt als Jira-Tickets erstellen
- **TOTP Recovery Codes** — 8 Einmal-Codes bei MFA-Einrichtung, sicher bcrypt-gehasht
- **Comments** — Kommentar-Threads auf Findings und Controls
- **Control Approvals** — Vier-Augen-Prinzip für Control-Statusänderungen (optionales Org-Setting)
- **Score-Verlauf** — Compliance-Score-Trend über Zeit, Recharts-Diagramm im Dashboard
- **Zertifizierungs-Timeline** — Countdown-Karten und Kalender für Audit-Meilensteine
- **Onboarding-Checkliste** — 6-Schritte-Assistent beim ersten Login

### Security
- **Rate-Limiting** — 300 Anfragen/min pro Organisation (Token-Bucket, Redis-backed), `X-RateLimit-*` Headers
- **Passwort-Mindestanforderungen** — min. 10 Zeichen, Großbuchstabe, Ziffer, Sonderzeichen bei Registrierung und Reset
- **Token-Cleanup-Job** — tägliche Bereinigung abgelaufener Passwort-Reset-Tokens (03:00 UTC)

### Improved (WCAG 2.1 AA)
- Farbkontrast Dark Mode: `--color-text3` von 3,1:1 auf 4,6:1 angehoben
- Globale `:focus-visible`-Regel für alle interaktiven Elemente
- ARIA-Attribute auf allen Formularen, Buttons und Navigationen
- Live Regions (aria-live) für Toasts und Fehlermeldungen
- Skip-to-main-content Link (screenreader + keyboard)
- Tabellenheader mit `scope="col"`
- `<html lang="de">` gesetzt (war "en")

### Infrastructure
- Worker HTTP-Healthcheck-Server (:9090) — Docker-Healthcheck repariert
- Dashboard-Cache-Invalidierung nach Control/Risk/Finding-Updates

---

## [v0.4.5] — 2026-05-17

### Security
- **Account Lockout** — nach 5 aufeinanderfolgenden Fehlversuchen wird das Konto 15 Minuten gesperrt (gleitendes Fenster, Redis-backed)
- **Session-Invalidierung** — alle aktiven Sessions werden bei Passwort-Reset sofort ungültig (`pw_version`-Claim im Paseto-Token)
- **Content-Security-Policy** — CSP-Header auf allen Antworten (script/style `unsafe-inline` für React SPA, `frame-ancestors 'none'`)

### Added
- **System-Status-Seite** (`/admin/health`) — DB-Latenz, Redis-Latenz, Queue-Tiefe (pending/active/failed), Uptime, Goroutinen, Version; automatische Aktualisierung alle 30 Sekunden
- **License-Ablauf-Banner** — gelbe Warnung ab 30 Tagen vor Ablauf, rote Warnung ab 7 Tagen; tageweise dismissbar, nur für Admins sichtbar

### Improved
- **Inline Evidence-Vorschau** — PDF- und Bild-Dateien öffnen sich direkt im Browser-Dialog statt als Download
- **Gespeicherte Filter** — Filterzustände in Audit-Log und Findings werden im Browser gespeichert und bei erneutem Besuch wiederhergestellt

---

## [v0.4.4] — 2026-05-17

### Security
- Security-Header im Backend: `X-Frame-Options: DENY`, `X-Content-Type-Options: nosniff`, `Strict-Transport-Security` (1 Jahr)
- Access Token TTL von 8 Stunden auf 1 Stunde reduziert
- `VAKT_SECRET_KEY` Länge wird beim Start validiert (exakt 32 Bytes / 64 Hex-Zeichen)
- MIME/Extension-Allowlist im Evidence-Upload-Handler

### Added
- **Passwort zurücksetzen** — "Passwort vergessen?"-Link auf der Login-Seite, E-Mail mit Reset-Link (1h gültig)
- **Audit-Log UI** — Admin-Seite mit Datum-, Benutzer- und Aktionsfilter, server-seitige Paginierung, CSV-Export
- **Granulare Modul-Berechtigungen** (Pro) — Lese-/Schreibrechte pro Modul pro Benutzer
- **Org-weites MFA-Enforcement** — Admins können 2FA für alle Mitglieder vorschreiben
- **API-Key-Verwaltung** (Pro) — Persönliche API-Keys (`vakt_...`) für programmatischen Zugriff
- **SSO-Login-Button** — erscheint auf der Login-Seite wenn `CASDOOR_URL` konfiguriert ist
- **Update-Status in Einstellungen** — zeigt installierte und aktuelle Version mit Link zu Release Notes
- **"Was ist neu"-Modal** — erscheint einmalig pro Version nach dem Login
- **Compliance-Fortschrittsbalken** — Dashboard-Widget zeigt umgesetzte vs. offene Controls
- **Wöchentlicher Sicherheits-Digest** — opt-in E-Mail-Zusammenfassung jeden Montag

### Improved
- Audit-Log: server-seitige Filterung (statt client-seitig)
- Update-Prüfung zeigt korrekt auf `norvik-ops/vatk` Repository


---

## [v0.4.1] — 2026-05-14

### Added
- **DSGVO Art. 32 TOM-Mapping** — New framework "DSGVO-TOM" with 13 technical and organisational measures (TOM-1 through TOM-13) mapped automatically to existing ISO 27001 controls. Coverage dashboard shows which TOMs are fully covered, partially covered, or open.

---

## [v0.4.0] — 2026-05-14

### Added
- **DORA support** — Digital Operational Resilience Act (EU 2022/2554) is now a selectable framework in Vakt Comply. Includes all relevant DORA articles as controls (German), DORA ↔ ISO 27001 mapping, gap analysis, readiness score, and PDF export.
- **DORA IKT Incident Register** — New incident type "IKT-Vorfall (DORA)" with automatic deadline calculation (T+4h / T+24h / T+72h / T+30d) and traffic-light status per deadline. Webhook notifications on deadline breach.
- **DORA IKT Third-Party Register** — Supplier records extended with DORA criticality, subcontractors, data processing location (EU/non-EU), and exit strategy fields.
- **DORA Resilience Tests** — New section in Vakt Comply for TLPT documentation (DORA Art. 24–27): test type, status, execution date, results, and recommendations.
- **TISAX support** — VDA ISA question catalogue as a selectable framework with protection-level selection (Normal / High / Very high). Maturity scale 0–3 per control. Chapter 15 (prototype protection) shown only when relevant.
- **TISAX ↔ ISO 27001 Mapping** — Static mapping with coverage badges. "Gaps only" toggle filters already-covered controls. Readiness score accounts for ISO 27001 evidence as TISAX coverage.
- **TISAX Readiness Report** — PDF export with protection-level category, readiness score per chapter, maturity distribution, and gap list.
- **Supply Chain Compliance — Supplier Portal** — External, token-based supplier portal at `/supplier/:token` (no login required). Compliance managers send time-limited invitation links; suppliers complete questionnaires and upload certificates (ISO 27001, TISAX labels, etc.) directly in the portal.
- **Questionnaire Builder** — Build supplier assessment questionnaires with question types: Yes/No, Multiple Choice, Free Text, File Upload. Predefined templates: "NIS2 Supplier Assessment", "DORA IKT Third Party", "ISO 27001 Basic Check".
- **Supplier Assessment Review** — Incoming questionnaires reviewable per answer (accepted / requires improvement). Uploaded certificates tracked with expiry date; warning 30 days before expiry. Accepted responses linked automatically as evidence to controls.
- **EU AI Act — AI System Inventory** — New section in Vakt Comply. Register AI systems with provider, use case, affected population groups, decision autonomy, and status. Filter by risk class.
- **EU AI Act — Risk Classification Wizard** — Step-by-step wizard following the EU AI Act Annex III decision tree (prohibition check → high-risk categories → transparency obligations). Result: risk class + justification + relevant articles. Reclassification with change log.
- **EU AI Act — Technical Documentation** — Documentation template per EU AI Act Art. 11 / Annex IV (German). Fields: system description, training data, performance metrics, risk management, human oversight, logging. PDF export and version history.
- **NIS2 / DORA Incident Reporting Assistant** — Reportability classification wizard on incident creation. Automatic authority suggestion based on configured sector. Deadline tracking (T+24h / T+72h / T+30d) with traffic-light status and email notifications 12 hours before each deadline.
- **Incident Report Generator** — One-click report form per deadline (24h / 72h / 30d): pre-filled from incident data, exported as PDF (BSI layout) and JSON. Sent reports archived with timestamp.
- **Authority Directory** — New page in Vakt Comply: list of notification authorities (BSI, BaFin, BNetzA, Luftfahrtbundesamt, BAFZA) with portal URL, phone, and sector-specific notes.
- **Sector Configuration** — Organisation settings now include sector and federal state selection. Responsible authority is suggested automatically in the incident register.
- **Supplier filter improvements** — Criticality filter (critical / essential / standard), assessment status filter, NIS2-relevant and DORA-relevant flags, contract status badges (Active / Expiring / Expired), CSV import and export.

### Fixed
- TypeScript build errors after feature merge (6 type issues resolved).
- Migration 037 (`pg_trgm` indexes) failed in transaction context — added `no-transaction` directive.

---

## [v0.3.0] — 2026-05-13

### Added
- **PDF report exports** — Vakt Scan generates real PDF reports with findings summary, severity breakdown, and paginated findings table. Vakt Comply frameworks export a readiness PDF (colour-coded score, domain breakdown, gap list). Vakt Aware campaigns export a campaign PDF (click rate, rate bars, Betriebsrat-mode banner).
- **External alerting & webhooks** — Send alerts to Slack, Teams, or any webhook endpoint with HMAC signing (`X-Vakt-Signature`). Configurable per alert type. Exponential backoff on delivery failure (up to 4 retries).
- **Backup & Restore** — `scripts/backup.sh` creates timestamped encrypted archives (PostgreSQL dump + AES-encrypted master key). `scripts/restore.sh` supports `--dry-run` for validation without touching the database. Passphrase must be at least 12 characters.
- **Global Search** — Full-text search across all modules (assets, findings, controls, incidents, policies, suppliers, VVT entries, and more). Powered by `pg_trgm` GIN indexes. Command palette shows "Recently viewed" entries.
- **Score configuration** — Admin UI to adjust weighting of compliance score components. "Reset to defaults" button added.
- **Automatic database migrations** — Dedicated `migrate` container runs all pending migrations before the API and worker start on every `docker compose up -d`.
- **Isolated demo instances** — `POST /demo/start` creates a fresh organisation with unique credentials per visitor. No shared demo state between visitors.

### Fixed
- Alert deduplication: alerts now fire at most once per 24 hours per event type per organisation (no more alert floods on each cron tick).
- `window.open()` exports caused 401 errors because Bearer tokens cannot be sent via URL — all exports switched to `fetch()` + Blob download.
- Nullable `description` field in breach records caused crashes when `NULL` — fixed with `COALESCE`.

---

## [v0.2.0] — 2026-03-15

### Added
- Initial Vakt Comply (Package `vaktcomply`) module with NIS2 and ISO 27001 control frameworks
- Vakt Scan (Package `vaktscan`) scanner orchestration: Trivy, Nuclei, OpenVAS integration
- Vakt Vault (Package `vaktvault`) secrets management with AES-256-GCM encryption and Git repo scanning
- Vakt Aware (Package `vaktaware`) phishing simulation engine with SMTP campaign delivery
- Vakt Privacy (Package `vaktprivacy`) DSGVO documentation: VVT (Art. 30), DPIA (Art. 35), AVV (Art. 28), breach records (Art. 33/34)
- Demo mode with seed data (`VAKT_DEMO=true`) and per-visitor ephemeral instances
- Initial Docker Compose production and development setups

---

## [v0.1.0] — 2026-02-01

### Added
- Initial open-source release of the SecHealth platform (now rebranded to Vakt)
- Echo v4 HTTP API with Paseto token authentication
- PostgreSQL 16 + sqlc type-safe query layer
- Redis 7 + Asynq background job queue
- golang-migrate database migration system
- Module isolation architecture with per-module RBAC scopes
- Docker Compose single-command deployment (`docker compose up -d`)
- CI/CD pipeline via GitHub Actions (build, lint, test, release)

---
