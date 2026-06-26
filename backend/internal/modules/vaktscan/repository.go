// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package vaktscan

import (
	"encoding/json"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/matharnica/vakt/internal/db"
)

// Repository handles VulnBoard data access. Assets use sqlc; remaining tables
// (findings, components, sboms, scans, reports, sla_config) stay on embedded
// SQL until follow-up sessions migrate them (see docs/sqlc-migration-plan.md).
type Repository struct {
	db *pgxpool.Pool
	q  *db.Queries
}

// NewRepository creates a new VulnBoard repository.
func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{db: pool, q: db.New(pool)}
}

// optStringText is the vaktscan-local nullable-text helper.
func spOptText(s string) pgtype.Text {
	if s == "" {
		return pgtype.Text{}
	}
	return pgtype.Text{String: s, Valid: true}
}

func spTextPtr(t pgtype.Text) *string {
	if !t.Valid {
		return nil
	}
	s := t.String
	return &s
}

func spUUIDPtr(u pgtype.UUID) *string {
	if !u.Valid {
		return nil
	}
	s := u.String()
	return &s
}

func spOptUUID(s *string) pgtype.UUID {
	if s == nil || *s == "" {
		return pgtype.UUID{}
	}
	var u pgtype.UUID
	_ = u.Scan(*s)
	return u
}

func spTsToTime(t pgtype.Timestamptz) time.Time {
	if !t.Valid {
		return time.Time{}
	}
	return t.Time
}

func spTsToTimePtr(t pgtype.Timestamptz) *time.Time {
	if !t.Valid {
		return nil
	}
	tm := t.Time
	return &tm
}

func spInt8ToInt64Ptr(i pgtype.Int8) *int64 {
	if !i.Valid {
		return nil
	}
	v := i.Int64
	return &v
}

func spOptInt8(v *int64) pgtype.Int8 {
	if v == nil {
		return pgtype.Int8{}
	}
	return pgtype.Int8{Int64: *v, Valid: true}
}

func spOptInt4(v *int) pgtype.Int4 {
	if v == nil {
		return pgtype.Int4{}
	}
	return pgtype.Int4{Int32: int32(*v), Valid: true}
}

func spOptTs(t *time.Time) pgtype.Timestamptz {
	if t == nil {
		return pgtype.Timestamptz{}
	}
	return pgtype.Timestamptz{Time: *t, Valid: true}
}

// numericToFloat64Ptr converts a nullable pgtype.Numeric (PostgreSQL NUMERIC)
// to a *float64; returns nil when the source value is NULL.
func numericToFloat64Ptr(n pgtype.Numeric) *float64 {
	if !n.Valid {
		return nil
	}
	f8, err := n.Float64Value()
	if err != nil || !f8.Valid {
		return nil
	}
	v := f8.Float64
	return &v
}

// float64PtrToNumeric converts a *float64 to pgtype.Numeric; nil → invalid.
// Uses string-based Scan because pgtype.Numeric has no direct float64 setter.
func float64PtrToNumeric(f *float64) pgtype.Numeric {
	if f == nil {
		return pgtype.Numeric{}
	}
	var n pgtype.Numeric
	if err := n.Scan(strconv.FormatFloat(*f, 'f', -1, 64)); err != nil {
		return pgtype.Numeric{}
	}
	return n
}

// dateToStringPtr converts pgtype.Date to *string (YYYY-MM-DD); nil if invalid.
func dateToStringPtr(d pgtype.Date) *string {
	if !d.Valid {
		return nil
	}
	s := d.Time.Format("2006-01-02")
	return &s
}

// suppressionFromVbSuppression maps a generated row to the SuppressionRule domain type.
func suppressionFromVbSuppression(r db.VbFindingSuppressions) SuppressionRule {
	return SuppressionRule{
		ID:         r.ID,
		OrgID:      r.OrgID,
		CVEID:      spTextPtr(r.CveID),
		AssetTag:   spTextPtr(r.AssetTag),
		Reason:     r.Reason,
		CreatedBy:  spUUIDPtr(r.CreatedBy),
		MatchCount: int(r.MatchCount),
		CreatedAt:  spTsToTime(r.CreatedAt),
	}
}

// scheduleFromVbScanSchedule maps a generated row to the ScanSchedule domain type.
func scheduleFromVbScanSchedule(r db.VbScanSchedules) ScanSchedule {
	return ScanSchedule{
		ID:        r.ID,
		OrgID:     r.OrgID,
		AssetID:   r.AssetID,
		Scanner:   r.Scanner,
		CronExpr:  r.CronExpr,
		IsActive:  r.IsActive,
		LastRun:   spTsToTimePtr(r.LastRun),
		NextRun:   spTsToTimePtr(r.NextRun),
		CreatedAt: spTsToTime(r.CreatedAt),
	}
}

