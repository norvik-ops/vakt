package vaktcomply

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/matharnica/vakt/internal/db"
)

func aiSystemFromCkAiSystems(r db.CkAiSystems) AISystem {
	return AISystem{
		ID:                      r.ID,
		OrgID:                   r.OrgID,
		Name:                    r.Name,
		Description:             r.Description.String,
		Provider:                r.Provider.String,
		UseCase:                 r.UseCase.String,
		AffectedGroups:          r.AffectedGroups.String,
		AutonomyLevel:           r.AutonomyLevel,
		InProductionSince:       ckDateToTimePtr(r.InProductionSince),
		Status:                  r.Status,
		RiskClass:               r.RiskClass.String,
		ClassificationRationale: r.ClassificationRationale.String,
		ClassifiedAt:            ckTsToTimePtr(r.ClassifiedAt),
		ClassifiedBy:            r.ClassifiedBy.String,
		CreatedAt:               ckTsToTime(r.CreatedAt),
		UpdatedAt:               ckTsToTime(r.UpdatedAt),
	}
}

func (r *Repository) ListAISystems(ctx context.Context, orgID string, filters AISystemFilters) ([]AISystem, error) {
	rows, err := r.q.ListCKAISystems(ctx, db.ListCKAISystemsParams{
		OrgID:     orgID,
		RiskClass: ckOptText(filters.RiskClass),
		Status:    ckOptText(filters.Status),
	})
	if err != nil {
		return nil, fmt.Errorf("list ai systems: %w", err)
	}
	out := make([]AISystem, 0, len(rows))
	for _, row := range rows {
		out = append(out, aiSystemFromCkAiSystems(db.CkAiSystems(row)))
	}
	return out, nil
}

