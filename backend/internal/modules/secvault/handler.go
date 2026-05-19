// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package secvault

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog/log"

	"github.com/matharnica/vakt/internal/shared/auditlog"
)

var validate = validator.New()

// Handler handles HTTP requests for SecretOps.
type Handler struct {
	service *Service
	db      *pgxpool.Pool
}

// NewHandler creates a new SecretOps handler.
func NewHandler(service *Service) *Handler {
	return &Handler{service: service, db: service.db}
}

// Init creates a new Handler with a fully wired Service. Convenience constructor
// that mirrors the pattern used by other modules.
func Init(service *Service) *Handler {
	return NewHandler(service)
}

// --- Projects ---

type createProjectRequest struct {
	Name        string `json:"name"        validate:"required,min=1,max=120"`
	Description string `json:"description"`
}

func (h *Handler) CreateProject(c echo.Context) error {
	orgID := mustString(c, "org_id")
	userID := mustString(c, "user_id")

	var req createProjectRequest
	if err := c.Bind(&req); err != nil {
		return badRequest(c, "invalid request body")
	}
	if err := validate.Struct(req); err != nil {
		return validationError(c, err)
	}

	project, err := h.service.CreateProject(c.Request().Context(), orgID, userID, req.Name, req.Description)
	if err != nil {
		log.Error().Err(err).Msg("CreateProject failed")
		return serverError(c, err)
	}
	auditlog.Log(c.Request().Context(), h.db, auditlog.Entry{
		OrgID: orgID, UserID: userID, Action: "create",
		ResourceType: "vakt-vault/project", ResourceID: project.ID, ResourceName: project.Name,
		IPAddress: c.RealIP(),
	})
	return c.JSON(http.StatusCreated, project)
}

func (h *Handler) ListProjects(c echo.Context) error {
	orgID := mustString(c, "org_id")

	projects, err := h.service.ListProjects(c.Request().Context(), orgID)
	if err != nil {
		log.Error().Err(err).Msg("ListProjects failed")
		return serverError(c, err)
	}
	if projects == nil {
		projects = []Project{}
	}
	return c.JSON(http.StatusOK, projects)
}

func (h *Handler) DeleteProject(c echo.Context) error {
	orgID := mustString(c, "org_id")
	projectID := c.Param("id")
	if projectID == "" {
		return badRequest(c, "project id is required")
	}

	if err := h.service.DeleteProject(c.Request().Context(), orgID, projectID); err != nil {
		log.Error().Err(err).Msg("DeleteProject failed")
		return serverError(c, err)
	}
	auditlog.Log(c.Request().Context(), h.db, auditlog.Entry{
		OrgID: orgID, UserID: mustString(c, "user_id"), Action: "delete",
		ResourceType: "vakt-vault/project", ResourceID: projectID,
		IPAddress: c.RealIP(),
	})
	return c.NoContent(http.StatusNoContent)
}

// --- Environments ---

type createEnvironmentRequest struct {
	Name string `json:"name" validate:"required,min=1,max=80"`
}

func (h *Handler) CreateEnvironment(c echo.Context) error {
	orgID := mustString(c, "org_id")
	projectID := c.Param("project_id")
	if projectID == "" {
		return badRequest(c, "project_id is required")
	}

	var req createEnvironmentRequest
	if err := c.Bind(&req); err != nil {
		return badRequest(c, "invalid request body")
	}
	if err := validate.Struct(req); err != nil {
		return validationError(c, err)
	}

	env, err := h.service.CreateEnvironment(c.Request().Context(), orgID, projectID, req.Name)
	if err != nil {
		log.Error().Err(err).Msg("CreateEnvironment failed")
		return serverError(c, err)
	}
	return c.JSON(http.StatusCreated, env)
}

func (h *Handler) ListEnvironments(c echo.Context) error {
	orgID, _ := c.Get("org_id").(string)
	if orgID == "" {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
	}
	projectID := c.Param("project_id")
	if projectID == "" {
		return badRequest(c, "project_id is required")
	}

	envs, err := h.service.ListEnvironments(c.Request().Context(), orgID, projectID)
	if err != nil {
		log.Error().Err(err).Msg("ListEnvironments failed")
		return serverError(c, err)
	}
	if envs == nil {
		envs = []Environment{}
	}
	return c.JSON(http.StatusOK, envs)
}

