// Package dataexport provides a full data export endpoint for DSGVO compliance
// and migration safety. Customers can export all their org data as a ZIP of JSON files.
package dataexport

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog/log"
)

// exportFormatVersion identifies the layout of the archive and is written to
// meta.json. Consumers should branch on it rather than sniffing values.
//
//	"1" — up to and including v0.42.x: every `uuid` column was serialised as a
//	      16-element byte array ([238,190,84,...]) instead of a UUID string, so
//	      no foreign key in the archive could be resolved. `bytea` came out as
//	      Base64, `date` as a midnight timestamp.
//	"2" — uuid → canonical string, bytea → PostgreSQL hex literal (\x…),
//	      date → YYYY-MM-DD, timestamps → RFC3339 in UTC.
const exportFormatVersion = "2"

// safeNameRe strips characters that are unsafe in filenames.
var safeNameRe = regexp.MustCompile(`[^a-zA-Z0-9_-]`)

// entry couples an archive filename with the table it reads and the query that
// fills it. The table name is carried explicitly so a schema-guard test can
// enumerate the exported tables without keeping a second, drifting copy of the
// list (see TestDataExport_NoUnrenderableColumnTypes).
type entry struct {
	file  string
	table string
	query string
}

// Vakt Comply tables (ck_ prefix).
var vitalsEntries = []entry{
	{"frameworks.json", "ck_frameworks", `SELECT * FROM ck_frameworks WHERE org_id = $1::uuid ORDER BY created_at`},
	{"controls.json", "ck_controls", `SELECT * FROM ck_controls WHERE org_id = $1::uuid ORDER BY created_at`},
	{"evidence.json", "ck_evidence", `SELECT * FROM ck_evidence WHERE org_id = $1::uuid ORDER BY created_at`},
	{"risks.json", "ck_risks", `SELECT * FROM ck_risks WHERE org_id = $1::uuid ORDER BY created_at`},
	{"incidents.json", "ck_incidents", `SELECT * FROM ck_incidents WHERE org_id = $1::uuid ORDER BY created_at`},
	{"policies.json", "ck_policies", `SELECT * FROM ck_policies WHERE org_id = $1::uuid ORDER BY created_at`},
	{"capas.json", "ck_capas", `SELECT * FROM ck_capas WHERE org_id = $1::uuid ORDER BY created_at`},
	{"tasks.json", "ck_tasks", `SELECT * FROM ck_tasks WHERE org_id = $1::uuid ORDER BY created_at`},
	{"comments.json", "ck_comments", `SELECT * FROM ck_comments WHERE org_id = $1::uuid ORDER BY created_at`},
}

// Vakt Privacy tables (po_ prefix).
var privacyEntries = []entry{
	{"vvt.json", "po_processing_activities", `SELECT * FROM po_processing_activities WHERE org_id = $1::uuid ORDER BY created_at`},
	{"dpias.json", "po_dpias", `SELECT * FROM po_dpias WHERE org_id = $1::uuid ORDER BY created_at`},
	{"avv.json", "po_avvs", `SELECT * FROM po_avvs WHERE org_id = $1::uuid ORDER BY created_at`},
	{"breaches.json", "po_breaches", `SELECT * FROM po_breaches WHERE org_id = $1::uuid ORDER BY created_at`},
}

// Vakt HR tables (hr_ prefix) — employee directory + lifecycle records.
var hrEntries = []entry{
	{"hr_employees.json", "hr_employees", `SELECT * FROM hr_employees WHERE org_id = $1::uuid ORDER BY created_at`},
	{"hr_checklist_runs.json", "hr_checklist_runs", `SELECT * FROM hr_checklist_runs WHERE org_id = $1::uuid ORDER BY created_at`},
	{"hr_contractors.json", "hr_contractors", `SELECT * FROM hr_contractors WHERE org_id = $1::uuid ORDER BY created_at`},
	{"hr_mover_events.json", "hr_mover_events", `SELECT * FROM hr_mover_events WHERE org_id = $1::uuid ORDER BY created_at`},
}

