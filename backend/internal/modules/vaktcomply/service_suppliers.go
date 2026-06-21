// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package vaktcomply

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/matharnica/vakt/internal/modules/vaktcomply/policy"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/matharnica/vakt/internal/shared/notify"
)

// --- Supplier Register ---

// computeContractStatus returns "expired", "expiring_soon", or "active" based on contractEnd.
func computeContractStatus(contractEnd *time.Time, now time.Time) string {
	if contractEnd == nil {
		return "active"
	}
	if contractEnd.Before(now) {
		return "expired"
	}
	if contractEnd.Before(now.Add(30 * 24 * time.Hour)) {
		return "expiring_soon"
	}
	return "active"
}

func (s *Service) ListSuppliers(ctx context.Context, orgID string, filter *SupplierFilter) ([]Supplier, error) {
	suppliers, err := s.repo.ListSuppliers(ctx, orgID, filter)
	if err != nil {
		return nil, err
	}
	if suppliers == nil {
		suppliers = []Supplier{}
	}
	now := time.Now().UTC()
	for i := range suppliers {
		suppliers[i].ContractStatus = computeContractStatus(suppliers[i].ContractEnd, now)
	}
	return suppliers, nil
}

func (s *Service) GetSupplier(ctx context.Context, orgID, id string) (*Supplier, error) {
	supplier, err := s.repo.GetSupplier(ctx, orgID, id)
	if err != nil {
		return nil, err
	}
	supplier.ContractStatus = computeContractStatus(supplier.ContractEnd, time.Now().UTC())
	return supplier, nil
}

func (s *Service) CreateSupplier(ctx context.Context, orgID string, in CreateSupplierInput) (*Supplier, error) {
	supplier, err := s.repo.CreateSupplier(ctx, orgID, in)
	if err != nil {
		return nil, err
	}
	supplier.ContractStatus = computeContractStatus(supplier.ContractEnd, time.Now().UTC())
	return supplier, nil
}

func (s *Service) UpdateSupplier(ctx context.Context, orgID, id string, in UpdateSupplierInput) (*Supplier, error) {
	supplier, err := s.repo.UpdateSupplier(ctx, orgID, id, in)
	if err != nil {
		return nil, err
	}
	supplier.ContractStatus = computeContractStatus(supplier.ContractEnd, time.Now().UTC())
	return supplier, nil
}

func (s *Service) DeleteSupplier(ctx context.Context, orgID, id string) error {
	return s.repo.DeleteSupplier(ctx, orgID, id)
}

// ListIncidentsBySupplier returns all incidents linked to a given supplier.
func (s *Service) ListIncidentsBySupplier(ctx context.Context, orgID, supplierID string) ([]Incident, error) {
	incidents, err := s.repo.ListIncidentsBySupplier(ctx, orgID, supplierID)
	if err != nil {
		return nil, err
	}
	if incidents == nil {
		incidents = []Incident{}
	}
	return incidents, nil
}

// LinkSupplierRisk links a risk to a supplier.
func (s *Service) LinkSupplierRisk(ctx context.Context, orgID, supplierID, riskID string) error {
	return s.repo.LinkSupplierRisk(ctx, orgID, supplierID, riskID)
}

// UnlinkSupplierRisk removes a risk link from a supplier.
func (s *Service) UnlinkSupplierRisk(ctx context.Context, orgID, supplierID, riskID string) error {
	return s.repo.UnlinkSupplierRisk(ctx, orgID, supplierID, riskID)
}

// ListSupplierRisks returns all risks linked to the given supplier.
func (s *Service) ListSupplierRisks(ctx context.Context, orgID, supplierID string) ([]Risk, error) {
	risks, err := s.repo.ListSupplierRisks(ctx, orgID, supplierID)
	if err != nil {
		return nil, err
	}
	if risks == nil {
		risks = []Risk{}
	}
	return risks, nil
}

// supplierCSVRow holds a parsed (but not yet saved) supplier row from a CSV import.
type supplierCSVRow struct {
	Name         string
	ContactName  string
	ContactEmail string
	ServiceType  string
	Criticality  string
	NIS2Relevant bool
	DORARelevant bool
}

