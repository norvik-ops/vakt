// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package secvitals

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
)

// --- Auditor links ---

// CreateAuditorLink generates a time-limited read-only access token for an external auditor.
// Returns the raw (unhashed) token that should be delivered to the auditor.
func (s *Service) CreateAuditorLink(ctx context.Context, orgID, frameworkID, userID string, expiresIn time.Duration, maxUses *int) (string, error) {
	rawToken, tokenHash, err := generateToken()
	if err != nil {
		return "", fmt.Errorf("generate auditor token: %w", err)
	}

	expiresAt := time.Now().UTC().Add(expiresIn)
	_, err = s.repo.CreateAuditorLink(ctx, orgID, frameworkID, userID, tokenHash, expiresAt, maxUses)
	if err != nil {
		return "", fmt.Errorf("create auditor link: %w", err)
	}

	return rawToken, nil
}

// ValidateAuditorLink looks up an auditor link by its raw token, increments usage,
// and returns the associated framework.
func (s *Service) ValidateAuditorLink(ctx context.Context, rawToken string) (*Framework, error) {
	sum := sha256.Sum256([]byte(rawToken))
	tokenHash := hex.EncodeToString(sum[:])

	al, err := s.repo.GetAuditorLinkByHash(ctx, tokenHash)
	if err != nil {
		return nil, fmt.Errorf("invalid auditor link")
	}

	if time.Now().UTC().After(al.ExpiresAt) {
		return nil, fmt.Errorf("auditor link expired")
	}
	if al.MaxUses != nil && al.UsedCount >= *al.MaxUses {
		return nil, fmt.Errorf("auditor link usage limit reached")
	}

	if err := s.repo.IncrementAuditorLinkUsage(ctx, al.ID); err != nil {
		log.Warn().Err(err).Str("link_id", al.ID).Msg("failed to increment auditor link usage")
	}

	return s.repo.GetFramework(ctx, al.OrgID, al.FrameworkID)
}

// validateAuditorToken resolves a raw token to an AuditorLink, enforcing expiry and revocation.
// Returns the internal AuditorLink (not exposed to callers directly).
func (s *Service) validateAuditorToken(ctx context.Context, rawToken string) (*AuditorLink, error) {
	sum := sha256.Sum256([]byte(rawToken))
	tokenHash := hex.EncodeToString(sum[:])

	al, err := s.repo.GetAuditorLinkByHash(ctx, tokenHash)
	if err != nil {
		return nil, fmt.Errorf("invalid auditor link")
	}
	if time.Now().UTC().After(al.ExpiresAt) {
		return nil, fmt.Errorf("auditor link expired")
	}
	if al.MaxUses != nil && al.UsedCount >= *al.MaxUses {
		return nil, fmt.Errorf("auditor link usage limit reached")
	}
	// Update access tracking (best-effort).
	if err := s.repo.UpdateAuditorLinkAccess(ctx, al.ID); err != nil {
		log.Warn().Err(err).Str("link_id", al.ID).Msg("failed to update auditor link access")
	}
	return al, nil
}

// PreflightAuditorExport validates a token and returns the framework name without
// incrementing the access counter. Used by the handler to set Content-Disposition
// before streaming the ZIP body (ExportAuditorBundle increments on its own call).
func (s *Service) PreflightAuditorExport(ctx context.Context, rawToken string) (string, error) {
	sum := sha256.Sum256([]byte(rawToken))
	tokenHash := hex.EncodeToString(sum[:])

	al, err := s.repo.GetAuditorLinkByHash(ctx, tokenHash)
	if err != nil {
		return "", fmt.Errorf("invalid auditor link")
	}
	if time.Now().UTC().After(al.ExpiresAt) {
		return "", fmt.Errorf("auditor link expired")
	}

	fw, err := s.repo.GetFramework(ctx, al.OrgID, al.FrameworkID)
	if err != nil {
		return "", fmt.Errorf("get framework: %w", err)
	}
	return fw.Name, nil
}

