package vaktscan

import (
	"fmt"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog/log"
)

// wazuhAlert represents a single alert received from the Wazuh API.
type wazuhAlert struct {
	Rule      wazuhRule  `json:"rule"`
	Agent     wazuhAgent `json:"agent"`
	Timestamp string     `json:"timestamp"`
}

type wazuhRule struct {
	Level       int    `json:"level"`
	Description string `json:"description"`
	ID          string `json:"id"`
}

type wazuhAgent struct {
	Name string `json:"name"`
	IP   string `json:"ip"`
}

// ImportWazuh handles POST /api/v1/vaktscan/import/wazuh.
//
// Accepts either a single Wazuh alert object or a JSON array of alert objects.
// Query params:
//
//	?asset_id=<uuid>  optional — if absent, attempts to match by agent.name
func (h *Handler) ImportWazuh(c echo.Context) error {
	orgID, _ := c.Get("org_id").(string)
	assetID := c.QueryParam("asset_id")

	// Decode the body: try array first, then single object.
	alerts, err := bindWazuhAlerts(c)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "invalid request body: " + err.Error(),
			"code":  "VB_BAD_REQUEST",
		})
	}
	if len(alerts) == 0 {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "no alerts provided",
			"code":  "VB_BAD_REQUEST",
		})
	}

	ctx := c.Request().Context()
	cfg, _ := h.service.repo.GetSLAConfig(ctx, orgID)

	imported := 0
	skipped := 0

	for _, alert := range alerts {
		// Resolve asset: prefer explicit query param, otherwise match by agent name.
		resolvedAssetID := assetID
		if resolvedAssetID == "" {
			asset, lookupErr := h.service.repo.GetAssetByName(ctx, orgID, alert.Agent.Name)
			if lookupErr != nil || asset == nil {
				log.Warn().
					Str("org_id", orgID).
					Str("agent_name", alert.Agent.Name).
					Msg("wazuh import: could not resolve asset by agent name, skipping alert")
				skipped++
				continue
			}
			resolvedAssetID = asset.ID
		}

		severity := wazuhLevelToSeverity(alert.Rule.Level)
		title := alert.Rule.Description
		if title == "" {
			title = fmt.Sprintf("Wazuh Rule %s", alert.Rule.ID)
		}
		description := fmt.Sprintf(
			"Wazuh Rule %s: %s. Agent: %s (%s)",
			alert.Rule.ID, alert.Rule.Description,
			alert.Agent.Name, alert.Agent.IP,
		)
		rawID := fmt.Sprintf("wazuh-%s-%s", alert.Rule.ID, alert.Agent.Name)

		slaDueAt := calcSLADueAt(cfg, severity)

		f := Finding{
			OrgID:       orgID,
			AssetID:     resolvedAssetID,
			Title:       truncate(title, 200),
			Description: description,
			Severity:    severity,
			Status:      "open",
			Scanner:     "wazuh",
			RawID:       rawID,
			SLADueAt:    slaDueAt,
		}

		if _, upsertErr := h.service.repo.UpsertFindingByRawID(ctx, orgID, f); upsertErr != nil {
			log.Error().Err(upsertErr).
				Str("org_id", orgID).
				Str("raw_id", rawID).
				Msg("wazuh import: upsert finding failed")
			skipped++
			continue
		}
		imported++
	}

	log.Info().
		Str("org_id", orgID).
		Int("imported", imported).
		Int("skipped", skipped).
		Msg("wazuh import complete")

	return c.JSON(http.StatusOK, map[string]int{
		"imported": imported,
		"skipped":  skipped,
	})
}

// bindWazuhAlerts decodes the request body as either a JSON array or a single object.
func bindWazuhAlerts(c echo.Context) ([]wazuhAlert, error) {
	// We try to decode as []wazuhAlert first; if that fails, try a single wazuhAlert.
	var alerts []wazuhAlert
	if err := c.Bind(&alerts); err == nil && len(alerts) > 0 {
		return alerts, nil
	}

	var single wazuhAlert
	if err := (&echo.DefaultBinder{}).BindBody(c, &single); err != nil {
		return nil, err
	}
	if single.Rule.ID == "" && single.Rule.Description == "" {
		return nil, fmt.Errorf("could not parse body as Wazuh alert object or array")
	}
	return []wazuhAlert{single}, nil
}

// wazuhLevelToSeverity maps a Wazuh rule level (1-15) to a SecPulse severity string.
func wazuhLevelToSeverity(level int) string {
	switch {
	case level <= 3:
		return "info"
	case level <= 7:
		return "low"
	case level <= 11:
		return "medium"
	case level <= 14:
		return "high"
	default:
		return "critical"
	}
}

// truncate returns s truncated to at most max runes.
func truncate(s string, max int) string {
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return string(runes[:max])
}
