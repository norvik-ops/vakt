// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package cloud

import (
	"net/http"

	"github.com/go-playground/validator/v10"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog/log"
)

var validate = validator.New()

// Handler handles HTTP requests for cloud integrations (AWS + Azure).
type Handler struct {
	svc *Service
}

// NewHandler creates a new cloud integration handler.
func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// RegisterRoutes wires cloud integration routes under the provided echo group.
// Expected group prefix: /integrations/cloud
// Returns the Service so callers can inject it into other handlers (e.g., the vakthr Personio webhook).
func RegisterRoutes(g *echo.Group, db *pgxpool.Pool, masterKey []byte, evidence EvidenceWriter) *Service {
	svc := NewService(db, masterKey, evidence)
	h := NewHandler(svc)

	// AWS
	aws := g.Group("/aws")
	aws.GET("/config", h.GetAWSConfig)
	aws.PUT("/config", h.SaveAWSConfig)
	aws.POST("/test", h.TestAWSConnection)
	aws.POST("/sync", h.SyncAWS)
	aws.GET("/status", h.GetAWSStatus)
	aws.GET("/evidence", h.GetAWSEvidence)

	// Azure
	az := g.Group("/azure")
	az.GET("/config", h.GetAzureConfig)
	az.PUT("/config", h.SaveAzureConfig)
	az.POST("/test", h.TestAzureConnection)
	az.POST("/sync", h.SyncAzure)
	az.GET("/status", h.GetAzureStatus)
	az.GET("/evidence", h.GetAzureEvidence)

	// Hetzner
	hz := g.Group("/hetzner")
	hz.GET("/config", h.GetHetznerConfig)
	hz.PUT("/config", h.SaveHetznerConfig)
	hz.POST("/sync", h.SyncHetzner)
	hz.GET("/status", h.GetHetznerStatus)
	hz.GET("/evidence", h.GetHetznerEvidence)

	// IONOS
	io := g.Group("/ionos")
	io.GET("/config", h.GetIONOSConfig)
	io.PUT("/config", h.SaveIONOSConfig)
	io.POST("/sync", h.SyncIONOS)
	io.GET("/status", h.GetIONOSStatus)
	io.GET("/evidence", h.GetIONOSEvidence)

	// Wazuh
	wz := g.Group("/wazuh")
	wz.GET("/config", h.GetWazuhConfig)
	wz.PUT("/config", h.SaveWazuhConfig)
	wz.POST("/sync", h.SyncWazuh)
	wz.GET("/status", h.GetWazuhStatus)
	wz.GET("/evidence", h.GetWazuhEvidence)

	// Prometheus
	pr := g.Group("/prometheus")
	pr.GET("/config", h.GetPrometheusConfig)
	pr.PUT("/config", h.SavePrometheusConfig)
	pr.POST("/sync", h.SyncPrometheus)
	pr.GET("/status", h.GetPrometheusStatus)
	pr.GET("/evidence", h.GetPrometheusEvidence)

	// Entra ID (Microsoft Graph API)
	ei := g.Group("/entra-id")
	ei.GET("/config", h.GetEntraIDConfig)
	ei.PUT("/config", h.SaveEntraIDConfig)
	ei.POST("/sync", h.SyncEntraID)
	ei.GET("/status", h.GetEntraIDStatus)
	ei.GET("/evidence", h.GetEntraIDEvidence)

	// Intune (Microsoft Graph API — MDM device posture)
	in := g.Group("/intune")
	in.GET("/config", h.GetIntuneConfig)
	in.PUT("/config", h.SaveIntuneConfig)
	in.POST("/sync", h.SyncIntune)
	in.GET("/status", h.GetIntuneStatus)
	in.GET("/evidence", h.GetIntuneEvidence)

	// Keycloak
	kc := g.Group("/keycloak")
	kc.GET("/config", h.GetKeycloakConfig)
	kc.PUT("/config", h.SaveKeycloakConfig)
	kc.POST("/sync", h.SyncKeycloak)
	kc.GET("/status", h.GetKeycloakStatus)
	kc.GET("/evidence", h.GetKeycloakEvidence)

	// LDAP / Active Directory
	ld := g.Group("/ldap")
	ld.GET("/config", h.GetLDAPConfig)
	ld.PUT("/config", h.SaveLDAPConfig)
	ld.POST("/sync", h.SyncLDAP)
	ld.GET("/status", h.GetLDAPStatus)
	ld.GET("/evidence", h.GetLDAPEvidence)

	// GitLab (self-managed + GitLab.com)
	gl := g.Group("/gitlab")
	gl.GET("/config", h.GetGitLabConfig)
	gl.PUT("/config", h.SaveGitLabConfig)
	gl.POST("/sync", h.SyncGitLab)
	gl.GET("/status", h.GetGitLabStatus)
	gl.GET("/evidence", h.GetGitLabEvidence)

	// SonarQube / SonarCloud
	sq := g.Group("/sonarqube")
	sq.GET("/config", h.GetSonarQubeConfig)
	sq.PUT("/config", h.SaveSonarQubeConfig)
	sq.POST("/sync", h.SyncSonarQube)
	sq.GET("/status", h.GetSonarQubeStatus)
	sq.GET("/evidence", h.GetSonarQubeEvidence)

	// Personio (push-only via webhook — no sync endpoint)
	pe := g.Group("/personio")
	pe.GET("/config", h.GetPersonioConfig)
	pe.PUT("/config", h.SavePersonioConfig)
	pe.GET("/status", h.GetPersonioStatus)

	return svc
}

