package vaktcomply

import (
	"bytes"
	"context"
	"crypto/sha256"
	_ "embed"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/matharnica/vakt/internal/modules/vaktcomply/bsi"
	"github.com/matharnica/vakt/internal/modules/vaktcomply/policy"
	"github.com/matharnica/vakt/internal/shared/notify"
	"github.com/matharnica/vakt/internal/shared/veriniceimport"
	"github.com/rs/zerolog/log"
)

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

type VVTControlLink struct {
	ID        string    `json:"id"`
	OrgID     string    `json:"org_id"`
	VVTID     string    `json:"vvt_id"`
	VVTName   string    `json:"vvt_name"`
	ControlID string    `json:"control_id"`
	CreatedAt time.Time `json:"created_at"`
}

// LinkVVTToControlInput is the validated create payload.
type LinkVVTToControlInput struct {
	VVTID     string `json:"vvt_id" validate:"required"`
	VVTName   string `json:"vvt_name" validate:"max=255"`
	ControlID string `json:"control_id" validate:"required,uuid"`
}

// LinkVVTToControl creates an idempotent link (org-scoped). Verifies the control
// belongs to the org before linking.
func (s *Service) LinkVVTToControl(ctx context.Context, orgID string, in LinkVVTToControlInput) (*VVTControlLink, error) {
	var owns bool
	if err := s.db.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM ck_controls WHERE id=$1::uuid AND org_id=$2)`,
		in.ControlID, orgID).Scan(&owns); err != nil {
		return nil, fmt.Errorf("verify control: %w", err)
	}
	if !owns {
		return nil, fmt.Errorf("control not found")
	}
	var l VVTControlLink
	err := s.db.QueryRow(ctx, `
		INSERT INTO ck_vvt_control_links (org_id, vvt_id, vvt_name, control_id)
		VALUES ($1, $2, $3, $4::uuid)
		ON CONFLICT (org_id, vvt_id, control_id) DO UPDATE SET vvt_name = EXCLUDED.vvt_name
		RETURNING id::text, org_id::text, vvt_id, vvt_name, control_id::text, created_at`,
		orgID, in.VVTID, in.VVTName, in.ControlID).
		Scan(&l.ID, &l.OrgID, &l.VVTID, &l.VVTName, &l.ControlID, &l.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("link vvt to control: %w", err)
	}
	return &l, nil
}

// UnlinkVVTFromControl removes a link by id (org-scoped).
func (s *Service) UnlinkVVTFromControl(ctx context.Context, orgID, id string) error {
	tag, err := s.db.Exec(ctx, `DELETE FROM ck_vvt_control_links WHERE id=$1::uuid AND org_id=$2`, id, orgID)
	if err != nil {
		return fmt.Errorf("unlink vvt: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("link not found")
	}
	return nil
}

// ListLinksForControl returns the VVT links attached to a control (reverse view).
func (s *Service) ListLinksForControl(ctx context.Context, orgID, controlID string) ([]VVTControlLink, error) {
	return s.queryVVTLinks(ctx,
		`WHERE org_id=$1 AND control_id=$2::uuid ORDER BY created_at DESC`, orgID, controlID)
}

// ListLinksForVVT returns the controls linked to a VVT entry.
func (s *Service) ListLinksForVVT(ctx context.Context, orgID, vvtID string) ([]VVTControlLink, error) {
	return s.queryVVTLinks(ctx,
		`WHERE org_id=$1 AND vvt_id=$2 ORDER BY created_at DESC`, orgID, vvtID)
}

func (s *Service) queryVVTLinks(ctx context.Context, where string, args ...any) ([]VVTControlLink, error) {
	rows, err := s.db.Query(ctx,
		`SELECT id::text, org_id::text, vvt_id, vvt_name, control_id::text, created_at
		 FROM ck_vvt_control_links `+where, args...)
	if err != nil {
		return nil, fmt.Errorf("list vvt links: %w", err)
	}
	defer rows.Close()
	out := []VVTControlLink{}
	for rows.Next() {
		var l VVTControlLink
		if err := rows.Scan(&l.ID, &l.OrgID, &l.VVTID, &l.VVTName, &l.ControlID, &l.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan vvt link: %w", err)
		}
		out = append(out, l)
	}
	return out, rows.Err()
}

//go:embed catalogs/threat-library.json
var threatLibraryData []byte

// ThreatCatalogItem is one generic threat/scenario in the embedded library.
type ThreatCatalogItem struct {
	ID                string   `json:"id"`
	Title             string   `json:"title"`
	Category          string   `json:"category"`
	AssetTypes        []string `json:"asset_types"`
	CIA               []string `json:"cia"`
	Frameworks        []string `json:"frameworks"`
	Scenario          string   `json:"scenario"`
	SuggestedMeasure  string   `json:"suggested_measure"`
	ControlLinks      []string `json:"control_links"`
	DefaultLikelihood int      `json:"default_likelihood"`
	DefaultImpact     int      `json:"default_impact"`
}

type threatLibraryRoot struct {
	Version string              `json:"version"`
	Edition string              `json:"edition"`
	Threats []ThreatCatalogItem `json:"threats"`
}

var (
	threatLibOnce  sync.Once
	threatLibCache *threatLibraryRoot
)

func loadThreatLibrary() *threatLibraryRoot {
	threatLibOnce.Do(func() {
		var root threatLibraryRoot
		if err := json.Unmarshal(threatLibraryData, &root); err != nil {
			// Compiled in via go:embed — corrupt JSON means a broken build (fail fast).
			panic(fmt.Sprintf("threat library: corrupt embedded JSON: %v", err))
		}
		log.Info().Str("version", root.Version).Int("threats", len(root.Threats)).Msg("threat library loaded")
		threatLibCache = &root
	})
	return threatLibCache
}

// ThreatCatalogVersion returns the embedded library version (for link provenance).
func ThreatCatalogVersion() string {
	return loadThreatLibrary().Version
}

// ThreatCatalogFilter narrows the catalog by framework, asset type and/or CIA goal.
type ThreatCatalogFilter struct {
	Framework string // e.g. "ISO27001", "BSI", "NIS2", "DSGVO-TOM", "C5"
	AssetType string // e.g. "server", "data", "identity"
	CIA       string // "confidentiality" | "integrity" | "availability"
}

func sliceContainsFold(haystack []string, needle string) bool {
	for _, h := range haystack {
		if strings.EqualFold(h, needle) {
			return true
		}
	}
	return false
}

// ListThreatCatalog returns catalog items matching the filter (empty filter = all).
func (s *Service) ListThreatCatalog(f ThreatCatalogFilter) []ThreatCatalogItem {
	all := loadThreatLibrary().Threats
	if f.Framework == "" && f.AssetType == "" && f.CIA == "" {
		return all
	}
	out := make([]ThreatCatalogItem, 0, len(all))
	for _, it := range all {
		if f.Framework != "" && !sliceContainsFold(it.Frameworks, f.Framework) {
			continue
		}
		if f.AssetType != "" && !sliceContainsFold(it.AssetTypes, f.AssetType) {
			continue
		}
		if f.CIA != "" && !sliceContainsFold(it.CIA, f.CIA) {
			continue
		}
		out = append(out, it)
	}
	return out
}

func findThreatCatalogItem(id string) (ThreatCatalogItem, bool) {
	for _, it := range loadThreatLibrary().Threats {
		if it.ID == id {
			return it, true
		}
	}
	return ThreatCatalogItem{}, false
}

// CreateRiskFromCatalogInput allows the caller to override catalog defaults.
type CreateRiskFromCatalogInput struct {
	CatalogID  string `json:"catalog_id" validate:"required"`
	Likelihood int    `json:"likelihood" validate:"omitempty,min=1,max=5"`
	Impact     int    `json:"impact" validate:"omitempty,min=1,max=5"`
	Owner      string `json:"owner"`
}

// CreateRiskFromCatalog creates a pre-filled risk from a catalog item and records
// the provenance in ck_threat_library_links.
func (s *Service) CreateRiskFromCatalog(ctx context.Context, orgID string, in CreateRiskFromCatalogInput, userID string) (*Risk, error) {
	item, ok := findThreatCatalogItem(in.CatalogID)
	if !ok {
		return nil, fmt.Errorf("unknown catalog item: %s", in.CatalogID)
	}
	likelihood := in.Likelihood
	if likelihood == 0 {
		likelihood = item.DefaultLikelihood
	}
	impact := in.Impact
	if impact == 0 {
		impact = item.DefaultImpact
	}
	desc := item.Scenario
	if len(item.ControlLinks) > 0 {
		desc += "\n\nVorgeschlagene Maßnahme: " + item.SuggestedMeasure +
			"\nControl-Verknüpfung: " + strings.Join(item.ControlLinks, ", ")
	} else {
		desc += "\n\nVorgeschlagene Maßnahme: " + item.SuggestedMeasure
	}
	risk, err := s.Risk.CreateRisk(ctx, orgID, CreateRiskInput{
		Title:          item.Title,
		Description:    desc,
		Category:       item.Category,
		Likelihood:     likelihood,
		Impact:         impact,
		Owner:          in.Owner,
		Treatment:      "mitigate",
		TreatmentNotes: item.SuggestedMeasure,
	})
	if err != nil {
		return nil, err
	}
	// Record provenance (best-effort — the risk already exists).
	if _, linkErr := s.db.Exec(ctx, `
		INSERT INTO ck_threat_library_links (org_id, risk_id, catalog_id, catalog_version)
		VALUES ($1, $2, $3, $4)`,
		orgID, risk.ID, item.ID, loadThreatLibrary().Version); linkErr != nil {
		log.Warn().Err(linkErr).Str("org_id", orgID).Str("catalog_id", item.ID).Msg("threat library link insert")
	}
	return risk, nil
}

type ImportResult struct {
	AssetsCreated   int    `json:"assets_created"`
	ControlsCreated int    `json:"controls_created"`
	RisksCreated    int    `json:"risks_created"`
	Skipped         int    `json:"skipped"`
	FrameworkID     string `json:"framework_id,omitempty"`
}

// PreviewVeriniceImport parses the .vna and returns a dry-run preview (no writes).
func (s *Service) PreviewVeriniceImport(data []byte) (veriniceimport.Preview, error) {
	objs, err := veriniceimport.ParseVNA(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return veriniceimport.Preview{}, err
	}
	return veriniceimport.BuildPreview(objs), nil
}

// CommitVeriniceImport parses the .vna and creates the mapped Vakt entities.
func (s *Service) CommitVeriniceImport(ctx context.Context, orgID, userID string, data []byte) (ImportResult, error) {
	objs, err := veriniceimport.ParseVNA(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return ImportResult{}, err
	}
	var res ImportResult

	for _, o := range objs {
		switch o.Category {
		case "asset":
			itype := mapAssetType(o.Type)
			if _, err := s.BSI.CreateBSITargetObject(ctx, orgID, bsi.CreateBSITargetObjectInput{
				Name:               truncateStr(o.Title, 200),
				Type:               itype,
				Description:        "Importiert aus verinice (.vna)",
				Absicherungsniveau: "standard",
			}); err != nil {
				log.Warn().Err(err).Str("ext_id", o.ExtID).Msg("verinice import: asset")
				res.Skipped++
				continue
			}
			res.AssetsCreated++

		case "risk":
			if _, err := s.Risk.CreateRisk(ctx, orgID, CreateRiskInput{
				Title:          truncateStr(o.Title, 255),
				Description:    "Importiert aus verinice (.vna). Bitte Eintrittswahrscheinlichkeit und Auswirkung prüfen.",
				Category:       "verinice-Import",
				Likelihood:     3,
				Impact:         3,
				Treatment:      "mitigate",
				TreatmentNotes: "",
			}); err != nil {
				log.Warn().Err(err).Str("ext_id", o.ExtID).Msg("verinice import: risk")
				res.Skipped++
				continue
			}
			res.RisksCreated++

		case "control":
			if res.FrameworkID == "" {
				fwID, err := s.ensureVeriniceImportFramework(ctx, orgID)
				if err != nil {
					return res, fmt.Errorf("verinice import: framework: %w", err)
				}
				res.FrameworkID = fwID
			}
			if err := s.insertImportedControl(ctx, orgID, res.FrameworkID, o); err != nil {
				log.Warn().Err(err).Str("ext_id", o.ExtID).Msg("verinice import: control")
				res.Skipped++
				continue
			}
			res.ControlsCreated++

		default:
			res.Skipped++
		}
	}
	return res, nil
}

// ensureVeriniceImportFramework returns the id of the org's "verinice-Import"
// framework, creating it on first use.
func (s *Service) ensureVeriniceImportFramework(ctx context.Context, orgID string) (string, error) {
	var id string
	err := s.db.QueryRow(ctx,
		`SELECT id::text FROM ck_frameworks WHERE org_id = $1 AND name = 'verinice-Import'`,
		orgID).Scan(&id)
	if err == nil && id != "" {
		return id, nil
	}
	if insErr := s.db.QueryRow(ctx, `
		INSERT INTO ck_frameworks (org_id, name)
		VALUES ($1, 'verinice-Import')
		RETURNING id::text`, orgID).Scan(&id); insErr != nil {
		return "", insErr
	}
	return id, nil
}

func (s *Service) insertImportedControl(ctx context.Context, orgID, frameworkID string, o veriniceimport.ImportObject) error {
	controlID := o.ExtID
	if controlID == "" {
		controlID = "VN-" + truncateStr(o.Title, 40)
	}
	_, err := s.db.Exec(ctx, `
		INSERT INTO ck_controls (framework_id, org_id, control_id, title, description, domain)
		VALUES ($1::uuid, $2::uuid, $3, $4, $5, 'verinice-Import')`,
		frameworkID, orgID, controlID, truncateStr(o.Title, 255), "Importiert aus verinice (.vna)")
	return err
}

// mapAssetType maps an SNCA asset-ish type to a Vakt target-object type.
func mapAssetType(snca string) string {
	switch {
	case strings.Contains(snca, "network"):
		return "network"
	case strings.Contains(snca, "application"), strings.Contains(snca, "app"):
		return "application"
	case strings.Contains(snca, "room"), strings.Contains(snca, "raum"):
		return "room"
	case strings.Contains(snca, "process"), strings.Contains(snca, "prozess"):
		return "process"
	default:
		return "it_system"
	}
}

func truncateStr(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max]
}

func (s *Service) ListAISystems(ctx context.Context, orgID string, filters AISystemFilters) ([]AISystem, error) {
	systems, err := s.repo.ListAISystems(ctx, orgID, filters)
	if err != nil {
		return nil, err
	}
	if systems == nil {
		systems = []AISystem{}
	}
	return systems, nil
}

func (s *Service) GetAISystem(ctx context.Context, orgID, id string) (*AISystem, error) {
	return s.repo.GetAISystem(ctx, orgID, id)
}

func (s *Service) CreateAISystem(ctx context.Context, orgID string, in CreateAISystemInput) (*AISystem, error) {
	return s.repo.CreateAISystem(ctx, orgID, in)
}

func (s *Service) UpdateAISystem(ctx context.Context, orgID, id string, in UpdateAISystemInput) (*AISystem, error) {
	return s.repo.UpdateAISystem(ctx, orgID, id, in)
}

func (s *Service) DeleteAISystem(ctx context.Context, orgID, id string) error {
	return s.repo.DeleteAISystem(ctx, orgID, id)
}

// ClassifyAISystem saves a classification from the wizard and updates the AI system record.
func (s *Service) ClassifyAISystem(ctx context.Context, orgID, systemID string, in ClassifyAISystemInput) error {
	validClasses := map[string]bool{"minimal": true, "limited": true, "high": true, "unacceptable": true}
	if !validClasses[in.RiskClass] {
		return fmt.Errorf("risk_class: must be one of minimal, limited, high, unacceptable")
	}
	if _, err := s.repo.InsertAIClassification(ctx, orgID, systemID, in); err != nil {
		return fmt.Errorf("insert ai classification: %w", err)
	}
	return s.repo.UpdateAISystemClassification(ctx, orgID, systemID, in)
}

// ListAIClassifications returns the classification history for an AI system.
func (s *Service) ListAIClassifications(ctx context.Context, orgID, systemID string) ([]AIClassification, error) {
	return s.repo.ListAIClassifications(ctx, orgID, systemID)
}

// SaveAIDocumentation creates a new documentation version for an AI system.
func (s *Service) SaveAIDocumentation(ctx context.Context, orgID, systemID string, in UpsertAIDocumentationInput) (*AIDocumentation, error) {
	return s.repo.UpsertAIDocumentation(ctx, orgID, systemID, in)
}

// GetLatestAIDocumentation returns the most recent documentation for an AI system.
func (s *Service) GetLatestAIDocumentation(ctx context.Context, orgID, systemID string) (*AIDocumentation, error) {
	doc, err := s.repo.GetLatestAIDocumentation(ctx, orgID, systemID)
	if err != nil {
		return nil, ErrNotFound
	}
	return doc, nil
}

// ListAIDocumentationVersions returns all saved versions.
func (s *Service) ListAIDocumentationVersions(ctx context.Context, orgID, systemID string) ([]AIDocumentation, error) {
	return s.repo.ListAIDocumentationVersions(ctx, orgID, systemID)
}

// ExportAIDocumentationPDF generates the PDF technical dossier for an AI system.
func (s *Service) ExportAIDocumentationPDF(ctx context.Context, orgID, systemID string) ([]byte, string, error) {
	system, err := s.repo.GetAISystem(ctx, orgID, systemID)
	if err != nil {
		return nil, "", ErrNotFound
	}
	doc, err := s.repo.GetLatestAIDocumentation(ctx, orgID, systemID)
	if err != nil {
		// Return PDF with empty documentation fields if none saved yet
		doc = &AIDocumentation{AISystemID: systemID, Version: 0}
	}
	pdfBytes, err := GenerateAIDocumentationPDF(system, doc)
	if err != nil {
		return nil, "", fmt.Errorf("generate ai documentation pdf: %w", err)
	}
	filename := fmt.Sprintf("ai-dossier-%s-v%d.pdf", system.Name, doc.Version)
	return pdfBytes, filename, nil
}

const euAIActHighRiskDeadline = "2026-08-02"

// GetEUAIActDashboard builds the EU AI Act compliance dashboard for an organisation.
func (s *Service) GetEUAIActDashboard(ctx context.Context, orgID string) (*EUAIActDashboard, error) {
	total, byRisk, byStatus, withoutDocs, err := s.repo.GetEUAIActStats(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("get eu ai act dashboard: %w", err)
	}
	deadline, _ := time.Parse("2006-01-02", euAIActHighRiskDeadline)
	daysLeft := int(time.Until(deadline).Hours() / 24)
	if daysLeft < 0 {
		daysLeft = 0
	}
	return &EUAIActDashboard{
		TotalSystems:             total,
		SystemsByRiskClass:       byRisk,
		SystemsByStatus:          byStatus,
		SystemsWithoutDocs:       withoutDocs,
		HighRiskDeadline:         euAIActHighRiskDeadline,
		HighRiskDeadlineDaysLeft: daysLeft,
		ISO27001Mappings:         euAIActISOMappings,
	}, nil
}

// ExportEUAIActReportPDF generates the full EU AI Act compliance report PDF.
func (s *Service) ExportEUAIActReportPDF(ctx context.Context, orgID string) ([]byte, error) {
	dashboard, err := s.GetEUAIActDashboard(ctx, orgID)
	if err != nil {
		return nil, err
	}
	systems, err := s.repo.ListAISystems(ctx, orgID, AISystemFilters{})
	if err != nil {
		return nil, fmt.Errorf("list ai systems for pdf: %w", err)
	}
	return GenerateEUAIActReportPDF(dashboard, systems)
}
