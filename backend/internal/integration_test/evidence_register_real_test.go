//go:build integration

package integration_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/matharnica/vakt/internal/modules/vaktcomply"
)

// TestEvidenceRegister_OrgWideList pins the page „Nachweise" (Kernweg 9):
// GET /vaktcomply/evidence lists every evidence record of the caller's org with
// control code, framework and whether a file is attached — and nothing of
// another org.
func TestEvidenceRegister_OrgWideList(t *testing.T) {
	pool, orgID, cleanup := bootPostgresWithOrg(t)
	defer cleanup()
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	var otherOrg, userID string
	require.NoError(t, pool.QueryRow(ctx, `
		INSERT INTO organizations (name, slug) VALUES ('Fremd', 'fremd') RETURNING id::text`).Scan(&otherOrg))
	require.NoError(t, pool.QueryRow(ctx, `
		INSERT INTO users (email, password_hash, display_name)
		VALUES ('register@evidence.test', 'x', 'Reg') RETURNING id::text`).Scan(&userID))

	fwISO := seedFramework(ctx, t, pool, orgID, "ISO 27001")
	fwNIS := seedFramework(ctx, t, pool, orgID, "NIS2")
	a52 := seedControl(ctx, t, pool, orgID, fwISO, "A.5.2", "Organizational", "")
	nis := seedControl(ctx, t, pool, orgID, fwNIS, "NIS2-A.1", "Governance", "")
	fwOther := seedFramework(ctx, t, pool, otherOrg, "ISO 27001")
	otherCtl := seedControl(ctx, t, pool, otherOrg, fwOther, "A.5.2", "Organizational", "")

	var withFile, withoutFile string
	require.NoError(t, pool.QueryRow(ctx, `
		INSERT INTO ck_evidence (org_id, control_id, title, expires_at)
		VALUES ($1::uuid, $2::uuid, 'Rollenkonzept', NOW() + INTERVAL '10 days') RETURNING id::text`,
		orgID, a52).Scan(&withFile))
	require.NoError(t, pool.QueryRow(ctx, `
		INSERT INTO ck_evidence (org_id, control_id, title)
		VALUES ($1::uuid, $2::uuid, 'Notiz ohne Datei') RETURNING id::text`,
		orgID, nis).Scan(&withoutFile))
	_, err := pool.Exec(ctx, `
		INSERT INTO ck_evidence_files (org_id, evidence_id, control_id, original_name, stored_name, mime_type, size_bytes, uploaded_by)
		VALUES ($1::uuid, $2::uuid, $3::uuid, 'rollen.pdf', 'x.pdf', 'application/pdf', 10, $4::uuid)`,
		orgID, withFile, a52, userID)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `
		INSERT INTO ck_evidence (org_id, control_id, title) VALUES ($1::uuid, $2::uuid, 'Geheim fremd')`,
		otherOrg, otherCtl)
	require.NoError(t, err)

	h := vaktcomply.NewHandler(vaktcomply.NewService(pool)).WithDB(pool)
	list := func(org string) []vaktcomply.EvidenceListItem {
		e := echo.New()
		rec := httptest.NewRecorder()
		c := e.NewContext(httptest.NewRequest(http.MethodGet, "/", nil), rec)
		c.Set("org_id", org)
		c.Set("user_id", userID)
		require.NoError(t, h.ListOrgEvidence(c))
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		var items []vaktcomply.EvidenceListItem
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &items))
		return items
	}

	items := list(orgID)
	require.Len(t, items, 2, "both evidence records of the org, none of the other org")
	byID := map[string]vaktcomply.EvidenceListItem{}
	for _, it := range items {
		assert.NotEqual(t, "Geheim fremd", it.Title, "evidence of another org leaked")
		byID[it.ID] = it
	}
	f := byID[withFile]
	assert.Equal(t, "A.5.2", f.ControlRef)
	assert.Equal(t, a52, f.ControlID)
	assert.Equal(t, "ISO 27001", f.FrameworkName)
	assert.Equal(t, fwISO, f.FrameworkID)
	assert.Equal(t, 1, f.FileCount)
	assert.True(t, f.HasFile)
	assert.NotNil(t, f.ExpiresAt)

	n := byID[withoutFile]
	assert.Equal(t, "NIS2-A.1", n.ControlRef)
	assert.Equal(t, "NIS2", n.FrameworkName)
	assert.Zero(t, n.FileCount)
	assert.False(t, n.HasFile)

	other := list(otherOrg)
	require.Len(t, other, 1)
	assert.Equal(t, "Geheim fremd", other[0].Title)
}
