//go:build integration

package integration_test

import (
	"archive/zip"
	"bytes"
	"context"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/matharnica/vakt/internal/modules/vaktcomply"
	"github.com/matharnica/vakt/internal/shared/audit"
)

// TestEvidenceUpload_LandsInAuditPackage pins R1-KW-12 (HIGH, Kernweg 9).
//
// „Nachweis hinzufügen → Dokument" posts to POST /controls/:id/evidence/upload.
// That route used to write the file only into ck_evidence.file_path under
// <upload>/<org>/evidence/, while the audit package and the download route read
// attachments (ck_evidence_files). The document was in evidence.csv, but neither
// in the ZIP nor downloadable. The test covers three paths:
//
//  1. a new upload becomes an attachment of the new evidence record and is in the ZIP,
//  2. a document uploaded the old way (production has such rows) is in the ZIP too,
//     while a stored path pointing outside the org's legacy directory reads nothing,
//  3. a rejected file type leaves no evidence record behind (evidence is append-only).
func TestEvidenceUpload_LandsInAuditPackage(t *testing.T) {
	pool, orgID, cleanup := bootPostgresWithOrg(t)
	defer cleanup()
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	uploadDir := t.TempDir()
	t.Setenv("VAKT_UPLOAD_DIR", uploadDir) // audit.GeneratePackage resolves it the same way

	var userID string
	require.NoError(t, pool.QueryRow(ctx, `
		INSERT INTO users (email, password_hash, display_name)
		VALUES ('kw12@upload.test', 'x', 'KW12') RETURNING id::text`).Scan(&userID))
	fwID := seedFramework(ctx, t, pool, orgID, "ISO 27001")
	ctrlID := seedControl(ctx, t, pool, orgID, fwID, "A.5.2", "Organizational", "")

	svc := vaktcomply.NewService(pool)
	h := vaktcomply.NewHandler(svc).WithDB(pool)
	h.WithEvidenceFileService(vaktcomply.NewEvidenceFileService(svc.Repo(), uploadDir))

	upload := func(title, filename string, content []byte) *httptest.ResponseRecorder {
		var body bytes.Buffer
		mw := multipart.NewWriter(&body)
		require.NoError(t, mw.WriteField("title", title))
		fw, err := mw.CreateFormFile("file", filename)
		require.NoError(t, err)
		_, err = fw.Write(content)
		require.NoError(t, err)
		require.NoError(t, mw.Close())

		e := echo.New()
		req := httptest.NewRequest(http.MethodPost, "/", &body)
		req.Header.Set(echo.HeaderContentType, mw.FormDataContentType())
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		c.SetParamNames("id")
		c.SetParamValues(ctrlID)
		c.Set("org_id", orgID)
		c.Set("user_id", userID)
		require.NoError(t, h.UploadEvidence(c))
		return rec
	}

	// 1. New upload → attachment of the new evidence record.
	newPDF := []byte("%PDF-1.4\n% kw12 neuer Upload\n")
	rec := upload("Informationssicherheitsleitlinie", "leitlinie.pdf", newPDF)
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())

	var evID, filePath string
	require.NoError(t, pool.QueryRow(ctx, `
		SELECT id::text, COALESCE(file_path, '') FROM ck_evidence
		WHERE org_id = $1::uuid AND title = 'Informationssicherheitsleitlinie'`, orgID).Scan(&evID, &filePath))
	assert.Empty(t, filePath, "the old storage column stays empty — the file is an attachment now")
	var attached int
	require.NoError(t, pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM ck_evidence_files
		WHERE org_id = $1::uuid AND evidence_id = $2::uuid AND original_name = 'leitlinie.pdf'`,
		orgID, evID).Scan(&attached))
	assert.Equal(t, 1, attached, "the upload must be an attachment of the evidence record it created")

	// 2. Document uploaded the old way: file in <upload>/<org>/evidence/, path in ck_evidence.
	legacyPDF := []byte("%PDF-1.4\n% kw12 Altbestand\n")
	legacyDir := filepath.Join(uploadDir, orgID, "evidence")
	require.NoError(t, os.MkdirAll(legacyDir, 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(legacyDir, "0b7c1e1e-legacy.pdf"), legacyPDF, 0o600))
	// A stored path that points somewhere else must not be followed.
	outside := filepath.Join(uploadDir, "outside-secret.txt")
	require.NoError(t, os.WriteFile(outside, []byte("geheim"), 0o600))
	_, err := pool.Exec(ctx, `
		INSERT INTO ck_evidence (control_id, org_id, title, file_path, uploaded_by) VALUES
		($1::uuid, $2::uuid, 'Altes Backup-Protokoll', $3, $4::uuid),
		($1::uuid, $2::uuid, 'Verirrter Pfad', $5, $4::uuid)`,
		ctrlID, orgID, "./data/uploads/"+orgID+"/evidence/0b7c1e1e-legacy.pdf", userID, outside)
	require.NoError(t, err)

	pkg, err := audit.GeneratePackage(ctx, pool, orgID)
	require.NoError(t, err)
	zr, err := zip.NewReader(bytes.NewReader(pkg.Zip), int64(len(pkg.Zip)))
	require.NoError(t, err)
	entries := map[string][]byte{}
	for _, f := range zr.File {
		rc, err := f.Open()
		require.NoError(t, err)
		b, err := io.ReadAll(rc)
		require.NoError(t, err)
		_ = rc.Close()
		entries[f.Name] = b
	}
	assert.Equal(t, newPDF, entries["evidence/leitlinie.pdf"], "new upload in the ZIP, byte for byte")
	assert.Equal(t, legacyPDF, entries["evidence/Altes Backup-Protokoll.pdf"], "old-route document in the ZIP, byte for byte")
	for name, b := range entries {
		assert.NotEqual(t, "geheim", string(b), "%s: a stored path outside the legacy directory was followed", name)
	}

	// 3. Rejected file type: 400 and no evidence record.
	rec = upload("Ausführbare Datei", "tool.exe", []byte("MZ\x90\x00"))
	assert.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	var rejected int
	require.NoError(t, pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM ck_evidence WHERE org_id = $1::uuid AND title = 'Ausführbare Datei'`, orgID).Scan(&rejected))
	assert.Zero(t, rejected, "a rejected file must not leave an evidence record behind")
}