// --- AWS handlers ---

// GetAWSConfig returns the AWS config with secrets masked.
func (h *Handler) GetAWSConfig(c echo.Context) error {
	orgID := mustString(c, "org_id")
	cfg, err := h.svc.GetAWSConfig(c.Request().Context(), orgID)
	if err != nil {
		log.Error().Err(err).Msg("GetAWSConfig failed")
		return serverError(c, err)
	}
	return c.JSON(http.StatusOK, cfg)
}

// SaveAWSConfig persists the AWS configuration.
func (h *Handler) SaveAWSConfig(c echo.Context) error {
	orgID := mustString(c, "org_id")

	var in SaveAWSConfigInput
	if err := c.Bind(&in); err != nil {
		return badRequest(c, "invalid request body")
	}
	if err := validate.Struct(in); err != nil {
		return validationError(c, err)
	}

	if err := h.svc.SaveAWSConfig(c.Request().Context(), orgID, in); err != nil {
		log.Error().Err(err).Msg("SaveAWSConfig failed")
		return serverError(c, err)
	}
	return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
}

// TestAWSConnection tests the configured AWS credentials.
func (h *Handler) TestAWSConnection(c echo.Context) error {
	orgID := mustString(c, "org_id")
	if err := h.svc.TestAWSConnection(c.Request().Context(), orgID); err != nil {
		log.Warn().Err(err).Str("org_id", orgID).Msg("TestAWSConnection failed")
		return c.JSON(http.StatusOK, map[string]any{"ok": false, "error": "cloud connection test failed"})
	}
	return c.JSON(http.StatusOK, map[string]any{"ok": true})
}

// SyncAWS triggers an immediate AWS evidence collection run.
func (h *Handler) SyncAWS(c echo.Context) error {
	orgID := mustString(c, "org_id")
	count, err := h.svc.SyncAWS(c.Request().Context(), orgID)
	if err != nil {
		log.Error().Err(err).Str("org_id", orgID).Msg("SyncAWS failed")
		return c.JSON(http.StatusOK, map[string]any{"ok": false, "error": "cloud sync failed", "evidence_created": 0})
	}
	return c.JSON(http.StatusOK, map[string]any{"ok": true, "evidence_created": count})
}

// GetAWSStatus returns the last sync status and evidence count for AWS.
func (h *Handler) GetAWSStatus(c echo.Context) error {
	orgID := mustString(c, "org_id")
	st, err := h.svc.GetAWSStatus(c.Request().Context(), orgID)
	if err != nil {
		log.Error().Err(err).Msg("GetAWSStatus failed")
		return serverError(c, err)
	}
	return c.JSON(http.StatusOK, st)
}

// GetAWSEvidence returns the 5 most recent AWS evidence items.
func (h *Handler) GetAWSEvidence(c echo.Context) error {
	orgID := mustString(c, "org_id")
	items, err := h.svc.RecentEvidence(c.Request().Context(), orgID, "aws")
	if err != nil {
		log.Error().Err(err).Msg("GetAWSEvidence failed")
		return serverError(c, err)
	}
	if items == nil {
		items = []EvidenceItem{}
	}
	return c.JSON(http.StatusOK, items)
}