// ListAuditorLinks returns all auditor links for the given organisation.
func (s *Service) ListAuditorLinks(ctx context.Context, orgID string) ([]AuditorLinkListItem, error) {
	links, err := s.repo.ListAuditorLinks(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("list auditor links: %w", err)
	}
	return links, nil
}

// RevokeAuditorLink marks an auditor link as revoked so it can no longer be used.
func (s *Service) RevokeAuditorLink(ctx context.Context, orgID, linkID string) error {
	if err := s.repo.RevokeAuditorLink(ctx, orgID, linkID); err != nil {
		return fmt.Errorf("revoke auditor link: %w", err)
	}
	return nil
}

// AuditorViewDetailed validates the token and returns the framework, readiness report,
// and each control with its evidence items — for the enhanced auditor portal (E09.2).
func (s *Service) AuditorViewDetailed(ctx context.Context, rawToken string) (*AuditorDetailView, error) {
	al, err := s.validateAuditorToken(ctx, rawToken)
	if err != nil {
		return nil, err
	}

	fw, err := s.repo.GetFramework(ctx, al.OrgID, al.FrameworkID)
	if err != nil {
		return nil, fmt.Errorf("get framework: %w", err)
	}

	controls, err := s.repo.ListControls(ctx, al.OrgID, al.FrameworkID)
	if err != nil {
		return nil, fmt.Errorf("list controls: %w", err)
	}

	evidenceCounts, err := s.repo.CountEvidenceByControl(ctx, al.OrgID, al.FrameworkID)
	if err != nil {
		return nil, fmt.Errorf("count evidence: %w", err)
	}

	report := computeReadinessReport(fw, controls, evidenceCounts)

	// Collect all control IDs for a single batch query instead of N per-control queries.
	controlIDs := make([]string, len(controls))
	for i, c := range controls {
		controlIDs[i] = c.ID
	}
	evidenceByControl, err := s.repo.ListEvidenceByControls(ctx, al.OrgID, controlIDs)
	if err != nil {
		return nil, fmt.Errorf("list evidence batch: %w", err)
	}

	withEvidence := make([]ControlWithEvidence, 0, len(controls))
	for i := range controls {
		c := controls[i]
		c.EvidenceCount = evidenceCounts[c.ID]
		c.Status = resolveStatus(c)

		items := evidenceByControl[c.ID]
		if items == nil {
			items = []Evidence{}
		}
		withEvidence = append(withEvidence, ControlWithEvidence{
			Control:  c,
			Evidence: items,
		})
	}

	return &AuditorDetailView{
		Framework: *fw,
		Report:    report,
		Controls:  withEvidence,
	}, nil
}

// ExportAuditorBundle validates the token and writes a ZIP to w with structure:
//
//	<framework_name>/
//	  <domain>/
//	    <control_code>/
//	      evidence_metadata.json
func (s *Service) ExportAuditorBundle(ctx context.Context, rawToken string, w io.Writer) (string, error) {
	al, err := s.validateAuditorToken(ctx, rawToken)
	if err != nil {
		return "", err
	}

	fw, err := s.repo.GetFramework(ctx, al.OrgID, al.FrameworkID)
	if err != nil {
		return "", fmt.Errorf("get framework: %w", err)
	}

	controls, err := s.repo.ListControls(ctx, al.OrgID, al.FrameworkID)
	if err != nil {
		return "", fmt.Errorf("list controls: %w", err)
	}

	// Batch-load all evidence in one query before writing the ZIP.
	controlIDs := make([]string, len(controls))
	for i, c := range controls {
		controlIDs[i] = c.ID
	}
	evidenceByControl, err := s.repo.ListEvidenceByControls(ctx, al.OrgID, controlIDs)
	if err != nil {
		return "", fmt.Errorf("list evidence batch: %w", err)
	}

	zw := zip.NewWriter(w)
	defer func() { _ = zw.Close() }()

	for i := range controls {
		c := controls[i]
		items := evidenceByControl[c.ID]
		if items == nil {
			items = []Evidence{}
		}

		path := fmt.Sprintf("%s/%s/%s/evidence_metadata.json", fw.Name, c.Domain, c.ControlID)
		f, err := zw.Create(path)
		if err != nil {
			return "", fmt.Errorf("create zip entry %s: %w", path, err)
		}
		meta := EvidenceMetadata{Control: c, Evidence: items}
		if err := json.NewEncoder(f).Encode(meta); err != nil {
			return "", fmt.Errorf("encode metadata for %s: %w", c.ControlID, err)
		}
	}

	if err := zw.Close(); err != nil {
		return "", fmt.Errorf("close zip: %w", err)
	}

	return fw.Name, nil
}


