// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package vaktscan

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"

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

// enrichEnvironments fetches the environment column for a slice of assets via a
// single IN-query and patches the slice in place. Called after every sqlc-based
// asset read because the environment column was added after sqlc generation.
func (r *Repository) enrichEnvironments(ctx context.Context, assets []Asset) {
	if len(assets) == 0 {
		return
	}
	ids := make([]string, len(assets))
	for i, a := range assets {
		ids[i] = a.ID
	}
	rows, err := r.db.Query(ctx,
		`SELECT id, environment FROM vb_assets WHERE id = ANY($1)`, ids)
	if err != nil {
		return
	}
	defer rows.Close()
	envMap := make(map[string]string, len(assets))
	for rows.Next() {
		var id, env string
		if rows.Scan(&id, &env) == nil {
			envMap[id] = env
		}
	}
	for i := range assets {
		if env, ok := envMap[assets[i].ID]; ok {
			assets[i].Environment = env
		}
	}
}

// CreateAsset inserts a new asset row and returns the created record.
func (r *Repository) CreateAsset(ctx context.Context, orgID string, input CreateAssetInput) (*Asset, error) {
	tags := input.Tags
	if tags == nil {
		tags = []string{}
	}
	row, err := r.q.CreateSPAsset(ctx, db.CreateSPAssetParams{
		OrgID:       orgID,
		Name:        input.Name,
		Type:        input.Type,
		Criticality: input.Criticality,
		Tags:        tags,
		OwnerID:     spOptUUID(input.OwnerID),
		ExternalUrl: spOptText(input.ExternalURL),
	})
	if err != nil {
		return nil, fmt.Errorf("insert asset: %w", err)
	}
	env := input.Environment
	if env == "" {
		env = "prod"
	}
	if _, execErr := r.db.Exec(ctx,
		`UPDATE vb_assets SET environment=$1 WHERE id=$2`, env, row.ID); execErr != nil {
		log.Warn().Err(execErr).Str("asset_id", row.ID).Msg("could not set environment on new asset")
	}
	a := assetFromFields(assetFields{
		ID: row.ID, OrgID: row.OrgID, Name: row.Name, Type: row.Type,
		Criticality: row.Criticality, Environment: env, Tags: row.Tags,
		OwnerID: row.OwnerID, ExternalUrl: row.ExternalUrl,
		CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	})
	return &a, nil
}

// ListAssets returns a paginated list of non-deleted assets for an org.
// An optional tag filter restricts results to assets containing that tag.
func (r *Repository) ListAssets(ctx context.Context, orgID string, page, limit int, tag string) ([]Asset, int, error) {
	var tagParam pgtype.Text
	if tag != "" {
		tagParam = pgtype.Text{String: tag, Valid: true}
	}
	total, err := r.q.CountSPAssets(ctx, db.CountSPAssetsParams{
		OrgID: orgID,
		Tag:   tagParam,
	})
	if err != nil {
		return nil, 0, fmt.Errorf("count assets: %w", err)
	}
	offset := (page - 1) * limit
	rows, err := r.q.ListSPAssets(ctx, db.ListSPAssetsParams{
		OrgID:  orgID,
		Tag:    tagParam,
		Limit:  int32(limit),
		Offset: int32(offset),
	})
	if err != nil {
		return nil, 0, fmt.Errorf("query assets: %w", err)
	}
	out := make([]Asset, 0, len(rows))
	for _, row := range rows {
		out = append(out, assetFromFields(assetFields{
			ID: row.ID, OrgID: row.OrgID, Name: row.Name, Type: row.Type,
			Criticality: row.Criticality, Tags: row.Tags,
			OwnerID: row.OwnerID, ExternalUrl: row.ExternalUrl,
			CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
		}))
	}
	r.enrichEnvironments(ctx, out)
	return out, int(total), nil
}

// GetAsset fetches a single non-deleted asset by ID within the org.
func (r *Repository) GetAsset(ctx context.Context, orgID, assetID string) (*Asset, error) {
	row, err := r.q.GetSPAsset(ctx, db.GetSPAssetParams{ID: assetID, OrgID: orgID})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("asset not found: %w", err)
		}
		return nil, fmt.Errorf("get asset: %w", err)
	}
	tmp := []Asset{assetFromFields(assetFields{
		ID: row.ID, OrgID: row.OrgID, Name: row.Name, Type: row.Type,
		Criticality: row.Criticality, Tags: row.Tags,
		OwnerID: row.OwnerID, ExternalUrl: row.ExternalUrl,
		CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	})}
	r.enrichEnvironments(ctx, tmp)
	return &tmp[0], nil
}

// GetAssetByName fetches the first non-deleted asset matching name (case-insensitive) within the org.
// Returns nil, nil when no asset matches.
func (r *Repository) GetAssetByName(ctx context.Context, orgID, name string) (*Asset, error) {
	row, err := r.q.GetSPAssetByName(ctx, db.GetSPAssetByNameParams{OrgID: orgID, Column2: name})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("get asset by name: %w", err)
	}
	tmp := []Asset{assetFromFields(assetFields{
		ID: row.ID, OrgID: row.OrgID, Name: row.Name, Type: row.Type,
		Criticality: row.Criticality, Tags: row.Tags,
		OwnerID: row.OwnerID, ExternalUrl: row.ExternalUrl,
		CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	})}
	r.enrichEnvironments(ctx, tmp)
	return &tmp[0], nil
}

// ResolveAssetRef resolves an asset reference (UUID or name) to an asset ID.
// It first tries to treat ref as a UUID and look up by ID, then falls back to
// a case-insensitive name lookup.  Returns an error when no asset is found.
func (r *Repository) ResolveAssetRef(ctx context.Context, orgID, ref string) (string, error) {
	if ref == "" {
		return "", fmt.Errorf("asset reference is empty")
	}

	// Try ID lookup first (valid UUIDs are 36 chars).
	if len(ref) == 36 {
		if a, err := r.GetAsset(ctx, orgID, ref); err == nil {
			return a.ID, nil
		}
	}

	// Fall back to name lookup.
	a, err := r.GetAssetByName(ctx, orgID, ref)
	if err != nil {
		return "", fmt.Errorf("resolve asset %q: %w", ref, err)
	}
	if a == nil {
		return "", fmt.Errorf("asset %q not found", ref)
	}
	return a.ID, nil
}