// Vakt Aware tables (sr_ prefix) — see the §87 BetrVG note on buildZip.
var (
	awareTargetsEntry     = entry{"sr_targets.json", "sr_targets", `SELECT * FROM sr_targets WHERE org_id = $1::uuid ORDER BY created_at`}
	awareCompletionsEntry = entry{"sr_completions.json", "sr_completions", `SELECT * FROM sr_completions WHERE org_id = $1::uuid ORDER BY completed_at`}
	awarePseudoEntries    = []entry{
		{"sr_events.json", "sr_events", `SELECT * FROM sr_events WHERE org_id = $1::uuid ORDER BY occurred_at`},
		{"sr_assignments.json", "sr_assignments", `SELECT * FROM sr_assignments WHERE org_id = $1::uuid ORDER BY created_at`},
	}
)

var auditEntry = entry{"audit_log.json", "audit_log",
	`SELECT * FROM audit_log WHERE org_id = $1::uuid AND deleted_at IS NULL ORDER BY created_at`}

// ExportedTables returns every table the archive can contain, across all
// modules. Used by the schema-guard test; the export itself walks the entry
// lists directly.
func ExportedTables() []string {
	var out []string
	for _, group := range [][]entry{vitalsEntries, privacyEntries, hrEntries, awarePseudoEntries} {
		for _, e := range group {
			out = append(out, e.table)
		}
	}
	return append(out, awareTargetsEntry.table, awareCompletionsEntry.table, auditEntry.table)
}

// moduleEnabled reports whether the named module appears in the comma-separated
// VAKT_MODULES_ENABLED value (case-insensitive). Mirrors config.IsModuleEnabled
// without importing config, so the export respects per-module activation.
func moduleEnabled(csv, name string) bool {
	for _, m := range strings.Split(csv, ",") {
		if strings.EqualFold(strings.TrimSpace(m), name) {
			return true
		}
	}
	return false
}

// ExportHandler returns an Echo handler that streams a full-data ZIP to the client.
// modulesEnabled is the VAKT_MODULES_ENABLED CSV; HR/Aware files are only included
// when the respective module is active (S89-2). version is the running Vakt version
// (cfg.Version, injected at build time) and is recorded in meta.json so a restored
// archive states which release produced it.
func ExportHandler(db *pgxpool.Pool, modulesEnabled, version string) echo.HandlerFunc {
	return func(c echo.Context) error {
		orgID, ok := c.Get("org_id").(string)
		if !ok || orgID == "" {
			return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		}

		ctx := c.Request().Context()

		// Resolve org name for the filename.
		var orgName string
		if err := db.QueryRow(ctx, `SELECT name FROM organizations WHERE id = $1::uuid`, orgID).Scan(&orgName); err != nil {
			log.Warn().Err(err).Str("org_id", orgID).Msg("dataexport: could not resolve org name")
		}
		if orgName == "" {
			orgName = orgID
		}

		zipBytes, err := buildZip(ctx, db, orgID, orgName, modulesEnabled, version)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "export failed"})
		}

		safeName := safeNameRe.ReplaceAllString(strings.ToLower(orgName), "-")
		date := time.Now().UTC().Format("2006-01-02")
		filename := fmt.Sprintf("vakt-export-%s-%s.zip", safeName, date)

		c.Response().Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
		return c.Blob(http.StatusOK, "application/zip", zipBytes)
	}
}