// --- Internal Audit Records (FR-CK15) ---

func (s *Service) ListAuditRecords(ctx context.Context, orgID string) ([]AuditRecord, error) {
	records, err := s.repo.ListAuditRecords(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("list audit records: %w", err)
	}
	if records == nil {
		records = []AuditRecord{}
	}
	return records, nil
}

func (s *Service) GetAuditRecord(ctx context.Context, orgID, id string) (*AuditRecord, error) {
	return s.repo.GetAuditRecord(ctx, orgID, id)
}

func (s *Service) CreateAuditRecord(ctx context.Context, orgID string, in CreateAuditRecordInput) (*AuditRecord, error) {
	return s.repo.CreateAuditRecord(ctx, orgID, in)
}

func (s *Service) UpdateAuditRecord(ctx context.Context, orgID, id string, in UpdateAuditRecordInput) (*AuditRecord, error) {
	return s.repo.UpdateAuditRecord(ctx, orgID, id, in)
}


// auditControlEntry groups evidence items under a single control for the audit export.
type auditControlEntry struct {
	ControlID    string // control_id column value, e.g. "A.5.1"
	ControlTitle string
	Evidence     []EvidenceForExport
}

// ExportAuditPackage erstellt ein ZIP-Archiv mit allen Compliance-Nachweisen für ein Framework.
// Die ZIP enthält:
//   - INDEX.pdf    — Übersicht aller Controls mit Status und Evidence-Liste
//   - summary.json — maschinenlesbare Zusammenfassung
//   - evidence/    — Ordner pro Control mit je einer Textdatei pro Evidence
func (s *Service) ExportAuditPackage(ctx context.Context, orgID, frameworkID string) (zipData []byte, filename string, err error) {
	// 1. Load framework metadata.
	fw, err := s.repo.GetFramework(ctx, orgID, frameworkID)
	if err != nil {
		return nil, "", fmt.Errorf("get framework: %w", err)
	}

	// 2. Load org name.
	var orgName string
	_ = s.db.QueryRow(ctx, `SELECT name FROM organizations WHERE id=$1::uuid`, orgID).Scan(&orgName)
	if orgName == "" {
		orgName = orgID
	}

	// 3. Load all evidence + control metadata in a single query.
	items, err := s.repo.ListEvidenceForFramework(ctx, orgID, frameworkID)
	if err != nil {
		return nil, "", fmt.Errorf("list evidence for framework: %w", err)
	}

	// 4. Build per-control groupings.
	var controlOrder []string
	controlMap := make(map[string]*auditControlEntry)
	evidenceTotal := 0

	for i := range items {
		item := &items[i]
		if _, seen := controlMap[item.ControlID]; !seen {
			controlOrder = append(controlOrder, item.ControlID)
			controlMap[item.ControlID] = &auditControlEntry{
				ControlID:    item.ControlDomain,
				ControlTitle: item.ControlTitle,
			}
		}
		if item.EvidenceID != "" {
			controlMap[item.ControlID].Evidence = append(controlMap[item.ControlID].Evidence, *item)
			evidenceTotal++
		}
	}

	controlsWithEvidence := 0
	for _, ce := range controlMap {
		if len(ce.Evidence) > 0 {
			controlsWithEvidence++
		}
	}
	controlsTotal := len(controlOrder)

	// 5. Generate INDEX.pdf.
	indexPDF, err := GenerateAuditIndexPDF(fw.Name, orgName, controlOrder, controlMap, time.Now())
	if err != nil {
		return nil, "", fmt.Errorf("generate index pdf: %w", err)
	}

	// 6. Build summary.json.
	type summaryJSON struct {
		Framework               string    `json:"framework"`
		Org                     string    `json:"org"`
		ExportedAt              time.Time `json:"exported_at"`
		ControlsTotal           int       `json:"controls_total"`
		ControlsWithEvidence    int       `json:"controls_with_evidence"`
		ControlsWithoutEvidence int       `json:"controls_without_evidence"`
		EvidenceTotal           int       `json:"evidence_total"`
	}
	summaryData, err := json.Marshal(summaryJSON{
		Framework:               fw.Name,
		Org:                     orgName,
		ExportedAt:              time.Now().UTC(),
		ControlsTotal:           controlsTotal,
		ControlsWithEvidence:    controlsWithEvidence,
		ControlsWithoutEvidence: controlsTotal - controlsWithEvidence,
		EvidenceTotal:           evidenceTotal,
	})
	if err != nil {
		return nil, "", fmt.Errorf("marshal summary: %w", err)
	}

	// 7. Assemble ZIP.
	exportDate := time.Now().UTC().Format("2006-01-02")
	safeName := strings.Map(func(r rune) rune {
		if r == ' ' {
			return '-'
		}
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			return r
		}
		return -1
	}, fw.Name)
	filename = fmt.Sprintf("audit-package-%s-%s.zip", safeName, exportDate)

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)

	// INDEX.pdf
	if f, zipErr := zw.Create("INDEX.pdf"); zipErr == nil {
		_, _ = f.Write(indexPDF)
	}

	// summary.json
	if f, zipErr := zw.Create("summary.json"); zipErr == nil {
		_, _ = f.Write(summaryData)
	}

	// evidence/ folder — one .txt file per evidence item.
	for _, ctrlID := range controlOrder {
		ce := controlMap[ctrlID]
		if len(ce.Evidence) == 0 {
			continue
		}
		folderName := auditSanitizePath(ce.ControlID + "-" + ce.ControlTitle)
		for i, ev := range ce.Evidence {
			entryName := fmt.Sprintf("evidence/%s/evidence_%03d.txt", folderName, i+1)
			f, zipErr := zw.Create(entryName)
			if zipErr != nil {
				continue
			}
			_, _ = fmt.Fprintf(f, "Evidence: %s\n", ev.EvidenceTitle)
			_, _ = fmt.Fprintf(f, "Control: %s — %s\n", ce.ControlID, ce.ControlTitle)
			_, _ = fmt.Fprintf(f, "Source: %s\n", ev.EvidenceSource)
			_, _ = fmt.Fprintf(f, "Collected: %s\n", ev.CollectedAt.UTC().Format("2006-01-02 15:04 UTC"))
			if ev.EvidenceDesc != "" {
				_, _ = fmt.Fprintf(f, "\nDescription:\n%s\n", ev.EvidenceDesc)
			}
			if ev.EvidenceFilePath != "" {
				_, _ = fmt.Fprintf(f, "\nFile reference: %s\n", ev.EvidenceFilePath)
			}
		}
	}

	if err := zw.Close(); err != nil {
		return nil, "", fmt.Errorf("close zip: %w", err)
	}
	return buf.Bytes(), filename, nil
}

// auditSanitizePath removes characters unsafe for ZIP entry paths.
func auditSanitizePath(s string) string {
	if len(s) > 60 {
		s = s[:60]
	}
	return strings.Map(func(r rune) rune {
		switch r {
		case '/', '\\', ':', '*', '?', '"', '<', '>', '|':
			return '_'
		}
		return r
	}, s)
}