// parseSupplierCSVRows parses a CSV string and returns valid rows.
// Rows with missing name or invalid criticality are silently skipped (for test use).
func parseSupplierCSVRows(content string) ([]supplierCSVRow, error) {
	reader := csv.NewReader(strings.NewReader(content))
	reader.TrimLeadingSpace = true

	header, err := reader.Read()
	if err != nil {
		return nil, fmt.Errorf("read csv header: %w", err)
	}
	colIdx := make(map[string]int, len(header))
	for i, col := range header {
		colIdx[strings.TrimSpace(col)] = i
	}

	validCriticalities := map[string]bool{"low": true, "medium": true, "high": true, "critical": true, "standard": true, "important": true}

	parseBool := func(v string) bool {
		return strings.EqualFold(v, "true") || v == "1"
	}

	getCol := func(record []string, name string) string {
		idx, ok := colIdx[name]
		if !ok || idx >= len(record) {
			return ""
		}
		return strings.TrimSpace(record[idx])
	}

	rows := []supplierCSVRow{}
	for {
		record, err := reader.Read()
		if err != nil {
			if err == io.EOF {
				break
			}
			continue
		}
		name := getCol(record, "name")
		if name == "" {
			continue
		}
		crit := getCol(record, "criticality")
		if crit != "" && !validCriticalities[crit] {
			continue
		}
		rows = append(rows, supplierCSVRow{
			Name:         name,
			ContactName:  getCol(record, "contact_name"),
			ContactEmail: getCol(record, "contact_email"),
			ServiceType:  getCol(record, "service_type"),
			Criticality:  crit,
			NIS2Relevant: parseBool(getCol(record, "nis2_relevant")),
			DORARelevant: parseBool(getCol(record, "dora_relevant")),
		})
	}
	return rows, nil
}

// ParseAndImportSupplierCSV reads a CSV stream and imports valid rows as suppliers.
// Expected header: name,contact_name,contact_email,service_type,criticality,nis2_relevant,dora_relevant
func (s *Service) ParseAndImportSupplierCSV(ctx context.Context, orgID string, r io.Reader) (*CSVImportResult, error) {
	reader := csv.NewReader(r)
	reader.TrimLeadingSpace = true

	header, err := reader.Read()
	if err != nil {
		return nil, fmt.Errorf("read csv header: %w", err)
	}

	// Build column index map.
	colIdx := make(map[string]int, len(header))
	for i, col := range header {
		colIdx[strings.TrimSpace(col)] = i
	}

	validCriticalities := map[string]bool{"low": true, "medium": true, "high": true, "critical": true, "standard": true, "important": true}

	result := &CSVImportResult{
		Errors: []CSVImportError{},
	}

	rowNum := 1 // header is row 0
	for {
		rowNum++
		record, err := reader.Read()
		if err != nil {
			if err == io.EOF {
				break
			}
			result.Skipped++
			result.Errors = append(result.Errors, CSVImportError{Row: rowNum, Message: fmt.Sprintf("read error: %s", err.Error())})
			continue
		}

		getCol := func(name string) string {
			idx, ok := colIdx[name]
			if !ok || idx >= len(record) {
				return ""
			}
			return strings.TrimSpace(record[idx])
		}

		name := getCol("name")
		if name == "" {
			result.Skipped++
			result.Errors = append(result.Errors, CSVImportError{Row: rowNum, Message: "required field 'name' is empty"})
			continue
		}

		criticality := getCol("criticality")
		if criticality != "" && !validCriticalities[criticality] {
			result.Skipped++
			result.Errors = append(result.Errors, CSVImportError{Row: rowNum, Message: fmt.Sprintf("invalid criticality %q: must be one of standard, important, critical", criticality)})
			continue
		}

		parseBool := func(v string) bool {
			return strings.EqualFold(v, "true") || v == "1"
		}

		in := CreateSupplierInput{
			Name:         name,
			ContactName:  getCol("contact_name"),
			ContactEmail: getCol("contact_email"),
			ServiceType:  getCol("service_type"),
			Criticality:  criticality,
			NIS2Relevant: parseBool(getCol("nis2_relevant")),
			DORARelevant: parseBool(getCol("dora_relevant")),
		}

		if _, err := s.CreateSupplier(ctx, orgID, in); err != nil {
			result.Skipped++
			result.Errors = append(result.Errors, CSVImportError{Row: rowNum, Message: fmt.Sprintf("create failed: %s", err.Error())})
			continue
		}
		result.Imported++
	}

	return result, nil
}

