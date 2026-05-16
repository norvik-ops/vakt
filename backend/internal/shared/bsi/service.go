// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package bsi

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"

	"github.com/sechealth-app/sechealth/internal/shared/notify"
)

// assetKeywords is a set of lower-case product names that are matched against
// vb_assets.name to associate advisories with concrete assets.
var assetKeywords = []string{
	"nginx", "apache", "openssl", "linux", "kernel", "wordpress",
	"php", "python", "java", "nodejs", "node.js", "mysql", "postgresql",
	"redis", "docker", "kubernetes", "windows", "exchange", "openssh",
}

// BSIService orchestrates fetching and storing CERT-Bund advisories.
type BSIService struct {
	db *pgxpool.Pool
}

// NewBSIService constructs a BSIService.
func NewBSIService(db *pgxpool.Pool) *BSIService {
	return &BSIService{db: db}
}

// SyncFeed fetches the BSI feed, stores new advisories, and creates SecPulse
// findings where CVEs overlap with existing open findings or matching assets.
//
// Performance: asset rows and CVE-existence data are loaded once before the
// per-advisory loop so that we avoid N full-table scans.
func (s *BSIService) SyncFeed(ctx context.Context) error {
	advisories, err := FetchAdvisories(ctx)
	if err != nil {
		return fmt.Errorf("bsi: fetch advisories: %w", err)
	}

	log.Info().Int("count", len(advisories)).Msg("bsi: fetched advisories")

	// ── Prefetch: one representative asset ID per org ─────────────────────────
	// We load this once for the whole sync run, not once per CVE.
	orgAssetMap, err := s.loadOrgAssetMap(ctx)
	if err != nil {
		return fmt.Errorf("bsi: load org asset map: %w", err)
	}

	// ── Collect all CVE IDs across new advisories ─────────────────────────────
	// We will batch-query which orgs already have findings for each CVE.
	var allCVEIDs []string
	for _, adv := range advisories {
		allCVEIDs = append(allCVEIDs, adv.CVEIDs...)
	}
	// cveExistingOrgs maps cveID → set of orgIDs that already have a finding.
	cveExistingOrgs, err := s.loadCVEExistingOrgs(ctx, allCVEIDs)
	if err != nil {
		log.Error().Err(err).Msg("bsi: load cve existing orgs")
		cveExistingOrgs = make(map[string]map[string]struct{})
	}

	newFindingsByOrg := make(map[string]int)

	for _, adv := range advisories {
		isNew, err := s.upsertAdvisory(ctx, adv)
		if err != nil {
			log.Error().Err(err).Str("bsi_id", adv.BSIID).Msg("bsi: upsert advisory")
			continue
		}
		if !isNew {
			continue
		}

		if len(adv.CVEIDs) == 0 {
			// No CVEs — check asset-name keyword match per org.
			if err := s.createFindingsForAssetMatch(ctx, adv, newFindingsByOrg); err != nil {
				log.Error().Err(err).Str("bsi_id", adv.BSIID).Msg("bsi: asset match finding")
			}
			continue
		}

		// CVE-based: use pre-fetched data to avoid per-CVE DB round trips.
		for _, cveID := range adv.CVEIDs {
			existingOrgs := cveExistingOrgs[cveID] // may be nil — that's fine
			if err := s.createFindingForCVEFromMap(ctx, adv, cveID, orgAssetMap, existingOrgs, newFindingsByOrg); err != nil {
				log.Error().Err(err).Str("cve", cveID).Msg("bsi: create cve finding")
			}
		}
	}

	for orgID, count := range newFindingsByOrg {
		msg := fmt.Sprintf("%d neue BSI-Warnmeldungen wurden importiert und als Findings erstellt.", count)
		notify.Send(ctx, s.db, orgID, "BSI CERT-Bund Update", msg, "warning", "secpulse")
	}

	return nil
}

// ── private helpers ───────────────────────────────────────────────────────────

// loadOrgAssetMap fetches one representative (non-deleted) asset ID per org
// and returns a map[orgID]assetID. Called once per SyncFeed run.
func (s *BSIService) loadOrgAssetMap(ctx context.Context) (map[string]string, error) {
	rows, err := s.db.Query(ctx, `
		SELECT DISTINCT ON (org_id) org_id::text, id::text
		FROM   vb_assets
		WHERE  is_deleted = false
		ORDER  BY org_id, created_at`)
	if err != nil {
		return nil, fmt.Errorf("query org asset map: %w", err)
	}
	defer rows.Close()

	m := make(map[string]string)
	for rows.Next() {
		var orgID, assetID string
		if err := rows.Scan(&orgID, &assetID); err != nil {
			continue
		}
		m[orgID] = assetID
	}
	return m, rows.Err()
}

