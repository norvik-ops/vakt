package vaktcomply

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/matharnica/vakt/internal/db"
)

// --- Supplier Register (NIS2 Art. 21 / DORA Art. 28) ---

// supplierFields holds the shared columns of every Supplier-returning sqlc Row.
// All Row-Types (Create/Get/List/Update) haben identische Shape.
type supplierFields struct {
	ID, OrgID, Name                        string
	ContactName, ContactEmail, ServiceType pgtype.Text
	Criticality                            string
	Nis2Relevant, DoraRelevant             bool
	ContractEnd                            pgtype.Date
	Notes                                  pgtype.Text
	SubSuppliers                           []string
	DataLocation                           pgtype.Text
	ExitStrategyExists                     bool
	AssessmentStatus                       string
	LastAssessmentAt                       pgtype.Timestamptz
	CreatedAt, UpdatedAt                   pgtype.Timestamptz
}

func supplierFromFields(f supplierFields) Supplier {
	return Supplier{
		ID:                 f.ID,
		OrgID:              f.OrgID,
		Name:               f.Name,
		ContactName:        f.ContactName.String,
		ContactEmail:       f.ContactEmail.String,
		ServiceType:        f.ServiceType.String,
		Criticality:        f.Criticality,
		NIS2Relevant:       f.Nis2Relevant,
		DORARelevant:       f.DoraRelevant,
		ContractEnd:        ckDateToTimePtr(f.ContractEnd),
		Notes:              f.Notes.String,
		SubSuppliers:       f.SubSuppliers,
		DataLocation:       f.DataLocation.String,
		ExitStrategyExists: f.ExitStrategyExists,
		AssessmentStatus:   f.AssessmentStatus,
		LastAssessmentAt:   ckTsToTimePtr(f.LastAssessmentAt),
		CreatedAt:          ckTsToTime(f.CreatedAt),
		UpdatedAt:          ckTsToTime(f.UpdatedAt),
	}
}

func (r *Repository) ListSuppliers(ctx context.Context, orgID string, filter *SupplierFilter) ([]Supplier, error) {
	params := db.ListCKSuppliersParams{OrgID: orgID}
	if filter != nil {
		params.Criticality = ckOptText(filter.Criticality)
		params.AssessmentStatus = ckOptText(filter.AssessmentStatus)
	}
	rows, err := r.q.ListCKSuppliers(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("list suppliers: %w", err)
	}
	out := make([]Supplier, 0, len(rows))
	for _, row := range rows {
		out = append(out, supplierFromFields(supplierFields{
			ID: row.ID, OrgID: row.OrgID, Name: row.Name,
			ContactName: row.ContactName, ContactEmail: row.ContactEmail,
			ServiceType: row.ServiceType, Criticality: row.Criticality,
			Nis2Relevant: row.Nis2Relevant, DoraRelevant: row.DoraRelevant,
			ContractEnd: row.ContractEnd, Notes: row.Notes,
			SubSuppliers: row.SubSuppliers, DataLocation: row.DataLocation,
			ExitStrategyExists: row.ExitStrategyExists,
			AssessmentStatus:   row.AssessmentStatus,
			LastAssessmentAt:   row.LastAssessmentAt,
			CreatedAt:          row.CreatedAt, UpdatedAt: row.UpdatedAt,
		}))
	}
	return out, nil
}

func (r *Repository) GetSupplier(ctx context.Context, orgID, id string) (*Supplier, error) {
	row, err := r.q.GetCKSupplier(ctx, db.GetCKSupplierParams{ID: id, OrgID: orgID})
	if err != nil {
		return nil, fmt.Errorf("get supplier: %w", err)
	}
	s := supplierFromFields(supplierFields{
		ID: row.ID, OrgID: row.OrgID, Name: row.Name,
		ContactName: row.ContactName, ContactEmail: row.ContactEmail,
		ServiceType: row.ServiceType, Criticality: row.Criticality,
		Nis2Relevant: row.Nis2Relevant, DoraRelevant: row.DoraRelevant,
		ContractEnd: row.ContractEnd, Notes: row.Notes,
		SubSuppliers: row.SubSuppliers, DataLocation: row.DataLocation,
		ExitStrategyExists: row.ExitStrategyExists,
		AssessmentStatus:   row.AssessmentStatus,
		LastAssessmentAt:   row.LastAssessmentAt,
		CreatedAt:          row.CreatedAt, UpdatedAt: row.UpdatedAt,
	})
	return &s, nil
}

