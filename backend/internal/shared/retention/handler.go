package retention

import (
	"net/http"

	"github.com/go-playground/validator/v10"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog/log"
)

// Handler handles HTTP requests for retention-config endpoints.
type Handler struct {
	db       *pgxpool.Pool
	validate *validator.Validate
}

// NewHandler constructs a Handler.
func NewHandler(db *pgxpool.Pool) *Handler {
	return &Handler{db: db, validate: validator.New()}
}

func orgIDFromCtx(c echo.Context) (string, bool) {
	orgID, ok := c.Get("org_id").(string)
	return orgID, ok && orgID != ""
}

func retentionErr(c echo.Context, code int, msg, errCode string) error {
	return c.JSON(code, map[string]string{"error": msg, "code": errCode})
}

// GetConfig handles GET /retention/config.
func (h *Handler) GetConfig(c echo.Context) error {
	orgID, ok := orgIDFromCtx(c)
	if !ok {
		return retentionErr(c, http.StatusUnauthorized, "unauthorized", "RETENTION_UNAUTHORIZED")
	}

	cfg, err := GetConfig(c.Request().Context(), h.db, orgID)
	if err != nil {
		log.Error().Err(err).Str("org_id", orgID).Msg("retention: get config")
		return retentionErr(c, http.StatusInternalServerError, "failed to load config", "RETENTION_GET_FAILED")
	}
	return c.JSON(http.StatusOK, cfg)
}

// UpdateConfig handles PUT /retention/config.
func (h *Handler) UpdateConfig(c echo.Context) error {
	orgID, ok := orgIDFromCtx(c)
	if !ok {
		return retentionErr(c, http.StatusUnauthorized, "unauthorized", "RETENTION_UNAUTHORIZED")
	}

	var in UpdateRetentionConfigInput
	if err := c.Bind(&in); err != nil {
		return retentionErr(c, http.StatusBadRequest, "invalid request body", "RETENTION_INVALID_BODY")
	}
	if err := h.validate.Struct(in); err != nil {
		return retentionErr(c, http.StatusUnprocessableEntity, err.Error(), "RETENTION_VALIDATION_ERROR")
	}

	// Fetch current config as base, apply partial updates.
	current, err := GetConfig(c.Request().Context(), h.db, orgID)
	if err != nil {
		log.Error().Err(err).Str("org_id", orgID).Msg("retention: load current config")
		return retentionErr(c, http.StatusInternalServerError, "failed to load config", "RETENTION_GET_FAILED")
	}

	if in.AuditLogDays != nil {
		current.AuditLogDays = *in.AuditLogDays
	}
	if in.FindingsResolvedDays != nil {
		current.FindingsResolvedDays = *in.FindingsResolvedDays
	}
	if in.NotificationsDays != nil {
		current.NotificationsDays = *in.NotificationsDays
	}
	if in.ScanHistoryDays != nil {
		current.ScanHistoryDays = *in.ScanHistoryDays
	}
	if in.DigestEnabled != nil {
		current.DigestEnabled = *in.DigestEnabled
	}
	if in.DigestHour != nil {
		current.DigestHour = *in.DigestHour
	}

	updated, err := UpsertConfig(c.Request().Context(), h.db, orgID, *current)
	if err != nil {
		log.Error().Err(err).Str("org_id", orgID).Msg("retention: upsert config")
		return retentionErr(c, http.StatusInternalServerError, "failed to update config", "RETENTION_UPDATE_FAILED")
	}
	return c.JSON(http.StatusOK, updated)
}