// GenerateSupplierCSV generates a CSV export of suppliers.
// sub_suppliers are encoded as semicolon-separated values in one cell.
func GenerateSupplierCSV(suppliers []Supplier) ([]byte, error) {
	var buf bytes.Buffer
	w := csv.NewWriter(&buf)

	header := []string{
		"id", "name", "contact_name", "contact_email", "service_type",
		"criticality", "dora_relevant", "nis2_relevant",
		"contract_end", "contract_status", "data_location",
		"exit_strategy_exists", "sub_suppliers", "notes",
		"assessment_status", "last_assessment_at",
	}
	if err := w.Write(header); err != nil {
		return nil, fmt.Errorf("csv write header: %w", err)
	}

	for _, s := range suppliers {
		contractEnd := ""
		if s.ContractEnd != nil {
			contractEnd = s.ContractEnd.Format(time.RFC3339)
		}
		lastAssessmentAt := ""
		if s.LastAssessmentAt != nil {
			lastAssessmentAt = s.LastAssessmentAt.Format(time.RFC3339)
		}
		subSuppliers := strings.Join(s.SubSuppliers, ";")
		row := []string{
			s.ID,
			s.Name,
			s.ContactName,
			s.ContactEmail,
			s.ServiceType,
			s.Criticality,
			strconv.FormatBool(s.DORARelevant),
			strconv.FormatBool(s.NIS2Relevant),
			contractEnd,
			s.ContractStatus,
			s.DataLocation,
			strconv.FormatBool(s.ExitStrategyExists),
			subSuppliers,
			s.Notes,
			s.AssessmentStatus,
			lastAssessmentAt,
		}
		if err := w.Write(row); err != nil {
			return nil, fmt.Errorf("csv write row: %w", err)
		}
	}

	w.Flush()
	if err := w.Error(); err != nil {
		return nil, fmt.Errorf("csv flush: %w", err)
	}
	return buf.Bytes(), nil
}

// --- Supplier Portal Assessments (Story 29.3) ---

// ErrAssessmentExpiredOrSubmitted is returned when a token references an expired or already-submitted assessment.
var ErrAssessmentExpiredOrSubmitted = errors.New("assessment_expired_or_submitted")

// hashToken computes the SHA-256 hex hash of a raw token. Exported for testing.
func hashToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

// CreateAssessment generates a token, inserts a supplier assessment, sends an invite email,
// and returns the assessment with share URL and the raw token.
func (s *Service) CreateAssessment(ctx context.Context, orgID, supplierID string, in CreateAssessmentInput, baseURL string) (*AssessmentWithQuestionnaire, string, error) {
	// Validate org owns supplier.
	supplier, err := s.repo.GetSupplier(ctx, orgID, supplierID)
	if err != nil {
		return nil, "", fmt.Errorf("supplier not found: %w", err)
	}

	// Validate questionnaire belongs to org.
	qnr, err := s.repo.GetQuestionnaire(ctx, orgID, in.QuestionnaireID)
	if err != nil {
		return nil, "", fmt.Errorf("questionnaire not found: %w", err)
	}

	rawToken, tokenHash, err := policy.GenerateToken()
	if err != nil {
		return nil, "", fmt.Errorf("generate assessment token: %w", err)
	}

	expiresAt := time.Now().UTC().Add(time.Duration(in.ExpiresInDays) * 24 * time.Hour)

	a := Assessment{
		OrgID:           orgID,
		SupplierID:      supplierID,
		QuestionnaireID: in.QuestionnaireID,
		TokenHash:       tokenHash,
		ExpiresAt:       expiresAt,
		Status:          "pending",
	}
	if err := s.repo.CreateAssessment(ctx, a); err != nil {
		return nil, "", fmt.Errorf("create assessment: %w", err)
	}

	// Fetch the newly created assessment by token hash.
	created, err := s.repo.GetAssessmentByTokenHash(ctx, tokenHash)
	if err != nil {
		return nil, "", fmt.Errorf("fetch created assessment: %w", err)
	}

	shareURL := baseURL + "/supplier/" + rawToken

	// Send invite email to supplier contact via SMTP (non-fatal).
	if supplier.ContactEmail != "" && s.notifSvc != nil {
		body := strings.ReplaceAll(EmailSupplierInviteBodyDE, "{{.ShareURL}}", shareURL)
		body = strings.ReplaceAll(body, "{{.ExpiresAt}}", expiresAt.Format("02.01.2006"))
		if err := s.notifSvc.Notify(ctx, notify.Message{
			Title:   EmailSupplierInviteSubjectDE,
			Body:    body,
			OrgID:   orgID,
			Channel: notify.ChannelEmail,
			Target:  supplier.ContactEmail,
		}); err != nil {
			log.Warn().Err(err).Str("supplier_id", supplierID).Msg("send assessment invite email")
		}
	}

	result := &AssessmentWithQuestionnaire{
		Assessment:    *created,
		Questionnaire: qnr,
		ShareURL:      shareURL,
	}
	return result, rawToken, nil
}