func (r *Repository) CreateSupplier(ctx context.Context, orgID string, in CreateSupplierInput) (*Supplier, error) {
	crit := in.Criticality
	if crit == "" {
		crit = "standard"
	}
	subSuppliers := in.SubSuppliers
	if subSuppliers == nil {
		subSuppliers = []string{}
	}
	assessmentStatus := in.AssessmentStatus
	if assessmentStatus == "" {
		assessmentStatus = "none"
	}
	row, err := r.q.CreateCKSupplier(ctx, db.CreateCKSupplierParams{
		OrgID:              orgID,
		Name:               in.Name,
		ContactName:        in.ContactName,
		ContactEmail:       in.ContactEmail,
		ServiceType:        in.ServiceType,
		Criticality:        crit,
		Nis2Relevant:       in.NIS2Relevant,
		DoraRelevant:       in.DORARelevant,
		ContractEnd:        policyDateFromTimePtr(in.ContractEnd),
		Notes:              in.Notes,
		SubSuppliers:       subSuppliers,
		DataLocation:       in.DataLocation,
		ExitStrategyExists: in.ExitStrategyExists,
		AssessmentStatus:   assessmentStatus,
		LastAssessmentAt:   ckOptTsPtr(in.LastAssessmentAt),
	})
	if err != nil {
		return nil, fmt.Errorf("create supplier: %w", err)
	}
	s := supplierFromFields(supplierFields{
		ID: row.ID, OrgID: row.OrgID, Name: row.Name,
		ContactName: row.ContactName, ContactEmail: row.ContactEmail,
		ServiceType: row.ServiceType, Criticality: row.Criticality,
		Nis2Relevant: row.Nis2Relevant, DoraRelevant: row.DoraRelevant,
		ContractEnd: row.ContractEnd, Notes: row.Notes,
		SubSuppliers: row.SubSuppliers, DataLocation: row.DataLocation,
		ExitStrategyExists: row.ExitStrategyExists,
		AssessmentStatus:   row.AssessmentStatus,
		LastAssessmentAt:   row.LastAssessmentAt,
		CreatedAt:          row.CreatedAt, UpdatedAt: row.UpdatedAt,
	})
	return &s, nil
}

func (r *Repository) UpdateSupplier(ctx context.Context, orgID, id string, in UpdateSupplierInput) (*Supplier, error) {
	crit := in.Criticality
	if crit == "" {
		crit = "standard"
	}
	subSuppliers := in.SubSuppliers
	if subSuppliers == nil {
		subSuppliers = []string{}
	}
	assessmentStatus := in.AssessmentStatus
	if assessmentStatus == "" {
		assessmentStatus = "none"
	}
	row, err := r.q.UpdateCKSupplier(ctx, db.UpdateCKSupplierParams{
		ID:                 id,
		OrgID:              orgID,
		Name:               in.Name,
		ContactName:        in.ContactName,
		ContactEmail:       in.ContactEmail,
		ServiceType:        in.ServiceType,
		Criticality:        crit,
		Nis2Relevant:       in.NIS2Relevant,
		DoraRelevant:       in.DORARelevant,
		ContractEnd:        policyDateFromTimePtr(in.ContractEnd),
		Notes:              in.Notes,
		SubSuppliers:       subSuppliers,
		DataLocation:       in.DataLocation,
		ExitStrategyExists: in.ExitStrategyExists,
		AssessmentStatus:   assessmentStatus,
		LastAssessmentAt:   ckOptTsPtr(in.LastAssessmentAt),
	})
	if err != nil {
		return nil, fmt.Errorf("update supplier: %w", err)
	}
	s := supplierFromFields(supplierFields{
		ID: row.ID, OrgID: row.OrgID, Name: row.Name,
		ContactName: row.ContactName, ContactEmail: row.ContactEmail,
		ServiceType: row.ServiceType, Criticality: row.Criticality,
		Nis2Relevant: row.Nis2Relevant, DoraRelevant: row.DoraRelevant,
		ContractEnd: row.ContractEnd, Notes: row.Notes,
		SubSuppliers: row.SubSuppliers, DataLocation: row.DataLocation,
		ExitStrategyExists: row.ExitStrategyExists,
		AssessmentStatus:   row.AssessmentStatus,
		LastAssessmentAt:   row.LastAssessmentAt,
		CreatedAt:          row.CreatedAt, UpdatedAt: row.UpdatedAt,
	})
	return &s, nil
}