// reportFields is the union of fields returned by every Report-returning sqlc
// query (Create, Get, List). All currently match — keep them in one container
// in case future RETURNING-clauses diverge (ADR-0013).
type reportFields struct {
	ID, OrgID   string
	GeneratedBy pgtype.UUID
	Scope       json.RawMessage
	FilePath    pgtype.Text
	Status      string
	ExpiresAt   pgtype.Timestamptz
	CreatedAt   pgtype.Timestamptz
}

func reportFromFields(f reportFields) Report {
	var scope ReportScope
	if len(f.Scope) > 0 {
		_ = json.Unmarshal(f.Scope, &scope)
	}
	return Report{
		ID:          f.ID,
		OrgID:       f.OrgID,
		GeneratedBy: spUUIDPtr(f.GeneratedBy),
		Title:       scope.Title,
		Scope:       scope,
		FilePath:    f.FilePath.String,
		Status:      f.Status,
		ExpiresAt:   spTsToTimePtr(f.ExpiresAt),
		CreatedAt:   spTsToTime(f.CreatedAt),
	}
}

// findingFromVbFindings maps the generated sqlc row to the Finding domain type.
func findingFromVbFindings(r db.VbFindings) Finding {
	return Finding{
		ID:              r.ID,
		OrgID:           r.OrgID,
		AssetID:         r.AssetID,
		ScanID:          spUUIDPtr(r.ScanID),
		CVEID:           spTextPtr(r.CveID),
		Title:           r.Title,
		Description:     r.Description.String,
		Severity:        r.Severity,
		CVSSScore:       numericToFloat64Ptr(r.CvssScore),
		EPSSScore:       numericToFloat64Ptr(r.EpssScore),
		EPSSPercentile:  numericToFloat64Ptr(r.EpssPercentile),
		RiskScore:       numericToFloat64Ptr(r.RiskScore),
		Status:          r.Status,
		Scanner:         r.Scanner,
		RawID:           r.RawID.String,
		Sources:         r.Sources,
		TemplateID:      r.TemplateID.String,
		AssignedTo:      spUUIDPtr(r.AssignedTo),
		Justification:   r.Justification.String,
		ReopenCount:     int(r.ReopenCount),
		OccurrenceCount: int(r.OccurrenceCount),
		LastSeenAt:      spTsToTime(r.LastSeenAt),
		SLADueAt:        spTsToTimePtr(r.SlaDueAt),
		CreatedAt:       spTsToTime(r.CreatedAt),
		UpdatedAt:       spTsToTime(r.UpdatedAt),
	}
}

// scanFromVbScans maps the generated sqlc row to the Scan domain type.
func scanFromVbScans(r db.VbScans) Scan {
	return Scan{
		ID:           r.ID,
		OrgID:        r.OrgID,
		AssetID:      r.AssetID,
		Scanner:      r.Scanner,
		Status:       r.Status,
		TargetURL:    r.TargetUrl.String,
		TargetIP:     r.TargetIp.String,
		ErrorMessage: r.ErrorMessage.String,
		FindingCount: int(r.FindingCount),
		DurationMs:   spInt8ToInt64Ptr(r.DurationMs),
		StartedAt:    spTsToTimePtr(r.StartedAt),
		CompletedAt:  spTsToTimePtr(r.CompletedAt),
		CreatedAt:    spTsToTime(r.CreatedAt),
	}
}

// assetFields is the union of fields returned by every Asset-returning sqlc
// query. The Row-types diverge in column order (ADR-0013), so we centralise
// the mapping here.
type assetFields struct {
	ID, OrgID, Name, Type, Criticality, Environment string
	Tags                                            []string
	OwnerID                                         pgtype.UUID
	ExternalUrl                                     pgtype.Text
	CreatedAt, UpdatedAt                            pgtype.Timestamptz
}

func assetFromFields(f assetFields) Asset {
	env := f.Environment
	if env == "" {
		env = "prod"
	}
	return Asset{
		ID:          f.ID,
		OrgID:       f.OrgID,
		Name:        f.Name,
		Type:        f.Type,
		Criticality: f.Criticality,
		Environment: env,
		Tags:        f.Tags,
		OwnerID:     spUUIDPtr(f.OwnerID),
		ExternalURL: spTextPtr(f.ExternalUrl),
		CreatedAt:   spTsToTime(f.CreatedAt),
		UpdatedAt:   spTsToTime(f.UpdatedAt),
	}
}

// (Asset, Scan, Finding, Report, SBOM, and SLA data-access methods live in the
// sibling repository_*.go files; all remain in package vaktscan.)
