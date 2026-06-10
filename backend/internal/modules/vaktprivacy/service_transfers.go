// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

// S69-6: Transfer Impact Assessment / TIA (Schrems II, DSGVO Art. 46).

package vaktprivacy

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"
)

// AdequacyDecision represents a row from po_adequacy_decisions.
type AdequacyDecision struct {
	CountryCode       string  `json:"country_code"`
	CountryName       string  `json:"country_name"`
	HasAdequacy       bool    `json:"has_adequacy"`
	DecisionDate      *string `json:"decision_date,omitempty"`
	DecisionReference *string `json:"decision_reference,omitempty"`
	Notes             *string `json:"notes,omitempty"`
	LastUpdated       string  `json:"last_updated"`
}

// DataTransfer represents a row from po_data_transfers.
type DataTransfer struct {
	ID                   string    `json:"id"`
	OrgID                string    `json:"org_id"`
	ProcessingActivityID *string   `json:"processing_activity_id,omitempty"`
	RecipientName        string    `json:"recipient_name"`
	RecipientCountry     string    `json:"recipient_country"`
	RecipientCountryName string    `json:"recipient_country_name"`
	DataCategories       []string  `json:"data_categories"`
	TransferMechanism    string    `json:"transfer_mechanism"`
	SCCVersion           *string   `json:"scc_version,omitempty"`
	Status               string    `json:"status"`
	IsActive             bool      `json:"is_active"`
	CreatedAt            time.Time `json:"created_at"`
	UpdatedAt            time.Time `json:"updated_at"`
}

// TransferImpactAssessment is a row from po_transfer_impact_assessments.
type TransferImpactAssessment struct {
	ID                         string     `json:"id"`
	OrgID                      string     `json:"org_id"`
	TransferID                 string     `json:"transfer_id"`
	LegalSystemNotes           string     `json:"legal_system_notes"`
	SurveillanceRisk           string     `json:"surveillance_risk"`
	DataSubjectRightsAvailable bool       `json:"data_subject_rights_available"`
	EncryptionInTransit        bool       `json:"encryption_in_transit"`
	EncryptionAtRest           bool       `json:"encryption_at_rest"`
	PseudonymizationApplied    bool       `json:"pseudonymization_applied"`
	AccessControlsDocumented   bool       `json:"access_controls_documented"`
	SupplementaryMeasures      *string    `json:"supplementary_measures,omitempty"`
	Outcome                    string     `json:"outcome"`
	ReviewedBy                 *string    `json:"reviewed_by,omitempty"`
	ReviewedAt                 *time.Time `json:"reviewed_at,omitempty"`
	ValidUntil                 *string    `json:"valid_until,omitempty"`
	CreatedAt                  time.Time  `json:"created_at"`
}

// TransferComplianceStatus summarizes TIA compliance for an org.
type TransferComplianceStatus struct {
	TotalTransfers  int `json:"total_transfers"`
	Adequate        int `json:"adequate"`
	RequiresTIA     int `json:"requires_tia"`
	TIAAdequate     int `json:"tia_adequate"`
	TIAWithMeasures int `json:"tia_adequate_with_measures"`
	TIAInadequate   int `json:"tia_inadequate"`
	UnderReview     int `json:"under_review"`
}

// CreateTransferInput is the request body for POST /privacy/transfers.
type CreateTransferInput struct {
	ProcessingActivityID string   `json:"processing_activity_id"`
	RecipientName        string   `json:"recipient_name" validate:"required"`
	RecipientCountry     string   `json:"recipient_country" validate:"required,len=2"`
	DataCategories       []string `json:"data_categories"`
	TransferMechanism    string   `json:"transfer_mechanism" validate:"required,oneof=adequacy_decision scc bcr derogation other"`
	SCCVersion           string   `json:"scc_version"`
}

