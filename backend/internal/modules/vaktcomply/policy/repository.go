// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package policy

import (
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/matharnica/vakt/internal/db"
)

// Repository provides policy-domain database operations (controls, policies,
// frameworks, SoA, framework mappings). ADR-0066 sub-package strategy.
type Repository struct {
	db *pgxpool.Pool
	q  *db.Queries
}

// NewRepository creates a new policy-domain repository.
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

// ckOptText: empty string -> invalid pgtype.Text (NULL in DB).
func ckOptText(s string) pgtype.Text {
	if s == "" {
		return pgtype.Text{}
	}
	return pgtype.Text{String: s, Valid: true}
}

// ckOptIntPtr: nil -> invalid pgtype.Int4 (NULL in DB).
func ckOptIntPtr(i *int) pgtype.Int4 {
	if i == nil {
		return pgtype.Int4{}
	}
	return pgtype.Int4{Int32: int32(*i), Valid: true}
}

// ckOptUUIDFromStr converts a string to pgtype.UUID; empty -> invalid.
func ckOptUUIDFromStr(s string) pgtype.UUID {
	if s == "" {
		return pgtype.UUID{}
	}
	var u pgtype.UUID
	_ = u.Scan(s)
	return u
}

// ckOptTsPtr converts *time.Time to pgtype.Timestamptz; nil -> invalid.
func ckOptTsPtr(t *time.Time) pgtype.Timestamptz {
	if t == nil {
		return pgtype.Timestamptz{}
	}
	return pgtype.Timestamptz{Time: *t, Valid: true}
}

// ckOptDatePtr: nil string ptr -> invalid; "YYYY-MM-DD" string -> pgtype.Date.
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

// optTextStrPtr converts *string to pgtype.Text (nil -> invalid, *"" -> valid empty).
func optTextStrPtr(s *string) pgtype.Text {
	if s == nil {
		return pgtype.Text{}
	}
	return pgtype.Text{String: *s, Valid: true}
}

// ckOptUUIDFromPtr converts *string to pgtype.UUID; nil/empty -> invalid.
func ckOptUUIDFromPtr(s *string) pgtype.UUID {
	if s == nil || *s == "" {
		return pgtype.UUID{}
	}
	return ckOptUUIDFromStr(*s)
}

// uuidStringFromPgtype returns the UUID as string ("" when invalid).
func uuidStringFromPgtype(u pgtype.UUID) string {
	if !u.Valid {
		return ""
	}
	return u.String()
}

// policyDateFromTimePtr converts *time.Time -> pgtype.Date.
func policyDateFromTimePtr(t *time.Time) pgtype.Date {
	if t == nil {
		return pgtype.Date{}
	}
	return pgtype.Date{Time: *t, Valid: true}
}

func intPtrFromInt4(v pgtype.Int4) *int {
	if !v.Valid {
		return nil
	}
	i := int(v.Int32)
	return &i
}

// --- domain mappers (moved from the root vaktcomply package) ---

// frameworkFromCkFrameworks maps the sqlc-generated row to the Framework
// domain type. ReadinessScore is not stored in the table -- it's computed
// per-call in service layer.
func frameworkFromCkFrameworks(r db.CkFrameworks) Framework {
	variant := r.FrameworkVariant
	if variant == "" {
		variant = "full"
	}
	return Framework{
		ID:               r.ID,
		OrgID:            r.OrgID,
		Name:             r.Name,
		Version:          r.Version,
		IsBuiltin:        r.IsBuiltin,
		FrameworkVariant: variant,
		CreatedAt:        ckTsToTime(r.CreatedAt),
	}
}

// controlFields holds all columns shared between ListCKControls and GetCKControl
// row types. ADR-0013: explicit container so a single mapper handles both.
type controlFields struct {
	ID, FrameworkID, OrgID, ControlID, Title string
	Description                              pgtype.Text
	Domain, EvidenceType                     string
	Weight                                   int32
	NotApplicable                            bool
	NotApplicableReason, ManualStatus        pgtype.Text
	MaturityScore                            int16
	Owner                                    pgtype.Text
	LastReviewedAt                           pgtype.Timestamptz
	ReviewIntervalDays                       int32
	NextReviewDue                            pgtype.Timestamptz
	LastReviewedBy, ReviewNote               string
	DueDate                                  pgtype.Date
}

// policyFields collects the columns shared by all Policy-returning sqlc rows.
type policyFields struct {
	ID, OrgID, Title, Description, Category, Status, Version string
	EffectiveDate, ReviewDate                                pgtype.Date
	Owner                                                    string
	CreatedAt, UpdatedAt                                     pgtype.Timestamptz
	VersionNum                                               int32
	VersionNote, LastUpdatedBy                               string
	ReviewedAt                                               pgtype.Timestamptz
	NextReviewDue                                            pgtype.Date
}

func policyFromFields(f policyFields) Policy {
	return Policy{
		ID:            f.ID,
		OrgID:         f.OrgID,
		Title:         f.Title,
		Description:   f.Description,
		Category:      f.Category,
		Status:        f.Status,
		Version:       f.Version,
		VersionNum:    int(f.VersionNum),
		VersionNote:   f.VersionNote,
		LastUpdatedBy: f.LastUpdatedBy,
		ReviewedAt:    ckTsToTimePtr(f.ReviewedAt),
		NextReviewDue: dateToStringPtrLocal(f.NextReviewDue),
		EffectiveDate: ckDateToTimePtr(f.EffectiveDate),
		ReviewDate:    ckDateToTimePtr(f.ReviewDate),
		Owner:         f.Owner,
		CreatedAt:     ckTsToTime(f.CreatedAt),
		UpdatedAt:     ckTsToTime(f.UpdatedAt),
	}
}

// dateToStringPtrLocal yields "YYYY-MM-DD" or nil from pgtype.Date.
func dateToStringPtrLocal(d pgtype.Date) *string {
	if !d.Valid {
		return nil
	}
	s := d.Time.Format("2006-01-02")
	return &s
}

func controlFromFields(f controlFields) Control {
	nextReview := ckTsToTimePtr(f.NextReviewDue)
	overdue := nextReview != nil && nextReview.Before(time.Now())
	return Control{
		ID:                  f.ID,
		FrameworkID:         f.FrameworkID,
		OrgID:               f.OrgID,
		ControlID:           f.ControlID,
		Title:               f.Title,
		Description:         f.Description.String,
		Domain:              f.Domain,
		EvidenceType:        f.EvidenceType,
		Weight:              int(f.Weight),
		NotApplicable:       f.NotApplicable,
		NotApplicableReason: f.NotApplicableReason.String,
		ManualStatus:        f.ManualStatus.String,
		MaturityScore:       int(f.MaturityScore),
		Owner:               f.Owner.String,
		LastReviewedAt:      ckTsToTimePtr(f.LastReviewedAt),
		ReviewIntervalDays:  int(f.ReviewIntervalDays),
		NextReviewDue:       nextReview,
		LastReviewedBy:      f.LastReviewedBy,
		ReviewNote:          f.ReviewNote,
		IsReviewOverdue:     overdue,
		DueDate:             ckDateToTimePtr(f.DueDate),
	}
}