// --- Secrets ---

type setSecretRequest struct {
	Value string `json:"value" validate:"required,max=65536"`
}

func (h *Handler) SetSecret(c echo.Context) error {
	orgID := mustString(c, "org_id")
	userID := mustString(c, "user_id")
	envID := c.Param("env_id")
	key := c.Param("key")

	if envID == "" || key == "" {
		return badRequest(c, "env_id and key are required")
	}

	var req setSecretRequest
	if err := c.Bind(&req); err != nil {
		return badRequest(c, "invalid request body")
	}
	if err := validate.Struct(req); err != nil {
		return validationError(c, err)
	}

	secret, err := h.service.SetSecret(c.Request().Context(), orgID, envID, userID, key, req.Value)
	if err != nil {
		log.Error().Err(err).Msg("SetSecret failed")
		return serverError(c, err)
	}
	auditlog.Log(c.Request().Context(), h.db, auditlog.Entry{
		OrgID: orgID, UserID: userID, Action: "update",
		ResourceType: "vakt-vault/secret", ResourceID: key,
		IPAddress: c.RealIP(),
	})
	return c.JSON(http.StatusOK, secret)
}

func (h *Handler) GetSecret(c echo.Context) error {
	orgID := mustString(c, "org_id")
	envID := c.Param("env_id")
	key := c.Param("key")

	if envID == "" || key == "" {
		return badRequest(c, "env_id and key are required")
	}

	ip := c.RealIP()
	secret, err := h.service.GetSecret(c.Request().Context(), orgID, envID, key, "api", ip)
	if err != nil {
		log.Error().Err(err).Msg("GetSecret failed")
		return notFound(c, "secret not found")
	}

	auditOrgID := orgID
	auditKey := key
	auditIP := c.RealIP()
	auditUserID, _ := c.Get("user_id").(string)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Error().Interface("panic", r).Msg("goroutine panic recovered")
			}
		}()
		if auditOrgID == "" {
			return
		}
		var uid *string
		if auditUserID != "" {
			uid = &auditUserID
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		h.db.Exec(ctx, `
			INSERT INTO audit_log (org_id, user_id, action, resource_type, resource_id, ip_address)
			VALUES ($1::uuid, $2::uuid, 'secret.reveal', 'secvault/secret', $3, $4)`,
			auditOrgID, uid, auditKey, auditIP)
	}()

	return c.JSON(http.StatusOK, secret)
}

func (h *Handler) ListSecretKeys(c echo.Context) error {
	orgID := mustString(c, "org_id")
	envID := c.Param("env_id")

	if envID == "" {
		return badRequest(c, "env_id is required")
	}

	secrets, err := h.service.ListSecretKeys(c.Request().Context(), orgID, envID)
	if err != nil {
		log.Error().Err(err).Msg("ListSecretKeys failed")
		return serverError(c, err)
	}
	if secrets == nil {
		secrets = []Secret{}
	}
	return c.JSON(http.StatusOK, secrets)
}

func (h *Handler) DeleteSecret(c echo.Context) error {
	orgID := mustString(c, "org_id")
	envID := c.Param("env_id")
	key := c.Param("key")

	if envID == "" || key == "" {
		return badRequest(c, "env_id and key are required")
	}

	if err := h.service.DeleteSecret(c.Request().Context(), orgID, envID, key); err != nil {
		log.Error().Err(err).Msg("DeleteSecret failed")
		return serverError(c, err)
	}
	auditlog.Log(c.Request().Context(), h.db, auditlog.Entry{
		OrgID: orgID, UserID: mustString(c, "user_id"), Action: "delete",
		ResourceType: "vakt-vault/secret", ResourceID: key,
		IPAddress: c.RealIP(),
	})
	return c.NoContent(http.StatusNoContent)
}

// --- Access log ---

func (h *Handler) GetAccessLog(c echo.Context) error {
	orgID := mustString(c, "org_id")
	envID := c.Param("env_id")
	key := c.Param("key")

	limit := queryInt(c, "limit", 25)
	offset := queryInt(c, "offset", 0)

	entries, err := h.service.GetAccessLog(c.Request().Context(), orgID, envID, key, limit, offset)
	if err != nil {
		log.Error().Err(err).Msg("GetAccessLog failed")
		return notFound(c, "secret not found")
	}
	if entries == nil {
		entries = []AccessLogEntry{}
	}
	return c.JSON(http.StatusOK, entries)
}

