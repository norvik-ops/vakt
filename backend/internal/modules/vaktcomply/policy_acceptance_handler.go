package vaktcomply

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog/log"
)

// smtpCfgFromHandler extracts the SMTP config that was set on the handler.
// The handler carries it so the service does not need to import config directly.
type PolicyAcceptanceHandlerConfig struct {
	SMTPHost    string
	SMTPPort    string
	SMTPUser    string
	SMTPPass    string
	SMTPFrom    string
	FrontendURL string
}

// paCfg holds handler-level config for policy acceptance.
// It is set via WithPolicyAcceptanceConfig after construction.
var _ = (*Handler)(nil)

// WithPolicyAcceptanceConfig attaches SMTP and frontend URL config to the handler.
func (h *Handler) WithPolicyAcceptanceConfig(cfg PolicyAcceptanceHandlerConfig) {
	h.paCfg = cfg
}

// CreateAcceptanceCampaign handles POST /vaktcomply/policies/:id/acceptance-campaigns.
// Creates an acceptance campaign and fires off invitation emails.
func (h *Handler) CreateAcceptanceCampaign(c echo.Context) error {
	policyID := c.Param("id")
	if policyID == "" {
		return errResp(c, http.StatusBadRequest, "policy ID required", "CK_BAD_REQUEST")
	}

	var in CreateCampaignInput
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "CK_BAD_REQUEST")
	}
	in.PolicyID = policyID

	if err := h.validate.Struct(in); err != nil {
		return errResp(c, http.StatusBadRequest, err.Error(), "CK_VALIDATION_ERROR")
	}

	smtpCfg := policyAcceptanceSMTPConfig{
		Host: h.paCfg.SMTPHost,
		Port: h.paCfg.SMTPPort,
		User: h.paCfg.SMTPUser,
		Pass: h.paCfg.SMTPPass,
		From: h.paCfg.SMTPFrom,
	}

	campaign, err := h.service.CreateAcceptanceCampaign(
		c.Request().Context(),
		orgID(c), userID(c),
		in,
		smtpCfg,
		h.paCfg.FrontendURL,
	)
	if err != nil {
		log.Error().Err(err).Msg("create acceptance campaign")
		return errResp(c, http.StatusInternalServerError, "failed to create campaign", "CK_CAMPAIGN_CREATE_FAILED")
	}
	return c.JSON(http.StatusCreated, campaign)
}

// ListAcceptanceCampaigns handles GET /vaktcomply/policies/:id/acceptance-campaigns.
func (h *Handler) ListAcceptanceCampaigns(c echo.Context) error {
	policyID := c.Param("id")
	if policyID == "" {
		return errResp(c, http.StatusBadRequest, "policy ID required", "CK_BAD_REQUEST")
	}

	campaigns, err := h.service.ListCampaigns(c.Request().Context(), orgID(c), policyID)
	if err != nil {
		log.Error().Err(err).Msg("list acceptance campaigns")
		return errResp(c, http.StatusInternalServerError, "failed to list campaigns", "CK_CAMPAIGN_LIST_FAILED")
	}
	if campaigns == nil {
		campaigns = []PolicyAcceptanceCampaign{}
	}
	return c.JSON(http.StatusOK, campaigns)
}

// GetCampaignStats handles GET /vaktcomply/policies/acceptance-campaigns/:cid/stats.
func (h *Handler) GetCampaignStats(c echo.Context) error {
	cid := c.Param("cid")
	if cid == "" {
		return errResp(c, http.StatusBadRequest, "campaign ID required", "CK_BAD_REQUEST")
	}

	stats, err := h.service.GetCampaignStats(c.Request().Context(), cid)
	if err != nil {
		log.Error().Err(err).Msg("get campaign stats")
		return errResp(c, http.StatusInternalServerError, "failed to get stats", "CK_CAMPAIGN_STATS_FAILED")
	}
	return c.JSON(http.StatusOK, stats)
}

// ListCampaignRequests handles GET /vaktcomply/policies/acceptance-campaigns/:cid/requests.
func (h *Handler) ListCampaignRequests(c echo.Context) error {
	cid := c.Param("cid")
	if cid == "" {
		return errResp(c, http.StatusBadRequest, "campaign ID required", "CK_BAD_REQUEST")
	}

	requests, err := h.service.ListCampaignRequests(c.Request().Context(), cid)
	if err != nil {
		log.Error().Err(err).Msg("list campaign requests")
		return errResp(c, http.StatusInternalServerError, "failed to list requests", "CK_CAMPAIGN_REQUESTS_FAILED")
	}
	if requests == nil {
		requests = []PolicyAcceptanceRequest{}
	}
	return c.JSON(http.StatusOK, requests)
}

// GetAcceptanceInfo handles GET /policy-accept/:token — public, no auth.
// Returns policy/org/message info for the accept page.
func (h *Handler) GetAcceptanceInfo(c echo.Context) error {
	token := c.Param("token")
	if token == "" {
		return errResp(c, http.StatusBadRequest, "token required", "CK_BAD_REQUEST")
	}

	info, err := h.service.GetAcceptanceRequestInfo(c.Request().Context(), token)
	if err != nil {
		return errResp(c, http.StatusNotFound, "token not found or expired", "CK_TOKEN_NOT_FOUND")
	}
	return c.JSON(http.StatusOK, info)
}

// AcceptPolicy handles POST /policy-accept/:token — public, no auth.
// Records the acceptance timestamp and IP; creates compliance evidence.
func (h *Handler) AcceptPolicy(c echo.Context) error {
	token := c.Param("token")
	if token == "" {
		return errResp(c, http.StatusBadRequest, "token required", "CK_BAD_REQUEST")
	}

	ip := c.RealIP()
	if err := h.service.AcceptPolicy(c.Request().Context(), token, ip); err != nil {
		log.Warn().Err(err).Str("ip", ip).Msg("accept policy")
		return errResp(c, http.StatusNotFound, "token not found or expired", "CK_TOKEN_NOT_FOUND")
	}
	return c.JSON(http.StatusOK, map[string]string{"status": "accepted"})
}
