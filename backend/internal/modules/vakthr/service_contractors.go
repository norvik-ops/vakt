// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

// S70-4: Contractor/Freelancer lifecycle management.

package vakthr

import (
	"context"
	"fmt"
	"time"

	"github.com/rs/zerolog/log"
)

// TaskContractorExpiryCheck is the Asynq task name for the daily contractor expiry check.
const TaskContractorExpiryCheck = "hr:contractor_expiry_check"

// Contractor represents an external contractor or freelancer.
type Contractor struct {
	ID                    string     `json:"id"`
	OrgID                 string     `json:"org_id"`
	FirstName             string     `json:"first_name"`
	LastName              string     `json:"last_name"`
	Email                 string     `json:"email,omitempty"`
	Company               string     `json:"company,omitempty"`
	ContractStart         string     `json:"contract_start"` // YYYY-MM-DD
	ContractEnd           string     `json:"contract_end"`   // YYYY-MM-DD
	AccessScope           []string   `json:"access_scope"`
	NDASigned             bool       `json:"nda_signed"`
	AVVSigned             bool       `json:"avv_signed"`
	Status                string     `json:"status"` // active | expiring_soon | offboarding | terminated
	ChecklistRunID        *string    `json:"checklist_run_id,omitempty"`
	OffboardingCompletedAt *time.Time `json:"offboarding_completed_at,omitempty"`
	CreatedAt             time.Time  `json:"created_at"`
	UpdatedAt             time.Time  `json:"updated_at"`
}

// CreateContractorInput is the request body for POST /vakthr/contractors.
type CreateContractorInput struct {
	FirstName     string   `json:"first_name"  validate:"required"`
	LastName      string   `json:"last_name"   validate:"required"`
	Email         string   `json:"email"       validate:"omitempty,email"`
	Company       string   `json:"company"`
	ContractStart string   `json:"contract_start" validate:"required"`
	ContractEnd   string   `json:"contract_end"   validate:"required"`
	AccessScope   []string `json:"access_scope"`
	NDASigned     bool     `json:"nda_signed"`
	AVVSigned     bool     `json:"avv_signed"`
}

// UpdateContractorInput is the request body for PUT /vakthr/contractors/:id.
type UpdateContractorInput struct {
	FirstName   string   `json:"first_name"  validate:"required"`
	LastName    string   `json:"last_name"   validate:"required"`
	Email       string   `json:"email"       validate:"omitempty,email"`
	Company     string   `json:"company"`
	ContractEnd string   `json:"contract_end"`
	AccessScope []string `json:"access_scope"`
	NDASigned   bool     `json:"nda_signed"`
	AVVSigned   bool     `json:"avv_signed"`
	Status      string   `json:"status" validate:"omitempty,oneof=active expiring_soon offboarding terminated"`
}

// ListContractors returns all contractors for the organisation.
func (s *Service) ListContractors(ctx context.Context, orgID string) ([]Contractor, error) {
	rows, err := s.db.Query(ctx, `
		SELECT id, org_id, first_name, last_name, email, company,
			contract_start::TEXT, contract_end::TEXT,
			access_scope, nda_signed, avv_signed, status,
			checklist_run_id, offboarding_completed_at, created_at, updated_at
		FROM hr_contractors
		WHERE org_id = $1
		ORDER BY contract_end ASC, created_at DESC`,
		orgID,
	)
	if err != nil {
		return nil, fmt.Errorf("list contractors: %w", err)
	}
	defer rows.Close()
	var out []Contractor
	for rows.Next() {
		var c Contractor
		if err := rows.Scan(
			&c.ID, &c.OrgID, &c.FirstName, &c.LastName, &c.Email, &c.Company,
			&c.ContractStart, &c.ContractEnd,
			&c.AccessScope, &c.NDASigned, &c.AVVSigned, &c.Status,
			&c.ChecklistRunID, &c.OffboardingCompletedAt, &c.CreatedAt, &c.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan contractor: %w", err)
		}
		out = append(out, c)
	}
	return out, nil
}

// GetContractor returns a single contractor.
func (s *Service) GetContractor(ctx context.Context, orgID, id string) (*Contractor, error) {
	row := s.db.QueryRow(ctx, `
		SELECT id, org_id, first_name, last_name, email, company,
			contract_start::TEXT, contract_end::TEXT,
			access_scope, nda_signed, avv_signed, status,
			checklist_run_id, offboarding_completed_at, created_at, updated_at
		FROM hr_contractors
		WHERE org_id = $1 AND id = $2`,
		orgID, id,
	)
	var c Contractor
	if err := row.Scan(
		&c.ID, &c.OrgID, &c.FirstName, &c.LastName, &c.Email, &c.Company,
		&c.ContractStart, &c.ContractEnd,
		&c.AccessScope, &c.NDASigned, &c.AVVSigned, &c.Status,
		&c.ChecklistRunID, &c.OffboardingCompletedAt, &c.CreatedAt, &c.UpdatedAt,
	); err != nil {
		return nil, fmt.Errorf("contractor not found: %w", err)
	}
	return &c, nil
}

