package secvitals

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog/log"

	"github.com/matharnica/vakt/internal/shared/audit"
	"github.com/matharnica/vakt/internal/shared/pagination"
)

// GetPolicy handles GET /api/v1/secvitals/policies/:id.
func (h *Handler) GetPolicy(c echo.Context) error {
	id := c.Param("id")
	policy, err := h.service.GetPolicy(c.Request().Context(), orgID(c), id)
	if err != nil {
		return errResp(c, http.StatusNotFound, "policy not found", "CK_POLICY_NOT_FOUND")
	}
	return c.JSON(http.StatusOK, policy)
}

// UpdatePolicy handles PATCH /api/v1/secvitals/policies/:id.
func (h *Handler) UpdatePolicy(c echo.Context) error {
	id := c.Param("id")
	var in UpdatePolicyInput
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "CK_BAD_REQUEST")
	}
	if err := h.validate.Struct(in); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, map[string]string{"error": "Ungültige Eingabe", "code": "VALIDATION_ERROR"})
	}
	policy, err := h.service.UpdatePolicy(c.Request().Context(), orgID(c), id, in)
	if err != nil {
		log.Error().Err(err).Msg("update policy")
		return errResp(c, http.StatusInternalServerError, "failed to update policy", "CK_UPDATE_POLICY_FAILED")
	}
	return c.JSON(http.StatusOK, policy)
}

// ListPolicies handles GET /api/v1/secvitals/policies.
func (h *Handler) ListPolicies(c echo.Context) error {
	offset, limit, meta := pagination.FromRequest(c)
	policies, total, err := h.service.ListPoliciesPaged(c.Request().Context(), orgID(c), offset, limit)
	if err != nil {
		log.Error().Err(err).Msg("list policies")
		return errResp(c, http.StatusInternalServerError, "failed to list policies", "CK_LIST_POLICIES_FAILED")
	}
	pagination.Complete(&meta, total)
	return c.JSON(http.StatusOK, pagination.Wrap(policies, meta))
}

// CreatePolicy handles POST /api/v1/secvitals/policies.
func (h *Handler) CreatePolicy(c echo.Context) error {
	var in CreatePolicyInput
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "CK_BAD_REQUEST")
	}
	if err := h.validate.Struct(in); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, map[string]string{"error": "Ungültige Eingabe", "code": "VALIDATION_ERROR"})
	}
	policy, err := h.service.CreatePolicy(c.Request().Context(), orgID(c), in)
	if err != nil {
		log.Error().Err(err).Msg("create policy")
		return errResp(c, http.StatusInternalServerError, "failed to create policy", "CK_CREATE_POLICY_FAILED")
	}
	audit.Write(c.Request().Context(), h.db, audit.WriteEntry{
		OrgID:        orgID(c),
		UserID:       userID(c),
		Action:       "create",
		ResourceType: "vakt-comply/policy",
		ResourceID:   policy.ID,
		ResourceName: policy.Title,
		IPAddress:    c.RealIP(),
	})
	return c.JSON(http.StatusCreated, policy)
}

// ListPolicyVersions handles GET /api/v1/secvitals/policies/:id/versions.
// Returns all historical version snapshots for a policy, newest first.
func (h *Handler) ListPolicyVersions(c echo.Context) error {
	policyID := c.Param("id")
	versions, err := h.service.ListPolicyVersions(c.Request().Context(), orgID(c), policyID)
	if err != nil {
		log.Error().Err(err).Str("policy_id", policyID).Msg("list policy versions")
		return errResp(c, http.StatusInternalServerError, "failed to list policy versions", "CK_LIST_POLICY_VERSIONS_FAILED")
	}
	return c.JSON(http.StatusOK, versions)
}

// GetPolicyVersion handles GET /api/v1/secvitals/policies/:id/versions/:v.
// Returns a single historical version snapshot by version number.
func (h *Handler) GetPolicyVersion(c echo.Context) error {
	policyID := c.Param("id")
	vStr := c.Param("v")
	vNum, err := strconv.Atoi(vStr)
	if err != nil || vNum < 1 {
		return errResp(c, http.StatusBadRequest, "invalid version number", "CK_BAD_REQUEST")
	}
	pv, err := h.service.GetPolicyVersion(c.Request().Context(), orgID(c), policyID, vNum)
	if err != nil {
		return errResp(c, http.StatusNotFound, "policy version not found", "CK_POLICY_VERSION_NOT_FOUND")
	}
	return c.JSON(http.StatusOK, pv)
}

// ListPolicyTemplates handles GET /api/v1/secvitals/policy-templates.
func (h *Handler) ListPolicyTemplates(c echo.Context) error {
	return c.JSON(http.StatusOK, BuiltinPolicyTemplates())
}

// CreatePolicyFromTemplate handles POST /api/v1/secvitals/policy-templates/:id/apply.
// Creates a new policy using the template content as description.
func (h *Handler) CreatePolicyFromTemplate(c echo.Context) error {
	orgID := orgID(c)
	if orgID == "" {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
	}
	templateID := c.Param("id")
	templates := BuiltinPolicyTemplates()
	var found *PolicyTemplate
	for i := range templates {
		if templates[i].ID == templateID {
			found = &templates[i]
			break
		}
	}
	if found == nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "template not found"})
	}

	in := CreatePolicyInput{
		Title:       found.Title,
		Category:    found.Category,
		Description: found.Content,
		Version:     "1.0",
	}
	policy, err := h.service.CreatePolicy(c.Request().Context(), orgID, in)
	if err != nil {
		log.Error().Err(err).Msg("CreatePolicyFromTemplate: create policy failed")
		return errResp(c, http.StatusInternalServerError, "failed to create policy from template", "CK_CREATE_POLICY_FAILED")
	}
	return c.JSON(http.StatusCreated, policy)
}

// GeneratePolicyDraft handles POST /api/v1/secvitals/policies/generate-draft.
// Generates an AI-written policy draft in German using the configured AI provider.
func (h *Handler) GeneratePolicyDraft(c echo.Context) error {
	var in GeneratePolicyDraftInput
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "CK_INVALID_BODY")
	}
	if err := h.validate.Struct(in); err != nil {
		return errResp(c, http.StatusUnprocessableEntity, "Ungültige Eingabe", "VALIDATION_ERROR")
	}
	draft, err := h.service.GeneratePolicyDraft(c.Request().Context(), orgID(c), in)
	if err != nil {
		log.Error().Err(err).Msg("generate policy draft")
		if errors.Is(err, ErrNotConfigured) {
			return errResp(c, http.StatusServiceUnavailable, err.Error(), "CK_AI_NOT_CONFIGURED")
		}
		return errResp(c, http.StatusInternalServerError, "AI generation failed", "CK_AI_FAILED")
	}
	return c.JSON(http.StatusOK, map[string]string{"draft": draft})
}
