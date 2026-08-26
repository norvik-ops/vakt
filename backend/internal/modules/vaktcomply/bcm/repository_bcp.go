// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package bcm

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	db "github.com/matharnica/vakt/internal/db"
	"github.com/matharnica/vakt/internal/shared/apperr"
)

// CreateBCPPlan inserts a new BCP plan for the given organisation.
func (r *Repository) CreateBCPPlan(ctx context.Context, orgID string, in CreateBCPPlanInput) (BCPPlan, error) {
	status := "draft"
	if in.Status != "" {
		status = in.Status
	}
	version := "1.0"
	if in.Version != "" {
		version = in.Version
	}
	row, err := r.q.CreateCKBCPPlan(ctx, db.CreateCKBCPPlanParams{
		OrgID:               orgID,
		Title:               in.Title,
		Scope:               in.Scope,
		Version:             version,
		Status:              status,
		Owner:               in.Owner,
		RtoHours:            optInt4(in.RTOHours),
		RpoHours:            optInt4(in.RPOHours),
		Schutzbedarfsklasse: optInt4(in.Schutzbedarfsklasse),
	})
	if err != nil {
		return BCPPlan{}, fmt.Errorf("create bcp plan: %w", err)
	}
	return bcpPlanFromRow(row), nil
}

// ListBCPPlans returns all BCP plans for an organisation.
func (r *Repository) ListBCPPlans(ctx context.Context, orgID string) ([]BCPPlan, error) {
	rows, err := r.q.ListCKBCPPlans(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("list bcp plans: %w", err)
	}
	out := make([]BCPPlan, len(rows))
	for i, row := range rows {
		out[i] = bcpPlanFromRow(row)
	}
	return out, nil
}

// GetBCPPlan returns a single BCP plan by ID within an organisation.
func (r *Repository) GetBCPPlan(ctx context.Context, orgID, id string) (BCPPlan, error) {
	row, err := r.q.GetCKBCPPlan(ctx, db.GetCKBCPPlanParams{ID: id, OrgID: orgID})
	if err != nil {
		return BCPPlan{}, fmt.Errorf("get bcp plan: %w", err)
	}
	return bcpPlanFromRow(row), nil
}

