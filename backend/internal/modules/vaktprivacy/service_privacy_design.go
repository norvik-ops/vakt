// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

// S70-3: DSGVO Art. 25 Privacy by Design & Default assessments.

package vaktprivacy

import (
	"context"
	"fmt"
	"time"
)

// PrivacyDesignAssessment is one Art-25-Bewertung for a processing activity.
type PrivacyDesignAssessment struct {
	ID                   string `json:"id"`
	OrgID                string `json:"org_id"`
	ProcessingActivityID string `json:"processing_activity_id"`
	// Art. 25 Abs. 1 — by Design
	DesignMeasures     string `json:"design_measures"`
	DesignAtConception bool   `json:"design_at_conception"`
	RiskConsidered     bool   `json:"risk_considered"`
	// Art. 25 Abs. 2 — by Default
	DataMinimization    bool   `json:"data_minimization"`
	PurposeLimitation   bool   `json:"purpose_limitation"`
	StorageLimitation   bool   `json:"storage_limitation"`
	AccessLimitation    bool   `json:"access_limitation"`
	DefaultSettingsNote string `json:"default_settings_note,omitempty"`
	// Gesamtbewertung
	AssessmentResult string     `json:"assessment_result"` // compliant | partially | not_assessed
	ReviewedBy       *string    `json:"reviewed_by,omitempty"`
	ReviewedAt       *time.Time `json:"reviewed_at,omitempty"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
}

// PrivacyDesignInput is the request body for creating or updating a Privacy by Design assessment.
type PrivacyDesignInput struct {
	DesignMeasures      string `json:"design_measures"`
	DesignAtConception  bool   `json:"design_at_conception"`
	RiskConsidered      bool   `json:"risk_considered"`
	DataMinimization    bool   `json:"data_minimization"`
	PurposeLimitation   bool   `json:"purpose_limitation"`
	StorageLimitation   bool   `json:"storage_limitation"`
	AccessLimitation    bool   `json:"access_limitation"`
	DefaultSettingsNote string `json:"default_settings_note"`
	AssessmentResult    string `json:"assessment_result" validate:"omitempty,oneof=compliant partially not_assessed"`
	ReviewedBy          string `json:"reviewed_by"`
}

// PrivacyDesignSummary aggregates Art-25-coverage across all processing activities.
type PrivacyDesignSummary struct {
	TotalActivities int     `json:"total_activities"`
	WithAssessment  int     `json:"with_assessment"`
	Compliant       int     `json:"compliant"`
	Partially       int     `json:"partially"`
	NotAssessed     int     `json:"not_assessed"`  // activities with assessment but result=not_assessed
	PendingCount    int     `json:"pending_count"` // activities with NO assessment at all
	PctCompliant    float64 `json:"pct_compliant"`
}

// CreateOrUpdatePrivacyDesign upserts an Art-25 assessment for a processing activity.
func (s *Service) CreateOrUpdatePrivacyDesign(ctx context.Context, orgID, activityID string, input PrivacyDesignInput) (*PrivacyDesignAssessment, error) {
	if orgID == "" || activityID == "" {
		return nil, fmt.Errorf("org_id and processing_activity_id are required")
	}

	var reviewedBy *string
	var reviewedAt *time.Time
	if input.ReviewedBy != "" {
		reviewedBy = &input.ReviewedBy
		now := time.Now().UTC()
		reviewedAt = &now
	}
	result := input.AssessmentResult
	if result == "" {
		result = "not_assessed"
	}

	row := s.db.QueryRow(ctx, `
		INSERT INTO po_privacy_design_assessments (
			org_id, processing_activity_id,
			design_measures, design_at_conception, risk_considered,
			data_minimization, purpose_limitation, storage_limitation, access_limitation,
			default_settings_note, assessment_result, reviewed_by, reviewed_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)
		ON CONFLICT (org_id, processing_activity_id) DO UPDATE SET
			design_measures      = EXCLUDED.design_measures,
			design_at_conception = EXCLUDED.design_at_conception,
			risk_considered      = EXCLUDED.risk_considered,
			data_minimization    = EXCLUDED.data_minimization,
			purpose_limitation   = EXCLUDED.purpose_limitation,
			storage_limitation   = EXCLUDED.storage_limitation,
			access_limitation    = EXCLUDED.access_limitation,
			default_settings_note= EXCLUDED.default_settings_note,
			assessment_result    = EXCLUDED.assessment_result,
			reviewed_by          = EXCLUDED.reviewed_by,
			reviewed_at          = EXCLUDED.reviewed_at,
			updated_at           = NOW()
		RETURNING id, org_id, processing_activity_id,
			design_measures, design_at_conception, risk_considered,
			data_minimization, purpose_limitation, storage_limitation, access_limitation,
			default_settings_note, assessment_result, reviewed_by, reviewed_at,
			created_at, updated_at`,
		orgID, activityID,
		input.DesignMeasures, input.DesignAtConception, input.RiskConsidered,
		input.DataMinimization, input.PurposeLimitation, input.StorageLimitation, input.AccessLimitation,
		input.DefaultSettingsNote, result, reviewedBy, reviewedAt,
	)

	var a PrivacyDesignAssessment
	if err := row.Scan(
		&a.ID, &a.OrgID, &a.ProcessingActivityID,
		&a.DesignMeasures, &a.DesignAtConception, &a.RiskConsidered,
		&a.DataMinimization, &a.PurposeLimitation, &a.StorageLimitation, &a.AccessLimitation,
		&a.DefaultSettingsNote, &a.AssessmentResult, &a.ReviewedBy, &a.ReviewedAt,
		&a.CreatedAt, &a.UpdatedAt,
	); err != nil {
		return nil, fmt.Errorf("upsert privacy design assessment: %w", err)
	}
	return &a, nil
}

// GetPrivacyDesign returns the Art-25 assessment for a processing activity, or nil if none exists.
func (s *Service) GetPrivacyDesign(ctx context.Context, orgID, activityID string) (*PrivacyDesignAssessment, error) {
	row := s.db.QueryRow(ctx, `
		SELECT id, org_id, processing_activity_id,
			design_measures, design_at_conception, risk_considered,
			data_minimization, purpose_limitation, storage_limitation, access_limitation,
			default_settings_note, assessment_result, reviewed_by, reviewed_at,
			created_at, updated_at
		FROM po_privacy_design_assessments
		WHERE org_id = $1 AND processing_activity_id = $2`,
		orgID, activityID,
	)
	var a PrivacyDesignAssessment
	if err := row.Scan(
		&a.ID, &a.OrgID, &a.ProcessingActivityID,
		&a.DesignMeasures, &a.DesignAtConception, &a.RiskConsidered,
		&a.DataMinimization, &a.PurposeLimitation, &a.StorageLimitation, &a.AccessLimitation,
		&a.DefaultSettingsNote, &a.AssessmentResult, &a.ReviewedBy, &a.ReviewedAt,
		&a.CreatedAt, &a.UpdatedAt,
	); err != nil {
		return nil, nil
	}
	return &a, nil
}

// GetPrivacyDesignSummary returns aggregate Art-25-coverage stats.
func (s *Service) GetPrivacyDesignSummary(ctx context.Context, orgID string) (*PrivacyDesignSummary, error) {
	// Count total activities
	var total int
	if err := s.db.QueryRow(ctx,
		`SELECT COUNT(*) FROM po_processing_activities WHERE org_id = $1`, orgID,
	).Scan(&total); err != nil {
		return nil, fmt.Errorf("count activities: %w", err)
	}

	// Count assessments by result
	rows, err := s.db.Query(ctx,
		`SELECT assessment_result, COUNT(*) FROM po_privacy_design_assessments WHERE org_id = $1 GROUP BY assessment_result`,
		orgID,
	)
	if err != nil {
		return nil, fmt.Errorf("count assessments: %w", err)
	}
	defer rows.Close()

	s70 := &PrivacyDesignSummary{TotalActivities: total}
	for rows.Next() {
		var result string
		var cnt int
		if err := rows.Scan(&result, &cnt); err != nil {
			continue
		}
		s70.WithAssessment += cnt
		switch result {
		case "compliant":
			s70.Compliant += cnt
		case "partially":
			s70.Partially += cnt
		case "not_assessed":
			s70.NotAssessed += cnt
		}
	}
	s70.PendingCount = total - s70.WithAssessment
	if total > 0 {
		s70.PctCompliant = float64(s70.Compliant) / float64(total) * 100
	}
	return s70, nil
}