// UpdateAsset applies a partial update to an asset.  Only non-nil fields are changed.
// Read-merge-write because sqlc cannot generate dynamic SET clauses (ADR-0005).
func (r *Repository) UpdateAsset(ctx context.Context, orgID, assetID string, input UpdateAssetInput) (*Asset, error) {
	cur, err := r.GetAsset(ctx, orgID, assetID)
	if err != nil {
		return nil, err
	}

	params := db.UpdateSPAssetParams{
		ID:          assetID,
		OrgID:       orgID,
		Name:        cur.Name,
		Type:        cur.Type,
		Criticality: cur.Criticality,
		Tags:        cur.Tags,
		OwnerID:     spOptUUID(cur.OwnerID),
		ExternalUrl: spOptText(derefStrPtr(cur.ExternalURL)),
	}
	if input.Name != nil {
		params.Name = *input.Name
	}
	if input.Type != nil {
		params.Type = *input.Type
	}
	if input.Criticality != nil {
		params.Criticality = *input.Criticality
	}
	if input.Tags != nil {
		params.Tags = input.Tags
	}
	if input.OwnerID != nil {
		params.OwnerID = spOptUUID(input.OwnerID)
	}
	if input.ExternalURL != nil {
		params.ExternalUrl = spOptText(*input.ExternalURL)
	}

	row, err := r.q.UpdateSPAsset(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("update asset: %w", err)
	}
	newEnv := cur.Environment
	if input.Environment != nil {
		newEnv = *input.Environment
		if _, execErr := r.db.Exec(ctx,
			`UPDATE vb_assets SET environment=$1 WHERE id=$2 AND org_id=$3`,
			newEnv, assetID, orgID); execErr != nil {
			log.Warn().Err(execErr).Str("asset_id", assetID).Msg("could not update environment")
		}
	}
	a := assetFromFields(assetFields{
		ID: row.ID, OrgID: row.OrgID, Name: row.Name, Type: row.Type,
		Criticality: row.Criticality, Environment: newEnv, Tags: row.Tags,
		OwnerID: row.OwnerID, ExternalUrl: row.ExternalUrl,
		CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	})
	return &a, nil
}

// SoftDeleteAsset marks an asset as deleted (is_deleted = TRUE).
func (r *Repository) SoftDeleteAsset(ctx context.Context, orgID, assetID string) error {
	n, err := r.q.SoftDeleteSPAsset(ctx, db.SoftDeleteSPAssetParams{ID: assetID, OrgID: orgID})
	if err != nil {
		return fmt.Errorf("soft delete asset: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("asset not found")
	}
	return nil
}

// derefStrPtr returns "" for a nil *string.
func derefStrPtr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// GetSLAConfig fetches the SLA configuration for an org; returns defaults if absent.
func (r *Repository) GetSLAConfig(ctx context.Context, orgID string) (*SLAConfig, error) {
	row, err := r.q.GetSPSLAConfig(ctx, orgID)
	if err != nil {
		// No row → return defaults
		return &SLAConfig{
			OrgID:        orgID,
			CriticalDays: 7,
			HighDays:     30,
			MediumDays:   90,
			LowDays:      180,
		}, nil
	}
	return &SLAConfig{
		OrgID:        row.OrgID,
		CriticalDays: int(row.CriticalDays),
		HighDays:     int(row.HighDays),
		MediumDays:   int(row.MediumDays),
		LowDays:      int(row.LowDays),
	}, nil
}

// UpsertSLAConfig inserts or updates the SLA config for an org.
func (r *Repository) UpsertSLAConfig(ctx context.Context, orgID string, input SLAConfig) error {
	err := r.q.UpsertSPSLAConfig(ctx, db.UpsertSPSLAConfigParams{
		OrgID:        orgID,
		CriticalDays: int32(input.CriticalDays),
		HighDays:     int32(input.HighDays),
		MediumDays:   int32(input.MediumDays),
		LowDays:      int32(input.LowDays),
	})
	if err != nil {
		return fmt.Errorf("upsert sla config: %w", err)
	}
	return nil
}

// slaDashboardRow is a raw DB row from the SLA dashboard query (no SLA logic applied).
type slaDashboardRow struct {
	AssetID      string
	AssetName    string
	FindingID    string
	FindingTitle string
	Severity     string
	Status       string
	DaysOpen     int
}

// GetSLADashboard returns up to 100 open findings with their age in days for the given org.
func (r *Repository) GetSLADashboard(ctx context.Context, orgID string) ([]slaDashboardRow, error) {
	rows, err := r.q.GetSPSLADashboard(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("sla dashboard query: %w", err)
	}
	result := make([]slaDashboardRow, 0, len(rows))
	for _, row := range rows {
		result = append(result, slaDashboardRow{
			AssetID:      row.AssetID,
			AssetName:    row.AssetName,
			FindingID:    row.FindingID,
			FindingTitle: row.FindingTitle,
			Severity:     row.Severity,
			Status:       row.Status,
			DaysOpen:     int(row.DaysOpen),
		})
	}
	return result, nil
}

// BulkCreateAssets inserts multiple assets in a single transaction and returns
// the number inserted, the number of errors, and a slice of error messages.
func (r *Repository) BulkCreateAssets(ctx context.Context, orgID string, rows []CSVAssetRow) (int, int, []string) {
	var inserted, errored int
	var errs []string

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return 0, len(rows), []string{fmt.Sprintf("begin transaction: %s", err)}
	}
	defer func() {
		_ = tx.Rollback(ctx) // no-op when Commit succeeded
	}()
	qtx := r.q.WithTx(tx)

	for i, row := range rows {
		tags := row.Tags
		if tags == nil {
			tags = []string{}
		}
		_, scanErr := qtx.CreateSPAsset(ctx, db.CreateSPAssetParams{
			OrgID:       orgID,
			Name:        row.Name,
			Type:        row.Type,
			Criticality: row.Criticality,
			Tags:        tags,
			OwnerID:     pgtype.UUID{}, // bulk import has no owner
			ExternalUrl: spOptText(row.ExternalURL),
		})
		if scanErr != nil {
			errored++
			errs = append(errs, fmt.Sprintf("row %d (%q): %s", i+1, row.Name, scanErr))
		} else {
			inserted++
		}
	}

	if err = tx.Commit(ctx); err != nil {
		return 0, len(rows), []string{fmt.Sprintf("commit: %s", err)}
	}
	return inserted, errored, errs
}

// ---------------------------------------------------------------------------
// Scans
// ---------------------------------------------------------------------------

// CreateScan inserts a new scan record and returns it.
func (r *Repository) CreateScan(ctx context.Context, orgID string, input CreateScanInput, assetID string) (*Scan, error) {
	row, err := r.q.CreateSPScan(ctx, db.CreateSPScanParams{
		OrgID:     orgID,
		AssetID:   assetID,
		Scanner:   input.Scanner,
		TargetUrl: spOptText(input.TargetURL),
		TargetIp:  spOptText(input.TargetIP),
	})
	if err != nil {
		return nil, fmt.Errorf("insert scan: %w", err)
	}
	s := scanFromVbScans(row)
	return &s, nil
}