func (r *Repository) DeleteSupplier(ctx context.Context, orgID, id string) error {
	n, err := r.q.DeleteCKSupplier(ctx, db.DeleteCKSupplierParams{ID: id, OrgID: orgID})
	if err != nil {
		return fmt.Errorf("delete supplier: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("supplier not found")
	}
	return nil
}

// supplierExists ensures the supplier belongs to the org; reused by link/unlink/list.
func (r *Repository) supplierExists(ctx context.Context, supplierID, orgID string) error {
	exists, err := r.q.CKSupplierExists(ctx, db.CKSupplierExistsParams{ID: supplierID, OrgID: orgID})
	if err != nil {
		return fmt.Errorf("verify supplier: %w", err)
	}
	if !exists {
		return fmt.Errorf("supplier not found")
	}
	return nil
}

// LinkSupplierRisk links a risk to a supplier. Idempotent (ON CONFLICT DO NOTHING).
func (r *Repository) LinkSupplierRisk(ctx context.Context, orgID, supplierID, riskID string) error {
	if err := r.supplierExists(ctx, supplierID, orgID); err != nil {
		return err
	}
	if err := r.q.LinkCKSupplierRisk(ctx, db.LinkCKSupplierRiskParams{SupplierID: supplierID, RiskID: riskID}); err != nil {
		return fmt.Errorf("link supplier risk: %w", err)
	}
	return nil
}

// UnlinkSupplierRisk removes a risk link from a supplier.
func (r *Repository) UnlinkSupplierRisk(ctx context.Context, orgID, supplierID, riskID string) error {
	if err := r.supplierExists(ctx, supplierID, orgID); err != nil {
		return err
	}
	if err := r.q.UnlinkCKSupplierRisk(ctx, db.UnlinkCKSupplierRiskParams{SupplierID: supplierID, RiskID: riskID}); err != nil {
		return fmt.Errorf("unlink supplier risk: %w", err)
	}
	return nil
}

// ListSupplierRisks returns all risks linked to the given supplier.
func (r *Repository) ListSupplierRisks(ctx context.Context, orgID, supplierID string) ([]Risk, error) {
	if err := r.supplierExists(ctx, supplierID, orgID); err != nil {
		return nil, err
	}
	rows, err := r.q.ListCKSupplierRisks(ctx, db.ListCKSupplierRisksParams{SupplierID: supplierID, OrgID: orgID})
	if err != nil {
		return nil, fmt.Errorf("list supplier risks: %w", err)
	}
	out := make([]Risk, 0, len(rows))
	for _, row := range rows {
		out = append(out, riskFromFields(riskFields{
			ID: row.ID, OrgID: row.OrgID, Title: row.Title,
			Description: row.Description, Category: row.Category,
			Likelihood: row.Likelihood, Impact: row.Impact, RiskScore: row.RiskScore,
			Owner: row.Owner, Status: row.Status, Treatment: row.Treatment,
			TreatmentNotes:  row.TreatmentNotes,
			TreatmentOption: row.TreatmentOption, TreatmentPlan: row.TreatmentPlan,
			TreatmentOwner: row.TreatmentOwner, TreatmentDueDate: row.TreatmentDueDate,
			TreatmentStatus:    row.TreatmentStatus,
			ResidualLikelihood: row.ResidualLikelihood, ResidualImpact: row.ResidualImpact,
			CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
		}))
	}
	return out, nil
}

// UpdateSupplierAssessmentStatus sets assessment_status and last_assessment_at on a supplier row.
func (r *Repository) UpdateSupplierAssessmentStatus(ctx context.Context, orgID, supplierID, status string, at *time.Time) error {
	if err := r.q.UpdateCKSupplierAssessmentStatus(ctx, db.UpdateCKSupplierAssessmentStatusParams{
		ID:               supplierID,
		AssessmentStatus: status,
		LastAssessmentAt: ckOptTsPtr(at),
		OrgID:            orgID,
	}); err != nil {
		return fmt.Errorf("update supplier assessment status: %w", err)
	}
	return nil
}

// --- Questionnaire Builder (Story 29.2) ---

func questionnaireFromCk(r db.CkQuestionnaires) Questionnaire {
	return Questionnaire{
		ID:          r.ID,
		OrgID:       r.OrgID,
		Name:        r.Name,
		Description: r.Description.String,
		IsTemplate:  r.IsTemplate,
		CreatedAt:   ckTsToTime(r.CreatedAt),
		UpdatedAt:   ckTsToTime(r.UpdatedAt),
	}
}

func questionFromCk(r db.CkQuestionnaireQuestions) Question {
	q := Question{
		ID:              r.ID,
		QuestionnaireID: r.QuestionnaireID,
		OrderIdx:        int(r.OrderIdx),
		QuestionText:    r.QuestionText,
		QuestionType:    r.QuestionType,
		Required:        r.Required,
		ControlID:       uuidPtrFromPgtype(r.ControlID),
		CreatedAt:       ckTsToTime(r.CreatedAt),
		UpdatedAt:       ckTsToTime(r.UpdatedAt),
	}
	if len(r.Options) > 0 {
		_ = json.Unmarshal(r.Options, &q.Options)
	}
	return q
}

// CreateQuestionnaire inserts a new questionnaire for an organisation.
func (r *Repository) CreateQuestionnaire(ctx context.Context, orgID, name, description string, isTemplate bool) (*Questionnaire, error) {
	row, err := r.q.CreateCKQuestionnaire(ctx, db.CreateCKQuestionnaireParams{
		OrgID:       orgID,
		Name:        name,
		Description: ckOptText(description),
		IsTemplate:  isTemplate,
	})
	if err != nil {
		return nil, fmt.Errorf("create questionnaire: %w", err)
	}
	q := questionnaireFromCk(db.CkQuestionnaires(row))
	return &q, nil
}

// GetQuestionnaire returns a questionnaire with its questions ordered by order_idx.
func (r *Repository) GetQuestionnaire(ctx context.Context, orgID, id string) (*Questionnaire, error) {
	row, err := r.q.GetCKQuestionnaireBase(ctx, db.GetCKQuestionnaireBaseParams{ID: id, OrgID: orgID})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("questionnaire not found")
		}
		return nil, fmt.Errorf("get questionnaire: %w", err)
	}
	q := questionnaireFromCk(db.CkQuestionnaires(row))
	questions, err := r.ListQuestions(ctx, id)
	if err != nil {
		return nil, err
	}
	q.Questions = questions
	return &q, nil
}