// CreateContractor creates a new contractor record.
func (s *Service) CreateContractor(ctx context.Context, orgID string, in CreateContractorInput) (*Contractor, error) {
	scope := in.AccessScope
	if scope == nil {
		scope = []string{}
	}
	row := s.db.QueryRow(ctx, `
		INSERT INTO hr_contractors (org_id, first_name, last_name, email, company, contract_start, contract_end, access_scope, nda_signed, avv_signed)
		VALUES ($1,$2,$3,$4,$5,$6::DATE,$7::DATE,$8,$9,$10)
		RETURNING id, org_id, first_name, last_name, email, company,
			contract_start::TEXT, contract_end::TEXT,
			access_scope, nda_signed, avv_signed, status,
			checklist_run_id, offboarding_completed_at, created_at, updated_at`,
		orgID, in.FirstName, in.LastName, in.Email, in.Company,
		in.ContractStart, in.ContractEnd, scope, in.NDASigned, in.AVVSigned,
	)
	var c Contractor
	if err := row.Scan(
		&c.ID, &c.OrgID, &c.FirstName, &c.LastName, &c.Email, &c.Company,
		&c.ContractStart, &c.ContractEnd,
		&c.AccessScope, &c.NDASigned, &c.AVVSigned, &c.Status,
		&c.ChecklistRunID, &c.OffboardingCompletedAt, &c.CreatedAt, &c.UpdatedAt,
	); err != nil {
		return nil, fmt.Errorf("create contractor: %w", err)
	}
	return &c, nil
}

// UpdateContractor updates a contractor record.
func (s *Service) UpdateContractor(ctx context.Context, orgID, id string, in UpdateContractorInput) (*Contractor, error) {
	scope := in.AccessScope
	if scope == nil {
		scope = []string{}
	}
	status := in.Status
	if status == "" {
		status = "active"
	}
	row := s.db.QueryRow(ctx, `
		UPDATE hr_contractors SET
			first_name   = $3,
			last_name    = $4,
			email        = $5,
			company      = $6,
			contract_end = $7::DATE,
			access_scope = $8,
			nda_signed   = $9,
			avv_signed   = $10,
			status       = $11,
			updated_at   = NOW()
		WHERE org_id = $1 AND id = $2
		RETURNING id, org_id, first_name, last_name, email, company,
			contract_start::TEXT, contract_end::TEXT,
			access_scope, nda_signed, avv_signed, status,
			checklist_run_id, offboarding_completed_at, created_at, updated_at`,
		orgID, id,
		in.FirstName, in.LastName, in.Email, in.Company,
		in.ContractEnd, scope, in.NDASigned, in.AVVSigned, status,
	)
	var c Contractor
	if err := row.Scan(
		&c.ID, &c.OrgID, &c.FirstName, &c.LastName, &c.Email, &c.Company,
		&c.ContractStart, &c.ContractEnd,
		&c.AccessScope, &c.NDASigned, &c.AVVSigned, &c.Status,
		&c.ChecklistRunID, &c.OffboardingCompletedAt, &c.CreatedAt, &c.UpdatedAt,
	); err != nil {
		return nil, fmt.Errorf("update contractor: %w", err)
	}
	return &c, nil
}

// CheckContractorExpiry updates statuses for contractors nearing or past contract end date.
// Called daily by Asynq task hr:contractor_expiry_check.
func (s *Service) CheckContractorExpiry(ctx context.Context) error {
	// Mark as expiring_soon (≤14 days)
	if _, err := s.db.Exec(ctx, `
		UPDATE hr_contractors SET status = 'expiring_soon', updated_at = NOW()
		WHERE status = 'active'
		  AND contract_end <= CURRENT_DATE + INTERVAL '14 days'
		  AND contract_end > CURRENT_DATE`,
	); err != nil {
		log.Error().Err(err).Msg("hr: mark expiring_soon contractors")
	}
	// Mark as offboarding (past end date, not yet terminated)
	rows, err := s.db.Query(ctx, `
		UPDATE hr_contractors SET status = 'offboarding', updated_at = NOW()
		WHERE status IN ('active', 'expiring_soon')
		  AND contract_end < CURRENT_DATE
		RETURNING id, org_id, first_name, last_name`,
	)
	if err != nil {
		return fmt.Errorf("hr: mark offboarding contractors: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var id, orgID, first, last string
		if err := rows.Scan(&id, &orgID, &first, &last); err != nil {
			continue
		}
		log.Info().Str("contractor_id", id).Str("org_id", orgID).
			Msgf("contractor %s %s — contract expired, set to offboarding", first, last)
		if s.evidence != nil {
			_ = s.evidence.WriteEvidence(ctx, orgID, "contractor_offboarding",
				fmt.Sprintf("Contractor %s %s (ID: %s) — contract expired, offboarding started", first, last, id), id)
		}
	}
	return nil
}
