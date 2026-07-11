package vaktcomply

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/labstack/echo/v4"
	auditmod "github.com/matharnica/vakt/internal/modules/vaktcomply/audit"
	"github.com/matharnica/vakt/internal/shared/audit"
	"github.com/matharnica/vakt/internal/shared/pagination"
	"github.com/rs/zerolog/log"
)

type MyTask struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Type        string `json:"type"` // "control" or "risk"
	Status      string `json:"status"`
	FrameworkID string `json:"framework_id,omitempty"`
	RiskID      string `json:"risk_id,omitempty"`
}

// GetMyTasks handles GET /vaktcomply/my-tasks.
// Returns controls and risks where the authenticated user is the owner.
func (h *Handler) GetMyTasks(c echo.Context) error {
	ctx := c.Request().Context()
	uID := userID(c)
	oID := orgID(c)

	// Resolve current user's display_name.
	displayName, err := h.service.repo.GetUserDisplayName(ctx, uID)
	if err != nil {
		log.Error().Err(err).Str("user_id", uID).Msg("get my tasks: resolve display_name")
		return errResp(c, http.StatusInternalServerError, "failed to resolve user", "MY_TASKS_USER_ERROR")
	}

	// Controls where owner = display_name.
	ctrlTasks, err := h.service.repo.GetMyTaskControls(ctx, oID, displayName)
	if err != nil {
		log.Error().Err(err).Msg("get my tasks: controls")
		return errResp(c, http.StatusInternalServerError, "failed to list tasks", "MY_TASKS_ERROR")
	}

	// Risks where owner = display_name.
	riskTasks, err := h.service.repo.GetMyTaskRisks(ctx, oID, displayName)
	if err != nil {
		log.Error().Err(err).Msg("get my tasks: risks")
		return errResp(c, http.StatusInternalServerError, "failed to list tasks", "MY_TASKS_ERROR")
	}

	tasks := make([]MyTask, 0, len(ctrlTasks)+len(riskTasks))
	tasks = append(tasks, ctrlTasks...)
	tasks = append(tasks, riskTasks...)

	return c.JSON(http.StatusOK, tasks)
}

// GetISMSScope handles GET /api/v1/vaktcomply/isms-scope.
func (h *Handler) GetISMSScope(c echo.Context) error {
	scope, err := h.service.GetCurrentISMSScope(c.Request().Context(), orgID(c))
	if err != nil {
		if isNotFound(err) {
			return c.JSON(http.StatusOK, nil)
		}
		log.Error().Err(err).Msg("get isms scope")
		return errResp(c, http.StatusInternalServerError, "failed to get ISMS scope", "CK_GET_ISMS_SCOPE_FAILED")
	}
	return c.JSON(http.StatusOK, scope)
}

// CreateOrUpdateISMSScope handles POST /api/v1/vaktcomply/isms-scope.
func (h *Handler) CreateOrUpdateISMSScope(c echo.Context) error {
	var in CreateISMSScopeInput
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "CK_BAD_REQUEST")
	}
	scope, err := h.service.CreateOrVersionISMSScope(c.Request().Context(), orgID(c), userID(c), in)
	if err != nil {
		log.Error().Err(err).Msg("create isms scope")
		return errResp(c, http.StatusInternalServerError, "failed to save ISMS scope", "CK_CREATE_ISMS_SCOPE_FAILED")
	}
	return c.JSON(http.StatusCreated, scope)
}

// ListISMSScopeVersions handles GET /api/v1/vaktcomply/isms-scope/versions.
func (h *Handler) ListISMSScopeVersions(c echo.Context) error {
	versions, err := h.service.ListISMSScopeVersions(c.Request().Context(), orgID(c))
	if err != nil {
		log.Error().Err(err).Msg("list isms scope versions")
		return errResp(c, http.StatusInternalServerError, "failed to list ISMS scope versions", "CK_LIST_ISMS_SCOPE_VERSIONS_FAILED")
	}
	if versions == nil {
		versions = []ISMSScope{}
	}
	return c.JSON(http.StatusOK, versions)
}

// ApproveISMSScope handles POST /api/v1/vaktcomply/isms-scope/approve.
func (h *Handler) ApproveISMSScope(c echo.Context) error {
	var body struct {
		ID string `json:"id"`
	}
	if err := c.Bind(&body); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "CK_BAD_REQUEST")
	}
	if body.ID == "" {
		return errResp(c, http.StatusBadRequest, "id is required", "CK_BAD_REQUEST")
	}
	userRole, _ := c.Get("role").(string)
	scope, err := h.service.ApproveISMSScope(c.Request().Context(), orgID(c), body.ID, userID(c), userRole)
	if err != nil {
		if isNotFound(err) {
			return errResp(c, http.StatusNotFound, "ISMS scope not found", "CK_ISMS_SCOPE_NOT_FOUND")
		}
		log.Error().Err(err).Msg("approve isms scope")
		return errResp(c, http.StatusForbidden, err.Error(), "CK_ISMS_SCOPE_APPROVE_FORBIDDEN")
	}
	return c.JSON(http.StatusOK, scope)
}

// ExportISMSScopePDF handles GET /api/v1/vaktcomply/isms-scope/export-pdf.
func (h *Handler) ExportISMSScopePDF(c echo.Context) error {
	return c.JSON(http.StatusNotImplemented, map[string]string{"message": "PDF export coming soon"})
}

// ListCryptoKeys handles GET /api/v1/vaktcomply/crypto-keys.
func (h *Handler) ListCryptoKeys(c echo.Context) error {
	keys, err := h.service.ListCryptoKeys(c.Request().Context(), orgID(c))
	if err != nil {
		log.Error().Err(err).Msg("list crypto keys")
		return errResp(c, http.StatusInternalServerError, "failed to list crypto keys", "CK_LIST_CRYPTO_KEYS_FAILED")
	}
	return c.JSON(http.StatusOK, keys)
}

// CreateCryptoKey handles POST /api/v1/vaktcomply/crypto-keys.
func (h *Handler) CreateCryptoKey(c echo.Context) error {
	var in CreateCryptoKeyInput
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "CK_BAD_REQUEST")
	}
	if err := h.validate.Struct(in); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, map[string]string{"error": "Ungültige Eingabe", "code": "VALIDATION_ERROR"})
	}
	key, err := h.service.CreateCryptoKey(c.Request().Context(), orgID(c), in)
	if err != nil {
		log.Error().Err(err).Msg("create crypto key")
		return errResp(c, http.StatusInternalServerError, "failed to create crypto key", "CK_CREATE_CRYPTO_KEY_FAILED")
	}
	return c.JSON(http.StatusCreated, key)
}

// RotateCryptoKey handles POST /api/v1/vaktcomply/crypto-keys/:id/rotate.
func (h *Handler) RotateCryptoKey(c echo.Context) error {
	keyID := c.Param("id")
	key, err := h.service.RecordKeyRotation(c.Request().Context(), orgID(c), keyID)
	if err != nil {
		if isNotFound(err) {
			return errResp(c, http.StatusNotFound, "crypto key not found", "CK_CRYPTO_KEY_NOT_FOUND")
		}
		log.Error().Err(err).Str("key_id", keyID).Msg("rotate crypto key")
		return errResp(c, http.StatusInternalServerError, "failed to rotate crypto key", "CK_ROTATE_CRYPTO_KEY_FAILED")
	}
	return c.JSON(http.StatusOK, key)
}

// DeleteCryptoKey handles DELETE /api/v1/vaktcomply/crypto-keys/:id.
func (h *Handler) DeleteCryptoKey(c echo.Context) error {
	keyID := c.Param("id")
	if err := h.service.DeleteCryptoKey(c.Request().Context(), orgID(c), keyID); err != nil {
		if isNotFound(err) {
			return errResp(c, http.StatusNotFound, "crypto key not found", "CK_CRYPTO_KEY_NOT_FOUND")
		}
		log.Error().Err(err).Str("key_id", keyID).Msg("delete crypto key")
		return errResp(c, http.StatusInternalServerError, "failed to delete crypto key", "CK_DELETE_CRYPTO_KEY_FAILED")
	}
	return c.NoContent(http.StatusNoContent)
}

// ListPentests handles GET /api/v1/vaktcomply/pentests.
// Supports optional ?tester_type=internal|external query parameter.
func (h *Handler) ListPentests(c echo.Context) error {
	var testerType *string
	if v := c.QueryParam("tester_type"); v != "" {
		testerType = &v
	}
	pentests, err := h.service.ListPentests(c.Request().Context(), orgID(c), testerType)
	if err != nil {
		log.Error().Err(err).Msg("list pentests")
		return errResp(c, http.StatusInternalServerError, "failed to list pentests", "CK_LIST_PENTESTS_FAILED")
	}
	if pentests == nil {
		pentests = []Pentest{}
	}
	return c.JSON(http.StatusOK, pentests)
}

// CreatePentest handles POST /api/v1/vaktcomply/pentests.
func (h *Handler) CreatePentest(c echo.Context) error {
	var in CreatePentestInput
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "CK_BAD_REQUEST")
	}
	if err := h.validate.Struct(in); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, map[string]string{"error": "Ungültige Eingabe", "code": "VALIDATION_ERROR"})
	}
	pentest, err := h.service.CreatePentest(c.Request().Context(), orgID(c), userID(c), in)
	if err != nil {
		log.Error().Err(err).Msg("create pentest")
		return errResp(c, http.StatusInternalServerError, "failed to create pentest", "CK_CREATE_PENTEST_FAILED")
	}
	return c.JSON(http.StatusCreated, pentest)
}

// GetPentest handles GET /api/v1/vaktcomply/pentests/:id.
func (h *Handler) GetPentest(c echo.Context) error {
	id := c.Param("id")
	pentest, err := h.service.GetPentest(c.Request().Context(), orgID(c), id)
	if err != nil {
		return errResp(c, http.StatusNotFound, "pentest not found", "CK_PENTEST_NOT_FOUND")
	}
	return c.JSON(http.StatusOK, pentest)
}

// UpdatePentest handles PATCH /api/v1/vaktcomply/pentests/:id.
func (h *Handler) UpdatePentest(c echo.Context) error {
	id := c.Param("id")
	var in UpdatePentestInput
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "CK_BAD_REQUEST")
	}
	if err := h.validate.Struct(in); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, map[string]string{"error": "Ungültige Eingabe", "code": "VALIDATION_ERROR"})
	}
	pentest, err := h.service.UpdatePentest(c.Request().Context(), orgID(c), id, in)
	if err != nil {
		log.Error().Err(err).Str("pentest_id", id).Msg("update pentest")
		return errResp(c, http.StatusInternalServerError, "failed to update pentest", "CK_UPDATE_PENTEST_FAILED")
	}
	return c.JSON(http.StatusOK, pentest)
}