// GetScan fetches a scan by ID within the org.
func (r *Repository) GetScan(ctx context.Context, orgID, scanID string) (*Scan, error) {
	row, err := r.q.GetSPScan(ctx, db.GetSPScanParams{ID: scanID, OrgID: orgID})
	if err != nil {
		return nil, fmt.Errorf("get scan: %w", err)
	}
	s := scanFromVbScans(row)
	return &s, nil
}

// UpdateScanStatus updates scan status and optional fields.
func (r *Repository) UpdateScanStatus(ctx context.Context, scanID, status string, opts ...ScanUpdateOpt) error {
	o := &scanUpdateOptions{}
	for _, opt := range opts {
		opt(o)
	}
	err := r.q.UpdateSPScanStatus(ctx, db.UpdateSPScanStatusParams{
		ID:           scanID,
		Status:       status,
		ErrorMessage: spOptText(derefStrPtr(o.errorMessage)),
		FindingCount: spOptInt4(o.findingCount),
		DurationMs:   spOptInt8(o.durationMs),
		StartedAt:    spOptTs(o.startedAt),
		CompletedAt:  spOptTs(o.completedAt),
	})
	if err != nil {
		return fmt.Errorf("update scan status: %w", err)
	}
	return nil
}

// ---------------------------------------------------------------------------
// Findings
// ---------------------------------------------------------------------------

// UpsertFinding inserts a finding or deduplicates if the same CVE / template
// was already found on the asset. Returns the upserted finding.
func (r *Repository) UpsertFinding(ctx context.Context, orgID string, f Finding) (*Finding, error) {
	sources := f.Sources
	if sources == nil {
		sources = []string{}
	}

	// Deduplicate on (org_id, asset_id, cve_id) for CVE findings or
	// (org_id, asset_id, scanner, template_id) for non-CVE findings.
	var existing Finding
	var scanErr error
	if f.CVEID != nil && *f.CVEID != "" {
		scanErr = r.db.QueryRow(ctx, `
			SELECT id::text, status, reopen_count, occurrence_count, sources
			FROM vb_findings
			WHERE org_id = $1::uuid AND asset_id = $2::uuid AND cve_id = $3
			LIMIT 1`,
			orgID, f.AssetID, *f.CVEID,
		).Scan(&existing.ID, &existing.Status, &existing.ReopenCount,
			&existing.OccurrenceCount, &existing.Sources)
	} else if f.TemplateID != "" {
		scanErr = r.db.QueryRow(ctx, `
			SELECT id::text, status, reopen_count, occurrence_count, sources
			FROM vb_findings
			WHERE org_id = $1::uuid AND asset_id = $2::uuid
			  AND scanner = $3 AND template_id = $4
			LIMIT 1`,
			orgID, f.AssetID, f.Scanner, f.TemplateID,
		).Scan(&existing.ID, &existing.Status, &existing.ReopenCount,
			&existing.OccurrenceCount, &existing.Sources)
	} else {
		scanErr = fmt.Errorf("no match key")
	}

	if scanErr == nil {
		// Existing record: update occurrence info.
		newStatus := existing.Status
		newReopenCount := existing.ReopenCount
		if existing.Status == "resolved" || existing.Status == "false_positive" {
			newStatus = "open"
			newReopenCount++
		}

		// Merge sources.
		srcSet := make(map[string]struct{})
		for _, s := range existing.Sources {
			srcSet[s] = struct{}{}
		}
		for _, s := range sources {
			srcSet[s] = struct{}{}
		}
		mergedSources := make([]string, 0, len(srcSet))
		for s := range srcSet {
			mergedSources = append(mergedSources, s)
		}

		var updated Finding
		err := r.db.QueryRow(ctx, `
			UPDATE vb_findings
			SET last_seen_at     = NOW(),
			    occurrence_count = occurrence_count + 1,
			    status           = $1,
			    reopen_count     = $2,
			    sources          = $3,
			    updated_at       = NOW()
			WHERE id = $4::uuid
			RETURNING id::text, org_id::text, asset_id::text,
			          scan_id::text, cve_id,
			          title, COALESCE(description,''), severity,
			          cvss_score, epss_score, epss_percentile, risk_score,
			          status, scanner, COALESCE(raw_id,''), sources,
			          COALESCE(template_id,''), assigned_to::text, COALESCE(justification,''),
			          reopen_count, occurrence_count,
			          last_seen_at, sla_due_at, created_at, updated_at`,
			newStatus, newReopenCount, mergedSources, existing.ID,
		).Scan(
			&updated.ID, &updated.OrgID, &updated.AssetID,
			&updated.ScanID, &updated.CVEID,
			&updated.Title, &updated.Description, &updated.Severity,
			&updated.CVSSScore, &updated.EPSSScore, &updated.EPSSPercentile, &updated.RiskScore,
			&updated.Status, &updated.Scanner, &updated.RawID, &updated.Sources,
			&updated.TemplateID, &updated.AssignedTo, &updated.Justification,
			&updated.ReopenCount, &updated.OccurrenceCount,
			&updated.LastSeenAt, &updated.SLADueAt, &updated.CreatedAt, &updated.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("update existing finding: %w", err)
		}
		return &updated, nil
	}

	// New finding: insert.
	var inserted Finding
	err := r.db.QueryRow(ctx, `
		INSERT INTO vb_findings
		  (org_id, asset_id, scan_id, cve_id, title, description, severity,
		   cvss_score, epss_score, epss_percentile, risk_score,
		   status, scanner, raw_id, sources, template_id,
		   assigned_to, justification, reopen_count, occurrence_count, last_seen_at)
		VALUES
		  ($1::uuid, $2::uuid, $3::uuid, $4, $5, $6, $7,
		   $8, $9, $10, $11,
		   $12, $13, $14, $15, $16,
		   $17::uuid, $18, 0, 1, NOW())
		RETURNING id::text, org_id::text, asset_id::text,
		          scan_id::text, cve_id,
		          title, COALESCE(description,''), severity,
		          cvss_score, epss_score, epss_percentile, risk_score,
		          status, scanner, COALESCE(raw_id,''), sources,
		          COALESCE(template_id,''), assigned_to::text, COALESCE(justification,''),
		          reopen_count, occurrence_count,
		          last_seen_at, sla_due_at, created_at, updated_at`,
		orgID, f.AssetID, f.ScanID, f.CVEID, f.Title, f.Description, f.Severity,
		f.CVSSScore, f.EPSSScore, f.EPSSPercentile, f.RiskScore,
		f.Status, f.Scanner, f.RawID, sources, f.TemplateID,
		f.AssignedTo, f.Justification,
	).Scan(
		&inserted.ID, &inserted.OrgID, &inserted.AssetID,
		&inserted.ScanID, &inserted.CVEID,
		&inserted.Title, &inserted.Description, &inserted.Severity,
		&inserted.CVSSScore, &inserted.EPSSScore, &inserted.EPSSPercentile, &inserted.RiskScore,
		&inserted.Status, &inserted.Scanner, &inserted.RawID, &inserted.Sources,
		&inserted.TemplateID, &inserted.AssignedTo, &inserted.Justification,
		&inserted.ReopenCount, &inserted.OccurrenceCount,
		&inserted.LastSeenAt, &inserted.SLADueAt, &inserted.CreatedAt, &inserted.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("insert finding: %w", err)
	}
	return &inserted, nil
}