// --- Azure handlers ---

// GetAzureConfig returns the Azure config with secrets masked.
func (h *Handler) GetAzureConfig(c echo.Context) error {
	orgID := mustString(c, "org_id")
	cfg, err := h.svc.GetAzureConfig(c.Request().Context(), orgID)
	if err != nil {
		log.Error().Err(err).Msg("GetAzureConfig failed")
		return serverError(c, err)
	}
	return c.JSON(http.StatusOK, cfg)
}

// SaveAzureConfig persists the Azure configuration.
func (h *Handler) SaveAzureConfig(c echo.Context) error {
	orgID := mustString(c, "org_id")

	var in SaveAzureConfigInput
	if err := c.Bind(&in); err != nil {
		return badRequest(c, "invalid request body")
	}
	if err := validate.Struct(in); err != nil {
		return validationError(c, err)
	}

	if err := h.svc.SaveAzureConfig(c.Request().Context(), orgID, in); err != nil {
		log.Error().Err(err).Msg("SaveAzureConfig failed")
		return serverError(c, err)
	}
	return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
}

// TestAzureConnection tests the configured Azure credentials.
func (h *Handler) TestAzureConnection(c echo.Context) error {
	orgID := mustString(c, "org_id")
	if err := h.svc.TestAzureConnection(c.Request().Context(), orgID); err != nil {
		log.Warn().Err(err).Str("org_id", orgID).Msg("TestAzureConnection failed")
		return c.JSON(http.StatusOK, map[string]any{"ok": false, "error": "cloud connection test failed"})
	}
	return c.JSON(http.StatusOK, map[string]any{"ok": true})
}

// SyncAzure triggers an immediate Azure evidence collection run.
func (h *Handler) SyncAzure(c echo.Context) error {
	orgID := mustString(c, "org_id")
	count, err := h.svc.SyncAzure(c.Request().Context(), orgID)
	if err != nil {
		log.Error().Err(err).Str("org_id", orgID).Msg("SyncAzure failed")
		return c.JSON(http.StatusOK, map[string]any{"ok": false, "error": "cloud sync failed", "evidence_created": 0})
	}
	return c.JSON(http.StatusOK, map[string]any{"ok": true, "evidence_created": count})
}

// GetAzureStatus returns the last sync status and evidence count for Azure.
func (h *Handler) GetAzureStatus(c echo.Context) error {
	orgID := mustString(c, "org_id")
	st, err := h.svc.GetAzureStatus(c.Request().Context(), orgID)
	if err != nil {
		log.Error().Err(err).Msg("GetAzureStatus failed")
		return serverError(c, err)
	}
	return c.JSON(http.StatusOK, st)
}

// GetAzureEvidence returns the 5 most recent Azure evidence items.
func (h *Handler) GetAzureEvidence(c echo.Context) error {
	orgID := mustString(c, "org_id")
	items, err := h.svc.RecentEvidence(c.Request().Context(), orgID, "azure")
	if err != nil {
		log.Error().Err(err).Msg("GetAzureEvidence failed")
		return serverError(c, err)
	}
	if items == nil {
		items = []EvidenceItem{}
	}
	return c.JSON(http.StatusOK, items)
}

// --- Hetzner handlers ---

func (h *Handler) GetHetznerConfig(c echo.Context) error {
	orgID := mustString(c, "org_id")
	cfg, err := h.svc.GetHetznerConfig(c.Request().Context(), orgID)
	if err != nil {
		return serverError(c, err)
	}
	return c.JSON(http.StatusOK, cfg)
}

func (h *Handler) SaveHetznerConfig(c echo.Context) error {
	orgID := mustString(c, "org_id")
	var in SaveHetznerConfigInput
	if err := c.Bind(&in); err != nil {
		return badRequest(c, "invalid request body")
	}
	if err := validate.Struct(in); err != nil {
		return validationError(c, err)
	}
	if err := h.svc.SaveHetznerConfig(c.Request().Context(), orgID, in); err != nil {
		return serverError(c, err)
	}
	return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handler) SyncHetzner(c echo.Context) error {
	orgID := mustString(c, "org_id")
	count, err := h.svc.SyncHetzner(c.Request().Context(), orgID)
	if err != nil {
		log.Error().Err(err).Str("org_id", orgID).Msg("SyncHetzner failed")
		return c.JSON(http.StatusOK, map[string]any{"ok": false, "error": "cloud sync failed", "evidence_created": 0})
	}
	return c.JSON(http.StatusOK, map[string]any{"ok": true, "evidence_created": count})
}

