// Package onboarding provides endpoints that drive the post-login onboarding
// wizard shown to new users.  Both handlers are pure SQL and do not import
// from any module package, keeping cross-module isolation intact.
package onboarding

import (
	"errors"
	"net/http"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog/log"
)

// StatusResponse is the JSON payload returned by GET /api/v1/onboarding/status.
type StatusResponse struct {
	Completed bool  `json:"completed"`
	Dismissed bool  `json:"dismissed"`
	Steps     Steps `json:"steps"`
}

// Steps holds the individual completion flags for each wizard step.
type Steps struct {
	OrgConfigured        bool `json:"org_configured"`
	FrameworkSelected    bool `json:"framework_selected"`
	FirstControlReviewed bool `json:"first_control_reviewed"`
	FirstRiskCreated     bool `json:"first_risk_created"`
}

// GetStatus handles GET /api/v1/onboarding/status.
// It checks four lightweight conditions and returns a completion summary.
func GetStatus(db *pgxpool.Pool) echo.HandlerFunc {
	return func(c echo.Context) error {
		orgID, ok := c.Get("org_id").(string)
		if !ok || orgID == "" {
			return c.JSON(http.StatusUnauthorized, map[string]string{
				"error": "unauthorized",
				"code":  "AUTH_MISSING_ORG",
			})
		}
		ctx := c.Request().Context()

		var steps Steps
		var dismissed bool

		// org_configured: org name is non-empty; also load dismissed flag.
		var orgName string
		err := db.QueryRow(ctx,
			`SELECT name, COALESCE(onboarding_dismissed, false)
			   FROM organizations WHERE id = $1::uuid`, orgID).
			Scan(&orgName, &dismissed)
		if err != nil {
			if !errors.Is(err, pgx.ErrNoRows) {
				log.Error().Err(err).Str("org_id", orgID).Msg("onboarding: query org")
				return c.JSON(http.StatusInternalServerError, map[string]string{
					"error": "failed to fetch onboarding status",
					"code":  "ONBOARDING_FETCH_FAILED",
				})
			}
		}
		steps.OrgConfigured = orgName != ""

		// framework_selected: at least one framework row for this org.
		var fwCount int64
		if err := db.QueryRow(ctx,
			`SELECT COUNT(*) FROM ck_frameworks WHERE org_id = $1::uuid`, orgID).
			Scan(&fwCount); err != nil {
			log.Error().Err(err).Msg("onboarding: count frameworks")
		}
		steps.FrameworkSelected = fwCount > 0

		// first_control_reviewed: at least one control review row for this org.
		var reviewCount int64
		if err := db.QueryRow(ctx,
			`SELECT COUNT(*) FROM ck_control_reviews WHERE org_id = $1::uuid`, orgID).
			Scan(&reviewCount); err != nil {
			log.Error().Err(err).Msg("onboarding: count control reviews")
		}
		steps.FirstControlReviewed = reviewCount > 0

		// first_risk_created: at least one risk row for this org.
		var riskCount int64
		if err := db.QueryRow(ctx,
			`SELECT COUNT(*) FROM ck_risks WHERE org_id = $1::uuid`, orgID).
			Scan(&riskCount); err != nil {
			log.Error().Err(err).Msg("onboarding: count risks")
		}
		steps.FirstRiskCreated = riskCount > 0

		completed := steps.OrgConfigured &&
			steps.FrameworkSelected &&
			steps.FirstControlReviewed &&
			steps.FirstRiskCreated

		return c.JSON(http.StatusOK, StatusResponse{
			Completed: completed,
			Dismissed: dismissed,
			Steps:     steps,
		})
	}
}

// GetProgressHandler handles GET /api/v1/onboarding/progress — the S89-5 guided
// "first 30 days" path with 7 steps whose completion is derived from real data.
func GetProgressHandler(db *pgxpool.Pool) echo.HandlerFunc {
	return func(c echo.Context) error {
		orgID, ok := c.Get("org_id").(string)
		if !ok || orgID == "" {
			return c.JSON(http.StatusUnauthorized, map[string]string{
				"error": "unauthorized",
				"code":  "AUTH_MISSING_ORG",
			})
		}
		p, err := GetProgress(c.Request().Context(), db, orgID)
		if err != nil {
			log.Error().Err(err).Str("org_id", orgID).Msg("onboarding: get progress")
			return c.JSON(http.StatusInternalServerError, map[string]string{
				"error": "failed to load onboarding progress",
				"code":  "ONBOARDING_PROGRESS_FAILED",
			})
		}
		return c.JSON(http.StatusOK, p)
	}
}

// Dismiss handles POST /api/v1/onboarding/dismiss.
// It sets onboarding_dismissed = true on the organisation row so the wizard
// no longer appears for any user in this org.
func Dismiss(db *pgxpool.Pool) echo.HandlerFunc {
	return func(c echo.Context) error {
		orgID, ok := c.Get("org_id").(string)
		if !ok || orgID == "" {
			return c.JSON(http.StatusUnauthorized, map[string]string{
				"error": "unauthorized",
				"code":  "AUTH_MISSING_ORG",
			})
		}

		_, err := db.Exec(c.Request().Context(),
			`UPDATE organizations SET onboarding_dismissed = true WHERE id = $1::uuid`, orgID)
		if err != nil {
			log.Error().Err(err).Str("org_id", orgID).Msg("onboarding: dismiss")
			return c.JSON(http.StatusInternalServerError, map[string]string{
				"error": "failed to dismiss onboarding",
				"code":  "ONBOARDING_DISMISS_FAILED",
			})
		}

		return c.NoContent(http.StatusNoContent)
	}
}
