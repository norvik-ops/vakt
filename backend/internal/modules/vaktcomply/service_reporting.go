// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package vaktcomply

import (
	"context"
	"fmt"
	"time"

	"github.com/rs/zerolog/log"
)

// ExportFrameworkPDF generates a human-readable compliance overview PDF.
// Returns (pdfBytes, filename, error).
func (s *Service) ExportFrameworkPDF(ctx context.Context, orgID, frameworkID string) ([]byte, string, error) {
	report, err := s.GetReadinessReport(ctx, orgID, frameworkID)
	if err != nil {
		return nil, "", fmt.Errorf("get readiness report: %w", err)
	}
	gaps, err := s.GetGapAnalysis(ctx, orgID, frameworkID)
	if err != nil {
		return nil, "", fmt.Errorf("get gap analysis: %w", err)
	}
	orgName := fetchOrgName(ctx, s.db, orgID)

	pdfBytes, err := GenerateFrameworkPDF(report, gaps, orgName)
	if err != nil {
		return nil, "", fmt.Errorf("generate pdf: %w", err)
	}
	filename := report.FrameworkName + " Compliance-Übersicht.pdf"
	return pdfBytes, filename, nil
}

// ExportSoAPDF generates an ISO 27001 Statement of Applicability PDF for the given framework.
// Returns (pdfBytes, filename, error).
func (s *Service) ExportSoAPDF(ctx context.Context, orgID, frameworkID string) ([]byte, string, error) {
	fw, err := s.repo.GetFramework(ctx, orgID, frameworkID)
	if err != nil {
		return nil, "", fmt.Errorf("get framework: %w", err)
	}
	rows, err := s.repo.ListControlsForSoA(ctx, orgID, frameworkID)
	if err != nil {
		return nil, "", fmt.Errorf("list controls for soa: %w", err)
	}
	orgName := fetchOrgName(ctx, s.db, orgID)

	pdfBytes, err := GenerateSoAPDF(rows, fw.Name, orgName, time.Now())
	if err != nil {
		return nil, "", fmt.Errorf("generate soa pdf: %w", err)
	}
	filename := fw.Name + " — Statement of Applicability.pdf"
	return pdfBytes, filename, nil
}

// UpdateSoAMetadata persists the SoA-specific fields for a single control.
func (s *Service) UpdateSoAMetadata(ctx context.Context, orgID, controlID string, in UpdateSoAMetadataInput) error {
	return s.repo.UpdateSoAMetadata(ctx, orgID, controlID, in.Justification, in.Implementation, in.Responsible)
}

// ExportTISAXReportPDF generates a TISAX® Bereitschaftsbericht PDF.
// Returns (pdfBytes, filename, error).
func (s *Service) ExportTISAXReportPDF(ctx context.Context, orgID, frameworkID, protectionLevel, assessmentLevel string) ([]byte, string, error) {
	// Validate and default protectionLevel.
	if protectionLevel == "" {
		protectionLevel = "normal"
	}
	validProtectionLevels := map[string]bool{"normal": true, "high": true, "very_high": true}
	if !validProtectionLevels[protectionLevel] {
		return nil, "", fmt.Errorf("invalid protection_level %q: must be one of normal, high, very_high", protectionLevel)
	}

	// Validate and default assessmentLevel.
	if assessmentLevel == "" {
		assessmentLevel = "AL2"
	}
	validAssessmentLevels := map[string]bool{"AL1": true, "AL2": true, "AL3": true}
	if !validAssessmentLevels[assessmentLevel] {
		return nil, "", fmt.Errorf("invalid assessment_level %q: must be one of AL1, AL2, AL3", assessmentLevel)
	}

	report, err := s.GetReadinessReport(ctx, orgID, frameworkID)
	if err != nil {
		return nil, "", fmt.Errorf("get readiness report: %w", err)
	}

	controls, err := s.ListTISAXControls(ctx, orgID, frameworkID, protectionLevel)
	if err != nil {
		return nil, "", fmt.Errorf("list tisax controls: %w", err)
	}

	gaps, err := s.GetTISAXGapAnalysis(ctx, orgID, frameworkID)
	if err != nil {
		return nil, "", fmt.Errorf("get tisax gap analysis: %w", err)
	}

	var orgName string
	orgName = fetchOrgName(ctx, s.db, orgID)
	if orgName == "" {
		orgName = orgID
	}

	assessmentDate := time.Now().UTC()
	pdfBytes, err := GenerateTISAXReportPDF(report, controls, gaps, orgName, protectionLevel, assessmentLevel, assessmentDate)
	if err != nil {
		return nil, "", fmt.Errorf("generate tisax pdf: %w", err)
	}

	filename := "tisax-bereitschaftsbericht-" + assessmentDate.Format("2006-01-02") + ".pdf"
	return pdfBytes, filename, nil
}