func (h *Handler) GetHetznerStatus(c echo.Context) error {
	orgID := mustString(c, "org_id")
	st, err := h.svc.GetHetznerStatus(c.Request().Context(), orgID)
	if err != nil {
		return serverError(c, err)
	}
	return c.JSON(http.StatusOK, st)
}

func (h *Handler) GetHetznerEvidence(c echo.Context) error {
	orgID := mustString(c, "org_id")
	items, err := h.svc.RecentEvidence(c.Request().Context(), orgID, ProviderHetzner)
	if err != nil {
		return serverError(c, err)
	}
	if items == nil {
		items = []EvidenceItem{}
	}
	return c.JSON(http.StatusOK, items)
}

// --- IONOS handlers ---

func (h *Handler) GetIONOSConfig(c echo.Context) error {
	orgID := mustString(c, "org_id")
	cfg, err := h.svc.GetIONOSConfig(c.Request().Context(), orgID)
	if err != nil {
		return serverError(c, err)
	}
	return c.JSON(http.StatusOK, cfg)
}

func (h *Handler) SaveIONOSConfig(c echo.Context) error {
	orgID := mustString(c, "org_id")
	var in SaveIONOSConfigInput
	if err := c.Bind(&in); err != nil {
		return badRequest(c, "invalid request body")
	}
	if err := h.svc.SaveIONOSConfig(c.Request().Context(), orgID, in); err != nil {
		return serverError(c, err)
	}
	return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handler) SyncIONOS(c echo.Context) error {
	orgID := mustString(c, "org_id")
	count, err := h.svc.SyncIONOS(c.Request().Context(), orgID)
	if err != nil {
		log.Error().Err(err).Str("org_id", orgID).Msg("SyncIONOS failed")
		return c.JSON(http.StatusOK, map[string]any{"ok": false, "error": "cloud sync failed", "evidence_created": 0})
	}
	return c.JSON(http.StatusOK, map[string]any{"ok": true, "evidence_created": count})
}

func (h *Handler) GetIONOSStatus(c echo.Context) error {
	orgID := mustString(c, "org_id")
	st, err := h.svc.GetIONOSStatus(c.Request().Context(), orgID)
	if err != nil {
		return serverError(c, err)
	}
	return c.JSON(http.StatusOK, st)
}

func (h *Handler) GetIONOSEvidence(c echo.Context) error {
	orgID := mustString(c, "org_id")
	items, err := h.svc.RecentEvidence(c.Request().Context(), orgID, ProviderIONOS)
	if err != nil {
		return serverError(c, err)
	}
	if items == nil {
		items = []EvidenceItem{}
	}
	return c.JSON(http.StatusOK, items)
}

// --- Wazuh handlers ---

func (h *Handler) GetWazuhConfig(c echo.Context) error {
	orgID := mustString(c, "org_id")
	cfg, err := h.svc.GetWazuhConfig(c.Request().Context(), orgID)
	if err != nil {
		return serverError(c, err)
	}
	return c.JSON(http.StatusOK, cfg)
}

func (h *Handler) SaveWazuhConfig(c echo.Context) error {
	orgID := mustString(c, "org_id")
	var in SaveWazuhConfigInput
	if err := c.Bind(&in); err != nil {
		return badRequest(c, "invalid request body")
	}
	if err := validate.Struct(in); err != nil {
		return validationError(c, err)
	}
	if err := h.svc.SaveWazuhConfig(c.Request().Context(), orgID, in); err != nil {
		return serverError(c, err)
	}
	return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handler) SyncWazuh(c echo.Context) error {
	orgID := mustString(c, "org_id")
	count, err := h.svc.SyncWazuh(c.Request().Context(), orgID)
	if err != nil {
		log.Error().Err(err).Str("org_id", orgID).Msg("SyncWazuh failed")
		return c.JSON(http.StatusOK, map[string]any{"ok": false, "error": "cloud sync failed", "evidence_created": 0})
	}
	return c.JSON(http.StatusOK, map[string]any{"ok": true, "evidence_created": count})
}

