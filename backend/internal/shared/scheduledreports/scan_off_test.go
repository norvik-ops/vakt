// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package scheduledreports

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestFindingsReportArchived covers the R1-KW-15 follow-up: Vakt Scan is
// archived, so no findings report can be created, and a stored one is skipped
// before anything touches the database or SMTP (db is nil here on purpose).
func TestFindingsReportArchived(t *testing.T) {
	svc := NewService(nil, SMTPConfig{})
	h := NewHandler(svc)

	e := echo.New()
	body := `{"name":"Wöchentlich","report_type":"findings","schedule":"weekly","recipients":["a@example.org"],"format":"csv"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/reports/scheduled", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.Set("org_id", "00000000-0000-0000-0000-000000000001")

	require.NoError(t, h.Create(c))
	assert.Equal(t, http.StatusUnprocessableEntity, rec.Code)
	assert.Contains(t, rec.Body.String(), "VALIDATION_ERROR")

	err := svc.RunReport(context.Background(), ScheduledReport{ID: "r1", ReportType: "findings", Format: "csv", Recipients: []string{"a@example.org"}})
	assert.NoError(t, err)
}
