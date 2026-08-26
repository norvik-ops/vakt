//go:build integration

package integration_test

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/csv"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go/modules/postgres"

	"github.com/matharnica/vakt/internal/shared/audit"
	shareddb "github.com/matharnica/vakt/internal/shared/db"
)

// TestAuditPackage_ControlStatusReflectsManualStatus pins R1-14b-A1 / R1-20-A12:
// the audit deliverable must report the status the user actually maintains.
//
// ck_controls.status was a column no code path ever wrote (migration 153 added
// it with DEFAULT 'missing' purely so the export queries would stop erroring),
// while every write path — the UI, the approval workflow, the NIS2 auto-mapping
// and the demo seed — writes ck_controls.manual_status. controls.csv and
// gap_analysis.html read c.status, so both reported "missing" / "Fehlend" for
// all controls of every org, including controls explicitly set to implemented.
//
// The test asserts the two ZIP members that the customer hands to an auditor
// agree with manual_status/not_applicable. Reverting the generated-column
// migration (163) turns every expected value below back into "missing", so all
// four control assertions in controls.csv fail.
func TestAuditPackage_ControlStatusReflectsManualStatus(t *testing.T) {
	if testing.Short() {
		t.Skip("integration: -short mode")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	pgC, err := postgres.Run(ctx,
		imagePostgres,
		postgres.WithDatabase("vakt_test"),
		postgres.WithUsername("vakt"),
		postgres.WithPassword("vakt"),
		postgres.WithSQLDriver("pgx"),
		postgres.BasicWaitStrategies(),
	)
	if err != nil {
		if strings.Contains(err.Error(), "permission denied") ||
			strings.Contains(err.Error(), "Cannot connect to the Docker daemon") {
			t.Skipf("integration: Docker unavailable (%v)", err)
		}
		t.Fatalf("postgres container: %v", err)
	}
	defer func() { _ = pgC.Terminate(ctx) }()

	dsn, err := pgC.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)
	require.NoError(t, shareddb.RunMigrations(dsn, migrationsDir(t)))

	pool, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)
	defer pool.Close()

	var orgID string
	require.NoError(t, pool.QueryRow(ctx, `
		INSERT INTO organizations (name, slug) VALUES ('StatusOrg', 'statusorg')
		RETURNING id::text`).Scan(&orgID))

	fwID := seedFramework(ctx, t, pool, orgID, "ISO 27001")

	// One control per user-reachable state. The FE writes '' | in_progress |
	// implemented into manual_status and toggles not_applicable separately;
	// the NIS2 wizard auto-mapping writes the not_implemented/partial dialect.
	implID := seedControl(ctx, t, pool, orgID, fwID, "A-1", "Access", "implemented")
	seedControl(ctx, t, pool, orgID, fwID, "A-2", "Access", "in_progress")
	seedControl(ctx, t, pool, orgID, fwID, "A-3", "Access", "")
	naID := seedControl(ctx, t, pool, orgID, fwID, "A-4", "Access", "")
	_, err = pool.Exec(ctx,
		`UPDATE ck_controls SET not_applicable = true WHERE org_id = $1::uuid AND id = $2::uuid`,
		orgID, naID)
	require.NoError(t, err)

	// The implemented control also carries evidence — the reported case had
	// maturity 3 and two evidence rows and still exported as "missing".
	_, err = pool.Exec(ctx,
		`UPDATE ck_controls SET maturity_score = 3 WHERE org_id = $1::uuid AND id = $2::uuid`,
		orgID, implID)
	require.NoError(t, err)
	seedEvidence(ctx, t, pool, orgID, implID)
	seedEvidence(ctx, t, pool, orgID, implID)

	pkg, err := audit.GeneratePackage(ctx, pool, orgID)
	require.NoError(t, err)
	require.NotNil(t, pkg)

	zr, err := zip.NewReader(bytes.NewReader(pkg.Zip), int64(len(pkg.Zip)))
	require.NoError(t, err)

	member := func(name string) []byte {
		for _, f := range zr.File {
			if f.Name == name {
				rc, err := f.Open()
				require.NoError(t, err)
				defer func() { _ = rc.Close() }()
				b, err := io.ReadAll(rc)
				require.NoError(t, err)
				return b
			}
		}
		t.Fatalf("zip member %q missing", name)
		return nil
	}

	// ── controls.csv ─────────────────────────────────────────────────────────
	records, err := csv.NewReader(bytes.NewReader(member("controls.csv"))).ReadAll()
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(records), 5, "header + 4 controls")
	require.Equal(t, "Status", records[0][5])

	statusByControl := map[string]string{}
	for _, rec := range records[1:] {
		statusByControl[rec[3]] = rec[5]
	}
	require.Equal(t, "implemented", statusByControl["A-1"],
		"a control set to implemented (maturity 3, 2 evidences) must not export as missing")
	require.Equal(t, "in_progress", statusByControl["A-2"])
	require.Equal(t, "missing", statusByControl["A-3"])
	require.Equal(t, "not_applicable", statusByControl["A-4"])

	// ── gap_analysis.html ────────────────────────────────────────────────────
	// Same source column, second deliverable: the summary block must count the
	// implemented control, and the per-control row must render "Implementiert".
	html := string(member("gap_analysis.html"))
	require.Contains(t, html,
		`<tr><td>Implementiert</td><td><span class="badge implemented">1</span></td></tr>`,
		"gap analysis summary must count the implemented control")
	require.Contains(t, html, `<span class="badge implemented">Implementiert</span>`,
		"gap analysis control table must label the implemented control")
}