func (h *Handler) GetWazuhStatus(c echo.Context) error {
	orgID := mustString(c, "org_id")
	st, err := h.svc.GetWazuhStatus(c.Request().Context(), orgID)
	if err != nil {
		return serverError(c, err)
	}
	return c.JSON(http.StatusOK, st)
}

func (h *Handler) GetWazuhEvidence(c echo.Context) error {
	orgID := mustString(c, "org_id")
	items, err := h.svc.RecentEvidence(c.Request().Context(), orgID, ProviderWazuh)
	if err != nil {
		return serverError(c, err)
	}
	if items == nil {
		items = []EvidenceItem{}
	}
	return c.JSON(http.StatusOK, items)
}

// --- Prometheus handlers ---

func (h *Handler) GetPrometheusConfig(c echo.Context) error {
	orgID := mustString(c, "org_id")
	cfg, err := h.svc.GetPrometheusConfig(c.Request().Context(), orgID)
	if err != nil {
		return serverError(c, err)
	}
	return c.JSON(http.StatusOK, cfg)
}

func (h *Handler) SavePrometheusConfig(c echo.Context) error {
	orgID := mustString(c, "org_id")
	var in SavePrometheusConfigInput
	if err := c.Bind(&in); err != nil {
		return badRequest(c, "invalid request body")
	}
	if err := validate.Struct(in); err != nil {
		return validationError(c, err)
	}
	if err := h.svc.SavePrometheusConfig(c.Request().Context(), orgID, in); err != nil {
		return serverError(c, err)
	}
	return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handler) SyncPrometheus(c echo.Context) error {
	orgID := mustString(c, "org_id")
	count, err := h.svc.SyncPrometheus(c.Request().Context(), orgID)
	if err != nil {
		log.Error().Err(err).Str("org_id", orgID).Msg("SyncPrometheus failed")
		return c.JSON(http.StatusOK, map[string]any{"ok": false, "error": "cloud sync failed", "evidence_created": 0})
	}
	return c.JSON(http.StatusOK, map[string]any{"ok": true, "evidence_created": count})
}

func (h *Handler) GetPrometheusStatus(c echo.Context) error {
	orgID := mustString(c, "org_id")
	st, err := h.svc.GetPrometheusStatus(c.Request().Context(), orgID)
	if err != nil {
		return serverError(c, err)
	}
	return c.JSON(http.StatusOK, st)
}

func (h *Handler) GetPrometheusEvidence(c echo.Context) error {
	orgID := mustString(c, "org_id")
	items, err := h.svc.RecentEvidence(c.Request().Context(), orgID, ProviderPrometheus)
	if err != nil {
		return serverError(c, err)
	}
	if items == nil {
		items = []EvidenceItem{}
	}
	return c.JSON(http.StatusOK, items)
}

// --- Entra ID handlers ---

func (h *Handler) GetEntraIDConfig(c echo.Context) error {
	orgID := mustString(c, "org_id")
	cfg, err := h.svc.GetEntraIDConfig(c.Request().Context(), orgID)
	if err != nil {
		return serverError(c, err)
	}
	return c.JSON(http.StatusOK, cfg)
}

func (h *Handler) SaveEntraIDConfig(c echo.Context) error {
	orgID := mustString(c, "org_id")
	var in SaveEntraIDConfigInput
	if err := c.Bind(&in); err != nil {
		return badRequest(c, "invalid request body")
	}
	if err := validate.Struct(in); err != nil {
		return validationError(c, err)
	}
	if err := h.svc.SaveEntraIDConfig(c.Request().Context(), orgID, in); err != nil {
		return serverError(c, err)
	}
	return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handler) SyncEntraID(c echo.Context) error {
	orgID := mustString(c, "org_id")
	count, err := h.svc.SyncEntraID(c.Request().Context(), orgID)
	if err != nil {
		log.Error().Err(err).Str("org_id", orgID).Msg("SyncEntraID failed")
		return c.JSON(http.StatusOK, map[string]any{"ok": false, "error": "cloud sync failed", "evidence_created": 0})
	}
	return c.JSON(http.StatusOK, map[string]any{"ok": true, "evidence_created": count})
}

