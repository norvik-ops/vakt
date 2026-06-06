// Package dataexport provides a full data export endpoint for DSGVO compliance
// and migration safety. Customers can export all their org data as a ZIP of JSON files.
package dataexport

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog/log"
)

// vaktVersion is set at build time; falls back to "dev".
const vaktVersion = "dev"

// safeNameRe strips characters that are unsafe in filenames.
var safeNameRe = regexp.MustCompile(`[^a-zA-Z0-9_-]`)

// ExportHandler returns an Echo handler that streams a full-data ZIP to the client.
func ExportHandler(db *pgxpool.Pool) echo.HandlerFunc {
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

		zipBytes, err := buildZip(ctx, db, orgID, orgName)
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
func buildZip(ctx context.Context, db *pgxpool.Pool, orgID, orgName string) ([]byte, error) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)

	// meta.json
	meta := map[string]string{
		"export_date":  time.Now().UTC().Format(time.RFC3339),
		"org_id":       orgID,
		"org_name":     orgName,
		"vakt_version": vaktVersion,
	}
	if err := writeJSON(zw, "meta.json", meta); err != nil {
		return nil, fmt.Errorf("meta: %w", err)
	}

	// SecVitals tables (ck_ prefix)
	vitalsEntries := []struct {
		file  string
		query string
	}{
		{"frameworks.json", `SELECT * FROM ck_frameworks WHERE org_id = $1::uuid ORDER BY created_at`},
		{"controls.json", `SELECT * FROM ck_controls WHERE org_id = $1::uuid ORDER BY created_at`},
		{"evidence.json", `SELECT * FROM ck_evidence WHERE org_id = $1::uuid ORDER BY created_at`},
		{"risks.json", `SELECT * FROM ck_risks WHERE org_id = $1::uuid ORDER BY created_at`},
		{"incidents.json", `SELECT * FROM ck_incidents WHERE org_id = $1::uuid ORDER BY created_at`},
		{"policies.json", `SELECT * FROM ck_policies WHERE org_id = $1::uuid ORDER BY created_at`},
		{"capas.json", `SELECT * FROM ck_capas WHERE org_id = $1::uuid ORDER BY created_at`},
		{"tasks.json", `SELECT * FROM ck_tasks WHERE org_id = $1::uuid ORDER BY created_at`},
		{"comments.json", `SELECT * FROM ck_comments WHERE org_id = $1::uuid ORDER BY created_at`},
	}

	for _, e := range vitalsEntries {
		data, err := queryToJSON(ctx, db, orgID, e.query)
		if err != nil {
			// Non-fatal: write an empty array so the file still exists.
			data = []byte("[]")
		}
		if err := writeRaw(zw, e.file, data); err != nil {
			return nil, fmt.Errorf("%s: %w", e.file, err)
		}
	}

	// SecPrivacy tables (po_ prefix)
	privacyEntries := []struct {
		file  string
		query string
	}{
		{"vvt.json", `SELECT * FROM po_vvt_entries WHERE org_id = $1::uuid ORDER BY created_at`},
		{"dpias.json", `SELECT * FROM po_dpias WHERE org_id = $1::uuid ORDER BY created_at`},
		{"avv.json", `SELECT * FROM po_avvs WHERE org_id = $1::uuid ORDER BY created_at`},
		{"breaches.json", `SELECT * FROM po_breaches WHERE org_id = $1::uuid ORDER BY created_at`},
	}

	for _, e := range privacyEntries {
		data, err := queryToJSON(ctx, db, orgID, e.query)
		if err != nil {
			data = []byte("[]")
		}
		if err := writeRaw(zw, e.file, data); err != nil {
			return nil, fmt.Errorf("%s: %w", e.file, err)
		}
	}

	// Audit log — scoped to org.
	auditData, err := queryToJSON(ctx, db, orgID,
		`SELECT * FROM audit_log WHERE org_id = $1::uuid AND deleted_at IS NULL ORDER BY created_at`)
	if err != nil {
		auditData = []byte("[]")
	}
	if err := writeRaw(zw, "audit_log.json", auditData); err != nil {
		return nil, fmt.Errorf("audit_log: %w", err)
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
	rows, err := db.Query(ctx, query, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	fields := rows.FieldDescriptions()
	var results []map[string]any
	for rows.Next() {
		vals, err := rows.Values()
		if err != nil {
			return nil, err
		}
		row := make(map[string]any, len(fields))
		for i, f := range fields {
			row[string(f.Name)] = vals[i]
		}
		results = append(results, row)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if results == nil {
		results = []map[string]any{}
	}
	return json.Marshal(results)
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