// UpdateBCPPlan updates an existing BCP plan with merging PATCH semantics.
//
// REV-ESK12 B1: Lesen, Mergen, Pruefen und Schreiben liegen in EINER
// Transaktion, und der Lesevorgang sperrt die Zeile (GetCKBCPPlanForUpdate,
// `FOR UPDATE`). Getrennt waeren es zwei Fenster fuer denselben stillen Verlust,
// den die mergende Semantik gerade beseitigt:
//
//   - ohne Sperre ueberschreibt ein zweiter, gleichzeitiger PATCH die Aenderung
//     des ersten an einem Feld, das er selbst gar nicht geschickt hat;
//   - ohne die Pruefung AUF DEM GEMERGTEN Zustand kaeme rpo > rto in die Tabelle,
//     sobald nur eines der beiden Felder im Body steht (Bestand rto=4, PATCH
//     rpo=8: die uebergebenen Werte allein sind unauffaellig).
//
// Deshalb prueft validateBCPPlanTargets hier den Zielzustand, nicht der Service
// den Eingabezustand. Der Fehler wird unveraendert durchgereicht, damit
// isBCPPlanInputError im Handler seine Sentinels wiedererkennt.
func (r *Repository) UpdateBCPPlan(ctx context.Context, orgID, id string, in UpdateBCPPlanInput) (BCPPlan, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return BCPPlan{}, fmt.Errorf("update bcp plan: begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	qtx := r.q.WithTx(tx)
	curRow, err := qtx.GetCKBCPPlanForUpdate(ctx, db.GetCKBCPPlanForUpdateParams{ID: id, OrgID: orgID})
	if err != nil {
		return BCPPlan{}, fmt.Errorf("update bcp plan: load current: %w", err)
	}
	next := in.MergeInto(bcpPlanFromRow(curRow))
	if err := validateBCPPlanTargets(next.RTOHours, next.RPOHours, next.Schutzbedarfsklasse); err != nil {
		return BCPPlan{}, err
	}

	row, err := qtx.UpdateCKBCPPlan(ctx, db.UpdateCKBCPPlanParams{
		ID:                  id,
		OrgID:               orgID,
		Title:               next.Title,
		Scope:               next.Scope,
		Version:             next.Version,
		Status:              next.Status,
		Owner:               next.Owner,
		RtoHours:            optInt4(next.RTOHours),
		RpoHours:            optInt4(next.RPOHours),
		Schutzbedarfsklasse: optInt4(next.Schutzbedarfsklasse),
	})
	if err != nil {
		return BCPPlan{}, fmt.Errorf("update bcp plan: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return BCPPlan{}, fmt.Errorf("update bcp plan: commit: %w", err)
	}
	return bcpPlanFromRow(row), nil
}

// DeleteBCPPlan removes a BCP plan and returns an error if not found.
func (r *Repository) DeleteBCPPlan(ctx context.Context, orgID, id string) error {
	n, err := r.q.DeleteCKBCPPlan(ctx, db.DeleteCKBCPPlanParams{ID: id, OrgID: orgID})
	if err != nil {
		return fmt.Errorf("delete bcp plan: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("bcp plan %w", apperr.ErrNotFound)
	}
	return nil
}

// AddBCPTest logs a test result against a BCP plan and carries the plan's
// last_tested_at along.
//
// ESK-12: Das Fortschreiben passiert in DERSELBEN Transaktion wie das Einfuegen.
// Zwei getrennte Anweisungen koennten auseinanderfallen — der Testeintrag stuende
// da, last_tested_at bliebe `null`, und das ist exakt der Ausgangszustand dieses
// Befundes: ein Feld, das genau dann gesetzt gehoert, wenn das Ereignis eintritt,
// und es nicht wird. Ein Fehlschlag beim Fortschreiben muss den Testeintrag mit
// zuruecknehmen, sonst ist die Ableitung nur meistens wahr.
func (r *Repository) AddBCPTest(ctx context.Context, orgID, planID string, in CreateBCPTestInput) (BCPTest, error) {
	var testDate pgtype.Date
	if err := testDate.Scan(in.TestDate); err != nil {
		return BCPTest{}, fmt.Errorf("parse test_date: %w", err)
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return BCPTest{}, fmt.Errorf("add bcp test: begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	qtx := r.q.WithTx(tx)
	row, err := qtx.CreateCKBCPTest(ctx, db.CreateCKBCPTestParams{
		OrgID:    orgID,
		PlanID:   planID,
		TestDate: testDate,
		TestType: in.TestType,
		Outcome:  in.Outcome,
		Findings: in.Findings,
	})
	if err != nil {
		return BCPTest{}, fmt.Errorf("add bcp test: %w", err)
	}
	if err := qtx.RefreshCKBCPPlanLastTested(ctx, db.RefreshCKBCPPlanLastTestedParams{
		ID: planID, OrgID: orgID,
	}); err != nil {
		return BCPTest{}, fmt.Errorf("add bcp test: refresh last_tested_at: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return BCPTest{}, fmt.Errorf("add bcp test: commit: %w", err)
	}
	return bcpTestFromRow(row), nil
}

// ListBCPTests returns all test records for a BCP plan.
func (r *Repository) ListBCPTests(ctx context.Context, orgID, planID string) ([]BCPTest, error) {
	rows, err := r.q.ListCKBCPTests(ctx, db.ListCKBCPTestsParams{PlanID: planID, OrgID: orgID})
	if err != nil {
		return nil, fmt.Errorf("list bcp tests: %w", err)
	}
	out := make([]BCPTest, len(rows))
	for i, row := range rows {
		out[i] = bcpTestFromRow(row)
	}
	return out, nil
}

// GetLatestBCPTest returns the most recent test record for a plan, or nil if none exists.
func (r *Repository) GetLatestBCPTest(ctx context.Context, orgID, planID string) (*BCPTest, error) {
	row, err := r.q.GetLatestCKBCPTest(ctx, db.GetLatestCKBCPTestParams{PlanID: planID, OrgID: orgID})
	if err != nil {
		return nil, fmt.Errorf("get latest bcp test: %w", err)
	}
	t := bcpTestFromRow(row)
	return &t, nil
}

// bcpPlanFromRow maps a db.CkBcpPlans row to the BCPPlan domain model.
func bcpPlanFromRow(row db.CkBcpPlans) BCPPlan {
	var lastTested *string
	if row.LastTestedAt.Valid {
		s := row.LastTestedAt.Time.Format("2006-01-02")
		lastTested = &s
	}
	return BCPPlan{
		ID:                  row.ID,
		OrgID:               row.OrgID,
		Title:               row.Title,
		Scope:               row.Scope,
		Version:             row.Version,
		Status:              row.Status,
		Owner:               row.Owner,
		RTOHours:            int4ToOpt(row.RtoHours),
		RPOHours:            int4ToOpt(row.RpoHours),
		Schutzbedarfsklasse: int4ToOpt(row.Schutzbedarfsklasse),
		LastTestedAt:        lastTested,
		CreatedAt:           bcmTsToTime(row.CreatedAt),
		UpdatedAt:           bcmTsToTime(row.UpdatedAt),
	}
}

// optInt4 traegt "nicht gesetzt" als SQL NULL in die Query. Ohne den Zeiger
// waere 0 der einzige Ausdruck fuer "nicht gesetzt" — und 0 verletzt den
// CHECK schutzbedarfsklasse IN (1,2,3) der Tabelle (Migration 216).
func optInt4(v *int) pgtype.Int4 {
	if v == nil {
		return pgtype.Int4{}
	}
	return pgtype.Int4{Int32: int32(*v), Valid: true}
}

// int4ToOpt bildet SQL NULL auf `null` in der JSON-Antwort ab statt auf 0.
func int4ToOpt(v pgtype.Int4) *int {
	if !v.Valid {
		return nil
	}
	i := int(v.Int32)
	return &i
}

// bcpTestFromRow maps a db.CkBcpTests row to the BCPTest domain model.
func bcpTestFromRow(row db.CkBcpTests) BCPTest {
	var dateStr string
	if row.TestDate.Valid {
		dateStr = row.TestDate.Time.Format("2006-01-02")
	}
	return BCPTest{
		ID:        row.ID,
		OrgID:     row.OrgID,
		PlanID:    row.PlanID,
		TestDate:  dateStr,
		TestType:  row.TestType,
		Outcome:   row.Outcome,
		Findings:  row.Findings,
		CreatedAt: bcmTsToTime(row.CreatedAt),
	}
}

// bcmTsToTime converts pgtype.Timestamptz to time.Time (zero on NULL).
func bcmTsToTime(t pgtype.Timestamptz) time.Time {
	if !t.Valid {
		return time.Time{}
	}
	return t.Time
}