func (h *Handler) GetEntraIDStatus(c echo.Context) error {
	orgID := mustString(c, "org_id")
	st, err := h.svc.GetEntraIDStatus(c.Request().Context(), orgID)
	if err != nil {
		return serverError(c, err)
	}
	return c.JSON(http.StatusOK, st)
}

func (h *Handler) GetEntraIDEvidence(c echo.Context) error {
	orgID := mustString(c, "org_id")
	items, err := h.svc.RecentEvidence(c.Request().Context(), orgID, ProviderEntraID)
	if err != nil {
		return serverError(c, err)
	}
	if items == nil {
		items = []EvidenceItem{}
	}
	return c.JSON(http.StatusOK, items)
}

// --- Intune handlers (S88-7) ---

func (h *Handler) GetIntuneConfig(c echo.Context) error {
	orgID := mustString(c, "org_id")
	cfg, err := h.svc.GetIntuneConfig(c.Request().Context(), orgID)
	if err != nil {
		return serverError(c, err)
	}
	return c.JSON(http.StatusOK, cfg)
}

func (h *Handler) SaveIntuneConfig(c echo.Context) error {
	orgID := mustString(c, "org_id")
	var in SaveIntuneConfigInput
	if err := c.Bind(&in); err != nil {
		return badRequest(c, "invalid request body")
	}
	if err := validate.Struct(in); err != nil {
		return validationError(c, err)
	}
	if err := h.svc.SaveIntuneConfig(c.Request().Context(), orgID, in); err != nil {
		return serverError(c, err)
	}
	return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handler) SyncIntune(c echo.Context) error {
	orgID := mustString(c, "org_id")
	count, err := h.svc.SyncIntune(c.Request().Context(), orgID)
	if err != nil {
		log.Error().Err(err).Str("org_id", orgID).Msg("SyncIntune failed")
		return c.JSON(http.StatusOK, map[string]any{"ok": false, "error": "cloud sync failed", "evidence_created": 0})
	}
	return c.JSON(http.StatusOK, map[string]any{"ok": true, "evidence_created": count})
}

func (h *Handler) GetIntuneStatus(c echo.Context) error {
	orgID := mustString(c, "org_id")
	st, err := h.svc.GetIntuneStatus(c.Request().Context(), orgID)
	if err != nil {
		return serverError(c, err)
	}
	return c.JSON(http.StatusOK, st)
}

func (h *Handler) GetIntuneEvidence(c echo.Context) error {
	orgID := mustString(c, "org_id")
	items, err := h.svc.RecentEvidence(c.Request().Context(), orgID, ProviderIntune)
	if err != nil {
		return serverError(c, err)
	}
	if items == nil {
		items = []EvidenceItem{}
	}
	return c.JSON(http.StatusOK, items)
}

// --- Keycloak handlers ---

func (h *Handler) GetKeycloakConfig(c echo.Context) error {
	orgID := mustString(c, "org_id")
	cfg, err := h.svc.GetKeycloakConfig(c.Request().Context(), orgID)
	if err != nil {
		return serverError(c, err)
	}
	return c.JSON(http.StatusOK, cfg)
}

func (h *Handler) SaveKeycloakConfig(c echo.Context) error {
	orgID := mustString(c, "org_id")
	var in SaveKeycloakConfigInput
	if err := c.Bind(&in); err != nil {
		return badRequest(c, "invalid request body")
	}
	if err := validate.Struct(in); err != nil {
		return validationError(c, err)
	}
	if err := h.svc.SaveKeycloakConfig(c.Request().Context(), orgID, in); err != nil {
		return serverError(c, err)
	}
	return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handler) SyncKeycloak(c echo.Context) error {
	orgID := mustString(c, "org_id")
	count, err := h.svc.SyncKeycloak(c.Request().Context(), orgID)
	if err != nil {
		log.Error().Err(err).Str("org_id", orgID).Msg("SyncKeycloak failed")
		return c.JSON(http.StatusOK, map[string]any{"ok": false, "error": "cloud sync failed", "evidence_created": 0})
	}
	return c.JSON(http.StatusOK, map[string]any{"ok": true, "evidence_created": count})
}

func (h *Handler) GetKeycloakStatus(c echo.Context) error {
	orgID := mustString(c, "org_id")
	st, err := h.svc.GetKeycloakStatus(c.Request().Context(), orgID)
	if err != nil {
		return serverError(c, err)
	}
	return c.JSON(http.StatusOK, st)
}

