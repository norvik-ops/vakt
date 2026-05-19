// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package secvitals

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog/log"
	"golang.org/x/sync/errgroup"
)

// GetBoardReport handles GET /api/v1/secvitals/board-report
// Returns a PDF summary for management review.
func (h *Handler) GetBoardReport(c echo.Context) error {
	ctx := c.Request().Context()
	oid := orgID(c)

	data, err := h.service.GetBoardReportData(ctx, oid)
	if err != nil {
		log.Error().Err(err).Msg("board report: gather data")
		return errResp(c, http.StatusInternalServerError, "failed to gather board report data", "CK_BOARD_REPORT_FAILED")
	}

	pdfBytes, err := GenerateBoardReportPDF(*data)
	if err != nil {
		log.Error().Err(err).Msg("board report: generate pdf")
		return errResp(c, http.StatusInternalServerError, "failed to generate board report PDF", "CK_BOARD_REPORT_FAILED")
	}

	filename := fmt.Sprintf("vakt-board-report-%s.pdf", data.GeneratedAt.Format("2006-01-02"))
	c.Response().Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename=%q`, filename))
	return c.Blob(http.StatusOK, "application/pdf", pdfBytes)
}

// GetBoardReportPDF generates a board report PDF for the given org and returns
// the raw bytes. It satisfies the scheduledreports.BoardReportProvider interface.
func (s *Service) GetBoardReportPDF(ctx context.Context, orgID string) ([]byte, error) {
	data, err := s.GetBoardReportData(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("board report data: %w", err)
	}
	return GenerateBoardReportPDF(*data)
}

// GetBoardReportData collects all data required for the Board Report PDF.
// The six independent data sources are fetched in parallel using errgroup.
func (s *Service) GetBoardReportData(ctx context.Context, orgID string) (*BoardReportData, error) {
	d := &BoardReportData{GeneratedAt: time.Now().UTC()}

	g, gctx := errgroup.WithContext(ctx)

	// 1. Org name (soft-fail — never blocks the report).
	g.Go(func() error {
		_ = s.db.QueryRow(gctx, `SELECT name FROM organizations WHERE id=$1::uuid`, orgID).Scan(&d.OrgName)
		if d.OrgName == "" {
			d.OrgName = orgID
		}
		return nil
	})

	// 2. Compliance score: weighted average of implemented/total controls across all frameworks.
	var (
		scoreMu     sync.Mutex
		totalWeight float64
		weightedSum float64
	)
	g.Go(func() error {
		rows, err := s.db.Query(gctx, `
			SELECT
			    COUNT(c.id)::int                                                     AS total,
			    COUNT(c.id) FILTER (WHERE c.manual_status = 'implemented')::int     AS implemented
			FROM ck_frameworks f
			LEFT JOIN ck_controls c ON c.framework_id = f.id AND c.org_id = f.org_id
			WHERE f.org_id = $1::uuid
			GROUP BY f.id
		`, orgID)
		if err != nil {
			// Non-fatal: leave score at 0.
			return nil //nolint:nilerr
		}
		defer rows.Close()
		for rows.Next() {
			var total, implemented int
			if err := rows.Scan(&total, &implemented); err != nil {
				continue
			}
			if total > 0 {
				score := float64(implemented) / float64(total) * 100
				scoreMu.Lock()
				weightedSum += score * float64(total)
				totalWeight += float64(total)
				scoreMu.Unlock()
			}
		}
		return rows.Err()
	})

	// 3. Previous score from score_history (most recent snapshot before today).
	g.Go(func() error {
		var prevScore int
		_ = s.db.QueryRow(gctx, `
			SELECT score FROM ck_score_history
			WHERE org_id = $1::uuid AND recorded_at < NOW()::date
			ORDER BY recorded_at DESC
			LIMIT 1
		`, orgID).Scan(&prevScore)
		d.ScorePrevious = prevScore
		return nil
	})

	// 4. Open risks.
	g.Go(func() error {
		risks, err := s.ListRisks(gctx, orgID)
		if err != nil {
			return nil //nolint:nilerr
		}
		var openRisks, criticalRisks int
		for _, r := range risks {
			if r.Status == "open" {
				openRisks++
				if r.RiskScore >= 15 {
					criticalRisks++
				}
			}
		}
		d.OpenRisks = openRisks
		d.CriticalRisks = criticalRisks
		return nil
	})

	// 5. Open & overdue CAPAs.
	g.Go(func() error {
		capas, err := s.ListCAPAs(gctx, orgID, "")
		if err != nil {
			return nil //nolint:nilerr
		}
		now := time.Now()
		var openCAPAs, overdueCAPAs int
		for _, ca := range capas {
			if ca.Status == "open" || ca.Status == "in_progress" {
				openCAPAs++
				if ca.DueDate != nil && ca.DueDate.Before(now) {
					overdueCAPAs++
				}
			}
		}
		d.OpenCAPAs = openCAPAs
		d.OverdueCAPAs = overdueCAPAs
		return nil
	})

	// 6. Incidents in the last 30 days.
	g.Go(func() error {
		since := time.Now().UTC().Add(-30 * 24 * time.Hour)
		_ = s.db.QueryRow(gctx, `
			SELECT COUNT(*)::int FROM ck_incidents
			WHERE org_id = $1::uuid AND created_at >= $2
		`, orgID, since).Scan(&d.RecentIncidents)
		return nil
	})

	if err := g.Wait(); err != nil {
		return nil, err
	}

	// Apply weighted score now that goroutine 2 has finished.
	if totalWeight > 0 {
		d.Score = int(weightedSum / totalWeight)
	}

	return d, nil
}