// GetAssessmentForPortal looks up an assessment by raw token, transitions pending→in_progress,
// and returns the assessment with questionnaire+questions.
func (s *Service) GetAssessmentForPortal(ctx context.Context, rawToken string) (*AssessmentWithQuestionnaire, error) {
	hash := hashToken(rawToken)
	a, err := s.repo.GetAssessmentByTokenHash(ctx, hash)
	if err != nil {
		return nil, ErrAssessmentExpiredOrSubmitted
	}
	if time.Now().UTC().After(a.ExpiresAt) || a.Status == "submitted" || a.Status == "reviewed" {
		return nil, ErrAssessmentExpiredOrSubmitted
	}

	// Transition pending → in_progress.
	if a.Status == "pending" {
		if err := s.repo.UpdateAssessmentStatus(ctx, a.ID, "in_progress", nil, "", ""); err != nil {
			log.Warn().Err(err).Str("assessment_id", a.ID).Msg("failed to transition assessment to in_progress")
		} else {
			a.Status = "in_progress"
		}
	}

	result, err := s.repo.GetAssessmentWithQuestionnaire(ctx, a.ID)
	if err != nil {
		return nil, fmt.Errorf("get assessment with questionnaire: %w", err)
	}
	return result, nil
}

// SaveAnswers upserts answers for an in-progress assessment (intermediate save).
func (s *Service) SaveAnswers(ctx context.Context, rawToken string, in SaveAnswersInput) error {
	hash := hashToken(rawToken)
	a, err := s.repo.GetAssessmentByTokenHash(ctx, hash)
	if err != nil {
		return ErrAssessmentExpiredOrSubmitted
	}
	if time.Now().UTC().After(a.ExpiresAt) || a.Status == "submitted" || a.Status == "reviewed" {
		return ErrAssessmentExpiredOrSubmitted
	}
	return s.repo.UpsertAnswers(ctx, a.ID, in.Answers)
}

// SubmitAssessment upserts final answers, marks the assessment as submitted,
// and sends confirmation emails.
func (s *Service) SubmitAssessment(ctx context.Context, rawToken, clientIP, userAgent string, in SaveAnswersInput) error {
	hash := hashToken(rawToken)
	a, err := s.repo.GetAssessmentByTokenHash(ctx, hash)
	if err != nil {
		return ErrAssessmentExpiredOrSubmitted
	}
	if time.Now().UTC().After(a.ExpiresAt) || a.Status == "submitted" || a.Status == "reviewed" {
		return ErrAssessmentExpiredOrSubmitted
	}

	if err := s.repo.UpsertAnswers(ctx, a.ID, in.Answers); err != nil {
		return fmt.Errorf("upsert answers on submit: %w", err)
	}

	now := time.Now().UTC()
	if err := s.repo.UpdateAssessmentStatus(ctx, a.ID, "submitted", &now, clientIP, userAgent); err != nil {
		if errors.Is(err, ErrAssessmentExpiredOrSubmitted) {
			return ErrAssessmentExpiredOrSubmitted
		}
		return fmt.Errorf("update assessment status: %w", err)
	}

	// Update supplier assessment_status to completed.
	_ = s.repo.UpdateSupplierAssessmentStatus(ctx, a.OrgID, a.SupplierID, "completed", &now)

	// Send confirmation email to supplier contact (non-fatal) + in-app internal notification.
	if s.notifSvc != nil {
		if supplier, sErr := s.repo.GetSupplier(ctx, a.OrgID, a.SupplierID); sErr == nil && supplier.ContactEmail != "" {
			if err := s.notifSvc.Notify(ctx, notify.Message{
				Title:   EmailSupplierConfirmSubjectDE,
				Body:    EmailSupplierConfirmBodyDE,
				OrgID:   a.OrgID,
				Channel: notify.ChannelEmail,
				Target:  supplier.ContactEmail,
			}); err != nil {
				log.Warn().Err(err).Str("assessment_id", a.ID).Msg("send assessment confirmation email")
			}
		}
	}

	internalBody := strings.ReplaceAll(EmailComplianceNotifyBodyDE, "{{.AssessmentID}}", a.ID)
	internalBody = strings.ReplaceAll(internalBody, "{{.SupplierID}}", a.SupplierID)
	notify.Send(ctx, s.db, a.OrgID, EmailComplianceNotifySubjectDE, internalBody, "supplier_assessment_submitted", "vaktcomply")

	return nil
}