// ListFindings returns findings for an org matching the given filter.
func (r *Repository) ListFindings(ctx context.Context, orgID string, filter FindingFilter) ([]Finding, error) {
	page := filter.Page
	if page < 1 {
		page = 1
	}
	limit := filter.Limit
	if limit < 1 || limit > 500 {
		limit = 25
	}
	offset := (page - 1) * limit

	severity := spOptText(filter.Severity)
	status := spOptText(filter.Status)
	assetID := spOptUUID(strPtrOrNil(filter.AssetID))

	var rows []db.VbFindings
	var err error
	if filter.SortBy == "created_at" {
		rows, err = r.q.ListSPFindingsByCreated(ctx, db.ListSPFindingsByCreatedParams{
			OrgID:    orgID,
			Limit:    int32(limit),
			Offset:   int32(offset),
			Severity: severity,
			Status:   status,
			AssetID:  assetID,
		})
	} else {
		rows, err = r.q.ListSPFindingsByRisk(ctx, db.ListSPFindingsByRiskParams{
			OrgID:    orgID,
			Limit:    int32(limit),
			Offset:   int32(offset),
			Severity: severity,
			Status:   status,
			AssetID:  assetID,
		})
	}
	if err != nil {
		return nil, fmt.Errorf("list findings: %w", err)
	}
	out := make([]Finding, 0, len(rows))
	for _, row := range rows {
		out = append(out, findingFromVbFindings(row))
	}
	return out, nil
}

// CountFindings returns the total number of findings matching the filter (ignoring page/limit).
func (r *Repository) CountFindings(ctx context.Context, orgID string, filter FindingFilter) (int, error) {
	total, err := r.q.CountSPFindings(ctx, db.CountSPFindingsParams{
		OrgID:    orgID,
		Severity: spOptText(filter.Severity),
		Status:   spOptText(filter.Status),
		AssetID:  spOptUUID(strPtrOrNil(filter.AssetID)),
	})
	if err != nil {
		return 0, fmt.Errorf("count findings: %w", err)
	}
	return int(total), nil
}

// GetFinding fetches a single finding by ID within the org.
func (r *Repository) GetFinding(ctx context.Context, orgID, findingID string) (*Finding, error) {
	row, err := r.q.GetSPFinding(ctx, db.GetSPFindingParams{ID: findingID, OrgID: orgID})
	if err != nil {
		return nil, fmt.Errorf("get finding: %w", err)
	}
	f := findingFromVbFindings(row)
	return &f, nil
}

// UpdateFinding applies a partial update to a finding.
func (r *Repository) UpdateFinding(ctx context.Context, orgID, findingID string, input UpdateFindingInput) (*Finding, error) {
	row, err := r.q.UpdateSPFinding(ctx, db.UpdateSPFindingParams{
		ID:            findingID,
		OrgID:         orgID,
		Status:        optTextPtr(input.Status),
		AssignedTo:    spOptUUID(input.AssignedTo),
		Justification: optTextPtr(input.Justification),
		Severity:      optTextPtr(input.Severity),
	})
	if err != nil {
		return nil, fmt.Errorf("update finding: %w", err)
	}
	f := findingFromVbFindings(row)
	return &f, nil
}

// BulkUpdateFindings applies a bulk status/assignee update; returns the number of affected rows.
func (r *Repository) BulkUpdateFindings(ctx context.Context, orgID string, input BulkFindingInput) (int, error) {
	if len(input.IDs) == 0 {
		return 0, nil
	}
	n, err := r.q.BulkUpdateSPFindings(ctx, db.BulkUpdateSPFindingsParams{
		OrgID:      orgID,
		Ids:        input.IDs,
		Status:     optTextPtr(input.Status),
		AssignedTo: spOptUUID(input.AssignedTo),
	})
	if err != nil {
		return 0, fmt.Errorf("bulk update findings: %w", err)
	}
	return int(n), nil
}

