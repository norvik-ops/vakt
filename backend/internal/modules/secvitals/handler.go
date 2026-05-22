package secvitals

import (
	"github.com/go-playground/validator/v10"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"

	"github.com/matharnica/vakt/internal/db"
)

// Handler handles HTTP requests for ComplyKit.
type Handler struct {
	service       *Service
	validate      *validator.Validate
	uploadDir     string
	db            *pgxpool.Pool
	q             *db.Queries
	paCfg         PolicyAcceptanceHandlerConfig
	evidenceFiles *EvidenceFileService
}

// NewHandler creates a new ComplyKit handler.
func NewHandler(service *Service) *Handler {
	return &Handler{
		service:  service,
		validate: validator.New(),
	}
}

// WithDB attaches a DB pool used for audit logging.
func (h *Handler) WithDB(dbPool *pgxpool.Pool) *Handler {
	h.db = dbPool
	h.q = db.New(dbPool)
	return h
}

// orgID extracts the authenticated organisation ID from the Echo context.
func orgID(c echo.Context) string {
	v, _ := c.Get("org_id").(string)
	return v
}

// userID extracts the authenticated user ID from the Echo context.
func userID(c echo.Context) string {
	v, _ := c.Get("user_id").(string)
	return v
}

// errResp returns a standardised JSON error response.
func errResp(c echo.Context, code int, msg, errCode string) error {
	return c.JSON(code, map[string]string{
		"error": msg,
		"code":  errCode,
	})
}