// ListAssessmentsForSupplier returns all assessments for a given supplier.
func (s *Service) ListAssessmentsForSupplier(ctx context.Context, orgID, supplierID string) ([]Assessment, error) {
	return s.repo.ListAssessmentsForSupplier(ctx, orgID, supplierID)
}

// GetAssessment returns a single assessment by ID (with questionnaire).
func (s *Service) GetAssessment(ctx context.Context, orgID, id string) (*AssessmentWithQuestionnaire, error) {
	a, err := s.repo.GetAssessmentWithQuestionnaire(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("assessment not found: %w", err)
	}
	if a.OrgID != orgID {
		return nil, fmt.Errorf("assessment not found")
	}
	return a, nil
}

// --- Assessment Review (Story 29.4) ---

// computeStatus is a pure function (no DB) — fully unit-testable.
// Returns green/yellow/red based on assessment state and supplier metadata.
func computeStatus(supplier Supplier, assessments []Assessment, answers []AnswerWithReview, now time.Time) SupplierStatus {
	st := SupplierStatus{
		SupplierID: supplier.ID,
		Details:    map[string]any{},
	}

	// No assessment ever: red for critical, yellow otherwise
	if len(assessments) == 0 {
		if supplier.Criticality == "critical" {
			st.Status = "red"
			st.Score = 0
			st.Details["reason"] = "no_assessment_critical"
		} else {
			st.Status = "yellow"
			st.Score = 25
			st.Details["reason"] = "no_assessment"
		}
		return st
	}

	latest := assessments[0]

	// Pending or in-progress assessment → yellow
	if latest.Status == "pending" || latest.Status == "in_progress" {
		st.Status = "yellow"
		st.Score = 50
		st.Details["reason"] = "assessment_pending"
		st.Details["assessment_status"] = latest.Status
		return st
	}

	// Contract ending within 90 days → at most yellow
	contractWarning := false
	if supplier.ContractEnd != nil {
		daysLeft := supplier.ContractEnd.Sub(now).Hours() / 24
		if daysLeft >= 0 && daysLeft < 90 {
			contractWarning = true
			st.Details["contract_days_left"] = int(daysLeft)
		}
	}

	// Check review results when assessment is reviewed
	if latest.Status == "reviewed" && len(answers) > 0 {
		total := len(answers)
		accepted := 0
		rework := 0
		for _, a := range answers {
			if a.ReviewStatus != nil {
				switch *a.ReviewStatus {
				case "accepted":
					accepted++
				case "needs_rework":
					rework++
				}
			}
		}
		score := 0
		if total > 0 {
			score = (accepted * 100) / total
		}
		st.Score = score
		st.Details["total_answers"] = total
		st.Details["accepted"] = accepted
		st.Details["needs_rework"] = rework

		if rework > 0 {
			st.Status = "red"
			st.Details["reason"] = "needs_rework"
			return st
		}
		if contractWarning {
			st.Status = "yellow"
			st.Details["reason"] = "contract_expiring"
			return st
		}
		st.Status = "green"
		return st
	}

	// Submitted but not yet reviewed
	if latest.Status == "submitted" {
		st.Status = "yellow"
		st.Score = 60
		st.Details["reason"] = "awaiting_review"
		return st
	}

	// Fallback
	if contractWarning {
		st.Status = "yellow"
		st.Score = 40
		st.Details["reason"] = "contract_expiring"
		return st
	}
	st.Status = "yellow"
	st.Score = 30
	st.Details["reason"] = "incomplete"
	return st
}

