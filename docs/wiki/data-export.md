# Daten-Export (DSGVO Art. 20 / Migration)

Vakt bietet einen vollständigen Org-Daten-Export als ZIP von JSON-Dateien —
für die Datenübertragbarkeit (DSGVO Art. 20), für Migrationen und als
Vendor-Lock-in-Versicherung. Alle Daten verlassen die Instanz nur auf
ausdrücklichen Abruf eines berechtigten Org-Mitglieds.

## Endpoint

```
GET /api/v1/export        (alias: /api/v1/export/full)
Authorization: Bearer <token>
→ vakt-export-<org>-<datum>.zip
```

Der Export ist **org-scoped**: jede Abfrage filtert `WHERE org_id = <eigene Org>`;
es werden niemals Daten anderer Organisationen ausgeleitet.

## Enthaltene Dateien

| Datei | Inhalt | Modul |
|-------|--------|-------|
| `meta.json` | Export-Datum, Org-ID/-Name, Vakt-Version, Format-Version | — |
| `frameworks.json`, `controls.json`, `evidence.json`, `risks.json`, `incidents.json`, `policies.json`, `capas.json`, `tasks.json`, `comments.json` | Compliance-Daten | Vakt Comply |
| `vvt.json`, `dpias.json`, `avv.json`, `breaches.json` | DSGVO-Dokumentation | Vakt Privacy |
| `hr_employees.json`, `hr_checklist_runs.json`, `hr_contractors.json`, `hr_mover_events.json` | Mitarbeiterverzeichnis + Lifecycle (enthält PII) | Vakt HR |
| `sr_targets.json` | Awareness-Zielverzeichnis (Name/E-Mail, **roh**) | Vakt Aware |
| `sr_events.json`, `sr_assignments.json` | Phishing-/Trainings-**Ergebnisse** (pseudonymisiert) | Vakt Aware |
| `sr_completions.json` | Trainings-Abschlüsse (kein direkter Personenbezug) | Vakt Aware |
| `audit_log.json` | Manipulationssicheres Audit-Log (org-scoped) | — |

**Modul-Abhängigkeit:** HR-Dateien erscheinen nur, wenn `vakthr` in
`VAKT_MODULES_ENABLED` aktiv ist; Aware-Dateien nur bei aktivem `vaktaware`.

## Format-Version

`meta.json` nennt zwei Versionen. `vakt_version` ist die Vakt-Version, die das
Archiv erzeugt hat — sie stammt aus der laufenden Instanz und sagt beim
Zurückspielen, gegen welchen Stand das Schema passt. `format_version` beschreibt
die Darstellung der Werte in den JSON-Dateien:

| `format_version` | Stand | Darstellung |
|---|---|---|
| *(fehlt)* bzw. `1` | bis einschließlich v0.42.x | `uuid` als 16-Zahlen-Array, `bytea` als Base64, `date` als Mitternachts-Zeitstempel |
| `2` | ab v0.42.50 | `uuid` als Zeichenkette, `bytea` als PostgreSQL-Hex-Literal, `date` als `JJJJ-MM-TT` |

**Ein Archiv ohne `format_version` ist ein Archiv der Version 1.** In Version 1
stand jeder Fremdschlüssel als Liste von 16 Zahlen in der Datei
(`"framework_id": [238,190,84,224,…]`), sodass sich die Beziehungen zwischen den
Dateien mit Standardwerkzeugen nicht auflösen ließen. Wer ein altes Archiv
weiterverarbeitet, muss diese Arrays selbst in UUIDs zurückrechnen (16 Bytes,
Big-Endian, Gruppierung 8-4-4-4-12); ein neuer Export derselben Organisation ist
der einfachere Weg.

## Darstellung der Spaltentypen

Der Export liest die Tabellen generisch (`SELECT *`), damit neue Spalten ohne
Codeänderung mitkommen. Die Werte werden dabei anhand des PostgreSQL-Spaltentyps
in eine Form gebracht, die JSON verlustfrei trägt:

| PostgreSQL-Typ | JSON | Beispiel |
|---|---|---|
| `uuid`, `uuid[]` | Zeichenkette | `"eebe54e0-1111-2222-3333-444455556666"` |
| `timestamptz`, `timestamp` | RFC3339, in UTC | `"2026-08-07T04:35:13Z"` |
| `date` | Kalendertag ohne Uhrzeit | `"2026-11-03"` |
| `bytea` | PostgreSQL-Hex-Literal | `"\\x00ff41"` |
| `text`, `varchar`, `text[]` | Zeichenkette bzw. Array | `"offen"`, `["a","b"]` |
| `bool`, `int2`/`int4`/`int8`, `numeric`, `float8` | native JSON-Typen | `true`, `42`, `1.5` |
| `json`, `jsonb` | eingebettetes JSON-Objekt | `{"a":1}` |
| NULL | `null` | `null` |

`date` bewusst ohne Uhrzeit: Ein `"2026-11-03T00:00:00Z"` verschiebt sich beim
Einlesen in einer anderen Zeitzone um einen Tag. `bytea` bewusst als
Hex-Literal statt Base64: Der Wert ist so direkt wieder in PostgreSQL
einfügbar — der Export ist auch ein Migrationsweg.

Die Typen `interval` und `time` kommen in keiner exportierten Tabelle vor und
werden deshalb nicht abgebildet. Ein Test (`TestDataExport_NoUnrenderableColumnTypes`)
prüft bei jedem Lauf alle Spalten aller exportierten Tabellen gegen die Liste der
abgedeckten Typen und schlägt fehl, sobald eine Migration einen nicht abgedeckten
Typ hinzufügt — das Archiv kann also nicht stillschweigend unbrauchbare Werte
bekommen.

## §87 BetrVG / Betriebsrat — Pseudonymisierung der Awareness-Ergebnisse

Vakt Aware verspricht, dass der Org-Admin **nicht** sehen kann, *welche* Person
auf eine Phishing-Simulation geklickt hat (Mitbestimmung nach § 87 Abs. 1 Nr. 6
BetrVG; in der Report-/PDF-Schicht über SHA-256-Anonymisierung umgesetzt).

Der Org-Takeout ist ein **Org-Export**, kein personenbezogener DSAR. Damit er die
§87-Zusage nicht unterläuft, gilt:

- **`sr_targets`** (das Verzeichnis: Name/E-Mail) wird **roh** exportiert — dieselbe
  PII-Klasse wie `hr_employees`; der Admin kennt seine Belegschaft ohnehin.
- **`sr_events` / `sr_assignments`** (die personenbezogenen *Ergebnisse*) werden
  **pseudonymisiert**: Die Spalte `target_id` wird durch einen **gesalzenen
  SHA-256-Digest** (`anon_…`) ersetzt. Der Salt wird **pro Export zufällig**
  erzeugt und **nie** in das Archiv geschrieben — der Admin kann daher die
  `sr_targets.id` nicht nachhashen, um Ergebnisse einer Person zuzuordnen. Der
  Digest ist innerhalb eines Exports deterministisch, sodass die Ergebnis-Tabellen
  intern konsistent bleiben.
- **`sr_completions`** hat keine direkte Personenspalte (Bezug nur über
  `assignment_id` → die bereits pseudonymisierte `sr_assignments`) und wird roh
  exportiert.

> **Entscheidung (Produkt/Recht):** Diese Pseudonymisierung ist die bewusst
> gewählte Voreinstellung für den Org-Takeout. Ein **echter personenbezogener
> DSAR** (Auskunft einer einzelnen betroffenen Person nach Art. 15 DSGVO) würde
> deren eigene Ergebnisse vollständig enthalten — das ist ein separater,
> per-Person-autorisierter Pfad und nicht Teil des Org-Exports.

## Hinweis

Der Export enthält Klartext-PII (HR-Verzeichnis, Awareness-Verzeichnis). Die
exportierte ZIP-Datei ist entsprechend wie ein Backup zu behandeln — verschlüsselt
aufbewahren, Zugriff begrenzen, Löschfristen beachten.