// GetProjectAccessLog handles GET /secvault/projects/:project_id/access-log.
// Accepts optional ?page and ?limit query parameters (both default to 25 when absent or invalid).
// Responds with an AccessLogPage JSON object containing entries, total, page, and limit.
func (h *Handler) GetProjectAccessLog(c echo.Context) error {
	orgID := mustString(c, "org_id")
	projectID := c.Param("project_id")
	if projectID == "" {
		return badRequest(c, "project_id is required")
	}

	page := queryInt(c, "page", 1)
	limit := queryInt(c, "limit", 25)

	result, err := h.service.GetProjectAccessLog(c.Request().Context(), orgID, projectID, page, limit)
	if err != nil {
		log.Error().Err(err).Msg("GetProjectAccessLog failed")
		return serverError(c, err)
	}
	return c.JSON(http.StatusOK, result)
}

// --- Health ---

func (h *Handler) GetProjectHealth(c echo.Context) error {
	orgID := mustString(c, "org_id")
	projectID := c.Param("project_id")

	if projectID == "" {
		return badRequest(c, "project_id is required")
	}

	scores, err := h.service.GetProjectHealth(c.Request().Context(), orgID, projectID)
	if err != nil {
		log.Error().Err(err).Msg("GetProjectHealth failed")
		return serverError(c, err)
	}
	if scores == nil {
		scores = []SecretHealth{}
	}
	return c.JSON(http.StatusOK, scores)
}

// --- Share links ---

type createShareLinkRequest struct {
	ExpiresInHours int `json:"expires_in_hours" validate:"required,min=1,max=168"`
}

type shareLinkResponse struct {
	ShareURL  string    `json:"share_url"`
	ExpiresAt time.Time `json:"expires_at"`
}

func (h *Handler) CreateShareLink(c echo.Context) error {
	orgID := mustString(c, "org_id")
	userID := mustString(c, "user_id")
	envID := c.Param("env_id")
	key := c.Param("key")

	if envID == "" || key == "" {
		return badRequest(c, "env_id and key are required")
	}

	var req createShareLinkRequest
	if err := c.Bind(&req); err != nil {
		return badRequest(c, "invalid request body")
	}
	if err := validate.Struct(req); err != nil {
		return validationError(c, err)
	}

	sl, err := h.service.CreateShareLink(c.Request().Context(), orgID, envID, userID, key, req.ExpiresInHours)
	if err != nil {
		log.Error().Err(err).Msg("CreateShareLink failed")
		return serverError(c, err)
	}

	return c.JSON(http.StatusCreated, shareLinkResponse{
		ShareURL:  sl.ShareURL,
		ExpiresAt: sl.ExpiresAt,
	})
}

type useShareLinkResponse struct {
	Value string `json:"value"`
}

func (h *Handler) UseShareLink(c echo.Context) error {
	token := c.Param("token")
	if token == "" {
		return badRequest(c, "token is required")
	}

	value, err := h.service.UseShareLink(c.Request().Context(), token)
	if err != nil {
		switch err.Error() {
		case "share link already used":
			return c.JSON(http.StatusGone, errorResponse("share link already used", "SHARE_LINK_USED"))
		case "share link expired":
			return c.JSON(http.StatusGone, errorResponse("share link expired", "SHARE_LINK_EXPIRED"))
		default:
			return notFound(c, "share link not found")
		}
	}

	return c.JSON(http.StatusOK, useShareLinkResponse{Value: value})
}

// --- API tokens ---

type createTokenRequest struct {
	Name          string     `json:"name"           validate:"required,min=1,max=120"`
	ExpiresAt     *time.Time `json:"expires_at"`
}

func (h *Handler) CreateToken(c echo.Context) error {
	orgID := mustString(c, "org_id")
	userID := mustString(c, "user_id")

	var req createTokenRequest
	if err := c.Bind(&req); err != nil {
		return badRequest(c, "invalid request body")
	}
	if err := validate.Struct(req); err != nil {
		return validationError(c, err)
	}

	token, err := h.service.CreateToken(c.Request().Context(), orgID, userID, req.Name, req.ExpiresAt)
	if err != nil {
		log.Error().Err(err).Msg("CreateToken failed")
		return serverError(c, err)
	}
	return c.JSON(http.StatusCreated, token)
}