// loadCVEExistingOrgs batch-loads, for every CVE ID in cveIDs, the set of
// org IDs that already have a finding for that CVE. Returns map[cveID]→set of orgIDs.
func (s *BSIService) loadCVEExistingOrgs(ctx context.Context, cveIDs []string) (map[string]map[string]struct{}, error) {
	result := make(map[string]map[string]struct{})
	if len(cveIDs) == 0 {
		return result, nil
	}

	// Deduplicate CVE IDs before querying.
	seen := make(map[string]struct{}, len(cveIDs))
	unique := make([]string, 0, len(cveIDs))
	for _, id := range cveIDs {
		if _, ok := seen[id]; !ok {
			seen[id] = struct{}{}
			unique = append(unique, id)
		}
	}

	rows, err := s.db.Query(ctx, `
		SELECT DISTINCT cve_id, org_id::text
		FROM vb_findings
		WHERE cve_id = ANY($1::text[])`, unique)
	if err != nil {
		return nil, fmt.Errorf("query cve existing orgs: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var cveID, orgID string
		if err := rows.Scan(&cveID, &orgID); err != nil {
			continue
		}
		if result[cveID] == nil {
			result[cveID] = make(map[string]struct{})
		}
		result[cveID][orgID] = struct{}{}
	}
	return result, rows.Err()
}

// upsertAdvisory inserts the advisory or skips if it already exists.
// Returns true if a new row was inserted.
func (s *BSIService) upsertAdvisory(ctx context.Context, adv Advisory) (bool, error) {
	tag, err := s.db.Exec(ctx, `
		INSERT INTO bsi_advisories
		    (bsi_id, title, summary, severity, published_at, url, cve_ids)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (bsi_id) DO NOTHING`,
		adv.BSIID,
		adv.Title,
		adv.Summary,
		adv.Severity,
		adv.PublishedAt,
		adv.URL,
		adv.CVEIDs,
	)
	if err != nil {
		return false, fmt.Errorf("insert advisory: %w", err)
	}
	return tag.RowsAffected() > 0, nil
}

// createFindingForCVEFromMap uses pre-fetched orgAssetMap and existingOrgs to
// create findings without issuing any additional DB queries per CVE.
func (s *BSIService) createFindingForCVEFromMap(
	ctx context.Context,
	adv Advisory,
	cveID string,
	orgAssetMap map[string]string,
	existingOrgs map[string]struct{},
	newFindingsByOrg map[string]int,
) error {
	for orgID, assetID := range orgAssetMap {
		if _, already := existingOrgs[orgID]; already {
			continue
		}
		if err := s.insertFinding(ctx, orgID, assetID, adv, cveID, newFindingsByOrg); err != nil {
			log.Error().Err(err).Str("org_id", orgID).Str("cve", cveID).Msg("bsi: insert cve finding")
		}
	}
	return nil
}

// createFindingsForAssetMatch creates findings for assets whose name contains
// keywords found in the advisory title.
//
// To avoid an N+1 query pattern (one EXISTS per asset row), we collect all
// matching assets first and then batch-query which org_ids already have a BSI
// finding for this advisory title in a single round trip.
func (s *BSIService) createFindingsForAssetMatch(ctx context.Context, adv Advisory, newFindingsByOrg map[string]int) error {
	lowerTitle := strings.ToLower(adv.Title)

	// Collect all matching (orgID, assetID) pairs across all keywords first.
	type orgAsset struct{ orgID, assetID string }
	var candidates []orgAsset

	for _, kw := range assetKeywords {
		if !strings.Contains(lowerTitle, kw) {
			continue
		}

		rows, err := s.db.Query(ctx, `
			SELECT org_id::text, id::text
			FROM   vb_assets
			WHERE  is_deleted = false
			  AND  LOWER(name) LIKE $1`,
			"%"+kw+"%",
		)
		if err != nil {
			log.Error().Err(err).Str("kw", kw).Msg("bsi: asset keyword query")
			continue
		}
		for rows.Next() {
			var orgID, assetID string
			if err := rows.Scan(&orgID, &assetID); err != nil {
				continue
			}
			candidates = append(candidates, orgAsset{orgID, assetID})
		}
		rows.Close()
	}

	if len(candidates) == 0 {
		return nil
	}

	// Deduplicate by orgID (one finding check per org is sufficient).
	seen := make(map[string]string, len(candidates)) // orgID → assetID (first match)
	for _, c := range candidates {
		if _, ok := seen[c.orgID]; !ok {
			seen[c.orgID] = c.assetID
		}
	}

	// Batch-query which org_ids already have a BSI finding for this title.
	orgIDs := make([]string, 0, len(seen))
	for orgID := range seen {
		orgIDs = append(orgIDs, orgID)
	}

	existRows, err := s.db.Query(ctx, `
		SELECT DISTINCT org_id::text
		FROM   vb_findings
		WHERE  scanner = 'bsi'
		  AND  title   = $1
		  AND  org_id  = ANY($2::uuid[])`,
		adv.Title, orgIDs,
	)
	existingOrgs := make(map[string]struct{})
	if err != nil {
		log.Error().Err(err).Msg("bsi: batch exists query for asset match")
		// Continue without the exists filter — safer than skipping entirely.
	} else {
		for existRows.Next() {
			var orgID string
			if scanErr := existRows.Scan(&orgID); scanErr == nil {
				existingOrgs[orgID] = struct{}{}
			}
		}
		existRows.Close()
	}

	for orgID, assetID := range seen {
		if _, already := existingOrgs[orgID]; already {
			continue
		}
		if err := s.insertFinding(ctx, orgID, assetID, adv, "", newFindingsByOrg); err != nil {
			log.Error().Err(err).Str("org_id", orgID).Msg("bsi: insert asset match finding")
		}
	}
	return nil
}

// insertFinding creates a single vb_findings row.
func (s *BSIService) insertFinding(ctx context.Context, orgID, assetID string, adv Advisory, cveID string, newFindingsByOrg map[string]int) error {
	var cveArg *string
	if cveID != "" {
		cveArg = &cveID
	}
	_, err := s.db.Exec(ctx, `
		INSERT INTO vb_findings
		    (org_id, asset_id, cve_id, title, description,
		     severity, status, scanner, sources)
		VALUES
		    ($1::uuid, $2::uuid, $3, $4, $5,
		     $6, 'open', 'bsi', ARRAY['bsi'])`,
		orgID, assetID, cveArg, adv.Title, adv.Summary, adv.Severity,
	)
	if err != nil {
		return fmt.Errorf("insert bsi finding: %w", err)
	}
	newFindingsByOrg[orgID]++
	return nil
}