// ListQuestionnaires returns questionnaires for an org, optionally filtered by is_template.
func (r *Repository) ListQuestionnaires(ctx context.Context, orgID string, isTemplate *bool) ([]Questionnaire, error) {
	params := db.ListCKQuestionnairesParams{OrgID: orgID}
	if isTemplate != nil {
		params.IsTemplate = pgtype.Bool{Bool: *isTemplate, Valid: true}
	}
	rows, err := r.q.ListCKQuestionnaires(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("list questionnaires: %w", err)
	}
	out := make([]Questionnaire, 0, len(rows))
	for _, row := range rows {
		out = append(out, questionnaireFromCk(db.CkQuestionnaires(row)))
	}
	return out, nil
}

// UpdateQuestionnaire updates name/description/is_template of a questionnaire.
func (r *Repository) UpdateQuestionnaire(ctx context.Context, orgID, id, name, description string, isTemplate bool) (*Questionnaire, error) {
	row, err := r.q.UpdateCKQuestionnaire(ctx, db.UpdateCKQuestionnaireParams{
		ID:          id,
		OrgID:       orgID,
		Name:        name,
		Description: ckOptText(description),
		IsTemplate:  isTemplate,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("questionnaire not found")
		}
		return nil, fmt.Errorf("update questionnaire: %w", err)
	}
	q := questionnaireFromCk(db.CkQuestionnaires(row))
	return &q, nil
}

