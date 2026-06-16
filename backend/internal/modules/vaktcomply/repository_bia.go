// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package vaktcomply

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5/pgtype"

	db "github.com/matharnica/vakt/internal/db"
)

func (r *Repository) CreateBIAProcess(ctx context.Context, orgID string, in CreateBIAProcessInput) (BIAProcess, error) {
	if in.Dependencies == nil {
		in.Dependencies = []string{}
	}
	row, err := r.q.CreateCKBIAProcess(ctx, db.CreateCKBIAProcessParams{
		OrgID:               orgID,
		Name:                in.Name,
		Description:         in.Description,
		ProcessOwner:        in.ProcessOwner,
		Criticality:         in.Criticality,
		Schutzbedarfsklasse: int32(in.Schutzbedarfsklasse),
		RtoHours:            int32(in.RTOHours),
		RpoHours:            int32(in.RPOHours),
		MbcoPercent:         int32(in.MBCOPercent),
		Dependencies:        in.Dependencies,
	})
	if err != nil {
		return BIAProcess{}, fmt.Errorf("create bia process: %w", err)
	}
	return biaProcessFromRow(row), nil
}

func (r *Repository) ListBIAProcesses(ctx context.Context, orgID string) ([]BIAProcess, error) {
	rows, err := r.q.ListCKBIAProcesses(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("list bia processes: %w", err)
	}
	out := make([]BIAProcess, len(rows))
	for i, row := range rows {
		out[i] = biaProcessFromRow(row)
	}
	return out, nil
}

func (r *Repository) GetBIAProcess(ctx context.Context, orgID, id string) (BIAProcess, error) {
	row, err := r.q.GetCKBIAProcess(ctx, db.GetCKBIAProcessParams{ID: id, OrgID: orgID})
	if err != nil {
		return BIAProcess{}, fmt.Errorf("get bia process: %w", err)
	}
	return biaProcessFromRow(row), nil
}

func (r *Repository) UpdateBIAProcess(ctx context.Context, orgID, id string, in UpdateBIAProcessInput) (BIAProcess, error) {
	if in.Dependencies == nil {
		in.Dependencies = []string{}
	}
	row, err := r.q.UpdateCKBIAProcess(ctx, db.UpdateCKBIAProcessParams{
		ID:                  id,
		OrgID:               orgID,
		Name:                in.Name,
		Description:         in.Description,
		ProcessOwner:        in.ProcessOwner,
		Criticality:         in.Criticality,
		Schutzbedarfsklasse: int32(in.Schutzbedarfsklasse),
		RtoHours:            int32(in.RTOHours),
		RpoHours:            int32(in.RPOHours),
		MbcoPercent:         int32(in.MBCOPercent),
		Dependencies:        in.Dependencies,
	})
	if err != nil {
		return BIAProcess{}, fmt.Errorf("update bia process: %w", err)
	}
	return biaProcessFromRow(row), nil
}

