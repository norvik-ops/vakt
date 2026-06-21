// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package risk

import (
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/matharnica/vakt/internal/db"
)

// Repository provides risk-domain database operations.
type Repository struct {
	db *pgxpool.Pool
	q  *db.Queries
}

// NewRepository creates a new risk-domain repository.
func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{db: pool, q: db.New(pool)}
}

// --- shared pgtype helpers (duplicated from the parent vaktcomply package) ---

// ckTsToTime converts pgtype.Timestamptz to time.Time (zero on NULL).
func ckTsToTime(t pgtype.Timestamptz) time.Time {
	if !t.Valid {
		return time.Time{}
	}
	return t.Time
}

// ckTsToTimePtr converts pgtype.Timestamptz to *time.Time (nil on NULL).
func ckTsToTimePtr(t pgtype.Timestamptz) *time.Time {
	if !t.Valid {
		return nil
	}
	tm := t.Time
	return &tm
}

// ckDateToTimePtr converts pgtype.Date to *time.Time (nil on NULL).
func ckDateToTimePtr(d pgtype.Date) *time.Time {
	if !d.Valid {
		return nil
	}
	tm := d.Time
	return &tm
}

// ckOptText: empty string → invalid pgtype.Text (NULL in DB).
func ckOptText(s string) pgtype.Text {
	if s == "" {
		return pgtype.Text{}
	}
	return pgtype.Text{String: s, Valid: true}
}

// ckOptIntPtr: nil → invalid pgtype.Int4 (NULL in DB).
func ckOptIntPtr(i *int) pgtype.Int4 {
	if i == nil {
		return pgtype.Int4{}
	}
	return pgtype.Int4{Int32: int32(*i), Valid: true}
}

// ckOptTsPtr converts *time.Time to pgtype.Timestamptz; nil → invalid.
func ckOptTsPtr(t *time.Time) pgtype.Timestamptz {
	if t == nil {
		return pgtype.Timestamptz{}
	}
	return pgtype.Timestamptz{Time: *t, Valid: true}
}

// ckOptDatePtr: nil string ptr → invalid; "YYYY-MM-DD" string → pgtype.Date.
func ckOptDatePtr(s *string) pgtype.Date {
	if s == nil || *s == "" {
		return pgtype.Date{}
	}
	t, err := time.Parse("2006-01-02", *s)
	if err != nil {
		return pgtype.Date{}
	}
	return pgtype.Date{Time: t, Valid: true}
}

// optTextStrPtr converts *string to pgtype.Text (nil → invalid, *"" → valid empty).
func optTextStrPtr(s *string) pgtype.Text {
	if s == nil {
		return pgtype.Text{}
	}
	return pgtype.Text{String: *s, Valid: true}
}

func intPtrFromInt4(v pgtype.Int4) *int {
	if !v.Valid {
		return nil
	}
	i := int(v.Int32)
	return &i
}

// riskFields collects all columns shared between every Risk-returning sqlc
// query. ADR-0013: centralise mapping in one helper.
type riskFields struct {
	ID, OrgID, Title, Description, Category  string
	Likelihood, Impact                       int16
	RiskScore                                pgtype.Int2
	Owner, Status, Treatment, TreatmentNotes string
	TreatmentOption                          pgtype.Text
	TreatmentPlan, TreatmentOwner            string
	TreatmentDueDate                         pgtype.Date
	TreatmentStatus                          string
	ResidualLikelihood                       pgtype.Int4
	ResidualImpact                           pgtype.Int4
	// S61-4 residual fields (Migration 164) — nil when populated via sqlc (query not regenerated)
	InherentLikelihood          pgtype.Int4
	InherentImpact              pgtype.Int4
	RiskAcceptedBy              pgtype.UUID
	RiskAcceptedAt              pgtype.Timestamptz
	RiskAcceptanceJustification pgtype.Text
	CreatedAt, UpdatedAt        pgtype.Timestamptz
}

func riskFromFields(f riskFields) Risk {
	r := Risk{
		ID:                 f.ID,
		OrgID:              f.OrgID,
		Title:              f.Title,
		Description:        f.Description,
		Category:           f.Category,
		Likelihood:         int(f.Likelihood),
		Impact:             int(f.Impact),
		RiskScore:          int(f.RiskScore.Int16),
		Owner:              f.Owner,
		Status:             f.Status,
		Treatment:          f.Treatment,
		TreatmentNotes:     f.TreatmentNotes,
		TreatmentOption:    f.TreatmentOption.String,
		TreatmentPlan:      f.TreatmentPlan,
		TreatmentOwner:     f.TreatmentOwner,
		TreatmentDueDate:   ckDateToTimePtr(f.TreatmentDueDate),
		TreatmentStatus:    f.TreatmentStatus,
		ResidualLikelihood: intPtrFromInt4(f.ResidualLikelihood),
		ResidualImpact:     intPtrFromInt4(f.ResidualImpact),
		// S61-4 residual fields
		InherentLikelihood: intPtrFromInt4(f.InherentLikelihood),
		InherentImpact:     intPtrFromInt4(f.InherentImpact),
		CreatedAt:          ckTsToTime(f.CreatedAt),
		UpdatedAt:          ckTsToTime(f.UpdatedAt),
	}
	if f.RiskAcceptedBy.Valid {
		s := f.RiskAcceptedBy.Bytes
		str := fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
			s[0:4], s[4:6], s[6:8], s[8:10], s[10:16])
		r.RiskAcceptedBy = &str
	}
	r.RiskAcceptedAt = ckTsToTimePtr(f.RiskAcceptedAt)
	if f.RiskAcceptanceJustification.Valid {
		r.RiskAcceptanceJustification = f.RiskAcceptanceJustification.String
	}
	r.ComputeScores()
	return r
}