// strPtrOrNil returns a pointer to s when s != "", else nil.
func strPtrOrNil(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// optTextPtr converts a *string to a nullable pgtype.Text (nil → invalid).
func optTextPtr(s *string) pgtype.Text {
	if s == nil {
		return pgtype.Text{}
	}
	return pgtype.Text{String: *s, Valid: true}
}

// ---------------------------------------------------------------------------
// Suppression Rules
// ---------------------------------------------------------------------------

// CreateSuppressionRule inserts a new suppression rule.
func (r *Repository) CreateSuppressionRule(ctx context.Context, orgID, userID string, input CreateSuppressionInput) (*SuppressionRule, error) {
	row, err := r.q.CreateSPSuppression(ctx, db.CreateSPSuppressionParams{
		OrgID:     orgID,
		CveID:     optTextPtr(input.CVEID),
		AssetTag:  optTextPtr(input.AssetTag),
		Reason:    input.Reason,
		CreatedBy: spOptUUID(&userID),
	})
	if err != nil {
		return nil, fmt.Errorf("insert suppression rule: %w", err)
	}
	rule := suppressionFromVbSuppression(row)
	return &rule, nil
}

// ListSuppressionRules returns all suppression rules for an org.
func (r *Repository) ListSuppressionRules(ctx context.Context, orgID string) ([]SuppressionRule, error) {
	rows, err := r.q.ListSPSuppressions(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("list suppression rules: %w", err)
	}
	out := make([]SuppressionRule, 0, len(rows))
	for _, row := range rows {
		out = append(out, suppressionFromVbSuppression(row))
	}
	return out, nil
}

// DeleteSuppressionRule deletes a suppression rule by ID within the org.
func (r *Repository) DeleteSuppressionRule(ctx context.Context, orgID, ruleID string) error {
	n, err := r.q.DeleteSPSuppression(ctx, db.DeleteSPSuppressionParams{ID: ruleID, OrgID: orgID})
	if err != nil {
		return fmt.Errorf("delete suppression rule: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("suppression rule not found")
	}
	return nil
}

// ---------------------------------------------------------------------------
// Scan Schedules
// ---------------------------------------------------------------------------

// CreateScanSchedule inserts a new scan schedule for an asset.
func (r *Repository) CreateScanSchedule(ctx context.Context, orgID, assetID string, input CreateScanScheduleInput) (*ScanSchedule, error) {
	row, err := r.q.CreateSPScanSchedule(ctx, db.CreateSPScanScheduleParams{
		OrgID:    orgID,
		AssetID:  assetID,
		Scanner:  input.Scanner,
		CronExpr: input.CronExpr,
	})
	if err != nil {
		return nil, fmt.Errorf("insert scan schedule: %w", err)
	}
	s := scheduleFromVbScanSchedule(row)
	return &s, nil
}

// ListScanSchedules returns all scan schedules for an asset.
func (r *Repository) ListScanSchedules(ctx context.Context, orgID, assetID string) ([]ScanSchedule, error) {
	rows, err := r.q.ListSPScanSchedules(ctx, db.ListSPScanSchedulesParams{OrgID: orgID, AssetID: assetID})
	if err != nil {
		return nil, fmt.Errorf("list scan schedules: %w", err)
	}
	out := make([]ScanSchedule, 0, len(rows))
	for _, row := range rows {
		out = append(out, scheduleFromVbScanSchedule(row))
	}
	return out, nil
}

// DeleteScanSchedule removes a scan schedule by ID within the org.
func (r *Repository) DeleteScanSchedule(ctx context.Context, orgID, scheduleID string) error {
	n, err := r.q.DeleteSPScanSchedule(ctx, db.DeleteSPScanScheduleParams{ID: scheduleID, OrgID: orgID})
	if err != nil {
		return fmt.Errorf("delete scan schedule: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("scan schedule not found")
	}
	return nil
}

// ---------------------------------------------------------------------------
// Risk Trend
// ---------------------------------------------------------------------------

// GetRiskTrend returns daily aggregated risk data over the last N days.
// It reads from vb_risk_trend_snapshots when pre-computed data is available,
// falling back to the live generate_series query for orgs without snapshots yet.
func (r *Repository) GetRiskTrend(ctx context.Context, orgID string, days int) ([]RiskTrendPoint, error) {
	if days <= 0 {
		days = 30
	}

	// Prefer snapshot table — O(days) index scan instead of cartesian join.
	const snapshotSQL = `
		SELECT
			TO_CHAR(d::date, 'YYYY-MM-DD')         AS date,
			COALESCE(s.total_risk_score, 0)::float8 AS total_risk_score,
			COALESCE(s.open_count, 0)::int          AS open_count,
			COALESCE(s.critical_count, 0)::int      AS critical_count
		FROM generate_series(
			(CURRENT_DATE - make_interval(days => $2::int))::date,
			CURRENT_DATE,
			'1 day'::interval
		) AS d
		LEFT JOIN vb_risk_trend_snapshots s
			ON s.org_id = $1::uuid
		   AND s.snapshot_date = d::date
		ORDER BY d`

	snapRows, err := r.db.Query(ctx, snapshotSQL, orgID, days)
	if err == nil {
		defer snapRows.Close()
		var out []RiskTrendPoint
		for snapRows.Next() {
			var p RiskTrendPoint
			var openC, critC int32
			if scanErr := snapRows.Scan(&p.Date, &p.TotalRiskScore, &openC, &critC); scanErr != nil {
				continue
			}
			p.OpenCount = int(openC)
			p.CriticalCount = int(critC)
			out = append(out, p)
		}
		if snapRows.Err() == nil && len(out) > 0 {
			// At least one snapshot row exists — return snapshot data.
			return out, nil
		}
	}

	// No snapshots yet (fresh install, job hasn't run). Fall back to live query.
	liveRows, err := r.q.GetSPRiskTrend(ctx, db.GetSPRiskTrendParams{OrgID: orgID, Column2: int32(days)})
	if err != nil {
		return nil, fmt.Errorf("get risk trend: %w", err)
	}
	out := make([]RiskTrendPoint, 0, len(liveRows))
	for _, row := range liveRows {
		out = append(out, RiskTrendPoint{
			Date:           row.Date,
			TotalRiskScore: row.TotalRiskScore,
			OpenCount:      int(row.OpenCount),
			CriticalCount:  int(row.CriticalCount),
		})
	}
	return out, nil
}

// ---------------------------------------------------------------------------
// Reports
// ---------------------------------------------------------------------------

// CreateReport inserts a new report record.
func (r *Repository) CreateReport(ctx context.Context, orgID, userID string, scope ReportScope) (*Report, error) {
	scopeJSON, err := json.Marshal(scope)
	if err != nil {
		return nil, fmt.Errorf("marshal report scope: %w", err)
	}
	row, err := r.q.CreateSPReport(ctx, db.CreateSPReportParams{
		OrgID:       orgID,
		GeneratedBy: spOptUUID(&userID),
		Scope:       scopeJSON,
	})
	if err != nil {
		return nil, fmt.Errorf("insert report: %w", err)
	}
	rpt := reportFromFields(reportFields{
		ID: row.ID, OrgID: row.OrgID, GeneratedBy: row.GeneratedBy,
		Scope: row.Scope, FilePath: row.FilePath, Status: row.Status,
		ExpiresAt: row.ExpiresAt, CreatedAt: row.CreatedAt,
	})
	return &rpt, nil
}

// GetReport fetches a report by ID within the org.
func (r *Repository) GetReport(ctx context.Context, orgID, reportID string) (*Report, error) {
	row, err := r.q.GetSPReport(ctx, db.GetSPReportParams{ID: reportID, OrgID: orgID})
	if err != nil {
		return nil, fmt.Errorf("get report: %w", err)
	}
	rpt := reportFromFields(reportFields{
		ID: row.ID, OrgID: row.OrgID, GeneratedBy: row.GeneratedBy,
		Scope: row.Scope, FilePath: row.FilePath, Status: row.Status,
		ExpiresAt: row.ExpiresAt, CreatedAt: row.CreatedAt,
	})
	return &rpt, nil
}

// ListReports returns reports for an org, newest first (metadata only — no PDF blob).
func (r *Repository) ListReports(ctx context.Context, orgID string) ([]Report, error) {
	rows, err := r.q.ListSPReports(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("list reports: %w", err)
	}
	out := make([]Report, 0, len(rows))
	for _, row := range rows {
		out = append(out, reportFromFields(reportFields{
			ID: row.ID, OrgID: row.OrgID, GeneratedBy: row.GeneratedBy,
			Scope: row.Scope, FilePath: row.FilePath, Status: row.Status,
			ExpiresAt: row.ExpiresAt, CreatedAt: row.CreatedAt,
		}))
	}
	return out, nil
}

// UpsertFindingByRawID inserts a finding or updates it on conflict of
// (org_id, raw_id, scanner). This is used for import operations (SARIF, CycloneDX, CSV).
func (r *Repository) UpsertFindingByRawID(ctx context.Context, orgID string, f Finding) (*Finding, error) {
	sources := f.Sources
	if sources == nil {
		sources = []string{}
	}
	row, err := r.q.UpsertSPFindingByRawID(ctx, db.UpsertSPFindingByRawIDParams{
		OrgID:       orgID,
		AssetID:     f.AssetID,
		CveID:       optTextPtr(f.CVEID),
		Title:       f.Title,
		Description: spOptText(f.Description),
		Severity:    f.Severity,
		CvssScore:   float64PtrToNumeric(f.CVSSScore),
		Status:      f.Status,
		Scanner:     f.Scanner,
		RawID:       spOptText(f.RawID),
		Sources:     sources,
		SlaDueAt:    spOptTs(f.SLADueAt),
	})
	if err != nil {
		return nil, fmt.Errorf("upsert finding by raw_id: %w", err)
	}
	out := findingFromVbFindings(row)
	return &out, nil
}

// UpdateReport updates a report's file path, status, and expiry.
func (r *Repository) UpdateReport(ctx context.Context, reportID, filePath, status string, expiresAt *time.Time) error {
	err := r.q.UpdateSPReport(ctx, db.UpdateSPReportParams{
		ID:        reportID,
		FilePath:  spOptText(filePath),
		Status:    status,
		ExpiresAt: spOptTs(expiresAt),
	})
	if err != nil {
		return fmt.Errorf("update report: %w", err)
	}
	return nil
}

// StoreReportContent saves a generated PDF and marks the report completed.
func (r *Repository) StoreReportContent(ctx context.Context, reportID string, content []byte, expiresAt time.Time) error {
	err := r.q.StoreSPReportContent(ctx, db.StoreSPReportContentParams{
		ID:        reportID,
		Content:   content,
		ExpiresAt: pgtype.Timestamptz{Time: expiresAt, Valid: true},
	})
	if err != nil {
		return fmt.Errorf("store report content: %w", err)
	}
	return nil
}

// GetReportContent returns the raw PDF bytes and title for a completed report.
func (r *Repository) GetReportContent(ctx context.Context, orgID, reportID string) ([]byte, string, error) {
	row, err := r.q.GetSPReportContent(ctx, db.GetSPReportContentParams{ID: reportID, OrgID: orgID})
	if err != nil {
		return nil, "", fmt.Errorf("get report content: %w", err)
	}
	var scope ReportScope
	if len(row.Scope) > 0 {
		_ = json.Unmarshal(row.Scope, &scope)
	}
	title := scope.Title
	if title == "" {
		title = "report"
	}
	return row.Content, title, nil
}

// ---------------------------------------------------------------------------
// SBOM & EOL
// ---------------------------------------------------------------------------

// CreateSBOM inserts a new SBOM record and its components in a single transaction.
// Returns the newly created SBOM's UUID as a string.
func (r *Repository) CreateSBOM(ctx context.Context, orgID, assetID string, doc SBOMDocument) (string, error) {
	docJSON, err := json.Marshal(doc)
	if err != nil {
		return "", fmt.Errorf("marshal SBOM document: %w", err)
	}

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return "", fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }() // no-op when Commit succeeded
	qtx := r.q.WithTx(tx)

	sbomID, err := qtx.CreateSPSBOM(ctx, db.CreateSPSBOMParams{
		OrgID:          orgID,
		AssetID:        assetID,
		Format:         doc.BOMFormat,
		SpecVersion:    doc.SpecVersion,
		Document:       docJSON,
		ComponentCount: int32(len(doc.Components)),
	})
	if err != nil {
		return "", fmt.Errorf("insert vb_sboms: %w", err)
	}

	for _, comp := range doc.Components {
		if err := qtx.InsertSPComponent(ctx, db.InsertSPComponentParams{
			OrgID:   orgID,
			SbomID:  sbomID,
			Name:    comp.Name,
			Version: comp.Version,
			Purl:    spOptText(comp.PURL),
		}); err != nil {
			return "", fmt.Errorf("insert vb_components (%s %s): %w", comp.Name, comp.Version, err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return "", fmt.Errorf("commit SBOM tx: %w", err)
	}
	return sbomID, nil
}

// GetLatestSBOM returns the most recently created SBOM summary for the given asset.
func (r *Repository) GetLatestSBOM(ctx context.Context, orgID, assetID string) (*SBOMSummary, error) {
	row, err := r.q.GetLatestSPSBOM(ctx, db.GetLatestSPSBOMParams{OrgID: orgID, AssetID: assetID})
	if err != nil {
		return nil, fmt.Errorf("get latest SBOM: %w", err)
	}
	return &SBOMSummary{
		ID:             row.ID,
		AssetID:        row.AssetID,
		Format:         row.Format,
		ComponentCount: int(row.ComponentCount),
		CreatedAt:      spTsToTime(row.CreatedAt),
	}, nil
}

// ListComponentsWithEOL returns paginated components for an org, optionally filtered to EOL-only.
// page is 1-based; up to 500 rows per page.
func (r *Repository) ListComponentsWithEOL(ctx context.Context, orgID string, eolOnly bool, page int) ([]ComponentSummary, error) {
	if page < 1 {
		page = 1
	}
	const limit = 500
	offset := (page - 1) * limit

	if eolOnly {
		rows, err := r.q.ListSPComponentsEOL(ctx, db.ListSPComponentsEOLParams{
			OrgID:  orgID,
			Limit:  int32(limit),
			Offset: int32(offset),
		})
		if err != nil {
			return nil, fmt.Errorf("list components: %w", err)
		}
		out := make([]ComponentSummary, 0, len(rows))
		for _, c := range rows {
			out = append(out, ComponentSummary{
				ID: c.ID, Name: c.Name, Version: c.Version,
				PURL: c.Purl.String, EOLStatus: c.EolStatus,
				EOLDate: dateToStringPtr(c.EolDate), AssetID: c.AssetID,
			})
		}
		return out, nil
	}
	rows, err := r.q.ListSPComponentsAll(ctx, db.ListSPComponentsAllParams{
		OrgID:  orgID,
		Limit:  int32(limit),
		Offset: int32(offset),
	})
	if err != nil {
		return nil, fmt.Errorf("list components: %w", err)
	}
	out := make([]ComponentSummary, 0, len(rows))
	for _, c := range rows {
		out = append(out, ComponentSummary{
			ID: c.ID, Name: c.Name, Version: c.Version,
			PURL: c.Purl.String, EOLStatus: c.EolStatus,
			EOLDate: dateToStringPtr(c.EolDate), AssetID: c.AssetID,
		})
	}
	return out, nil
}

// listComponentsBySBOM returns the raw component rows for a given sbom_id (used by EOLChecker).
func (r *Repository) listComponentsBySBOM(ctx context.Context, sbomID string) ([]componentRow, error) {
	rows, err := r.q.ListSPComponentsBySBOM(ctx, sbomID)
	if err != nil {
		return nil, fmt.Errorf("list components by SBOM: %w", err)
	}
	out := make([]componentRow, 0, len(rows))
	for _, c := range rows {
		out = append(out, componentRow{ID: c.ID, Name: c.Name, Version: c.Version})
	}
	return out, nil
}

// upsertEOLCache inserts or updates a cache row for the (product, cycle) pair.
func (r *Repository) upsertEOLCache(ctx context.Context, product, cycle string, payload []byte) error {
	err := r.q.UpsertSPEOLCache(ctx, db.UpsertSPEOLCacheParams{
		Product: product,
		Cycle:   cycle,
		Payload: payload,
	})
	if err != nil {
		return fmt.Errorf("upsert EOL cache: %w", err)
	}
	return nil
}

// ---------------------------------------------------------------------------
// Batch finding upsert
// ---------------------------------------------------------------------------

// BatchUpsertFindings inserts or deduplicates multiple findings in a single
// pgx.Batch round-trip. Each finding is upserted using the same logic as
// UpsertFinding but without returning the full row, to minimise wire overhead.
// Errors for individual rows are logged but do not abort the batch; the number
// of successfully processed rows is returned.
func (r *Repository) BatchUpsertFindings(ctx context.Context, orgID string, findings []Finding) (int, error) {
	if len(findings) == 0 {
		return 0, nil
	}

	batch := &pgx.Batch{}
	for _, f := range findings {
		sources := f.Sources
		if sources == nil {
			sources = []string{}
		}

		if f.CVEID != nil && *f.CVEID != "" {
			// CVE-keyed upsert: merge on (org_id, asset_id, cve_id).
			batch.Queue(`
				INSERT INTO vb_findings
				  (org_id, asset_id, scan_id, cve_id, title, description, severity,
				   cvss_score, epss_score, epss_percentile, risk_score,
				   status, scanner, raw_id, sources, template_id,
				   assigned_to, justification, reopen_count, occurrence_count, last_seen_at)
				VALUES
				  ($1::uuid, $2::uuid, $3::uuid, $4, $5, $6, $7,
				   $8, $9, $10, $11,
				   $12, $13, $14, $15, $16,
				   $17::uuid, $18, 0, 1, NOW())
				ON CONFLICT (org_id, asset_id, cve_id) WHERE cve_id IS NOT NULL DO UPDATE
				  SET last_seen_at     = NOW(),
				      occurrence_count = vb_findings.occurrence_count + 1,
				      status           = CASE
				                          WHEN vb_findings.status IN ('resolved','false_positive') THEN 'open'
				                          ELSE vb_findings.status
				                        END,
				      reopen_count     = CASE
				                          WHEN vb_findings.status IN ('resolved','false_positive') THEN vb_findings.reopen_count + 1
				                          ELSE vb_findings.reopen_count
				                        END,
				      sources          = (SELECT ARRAY(SELECT DISTINCT unnest(vb_findings.sources || EXCLUDED.sources))),
				      updated_at       = NOW()`,
				orgID, f.AssetID, f.ScanID, f.CVEID, f.Title, f.Description, f.Severity,
				f.CVSSScore, f.EPSSScore, f.EPSSPercentile, f.RiskScore,
				f.Status, f.Scanner, f.RawID, sources, f.TemplateID,
				f.AssignedTo, f.Justification,
			)
		} else if f.TemplateID != "" {
			// Template-keyed upsert: merge on (org_id, asset_id, scanner, template_id).
			batch.Queue(`
				INSERT INTO vb_findings
				  (org_id, asset_id, scan_id, cve_id, title, description, severity,
				   cvss_score, epss_score, epss_percentile, risk_score,
				   status, scanner, raw_id, sources, template_id,
				   assigned_to, justification, reopen_count, occurrence_count, last_seen_at)
				VALUES
				  ($1::uuid, $2::uuid, $3::uuid, $4, $5, $6, $7,
				   $8, $9, $10, $11,
				   $12, $13, $14, $15, $16,
				   $17::uuid, $18, 0, 1, NOW())
				ON CONFLICT (org_id, asset_id, scanner, template_id) WHERE template_id IS NOT NULL DO UPDATE
				  SET last_seen_at     = NOW(),
				      occurrence_count = vb_findings.occurrence_count + 1,
				      status           = CASE
				                          WHEN vb_findings.status IN ('resolved','false_positive') THEN 'open'
				                          ELSE vb_findings.status
				                        END,
				      reopen_count     = CASE
				                          WHEN vb_findings.status IN ('resolved','false_positive') THEN vb_findings.reopen_count + 1
				                          ELSE vb_findings.reopen_count
				                        END,
				      sources          = (SELECT ARRAY(SELECT DISTINCT unnest(vb_findings.sources || EXCLUDED.sources))),
				      updated_at       = NOW()`,
				orgID, f.AssetID, f.ScanID, f.CVEID, f.Title, f.Description, f.Severity,
				f.CVSSScore, f.EPSSScore, f.EPSSPercentile, f.RiskScore,
				f.Status, f.Scanner, f.RawID, sources, f.TemplateID,
				f.AssignedTo, f.Justification,
			)
		} else {
			// No dedup key: plain insert.
			batch.Queue(`
				INSERT INTO vb_findings
				  (org_id, asset_id, scan_id, cve_id, title, description, severity,
				   cvss_score, epss_score, epss_percentile, risk_score,
				   status, scanner, raw_id, sources, template_id,
				   assigned_to, justification, reopen_count, occurrence_count, last_seen_at)
				VALUES
				  ($1::uuid, $2::uuid, $3::uuid, $4, $5, $6, $7,
				   $8, $9, $10, $11,
				   $12, $13, $14, $15, $16,
				   $17::uuid, $18, 0, 1, NOW())`,
				orgID, f.AssetID, f.ScanID, f.CVEID, f.Title, f.Description, f.Severity,
				f.CVSSScore, f.EPSSScore, f.EPSSPercentile, f.RiskScore,
				f.Status, f.Scanner, f.RawID, sources, f.TemplateID,
				f.AssignedTo, f.Justification,
			)
		}
	}

	br := r.db.SendBatch(ctx, batch)
	defer br.Close()

	var count int
	for i := range findings {
		if _, err := br.Exec(); err != nil {
			log.Error().Err(err).Int("index", i).Msg("batch upsert finding: row failed")
		} else {
			count++
		}
	}
	return count, nil
}

// ---------------------------------------------------------------------------
// EOL batch helpers
// ---------------------------------------------------------------------------

// eolCacheRow is an internal struct for batch cache lookups.
type eolCacheRow struct {
	payload   []byte
	fetchedAt time.Time
}

// batchGetEOLCache loads all cache entries for the given (product, cycle) pairs
// in a single query. Returns a map keyed by [product, cycle].
func (r *Repository) batchGetEOLCache(ctx context.Context, pairs [][2]string) (map[[2]string]eolCacheRow, error) {
	result := make(map[[2]string]eolCacheRow, len(pairs))
	if len(pairs) == 0 {
		return result, nil
	}

	// Build WHERE clause: (product, cycle) IN (($1,$2), ($3,$4), ...)
	args := make([]any, 0, len(pairs)*2)
	placeholders := make([]string, 0, len(pairs))
	for i, p := range pairs {
		a := i * 2
		placeholders = append(placeholders, fmt.Sprintf("($%d, $%d)", a+1, a+2))
		args = append(args, p[0], p[1])
	}

	query := fmt.Sprintf(`
		SELECT product, cycle, payload, fetched_at
		FROM vb_eol_cache
		WHERE (product, cycle) IN (%s)`, strings.Join(placeholders, ", "))

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("batch get EOL cache: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var product, cycle string
		var row eolCacheRow
		if err := rows.Scan(&product, &cycle, &row.payload, &row.fetchedAt); err != nil {
			continue
		}
		result[[2]string{product, cycle}] = row
	}
	return result, rows.Err()
}

// batchUpdateComponentEOL updates eol_status and eol_date for multiple components
// in a single statement using an unnest-based approach.
func (r *Repository) batchUpdateComponentEOL(ctx context.Context, results []eolResult) error {
	if len(results) == 0 {
		return nil
	}

	ids := make([]string, len(results))
	statuses := make([]string, len(results))
	dates := make([]*string, len(results))
	for i, res := range results {
		ids[i] = res.componentID
		statuses[i] = res.eolStatus
		dates[i] = res.eolDate
	}

	return r.q.BatchUpdateSPComponentEOL(ctx, db.BatchUpdateSPComponentEOLParams{
		Ids:      ids,
		Statuses: statuses,
		Dates:    dates,
	})
}

// ListFindingsCursor returns findings using keyset pagination.
// Fetch limit+1 rows so callers can detect HasMore; the caller strips the extra row.
func (r *Repository) ListFindingsCursor(ctx context.Context, orgID string, filter FindingFilter, cursorID string, cursorTS time.Time, limit int) ([]Finding, error) {
	const baseQuery = `
		SELECT id, org_id, asset_id, scan_id, cve_id,
		       title, description, severity,
		       cvss_score, epss_score, epss_percentile, risk_score,
		       status, scanner, raw_id, sources, template_id,
		       assigned_to, justification,
		       reopen_count, occurrence_count,
		       last_seen_at, sla_due_at, created_at, updated_at
		FROM vb_findings
		WHERE org_id = $1`

	args := []any{orgID}
	q := baseQuery
	n := 2

	if filter.Severity != "" {
		args = append(args, filter.Severity)
		q += fmt.Sprintf(" AND severity = $%d", n)
		n++
	}
	if filter.Status != "" {
		args = append(args, filter.Status)
		q += fmt.Sprintf(" AND status = $%d", n)
		n++
	}
	if !cursorTS.IsZero() && cursorID != "" {
		args = append(args, cursorTS, cursorID)
		q += fmt.Sprintf(" AND (created_at < $%d OR (created_at = $%d AND id::text < $%d))", n, n, n+1)
		n += 2
	}
	args = append(args, int32(limit+1))
	q += fmt.Sprintf(" ORDER BY created_at DESC, id DESC LIMIT $%d", n)

	rows, err := r.db.Query(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("list findings cursor: %w", err)
	}
	defer rows.Close()

	var out []Finding
	for rows.Next() {
		var f vbFindingRow
		if err := rows.Scan(
			&f.ID, &f.OrgID, &f.AssetID, &f.ScanID, &f.CVEID,
			&f.Title, &f.Description, &f.Severity,
			&f.CVSSScore, &f.EPSSScore, &f.EPSSPercentile, &f.RiskScore,
			&f.Status, &f.Scanner, &f.RawID, &f.Sources, &f.TemplateID,
			&f.AssignedTo, &f.Justification,
			&f.ReopenCount, &f.OccurrenceCount,
			&f.LastSeenAt, &f.SLADueAt, &f.CreatedAt, &f.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan finding cursor row: %w", err)
		}
		out = append(out, findingFromRow(f))
	}
	return out, rows.Err()
}

// vbFindingRow is a scan target for raw vb_findings queries.
type vbFindingRow struct {
	ID              pgtype.UUID
	OrgID           string
	AssetID         pgtype.UUID
	ScanID          pgtype.UUID
	CVEID           pgtype.Text
	Title           string
	Description     pgtype.Text
	Severity        string
	CVSSScore       pgtype.Float8
	EPSSScore       pgtype.Float8
	EPSSPercentile  pgtype.Float8
	RiskScore       pgtype.Float8
	Status          string
	Scanner         string
	RawID           pgtype.Text
	Sources         []string
	TemplateID      pgtype.Text
	AssignedTo      pgtype.UUID
	Justification   pgtype.Text
	ReopenCount     int32
	OccurrenceCount int32
	LastSeenAt      pgtype.Timestamptz
	SLADueAt        pgtype.Timestamptz
	CreatedAt       pgtype.Timestamptz
	UpdatedAt       pgtype.Timestamptz
}

func findingFromRow(r vbFindingRow) Finding {
	f := Finding{
		ID:              r.ID.String(),
		OrgID:           r.OrgID,
		AssetID:         r.AssetID.String(),
		Severity:        r.Severity,
		Status:          r.Status,
		Scanner:         r.Scanner,
		Title:           r.Title,
		ReopenCount:     int(r.ReopenCount),
		OccurrenceCount: int(r.OccurrenceCount),
		CreatedAt:       spTsToTime(r.CreatedAt),
		UpdatedAt:       spTsToTime(r.UpdatedAt),
		LastSeenAt:      spTsToTime(r.LastSeenAt),
	}
	if r.Description.Valid {
		f.Description = r.Description.String
	}
	if r.CVEID.Valid {
		s := r.CVEID.String
		f.CVEID = &s
	}
	if r.ScanID.Valid {
		s := r.ScanID.String()
		f.ScanID = &s
	}
	if r.CVSSScore.Valid {
		v := r.CVSSScore.Float64
		f.CVSSScore = &v
	}
	if r.EPSSScore.Valid {
		v := r.EPSSScore.Float64
		f.EPSSScore = &v
	}
	if r.EPSSPercentile.Valid {
		v := r.EPSSPercentile.Float64
		f.EPSSPercentile = &v
	}
	if r.RiskScore.Valid {
		v := r.RiskScore.Float64
		f.RiskScore = &v
	}
	if r.RawID.Valid {
		f.RawID = r.RawID.String
	}
	if r.Sources != nil {
		f.Sources = r.Sources
	}
	if r.TemplateID.Valid {
		f.TemplateID = r.TemplateID.String
	}
	if r.AssignedTo.Valid {
		s := r.AssignedTo.String()
		f.AssignedTo = &s
	}
	if r.Justification.Valid {
		f.Justification = r.Justification.String
	}
	if r.SLADueAt.Valid {
		t := r.SLADueAt.Time
		f.SLADueAt = &t
	}
	return f
}