func (r *Repository) DeleteBIAProcess(ctx context.Context, orgID, id string) error {
	n, err := r.q.DeleteCKBIAProcess(ctx, db.DeleteCKBIAProcessParams{ID: id, OrgID: orgID})
	if err != nil {
		return fmt.Errorf("delete bia process: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("bia process not found")
	}
	return nil
}

func (r *Repository) GetBIASummary(ctx context.Context, orgID string) (BIASummary, error) {
	row, err := r.q.GetCKBIASummary(ctx, orgID)
	if err != nil {
		return BIASummary{}, fmt.Errorf("get bia summary: %w", err)
	}
	return BIASummary{
		TotalProcesses:   int(row.TotalProcesses),
		CriticalCount:    int(row.CriticalCount),
		ShortestRTOHours: int(row.ShortestRtoHours),
		KlasseBreakdown: map[int]int{
			1: int(row.Klasse1Count),
			2: int(row.Klasse2Count),
			3: int(row.Klasse3Count),
		},
	}, nil
}

// ── Recovery Plans ────────────────────────────────────────────────────────────

func (r *Repository) CreateRecoveryPlan(ctx context.Context, orgID string, in CreateRecoveryPlanInput) (RecoveryPlan, error) {
	stepsJSON, err := json.Marshal(in.Steps)
	if err != nil {
		return RecoveryPlan{}, fmt.Errorf("marshal steps: %w", err)
	}
	biaID := pgtype.UUID{}
	if in.BIAProcessID != nil && *in.BIAProcessID != "" {
		if err := biaID.Scan(*in.BIAProcessID); err != nil {
			return RecoveryPlan{}, fmt.Errorf("parse bia_process_id: %w", err)
		}
	}
	row, err := r.q.CreateCKRecoveryPlan(ctx, db.CreateCKRecoveryPlanParams{
		OrgID:              orgID,
		BiaProcessID:       biaID,
		Title:              in.Title,
		ActivationCriteria: in.ActivationCriteria,
		Responsible:        in.Responsible,
		RtoHours:           int32(in.RTOHours),
		Status:             in.Status,
		Steps:              stepsJSON,
	})
	if err != nil {
		return RecoveryPlan{}, fmt.Errorf("create recovery plan: %w", err)
	}
	return recoveryPlanFromRow(db.ListCKRecoveryPlansRow{CkRecoveryPlans: row}), nil
}

func (r *Repository) ListRecoveryPlans(ctx context.Context, orgID string) ([]RecoveryPlan, error) {
	rows, err := r.q.ListCKRecoveryPlans(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("list recovery plans: %w", err)
	}
	out := make([]RecoveryPlan, len(rows))
	for i, row := range rows {
		out[i] = recoveryPlanFromRow(row)
	}
	return out, nil
}

func (r *Repository) ListRecoveryPlansByBIAProcess(ctx context.Context, orgID, biaProcessID string) ([]RecoveryPlan, error) {
	var biaID pgtype.UUID
	if err := biaID.Scan(biaProcessID); err != nil {
		return nil, fmt.Errorf("parse bia_process_id: %w", err)
	}
	rows, err := r.q.ListCKRecoveryPlansByBIAProcess(ctx, db.ListCKRecoveryPlansByBIAProcessParams{
		OrgID: orgID, BiaProcessID: biaID,
	})
	if err != nil {
		return nil, fmt.Errorf("list recovery plans by bia: %w", err)
	}
	out := make([]RecoveryPlan, len(rows))
	for i, row := range rows {
		out[i] = recoveryPlanFromRow(row)
	}
	return out, nil
}

func (r *Repository) GetRecoveryPlan(ctx context.Context, orgID, id string) (RecoveryPlan, error) {
	row, err := r.q.GetCKRecoveryPlan(ctx, db.GetCKRecoveryPlanParams{ID: id, OrgID: orgID})
	if err != nil {
		return RecoveryPlan{}, fmt.Errorf("get recovery plan: %w", err)
	}
	return recoveryPlanFromRow(row), nil
}

func (r *Repository) UpdateRecoveryPlan(ctx context.Context, orgID, id string, in UpdateRecoveryPlanInput) (RecoveryPlan, error) {
	stepsJSON, err := json.Marshal(in.Steps)
	if err != nil {
		return RecoveryPlan{}, fmt.Errorf("marshal steps: %w", err)
	}
	biaID := pgtype.UUID{}
	if in.BIAProcessID != nil && *in.BIAProcessID != "" {
		if err := biaID.Scan(*in.BIAProcessID); err != nil {
			return RecoveryPlan{}, fmt.Errorf("parse bia_process_id: %w", err)
		}
	}
	var lastTested pgtype.Date
	if in.LastTestedAt != nil && *in.LastTestedAt != "" {
		if err := lastTested.Scan(*in.LastTestedAt); err != nil {
			return RecoveryPlan{}, fmt.Errorf("parse last_tested_at: %w", err)
		}
	}
	row, err := r.q.UpdateCKRecoveryPlan(ctx, db.UpdateCKRecoveryPlanParams{
		ID: id, OrgID: orgID,
		BiaProcessID: biaID, Title: in.Title,
		ActivationCriteria: in.ActivationCriteria, Responsible: in.Responsible,
		RtoHours: int32(in.RTOHours), Status: in.Status,
		Steps: stepsJSON, LastTestedAt: lastTested,
	})
	if err != nil {
		return RecoveryPlan{}, fmt.Errorf("update recovery plan: %w", err)
	}
	return recoveryPlanFromRow(db.ListCKRecoveryPlansRow{CkRecoveryPlans: row}), nil
}

func (r *Repository) DeleteRecoveryPlan(ctx context.Context, orgID, id string) error {
	n, err := r.q.DeleteCKRecoveryPlan(ctx, db.DeleteCKRecoveryPlanParams{ID: id, OrgID: orgID})
	if err != nil {
		return fmt.Errorf("delete recovery plan: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("recovery plan not found")
	}
	return nil
}

// ── Emergency Contacts ────────────────────────────────────────────────────────

func (r *Repository) CreateEmergencyContact(ctx context.Context, orgID string, in CreateEmergencyContactInput) (EmergencyContact, error) {
	row, err := r.q.CreateCKEmergencyContact(ctx, db.CreateCKEmergencyContactParams{
		OrgID: orgID, Name: in.Name, Role: in.Role, Phone: in.Phone, Email: in.Email,
		EscalationLevel: int32(in.EscalationLevel), Available247: in.Available247, Notes: in.Notes,
	})
	if err != nil {
		return EmergencyContact{}, fmt.Errorf("create emergency contact: %w", err)
	}
	return emergencyContactFromRow(row), nil
}

func (r *Repository) ListEmergencyContacts(ctx context.Context, orgID string) ([]EmergencyContact, error) {
	rows, err := r.q.ListCKEmergencyContacts(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("list emergency contacts: %w", err)
	}
	out := make([]EmergencyContact, len(rows))
	for i, row := range rows {
		out[i] = emergencyContactFromRow(row)
	}
	return out, nil
}

func (r *Repository) UpdateEmergencyContact(ctx context.Context, orgID, id string, in UpdateEmergencyContactInput) (EmergencyContact, error) {
	row, err := r.q.UpdateCKEmergencyContact(ctx, db.UpdateCKEmergencyContactParams{
		ID: id, OrgID: orgID, Name: in.Name, Role: in.Role, Phone: in.Phone, Email: in.Email,
		EscalationLevel: int32(in.EscalationLevel), Available247: in.Available247, Notes: in.Notes,
	})
	if err != nil {
		return EmergencyContact{}, fmt.Errorf("update emergency contact: %w", err)
	}
	return emergencyContactFromRow(row), nil
}

func (r *Repository) DeleteEmergencyContact(ctx context.Context, orgID, id string) error {
	n, err := r.q.DeleteCKEmergencyContact(ctx, db.DeleteCKEmergencyContactParams{ID: id, OrgID: orgID})
	if err != nil {
		return fmt.Errorf("delete emergency contact: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("emergency contact not found")
	}
	return nil
}

// ── BCM Readiness Score helpers ───────────────────────────────────────────────

func (r *Repository) CountRecoveryPlansTested(ctx context.Context, orgID string) (int, error) {
	n, err := r.q.CountCKRecoveryPlansTested(ctx, orgID)
	return int(n), err
}

func (r *Repository) CountRecoveryPlansActive(ctx context.Context, orgID string) (int, error) {
	n, err := r.q.CountCKRecoveryPlansActive(ctx, orgID)
	return int(n), err
}

func (r *Repository) CountRecoveryPlansForHighCriticality(ctx context.Context, orgID string) (int, error) {
	n, err := r.q.CountCKRecoveryPlansForHighCriticality(ctx, orgID)
	return int(n), err
}

func (r *Repository) CountHighCriticalityBIAProcesses(ctx context.Context, orgID string) (int, error) {
	n, err := r.q.CountCKHighCriticalityBIAProcesses(ctx, orgID)
	return int(n), err
}

func (r *Repository) CountEmergencyContacts(ctx context.Context, orgID string) (int, error) {
	n, err := r.q.CountCKEmergencyContacts(ctx, orgID)
	return int(n), err
}

// ── Row converters ────────────────────────────────────────────────────────────

func biaProcessFromRow(row db.CkBiaProcesses) BIAProcess {
	deps := row.Dependencies
	if deps == nil {
		deps = []string{}
	}
	return BIAProcess{
		ID:                  row.ID,
		OrgID:               row.OrgID,
		Name:                row.Name,
		Description:         row.Description,
		ProcessOwner:        row.ProcessOwner,
		Criticality:         row.Criticality,
		Schutzbedarfsklasse: int(row.Schutzbedarfsklasse),
		RTOHours:            int(row.RtoHours),
		RPOHours:            int(row.RpoHours),
		MBCOPercent:         int(row.MbcoPercent),
		Dependencies:        deps,
		CreatedAt:           row.CreatedAt.Time,
		UpdatedAt:           row.UpdatedAt.Time,
	}
}

func recoveryPlanFromRow(row db.ListCKRecoveryPlansRow) RecoveryPlan {
	var biaID *string
	if row.BiaProcessID.Valid {
		s := row.BiaProcessID.Bytes
		id := fmt.Sprintf("%x-%x-%x-%x-%x", s[0:4], s[4:6], s[6:8], s[8:10], s[10:16])
		biaID = &id
	}
	var steps []RecoveryStep
	if len(row.Steps) > 0 {
		_ = json.Unmarshal(row.Steps, &steps)
	}
	if steps == nil {
		steps = []RecoveryStep{}
	}
	var lastTested *string
	if row.LastTestedAt.Valid {
		s := row.LastTestedAt.Time.Format("2006-01-02")
		lastTested = &s
	}
	return RecoveryPlan{
		ID:                 row.ID,
		OrgID:              row.OrgID,
		BIAProcessID:       biaID,
		BIAProcessName:     row.BiaProcessName,
		Title:              row.Title,
		ActivationCriteria: row.ActivationCriteria,
		Responsible:        row.Responsible,
		RTOHours:           int(row.RtoHours),
		Status:             row.Status,
		Steps:              steps,
		LastTestedAt:       lastTested,
		CreatedAt:          row.CreatedAt.Time,
		UpdatedAt:          row.UpdatedAt.Time,
	}
}

func emergencyContactFromRow(row db.CkEmergencyContacts) EmergencyContact {
	return EmergencyContact{
		ID:              row.ID,
		OrgID:           row.OrgID,
		Name:            row.Name,
		Role:            row.Role,
		Phone:           row.Phone,
		Email:           row.Email,
		EscalationLevel: int(row.EscalationLevel),
		Available247:    row.Available247,
		Notes:           row.Notes,
		CreatedAt:       row.CreatedAt.Time,
		UpdatedAt:       row.UpdatedAt.Time,
	}
}