func (h *Handler) GetKeycloakEvidence(c echo.Context) error {
	orgID := mustString(c, "org_id")
	items, err := h.svc.RecentEvidence(c.Request().Context(), orgID, ProviderKeycloak)
	if err != nil {
		return serverError(c, err)
	}
	if items == nil {
		items = []EvidenceItem{}
	}
	return c.JSON(http.StatusOK, items)
}

// --- LDAP handlers ---

func (h *Handler) GetLDAPConfig(c echo.Context) error {
	orgID := mustString(c, "org_id")
	cfg, err := h.svc.GetLDAPConfig(c.Request().Context(), orgID)
	if err != nil {
		return serverError(c, err)
	}
	return c.JSON(http.StatusOK, cfg)
}

func (h *Handler) SaveLDAPConfig(c echo.Context) error {
	orgID := mustString(c, "org_id")
	var in SaveLDAPConfigInput
	if err := c.Bind(&in); err != nil {
		return badRequest(c, "invalid request body")
	}
	if err := validate.Struct(in); err != nil {
		return validationError(c, err)
	}
	if err := h.svc.SaveLDAPConfig(c.Request().Context(), orgID, in); err != nil {
		return serverError(c, err)
	}
	return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handler) SyncLDAP(c echo.Context) error {
	orgID := mustString(c, "org_id")
	count, err := h.svc.SyncLDAP(c.Request().Context(), orgID)
	if err != nil {
		log.Error().Err(err).Str("org_id", orgID).Msg("SyncLDAP failed")
		return c.JSON(http.StatusOK, map[string]any{"ok": false, "error": "cloud sync failed", "evidence_created": 0})
	}
	return c.JSON(http.StatusOK, map[string]any{"ok": true, "evidence_created": count})
}

func (h *Handler) GetLDAPStatus(c echo.Context) error {
	orgID := mustString(c, "org_id")
	st, err := h.svc.GetLDAPStatus(c.Request().Context(), orgID)
	if err != nil {
		return serverError(c, err)
	}
	return c.JSON(http.StatusOK, st)
}

func (h *Handler) GetLDAPEvidence(c echo.Context) error {
	orgID := mustString(c, "org_id")
	items, err := h.svc.RecentEvidence(c.Request().Context(), orgID, ProviderLDAP)
	if err != nil {
		return serverError(c, err)
	}
	if items == nil {
		items = []EvidenceItem{}
	}
	return c.JSON(http.StatusOK, items)
}

// --- GitLab handlers ---

func (h *Handler) GetGitLabConfig(c echo.Context) error {
	orgID := mustString(c, "org_id")
	cfg, err := h.svc.GetGitLabConfig(c.Request().Context(), orgID)
	if err != nil {
		return serverError(c, err)
	}
	return c.JSON(http.StatusOK, cfg)
}

func (h *Handler) SaveGitLabConfig(c echo.Context) error {
	orgID := mustString(c, "org_id")
	var in SaveGitLabConfigInput
	if err := c.Bind(&in); err != nil {
		return badRequest(c, "invalid request body")
	}
	if err := validate.Struct(in); err != nil {
		return validationError(c, err)
	}
	if err := h.svc.SaveGitLabConfig(c.Request().Context(), orgID, in); err != nil {
		return serverError(c, err)
	}
	return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handler) SyncGitLab(c echo.Context) error {
	orgID := mustString(c, "org_id")
	count, err := h.svc.SyncGitLab(c.Request().Context(), orgID)
	if err != nil {
		log.Error().Err(err).Str("org_id", orgID).Msg("SyncGitLab failed")
		return c.JSON(http.StatusOK, map[string]any{"ok": false, "error": "cloud sync failed", "evidence_created": 0})
	}
	return c.JSON(http.StatusOK, map[string]any{"ok": true, "evidence_created": count})
}

func (h *Handler) GetGitLabStatus(c echo.Context) error {
	orgID := mustString(c, "org_id")
	st, err := h.svc.GetGitLabStatus(c.Request().Context(), orgID)
	if err != nil {
		return serverError(c, err)
	}
	return c.JSON(http.StatusOK, st)
}