func (h *Handler) ListTokens(c echo.Context) error {
	orgID := mustString(c, "org_id")
	userID := mustString(c, "user_id")

	tokens, err := h.service.ListTokens(c.Request().Context(), orgID, userID)
	if err != nil {
		log.Error().Err(err).Msg("ListTokens failed")
		return serverError(c, err)
	}
	if tokens == nil {
		tokens = []APIToken{}
	}
	return c.JSON(http.StatusOK, tokens)
}

func (h *Handler) RevokeToken(c echo.Context) error {
	orgID := mustString(c, "org_id")
	userID := mustString(c, "user_id")
	tokenID := c.Param("id")

	if tokenID == "" {
		return badRequest(c, "token id is required")
	}

	if err := h.service.RevokeToken(c.Request().Context(), orgID, userID, tokenID); err != nil {
		log.Error().Err(err).Msg("RevokeToken failed")
		return serverError(c, err)
	}
	return c.NoContent(http.StatusNoContent)
}

// --- Import / Export ---

// ImportSecrets handles POST /secvault/projects/:project_id/import
func (h *Handler) ImportSecrets(c echo.Context) error {
	orgID := mustString(c, "org_id")
	projectID := c.Param("project_id")

	var input ImportInput
	if err := c.Bind(&input); err != nil {
		return badRequest(c, "invalid request body")
	}
	if err := validate.Struct(input); err != nil {
		return validationError(c, err)
	}

	result, err := h.service.ImportSecrets(c.Request().Context(), orgID, projectID, input.Environment, input.FileContent, input.Source)
	if err != nil {
		log.Error().Err(err).Msg("ImportSecrets failed")
		if strings.HasPrefix(err.Error(), "too many secrets") {
			return c.JSON(http.StatusUnprocessableEntity, errorResponse("too many secrets: max 500 per import", "IMPORT_LIMIT_EXCEEDED"))
		}
		return c.JSON(http.StatusBadRequest, errorResponse("import failed", "SO_IMPORT_ERROR"))
	}
	return c.JSON(http.StatusOK, result)
}

// ExportSecrets handles GET /secvault/projects/:project_id/envs/:env_id/export
// Bulk secret export is a privileged operation restricted to the Admin role.
func (h *Handler) ExportSecrets(c echo.Context) error {
	// Enforce Admin-only access — exporting all secrets is highly privileged.
	roles, _ := c.Get("roles").([]string)
	isAdmin := false
	for _, r := range roles {
		if r == "Admin" {
			isAdmin = true
			break
		}
	}
	if !isAdmin {
		return c.JSON(http.StatusForbidden, errorResponse("forbidden: admin role required", "AUTH_INSUFFICIENT_ROLE"))
	}

	orgID := mustString(c, "org_id")
	content, err := h.service.ExportSecrets(c.Request().Context(), orgID, c.Param("project_id"), c.Param("env_id"))
	if err != nil {
		log.Error().Err(err).Msg("ExportSecrets failed")
		return serverError(c, err)
	}
	c.Response().Header().Set("Content-Type", "text/plain")
	return c.String(http.StatusOK, content)
}

// --- Secret Rotation ---

// RotateSecret handles POST /secvault/projects/:project_id/envs/:env_id/secrets/:key/rotate
func (h *Handler) RotateSecret(c echo.Context) error {
	orgID := mustString(c, "org_id")
	var input RotateInput
	if err := c.Bind(&input); err != nil {
		return badRequest(c, "invalid request body")
	}
	if err := validate.Struct(input); err != nil {
		return validationError(c, err)
	}
	if err := h.service.RotateSecret(c.Request().Context(), orgID, c.Param("env_id"), c.Param("key"), input); err != nil {
		log.Error().Err(err).Msg("RotateSecret failed")
		return c.JSON(http.StatusInternalServerError, errorResponse("rotation failed", "SO_ROTATE_ERROR"))
	}
	return c.JSON(http.StatusOK, map[string]string{"status": "rotated"})
}

// --- Git Scanner ---