// --- DORA Dashboard (Story 27.5) ---

// computeNextDeadline returns the nearest unreported DORA deadline across a list of incidents.
// A deadline qualifies if: it is non-nil, non-zero, in the future (after now), and has not been reported.
func computeNextDeadline(incidents []Incident, now time.Time) *NextDeadline {
	type candidate struct {
		incidentID   string
		title        string
		deadlineType string
		deadlineAt   time.Time
	}

	var best *candidate
	for _, inc := range incidents {
		type dlPair struct {
			deadline   *time.Time
			reportedAt *time.Time
			label      string
		}
		pairs := []dlPair{
			{inc.Deadline4h, inc.Reported4hAt, "4h"},
			{inc.Deadline24h, inc.Reported24hAt, "24h"},
			{inc.Deadline72h, inc.Reported72hAt, "72h"},
			{inc.Deadline30d, inc.Reported30dAt, "30d"},
		}
		for _, p := range pairs {
			if p.deadline == nil || p.deadline.IsZero() {
				continue
			}
			if !p.deadline.After(now) {
				continue
			}
			if p.reportedAt != nil {
				continue
			}
			// This is a valid future, unreported deadline.
			if best == nil || p.deadline.Before(best.deadlineAt) {
				best = &candidate{
					incidentID:   inc.ID,
					title:        inc.Title,
					deadlineType: p.label,
					deadlineAt:   *p.deadline,
				}
			}
		}
	}

	if best == nil {
		return nil
	}
	return &NextDeadline{
		IncidentID:   best.incidentID,
		Title:        best.title,
		DeadlineType: best.deadlineType,
		DeadlineAt:   best.deadlineAt,
	}
}