// DeleteQuestionnaire removes a questionnaire and its questions (cascade).
func (r *Repository) DeleteQuestionnaire(ctx context.Context, orgID, id string) error {
	n, err := r.q.DeleteCKQuestionnaire(ctx, db.DeleteCKQuestionnaireParams{ID: id, OrgID: orgID})
	if err != nil {
		return fmt.Errorf("delete questionnaire: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("questionnaire not found")
	}
	return nil
}

// CreateQuestion inserts a new question into a questionnaire.
func (r *Repository) CreateQuestion(ctx context.Context, questionnaireID, questionText, questionType string, options []string, required bool, controlID *string) (*Question, error) {
	maxIdx, err := r.q.NextCKQuestionOrderIdx(ctx, questionnaireID)
	if err != nil {
		return nil, fmt.Errorf("next order_idx: %w", err)
	}
	var optionsJSON []byte
	if len(options) > 0 {
		var err error
		optionsJSON, err = json.Marshal(options)
		if err != nil {
			return nil, fmt.Errorf("marshal options: %w", err)
		}
	}
	row, err := r.q.CreateCKQuestion(ctx, db.CreateCKQuestionParams{
		QuestionnaireID: questionnaireID,
		OrderIdx:        maxIdx,
		QuestionText:    questionText,
		QuestionType:    questionType,
		Options:         optionsJSON,
		Required:        required,
		ControlID:       ckOptUUIDFromPtr(controlID),
	})
	if err != nil {
		return nil, fmt.Errorf("create question: %w", err)
	}
	q := questionFromCk(db.CkQuestionnaireQuestions(row))
	return &q, nil
}

// GetQuestion returns a single question by ID.
func (r *Repository) GetQuestion(ctx context.Context, questionnaireID, questionID string) (*Question, error) {
	row, err := r.q.GetCKQuestion(ctx, db.GetCKQuestionParams{ID: questionID, QuestionnaireID: questionnaireID})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("question not found")
		}
		return nil, fmt.Errorf("get question: %w", err)
	}
	q := questionFromCk(db.CkQuestionnaireQuestions(row))
	return &q, nil
}

// UpdateQuestion updates an existing question.
func (r *Repository) UpdateQuestion(ctx context.Context, questionnaireID, questionID, questionText, questionType string, options []string, required bool, controlID *string) (*Question, error) {
	var optionsJSON []byte
	if len(options) > 0 {
		var err error
		optionsJSON, err = json.Marshal(options)
		if err != nil {
			return nil, fmt.Errorf("marshal options: %w", err)
		}
	}
	row, err := r.q.UpdateCKQuestion(ctx, db.UpdateCKQuestionParams{
		ID:              questionID,
		QuestionnaireID: questionnaireID,
		QuestionText:    questionText,
		QuestionType:    questionType,
		Options:         optionsJSON,
		Required:        required,
		ControlID:       ckOptUUIDFromPtr(controlID),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("question not found")
		}
		return nil, fmt.Errorf("update question: %w", err)
	}
	q := questionFromCk(db.CkQuestionnaireQuestions(row))
	return &q, nil
}