// ComputeSupplierStatus fetches supplier + assessment data and delegates to computeStatus.
func (s *Service) ComputeSupplierStatus(ctx context.Context, orgID, supplierID string) (*SupplierStatus, error) {
	supplier, err := s.repo.GetSupplier(ctx, orgID, supplierID)
	if err != nil {
		return nil, fmt.Errorf("compute supplier status: get supplier: %w", err)
	}
	assessments, err := s.repo.GetAssessmentsForSupplier(ctx, orgID, supplierID)
	if err != nil {
		return nil, fmt.Errorf("compute supplier status: get assessments: %w", err)
	}
	var answers []AnswerWithReview
	if len(assessments) > 0 && assessments[0].Status == "reviewed" {
		answers, err = s.repo.GetAnswersForAssessment(ctx, orgID, assessments[0].ID)
		if err != nil {
			return nil, fmt.Errorf("compute supplier status: get answers: %w", err)
		}
	}
	result := computeStatus(*supplier, assessments, answers, time.Now().UTC())
	return &result, nil
}

// ReviewAnswer validates input, saves review status, and optionally creates evidence.
// Returns the created evidence ID (or nil if none was created).
func (s *Service) ReviewAnswer(ctx context.Context, orgID, assessmentID, answerID string, in ReviewAnswerInput) (*string, error) {
	if in.ReviewStatus != "accepted" && in.ReviewStatus != "needs_rework" {
		return nil, fmt.Errorf("review_status must be accepted or needs_rework")
	}
	if err := s.repo.UpdateAnswerReview(ctx, orgID, assessmentID, answerID, in.ReviewStatus, in.ReworkNote); err != nil {
		return nil, err
	}
	if in.ReviewStatus != "accepted" {
		return nil, nil
	}
	// Load answer+question to check for control_id
	aq, err := s.repo.GetAnswerWithQuestion(ctx, orgID, answerID)
	if err != nil || aq.ControlID == nil {
		return nil, nil
	}
	// Load supplier name for the evidence title
	a, err := s.repo.GetAssessmentWithQuestionnaire(ctx, assessmentID)
	if err != nil {
		return nil, nil
	}
	supplier, err := s.repo.GetSupplier(ctx, orgID, a.SupplierID)
	if err != nil {
		return nil, nil
	}
	title := "Lieferant " + supplier.Name + ": " + aq.QuestionText
	ev, err := s.repo.AddEvidence(ctx, orgID, *aq.ControlID, orgID, AddEvidenceInput{
		Title:  title,
		Source: "supplier_assessment",
	})
	if err != nil {
		log.Warn().Err(err).Str("answer_id", answerID).Msg("create evidence from assessment")
		return nil, nil
	}
	return &ev.ID, nil
}

// MarkAssessmentReviewed sets assessment=reviewed and supplier=completed.
func (s *Service) MarkAssessmentReviewed(ctx context.Context, orgID, assessmentID string) error {
	return s.repo.MarkAssessmentReviewed(ctx, orgID, assessmentID)
}

// GetAnswersForAssessment returns all answers for the review UI.
func (s *Service) GetAnswersForAssessment(ctx context.Context, orgID, assessmentID string) ([]AnswerWithReview, error) {
	return s.repo.GetAnswersForAssessment(ctx, orgID, assessmentID)
}

// FindExpiringCertificates returns certificate answers expiring within withinDays.
func (s *Service) FindExpiringCertificates(ctx context.Context, orgID string, withinDays int) ([]CertExpiryWarning, error) {
	before := time.Now().UTC().AddDate(0, 0, withinDays)
	return s.repo.FindExpiringCerts(ctx, orgID, before)
}

// GenerateAssessmentReportPDF builds a PDF report for an assessment.
func (s *Service) GenerateAssessmentReportPDF(ctx context.Context, orgID, assessmentID string) ([]byte, error) {
	asm, err := s.repo.GetAssessmentWithQuestionnaire(ctx, assessmentID)
	if err != nil || asm.OrgID != orgID {
		return nil, ErrNotFound
	}
	supplier, err := s.repo.GetSupplier(ctx, orgID, asm.SupplierID)
	if err != nil {
		return nil, fmt.Errorf("generate assessment pdf: get supplier: %w", err)
	}
	answers, err := s.repo.GetAnswersForAssessment(ctx, orgID, assessmentID)
	if err != nil {
		return nil, fmt.Errorf("generate assessment pdf: get answers: %w", err)
	}
	assessments, _ := s.repo.GetAssessmentsForSupplier(ctx, orgID, asm.SupplierID)
	status := computeStatus(*supplier, assessments, answers, time.Now().UTC())
	return GenerateAssessmentReportPDFBytes(asm, supplier, answers, status)
}
