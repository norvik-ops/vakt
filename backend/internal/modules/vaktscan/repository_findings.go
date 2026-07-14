// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package vaktscan

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/matharnica/vakt/internal/db"
)

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
		f.Status, f.Scanner, dedupKey(f.RawID), sources, dedupKey(f.TemplateID),
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

// DeleteFinding removes a finding by ID within the org.
func (r *Repository) DeleteFinding(ctx context.Context, orgID, findingID string) error {
	n, err := r.q.DeleteSPFinding(ctx, db.DeleteSPFindingParams{ID: findingID, OrgID: orgID})
	if err != nil {
		return fmt.Errorf("delete finding: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("finding not found")
	}
	return nil
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