func (r *Repository) DeleteAISystem(ctx context.Context, orgID, id string) error {
	n, err := r.q.DeleteCKAISystem(ctx, db.DeleteCKAISystemParams{ID: id, OrgID: orgID})
	if err != nil {
		return fmt.Errorf("delete ai system: %w", err)
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *Repository) GetAISystem(ctx context.Context, orgID, id string) (*AISystem, error) {
	row, err := r.q.GetCKAISystem(ctx, db.GetCKAISystemParams{ID: id, OrgID: orgID})
	if err != nil {
		return nil, fmt.Errorf("get ai system: %w", err)
	}
	a := aiSystemFromCkAiSystems(db.CkAiSystems(row))
	return &a, nil
}

func (r *Repository) CreateAISystem(ctx context.Context, orgID string, in CreateAISystemInput) (*AISystem, error) {
	al := in.AutonomyLevel
	if al == "" {
		al = "assistive"
	}
	row, err := r.q.CreateCKAISystem(ctx, db.CreateCKAISystemParams{
		OrgID:                   orgID,
		Name:                    in.Name,
		Description:             in.Description,
		Provider:                in.Provider,
		UseCase:                 in.UseCase,
		AffectedGroups:          in.AffectedGroups,
		AutonomyLevel:           al,
		InProductionSince:       policyDateFromTimePtr(in.InProductionSince),
		RiskClass:               in.RiskClass,
		ClassificationRationale: in.ClassificationRationale,
	})
	if err != nil {
		return nil, fmt.Errorf("create ai system: %w", err)
	}
	a := aiSystemFromCkAiSystems(db.CkAiSystems(row))
	return &a, nil
}

func (r *Repository) UpdateAISystem(ctx context.Context, orgID, id string, in UpdateAISystemInput) (*AISystem, error) {
	al := in.AutonomyLevel
	if al == "" {
		al = "assistive"
	}
	st := in.Status
	if st == "" {
		st = "under_review"
	}
	var classifiedAt pgtype.Timestamptz
	if in.ClassifiedBy != "" && in.RiskClass != "" {
		classifiedAt = pgtype.Timestamptz{Time: time.Now(), Valid: true}
	}
	row, err := r.q.UpdateCKAISystem(ctx, db.UpdateCKAISystemParams{
		ID:                      id,
		OrgID:                   orgID,
		Name:                    in.Name,
		Description:             in.Description,
		Provider:                in.Provider,
		UseCase:                 in.UseCase,
		AffectedGroups:          in.AffectedGroups,
		AutonomyLevel:           al,
		InProductionSince:       policyDateFromTimePtr(in.InProductionSince),
		Status:                  st,
		RiskClass:               in.RiskClass,
		ClassificationRationale: in.ClassificationRationale,
		ClassifiedAt:            classifiedAt,
		ClassifiedBy:            in.ClassifiedBy,
	})
	if err != nil {
		return nil, fmt.Errorf("update ai system: %w", err)
	}
	a := aiSystemFromCkAiSystems(db.CkAiSystems(row))
	return &a, nil
}

// InsertAIClassification saves a new classification event and returns its ID.
func (r *Repository) InsertAIClassification(ctx context.Context, orgID, systemID string, in ClassifyAISystemInput) (string, error) {
	var wizardJSON []byte
	if in.WizardAnswers != nil {
		var err error
		wizardJSON, err = json.Marshal(in.WizardAnswers)
		if err != nil {
			return "", fmt.Errorf("marshal wizard answers: %w", err)
		}
	}
	id, err := r.q.InsertCKAIClassification(ctx, db.InsertCKAIClassificationParams{
		OrgID:         orgID,
		AiSystemID:    systemID,
		RiskClass:     in.RiskClass,
		Rationale:     ckOptText(in.Rationale),
		ClassifiedBy:  ckOptText(in.ClassifiedBy),
		WizardAnswers: wizardJSON,
	})
	if err != nil {
		return "", fmt.Errorf("insert ai classification: %w", err)
	}
	return id, nil
}

// UpdateAISystemClassification sets the denormalized classification fields on the AI system row.
func (r *Repository) UpdateAISystemClassification(ctx context.Context, orgID, systemID string, in ClassifyAISystemInput) error {
	n, err := r.q.UpdateCKAISystemClassification(ctx, db.UpdateCKAISystemClassificationParams{
		ID:                      systemID,
		OrgID:                   orgID,
		RiskClass:               ckOptText(in.RiskClass),
		ClassificationRationale: ckOptText(in.Rationale),
		ClassifiedBy:            ckOptText(in.ClassifiedBy),
	})
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// ListAIClassifications returns the classification history for an AI system, newest first.
func (r *Repository) ListAIClassifications(ctx context.Context, orgID, systemID string) ([]AIClassification, error) {
	rows, err := r.q.ListCKAIClassifications(ctx, db.ListCKAIClassificationsParams{
		OrgID:      orgID,
		AiSystemID: systemID,
	})
	if err != nil {
		return nil, fmt.Errorf("list ai classifications: %w", err)
	}
	results := make([]AIClassification, 0, len(rows))
	for _, row := range rows {
		c := AIClassification{
			ID:           row.ID,
			OrgID:        row.OrgID,
			AISystemID:   row.AiSystemID,
			RiskClass:    row.RiskClass,
			Rationale:    row.Rationale.String,
			ClassifiedBy: row.ClassifiedBy.String,
			ClassifiedAt: ckTsToTime(row.ClassifiedAt),
		}
		if len(row.WizardAnswers) > 0 {
			_ = json.Unmarshal(row.WizardAnswers, &c.WizardAnswers)
		}
		results = append(results, c)
	}
	return results, nil
}

// aiDocFromCk maps the sqlc CkAiDocumentation row to the domain AIDocumentation type.
func aiDocFromCk(r db.CkAiDocumentation) AIDocumentation {
	return AIDocumentation{
		ID:                 r.ID,
		OrgID:              r.OrgID,
		AISystemID:         r.AiSystemID,
		Version:            int(r.Version),
		SystemDescription:  r.SystemDescription.String,
		IntendedPurpose:    r.IntendedPurpose.String,
		TrainingData:       r.TrainingData.String,
		DataQuality:        r.DataQuality.String,
		PerformanceMetrics: r.PerformanceMetrics.String,
		SystemLimits:       r.SystemLimits.String,
		RiskManagement:     r.RiskManagement.String,
		HumanOversight:     r.HumanOversight.String,
		LoggingAuditTrail:  r.LoggingAuditTrail.String,
		AuthoredBy:         r.AuthoredBy.String,
		Status:             r.Status,
		CreatedAt:          ckTsToTime(r.CreatedAt),
		UpdatedAt:          ckTsToTime(r.UpdatedAt),
	}
}

// UpsertAIDocumentation inserts or updates (creates a new version) the technical documentation for an AI system.
// Each save creates a new version row; returns the saved document.
func (r *Repository) UpsertAIDocumentation(ctx context.Context, orgID, systemID string, in UpsertAIDocumentationInput) (*AIDocumentation, error) {
	nextVer, err := r.q.NextCKAIDocumentationVersion(ctx, db.NextCKAIDocumentationVersionParams{
		OrgID:      orgID,
		AiSystemID: systemID,
	})
	if err != nil {
		return nil, fmt.Errorf("next ai documentation version: %w", err)
	}
	status := in.Status
	if status == "" {
		status = "draft"
	}
	row, err := r.q.InsertCKAIDocumentation(ctx, db.InsertCKAIDocumentationParams{
		OrgID:              orgID,
		AiSystemID:         systemID,
		Version:            nextVer,
		SystemDescription:  ckOptText(in.SystemDescription),
		IntendedPurpose:    ckOptText(in.IntendedPurpose),
		TrainingData:       ckOptText(in.TrainingData),
		DataQuality:        ckOptText(in.DataQuality),
		PerformanceMetrics: ckOptText(in.PerformanceMetrics),
		SystemLimits:       ckOptText(in.SystemLimits),
		RiskManagement:     ckOptText(in.RiskManagement),
		HumanOversight:     ckOptText(in.HumanOversight),
		LoggingAuditTrail:  ckOptText(in.LoggingAuditTrail),
		AuthoredBy:         ckOptText(in.AuthoredBy),
		Status:             status,
	})
	if err != nil {
		return nil, fmt.Errorf("insert ai documentation: %w", err)
	}
	doc := aiDocFromCk(row)
	return &doc, nil
}

// GetLatestAIDocumentation returns the most recent documentation version for an AI system.
func (r *Repository) GetLatestAIDocumentation(ctx context.Context, orgID, systemID string) (*AIDocumentation, error) {
	row, err := r.q.GetLatestCKAIDocumentation(ctx, db.GetLatestCKAIDocumentationParams{
		OrgID:      orgID,
		AiSystemID: systemID,
	})
	if err != nil {
		return nil, err
	}
	doc := aiDocFromCk(row)
	return &doc, nil
}

// ListAIDocumentationVersions returns all versions of a system's documentation, newest first.
func (r *Repository) ListAIDocumentationVersions(ctx context.Context, orgID, systemID string) ([]AIDocumentation, error) {
	rows, err := r.q.ListCKAIDocumentationVersions(ctx, db.ListCKAIDocumentationVersionsParams{
		OrgID:      orgID,
		AiSystemID: systemID,
	})
	if err != nil {
		return nil, fmt.Errorf("list ai doc versions: %w", err)
	}
	results := make([]AIDocumentation, 0, len(rows))
	for _, row := range rows {
		results = append(results, aiDocFromCk(row))
	}
	return results, nil
}

// GetEUAIActStats returns aggregate counts needed for the EU AI Act dashboard.
func (r *Repository) GetEUAIActStats(ctx context.Context, orgID string) (total int, byRisk map[string]int, byStatus map[string]int, withoutDocs int, err error) {
	byRisk = map[string]int{}
	byStatus = map[string]int{}

	rows, err := r.q.ListCKAISystemsForStats(ctx, orgID)
	if err != nil {
		return 0, nil, nil, 0, fmt.Errorf("get eu ai act stats: %w", err)
	}
	for _, row := range rows {
		total++
		byRisk[row.RiskClass]++
		byStatus[row.Status]++
	}

	count, err := r.q.CountCKAISystemsWithoutDocs(ctx, orgID)
	if err != nil {
		return 0, nil, nil, 0, fmt.Errorf("count ai systems without docs: %w", err)
	}
	withoutDocs = int(count)
	return total, byRisk, byStatus, withoutDocs, nil
}

// --- Maßnahmen-Katalog (control measures) ---

// measureFromCkControlMeasures maps the sqlc Table-Row to the domain ControlMeasure.
func measureFromCkControlMeasures(r db.CkControlMeasures) ControlMeasure {
	return ControlMeasure{
		ID:          r.ID,
		ControlID:   r.ControlID,
		OrgID:       r.OrgID,
		Title:       r.Title,
		Description: r.Description,
		Difficulty:  r.Difficulty,
		StepOrder:   int(r.StepOrder),
		IsBuiltin:   r.IsBuiltin,
		CreatedAt:   ckTsToTime(r.CreatedAt),
	}
}

// ListMeasures returns all measures for a control within an organisation, ordered by step_order.
func (r *Repository) ListMeasures(ctx context.Context, orgID, controlID string) ([]ControlMeasure, error) {
	rows, err := r.q.ListCKMeasures(ctx, db.ListCKMeasuresParams{OrgID: orgID, ControlID: controlID})
	if err != nil {
		return nil, fmt.Errorf("list measures: %w", err)
	}
	out := make([]ControlMeasure, 0, len(rows))
	for _, row := range rows {
		out = append(out, measureFromCkControlMeasures(row))
	}
	return out, nil
}

// CreateMeasure inserts a new measure for a control.
func (r *Repository) CreateMeasure(ctx context.Context, orgID, controlID string, in CreateMeasureInput) (ControlMeasure, error) {
	row, err := r.q.CreateCKMeasure(ctx, db.CreateCKMeasureParams{
		ControlID:   controlID,
		OrgID:       orgID,
		Title:       in.Title,
		Description: in.Description,
		Difficulty:  in.Difficulty,
		StepOrder:   int32(in.StepOrder),
	})
	if err != nil {
		return ControlMeasure{}, fmt.Errorf("create measure: %w", err)
	}
	return measureFromCkControlMeasures(row), nil
}

// UpdateMeasure updates editable fields of a measure.
func (r *Repository) UpdateMeasure(ctx context.Context, orgID, measureID string, in UpdateMeasureInput) (ControlMeasure, error) {
	row, err := r.q.UpdateCKMeasure(ctx, db.UpdateCKMeasureParams{
		ID:          measureID,
		OrgID:       orgID,
		Title:       optTextStrPtr(in.Title),
		Description: optTextStrPtr(in.Description),
		Difficulty:  optTextStrPtr(in.Difficulty),
		StepOrder:   ckOptIntPtr(in.StepOrder),
	})
	if err != nil {
		return ControlMeasure{}, fmt.Errorf("update measure: %w", err)
	}
	return measureFromCkControlMeasures(row), nil
}

// DeleteMeasure removes a non-builtin measure by ID.
func (r *Repository) DeleteMeasure(ctx context.Context, orgID, measureID string) error {
	n, err := r.q.DeleteCKMeasure(ctx, db.DeleteCKMeasureParams{ID: measureID, OrgID: orgID})
	if err != nil {
		return fmt.Errorf("delete measure: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("measure not found or is builtin: %w", ErrNotFound)
	}
	return nil
}

// SeedMeasuresForControl inserts builtin measures for a control, skipping duplicates by title.
func (r *Repository) SeedMeasuresForControl(ctx context.Context, orgID, controlID string, measures []CreateMeasureInput) error {
	for i, m := range measures {
		if err := r.q.SeedCKMeasure(ctx, db.SeedCKMeasureParams{
			ControlID:   controlID,
			OrgID:       orgID,
			Title:       m.Title,
			Description: m.Description,
			Difficulty:  m.Difficulty,
			StepOrder:   int32(i),
		}); err != nil {
			return fmt.Errorf("seed measure %q: %w", m.Title, err)
		}
	}
	return nil
}

type StaleEvidenceControl struct {
	ControlID    string
	ControlTitle string
	DaysSince    int
}

// FindStaleEvidenceControls returns controls where all linked evidence is older than
// olderThanDays days. Controls with no evidence at all are excluded — only controls
// that once had evidence but haven't been updated are returned.
func (r *Repository) FindStaleEvidenceControls(ctx context.Context, orgID string, olderThanDays int) ([]StaleEvidenceControl, error) {
	rows, err := r.db.Query(ctx, `
		SELECT
			c.id::text,
			c.title,
			EXTRACT(EPOCH FROM (NOW() - MAX(e.created_at)))::int / 86400 AS days_since
		FROM ck_controls c
		JOIN ck_evidence e ON e.control_id = c.id
		WHERE c.org_id = $1::uuid
		  AND c.status != 'not_applicable'
		GROUP BY c.id, c.title
		HAVING MAX(e.created_at) < NOW() - ($2 * INTERVAL '1 day')
		ORDER BY days_since DESC
	`, orgID, olderThanDays)
	if err != nil {
		return nil, fmt.Errorf("find stale evidence controls: %w", err)
	}
	defer rows.Close()

	var results []StaleEvidenceControl
	for rows.Next() {
		var row StaleEvidenceControl
		if err := rows.Scan(&row.ControlID, &row.ControlTitle, &row.DaysSince); err != nil {
			return nil, fmt.Errorf("scan stale evidence control: %w", err)
		}
		results = append(results, row)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate stale evidence controls: %w", err)
	}
	return results, nil
}

// UpsertAIInsight inserts an AI insight, skipping duplicates where the same org,
// type, and control within the last 24 hours already exists.
func (r *Repository) UpsertAIInsight(
	ctx context.Context,
	orgID, insightType, title, message string,
	controlID, riskID, findingID *string,
	urgency int,
	metadata json.RawMessage,
) error {
	// Deduplication: skip if an identical (org+type+control) insight was created within 24h.
	var existing int
	err := r.db.QueryRow(ctx, `
		SELECT COUNT(*) FROM ck_ai_insights
		WHERE org_id = $1::uuid
		  AND type = $2
		  AND ($3::uuid IS NULL OR control_id = $3::uuid)
		  AND created_at > NOW() - INTERVAL '24 hours'
		  AND dismissed_at IS NULL
	`, orgID, insightType, controlID).Scan(&existing)
	if err != nil {
		return fmt.Errorf("upsert ai insight dedup check: %w", err)
	}
	if existing > 0 {
		return nil
	}

	_, err = r.db.Exec(ctx, `
		INSERT INTO ck_ai_insights
			(org_id, type, title, message, control_id, risk_id, finding_id, urgency, metadata)
		VALUES
			($1::uuid, $2, $3, $4, $5::uuid, $6::uuid, $7::uuid, $8, $9)
	`,
		orgID,
		insightType,
		title,
		message,
		controlID,
		riskID,
		findingID,
		urgency,
		nullableJSON(metadata),
	)
	if err != nil {
		return fmt.Errorf("insert ai insight: %w", err)
	}
	return nil
}

// AIInsight is a single insight record returned from ListActiveAIInsights.
type AIInsight struct {
	ID        string
	Type      string
	Title     string
	Message   string
	ControlID *string
	RiskID    *string
	FindingID *string
	Urgency   int
	CreatedAt time.Time
}

// ListActiveAIInsights returns up to 5 active (non-dismissed) insights for an org,
// ordered by urgency ascending (1=high first) then by creation date descending.
func (r *Repository) ListActiveAIInsights(ctx context.Context, orgID string) ([]AIInsight, error) {
	rows, err := r.db.Query(ctx, `
		SELECT
			id::text,
			type,
			title,
			message,
			control_id::text,
			risk_id::text,
			finding_id::text,
			urgency,
			created_at
		FROM ck_ai_insights
		WHERE org_id = $1::uuid
		  AND dismissed_at IS NULL
		ORDER BY urgency ASC, created_at DESC
		LIMIT 5
	`, orgID)
	if err != nil {
		return nil, fmt.Errorf("list active ai insights: %w", err)
	}
	defer rows.Close()

	var results []AIInsight
	for rows.Next() {
		var insight AIInsight
		var controlID, riskID, findingID *string
		if err := rows.Scan(
			&insight.ID,
			&insight.Type,
			&insight.Title,
			&insight.Message,
			&controlID,
			&riskID,
			&findingID,
			&insight.Urgency,
			&insight.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan ai insight: %w", err)
		}
		insight.ControlID = controlID
		insight.RiskID = riskID
		insight.FindingID = findingID
		results = append(results, insight)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate ai insights: %w", err)
	}
	return results, nil
}

// DismissAIInsight sets dismissed_at for the given insight, scoped to the org.
// Returns an error if the insight does not exist or belongs to a different org.
func (r *Repository) DismissAIInsight(ctx context.Context, orgID, insightID string) error {
	tag, err := r.db.Exec(ctx, `
		UPDATE ck_ai_insights
		SET dismissed_at = NOW()
		WHERE id = $1::uuid AND org_id = $2::uuid AND dismissed_at IS NULL
	`, insightID, orgID)
	if err != nil {
		return fmt.Errorf("dismiss ai insight: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("insight not found or already dismissed")
	}
	return nil
}

// nullableJSON returns nil when the RawMessage is empty, so the DB column stores NULL.
func nullableJSON(m json.RawMessage) any {
	if len(m) == 0 {
		return nil
	}
	return []byte(m)
}
