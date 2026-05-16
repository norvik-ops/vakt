package setup

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/go-playground/validator/v10"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog/log"
	"golang.org/x/crypto/bcrypt"
)

// Handler holds HTTP handler methods for the setup endpoints.
type Handler struct {
	db       *pgxpool.Pool
	validate *validator.Validate
}

// NewHandler constructs a setup Handler.
func NewHandler(db *pgxpool.Pool) *Handler {
	return &Handler{
		db:       db,
		validate: validator.New(),
	}
}

// SetupInput is the request body for POST /api/v1/setup.
type SetupInput struct {
	OrgName        string   `json:"org_name"        validate:"required,min=2,max=120"`
	AdminEmail     string   `json:"admin_email"     validate:"required,email"`
	AdminPassword  string   `json:"admin_password"  validate:"required,min=8,max=72"`
	ModulesEnabled []string `json:"modules_enabled"`
	SMTPHost       string   `json:"smtp_host"`
	SMTPPort       string   `json:"smtp_port"`
}

// SetupResponse is returned on successful first-run setup.
type SetupResponse struct {
	OrgID   string `json:"org_id"`
	UserID  string `json:"user_id"`
	Message string `json:"message"`
}

// StatusResponse is returned by GET /api/v1/setup/status.
type StatusResponse struct {
	Complete bool `json:"setup_complete"`
}

// GetStatus handles GET /api/v1/setup/status.
// It returns whether initial setup has been completed, allowing the frontend
// to decide whether to redirect the user to /setup.
func (h *Handler) GetStatus(c echo.Context) error {
	complete, err := IsSetupComplete(c.Request().Context(), h.db)
	if err != nil {
		log.Error().Err(err).Msg("setup status check failed")
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "failed to check setup status",
			"code":  "SETUP_STATUS_ERROR",
		})
	}
	return c.JSON(http.StatusOK, StatusResponse{Complete: complete})
}

// PostSetup handles POST /api/v1/setup.
// It is idempotent-safe: returns 409 if setup is already complete.
func (h *Handler) PostSetup(c echo.Context) error {
	ctx := c.Request().Context()

	// Guard: reject if already initialised.
	complete, err := IsSetupComplete(ctx, h.db)
	if err != nil {
		log.Error().Err(err).Msg("setup status check failed")
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "failed to check setup status",
			"code":  "SETUP_STATUS_ERROR",
		})
	}
	if complete {
		return c.JSON(http.StatusConflict, map[string]string{
			"error": "setup has already been completed",
			"code":  "SETUP_ALREADY_COMPLETE",
		})
	}

	var input SetupInput
	if err := c.Bind(&input); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "invalid request body",
			"code":  "SETUP_BAD_REQUEST",
		})
	}
	if err := h.validate.Struct(input); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, map[string]string{
			"error": err.Error(),
			"code":  "SETUP_VALIDATION_ERROR",
		})
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(input.AdminPassword), 12)
	if err != nil {
		log.Error().Err(err).Msg("hash password")
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "internal error",
			"code":  "SETUP_INTERNAL_ERROR",
		})
	}

	tx, err := h.db.Begin(ctx)
	if err != nil {
		log.Error().Err(err).Msg("begin transaction")
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "internal error",
			"code":  "SETUP_INTERNAL_ERROR",
		})
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback(ctx)
		}
	}()

	// Insert organization.
	slug := slugify(input.OrgName)
	var orgID string
	err = tx.QueryRow(ctx, `
		INSERT INTO organizations (name, slug)
		VALUES ($1, $2)
		RETURNING id::text`,
		input.OrgName, slug,
	).Scan(&orgID)
	if err != nil {
		log.Error().Err(err).Msg("insert organization")
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "failed to create organization",
			"code":  "SETUP_ORG_ERROR",
		})
	}

	// Insert admin user.
	var userID string
	err = tx.QueryRow(ctx, `
		INSERT INTO users (email, password_hash, display_name)
		VALUES ($1, $2, 'Admin')
		RETURNING id::text`,
		input.AdminEmail, string(hash),
	).Scan(&userID)
	if err != nil {
		log.Error().Err(err).Msg("insert admin user")
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "failed to create admin user",
			"code":  "SETUP_USER_ERROR",
		})
	}

	// Lookup Admin role.
	var roleID string
	err = tx.QueryRow(ctx, `SELECT id::text FROM roles WHERE name = 'Admin'`).Scan(&roleID)
	if err != nil {
		log.Error().Err(err).Msg("lookup admin role")
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "admin role not found — ensure migrations have run",
			"code":  "SETUP_ROLE_ERROR",
		})
	}

	// Link admin user to org.
	_, err = tx.Exec(ctx, `
		INSERT INTO org_members (org_id, user_id, role_id)
		VALUES ($1::uuid, $2::uuid, $3::uuid)`,
		orgID, userID, roleID,
	)
	if err != nil {
		log.Error().Err(err).Msg("insert org member")
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "failed to assign admin role",
			"code":  "SETUP_MEMBER_ERROR",
		})
	}

	if err = tx.Commit(ctx); err != nil {
		log.Error().Err(err).Msg("commit transaction")
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "internal error",
			"code":  "SETUP_INTERNAL_ERROR",
		})
	}

	log.Info().
		Str("org_id", orgID).
		Str("user_id", userID).
		Str("org_name", input.OrgName).
		Msg("first-run setup complete")

	return c.JSON(http.StatusCreated, SetupResponse{
		OrgID:   orgID,
		UserID:  userID,
		Message: fmt.Sprintf("Setup complete. Organization %q created with admin account.", input.OrgName),
	})
}

// Register attaches setup routes to the provided Echo group.
func Register(g *echo.Group, h *Handler) {
	g.GET("/status", h.GetStatus)
	g.POST("", h.PostSetup)
}

// slugify converts a display name to a URL-safe slug.
func slugify(s string) string {
	s = strings.ToLower(s)
	s = strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			return r
		}
		if r == ' ' || r == '_' {
			return '-'
		}
		return -1
	}, s)
	for strings.Contains(s, "--") {
		s = strings.ReplaceAll(s, "--", "-")
	}
	return strings.Trim(s, "-")
}