// CreateTIAInput is the request body for POST /privacy/transfers/:id/tia.
type CreateTIAInput struct {
	LegalSystemNotes           string `json:"legal_system_notes" validate:"required"`
	SurveillanceRisk           string `json:"surveillance_risk" validate:"required,oneof=low medium high"`
	DataSubjectRightsAvailable bool   `json:"data_subject_rights_available"`
	EncryptionInTransit        bool   `json:"encryption_in_transit"`
	EncryptionAtRest           bool   `json:"encryption_at_rest"`
	PseudonymizationApplied    bool   `json:"pseudonymization_applied"`
	AccessControlsDocumented   bool   `json:"access_controls_documented"`
	SupplementaryMeasures      string `json:"supplementary_measures"`
	Outcome                    string `json:"outcome" validate:"required,oneof=adequate adequate_with_measures inadequate"`
	ValidUntil                 string `json:"valid_until"`
}

// TIAService handles Transfer Impact Assessments.
type TIAService struct {
	db *pgxpool.Pool
}

// NewTIAService creates a new TIAService.
func NewTIAService(db *pgxpool.Pool) *TIAService {
	return &TIAService{db: db}
}

// ListAdequacyDecisions returns all adequacy decisions from the global table.
func (s *TIAService) ListAdequacyDecisions(ctx context.Context) ([]AdequacyDecision, error) {
	rows, err := s.db.Query(ctx, `
		SELECT country_code, country_name, has_adequacy,
		       decision_date::text, decision_reference, notes, last_updated::text
		FROM po_adequacy_decisions ORDER BY country_name`,
	)
	if err != nil {
		return nil, fmt.Errorf("list adequacy decisions: %w", err)
	}
	defer rows.Close()

	var out []AdequacyDecision
	for rows.Next() {
		var d AdequacyDecision
		if err := rows.Scan(&d.CountryCode, &d.CountryName, &d.HasAdequacy,
			&d.DecisionDate, &d.DecisionReference, &d.Notes, &d.LastUpdated); err != nil {
			return nil, fmt.Errorf("scan adequacy decision: %w", err)
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

// GetAdequacyDecision returns the adequacy status for a country code.
func (s *TIAService) GetAdequacyDecision(ctx context.Context, countryCode string) (*AdequacyDecision, error) {
	var d AdequacyDecision
	err := s.db.QueryRow(ctx, `
		SELECT country_code, country_name, has_adequacy,
		       decision_date::text, decision_reference, notes, last_updated::text
		FROM po_adequacy_decisions WHERE country_code = $1`,
		countryCode,
	).Scan(&d.CountryCode, &d.CountryName, &d.HasAdequacy,
		&d.DecisionDate, &d.DecisionReference, &d.Notes, &d.LastUpdated)
	if err != nil {
		return nil, nil // not found = no adequacy decision
	}
	return &d, nil
}

// CreateTransfer creates a new data transfer record, auto-assigning status from adequacy table.
func (s *TIAService) CreateTransfer(ctx context.Context, orgID string, in CreateTransferInput) (*DataTransfer, error) {
	// Auto-lookup adequacy decision.
	adq, _ := s.GetAdequacyDecision(ctx, in.RecipientCountry)
	status := "requires_tia"
	if adq != nil && adq.HasAdequacy && in.TransferMechanism == "adequacy_decision" {
		status = "adequate"
	}

	// Lookup country name.
	countryName := in.RecipientCountry
	if adq != nil {
		countryName = adq.CountryName
	}

	var paID *string
	if in.ProcessingActivityID != "" {
		paID = &in.ProcessingActivityID
	}
	var sccVersion *string
	if in.SCCVersion != "" {
		sccVersion = &in.SCCVersion
	}

	var t DataTransfer
	var paIDOut *string
	err := s.db.QueryRow(ctx, `
		INSERT INTO po_data_transfers
			(org_id, processing_activity_id, recipient_name, recipient_country,
			 recipient_country_name, data_categories, transfer_mechanism, scc_version, status)
		VALUES ($1::uuid, $2::uuid, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id::text, org_id::text, processing_activity_id::text,
		          recipient_name, recipient_country, recipient_country_name,
		          data_categories, transfer_mechanism, scc_version, status, is_active,
		          created_at, updated_at`,
		orgID, paID, in.RecipientName, in.RecipientCountry,
		countryName, in.DataCategories, in.TransferMechanism, sccVersion, status,
	).Scan(
		&t.ID, &t.OrgID, &paIDOut,
		&t.RecipientName, &t.RecipientCountry, &t.RecipientCountryName,
		&t.DataCategories, &t.TransferMechanism, &t.SCCVersion, &t.Status, &t.IsActive,
		&t.CreatedAt, &t.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("create transfer: %w", err)
	}
	t.ProcessingActivityID = paIDOut
	return &t, nil
}

// ListTransfers returns all data transfers for an org.
func (s *TIAService) ListTransfers(ctx context.Context, orgID string) ([]DataTransfer, error) {
	rows, err := s.db.Query(ctx, `
		SELECT id::text, org_id::text, processing_activity_id::text,
		       recipient_name, recipient_country, recipient_country_name,
		       data_categories, transfer_mechanism, scc_version, status, is_active,
		       created_at, updated_at
		FROM po_data_transfers
		WHERE org_id = $1::uuid AND is_active = true
		ORDER BY created_at DESC`,
		orgID,
	)
	if err != nil {
		return nil, fmt.Errorf("list transfers: %w", err)
	}
	defer rows.Close()

	var out []DataTransfer
	for rows.Next() {
		var t DataTransfer
		if err := rows.Scan(
			&t.ID, &t.OrgID, &t.ProcessingActivityID,
			&t.RecipientName, &t.RecipientCountry, &t.RecipientCountryName,
			&t.DataCategories, &t.TransferMechanism, &t.SCCVersion, &t.Status, &t.IsActive,
			&t.CreatedAt, &t.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan transfer: %w", err)
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// CreateTIA creates a new Transfer Impact Assessment for a transfer.
func (s *TIAService) CreateTIA(ctx context.Context, orgID, transferID, reviewerID string, in CreateTIAInput) (*TransferImpactAssessment, error) {
	var validUntil *string
	if in.ValidUntil != "" {
		validUntil = &in.ValidUntil
	}
	var supMeasures *string
	if in.SupplementaryMeasures != "" {
		supMeasures = &in.SupplementaryMeasures
	}
	var reviewerIDp *string
	if reviewerID != "" {
		reviewerIDp = &reviewerID
	}

	var tia TransferImpactAssessment
	var reviewedAt *time.Time
	var reviewedByOut, validUntilOut, supOut *string
	err := s.db.QueryRow(ctx, `
		INSERT INTO po_transfer_impact_assessments
			(org_id, transfer_id, legal_system_notes, surveillance_risk,
			 data_subject_rights_available, encryption_in_transit, encryption_at_rest,
			 pseudonymization_applied, access_controls_documented, supplementary_measures,
			 outcome, reviewed_by, reviewed_at, valid_until)
		VALUES ($1::uuid, $2::uuid, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12::uuid, NOW(), $13::date)
		RETURNING id::text, org_id::text, transfer_id::text,
		          legal_system_notes, surveillance_risk,
		          data_subject_rights_available, encryption_in_transit, encryption_at_rest,
		          pseudonymization_applied, access_controls_documented, supplementary_measures,
		          outcome, reviewed_by::text, reviewed_at, valid_until::text, created_at`,
		orgID, transferID, in.LegalSystemNotes, in.SurveillanceRisk,
		in.DataSubjectRightsAvailable, in.EncryptionInTransit, in.EncryptionAtRest,
		in.PseudonymizationApplied, in.AccessControlsDocumented, supMeasures,
		in.Outcome, reviewerIDp, validUntil,
	).Scan(
		&tia.ID, &tia.OrgID, &tia.TransferID,
		&tia.LegalSystemNotes, &tia.SurveillanceRisk,
		&tia.DataSubjectRightsAvailable, &tia.EncryptionInTransit, &tia.EncryptionAtRest,
		&tia.PseudonymizationApplied, &tia.AccessControlsDocumented, &supOut,
		&tia.Outcome, &reviewedByOut, &reviewedAt, &validUntilOut, &tia.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("create tia: %w", err)
	}
	tia.SupplementaryMeasures = supOut
	tia.ReviewedBy = reviewedByOut
	tia.ReviewedAt = reviewedAt
	tia.ValidUntil = validUntilOut

	// Update transfer status based on TIA outcome.
	newStatus := tiaOutcomeToTransferStatus(in.Outcome)
	if _, err := s.db.Exec(ctx, `UPDATE po_data_transfers SET status = $1, updated_at = NOW() WHERE id = $2::uuid AND org_id = $3::uuid`,
		newStatus, transferID, orgID); err != nil {
		log.Warn().Err(err).Str("transfer_id", transferID).Msg("update transfer status after TIA failed")
	}

	return &tia, nil
}

// ListTIAsForTransfer returns all TIAs for a transfer, newest first.
func (s *TIAService) ListTIAsForTransfer(ctx context.Context, orgID, transferID string) ([]TransferImpactAssessment, error) {
	rows, err := s.db.Query(ctx, `
		SELECT id::text, org_id::text, transfer_id::text,
		       legal_system_notes, surveillance_risk,
		       data_subject_rights_available, encryption_in_transit, encryption_at_rest,
		       pseudonymization_applied, access_controls_documented, supplementary_measures,
		       outcome, reviewed_by::text, reviewed_at, valid_until::text, created_at
		FROM po_transfer_impact_assessments
		WHERE org_id = $1::uuid AND transfer_id = $2::uuid
		ORDER BY created_at DESC`,
		orgID, transferID,
	)
	if err != nil {
		return nil, fmt.Errorf("list tias: %w", err)
	}
	defer rows.Close()

	var out []TransferImpactAssessment
	for rows.Next() {
		var t TransferImpactAssessment
		var supOut, reviewedBy, validUntil *string
		var reviewedAt *time.Time
		if err := rows.Scan(
			&t.ID, &t.OrgID, &t.TransferID,
			&t.LegalSystemNotes, &t.SurveillanceRisk,
			&t.DataSubjectRightsAvailable, &t.EncryptionInTransit, &t.EncryptionAtRest,
			&t.PseudonymizationApplied, &t.AccessControlsDocumented, &supOut,
			&t.Outcome, &reviewedBy, &reviewedAt, &validUntil, &t.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan tia: %w", err)
		}
		t.SupplementaryMeasures = supOut
		t.ReviewedBy = reviewedBy
		t.ReviewedAt = reviewedAt
		t.ValidUntil = validUntil
		out = append(out, t)
	}
	return out, rows.Err()
}

// tiaOutcomeToTransferStatus converts a TIA outcome to the corresponding transfer status.
func tiaOutcomeToTransferStatus(outcome string) string {
	switch outcome {
	case "adequate_with_measures":
		return "tia_adequate_measures"
	case "inadequate":
		return "tia_inadequate"
	default:
		return "tia_adequate"
	}
}

// GetTransferComplianceStatus returns aggregate transfer compliance for an org.
func (s *TIAService) GetTransferComplianceStatus(ctx context.Context, orgID string) (*TransferComplianceStatus, error) {
	rows, err := s.db.Query(ctx, `
		SELECT status, COUNT(*) FROM po_data_transfers
		WHERE org_id = $1::uuid AND is_active = true
		GROUP BY status`,
		orgID,
	)
	if err != nil {
		return nil, fmt.Errorf("get transfer compliance: %w", err)
	}
	defer rows.Close()

	status := &TransferComplianceStatus{}
	for rows.Next() {
		var stat string
		var cnt int
		if err := rows.Scan(&stat, &cnt); err != nil {
			continue
		}
		status.TotalTransfers += cnt
		switch stat {
		case "adequate":
			status.Adequate += cnt
		case "requires_tia":
			status.RequiresTIA += cnt
		case "tia_adequate":
			status.TIAAdequate += cnt
		case "tia_adequate_measures":
			status.TIAWithMeasures += cnt
		case "tia_inadequate":
			status.TIAInadequate += cnt
		case "under_review":
			status.UnderReview += cnt
		}
	}
	return status, rows.Err()
}