// DeletePentest handles DELETE /api/v1/vaktcomply/pentests/:id.
func (h *Handler) DeletePentest(c echo.Context) error {
	id := c.Param("id")
	if err := h.service.DeletePentest(c.Request().Context(), orgID(c), id); err != nil {
		log.Error().Err(err).Str("pentest_id", id).Msg("delete pentest")
		// S121-D4 (P3): not-found → 404, not 500
		if isNotFound(err) {
			return errResp(c, http.StatusNotFound, "pentest not found", "CK_PENTEST_NOT_FOUND")
		}
		return errResp(c, http.StatusInternalServerError, "failed to delete pentest", "CK_DELETE_PENTEST_FAILED")
	}
	return c.NoContent(http.StatusNoContent)
}

// UploadPentestReport handles POST /api/v1/vaktcomply/pentests/:id/report.
// Report file upload is not yet implemented.
func (h *Handler) UploadPentestReport(c echo.Context) error {
	return c.JSON(http.StatusNotImplemented, map[string]string{
		"error": "report upload not yet implemented",
		"code":  "CK_NOT_IMPLEMENTED",
	})
}

// LinkPentestAsEvidence handles POST /api/v1/vaktcomply/pentests/:id/link-evidence.
// Evidence linking is not yet implemented.
func (h *Handler) LinkPentestAsEvidence(c echo.Context) error {
	return c.JSON(http.StatusNotImplemented, map[string]string{
		"error": "evidence linking not yet implemented",
		"code":  "CK_NOT_IMPLEMENTED",
	})
}

type InterestedParty struct {
	ID              string  `json:"id"`
	OrgID           string  `json:"org_id"`
	Name            string  `json:"name"`
	Category        string  `json:"category"`
	Requirements    string  `json:"requirements,omitempty"`
	Concerns        string  `json:"concerns,omitempty"`
	ReviewDate      *string `json:"review_date,omitempty"`
	ReviewOverdue   bool    `json:"review_overdue"`
	IsSystemDefault bool    `json:"is_system_default"`
	CreatedAt       string  `json:"created_at"`
	UpdatedAt       string  `json:"updated_at"`
}

// CreateInterestedPartyInput holds validated input for a new interested party.
type CreateInterestedPartyInput struct {
	Name         string  `json:"name"     validate:"required,max=200"`
	Category     string  `json:"category" validate:"required,oneof=customer regulator employee shareholder supplier insurer it_provider other"`
	Requirements string  `json:"requirements,omitempty" validate:"max=5000"`
	Concerns     string  `json:"concerns,omitempty"     validate:"max=5000"`
	ReviewDate   *string `json:"review_date,omitempty"`
}

// ListInterestedParties handles GET /api/v1/vaktcomply/interested-parties
func (h *Handler) ListInterestedParties(c echo.Context) error {
	parties, err := h.service.ListInterestedParties(c.Request().Context(), orgID(c))
	if err != nil {
		log.Error().Err(err).Msg("list interested parties")
		return errResp(c, http.StatusInternalServerError, "failed to list interested parties", "CK_INTERESTED_PARTIES_FAILED")
	}
	return c.JSON(http.StatusOK, parties)
}

// CreateInterestedParty handles POST /api/v1/vaktcomply/interested-parties
func (h *Handler) CreateInterestedParty(c echo.Context) error {
	var in CreateInterestedPartyInput
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "CK_BAD_REQUEST")
	}
	if err := h.validate.Struct(in); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, map[string]string{"error": "Ungültige Eingabe", "code": "VALIDATION_ERROR"})
	}
	party, err := h.service.CreateInterestedParty(c.Request().Context(), orgID(c), in)
	if err != nil {
		log.Error().Err(err).Msg("create interested party")
		return errResp(c, http.StatusInternalServerError, "failed to create interested party", "CK_INTERESTED_PARTIES_FAILED")
	}
	return c.JSON(http.StatusCreated, party)
}

// UpdateInterestedParty handles PUT /api/v1/vaktcomply/interested-parties/:id
func (h *Handler) UpdateInterestedParty(c echo.Context) error {
	var in CreateInterestedPartyInput
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "CK_BAD_REQUEST")
	}
	if err := h.validate.Struct(in); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, map[string]string{"error": "Ungültige Eingabe", "code": "VALIDATION_ERROR"})
	}
	party, err := h.service.UpdateInterestedParty(c.Request().Context(), orgID(c), c.Param("id"), in)
	if err != nil {
		log.Error().Err(err).Msg("update interested party")
		return errResp(c, http.StatusInternalServerError, "failed to update interested party", "CK_INTERESTED_PARTIES_FAILED")
	}
	return c.JSON(http.StatusOK, party)
}

// DeleteInterestedParty handles DELETE /api/v1/vaktcomply/interested-parties/:id
func (h *Handler) DeleteInterestedParty(c echo.Context) error {
	if err := h.service.DeleteInterestedParty(c.Request().Context(), orgID(c), c.Param("id")); err != nil {
		log.Error().Err(err).Msg("delete interested party")
		if isNotFound(err) {
			return errResp(c, http.StatusNotFound, "interested party not found", "CK_INTERESTED_PARTY_NOT_FOUND")
		}
		return errResp(c, http.StatusInternalServerError, "failed to delete interested party", "CK_INTERESTED_PARTIES_FAILED")
	}
	return c.NoContent(http.StatusNoContent)
}

// SeedDefaultInterestedParties handles POST /api/v1/vaktcomply/interested-parties/seed-defaults
// Inserts the 6 standard ISMS stakeholders if the org has none.
func (h *Handler) SeedDefaultInterestedParties(c echo.Context) error {
	if err := h.service.SeedDefaultInterestedParties(c.Request().Context(), orgID(c)); err != nil {
		log.Error().Err(err).Msg("seed default interested parties")
		return errResp(c, http.StatusInternalServerError, "failed to seed interested parties", "CK_INTERESTED_PARTIES_FAILED")
	}
	return c.NoContent(http.StatusNoContent)
}