// buildZip assembles all entity JSON files into a single ZIP archive.
func buildZip(ctx context.Context, db *pgxpool.Pool, orgID, orgName, modulesEnabled, version string) ([]byte, error) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)

	if version == "" {
		version = "unknown"
	}

	// meta.json
	meta := map[string]string{
		"export_date":    time.Now().UTC().Format(time.RFC3339),
		"org_id":         orgID,
		"org_name":       orgName,
		"vakt_version":   version,
		"format_version": exportFormatVersion,
	}
	if err := writeJSON(zw, "meta.json", meta); err != nil {
		return nil, fmt.Errorf("meta: %w", err)
	}

	writeEntries := func(entries []entry) error {
		for _, e := range entries {
			data, err := queryToJSON(ctx, db, orgID, e.query)
			if err != nil {
				// Non-fatal: write an empty array so the file still exists.
				log.Warn().Err(err).Str("file", e.file).Msg("dataexport: table export failed, writing empty array")
				data = []byte("[]")
			}
			if err := writeRaw(zw, e.file, data); err != nil {
				return fmt.Errorf("%s: %w", e.file, err)
			}
		}
		return nil
	}

	if err := writeEntries(vitalsEntries); err != nil {
		return nil, err
	}
	if err := writeEntries(privacyEntries); err != nil {
		return nil, err
	}

	// Only exported when the vakthr module is enabled (S89-2, PRIV-001).
	// These carry identifying PII (names, e-mails) and belong in an Art. 20 export.
	if moduleEnabled(modulesEnabled, "vakthr") {
		if err := writeEntries(hrEntries); err != nil {
			return nil, err
		}
	}

	// Vakt Aware tables (sr_ prefix). Only exported when vaktaware is enabled.
	//
	// §87 BetrVG / Betriebsrat: the awareness module promises that the org admin
	// can never see WHICH person clicked a phishing simulation. This org-takeout
	// must not break that promise:
	//   - sr_targets (the directory: name/e-mail) is exported RAW — same class as
	//     hr_employees; the admin already knows their staff.
	//   - sr_events / sr_assignments (phishing RESULTS) are PSEUDONYMISED: the
	//     target_id is replaced by a salted SHA-256 digest. A fresh random salt is
	//     generated per export and never written to the archive, so the admin
	//     cannot re-hash sr_targets.id to re-identify who had which result. The
	//     digest is deterministic within one export, preserving internal
	//     consistency across the result tables.
	//   - sr_completions has no direct person column (it links via assignment_id
	//     to the already-pseudonymised sr_assignments), so it is exported raw.
	// A true per-person DSAR (out of scope here) would export these fully.
	if moduleEnabled(modulesEnabled, "vaktaware") {
		salt := make([]byte, 16)
		_, _ = rand.Read(salt)

		// Directory PII — raw.
		if err := writeEntries([]entry{awareTargetsEntry}); err != nil {
			return nil, err
		}

		// Phishing results — pseudonymise the person link (target_id).
		for _, e := range awarePseudoEntries {
			data, err := queryToJSONPseudonymised(ctx, db, orgID, e.query, salt, map[string]bool{"target_id": true})
			if err != nil {
				log.Warn().Err(err).Str("file", e.file).Msg("dataexport: table export failed, writing empty array")
				data = []byte("[]")
			}
			if err := writeRaw(zw, e.file, data); err != nil {
				return nil, fmt.Errorf("%s: %w", e.file, err)
			}
		}

		// Completions — no direct person column; exported raw.
		if err := writeEntries([]entry{awareCompletionsEntry}); err != nil {
			return nil, err
		}
	}

	// Audit log — scoped to org.
	if err := writeEntries([]entry{auditEntry}); err != nil {
		return nil, err
	}

	if err := zw.Close(); err != nil {
		return nil, fmt.Errorf("close zip: %w", err)
	}
	return buf.Bytes(), nil
}

// queryToJSON runs a SELECT query with a single $1 org_id parameter and returns
// the result rows as a JSON array. Uses generic column scanning so callers do not
// need to know the exact schema.
func queryToJSON(ctx context.Context, db *pgxpool.Pool, orgID, query string) ([]byte, error) {
	return queryToJSONPseudonymised(ctx, db, orgID, query, nil, nil)
}

