package evidence_auto

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog/log"
)

// AutoEvidence is the response shape for a single auto-collected evidence item.
type AutoEvidence struct {
	ID                   string     `json:"id"`
	OrgID                string     `json:"org_id"`
	Title                string     `json:"title"`
	Description          string     `json:"description,omitempty"`
	AutoSourceType       string     `json:"auto_source_type"`
	AutoSourceRef        string     `json:"auto_source_ref,omitempty"`
	AutoCollectedAt      *time.Time `json:"auto_collected_at,omitempty"`
	CreatedAt            time.Time  `json:"created_at"`
	SuggestedControlHint string     `json:"suggested_control_hint,omitempty"`
}

// hrControlHint returns a suggested control text for HR checklist completions.
// Uses collector_data.checklist_type to determine onboarding vs offboarding.
func hrControlHint(collectorData []byte) string {
	if len(collectorData) == 0 {
		return ""
	}
	var data struct {
		ChecklistType string `json:"checklist_type"`
	}
	if err := json.Unmarshal(collectorData, &data); err != nil {
		return ""
	}
	switch data.ChecklistType {
	case "onboarding":
		return "ISO 27001 A.6.1 Überprüfung von Bewerbern / NIS2 Art. 21 Zugangsmanagement"
	case "offboarding":
		return "ISO 27001 A.6.5 Verantwortlichkeiten beim Ausscheiden / NIS2 Art. 21 Zugangsmanagement"
	default:
		return "ISO 27001 A.6 Personenbezogene Maßnahmen"
	}
}

// Handler handles HTTP requests for auto-evidence endpoints.
type Handler struct {
	db *pgxpool.Pool
}

func NewHandler(db *pgxpool.Pool) *Handler {
	return &Handler{db: db}
}

// ListAutoEvidence handles GET /vaktcomply/evidence/auto.
// Returns all unassigned auto-collected evidence (control_id IS NULL AND auto_source_type IS NOT NULL).
func (h *Handler) ListAutoEvidence(c echo.Context) error {
	orgID, _ := c.Get("org_id").(string)
	if orgID == "" {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthorized", "code": "UNAUTHORIZED"})
	}

	rows, err := h.db.Query(c.Request().Context(), `
		SELECT id::text, org_id::text, title, COALESCE(description,''),
		       auto_source_type, COALESCE(auto_source_ref,''), auto_collected_at, created_at,
		       COALESCE(collector_data, '{}'::jsonb)
		FROM ck_evidence
		WHERE org_id = $1::uuid
		  AND control_id IS NULL
		  AND auto_source_type IS NOT NULL
		ORDER BY auto_collected_at DESC NULLS LAST, created_at DESC`,
		orgID,
	)
	if err != nil {
		log.Error().Err(err).Msg("list auto evidence")
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "failed to list auto evidence",
			"code":  "EA_LIST_FAILED",
		})
	}
	defer rows.Close()

	items := make([]AutoEvidence, 0)
	for rows.Next() {
		var ev AutoEvidence
		var collectorData []byte
		if err := rows.Scan(
			&ev.ID, &ev.OrgID, &ev.Title, &ev.Description,
			&ev.AutoSourceType, &ev.AutoSourceRef, &ev.AutoCollectedAt, &ev.CreatedAt,
			&collectorData,
		); err != nil {
			log.Error().Err(err).Msg("scan auto evidence row")
			continue
		}
		if ev.AutoSourceType == "vakthr" {
			ev.SuggestedControlHint = hrControlHint(collectorData)
		}
		items = append(items, ev)
	}
	if err := rows.Err(); err != nil {
		log.Error().Err(err).Msg("iterate auto evidence rows")
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "failed to list auto evidence",
			"code":  "EA_LIST_FAILED",
		})
	}

	return c.JSON(http.StatusOK, items)
}

// AssignAutoEvidence handles POST /vaktcomply/evidence/auto/:id/assign.
// Body: {"control_id": "<uuid>"}. Sets control_id on the evidence row.
func (h *Handler) AssignAutoEvidence(c echo.Context) error {
	orgID, _ := c.Get("org_id").(string)
	if orgID == "" {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthorized", "code": "UNAUTHORIZED"})
	}

	evidenceID := c.Param("id")
	if evidenceID == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "evidence id required", "code": "EA_BAD_REQUEST"})
	}

	var body struct {
		ControlID string `json:"control_id"`
	}
	if err := c.Bind(&body); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request body", "code": "EA_BAD_REQUEST"})
	}
	if body.ControlID == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "control_id is required", "code": "EA_BAD_REQUEST"})
	}

	// Verify the control belongs to this org before assigning.
	var exists bool
	err := h.db.QueryRow(c.Request().Context(), `
		SELECT EXISTS(SELECT 1 FROM ck_controls WHERE id = $1::uuid AND org_id = $2::uuid)`,
		body.ControlID, orgID,
	).Scan(&exists)
	if err != nil || !exists {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "control not found", "code": "EA_CONTROL_NOT_FOUND"})
	}

	tag, err := h.db.Exec(c.Request().Context(), `
		UPDATE ck_evidence
		SET control_id = $1::uuid, updated_at = NOW()
		WHERE id = $2::uuid AND org_id = $3::uuid
		  AND control_id IS NULL AND auto_source_type IS NOT NULL`,
		body.ControlID, evidenceID, orgID,
	)
	if err != nil {
		log.Error().Err(err).Str("evidence_id", evidenceID).Msg("assign auto evidence")
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "failed to assign evidence",
			"code":  "EA_ASSIGN_FAILED",
		})
	}
	if tag.RowsAffected() == 0 {
		return c.JSON(http.StatusNotFound, map[string]string{
			"error": "evidence not found or already assigned",
			"code":  "EA_NOT_FOUND",
		})
	}

	return c.JSON(http.StatusOK, map[string]string{"status": "assigned"})
}
