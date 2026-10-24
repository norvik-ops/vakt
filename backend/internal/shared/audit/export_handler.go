package audit

import (
	"fmt"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog/log"
)

// Handler handles HTTP requests for the audit export endpoint.
type ExportHandler struct {
	db *pgxpool.Pool
}

// NewHandler creates a new Handler.
func NewExportHandler(db *pgxpool.Pool) *ExportHandler {
	return &ExportHandler{db: db}
}

// Export handles GET /export/audit-package.
// It generates a ZIP containing all compliance data and streams it to the client.
func (h *ExportHandler) Export(c echo.Context) error {
	orgID, ok := c.Get("org_id").(string)
	if !ok || orgID == "" {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
	}

	pkg, err := GeneratePackage(c.Request().Context(), h.db, orgID)
	if err != nil {
		log.Error().Err(err).Str("org_id", orgID).Msg("audit export: generate package")
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "export failed"})
	}

	filename := fmt.Sprintf("audit-paket-%s.zip", time.Now().Format("2006-01-02"))
	c.Response().Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename=%q`, filename))
	c.Response().Header().Set("Content-Type", "application/zip")
	c.Response().Header().Set("Content-Length", fmt.Sprint(len(pkg.Zip)))
	return c.Blob(http.StatusOK, "application/zip", pkg.Zip)
}