// DeleteQuestion removes a question.
func (r *Repository) DeleteQuestion(ctx context.Context, questionnaireID, questionID string) error {
	n, err := r.q.DeleteCKQuestion(ctx, db.DeleteCKQuestionParams{ID: questionID, QuestionnaireID: questionnaireID})
	if err != nil {
		return fmt.Errorf("delete question: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("question not found")
	}
	return nil
}

// ListQuestions returns all questions for a questionnaire ordered by order_idx.
func (r *Repository) ListQuestions(ctx context.Context, questionnaireID string) ([]Question, error) {
	rows, err := r.q.ListCKQuestions(ctx, questionnaireID)
	if err != nil {
		return nil, fmt.Errorf("list questions: %w", err)
	}
	out := make([]Question, 0, len(rows))
	for _, row := range rows {
		out = append(out, questionFromCk(db.CkQuestionnaireQuestions(row)))
	}
	return out, nil
}

// ReorderQuestions updates order_idx for each question ID in the provided slice.
// Original used pgx.Batch; sqlc-Variante iteriert sequentiell. Bei kleinen Listen
// (typischerweise <20 Questions) keine messbare Performance-Differenz.
func (r *Repository) ReorderQuestions(ctx context.Context, questionnaireID string, order []string) error {
	for i, qID := range order {
		if err := r.q.ReorderCKQuestion(ctx, db.ReorderCKQuestionParams{
			OrderIdx:        int32(i),
			ID:              qID,
			QuestionnaireID: questionnaireID,
		}); err != nil {
			return fmt.Errorf("reorder questions: %w", err)
		}
	}
	return nil
}

// CloneQuestionnaire copies a questionnaire and all its questions with new UUIDs.
func (r *Repository) CloneQuestionnaire(ctx context.Context, orgID, sourceID, name string) (*Questionnaire, error) {
	src, err := r.GetQuestionnaire(ctx, orgID, sourceID)
	if err != nil {
		return nil, fmt.Errorf("clone: get source: %w", err)
	}

	newQ, err := r.CreateQuestionnaire(ctx, orgID, name, src.Description, false)
	if err != nil {
		return nil, fmt.Errorf("clone: create questionnaire: %w", err)
	}

	for _, sq := range src.Questions {
		if _, err := r.CreateQuestion(ctx, newQ.ID, sq.QuestionText, sq.QuestionType, sq.Options, sq.Required, sq.ControlID); err != nil {
			return nil, fmt.Errorf("clone: copy question: %w", err)
		}
	}

	return r.GetQuestionnaire(ctx, orgID, newQ.ID)
}

// --- Supplier Portal Assessments (Story 29.3) ---

func assessmentFromCk(r db.CkSupplierAssessments) Assessment {
	return Assessment{
		ID:              r.ID,
		OrgID:           r.OrgID,
		SupplierID:      r.SupplierID,
		QuestionnaireID: r.QuestionnaireID,
		TokenHash:       r.TokenHash,
		ExpiresAt:       ckTsToTime(r.ExpiresAt),
		Status:          r.Status,
		SubmittedAt:     ckTsToTimePtr(r.SubmittedAt),
		SubmittedByIP:   r.SubmittedByIp.String,
		UserAgent:       r.UserAgent.String,
		CreatedAt:       ckTsToTime(r.CreatedAt),
	}
}

// CreateAssessment inserts a new supplier assessment record.
func (r *Repository) CreateAssessment(ctx context.Context, a Assessment) error {
	if err := r.q.CreateCKAssessment(ctx, db.CreateCKAssessmentParams{
		OrgID:           a.OrgID,
		SupplierID:      a.SupplierID,
		QuestionnaireID: a.QuestionnaireID,
		TokenHash:       a.TokenHash,
		ExpiresAt:       pgtype.Timestamptz{Time: a.ExpiresAt, Valid: true},
		Status:          a.Status,
	}); err != nil {
		return fmt.Errorf("create assessment: %w", err)
	}
	return nil
}

// GetAssessmentByTokenHash looks up an assessment by its SHA-256 token hash.
func (r *Repository) GetAssessmentByTokenHash(ctx context.Context, hash string) (*Assessment, error) {
	row, err := r.q.GetCKAssessmentByTokenHash(ctx, hash)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("assessment not found")
		}
		return nil, fmt.Errorf("get assessment: %w", err)
	}
	a := assessmentFromCk(db.CkSupplierAssessments(row))
	return &a, nil
}

// UpdateAssessmentStatus updates status and optional submission metadata.
// For terminal transitions (submitted/reviewed), the UPDATE is conditional on
// the current status not already being terminal — preventing double-submit races.
func (r *Repository) UpdateAssessmentStatus(ctx context.Context, id, status string, submittedAt *time.Time, submittedByIP, userAgent string) error {
	n, err := r.q.UpdateCKAssessmentStatus(ctx, db.UpdateCKAssessmentStatusParams{
		ID:            id,
		Status:        status,
		SubmittedAt:   ckOptTsPtr(submittedAt),
		SubmittedByIp: ckOptText(submittedByIP),
		UserAgent:     ckOptText(userAgent),
	})
	if err != nil {
		return fmt.Errorf("update assessment status: %w", err)
	}
	if n == 0 && (status == "submitted" || status == "reviewed") {
		return ErrAssessmentExpiredOrSubmitted
	}
	return nil
}