func (h *Handler) GetGitLabEvidence(c echo.Context) error {
	orgID := mustString(c, "org_id")
	items, err := h.svc.RecentEvidence(c.Request().Context(), orgID, ProviderGitLab)
	if err != nil {
		return serverError(c, err)
	}
	if items == nil {
		items = []EvidenceItem{}
	}
	return c.JSON(http.StatusOK, items)
}

// --- SonarQube handlers ---

func (h *Handler) GetSonarQubeConfig(c echo.Context) error {
	orgID := mustString(c, "org_id")
	cfg, err := h.svc.GetSonarQubeConfig(c.Request().Context(), orgID)
	if err != nil {
		return serverError(c, err)
	}
	return c.JSON(http.StatusOK, cfg)
}

func (h *Handler) SaveSonarQubeConfig(c echo.Context) error {
	orgID := mustString(c, "org_id")
	var in SaveSonarQubeConfigInput
	if err := c.Bind(&in); err != nil {
		return badRequest(c, "invalid request body")
	}
	if err := validate.Struct(in); err != nil {
		return validationError(c, err)
	}
	if err := h.svc.SaveSonarQubeConfig(c.Request().Context(), orgID, in); err != nil {
		return serverError(c, err)
	}
	return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handler) SyncSonarQube(c echo.Context) error {
	orgID := mustString(c, "org_id")
	count, err := h.svc.SyncSonarQube(c.Request().Context(), orgID)
	if err != nil {
		log.Error().Err(err).Str("org_id", orgID).Msg("SyncSonarQube failed")
		return c.JSON(http.StatusOK, map[string]any{"ok": false, "error": "cloud sync failed", "evidence_created": 0})
	}
	return c.JSON(http.StatusOK, map[string]any{"ok": true, "evidence_created": count})
}

func (h *Handler) GetSonarQubeStatus(c echo.Context) error {
	orgID := mustString(c, "org_id")
	st, err := h.svc.GetSonarQubeStatus(c.Request().Context(), orgID)
	if err != nil {
		return serverError(c, err)
	}
	return c.JSON(http.StatusOK, st)
}

func (h *Handler) GetSonarQubeEvidence(c echo.Context) error {
	orgID := mustString(c, "org_id")
	items, err := h.svc.RecentEvidence(c.Request().Context(), orgID, ProviderSonarQube)
	if err != nil {
		return serverError(c, err)
	}
	if items == nil {
		items = []EvidenceItem{}
	}
	return c.JSON(http.StatusOK, items)
}

// --- Personio handlers ---

func (h *Handler) GetPersonioConfig(c echo.Context) error {
	orgID := mustString(c, "org_id")
	cfg, err := h.svc.GetPersonioConfig(c.Request().Context(), orgID)
	if err != nil {
		return serverError(c, err)
	}
	return c.JSON(http.StatusOK, cfg)
}

func (h *Handler) SavePersonioConfig(c echo.Context) error {
	orgID := mustString(c, "org_id")
	var in SavePersonioConfigInput
	if err := c.Bind(&in); err != nil {
		return badRequest(c, "invalid request body")
	}
	if err := validate.Struct(in); err != nil {
		return validationError(c, err)
	}
	if err := h.svc.SavePersonioConfig(c.Request().Context(), orgID, in); err != nil {
		return serverError(c, err)
	}
	return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handler) GetPersonioStatus(c echo.Context) error {
	orgID := mustString(c, "org_id")
	st, err := h.svc.GetPersonioStatus(c.Request().Context(), orgID)
	if err != nil {
		return serverError(c, err)
	}
	st.WebhookURL = "/api/v1/vakthr/webhooks/personio/" + orgID
	return c.JSON(http.StatusOK, st)
}

// --- helpers ---

func mustString(c echo.Context, key string) string {
	v, _ := c.Get(key).(string)
	return v
}

func badRequest(c echo.Context, msg string) error {
	return c.JSON(http.StatusBadRequest, map[string]string{"error": msg, "code": "BAD_REQUEST"})
}

func serverError(c echo.Context, err error) error {
	log.Error().Err(err).Msg("cloud integration operation failed")
	return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal server error", "code": "INTERNAL_ERROR"})
}

func validationError(c echo.Context, err error) error {
	return c.JSON(http.StatusUnprocessableEntity, map[string]string{"error": "validation error", "code": "VALIDATION_ERROR"})
}