// GetDORADashboard assembles the DORA readiness dashboard for the given organisation.
// Returns ErrDORANotEnabled if DORA framework is not enabled for the org.
func (s *Service) GetDORADashboard(ctx context.Context, orgID string) (*DORADashboard, error) {
	// 1. Look up DORA framework for this org.
	framework, err := s.repo.FindFrameworkByName(ctx, orgID, "DORA")
	if err != nil {
		return nil, fmt.Errorf("find DORA framework: %w", err)
	}
	if framework == nil {
		return nil, ErrDORANotEnabled
	}

	// 2. Readiness score.
	report, err := s.GetReadinessReport(ctx, orgID, framework.ID)
	if err != nil {
		return nil, fmt.Errorf("get readiness report: %w", err)
	}

	// 3. Open critical controls (Weight >= 3, status not "covered").
	controls, err := s.ListControls(ctx, orgID, framework.ID)
	if err != nil {
		return nil, fmt.Errorf("list controls: %w", err)
	}
	openCritical := 0
	for _, c := range controls {
		if c.Weight >= 3 && c.Status != "covered" && c.Status != "not_applicable" {
			openCritical++
		}
	}

	// 4. Next deadline from DORA incidents.
	incidents, err := s.repo.ListIncidentsByType(ctx, orgID, "dora")
	if err != nil {
		return nil, fmt.Errorf("list dora incidents: %w", err)
	}
	nextDeadline := computeNextDeadline(incidents, time.Now().UTC())

	// 5. Expired suppliers.
	suppliers, err := s.ListSuppliers(ctx, orgID, nil)
	if err != nil {
		return nil, fmt.Errorf("list suppliers: %w", err)
	}
	expiredSuppliers := 0
	for _, sup := range suppliers {
		if sup.ContractStatus == "expired" {
			expiredSuppliers++
		}
	}

	// 6+7. TLPT overdue warning + recent tests for PDF (S40-1).
	allTests, tlptOverdue, err := s.ListResilienceTests(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("list resilience tests: %w", err)
	}
	recentTests := allTests
	if len(recentTests) > 3 {
		recentTests = recentTests[:3]
	}

	// 8. IKT-Drittanbieter counts (S38-1/2/3).
	thirdParties, err := s.repo.ListDORAThirdParties(ctx, orgID, "")
	if err != nil {
		return nil, fmt.Errorf("list dora third parties: %w", err)
	}
	criticalTP, missingExit := 0, 0
	for _, tp := range thirdParties {
		if tp.Criticality == "kritisch" {
			criticalTP++
			if !tp.ExitStrategy {
				missingExit++
			}
		}
	}

	return &DORADashboard{
		ReadinessPct:          report.ReadinessScore,
		OpenCriticalControls:  openCritical,
		NextDeadline:          nextDeadline,
		ExpiredSuppliers:      expiredSuppliers,
		TLPTOverdueWarning:    tlptOverdue,
		ThirdPartyCount:       len(thirdParties),
		CriticalThirdParties:  criticalTP,
		MissingExitStrategies: missingExit,
		RecentResilienceTests: recentTests,
	}, nil
}

// ExportDORAPDF generates the DORA readiness PDF for the given organisation.
func (s *Service) ExportDORAPDF(ctx context.Context, orgID string) ([]byte, error) {
	dashboard, err := s.GetDORADashboard(ctx, orgID)
	if err != nil {
		return nil, err
	}
	var orgName string
	orgName = fetchOrgName(ctx, s.db, orgID)
	if orgName == "" {
		orgName = orgID
	}
	return GenerateDORAPDF(dashboard, orgName)
}

// dsgvoToISOMappings maps each DSGVO Art. 32 TOM control ID to its primary ISO 27001 control ID.
var dsgvoToISOMappings = map[string]string{
	"TOM-1":  "A.9.1.2",  // Zutrittskontrolle → Netzwerkzugänge
	"TOM-2":  "A.9.4.2",  // Zugangskontrolle → MFA/Anmeldeverfahren
	"TOM-3":  "A.9.2.2",  // Zugriffskontrolle → Zugangsprovisionierung
	"TOM-4":  "A.14.1.2", // Weitergabekontrolle → Absicherung öffentlicher Dienste
	"TOM-5":  "A.12.1.1", // Eingabekontrolle → Betriebsverfahren/Protokollierung
	"TOM-6":  "A.18.1.1", // Auftragskontrolle → Compliance-Anforderungen
	"TOM-7":  "A.12.3.1", // Verfügbarkeitskontrolle → Datensicherung
	"TOM-8":  "A.6.1.2",  // Trennungsgebot → Aufgabentrennung
	"TOM-9":  "A.10.1.1", // Pseudonymisierung → Kryptographierichtlinie
	"TOM-10": "A.10.1.2", // Verschlüsselung → Schlüsselverwaltung
	"TOM-11": "A.12.1.2", // Integrität → Änderungsmanagement
	"TOM-12": "A.17.1.2", // Wiederherstellung → BCM-Implementierung
	"TOM-13": "A.18.1.1", // Überprüfungsverfahren → Compliance-Register
}

