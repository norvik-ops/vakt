package vaktcomply

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// StaleEvidenceControl is a control where all linked evidence is older than a threshold.
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
func nullableJSON(m json.RawMessage) interface{} {
	if len(m) == 0 {
		return nil
	}
	return []byte(m)
}