// TriggerGitScan handles POST /secvault/git-scans
func (h *Handler) TriggerGitScan(c echo.Context) error {
	orgID := mustString(c, "org_id")
	var input TriggerGitScanInput
	if err := c.Bind(&input); err != nil {
		return badRequest(c, "invalid request body")
	}
	if err := validate.Struct(input); err != nil {
		return validationError(c, err)
	}
	// SSRF guard: enforce HTTPS-only and block private/loopback addresses.
	if err := ValidateRepoURL(input.RepoURL); err != nil {
		return badRequest(c, err.Error())
	}
	scan, err := h.service.TriggerGitScan(c.Request().Context(), orgID, input)
	if err != nil {
		log.Error().Err(err).Msg("TriggerGitScan failed")
		return serverError(c, err)
	}
	return c.JSON(http.StatusAccepted, scan)
}

// ListGitScans handles GET /secvault/git-scans
func (h *Handler) ListGitScans(c echo.Context) error {
	orgID := mustString(c, "org_id")
	scans, err := h.service.ListGitScans(c.Request().Context(), orgID)
	if err != nil {
		log.Error().Err(err).Msg("ListGitScans failed")
		return serverError(c, err)
	}
	if scans == nil {
		scans = []GitScan{}
	}
	return c.JSON(http.StatusOK, scans)
}

// GetGitScan handles GET /secvault/git-scans/:id
func (h *Handler) GetGitScan(c echo.Context) error {
	orgID := mustString(c, "org_id")
	scan, err := h.service.GetGitScan(c.Request().Context(), orgID, c.Param("id"))
	if err != nil {
		return notFound(c, "scan not found")
	}
	return c.JSON(http.StatusOK, scan)
}

// GetGitScanResults handles GET /secvault/git-scans/:id/results
func (h *Handler) GetGitScanResults(c echo.Context) error {
	orgID := mustString(c, "org_id")
	results, err := h.service.GetGitScanResults(c.Request().Context(), orgID, c.Param("id"))
	if err != nil {
		log.Error().Err(err).Msg("GetGitScanResults failed")
		return serverError(c, err)
	}
	if results == nil {
		results = []ScanResult{}
	}
	return c.JSON(http.StatusOK, results)
}

// DismissScanResult handles POST /secvault/git-scans/results/:result_id/dismiss
func (h *Handler) DismissScanResult(c echo.Context) error {
	orgID := mustString(c, "org_id")
	var input DismissScanResultInput
	if err := c.Bind(&input); err != nil {
		return badRequest(c, "invalid request body")
	}
	if err := validate.Struct(input); err != nil {
		return validationError(c, err)
	}
	if err := h.service.DismissScanResult(c.Request().Context(), orgID, c.Param("result_id"), input); err != nil {
		return notFound(c, "result not found")
	}
	return c.JSON(http.StatusOK, map[string]string{"status": "dismissed"})
}

// --- Response helpers ---

type apiError struct {
	Error   string `json:"error"`
	Code    string `json:"code"`
}

func errorResponse(msg, code string) apiError {
	return apiError{Error: msg, Code: code}
}

func badRequest(c echo.Context, msg string) error {
	return c.JSON(http.StatusBadRequest, errorResponse(msg, "BAD_REQUEST"))
}

func notFound(c echo.Context, msg string) error {
	return c.JSON(http.StatusNotFound, errorResponse(msg, "NOT_FOUND"))
}

func serverError(c echo.Context, err error) error {
	// Log the real error internally but never expose it in the API response.
	c.Logger().Error(err)
	return c.JSON(http.StatusInternalServerError, errorResponse("internal server error", "INTERNAL_ERROR"))
}

func validationError(c echo.Context, err error) error {
	_ = err // real error is never exposed to clients
	return c.JSON(http.StatusUnprocessableEntity, errorResponse("Ungültige Eingabe", "VALIDATION_ERROR"))
}

// mustString extracts a string value from the echo context, returning "" if missing.
func mustString(c echo.Context, key string) string {
	v, _ := c.Get(key).(string)
	return v
}

// queryInt parses an integer query parameter, returning def on failure.
func queryInt(c echo.Context, name string, def int) int {
	s := c.QueryParam(name)
	if s == "" {
		return def
	}
	v, err := strconv.Atoi(s)
	if err != nil {
		return def
	}
	return v
}