// SeedDSGVOMappings idempotently seeds DSGVO-TOM → ISO 27001 mappings.
// Returns nil if either framework is not yet enabled.
func (s *Service) SeedDSGVOMappings(ctx context.Context, orgID string) error {
	dsgvoFW, err := s.repo.FindFrameworkByName(ctx, orgID, "DSGVO-TOM")
	if err != nil {
		return fmt.Errorf("find DSGVO-TOM framework: %w", err)
	}
	if dsgvoFW == nil {
		return nil
	}

	isoFW, err := s.repo.FindFrameworkByName(ctx, orgID, "ISO27001")
	if err != nil {
		return fmt.Errorf("find ISO27001 framework: %w", err)
	}
	if isoFW == nil {
		return nil
	}

	dsgvoControls, err := s.repo.ListControls(ctx, orgID, dsgvoFW.ID)
	if err != nil {
		return fmt.Errorf("list DSGVO-TOM controls: %w", err)
	}
	isoControls, err := s.repo.ListControls(ctx, orgID, isoFW.ID)
	if err != nil {
		return fmt.Errorf("list ISO27001 controls: %w", err)
	}

	dsgvoByID := make(map[string]string, len(dsgvoControls))
	for _, c := range dsgvoControls {
		dsgvoByID[c.ControlID] = c.ID
	}
	isoByID := make(map[string]string, len(isoControls))
	for _, c := range isoControls {
		isoByID[c.ControlID] = c.ID
	}

	for tomID, isoID := range dsgvoToISOMappings {
		tomUUID, ok1 := dsgvoByID[tomID]
		isoUUID, ok2 := isoByID[isoID]
		if !ok1 || !ok2 {
			continue
		}
		if _, err := s.repo.CreateMapping(ctx, orgID, tomUUID, isoUUID); err != nil {
			log.Warn().Err(err).Str("tom", tomID).Str("iso", isoID).Msg("seed DSGVO mapping failed")
		}
	}
	return nil
}

// GetDSGVOTOMCoverage returns coverage status for each TOM based on mapped ISO 27001 controls.
func (s *Service) GetDSGVOTOMCoverage(ctx context.Context, orgID, dsgvoFrameworkID string) ([]MappingResult, error) {
	tomControls, err := s.ListControls(ctx, orgID, dsgvoFrameworkID)
	if err != nil {
		return nil, fmt.Errorf("list DSGVO-TOM controls: %w", err)
	}

	isoFW, err := s.repo.FindFrameworkByName(ctx, orgID, "ISO27001")
	if err != nil {
		return nil, fmt.Errorf("find ISO27001 framework: %w", err)
	}

	var isoControls []Control
	var evidenceCounts map[string]int
	if isoFW != nil {
		isoControls, err = s.ListControls(ctx, orgID, isoFW.ID)
		if err != nil {
			return nil, fmt.Errorf("list ISO27001 controls: %w", err)
		}
		evidenceCounts, err = s.repo.CountEvidenceByControl(ctx, orgID, isoFW.ID)
		if err != nil {
			return nil, fmt.Errorf("count ISO27001 evidence: %w", err)
		}
	}

	isoByControlID := make(map[string]Control, len(isoControls))
	for _, c := range isoControls {
		isoByControlID[c.ControlID] = c
	}
	if evidenceCounts == nil {
		evidenceCounts = map[string]int{}
	}

	results := make([]MappingResult, 0, len(tomControls))
	for _, tom := range tomControls {
		isoControlID, hasMapped := dsgvoToISOMappings[tom.ControlID]
		isoControl, hasISO := isoByControlID[isoControlID]

		covered := false
		if hasMapped && hasISO {
			evCount := evidenceCounts[isoControl.ID]
			covered = isoControl.Status == "covered" || isoControl.Status == "implemented" || evCount > 0
		}

		isoTitle := isoControlID
		if c, ok := isoByControlID[isoControlID]; ok {
			isoTitle = c.Title
		}

		results = append(results, MappingResult{
			TISAXControlID:    tom.ID,
			TISAXControlTitle: tom.Title,
			ISOControlID:      isoControlID,
			ISOControlTitle:   isoTitle,
			Covered:           covered,
		})
	}
	return results, nil
}