// ExportInterestedPartiesPDF handles GET /api/v1/vaktcomply/interested-parties/export
func (h *Handler) ExportInterestedPartiesPDF(c echo.Context) error {
	data, err := h.service.ExportInterestedPartiesPDF(c.Request().Context(), orgID(c))
	if err != nil {
		log.Error().Err(err).Msg("export interested parties pdf")
		return errResp(c, http.StatusInternalServerError, "export failed", "CK_INTERESTED_PARTIES_EXPORT_FAILED")
	}
	filename := fmt.Sprintf("vakt-interested-parties-%s.pdf", time.Now().UTC().Format("2006-01-02"))
	c.Response().Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename=%q`, filename))
	return c.Blob(http.StatusOK, "application/pdf", data)
}

func (h *Handler) ListAccessReviewCampaigns(c echo.Context) error {
	campaigns, err := h.service.ListAccessReviewCampaigns(c.Request().Context(), orgID(c))
	if err != nil {
		log.Error().Err(err).Msg("list access review campaigns")
		return errResp(c, http.StatusInternalServerError, "failed to list access review campaigns", "CK_LIST_ACCESS_REVIEWS_FAILED")
	}
	return c.JSON(http.StatusOK, campaigns)
}

// CreateAccessReviewCampaign handles POST /api/v1/vaktcomply/access-reviews.
func (h *Handler) CreateAccessReviewCampaign(c echo.Context) error {
	var in CreateAccessReviewCampaignInput
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "CK_BAD_REQUEST")
	}
	if err := h.validate.Struct(in); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, map[string]string{"error": "Ungültige Eingabe", "code": "VALIDATION_ERROR"})
	}
	campaign, err := h.service.CreateAccessReviewCampaign(c.Request().Context(), orgID(c), in)
	if err != nil {
		log.Error().Err(err).Msg("create access review campaign")
		return errResp(c, http.StatusInternalServerError, "failed to create access review campaign", "CK_CREATE_ACCESS_REVIEW_FAILED")
	}
	return c.JSON(http.StatusCreated, campaign)
}

// GetAccessReviewCampaign handles GET /api/v1/vaktcomply/access-reviews/:id.
func (h *Handler) GetAccessReviewCampaign(c echo.Context) error {
	id := c.Param("id")
	campaign, err := h.service.GetAccessReviewCampaign(c.Request().Context(), orgID(c), id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return errResp(c, http.StatusNotFound, "access review campaign not found", "CK_ACCESS_REVIEW_NOT_FOUND")
		}
		log.Error().Err(err).Str("id", id).Msg("get access review campaign")
		return errResp(c, http.StatusInternalServerError, "failed to get access review campaign", "CK_GET_ACCESS_REVIEW_FAILED")
	}
	return c.JSON(http.StatusOK, campaign)
}

// UpdateAccessReviewCampaign handles PUT /api/v1/vaktcomply/access-reviews/:id.
func (h *Handler) UpdateAccessReviewCampaign(c echo.Context) error {
	id := c.Param("id")
	var in UpdateAccessReviewCampaignInput
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "CK_BAD_REQUEST")
	}
	if err := h.validate.Struct(in); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, map[string]string{"error": "Ungültige Eingabe", "code": "VALIDATION_ERROR"})
	}
	campaign, err := h.service.UpdateAccessReviewCampaign(c.Request().Context(), orgID(c), id, in)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return errResp(c, http.StatusNotFound, "access review campaign not found", "CK_ACCESS_REVIEW_NOT_FOUND")
		}
		log.Error().Err(err).Msg("update access review campaign")
		return errResp(c, http.StatusInternalServerError, "failed to update access review campaign", "CK_UPDATE_ACCESS_REVIEW_FAILED")
	}
	return c.JSON(http.StatusOK, campaign)
}

// DeleteAccessReviewCampaign handles DELETE /api/v1/vaktcomply/access-reviews/:id.
func (h *Handler) DeleteAccessReviewCampaign(c echo.Context) error {
	id := c.Param("id")
	if err := h.service.DeleteAccessReviewCampaign(c.Request().Context(), orgID(c), id); err != nil {
		if isNotFound(err) {
			return errResp(c, http.StatusNotFound, "access review campaign not found", "CK_ACCESS_REVIEW_NOT_FOUND")
		}
		log.Error().Err(err).Msg("delete access review campaign")
		return errResp(c, http.StatusInternalServerError, "failed to delete access review campaign", "CK_DELETE_ACCESS_REVIEW_FAILED")
	}
	return c.NoContent(http.StatusNoContent)
}

// --- Access Review Items ---

// ListAccessReviewItems handles GET /api/v1/vaktcomply/access-reviews/:id/items.
func (h *Handler) ListAccessReviewItems(c echo.Context) error {
	campaignID := c.Param("id")
	items, err := h.service.ListAccessReviewItems(c.Request().Context(), orgID(c), campaignID)
	if err != nil {
		log.Error().Err(err).Str("campaign_id", campaignID).Msg("list access review items")
		return errResp(c, http.StatusInternalServerError, "failed to list access review items", "CK_LIST_ACCESS_REVIEW_ITEMS_FAILED")
	}
	return c.JSON(http.StatusOK, items)
}

// CreateAccessReviewItem handles POST /api/v1/vaktcomply/access-reviews/:id/items.
func (h *Handler) CreateAccessReviewItem(c echo.Context) error {
	campaignID := c.Param("id")
	var in CreateAccessReviewItemInput
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "CK_BAD_REQUEST")
	}
	// Inject campaign_id from path so callers don't have to repeat it in the body
	in.CampaignID = campaignID
	if err := h.validate.Struct(in); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, map[string]string{"error": "Ungültige Eingabe", "code": "VALIDATION_ERROR"})
	}
	item, err := h.service.CreateAccessReviewItem(c.Request().Context(), orgID(c), in)
	if err != nil {
		log.Error().Err(err).Msg("create access review item")
		return errResp(c, http.StatusInternalServerError, "failed to create access review item", "CK_CREATE_ACCESS_REVIEW_ITEM_FAILED")
	}
	return c.JSON(http.StatusCreated, item)
}

// UpdateAccessReviewItem handles PUT /api/v1/vaktcomply/access-reviews/:id/items/:itemId.
func (h *Handler) UpdateAccessReviewItem(c echo.Context) error {
	itemID := c.Param("itemId")
	var in UpdateAccessReviewItemInput
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "CK_BAD_REQUEST")
	}
	if err := h.validate.Struct(in); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, map[string]string{"error": "Ungültige Eingabe", "code": "VALIDATION_ERROR"})
	}
	item, err := h.service.UpdateAccessReviewItem(c.Request().Context(), orgID(c), itemID, in)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return errResp(c, http.StatusNotFound, "access review item not found", "CK_ACCESS_REVIEW_ITEM_NOT_FOUND")
		}
		log.Error().Err(err).Msg("update access review item")
		return errResp(c, http.StatusInternalServerError, "failed to update access review item", "CK_UPDATE_ACCESS_REVIEW_ITEM_FAILED")
	}
	return c.JSON(http.StatusOK, item)
}

func (h *Handler) ListMeasures(c echo.Context) error {
	controlID := c.Param("id")
	if controlID == "" {
		return errResp(c, http.StatusBadRequest, "control id is required", "CK_BAD_REQUEST")
	}
	measures, err := h.service.ListMeasures(c.Request().Context(), orgID(c), controlID)
	if err != nil {
		log.Error().Err(err).Str("control_id", controlID).Msg("list measures")
		return errResp(c, http.StatusInternalServerError, "failed to list measures", "CK_INTERNAL")
	}
	if measures == nil {
		measures = []ControlMeasure{}
	}
	return c.JSON(http.StatusOK, measures)
}

// CreateMeasure handles POST /api/v1/vaktcomply/controls/:id/measures.
func (h *Handler) CreateMeasure(c echo.Context) error {
	controlID := c.Param("id")
	if controlID == "" {
		return errResp(c, http.StatusBadRequest, "control id is required", "CK_BAD_REQUEST")
	}
	var in CreateMeasureInput
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "CK_BAD_REQUEST")
	}
	if err := h.validate.Struct(in); err != nil {
		return errResp(c, http.StatusBadRequest, "Ungültige Eingabe", "VALIDATION_ERROR")
	}
	measure, err := h.service.CreateMeasure(c.Request().Context(), orgID(c), controlID, in)
	if err != nil {
		log.Error().Err(err).Str("control_id", controlID).Msg("create measure")
		return errResp(c, http.StatusInternalServerError, "failed to create measure", "CK_INTERNAL")
	}
	return c.JSON(http.StatusCreated, measure)
}

// UpdateMeasure handles PATCH /api/v1/vaktcomply/controls/:id/measures/:mid.
func (h *Handler) UpdateMeasure(c echo.Context) error {
	measureID := c.Param("mid")
	if measureID == "" {
		return errResp(c, http.StatusBadRequest, "measure id is required", "CK_BAD_REQUEST")
	}
	var in UpdateMeasureInput
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "CK_BAD_REQUEST")
	}
	if err := h.validate.Struct(in); err != nil {
		return errResp(c, http.StatusBadRequest, "Ungültige Eingabe", "VALIDATION_ERROR")
	}
	measure, err := h.service.UpdateMeasure(c.Request().Context(), orgID(c), measureID, in)
	if err != nil {
		log.Error().Err(err).Str("measure_id", measureID).Msg("update measure")
		return errResp(c, http.StatusInternalServerError, "failed to update measure", "CK_INTERNAL")
	}
	return c.JSON(http.StatusOK, measure)
}

// DeleteMeasure handles DELETE /api/v1/vaktcomply/controls/:id/measures/:mid.
func (h *Handler) DeleteMeasure(c echo.Context) error {
	measureID := c.Param("mid")
	if measureID == "" {
		return errResp(c, http.StatusBadRequest, "measure id is required", "CK_BAD_REQUEST")
	}
	if err := h.service.DeleteMeasure(c.Request().Context(), orgID(c), measureID); err != nil {
		log.Error().Err(err).Str("measure_id", measureID).Msg("delete measure")
		if isNotFound(err) {
			return errResp(c, http.StatusNotFound, "measure not found", "CK_MEASURE_NOT_FOUND")
		}
		return errResp(c, http.StatusInternalServerError, "failed to delete measure", "CK_INTERNAL")
	}
	return c.NoContent(http.StatusNoContent)
}

func (h *Handler) ListThreatCatalog(c echo.Context) error {
	items := h.service.ListThreatCatalog(ThreatCatalogFilter{
		Framework: c.QueryParam("framework"),
		AssetType: c.QueryParam("asset_type"),
		CIA:       c.QueryParam("cia"),
	})
	return c.JSON(http.StatusOK, items)
}

// CreateRiskFromCatalog handles POST /api/v1/vaktcomply/threat-catalog/create-risk
func (h *Handler) CreateRiskFromCatalog(c echo.Context) error {
	var in CreateRiskFromCatalogInput
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "CK_BAD_REQUEST")
	}
	if err := h.validate.Struct(in); err != nil {
		return errResp(c, http.StatusUnprocessableEntity, err.Error(), "CK_VALIDATION_ERROR")
	}
	risk, err := h.service.CreateRiskFromCatalog(c.Request().Context(), orgID(c), in, userID(c))
	if err != nil {
		log.Warn().Err(err).Str("catalog_id", in.CatalogID).Msg("create risk from catalog")
		return errResp(c, http.StatusBadRequest, err.Error(), "CK_THREAT_CATALOG_FAILED")
	}
	return c.JSON(http.StatusCreated, risk)
}

func (h *Handler) isOrgAdmin(c echo.Context) (bool, error) {
	uid := userID(c)
	oid := orgID(c)
	if uid == "" || oid == "" {
		return false, nil
	}
	roleName, err := h.service.Audit.GetOrgMemberRole(c.Request().Context(), uid, oid)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, nil
		}
		return false, err
	}
	return roleName == "Admin", nil
}

// ─── Request approval ─────────────────────────────────────────────────────────

// ApprovalRequestInput is the validated body for POST /controls/:id/approval-request.
type ApprovalRequestInput struct {
	RequestedStatus string `json:"requested_status" validate:"required,oneof=missing in_progress implemented not_applicable"`
	Comment         string `json:"comment"          validate:"max=2000"`
}

// RequestControlApproval handles POST /api/v1/vaktcomply/controls/:id/approval-request.
// Non-admin users submit a status-change request; admins get a 409 telling them to use the direct PATCH.
func (h *Handler) RequestControlApproval(c echo.Context) error {
	controlID := c.Param("id")

	var in ApprovalRequestInput
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "CK_BAD_REQUEST")
	}
	if err := h.validate.Struct(in); err != nil {
		return errResp(c, http.StatusUnprocessableEntity, "validation error", "CK_VALIDATION_ERROR")
	}

	admin, err := h.isOrgAdmin(c)
	if err != nil {
		log.Error().Err(err).Msg("check admin role for approval request")
		return errResp(c, http.StatusInternalServerError, "role check failed", "CK_INTERNAL")
	}
	if admin {
		return errResp(c, http.StatusConflict,
			"admins können Status direkt ändern — kein Genehmigungsantrag nötig",
			"CK_APPROVAL_ADMIN_DIRECT",
		)
	}

	// Fetch current control status.
	ctrl, err := h.service.GetControl(c.Request().Context(), orgID(c), controlID)
	if err != nil {
		log.Error().Err(err).Msg("get control for approval request")
		return errResp(c, http.StatusNotFound, "control not found", "CK_NOT_FOUND")
	}

	approval, err := h.service.Audit.CreateApprovalRequest(
		c.Request().Context(),
		orgID(c), controlID, userID(c),
		in.RequestedStatus, ctrl.Status, in.Comment,
	)
	if err != nil {
		log.Error().Err(err).Msg("create approval request")
		return errResp(c, http.StatusInternalServerError, "failed to create approval request", "CK_INTERNAL")
	}

	audit.Write(c.Request().Context(), h.db, audit.WriteEntry{
		OrgID:        orgID(c),
		UserID:       userID(c),
		Action:       "request_approval",
		ResourceType: "vakt-comply/control",
		ResourceID:   controlID,
		ResourceName: ctrl.Title,
		IPAddress:    c.RealIP(),
	})

	return c.JSON(http.StatusCreated, approval)
}

// ─── List pending approvals ───────────────────────────────────────────────────

// ListPendingApprovals handles GET /api/v1/vaktcomply/approvals.
// Admin-only: returns all pending approval requests for the org.
func (h *Handler) ListPendingApprovals(c echo.Context) error {
	admin, err := h.isOrgAdmin(c)
	if err != nil {
		log.Error().Err(err).Msg("check admin role for list approvals")
		return errResp(c, http.StatusInternalServerError, "role check failed", "CK_INTERNAL")
	}
	if !admin {
		return errResp(c, http.StatusForbidden, "admin role required", "CK_FORBIDDEN")
	}

	approvals, err := h.service.Audit.ListPendingApprovals(c.Request().Context(), orgID(c))
	if err != nil {
		log.Error().Err(err).Msg("list pending approvals")
		return errResp(c, http.StatusInternalServerError, "failed to list approvals", "CK_INTERNAL")
	}
	if approvals == nil {
		approvals = []auditmod.ApprovalWithDetails{}
	}
	return c.JSON(http.StatusOK, approvals)
}

// CountPendingApprovals handles GET /api/v1/vaktcomply/approvals/count.
// Returns the number of pending approvals — used for the nav badge.
func (h *Handler) CountPendingApprovals(c echo.Context) error {
	count, err := h.service.Audit.CountPendingApprovals(c.Request().Context(), orgID(c))
	if err != nil {
		log.Error().Err(err).Msg("count pending approvals")
		return errResp(c, http.StatusInternalServerError, "failed to count approvals", "CK_INTERNAL")
	}
	return c.JSON(http.StatusOK, map[string]int{"count": count})
}

// ─── Review helpers ───────────────────────────────────────────────────────────

// ReviewCommentInput is the body for approve/reject endpoints.
type ReviewCommentInput struct {
	Comment string `json:"comment" validate:"max=2000"`
}

func (h *Handler) reviewApproval(c echo.Context, approve bool) error {
	approvalID := c.Param("id")

	admin, err := h.isOrgAdmin(c)
	if err != nil {
		log.Error().Err(err).Msg("check admin role for review approval")
		return errResp(c, http.StatusInternalServerError, "role check failed", "CK_INTERNAL")
	}
	if !admin {
		return errResp(c, http.StatusForbidden, "admin role required", "CK_FORBIDDEN")
	}

	var in ReviewCommentInput
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "CK_BAD_REQUEST")
	}

	if err := h.service.Audit.ReviewApproval(
		c.Request().Context(),
		orgID(c), approvalID, userID(c),
		approve, in.Comment,
	); err != nil {
		log.Error().Err(err).Msg("review approval")
		return errResp(c, http.StatusInternalServerError, "failed to review approval", "CK_INTERNAL")
	}

	action := "reject_approval"
	if approve {
		action = "approve_approval"
	}
	audit.Write(c.Request().Context(), h.db, audit.WriteEntry{
		OrgID:        orgID(c),
		UserID:       userID(c),
		Action:       action,
		ResourceType: "vakt-comply/control-approval",
		ResourceID:   approvalID,
		IPAddress:    c.RealIP(),
	})

	return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
}

// ApproveApproval handles POST /api/v1/vaktcomply/approvals/:id/approve.
func (h *Handler) ApproveApproval(c echo.Context) error { return h.reviewApproval(c, true) }

// RejectApproval handles POST /api/v1/vaktcomply/approvals/:id/reject.
func (h *Handler) RejectApproval(c echo.Context) error { return h.reviewApproval(c, false) }

// ─── Org setting ──────────────────────────────────────────────────────────────

// GetApprovalSetting handles GET /api/v1/vaktcomply/org/approval-setting.
func (h *Handler) GetApprovalSetting(c echo.Context) error {
	required, err := h.service.Audit.OrgApprovalRequired(c.Request().Context(), orgID(c))
	if err != nil {
		log.Error().Err(err).Msg("get approval setting")
		return errResp(c, http.StatusInternalServerError, "failed to get setting", "CK_INTERNAL")
	}
	return c.JSON(http.StatusOK, map[string]bool{"approval_required": required})
}

// UpdateApprovalSettingInput is the body for PUT /api/v1/vaktcomply/org/approval-setting.
type UpdateApprovalSettingInput struct {
	ApprovalRequired bool `json:"approval_required"`
}

// UpdateApprovalSetting handles PUT /api/v1/vaktcomply/org/approval-setting.
// Admin-only: toggles the 4-eyes requirement for the org.
func (h *Handler) UpdateApprovalSetting(c echo.Context) error {
	admin, err := h.isOrgAdmin(c)
	if err != nil {
		log.Error().Err(err).Msg("check admin role for update approval setting")
		return errResp(c, http.StatusInternalServerError, "role check failed", "CK_INTERNAL")
	}
	if !admin {
		return errResp(c, http.StatusForbidden, "admin role required", "CK_FORBIDDEN")
	}

	var in UpdateApprovalSettingInput
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "CK_BAD_REQUEST")
	}

	if err := h.service.Audit.SetOrgApprovalRequired(c.Request().Context(), orgID(c), in.ApprovalRequired); err != nil {
		log.Error().Err(err).Msg("update approval setting")
		return errResp(c, http.StatusInternalServerError, "failed to update setting", "CK_INTERNAL")
	}
	return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handler) GetIncident(c echo.Context) error {
	id := c.Param("id")
	inc, err := h.service.GetIncident(c.Request().Context(), orgID(c), id)
	if err != nil {
		return errResp(c, http.StatusNotFound, "incident not found", "CK_INCIDENT_NOT_FOUND")
	}
	return c.JSON(http.StatusOK, inc)
}

// UpdateIncident handles PATCH /api/v1/vaktcomply/incidents/:id.
func (h *Handler) UpdateIncident(c echo.Context) error {
	id := c.Param("id")
	var in UpdateIncidentInput
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "CK_BAD_REQUEST")
	}
	if err := h.validate.Struct(in); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, map[string]string{"error": "Ungültige Eingabe", "code": "VALIDATION_ERROR"})
	}
	inc, err := h.service.UpdateIncident(c.Request().Context(), orgID(c), id, in)
	if err != nil {
		log.Error().Err(err).Msg("update incident")
		return errResp(c, http.StatusInternalServerError, "failed to update incident", "CK_UPDATE_INCIDENT_FAILED")
	}
	return c.JSON(http.StatusOK, inc)
}

// ListIncidents handles GET /api/v1/vaktcomply/incidents.
func (h *Handler) ListIncidents(c echo.Context) error {
	offset, limit, meta := pagination.FromRequest(c)
	incidents, total, err := h.service.ListIncidentsPaged(c.Request().Context(), orgID(c), offset, limit)
	if err != nil {
		log.Error().Err(err).Msg("list incidents")
		return errResp(c, http.StatusInternalServerError, "failed to list incidents", "CK_LIST_INCIDENTS_FAILED")
	}
	pagination.Complete(&meta, total)
	return c.JSON(http.StatusOK, pagination.Wrap(incidents, meta))
}

// CreateIncident handles POST /api/v1/vaktcomply/incidents.
func (h *Handler) CreateIncident(c echo.Context) error {
	var in CreateIncidentInput
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "CK_BAD_REQUEST")
	}
	if err := h.validate.Struct(in); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, map[string]string{"error": "Ungültige Eingabe", "code": "VALIDATION_ERROR"})
	}
	incident, err := h.service.CreateIncident(c.Request().Context(), orgID(c), in)
	if err != nil {
		log.Error().Err(err).Msg("create incident")
		return errResp(c, http.StatusInternalServerError, "failed to create incident", "CK_CREATE_INCIDENT_FAILED")
	}
	audit.Write(c.Request().Context(), h.db, audit.WriteEntry{
		OrgID:        orgID(c),
		UserID:       userID(c),
		Action:       "create",
		ResourceType: "vakt-comply/incident",
		ResourceID:   incident.ID,
		ResourceName: incident.Title,
		IPAddress:    c.RealIP(),
	})
	return c.JSON(http.StatusCreated, incident)
}

// AssessReportability handles POST /api/v1/vaktcomply/incidents/:id/assess-reportability.
func (h *Handler) AssessReportability(c echo.Context) error {
	id := c.Param("id")
	var in AssessReportabilityInput
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "CK_BAD_REQUEST")
	}
	result, err := h.service.AssessReportability(c.Request().Context(), orgID(c), id, in)
	if err != nil {
		if isNotFound(err) {
			return errResp(c, http.StatusNotFound, "incident not found", "CK_INCIDENT_NOT_FOUND")
		}
		log.Error().Err(err).Str("incident_id", id).Msg("assess reportability")
		return errResp(c, http.StatusInternalServerError, "failed to assess reportability", "CK_ASSESS_FAILED")
	}
	return c.JSON(http.StatusOK, result)
}

// GenerateIncidentReportForm handles POST /api/v1/vaktcomply/incidents/:id/reports.
func (h *Handler) GenerateIncidentReportForm(c echo.Context) error {
	id := c.Param("id")
	var body struct {
		ReportType string `json:"report_type" validate:"required,oneof=24h 72h 30d"`
	}
	if err := c.Bind(&body); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "CK_BAD_REQUEST")
	}
	if err := h.validate.Struct(body); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, map[string]string{"error": "Ungültige Eingabe", "code": "VALIDATION_ERROR"})
	}
	report, _, err := h.service.GenerateIncidentReportForm(c.Request().Context(), orgID(c), id, body.ReportType, orgID(c))
	if err != nil {
		if isNotFound(err) {
			return errResp(c, http.StatusNotFound, "incident not found", "CK_INCIDENT_NOT_FOUND")
		}
		log.Error().Err(err).Str("incident_id", id).Msg("generate incident report form")
		return errResp(c, http.StatusInternalServerError, "failed to generate report", "CK_REPORT_FAILED")
	}
	return c.JSON(http.StatusCreated, report)
}

// ListIncidentReports handles GET /api/v1/vaktcomply/incidents/:id/reports.
func (h *Handler) ListIncidentReports(c echo.Context) error {
	id := c.Param("id")
	reports, err := h.service.ListIncidentReports(c.Request().Context(), orgID(c), id)
	if err != nil {
		log.Error().Err(err).Str("incident_id", id).Msg("list incident reports")
		return errResp(c, http.StatusInternalServerError, "failed to list reports", "CK_LIST_FAILED")
	}
	if reports == nil {
		reports = []IncidentReport{}
	}
	return c.JSON(http.StatusOK, reports)
}

// DownloadIncidentReportPDF handles GET /api/v1/vaktcomply/incident-reports/:reportId/pdf.
func (h *Handler) DownloadIncidentReportPDF(c echo.Context) error {
	reportID := c.Param("reportId")
	pdfBytes, err := h.service.GetIncidentReportPDF(c.Request().Context(), orgID(c), reportID)
	if err != nil {
		if isNotFound(err) {
			return errResp(c, http.StatusNotFound, "report not found", "CK_REPORT_NOT_FOUND")
		}
		log.Error().Err(err).Str("report_id", reportID).Msg("download incident report pdf")
		return errResp(c, http.StatusInternalServerError, "failed to retrieve PDF", "CK_PDF_FAILED")
	}
	c.Response().Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", "nis2-meldung-"+reportID+".pdf"))
	return c.Blob(http.StatusOK, "application/pdf", pdfBytes)
}

// MarkDeadlineReported handles POST /api/v1/vaktcomply/incidents/:id/mark-reported.
func (h *Handler) MarkDeadlineReported(c echo.Context) error {
	id := c.Param("id")
	var in MarkDeadlineReportedInput
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "CK_BAD_REQUEST")
	}
	if err := h.validate.Struct(in); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, map[string]string{"error": "Ungültige Eingabe", "code": "VALIDATION_ERROR"})
	}
	inc, err := h.service.MarkDeadlineReported(c.Request().Context(), orgID(c), id, in.Deadline)
	if err != nil {
		log.Error().Err(err).Msg("mark deadline reported")
		return errResp(c, http.StatusInternalServerError, "failed to mark deadline", "CK_MARK_DEADLINE_FAILED")
	}
	return c.JSON(http.StatusOK, inc)
}

// IncidentReportPDF handles GET /api/v1/vaktcomply/incidents/:id/report-pdf.
// It streams a BaFin-style DORA incident report as a PDF download.
func (h *Handler) IncidentReportPDF(c echo.Context) error {
	id := c.Param("id")
	inc, err := h.service.GetIncident(c.Request().Context(), orgID(c), id)
	if err != nil {
		if isNotFound(err) {
			return errResp(c, http.StatusNotFound, "incident not found", "CK_INCIDENT_NOT_FOUND")
		}
		log.Error().Err(err).Str("incident_id", id).Msg("get incident for pdf")
		return errResp(c, http.StatusInternalServerError, "failed to retrieve incident", "CK_GET_INCIDENT_FAILED")
	}

	// Use org_id as a stand-in for org name when no name is available via context.
	// In production the org name can be resolved from the claims or a lookup.
	org := orgID(c)

	pdfBytes, err := GenerateIncidentReportPDF(inc, org)
	if err != nil {
		log.Error().Err(err).Str("incident_id", id).Msg("generate incident report pdf")
		return errResp(c, http.StatusInternalServerError, "failed to generate PDF", "CK_PDF_FAILED")
	}

	filename := fmt.Sprintf("incident-%s-bafin.pdf", inc.ID)
	c.Response().Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
	return c.Blob(http.StatusOK, "application/pdf", pdfBytes)
}

// ClassifyReportingObligation handles POST /api/v1/vaktcomply/incidents/:id/classify-reporting.
// S39-1: 3-question BSI meldepflicht wizard — returns obligation + authority + reason.
func (h *Handler) ClassifyReportingObligation(c echo.Context) error {
	id := c.Param("id")
	var in ClassifyReportingInput
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "CK_BAD_REQUEST")
	}
	result, err := h.service.ClassifyReportingObligation(c.Request().Context(), orgID(c), id, in)
	if err != nil {
		if isNotFound(err) {
			return errResp(c, http.StatusNotFound, "incident not found", "CK_INCIDENT_NOT_FOUND")
		}
		log.Error().Err(err).Str("incident_id", id).Msg("classify reporting obligation")
		return errResp(c, http.StatusInternalServerError, "failed to classify reporting obligation", "CK_CLASSIFY_FAILED")
	}
	return c.JSON(http.StatusOK, result)
}

// NIS2ReportingEnabled handles GET /api/v1/vaktcomply/nis2/enabled.
// License probe for the NIS2 reporting feature — the route itself is gated.
func (h *Handler) NIS2ReportingEnabled(c echo.Context) error {
	return c.JSON(http.StatusOK, map[string]bool{"enabled": true})
}

// NIS2AssessReportability handles POST /api/v1/vaktcomply/incidents/:id/nis2/assess.
// Stores the NIS2 meldepflicht assessment and sets deadline timers.
func (h *Handler) NIS2AssessReportability(c echo.Context) error {
	id := c.Param("id")
	var in struct {
		NIS2ReportabilityCheck
		DetectedAt *string `json:"detected_at"`
	}
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "CK_BAD_REQUEST")
	}

	detectedAt := time.Now().UTC()
	if in.DetectedAt != nil {
		if t, err := time.Parse(time.RFC3339, *in.DetectedAt); err == nil {
			detectedAt = t
		}
	}

	incidentID, err := uuid.Parse(id)
	if err != nil {
		return errResp(c, http.StatusBadRequest, "invalid incident id", "CK_BAD_REQUEST")
	}

	if err := h.service.MarkIncidentReportable(c.Request().Context(), orgID(c), incidentID, detectedAt, in.NIS2ReportabilityCheck); err != nil {
		if isNotFound(err) {
			return errResp(c, http.StatusNotFound, "incident not found", "CK_INCIDENT_NOT_FOUND")
		}
		log.Error().Err(err).Str("incident_id", id).Msg("nis2 assess reportability")
		return errResp(c, http.StatusInternalServerError, "failed to assess reportability", "CK_ASSESS_FAILED")
	}
	return c.JSON(http.StatusOK, map[string]any{
		"is_reportable": in.IsReportable(),
	})
}

// NIS2Status handles GET /api/v1/vaktcomply/incidents/:id/nis2-status.
func (h *Handler) NIS2Status(c echo.Context) error {
	id := c.Param("id")
	status, err := h.service.GetNIS2Status(c.Request().Context(), orgID(c), id)
	if err != nil {
		if isNotFound(err) {
			return errResp(c, http.StatusNotFound, "incident not found", "CK_INCIDENT_NOT_FOUND")
		}
		log.Error().Err(err).Str("incident_id", id).Msg("get nis2 status")
		return errResp(c, http.StatusInternalServerError, "failed to get nis2 status", "CK_NIS2_STATUS_FAILED")
	}
	return c.JSON(http.StatusOK, status)
}

// NIS2SubmitStage handles POST /api/v1/vaktcomply/incidents/:id/nis2/submit/:stage.
func (h *Handler) NIS2SubmitStage(c echo.Context) error {
	id := c.Param("id")
	stage := c.Param("stage")
	var in NIS2ReportInput
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "CK_BAD_REQUEST")
	}

	report, err := h.service.SubmitNIS2Stage(c.Request().Context(), orgID(c), id, userID(c), stage, in)
	if err != nil {
		log.Error().Err(err).Str("incident_id", id).Str("stage", stage).Msg("submit nis2 stage")
		return errResp(c, http.StatusInternalServerError, "failed to submit nis2 stage", "CK_NIS2_SUBMIT_FAILED")
	}
	return c.JSON(http.StatusOK, report)
}

// ListAuthorityContacts handles GET /api/v1/vaktcomply/authority-contacts.
func (h *Handler) ListAuthorityContacts(c echo.Context) error {
	contacts, err := h.service.ListAuthorityContacts(c.Request().Context(), orgID(c))
	if err != nil {
		log.Error().Err(err).Msg("list authority contacts")
		return errResp(c, http.StatusInternalServerError, "failed to list authority contacts", "CK_LIST_AUTH_CONTACTS_FAILED")
	}
	return c.JSON(http.StatusOK, contacts)
}

// CreateAuthorityContact handles POST /api/v1/vaktcomply/authority-contacts.
func (h *Handler) CreateAuthorityContact(c echo.Context) error {
	var in AuthorityContact
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "CK_BAD_REQUEST")
	}
	contact, err := h.service.CreateAuthorityContact(c.Request().Context(), orgID(c), in)
	if err != nil {
		log.Error().Err(err).Msg("create authority contact")
		return errResp(c, http.StatusInternalServerError, "failed to create authority contact", "CK_CREATE_AUTH_CONTACT_FAILED")
	}
	return c.JSON(http.StatusCreated, contact)
}

func (h *Handler) ListSuppliers(c echo.Context) error {
	filter := &SupplierFilter{
		Criticality:      c.QueryParam("criticality"),
		AssessmentStatus: c.QueryParam("assessment_status"),
	}
	if filter.Criticality == "" && filter.AssessmentStatus == "" {
		filter = nil
	}
	suppliers, err := h.service.ListSuppliers(c.Request().Context(), orgID(c), filter)
	if err != nil {
		log.Error().Err(err).Msg("list suppliers")
		return errResp(c, http.StatusInternalServerError, "failed to list suppliers", "CK_LIST_SUPPLIERS_FAILED")
	}
	return c.JSON(http.StatusOK, suppliers)
}

// CreateSupplier handles POST /api/v1/vaktcomply/suppliers.
func (h *Handler) CreateSupplier(c echo.Context) error {
	var in CreateSupplierInput
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "CK_BAD_REQUEST")
	}
	if err := h.validate.Struct(in); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, map[string]string{"error": "Ungültige Eingabe", "code": "VALIDATION_ERROR"})
	}
	s, err := h.service.CreateSupplier(c.Request().Context(), orgID(c), in)
	if err != nil {
		log.Error().Err(err).Msg("create supplier")
		return errResp(c, http.StatusInternalServerError, "failed to create supplier", "CK_CREATE_SUPPLIER_FAILED")
	}
	return c.JSON(http.StatusCreated, s)
}

// GetSupplier handles GET /api/v1/vaktcomply/suppliers/:id.
func (h *Handler) GetSupplier(c echo.Context) error {
	id := c.Param("id")
	if _, err := uuid.Parse(id); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid supplier id", "CK_BAD_REQUEST")
	}
	s, err := h.service.GetSupplier(c.Request().Context(), orgID(c), id)
	if err != nil {
		return errResp(c, http.StatusNotFound, "supplier not found", "CK_SUPPLIER_NOT_FOUND")
	}
	return c.JSON(http.StatusOK, s)
}

// UpdateSupplier handles PATCH /api/v1/vaktcomply/suppliers/:id.
func (h *Handler) UpdateSupplier(c echo.Context) error {
	id := c.Param("id")
	if _, err := uuid.Parse(id); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid supplier id", "CK_BAD_REQUEST")
	}
	var in UpdateSupplierInput
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "CK_BAD_REQUEST")
	}
	if err := h.validate.Struct(in); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, map[string]string{"error": "Ungültige Eingabe", "code": "VALIDATION_ERROR"})
	}
	s, err := h.service.UpdateSupplier(c.Request().Context(), orgID(c), id, in)
	if err != nil {
		log.Error().Err(err).Msg("update supplier")
		return errResp(c, http.StatusInternalServerError, "failed to update supplier", "CK_UPDATE_SUPPLIER_FAILED")
	}
	return c.JSON(http.StatusOK, s)
}

// DeleteSupplier handles DELETE /api/v1/vaktcomply/suppliers/:id.
func (h *Handler) DeleteSupplier(c echo.Context) error {
	id := c.Param("id")
	if _, err := uuid.Parse(id); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid supplier id", "CK_BAD_REQUEST")
	}
	if err := h.service.DeleteSupplier(c.Request().Context(), orgID(c), id); err != nil {
		if isNotFound(err) {
			return errResp(c, http.StatusNotFound, "supplier not found", "CK_SUPPLIER_NOT_FOUND")
		}
		log.Error().Err(err).Str("id", id).Msg("delete supplier")
		return errResp(c, http.StatusInternalServerError, "failed to delete supplier", "CK_DELETE_SUPPLIER_FAILED")
	}
	return c.NoContent(http.StatusNoContent)
}

// GetSupplierIncidents handles GET /api/v1/vaktcomply/suppliers/:id/incidents.
func (h *Handler) GetSupplierIncidents(c echo.Context) error {
	supplierID := c.Param("id")
	if _, err := uuid.Parse(supplierID); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid supplier id"})
	}
	incidents, err := h.service.ListIncidentsBySupplier(c.Request().Context(), orgID(c), supplierID)
	if err != nil {
		log.Error().Err(err).Str("supplier_id", supplierID).Msg("get supplier incidents")
		return errResp(c, http.StatusInternalServerError, "failed to list supplier incidents", "CK_LIST_SUPPLIER_INCIDENTS_FAILED")
	}
	return c.JSON(http.StatusOK, incidents)
}

// ExportSuppliers handles GET /api/v1/vaktcomply/suppliers/export.
// Returns a CSV file with all suppliers for the organisation.
func (h *Handler) ExportSuppliers(c echo.Context) error {
	suppliers, err := h.service.ListSuppliers(c.Request().Context(), orgID(c), nil)
	if err != nil {
		log.Error().Err(err).Msg("export suppliers: list suppliers")
		return errResp(c, http.StatusInternalServerError, "failed to list suppliers", "CK_LIST_SUPPLIERS_FAILED")
	}
	data, err := GenerateSupplierCSV(suppliers)
	if err != nil {
		log.Error().Err(err).Msg("export suppliers: generate csv")
		return errResp(c, http.StatusInternalServerError, "failed to generate CSV", "CK_EXPORT_SUPPLIERS_FAILED")
	}
	c.Response().Header().Set("Content-Disposition", `attachment; filename=suppliers-export.csv`)
	return c.Blob(http.StatusOK, "text/csv", data)
}

// ImportSuppliersCSV handles POST /api/v1/vaktcomply/suppliers/import-csv.
// Accepts a multipart form with field "file" containing a CSV.
func (h *Handler) ImportSuppliersCSV(c echo.Context) error {
	if err := c.Request().ParseMultipartForm(10 << 20); err != nil { // 10 MB
		return errResp(c, http.StatusBadRequest, "failed to parse multipart form", "CK_BAD_REQUEST")
	}
	file, _, err := c.Request().FormFile("file")
	if err != nil {
		return errResp(c, http.StatusBadRequest, "missing file field in multipart form", "CK_BAD_REQUEST")
	}
	defer file.Close()

	result, err := h.service.ParseAndImportSupplierCSV(c.Request().Context(), orgID(c), file)
	if err != nil {
		log.Error().Err(err).Msg("import suppliers csv")
		return errResp(c, http.StatusInternalServerError, "failed to import CSV", "CK_IMPORT_SUPPLIERS_FAILED")
	}
	return c.JSON(http.StatusOK, result)
}

// LinkSupplierRisk handles POST /api/v1/vaktcomply/suppliers/:id/risks.
func (h *Handler) LinkSupplierRisk(c echo.Context) error {
	supplierID := c.Param("id")
	if _, err := uuid.Parse(supplierID); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid supplier id", "CK_BAD_REQUEST")
	}
	var body struct {
		RiskID string `json:"risk_id"`
	}
	if err := c.Bind(&body); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "CK_BAD_REQUEST")
	}
	if _, err := uuid.Parse(body.RiskID); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid risk_id", "CK_BAD_REQUEST")
	}
	if err := h.service.LinkSupplierRisk(c.Request().Context(), orgID(c), supplierID, body.RiskID); err != nil {
		log.Error().Err(err).Str("supplier_id", supplierID).Str("risk_id", body.RiskID).Msg("link supplier risk")
		if isNotFound(err) {
			return errResp(c, http.StatusNotFound, err.Error(), "CK_NOT_FOUND")
		}
		return errResp(c, http.StatusInternalServerError, "failed to link risk", "CK_LINK_SUPPLIER_RISK_FAILED")
	}
	return c.NoContent(http.StatusNoContent)
}

// UnlinkSupplierRisk handles DELETE /api/v1/vaktcomply/suppliers/:id/risks/:riskId.
func (h *Handler) UnlinkSupplierRisk(c echo.Context) error {
	supplierID := c.Param("id")
	riskID := c.Param("riskId")
	if _, err := uuid.Parse(supplierID); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid supplier id", "CK_BAD_REQUEST")
	}
	if _, err := uuid.Parse(riskID); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid risk id", "CK_BAD_REQUEST")
	}
	if err := h.service.UnlinkSupplierRisk(c.Request().Context(), orgID(c), supplierID, riskID); err != nil {
		log.Error().Err(err).Str("supplier_id", supplierID).Str("risk_id", riskID).Msg("unlink supplier risk")
		if isNotFound(err) {
			return errResp(c, http.StatusNotFound, err.Error(), "CK_NOT_FOUND")
		}
		return errResp(c, http.StatusInternalServerError, "failed to unlink risk", "CK_UNLINK_SUPPLIER_RISK_FAILED")
	}
	return c.NoContent(http.StatusNoContent)
}

// ListSupplierRisks handles GET /api/v1/vaktcomply/suppliers/:id/risks.
func (h *Handler) ListSupplierRisks(c echo.Context) error {
	supplierID := c.Param("id")
	if _, err := uuid.Parse(supplierID); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid supplier id", "CK_BAD_REQUEST")
	}
	risks, err := h.service.ListSupplierRisks(c.Request().Context(), orgID(c), supplierID)
	if err != nil {
		log.Error().Err(err).Str("supplier_id", supplierID).Msg("list supplier risks")
		if isNotFound(err) {
			return errResp(c, http.StatusNotFound, err.Error(), "CK_NOT_FOUND")
		}
		return errResp(c, http.StatusInternalServerError, "failed to list supplier risks", "CK_LIST_SUPPLIER_RISKS_FAILED")
	}
	return c.JSON(http.StatusOK, risks)
}

// ListTemplates handles GET /api/v1/vaktcomply/questionnaires/templates.
func (h *Handler) ListTemplates(c echo.Context) error {
	templates, err := h.service.ListTemplates(c.Request().Context(), orgID(c))
	if err != nil {
		log.Error().Err(err).Msg("list templates")
		return errResp(c, http.StatusInternalServerError, "failed to list templates", "CK_LIST_TEMPLATES_FAILED")
	}
	return c.JSON(http.StatusOK, templates)
}

// ListQuestionnaires handles GET /api/v1/vaktcomply/questionnaires.
func (h *Handler) ListQuestionnaires(c echo.Context) error {
	var isTemplate *bool
	if raw := c.QueryParam("is_template"); raw != "" {
		v := raw == "true"
		isTemplate = &v
	}
	questionnaires, err := h.service.ListQuestionnaires(c.Request().Context(), orgID(c), isTemplate)
	if err != nil {
		log.Error().Err(err).Msg("list questionnaires")
		return errResp(c, http.StatusInternalServerError, "failed to list questionnaires", "CK_LIST_QUESTIONNAIRES_FAILED")
	}
	return c.JSON(http.StatusOK, questionnaires)
}

// CreateQuestionnaire handles POST /api/v1/vaktcomply/questionnaires.
func (h *Handler) CreateQuestionnaire(c echo.Context) error {
	var in CreateQuestionnaireInput
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "CK_BAD_REQUEST")
	}
	if err := h.validate.Struct(in); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, map[string]string{"error": "Ungültige Eingabe", "code": "VALIDATION_ERROR"})
	}
	if in.CloneFromID != "" {
		if _, err := uuid.Parse(in.CloneFromID); err != nil {
			return errResp(c, http.StatusBadRequest, "invalid clone_from_id", "CK_BAD_REQUEST")
		}
	}
	q, err := h.service.CreateQuestionnaire(c.Request().Context(), orgID(c), in)
	if err != nil {
		log.Error().Err(err).Msg("create questionnaire")
		return errResp(c, http.StatusInternalServerError, "failed to create questionnaire", "CK_CREATE_QUESTIONNAIRE_FAILED")
	}
	return c.JSON(http.StatusCreated, q)
}

// GetQuestionnaire handles GET /api/v1/vaktcomply/questionnaires/:id.
func (h *Handler) GetQuestionnaire(c echo.Context) error {
	id := c.Param("id")
	if _, err := uuid.Parse(id); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid questionnaire id", "CK_BAD_REQUEST")
	}
	q, err := h.service.GetQuestionnaire(c.Request().Context(), orgID(c), id)
	if err != nil {
		return errResp(c, http.StatusNotFound, "questionnaire not found", "CK_QUESTIONNAIRE_NOT_FOUND")
	}
	return c.JSON(http.StatusOK, q)
}

// UpdateQuestionnaire handles PATCH /api/v1/vaktcomply/questionnaires/:id.
func (h *Handler) UpdateQuestionnaire(c echo.Context) error {
	id := c.Param("id")
	if _, err := uuid.Parse(id); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid questionnaire id", "CK_BAD_REQUEST")
	}
	var in UpdateQuestionnaireInput
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "CK_BAD_REQUEST")
	}
	if err := h.validate.Struct(in); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, map[string]string{"error": "Ungültige Eingabe", "code": "VALIDATION_ERROR"})
	}
	q, err := h.service.UpdateQuestionnaire(c.Request().Context(), orgID(c), id, in)
	if err != nil {
		if isNotFound(err) {
			return errResp(c, http.StatusNotFound, "questionnaire not found", "CK_QUESTIONNAIRE_NOT_FOUND")
		}
		log.Error().Err(err).Str("id", id).Msg("update questionnaire")
		return errResp(c, http.StatusInternalServerError, "failed to update questionnaire", "CK_UPDATE_QUESTIONNAIRE_FAILED")
	}
	return c.JSON(http.StatusOK, q)
}

// DeleteQuestionnaire handles DELETE /api/v1/vaktcomply/questionnaires/:id.
func (h *Handler) DeleteQuestionnaire(c echo.Context) error {
	id := c.Param("id")
	if _, err := uuid.Parse(id); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid questionnaire id", "CK_BAD_REQUEST")
	}
	if err := h.service.DeleteQuestionnaire(c.Request().Context(), orgID(c), id); err != nil {
		log.Error().Err(err).Str("id", id).Msg("delete questionnaire")
		if isNotFound(err) {
			return errResp(c, http.StatusNotFound, "questionnaire not found", "CK_QUESTIONNAIRE_NOT_FOUND")
		}
		return errResp(c, http.StatusInternalServerError, "failed to delete questionnaire", "CK_DELETE_QUESTIONNAIRE_FAILED")
	}
	return c.NoContent(http.StatusNoContent)
}

// AddQuestion handles POST /api/v1/vaktcomply/questionnaires/:id/questions.
func (h *Handler) AddQuestion(c echo.Context) error {
	id := c.Param("id")
	if _, err := uuid.Parse(id); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid questionnaire id", "CK_BAD_REQUEST")
	}
	var in CreateQuestionInput
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "CK_BAD_REQUEST")
	}
	if err := h.validate.Struct(in); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, map[string]string{"error": "Ungültige Eingabe", "code": "VALIDATION_ERROR"})
	}
	if in.ControlID != "" {
		if _, err := uuid.Parse(in.ControlID); err != nil {
			return errResp(c, http.StatusBadRequest, "invalid control_id", "CK_BAD_REQUEST")
		}
	}
	q, err := h.service.AddQuestion(c.Request().Context(), orgID(c), id, in)
	if err != nil {
		if errors.Is(err, ErrInvalidOptions) {
			return errResp(c, http.StatusUnprocessableEntity, err.Error(), "CK_VALIDATION_ERROR")
		}
		log.Error().Err(err).Str("questionnaire_id", id).Msg("add question")
		return errResp(c, http.StatusInternalServerError, "failed to add question", "CK_ADD_QUESTION_FAILED")
	}
	return c.JSON(http.StatusCreated, q)
}

// UpdateQuestion handles PATCH /api/v1/vaktcomply/questionnaires/:id/questions/:qid.
func (h *Handler) UpdateQuestion(c echo.Context) error {
	id := c.Param("id")
	qid := c.Param("qid")
	if _, err := uuid.Parse(id); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid questionnaire id", "CK_BAD_REQUEST")
	}
	if _, err := uuid.Parse(qid); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid question id", "CK_BAD_REQUEST")
	}
	var in CreateQuestionInput
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "CK_BAD_REQUEST")
	}
	if err := h.validate.Struct(in); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, map[string]string{"error": "Ungültige Eingabe", "code": "VALIDATION_ERROR"})
	}
	if in.ControlID != "" {
		if _, err := uuid.Parse(in.ControlID); err != nil {
			return errResp(c, http.StatusBadRequest, "invalid control_id", "CK_BAD_REQUEST")
		}
	}
	q, err := h.service.UpdateQuestion(c.Request().Context(), orgID(c), id, qid, in)
	if err != nil {
		if errors.Is(err, ErrInvalidOptions) {
			return errResp(c, http.StatusUnprocessableEntity, err.Error(), "CK_VALIDATION_ERROR")
		}
		log.Error().Err(err).Str("questionnaire_id", id).Str("question_id", qid).Msg("update question")
		return errResp(c, http.StatusInternalServerError, "failed to update question", "CK_UPDATE_QUESTION_FAILED")
	}
	return c.JSON(http.StatusOK, q)
}

// DeleteQuestion handles DELETE /api/v1/vaktcomply/questionnaires/:id/questions/:qid.
func (h *Handler) DeleteQuestion(c echo.Context) error {
	id := c.Param("id")
	qid := c.Param("qid")
	if _, err := uuid.Parse(id); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid questionnaire id", "CK_BAD_REQUEST")
	}
	if _, err := uuid.Parse(qid); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid question id", "CK_BAD_REQUEST")
	}
	if err := h.service.DeleteQuestion(c.Request().Context(), orgID(c), id, qid); err != nil {
		log.Error().Err(err).Str("questionnaire_id", id).Str("question_id", qid).Msg("delete question")
		if isNotFound(err) {
			return errResp(c, http.StatusNotFound, "question not found", "CK_QUESTION_NOT_FOUND")
		}
		return errResp(c, http.StatusInternalServerError, "failed to delete question", "CK_DELETE_QUESTION_FAILED")
	}
	return c.NoContent(http.StatusNoContent)
}

// ReorderQuestions handles POST /api/v1/vaktcomply/questionnaires/:id/questions/reorder.
func (h *Handler) ReorderQuestions(c echo.Context) error {
	id := c.Param("id")
	if _, err := uuid.Parse(id); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid questionnaire id", "CK_BAD_REQUEST")
	}
	var in ReorderQuestionsInput
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "CK_BAD_REQUEST")
	}
	if err := h.validate.Struct(in); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, map[string]string{"error": "Ungültige Eingabe", "code": "VALIDATION_ERROR"})
	}
	for _, qid := range in.Order {
		if _, err := uuid.Parse(qid); err != nil {
			return errResp(c, http.StatusBadRequest, fmt.Sprintf("invalid question id in order: %s", qid), "CK_BAD_REQUEST")
		}
	}
	if err := h.service.ReorderQuestions(c.Request().Context(), orgID(c), id, in.Order); err != nil {
		log.Error().Err(err).Str("questionnaire_id", id).Msg("reorder questions")
		return errResp(c, http.StatusInternalServerError, "failed to reorder questions", "CK_REORDER_QUESTIONS_FAILED")
	}
	return c.NoContent(http.StatusNoContent)
}

// CreateSupplierAssessment handles POST /api/v1/vaktcomply/suppliers/:id/assessments.
func (h *Handler) CreateSupplierAssessment(c echo.Context) error {
	supplierID := c.Param("id")
	if _, err := uuid.Parse(supplierID); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid supplier id", "CK_BAD_REQUEST")
	}
	var in CreateAssessmentInput
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "CK_BAD_REQUEST")
	}
	if err := h.validate.Struct(in); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, map[string]string{"error": "Ungültige Eingabe", "code": "VALIDATION_ERROR"})
	}

	req := c.Request()
	scheme := "https"
	if req.TLS == nil {
		scheme = "http"
	}
	baseURL := scheme + "://" + req.Host

	assessment, _, err := h.service.CreateAssessment(c.Request().Context(), orgID(c), supplierID, in, baseURL)
	if err != nil {
		log.Error().Err(err).Str("supplier_id", supplierID).Msg("create supplier assessment")
		return errResp(c, http.StatusInternalServerError, "failed to create assessment", "CK_CREATE_ASSESSMENT_FAILED")
	}
	return c.JSON(http.StatusCreated, map[string]string{
		"id":        assessment.ID,
		"share_url": assessment.ShareURL,
	})
}

// ListSupplierAssessments handles GET /api/v1/vaktcomply/suppliers/:id/assessments.
func (h *Handler) ListSupplierAssessments(c echo.Context) error {
	supplierID := c.Param("id")
	if _, err := uuid.Parse(supplierID); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid supplier id", "CK_BAD_REQUEST")
	}
	assessments, err := h.service.ListAssessmentsForSupplier(c.Request().Context(), orgID(c), supplierID)
	if err != nil {
		log.Error().Err(err).Str("supplier_id", supplierID).Msg("list supplier assessments")
		return errResp(c, http.StatusInternalServerError, "failed to list assessments", "CK_LIST_ASSESSMENTS_FAILED")
	}
	if assessments == nil {
		assessments = []Assessment{}
	}
	return c.JSON(http.StatusOK, assessments)
}

// GetAssessment handles GET /api/v1/vaktcomply/assessments/:id.
func (h *Handler) GetAssessment(c echo.Context) error {
	id := c.Param("id")
	if _, err := uuid.Parse(id); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid assessment id", "CK_BAD_REQUEST")
	}
	a, err := h.service.GetAssessment(c.Request().Context(), orgID(c), id)
	if err != nil {
		return errResp(c, http.StatusNotFound, "assessment not found", "CK_ASSESSMENT_NOT_FOUND")
	}
	return c.JSON(http.StatusOK, a)
}

// PortalGetAssessment handles GET /supplier/:token (public, no auth).
func (h *Handler) PortalGetAssessment(c echo.Context) error {
	token := c.Param("token")
	a, err := h.service.GetAssessmentForPortal(c.Request().Context(), token)
	if err != nil {
		if errors.Is(err, ErrAssessmentExpiredOrSubmitted) {
			return c.JSON(http.StatusGone, map[string]string{"error": "assessment_expired_or_submitted"})
		}
		log.Error().Err(err).Msg("portal get assessment")
		return errResp(c, http.StatusInternalServerError, "failed to load assessment", "CK_ASSESSMENT_LOAD_FAILED")
	}
	return c.JSON(http.StatusOK, a)
}

// PortalSaveAnswers handles POST /supplier/:token/save (public, no auth).
func (h *Handler) PortalSaveAnswers(c echo.Context) error {
	token := c.Param("token")
	var in SaveAnswersInput
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "CK_BAD_REQUEST")
	}
	if err := h.validate.Struct(in); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, map[string]string{"error": "Ungültige Eingabe", "code": "VALIDATION_ERROR"})
	}
	if err := h.service.SaveAnswers(c.Request().Context(), token, in); err != nil {
		if errors.Is(err, ErrAssessmentExpiredOrSubmitted) {
			return c.JSON(http.StatusGone, map[string]string{"error": "assessment_expired_or_submitted"})
		}
		log.Error().Err(err).Msg("portal save answers")
		return errResp(c, http.StatusInternalServerError, "failed to save answers", "CK_SAVE_ANSWERS_FAILED")
	}
	return c.JSON(http.StatusOK, map[string]string{"status": "saved"})
}

// PortalSubmitAssessment handles POST /supplier/:token/submit (public, no auth).
func (h *Handler) PortalSubmitAssessment(c echo.Context) error {
	token := c.Param("token")
	var in SaveAnswersInput
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "CK_BAD_REQUEST")
	}
	if err := h.validate.Struct(in); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, map[string]string{"error": "Ungültige Eingabe", "code": "VALIDATION_ERROR"})
	}
	clientIP := c.RealIP()
	userAgent := c.Request().Header.Get("User-Agent")
	if len(userAgent) > 512 {
		userAgent = userAgent[:512]
	}
	if err := h.service.SubmitAssessment(c.Request().Context(), token, clientIP, userAgent, in); err != nil {
		if errors.Is(err, ErrAssessmentExpiredOrSubmitted) {
			return c.JSON(http.StatusGone, map[string]string{"error": "assessment_expired_or_submitted"})
		}
		log.Error().Err(err).Msg("portal submit assessment")
		return errResp(c, http.StatusInternalServerError, "failed to submit assessment", "CK_SUBMIT_ASSESSMENT_FAILED")
	}
	return c.JSON(http.StatusOK, map[string]string{"status": "submitted"})
}

// PortalUploadFile handles POST /supplier/:token/upload (public, no auth).
// Accepts a file (max 20 MB, allowed MIMEs: PDF/PNG/JPEG/XLSX).
func (h *Handler) PortalUploadFile(c echo.Context) error {
	token := c.Param("token")

	// Validate token is for a live assessment.
	a, err := h.service.GetAssessmentForPortal(c.Request().Context(), token)
	if err != nil {
		if errors.Is(err, ErrAssessmentExpiredOrSubmitted) {
			return c.JSON(http.StatusGone, map[string]string{"error": "assessment_expired_or_submitted"})
		}
		log.Error().Err(err).Msg("portal upload: validate token")
		return errResp(c, http.StatusInternalServerError, "failed to validate assessment", "CK_ASSESSMENT_LOAD_FAILED")
	}

	const maxUploadSize = 20 << 20 // 20 MB
	if err := c.Request().ParseMultipartForm(maxUploadSize); err != nil {
		return errResp(c, http.StatusBadRequest, "failed to parse multipart form", "CK_BAD_REQUEST")
	}

	fh, err := c.FormFile("file")
	if err != nil {
		return errResp(c, http.StatusBadRequest, "file is required", "CK_BAD_REQUEST")
	}
	if fh.Size > maxUploadSize {
		return errResp(c, http.StatusRequestEntityTooLarge, "file exceeds 20 MB limit", "CK_FILE_TOO_LARGE")
	}

	src, err := fh.Open()
	if err != nil {
		return errResp(c, http.StatusInternalServerError, "failed to open uploaded file", "CK_UPLOAD_FAILED")
	}
	defer src.Close()

	// Read first 512 bytes for MIME detection.
	buf := make([]byte, 512)
	n, _ := src.Read(buf)
	detectedMIME := http.DetectContentType(buf[:n])

	ext := strings.ToLower(filepath.Ext(fh.Filename))
	allowedMIMEs := map[string]bool{
		"application/pdf": true,
		"image/png":       true,
		"image/jpeg":      true,
	}
	// XLSX is a ZIP archive: http.DetectContentType returns "application/zip".
	// Accept only when extension AND detected type agree to prevent file-rename bypass.
	xlsxAllowed := ext == ".xlsx" && detectedMIME == "application/zip"
	if !allowedMIMEs[detectedMIME] && !xlsxAllowed {
		return errResp(c, http.StatusUnsupportedMediaType, "unsupported file type", "CK_UNSUPPORTED_MIME")
	}

	uploadDir := h.uploadDir
	if uploadDir == "" {
		uploadDir = "./data/uploads"
	}
	assessmentDir := filepath.Join(uploadDir, "supplier-assessments", a.ID)
	if err := os.MkdirAll(assessmentDir, 0o750); err != nil {
		return errResp(c, http.StatusInternalServerError, "failed to create upload directory", "CK_UPLOAD_FAILED")
	}

	destName := uuid.New().String() + ext
	destPath := filepath.Join(assessmentDir, destName)

	dst, err := os.Create(destPath)
	if err != nil {
		return errResp(c, http.StatusInternalServerError, "failed to save file", "CK_UPLOAD_FAILED")
	}
	defer dst.Close()

	// Write already-read bytes first.
	if _, err := dst.Write(buf[:n]); err != nil {
		return errResp(c, http.StatusInternalServerError, "failed to write file", "CK_UPLOAD_FAILED")
	}
	if _, err := io.Copy(dst, src); err != nil {
		return errResp(c, http.StatusInternalServerError, "failed to write file", "CK_UPLOAD_FAILED")
	}

	// Return a relative URL rather than the raw filesystem path.
	fileURL := "/uploads/supplier-assessments/" + a.ID + "/" + destName
	return c.JSON(http.StatusOK, map[string]string{"file_url": fileURL})
}

// ReviewAnswer handles PATCH /vaktcomply/assessments/:id/answers/:aid.
func (h *Handler) ReviewAnswer(c echo.Context) error {
	orgID := orgID(c)
	assessmentID := c.Param("id")
	answerID := c.Param("aid")
	if _, err := uuid.Parse(assessmentID); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid assessment ID", "CK_INVALID_ID")
	}
	if _, err := uuid.Parse(answerID); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid answer ID", "CK_INVALID_ID")
	}
	var in ReviewAnswerInput
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "CK_BAD_REQUEST")
	}
	if err := h.validate.Struct(in); err != nil {
		return errResp(c, http.StatusBadRequest, "Ungültige Eingabe", "VALIDATION_ERROR")
	}
	evidenceID, err := h.service.ReviewAnswer(c.Request().Context(), orgID, assessmentID, answerID, in)
	if err != nil {
		if isNotFound(err) {
			return errResp(c, http.StatusNotFound, "answer not found", "CK_NOT_FOUND")
		}
		return errResp(c, http.StatusBadRequest, err.Error(), "CK_VALIDATION_ERROR")
	}
	resp := map[string]any{"ok": true}
	if evidenceID != nil {
		resp["evidence_id"] = *evidenceID
	}
	return c.JSON(http.StatusOK, resp)
}

// GetSupplierStatus handles GET /vaktcomply/suppliers/:id/status.
func (h *Handler) GetSupplierStatus(c echo.Context) error {
	orgID := orgID(c)
	supplierID := c.Param("id")
	if _, err := uuid.Parse(supplierID); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid supplier ID", "CK_INVALID_ID")
	}
	status, err := h.service.ComputeSupplierStatus(c.Request().Context(), orgID, supplierID)
	if err != nil {
		if isNotFound(err) {
			return errResp(c, http.StatusNotFound, "supplier not found", "CK_NOT_FOUND")
		}
		return errResp(c, http.StatusInternalServerError, "failed to compute status", "CK_INTERNAL")
	}
	return c.JSON(http.StatusOK, status)
}

// UpdateAssessment handles PATCH /vaktcomply/assessments/:id (status=reviewed only).
func (h *Handler) UpdateAssessment(c echo.Context) error {
	orgID := orgID(c)
	assessmentID := c.Param("id")
	if _, err := uuid.Parse(assessmentID); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid assessment ID", "CK_INVALID_ID")
	}
	var in UpdateAssessmentInput
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "CK_BAD_REQUEST")
	}
	if err := h.validate.Struct(in); err != nil {
		return errResp(c, http.StatusBadRequest, "Ungültige Eingabe", "VALIDATION_ERROR")
	}
	if err := h.service.MarkAssessmentReviewed(c.Request().Context(), orgID, assessmentID); err != nil {
		if isNotFound(err) {
			return errResp(c, http.StatusNotFound, "assessment not found or not in submitted state", "CK_NOT_FOUND")
		}
		return errResp(c, http.StatusInternalServerError, "failed to update assessment", "CK_INTERNAL")
	}
	return c.JSON(http.StatusOK, map[string]bool{"ok": true})
}

// GetAssessmentAnswers handles GET /vaktcomply/assessments/:id/answers.
func (h *Handler) GetAssessmentAnswers(c echo.Context) error {
	orgID := orgID(c)
	assessmentID := c.Param("id")
	if _, err := uuid.Parse(assessmentID); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid assessment ID", "CK_INVALID_ID")
	}
	answers, err := h.service.GetAnswersForAssessment(c.Request().Context(), orgID, assessmentID)
	if err != nil {
		return errResp(c, http.StatusInternalServerError, "failed to load answers", "CK_INTERNAL")
	}
	return c.JSON(http.StatusOK, answers)
}

// ExportAuditPackage handles GET /frameworks/:id/audit-package.zip.
// Returns a ZIP archive with INDEX.pdf, summary.json, and per-control evidence files.
func (h *Handler) ExportAuditPackage(c echo.Context) error {
	data, filename, err := h.service.ExportAuditPackage(c.Request().Context(), orgID(c), c.Param("id"))
	if err != nil {
		log.Error().Err(err).Str("framework_id", c.Param("id")).Msg("export audit package")
		return errResp(c, http.StatusInternalServerError, "failed to generate audit package", "CK_AUDIT_PACKAGE_FAILED")
	}
	c.Response().Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
	return c.Blob(http.StatusOK, "application/zip", data)
}

// GetAssessmentReportPDF handles GET /vaktcomply/assessments/:id/report-pdf.
func (h *Handler) GetAssessmentReportPDF(c echo.Context) error {
	orgID := orgID(c)
	assessmentID := c.Param("id")
	if _, err := uuid.Parse(assessmentID); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid assessment ID", "CK_INVALID_ID")
	}
	pdf, err := h.service.GenerateAssessmentReportPDF(c.Request().Context(), orgID, assessmentID)
	if err != nil {
		if isNotFound(err) {
			return errResp(c, http.StatusNotFound, "assessment not found", "CK_NOT_FOUND")
		}
		return errResp(c, http.StatusInternalServerError, "failed to generate PDF", "CK_INTERNAL")
	}
	c.Response().Header().Set("Content-Type", "application/pdf")
	c.Response().Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", "assessment-"+assessmentID+".pdf"))
	return c.Blob(http.StatusOK, "application/pdf", pdf)
}