// UpsertAnswers upserts answers sequentially via UpsertCKAnswer. Original code
// nutzte pgx.Batch — sqlc-Variante macht den Trade-off Batch-Performance vs.
// Type-Safety. Da Supplier-Antworten typischerweise einzeln über das Portal
// gesendet werden (selten >50 in einem Batch), ist sequentiell akzeptabel.
func (r *Repository) UpsertAnswers(ctx context.Context, assessmentID string, answers []AnswerInput) error {
	if len(answers) == 0 {
		return nil
	}
	for _, ans := range answers {
		var optionsJSON []byte
		if len(ans.AnswerOptions) > 0 {
			var jsonErr error
			optionsJSON, jsonErr = json.Marshal(ans.AnswerOptions)
			if jsonErr != nil {
				return fmt.Errorf("marshal answer_options: %w", jsonErr)
			}
		}
		if err := r.q.UpsertCKAnswer(ctx, db.UpsertCKAnswerParams{
			AssessmentID:  assessmentID,
			QuestionID:    ans.QuestionID,
			AnswerText:    ckOptText(ans.AnswerText),
			AnswerBool:    pgtype.Bool{Bool: ans.AnswerBool != nil && *ans.AnswerBool, Valid: ans.AnswerBool != nil},
			AnswerOptions: optionsJSON,
			FileUrl:       ckOptText(ans.FileURL),
		}); err != nil {
			return fmt.Errorf("upsert answer: %w", err)
		}
	}
	return nil
}

// GetAssessmentWithQuestionnaire returns an assessment joined with its questionnaire and questions.
func (r *Repository) GetAssessmentWithQuestionnaire(ctx context.Context, id string) (*AssessmentWithQuestionnaire, error) {
	row, err := r.q.GetCKAssessmentBase(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("assessment not found")
		}
		return nil, fmt.Errorf("get assessment: %w", err)
	}
	a := assessmentFromCk(db.CkSupplierAssessments(row))
	qnr, err := r.GetQuestionnaire(ctx, a.OrgID, a.QuestionnaireID)
	if err != nil {
		return nil, fmt.Errorf("get questionnaire for assessment: %w", err)
	}
	return &AssessmentWithQuestionnaire{
		Assessment:    a,
		Questionnaire: qnr,
	}, nil
}

// ListAssessmentsForSupplier returns all assessments for a given supplier within an org.
func (r *Repository) ListAssessmentsForSupplier(ctx context.Context, orgID, supplierID string) ([]Assessment, error) {
	rows, err := r.q.ListCKAssessmentsForSupplier(ctx, db.ListCKAssessmentsForSupplierParams{
		OrgID:      orgID,
		SupplierID: supplierID,
	})
	if err != nil {
		return nil, fmt.Errorf("list assessments: %w", err)
	}
	out := make([]Assessment, 0, len(rows))
	for _, row := range rows {
		out = append(out, assessmentFromCk(db.CkSupplierAssessments(row)))
	}
	return out, nil
}

// --- Assessment Review (Story 29.4) ---