// --- Score History ---

// RecordScoreSnapshotForAllOrgs iterates all non-deleted organisations and captures
// the current compliance score (org-wide + per-framework) into ck_score_history.
// Called daily by the Asynq scheduler.
func (s *Service) RecordScoreSnapshotForAllOrgs(ctx context.Context) error {
	orgIDs, err := s.repo.ListActiveOrgIDs(ctx)
	if err != nil {
		return fmt.Errorf("score_snapshot: list orgs: %w", err)
	}

	for _, orgID := range orgIDs {
		if err := s.recordOrgScoreSnapshot(ctx, orgID); err != nil {
			log.Error().Err(err).Str("org_id", orgID).Msg("score_snapshot: failed for org")
			// Continue with next org — don't abort the whole run.
		}
	}
	return nil
}

// recordOrgScoreSnapshot captures one org-wide + per-framework score row.
func (s *Service) recordOrgScoreSnapshot(ctx context.Context, orgID string) error {
	frameworks, err := s.repo.ListFrameworks(ctx, orgID)
	if err != nil {
		return fmt.Errorf("list frameworks: %w", err)
	}

	var totalAll, implementedAll int

	for _, fw := range frameworks {
		controls, err := s.repo.ListControls(ctx, orgID, fw.ID)
		if err != nil {
			log.Warn().Err(err).Str("framework_id", fw.ID).Msg("score_snapshot: list controls failed")
			continue
		}
		evidenceCounts, err := s.repo.CountEvidenceByControl(ctx, orgID, fw.ID)
		if err != nil {
			log.Warn().Err(err).Str("framework_id", fw.ID).Msg("score_snapshot: count evidence failed")
			continue
		}

		report := computeReadinessReport(&fw, controls, evidenceCounts)
		totalAll += report.TotalControls
		implementedAll += report.Covered

		// Per-framework snapshot.
		fwID := fw.ID
		if insertErr := s.repo.InsertScoreSnapshot(ctx, orgID, &fwID, report.ReadinessScore, report.TotalControls, report.Covered); insertErr != nil {
			log.Warn().Err(insertErr).Str("framework_id", fw.ID).Msg("score_snapshot: insert per-framework failed")
		}
	}

	// Org-wide snapshot (framework_id = NULL).
	var orgScore float64
	if totalAll > 0 {
		orgScore = float64(implementedAll) / float64(totalAll) * 100
	}
	if insertErr := s.repo.InsertScoreSnapshot(ctx, orgID, nil, orgScore, totalAll, implementedAll); insertErr != nil {
		return fmt.Errorf("insert org-wide snapshot: %w", insertErr)
	}
	return nil
}

// GetScoreHistory returns daily score history for an organisation (org-wide snapshots).
func (s *Service) GetScoreHistory(ctx context.Context, orgID string, days int) ([]ScoreHistoryEntry, error) {
	if days <= 0 || days > 365 {
		days = 30
	}
	return s.repo.GetScoreHistory(ctx, orgID, days)
}

// ExecutiveSummaryData holds all data gathered for the Executive Summary PDF.
type ExecutiveSummaryData struct {
	OrgName     string
	GeneratedAt time.Time
	// Section 1 — Overall compliance score
	OverallScore float64 // 0–100, weighted average across all frameworks
	// Section 2 — Framework overview
	Frameworks []ExecutiveFrameworkRow
	// Section 3 — Top 5 open risks (by score)
	TopRisks []ExecutiveRiskRow
	// Section 4 — Last 30 days activity
	Last30DaysActivity ExecutiveActivity
}

// ExecutiveFrameworkRow is one row in the framework table.
type ExecutiveFrameworkRow struct {
	Name        string
	Score       float64
	Implemented int
	Total       int
}