// queryToJSONPseudonymised behaves like queryToJSON but replaces the value of
// each column named in pseudoCols with a salted, non-reversible digest. Used to
// pseudonymise the person link (target_id) in Vakt Aware result tables so an
// org-takeout cannot re-identify who had which phishing result (§87 BetrVG).
//
// This is the single scan loop for the whole export — queryToJSON delegates
// here with an empty pseudoCols rather than keeping a second copy, so a fix to
// the value conversion cannot land in one twin and miss the other.
func queryToJSONPseudonymised(ctx context.Context, db *pgxpool.Pool, orgID, query string, salt []byte, pseudoCols map[string]bool) ([]byte, error) {
	rows, err := db.Query(ctx, query, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	fields := rows.FieldDescriptions()
	// Nicht-nil initialisiert: eine leere Tabelle muss im Archiv als [] stehen,
	// nicht als null — ein Importeur, der ueber die Liste laeuft, faellt sonst
	// ueber genau die Tabellen, die noch keine Daten haben.
	results := []map[string]any{}
	for rows.Next() {
		vals, err := rows.Values()
		if err != nil {
			return nil, err
		}
		row := make(map[string]any, len(fields))
		for i, f := range fields {
			name := string(f.Name)
			v := normalizeValue(f.DataTypeOID, vals[i])
			if pseudoCols[name] && v != nil {
				// Digest the normalised (canonical UUID string) value, not the
				// raw [16]byte — the pseudonym stays stable regardless of how
				// the driver represents the column.
				v = pseudonymise(salt, fmt.Sprintf("%v", v))
			}
			row[name] = v
		}
		results = append(results, row)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return json.Marshal(results)
}

// normalizeValue converts one scanned column value into a form that JSON
// represents faithfully.
//
// The conversion is keyed on the column's PostgreSQL type OID rather than on
// the Go type pgx returned, because the Go type alone cannot decide the
// question: `date` and `timestamptz` both arrive as time.Time and must not be
// rendered the same way. The OID is available from rows.FieldDescriptions()
// exactly here, at the seam between scanning and serialising. One step later —
// inside json.Marshal, via a wrapper type — it is gone; one step earlier, at
// Scan time, we would have to hand-maintain a destination type per column for
// 22 tables, which is the schema coupling this generic export exists to avoid.
//
// Werte, deren OID keine Sonderbehandlung braucht, fallen in den
// Go-Typ-Zweig derselben Funktion und rekursieren dort in Arrays und jsonb,
// wo es keine Element-OID mehr gibt.
func normalizeValue(oid uint32, v any) any {
	if v == nil {
		return nil
	}
	switch oid {
	case pgtype.UUIDOID:
		// pgx hands uuid back as [16]byte; json.Marshal renders that as a
		// 16-element number array, which made every foreign key in the archive
		// unresolvable (R1-20-08).
		if b, ok := v.([16]byte); ok {
			return uuid.UUID(b).String()
		}
	case pgtype.ByteaOID:
		// PostgreSQL hex literal — directly re-insertable, and self-describing
		// as binary. Go's default (Base64) is neither.
		if b, ok := v.([]byte); ok {
			return `\x` + hex.EncodeToString(b)
		}
	case pgtype.DateOID:
		// A calendar date must not carry a fake midnight-UTC time: re-importing
		// "2026-08-07T00:00:00Z" in another timezone can shift the day.
		if t, ok := v.(time.Time); ok {
			return t.UTC().Format("2006-01-02")
		}
	case pgtype.TimestamptzOID, pgtype.TimestampOID:
		// RFC3339 UTC is the project-wide timestamp convention; without the
		// explicit .UTC() the archive would carry the server's session offset.
		if t, ok := v.(time.Time); ok {
			return t.UTC().Format(time.RFC3339Nano)
		}
	}

	// Fallback nach Go-Typ. Das ist zugleich der einzige Weg INNERHALB von
	// Containern: `text[]`/`uuid[]` kommen als []any an und jsonb als
	// map[string]any, dort gibt es keine Element-OID mehr — die Rekursion ruft
	// sich deshalb mit oid 0 auf und landet direkt hier.
	switch x := v.(type) {
	case [16]byte:
		return uuid.UUID(x).String()
	case []byte:
		return `\x` + hex.EncodeToString(x)
	case time.Time:
		return x.UTC().Format(time.RFC3339Nano)
	case []any:
		// An Ort und Stelle ersetzen statt umzukopieren: der Wert kommt frisch
		// aus rows.Values() fuer genau diese Zeile und wird unmittelbar danach
		// in die Ergebniskarte geschrieben, es gibt also keinen zweiten Halter.
		for i, e := range x {
			x[i] = normalizeValue(0, e)
		}
		return x
	case map[string]any:
		for k, e := range x {
			x[k] = normalizeValue(0, e)
		}
		return x
	}
	return v
}

// RenderableOIDs lists the PostgreSQL column types this export knows how to put
// into JSON faithfully — either because normalizeValue converts them (uuid,
// bytea, date, timestamp, timestamptz) or because the Go value pgx returns
// already marshals correctly (text, bool, the integer types, numeric, float,
// json/jsonb, inet, and text/varchar/uuid arrays).
//
// `interval` and `time` are deliberately ABSENT. pgx returns them as
// pgtype.Interval / pgtype.Time, which json.Marshal renders as their internal
// struct — {"Microseconds":7200000000,"Days":1,"Months":0,"Valid":true}. No
// exported table uses either type today (measured against the live schema), so
// encoding them now would be speculative code. Instead the schema-guard test
// asserts that every column of every exported table appears in this map: the
// day a migration adds an interval column to, say, ck_controls, that test goes
// red and names the column, rather than the archive silently gaining noise.
//
// Exported solely so that guard test (in package integration_test) can read the
// same set the code uses, instead of keeping a second copy that drifts.
var RenderableOIDs = map[uint32]string{
	pgtype.BoolOID:             "bool",
	pgtype.ByteaOID:            "bytea",
	pgtype.Int8OID:             "int8",
	pgtype.Int2OID:             "int2",
	pgtype.Int4OID:             "int4",
	pgtype.TextOID:             "text",
	pgtype.JSONOID:             "json",
	pgtype.InetOID:             "inet",
	pgtype.Float4OID:           "float4",
	pgtype.Float8OID:           "float8",
	pgtype.VarcharOID:          "varchar",
	pgtype.DateOID:             "date",
	pgtype.TimestampOID:        "timestamp",
	pgtype.TimestamptzOID:      "timestamptz",
	pgtype.NumericOID:          "numeric",
	pgtype.UUIDOID:             "uuid",
	pgtype.JSONBOID:            "jsonb",
	pgtype.TextArrayOID:        "text[]",
	pgtype.VarcharArrayOID:     "varchar[]",
	pgtype.UUIDArrayOID:        "uuid[]",
	pgtype.TimestamptzArrayOID: "timestamptz[]",
}

// pseudonymise returns a salted, non-reversible 16-char hex digest of value.
// The salt is generated per export and never leaves the process, so the digest
// cannot be re-computed from any exported plaintext id.
func pseudonymise(salt []byte, value string) string {
	h := sha256.New()
	h.Write(salt)
	h.Write([]byte(value))
	return "anon_" + hex.EncodeToString(h.Sum(nil)[:8])
}

// writeJSON marshals v to JSON and writes it as a zip entry.
func writeJSON(zw *zip.Writer, name string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return writeRaw(zw, name, data)
}

// writeRaw writes pre-serialised bytes as a named zip entry.
func writeRaw(zw *zip.Writer, name string, data []byte) error {
	f, err := zw.Create(name)
	if err != nil {
		return err
	}
	_, err = f.Write(data)
	return err
}
