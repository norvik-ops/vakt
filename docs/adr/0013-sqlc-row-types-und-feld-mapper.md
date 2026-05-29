# ADR-0013: sqlc Row-Types und Feld-Mapper

**Status:** Accepted
**Datum:** 2026-05-20

## Kontext

Während der inkrementellen sqlc-Migration (ADR-0005) zeigte sich ein wiederkehrendes Muster: sqlc generiert pro Query mit individueller Column-Auswahl bzw. -Reihenfolge **separate Go-Structs**, auch wenn die Felder semantisch identisch sind und aus derselben Tabelle stammen.

Konkret: Für die AVV-Tabelle existieren parallel
- `db.PoAvvs` (Tabellen-Struct mit der Migration-Reihenfolge der Spalten)
- `db.ListPPAVVsRow` (durch `RETURNING …`-Liste mit anderer Reihenfolge)
- `db.GetPPAVVRow`, `db.CreatePPAVVRow`, `db.UpdatePPAVVRow`, `db.CreatePPAVVWithBodyRow`

Alle haben dieselben Felder, aber Go-Types sind nicht zuweisungskompatibel — ein einziger `avvFromRow(row db.PoAvvs)` Mapper funktioniert nicht.

Naive Lösung: pro Row-Type einen eigenen Mapper schreiben. Ergebnis: 6× dieselbe 17-Zeilen-Funktion, jede Änderung am Domain-Struct muss in allen Kopien nachgezogen werden.

## Entscheidung

**Pro Tabelle ein expliziter Feld-Container (`<entity>Fields`) plus ein einziger Mapper (`<entity>FromFields`).** Jede Call-Site listet die Felder einmal als Struct-Literal auf — das ist mechanisch (`row.X` → `f.X`), kostet 12-15 Zeilen pro Aufruf, aber zentralisiert die Domain-Mapping-Logik an einer einzigen Stelle.

Beispiel aus Vakt Privacy (`vaktprivacy/repository.go`):

```go
type avvFields struct {
    ID, OrgID, ProcessorName, ServiceDescription, Status string
    ContractDate, ReviewDate                             pgtype.Date
    Notes, TemplateID, Body                              pgtype.Text
    SccModule, SccAnnexI, SccAnnexIi, SccAnnexIii        pgtype.Text
    CreatedAt, UpdatedAt                                 pgtype.Timestamptz
}

func avvFromFields(f avvFields) AVV { /* zentrales Mapping */ }

// Aufruf:
a := avvFromFields(avvFields{
    ID: row.ID, OrgID: row.OrgID, /* … */
})
```

Wenn alle Queries der Tabelle den `PoAvvs`-Struct zurückgeben (z.B. weil sie `SELECT id, org_id, …` in der DDL-Reihenfolge selektieren), entfällt der Feld-Mapper — dann reicht ein direkter `avvFromRow(row db.PoAvvs)`.

## Alternativen

- **Per-Query-Mapper** (6× dieselbe Funktion) — verworfen: Wartungsaufwand bei Schema-Änderung wächst linear mit der Query-Zahl.
- **`go:generate` für die Mapper** — verworfen: zusätzliche Toolchain-Abhängigkeit, sqlc generiert bereits genug Code.
- **Queries umschreiben, sodass sie immer `PoAvvs`-kompatibel sind** — funktioniert für `SELECT *`, aber RETURNING-Klauseln in INSERT/UPDATE zwingen oft zu expliziten Listen wegen JOINs oder Spalten-Subsets. Nicht alle Fälle lassen sich vereinheitlichen.
- **Generics mit Interface-Constraints** — Go-Generics unterstützen keine strukturellen Constraints auf Felder; einzig per Reflection lösbar (zu teuer).

## Konsequenzen

### Positive

- Eine einzige Mapping-Funktion pro Domain-Typ — Schema-Änderung erfordert eine Änderung.
- Call-Sites sind selbstdokumentierend (welches Feld kommt woher).
- Verträglich mit dem sqlc-Verhalten, das wir nicht ändern können.

### Negative

- ~15 Zeilen Boilerplate pro Aufruf-Site.
- Compiler erkennt vergessene Felder nicht (Struct-Literal mit fehlendem Feld kompiliert; Feld bleibt Zero-Value).

### Neutrale

- Pattern etabliert in Vakt Privacy (`vaktprivacy/repository.go`, `avvFields`).
- Bei nächsten Migrationen prüfen, ob `<entity>Fields` nötig — wenn die Tabelle bei allen Queries denselben Row-Type liefert (wie `PoDpias`), reicht direktes Mapping.

## Referenzen

- ADR-0005 (sqlc inkrementelle Migration)
- `backend/internal/modules/vaktprivacy/repository.go` — `avvFields` + `avvFromFields`