// ExecutiveRiskRow is one of the top-5 open risks.
type ExecutiveRiskRow struct {
	Title    string
	Score    int
	Severity string // "critical" | "high" | "medium" | "low"
}

// ExecutiveActivity holds counts of key activities in the last 30 days.
type ExecutiveActivity struct {
	ClosedControls   int
	NewIncidents     int
	ResolvedFindings int
}

// GetExecutiveSummaryData collects data required for the Executive Summary PDF.
func (s *Service) GetExecutiveSummaryData(ctx context.Context, orgID string) (*ExecutiveSummaryData, error) {
	d := &ExecutiveSummaryData{GeneratedAt: time.Now().UTC()}

	// Org name (soft-fail)
	d.OrgName = fetchOrgName(ctx, s.db, orgID)
	if d.OrgName == "" {
		d.OrgName = orgID
	}

	// Framework scores
	fwScores, err := s.repo.GetExecutiveFrameworkScores(ctx, orgID)
	if err != nil {
		log.Warn().Err(err).Msg("executive summary: frameworks query")
	} else {
		var totalWeight, weightedSum float64
		for _, row := range fwScores {
			r := ExecutiveFrameworkRow{
				Name:        row.Name,
				Total:       row.Total,
				Implemented: row.Implemented,
			}
			if r.Total > 0 {
				r.Score = float64(r.Implemented) / float64(r.Total) * 100
			}
			d.Frameworks = append(d.Frameworks, r)
			weightedSum += r.Score * float64(r.Total)
			totalWeight += float64(r.Total)
		}
		if totalWeight > 0 {
			d.OverallScore = weightedSum / totalWeight
		}
	}

	// Top 5 risks by score (likelihood * impact)
	topRisks, err := s.repo.GetExecutiveTopRisks(ctx, orgID)
	if err != nil {
		log.Warn().Err(err).Msg("executive summary: risks query")
	} else {
		for _, row := range topRisks {
			d.TopRisks = append(d.TopRisks, ExecutiveRiskRow(row))
		}
	}

	// Last 30 days activity — soft-fail: einzelne Counter-Abfragen sind nicht
	// kritisch fuer die Executive-Summary, aber Fehler MUESSEN sichtbar sein
	// (S13-18). Bei Fehler bleibt der Counter 0 und wir loggen die Ursache.
	since := time.Now().UTC().Add(-30 * 24 * time.Hour)
	if n, err := s.repo.CountClosedControlsSince(ctx, orgID, since); err != nil {
		log.Warn().Err(err).Str("org_id", orgID).Msg("executive-summary: ClosedControls counter failed")
	} else {
		d.Last30DaysActivity.ClosedControls = n
	}

	if n, err := s.repo.CountIncidentsSince(ctx, orgID, since); err != nil {
		log.Warn().Err(err).Str("org_id", orgID).Msg("executive-summary: NewIncidents counter failed")
	} else {
		d.Last30DaysActivity.NewIncidents = n
	}

	if n, err := s.repo.CountResolvedFindingsSince(ctx, orgID, since); err != nil {
		log.Warn().Err(err).Str("org_id", orgID).Msg("executive-summary: ResolvedFindings counter failed")
	} else {
		d.Last30DaysActivity.ResolvedFindings = n
	}

	return d, nil
}

// ExportExecutiveSummaryPDF generates the Executive Summary PDF bytes.
func (s *Service) ExportExecutiveSummaryPDF(ctx context.Context, orgID string) ([]byte, string, error) {
	data, err := s.GetExecutiveSummaryData(ctx, orgID)
	if err != nil {
		return nil, "", fmt.Errorf("gather executive summary data: %w", err)
	}
	pdfBytes, err := GenerateExecutiveSummaryPDF(data)
	if err != nil {
		return nil, "", fmt.Errorf("generate executive summary pdf: %w", err)
	}
	filename := fmt.Sprintf("executive-summary-%s.pdf", data.GeneratedAt.Format("2006-01-02"))
	return pdfBytes, filename, nil
}
