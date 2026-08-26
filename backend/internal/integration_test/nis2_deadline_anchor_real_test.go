//go:build integration

// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package integration_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/matharnica/vakt/internal/modules/vaktcomply"
)

// R1-14c-08 / R1-14c-09: the NIS2 Art. 23 assessment endpoint had two defects
// that together made the reporting obligation undeterminable and every deadline
// wrong.
//
//	R1-14c-08: the handler bound a FLAT body (embedded NIS2ReportabilityCheck
//	without a json tag promotes the three criteria to the top level), while the
//	frontend and openapi.yaml both send {"detected_at": …, "check": {…}}. The
//	three criteria never arrived, so is_reportable was permanently false and the
//	"Meldepflichtig" badge could not appear.
//
//	R1-14c-09: the deadline anchor was time.Now(), and ck_incidents.discovered_at
//	was never read. An incident discovered three days ago was shown a 24-hour
//	deadline starting now, although the legal deadline had expired two days
//	earlier.
//
// Both are exercised here through the REAL handler against real Postgres, with
// the exact payload the frontend sends. A unit test on the check struct cannot
// see either defect: the binding happens in echo, and the anchor comes from the
// database row.
func TestNIS2AssessReportability_AnchorAndPayload(t *testing.T) {
	pool, orgID, cleanup := bootPostgresWithOrg(t)
	defer cleanup()
	ctx := context.Background()

	svc := vaktcomply.NewService(pool)
	h := vaktcomply.NewHandler(svc)

	// seedIncident creates an incident with an explicit discovery time.
	seedIncident := func(t *testing.T, title string, discoveredAt time.Time) string {
		t.Helper()
		var id string
		require.NoError(t, pool.QueryRow(ctx, `
			INSERT INTO ck_incidents (org_id, title, severity, status, discovered_at)
			VALUES ($1::uuid, $2, 'high', 'open', $3)
			RETURNING id::text`, orgID, title, discoveredAt).Scan(&id))
		return id
	}

	// assess posts the EXACT body the frontend sends and returns the recorder.
	assess := func(t *testing.T, incidentID, body string) *httptest.ResponseRecorder {
		t.Helper()
		e := echo.New()
		req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		c.Set("org_id", orgID)
		c.SetParamNames("id")
		c.SetParamValues(incidentID)
		require.NoError(t, h.NIS2AssessReportability(c))
		return rec
	}

	// The real frontend payload: nested "check". detected_at is what the old
	// frontend also sent; it must be IGNORED, not used as the anchor.
	frontendBody := func(disruption, thirdParties, financial bool) string {
		b, err := json.Marshal(map[string]any{
			"detected_at": time.Now().UTC().Format(time.RFC3339),
			"check": map[string]bool{
				"causes_significant_disruption": disruption,
				"affects_third_parties":         thirdParties,
				"causes_financial_damage":       financial,
			},
		})
		require.NoError(t, err)
		return string(b)
	}

	readDeadlines := func(t *testing.T, incidentID string) (detected, ew, full, final time.Time, reportable bool) {
		t.Helper()
		require.NoError(t, pool.QueryRow(ctx, `
			SELECT nis2_detected_at, nis2_early_warning_due, nis2_full_report_due,
			       nis2_final_report_due, nis2_reportable
			  FROM ck_incidents WHERE id = $1::uuid AND org_id = $2::uuid`,
			incidentID, orgID).Scan(&detected, &ew, &full, &final, &reportable))
		return
	}

	t.Run("R1-14c-08 der echte Frontend-Payload setzt die Meldepflicht", func(t *testing.T) {
		id := seedIncident(t, "Ransomware auf Fileserver", time.Now().UTC().Add(-2*time.Hour))

		rec := assess(t, id, frontendBody(true, false, false))
		require.Equal(t, http.StatusOK, rec.Code)

		var out struct {
			IsReportable bool      `json:"is_reportable"`
			DetectedAt   time.Time `json:"detected_at"`
		}
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out))
		assert.True(t, out.IsReportable,
			"der verschachtelte check kam nicht an — is_reportable war strukturell immer false, "+
				"das Badge \"Meldepflichtig\" konnte nie erscheinen")

		_, _, _, _, reportable := readDeadlines(t, id)
		assert.True(t, reportable, "nis2_reportable muss auch in der Datenbank stehen, nicht nur in der Antwort")
	})

	t.Run("R1-14c-08 kein erfuelltes Kriterium bleibt korrekt nicht meldepflichtig", func(t *testing.T) {
		id := seedIncident(t, "Kurzer Ausfall ohne Folgen", time.Now().UTC().Add(-1*time.Hour))

		rec := assess(t, id, frontendBody(false, false, false))
		require.Equal(t, http.StatusOK, rec.Code)

		var out struct {
			IsReportable bool `json:"is_reportable"`
		}
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out))
		assert.False(t, out.IsReportable,
			"ohne erfuelltes Kriterium darf nichts meldepflichtig werden — sonst waere der Fix "+
				"nur ein hartkodiertes true")
	})

	t.Run("Baseline: ein frisch entdeckter Vorfall bekommt eine laufende Frist", func(t *testing.T) {
		discovered := time.Now().UTC().Add(-30 * time.Minute)
		id := seedIncident(t, "Frischer Vorfall", discovered)

		require.Equal(t, http.StatusOK, assess(t, id, frontendBody(true, false, false)).Code)

		detected, ew, _, _, _ := readDeadlines(t, id)
		assert.WithinDuration(t, discovered, detected, time.Second,
			"der Anker muss discovered_at sein")
		assert.True(t, ew.After(time.Now().UTC()),
			"eine vor 30 Minuten entdeckte Meldung hat noch rund 23,5 Stunden — die Frist muss laufen")
		assert.WithinDuration(t, discovered.Add(24*time.Hour), ew, time.Second)
	})

	t.Run("R1-14c-09 ein vor drei Tagen entdeckter Vorfall hat eine ABGELAUFENE Frist", func(t *testing.T) {
		discovered := time.Now().UTC().Add(-72 * time.Hour)
		id := seedIncident(t, "Vor drei Tagen entdeckt", discovered)

		rec := assess(t, id, frontendBody(true, false, false))
		require.Equal(t, http.StatusOK, rec.Code)

		detected, ew, full, final, _ := readDeadlines(t, id)

		assert.WithinDuration(t, discovered, detected, time.Second,
			"der Anker war time.Now() statt discovered_at — die Frist begann im Moment der Bewertung")
		assert.WithinDuration(t, discovered.Add(24*time.Hour), ew, time.Second)
		assert.WithinDuration(t, discovered.Add(72*time.Hour), full, time.Second)

		now := time.Now().UTC()
		assert.True(t, ew.Before(now),
			"die 24-Stunden-Fruehwarnung eines vor drei Tagen entdeckten Vorfalls ist gesetzlich "+
				"seit zwei Tagen abgelaufen und MUSS als abgelaufen angezeigt werden — vorher zeigte "+
				"die Oberflaeche eine laufende Frist und fuehrte den Betreiber in die Fristversaeumnis")
		assert.True(t, full.Before(now) || full.Equal(now) || full.Sub(now) < time.Minute,
			"auch die 72-Stunden-Meldung ist zum Bewertungszeitpunkt faellig")
		assert.True(t, final.After(now),
			"der Abschlussbericht laeuft noch — ein Monat ab Entdeckung")
	})

	t.Run("R1-14c-09 die Ein-Monats-Frist ist ein Kalendermonat, keine 30 Tage", func(t *testing.T) {
		// 31. Januar ist der Fall, an dem beides auseinanderlaeuft: +30 Tage
		// waere der 2. Maerz, ein Kalendermonat mit Klemmung ist der 28. Februar.
		discovered := time.Date(2026, 1, 31, 9, 0, 0, 0, time.UTC)
		id := seedIncident(t, "Monatsende-Vorfall", discovered)

		require.Equal(t, http.StatusOK, assess(t, id, frontendBody(true, false, false)).Code)

		_, _, _, final, _ := readDeadlines(t, id)
		assert.Equal(t, time.Date(2026, 2, 28, 9, 0, 0, 0, time.UTC), final.UTC(),
			"ein Monat ist ein Kalenderzeitraum: 31.01. + 1 Monat ist der 28.02. — "+
				"weder der 2. Maerz (30 Tage) noch der 3. Maerz (time.AddDate normalisiert statt zu klemmen)")
		assert.NotEqual(t, discovered.Add(30*24*time.Hour), final.UTC(),
			"30 Tage sind kein Monat")
	})

	t.Run("Ohne brauchbares discovered_at wird die Bewertung ehrlich abgelehnt", func(t *testing.T) {
		// Die Spalte ist NOT NULL, aber die Go-Nullzeit ist schreibbar — genau so
		// entsteht ein Vorfall, der jede Frist ab dem Jahr 1 rechnen wuerde.
		id := seedIncident(t, "Ohne Entdeckungszeitpunkt", time.Time{})

		rec := assess(t, id, frontendBody(true, false, false))
		assert.Equal(t, http.StatusUnprocessableEntity, rec.Code,
			"kein stiller Ersatzwert: weder now() (plausible falsche Frist) noch die Nullzeit "+
				"(Frist seit zwei Jahrtausenden abgelaufen) — die Bewertung wird abgelehnt")
		assert.Contains(t, rec.Body.String(), "CK_NIS2_NO_DISCOVERY_TIME")

		// Und es darf nichts geschrieben worden sein.
		var n int
		require.NoError(t, pool.QueryRow(ctx, `
			SELECT count(*) FROM ck_incidents
			 WHERE id = $1::uuid AND org_id = $2::uuid AND nis2_early_warning_due IS NOT NULL`,
			id, orgID).Scan(&n))
		assert.Zero(t, n, "eine abgelehnte Bewertung darf keine halbe Frist hinterlassen")
	})

	t.Run("Ein fremder Vorfall bleibt 404 — die Org-Grenze haelt", func(t *testing.T) {
		rec := assess(t, "00000000-0000-0000-0000-000000000000", frontendBody(true, false, false))
		assert.Equal(t, http.StatusNotFound, rec.Code)
	})
}