// UpdateAnswerReview sets review_status and rework_note on a single answer.
// org_id wird via JOIN auf ck_supplier_assessments validiert (ck_supplier_answers
// hat keine eigene org_id-Spalte — Schema-Lücke aus Migration 048; existierender
// Code referenzierte sa.org_id was zur Laufzeit gefehlt hätte).
func (r *Repository) UpdateAnswerReview(ctx context.Context, orgID, assessmentID, answerID, reviewStatus, reworkNote string) error {
	n, err := r.q.UpdateCKAnswerReview(ctx, db.UpdateCKAnswerReviewParams{
		ReviewStatus: ckOptText(reviewStatus),
		Column2:      reworkNote,
		ID:           answerID,
		AssessmentID: assessmentID,
		OrgID:        orgID,
	})
	if err != nil {
		return fmt.Errorf("update answer review: %w", err)
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// GetAnswerWithQuestion fetches a single answer joined with its question (for evidence creation).
func (r *Repository) GetAnswerWithQuestion(ctx context.Context, orgID, answerID string) (*AnswerWithQuestion, error) {
	row, err := r.q.GetCKAnswerWithQuestion(ctx, db.GetCKAnswerWithQuestionParams{ID: answerID, OrgID: orgID})
	if err != nil {
		return nil, fmt.Errorf("get answer with question: %w", err)
	}
	return &AnswerWithQuestion{
		AnswerID:       row.AnswerID,
		AssessmentID:   row.AssessmentID,
		OrgID:          row.OrgID,
		QuestionID:     row.QuestionID,
		QuestionText:   row.QuestionText,
		ControlID:      uuidPtrFromPgtype(row.ControlID),
		AnswerText:     row.AnswerText,
		FileURL:        row.FileUrl,
		ReviewStatus:   textPtrOrNil(row.ReviewStatus),
		ReworkNote:     textPtrOrNil(row.ReworkNote),
		CertExpiryDate: ckDateToTimePtr(row.CertExpiryDate),
	}, nil
}

// GetAnswersForAssessment returns all answers for an assessment with question info.
func (r *Repository) GetAnswersForAssessment(ctx context.Context, orgID, assessmentID string) ([]AnswerWithReview, error) {
	rows, err := r.q.GetCKAnswersForAssessment(ctx, db.GetCKAnswersForAssessmentParams{AssessmentID: assessmentID, OrgID: orgID})
	if err != nil {
		return nil, fmt.Errorf("get answers for assessment: %w", err)
	}
	out := make([]AnswerWithReview, 0, len(rows))
	for _, row := range rows {
		out = append(out, AnswerWithReview{
			ID:             row.ID,
			QuestionText:   row.QuestionText,
			AnswerText:     row.AnswerText,
			FileURL:        row.FileUrl,
			ReviewStatus:   textPtrOrNil(row.ReviewStatus),
			ReworkNote:     textPtrOrNil(row.ReworkNote),
			ControlID:      uuidPtrFromPgtype(row.ControlID),
			CertExpiryDate: ckDateToTimePtr(row.CertExpiryDate),
		})
	}
	return out, nil
}

// MarkAssessmentReviewed atomically sets status=reviewed and updates the supplier's assessment_status.
func (r *Repository) MarkAssessmentReviewed(ctx context.Context, orgID, assessmentID string) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("mark assessment reviewed: begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }() // no-op when Commit succeeded
	qtx := r.q.WithTx(tx)

	supplierID, err := qtx.MarkCKAssessmentReviewed(ctx, db.MarkCKAssessmentReviewedParams{ID: assessmentID, OrgID: orgID})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		return fmt.Errorf("mark assessment reviewed: update assessment: %w", err)
	}
	if err := qtx.UpdateCKSupplierAssessmentStatus(ctx, db.UpdateCKSupplierAssessmentStatusParams{
		ID:               supplierID,
		AssessmentStatus: "completed",
		LastAssessmentAt: pgtype.Timestamptz{Time: time.Now().UTC(), Valid: true},
		OrgID:            orgID,
	}); err != nil {
		return fmt.Errorf("mark assessment reviewed: update supplier: %w", err)
	}
	return tx.Commit(ctx)
}

// GetAssessmentsForSupplier returns all assessments for a supplier, newest first.
func (r *Repository) GetAssessmentsForSupplier(ctx context.Context, orgID, supplierID string) ([]Assessment, error) {
	rows, err := r.q.ListCKAssessmentsForSupplier(ctx, db.ListCKAssessmentsForSupplierParams{
		OrgID:      orgID,
		SupplierID: supplierID,
	})
	if err != nil {
		return nil, fmt.Errorf("get assessments for supplier: %w", err)
	}
	out := make([]Assessment, 0, len(rows))
	for _, row := range rows {
		out = append(out, assessmentFromCk(db.CkSupplierAssessments(row)))
	}
	return out, nil
}

// FindExpiringCerts returns certificate answers whose cert_expiry_date is on or before the threshold.
func (r *Repository) FindExpiringCerts(ctx context.Context, orgID string, before time.Time) ([]CertExpiryWarning, error) {
	rows, err := r.q.FindCKExpiringCerts(ctx, db.FindCKExpiringCertsParams{
		OrgID:          orgID,
		CertExpiryDate: pgtype.Date{Time: before, Valid: true},
	})
	if err != nil {
		return nil, fmt.Errorf("find expiring certs: %w", err)
	}
	results := make([]CertExpiryWarning, 0, len(rows))
	for _, row := range rows {
		w := CertExpiryWarning{
			AnswerID:     row.AnswerID,
			SupplierID:   row.SupplierID,
			SupplierName: row.SupplierName,
			QuestionText: row.QuestionText,
			FileURL:      row.FileUrl.String,
		}
		if row.CertExpiryDate.Valid {
			w.CertExpiryDate = row.CertExpiryDate.Time
		}
		results = append(results, w)
	}
	return results, nil
}
