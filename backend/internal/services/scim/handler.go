package scim

import (
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog/log"
)

// Handler serves the SCIM 2.0 endpoints under /api/v1/scim/v2/.
type Handler struct {
	svc *Service
}

// NewHandler constructs a SCIM Handler.
func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// SCIM schema URNs.
const (
	schemaUser         = "urn:ietf:params:scim:schemas:core:2.0:User"
	schemaGroup        = "urn:ietf:params:scim:schemas:core:2.0:Group"
	schemaListResponse = "urn:ietf:params:scim:api:messages:2.0:ListResponse"
	schemaPatchOp      = "urn:ietf:params:scim:api:messages:2.0:PatchOp"
	schemaServiceProvider = "urn:ietf:params:scim:schemas:core:2.0:ServiceProviderConfig"
)

// ─── Wire types ───────────────────────────────────────────────────────────────

type scimMeta struct {
	ResourceType string    `json:"resourceType"`
	Created      time.Time `json:"created"`
	LastModified time.Time `json:"lastModified"`
	Location     string    `json:"location,omitempty"`
	Version      string    `json:"version,omitempty"`
}

type scimName struct {
	Formatted  string `json:"formatted,omitempty"`
	GivenName  string `json:"givenName,omitempty"`
	FamilyName string `json:"familyName,omitempty"`
}

type scimEmail struct {
	Value   string `json:"value"`
	Type    string `json:"type,omitempty"`
	Primary bool   `json:"primary,omitempty"`
}

type scimUserResponse struct {
	Schemas    []string    `json:"schemas"`
	ID         string      `json:"id"`
	ExternalID string      `json:"externalId,omitempty"`
	UserName   string      `json:"userName"`
	Name       scimName    `json:"name,omitempty"`
	DisplayName string     `json:"displayName,omitempty"`
	Emails     []scimEmail `json:"emails,omitempty"`
	Active     bool        `json:"active"`
	Meta       scimMeta    `json:"meta"`
}

type scimGroupResponse struct {
	Schemas     []string          `json:"schemas"`
	ID          string            `json:"id"`
	ExternalID  string            `json:"externalId,omitempty"`
	DisplayName string            `json:"displayName"`
	Members     []SCIMGroupMember `json:"members,omitempty"`
	Meta        scimMeta          `json:"meta"`
}

type scimListResponse struct {
	Schemas      []string `json:"schemas"`
	TotalResults int      `json:"totalResults"`
	StartIndex   int      `json:"startIndex"`
	ItemsPerPage int      `json:"itemsPerPage"`
	Resources    any      `json:"Resources"`
}

// createUserRequest is the POST /Users request body.
type createUserRequest struct {
	Schemas     []string    `json:"schemas"`
	UserName    string      `json:"userName"`
	ExternalID  string      `json:"externalId"`
	DisplayName string      `json:"displayName"`
	Name        scimName    `json:"name"`
	Emails      []scimEmail `json:"emails"`
	Active      *bool       `json:"active"`
}

// createGroupRequest is the POST /Groups request body.
type createGroupRequest struct {
	Schemas     []string          `json:"schemas"`
	DisplayName string            `json:"displayName"`
	ExternalID  string            `json:"externalId"`
	Members     []SCIMGroupMember `json:"members"`
}

// patchRequest is the PATCH /Users/:id and /Groups/:id request body.
type patchRequest struct {
	Schemas    []string  `json:"schemas"`
	Operations []PatchOp `json:"Operations"`
}

// PatchOp represents one SCIM PATCH operation.
type PatchOp struct {
	Op    string `json:"op"`
	Path  string `json:"path"`
	Value any    `json:"value"`
}

// ─── ServiceProviderConfig ────────────────────────────────────────────────────

// GetServiceProviderConfig handles GET /ServiceProviderConfig.
func (h *Handler) GetServiceProviderConfig(c echo.Context) error {
	type supported struct {
		Supported bool `json:"supported"`
	}
	type filter struct {
		Supported  bool `json:"supported"`
		MaxResults int  `json:"maxResults"`
	}
	return c.JSON(http.StatusOK, map[string]any{
		"schemas":              []string{schemaServiceProvider},
		"documentationUri":     "https://vakt.io/docs/scim",
		"patch":                supported{Supported: true},
		"bulk":                 supported{Supported: false},
		"filter":               filter{Supported: true, MaxResults: 200},
		"changePassword":       supported{Supported: false},
		"sort":                 supported{Supported: false},
		"etag":                 supported{Supported: false},
		"authenticationSchemes": []map[string]any{
			{
				"type":             "oauthbearertoken",
				"name":             "SCIM Token",
				"description":      "Vakt SCIM Bearer token (managed in Admin › SCIM Tokens)",
				"specUri":          "https://www.rfc-editor.org/rfc/rfc6750",
				"primary":          true,
			},
		},
	})
}

// ─── Users ────────────────────────────────────────────────────────────────────

// ListUsers handles GET /Users.
func (h *Handler) ListUsers(c echo.Context) error {
	orgID, _ := c.Get("scim_org_id").(string)
	filter := c.QueryParam("filter")

	users, err := h.svc.ListUsers(c.Request().Context(), orgID, filter)
	if err != nil {
		log.Error().Err(err).Str("org_id", orgID).Msg("scim: list users failed")
		return scimError(c, http.StatusInternalServerError, "internalError", "failed to list users")
	}

	resources := make([]scimUserResponse, 0, len(users))
	for _, u := range users {
		resources = append(resources, userToResponse(u))
	}

	return c.JSON(http.StatusOK, scimListResponse{
		Schemas:      []string{schemaListResponse},
		TotalResults: len(resources),
		StartIndex:   1,
		ItemsPerPage: len(resources),
		Resources:    resources,
	})
}

// GetUser handles GET /Users/:id.
func (h *Handler) GetUser(c echo.Context) error {
	orgID, _ := c.Get("scim_org_id").(string)
	userID := c.Param("id")

	u, err := h.svc.GetUser(c.Request().Context(), orgID, userID)
	if err != nil {
		if err == pgx.ErrNoRows {
			return scimError(c, http.StatusNotFound, "notFound", "User not found")
		}
		log.Error().Err(err).Str("user_id", userID).Msg("scim: get user failed")
		return scimError(c, http.StatusInternalServerError, "internalError", "failed to get user")
	}
	return c.JSON(http.StatusOK, userToResponse(*u))
}

// CreateUser handles POST /Users.
func (h *Handler) CreateUser(c echo.Context) error {
	orgID, _ := c.Get("scim_org_id").(string)

	var req createUserRequest
	if err := c.Bind(&req); err != nil {
		return scimError(c, http.StatusBadRequest, "invalidValue", "invalid request body")
	}

	email := req.UserName
	if email == "" {
		for _, e := range req.Emails {
			if e.Primary || email == "" {
				email = e.Value
			}
		}
	}
	if email == "" {
		return scimError(c, http.StatusBadRequest, "invalidValue", "userName or emails[].value is required")
	}

	active := true
	if req.Active != nil {
		active = *req.Active
	}

	u, err := h.svc.CreateUser(c.Request().Context(), orgID, SCIMUser{
		UserName:    email,
		Email:       email,
		DisplayName: req.DisplayName,
		FirstName:   req.Name.GivenName,
		LastName:    req.Name.FamilyName,
		Active:      active,
		ExternalID:  req.ExternalID,
	})
	if err != nil {
		log.Error().Err(err).Str("org_id", orgID).Str("email", email).Msg("scim: create user failed")
		return scimError(c, http.StatusInternalServerError, "internalError", "failed to provision user")
	}
	return c.JSON(http.StatusCreated, userToResponse(*u))
}

// ReplaceUser handles PUT /Users/:id.
func (h *Handler) ReplaceUser(c echo.Context) error {
	orgID, _ := c.Get("scim_org_id").(string)
	userID := c.Param("id")

	var req createUserRequest
	if err := c.Bind(&req); err != nil {
		return scimError(c, http.StatusBadRequest, "invalidValue", "invalid request body")
	}

	email := req.UserName
	if email == "" {
		for _, e := range req.Emails {
			if e.Primary || email == "" {
				email = e.Value
			}
		}
	}

	active := true
	if req.Active != nil {
		active = *req.Active
	}

	u, err := h.svc.ReplaceUser(c.Request().Context(), orgID, userID, SCIMUser{
		UserName:    email,
		Email:       email,
		DisplayName: req.DisplayName,
		FirstName:   req.Name.GivenName,
		LastName:    req.Name.FamilyName,
		Active:      active,
		ExternalID:  req.ExternalID,
	})
	if err != nil {
		if err == pgx.ErrNoRows {
			return scimError(c, http.StatusNotFound, "notFound", "User not found")
		}
		log.Error().Err(err).Str("user_id", userID).Msg("scim: replace user failed")
		return scimError(c, http.StatusInternalServerError, "internalError", "failed to replace user")
	}
	return c.JSON(http.StatusOK, userToResponse(*u))
}

// PatchUser handles PATCH /Users/:id.
func (h *Handler) PatchUser(c echo.Context) error {
	orgID, _ := c.Get("scim_org_id").(string)
	userID := c.Param("id")

	var req patchRequest
	if err := c.Bind(&req); err != nil {
		return scimError(c, http.StatusBadRequest, "invalidValue", "invalid request body")
	}

	u, err := h.svc.PatchUser(c.Request().Context(), orgID, userID, req.Operations)
	if err != nil {
		if err == pgx.ErrNoRows {
			return scimError(c, http.StatusNotFound, "notFound", "User not found")
		}
		log.Error().Err(err).Str("user_id", userID).Msg("scim: patch user failed")
		return scimError(c, http.StatusInternalServerError, "internalError", "failed to patch user")
	}
	return c.JSON(http.StatusOK, userToResponse(*u))
}

// DeleteUser handles DELETE /Users/:id.
func (h *Handler) DeleteUser(c echo.Context) error {
	orgID, _ := c.Get("scim_org_id").(string)
	userID := c.Param("id")

	if err := h.svc.DeactivateUser(c.Request().Context(), orgID, userID); err != nil {
		log.Error().Err(err).Str("user_id", userID).Msg("scim: delete user failed")
		return scimError(c, http.StatusInternalServerError, "internalError", "failed to delete user")
	}
	return c.NoContent(http.StatusNoContent)
}

// ─── Groups ───────────────────────────────────────────────────────────────────

// ListGroups handles GET /Groups.
func (h *Handler) ListGroups(c echo.Context) error {
	orgID, _ := c.Get("scim_org_id").(string)
	filter := c.QueryParam("filter")

	groups, err := h.svc.ListGroups(c.Request().Context(), orgID, filter)
	if err != nil {
		log.Error().Err(err).Str("org_id", orgID).Msg("scim: list groups failed")
		return scimError(c, http.StatusInternalServerError, "internalError", "failed to list groups")
	}

	resources := make([]scimGroupResponse, 0, len(groups))
	for _, g := range groups {
		resources = append(resources, groupToResponse(g))
	}

	return c.JSON(http.StatusOK, scimListResponse{
		Schemas:      []string{schemaListResponse},
		TotalResults: len(resources),
		StartIndex:   1,
		ItemsPerPage: len(resources),
		Resources:    resources,
	})
}

// GetGroup handles GET /Groups/:id.
func (h *Handler) GetGroup(c echo.Context) error {
	orgID, _ := c.Get("scim_org_id").(string)
	groupID := c.Param("id")

	g, err := h.svc.GetGroup(c.Request().Context(), orgID, groupID)
	if err != nil {
		if err == pgx.ErrNoRows {
			return scimError(c, http.StatusNotFound, "notFound", "Group not found")
		}
		log.Error().Err(err).Str("group_id", groupID).Msg("scim: get group failed")
		return scimError(c, http.StatusInternalServerError, "internalError", "failed to get group")
	}
	return c.JSON(http.StatusOK, groupToResponse(*g))
}

// CreateGroup handles POST /Groups.
func (h *Handler) CreateGroup(c echo.Context) error {
	orgID, _ := c.Get("scim_org_id").(string)

	var req createGroupRequest
	if err := c.Bind(&req); err != nil {
		return scimError(c, http.StatusBadRequest, "invalidValue", "invalid request body")
	}
	if req.DisplayName == "" {
		return scimError(c, http.StatusBadRequest, "invalidValue", "displayName is required")
	}

	g, err := h.svc.CreateGroup(c.Request().Context(), orgID, SCIMGroup{
		DisplayName: req.DisplayName,
		ExternalID:  req.ExternalID,
		Members:     req.Members,
	})
	if err != nil {
		log.Error().Err(err).Str("org_id", orgID).Msg("scim: create group failed")
		return scimError(c, http.StatusInternalServerError, "internalError", "failed to create group")
	}
	return c.JSON(http.StatusCreated, groupToResponse(*g))
}

// ReplaceGroup handles PUT /Groups/:id.
func (h *Handler) ReplaceGroup(c echo.Context) error {
	orgID, _ := c.Get("scim_org_id").(string)
	groupID := c.Param("id")

	var req createGroupRequest
	if err := c.Bind(&req); err != nil {
		return scimError(c, http.StatusBadRequest, "invalidValue", "invalid request body")
	}

	g, err := h.svc.ReplaceGroup(c.Request().Context(), orgID, groupID, SCIMGroup{
		DisplayName: req.DisplayName,
		ExternalID:  req.ExternalID,
		Members:     req.Members,
	})
	if err != nil {
		if err == pgx.ErrNoRows {
			return scimError(c, http.StatusNotFound, "notFound", "Group not found")
		}
		log.Error().Err(err).Str("group_id", groupID).Msg("scim: replace group failed")
		return scimError(c, http.StatusInternalServerError, "internalError", "failed to replace group")
	}
	return c.JSON(http.StatusOK, groupToResponse(*g))
}

// PatchGroup handles PATCH /Groups/:id.
func (h *Handler) PatchGroup(c echo.Context) error {
	orgID, _ := c.Get("scim_org_id").(string)
	groupID := c.Param("id")

	var req patchRequest
	if err := c.Bind(&req); err != nil {
		return scimError(c, http.StatusBadRequest, "invalidValue", "invalid request body")
	}

	g, err := h.svc.PatchGroup(c.Request().Context(), orgID, groupID, req.Operations)
	if err != nil {
		if err == pgx.ErrNoRows {
			return scimError(c, http.StatusNotFound, "notFound", "Group not found")
		}
		log.Error().Err(err).Str("group_id", groupID).Msg("scim: patch group failed")
		return scimError(c, http.StatusInternalServerError, "internalError", "failed to patch group")
	}
	return c.JSON(http.StatusOK, groupToResponse(*g))
}

// DeleteGroup handles DELETE /Groups/:id.
func (h *Handler) DeleteGroup(c echo.Context) error {
	orgID, _ := c.Get("scim_org_id").(string)
	groupID := c.Param("id")

	if err := h.svc.DeleteGroup(c.Request().Context(), orgID, groupID); err != nil {
		if err == pgx.ErrNoRows {
			return scimError(c, http.StatusNotFound, "notFound", "Group not found")
		}
		log.Error().Err(err).Str("group_id", groupID).Msg("scim: delete group failed")
		return scimError(c, http.StatusInternalServerError, "internalError", "failed to delete group")
	}
	return c.NoContent(http.StatusNoContent)
}

// ─── Conversion helpers ───────────────────────────────────────────────────────

func userToResponse(u SCIMUser) scimUserResponse {
	return scimUserResponse{
		Schemas:     []string{schemaUser},
		ID:          u.ID,
		ExternalID:  u.ExternalID,
		UserName:    u.UserName,
		DisplayName: u.DisplayName,
		Name: scimName{
			Formatted: u.DisplayName,
		},
		Emails: []scimEmail{{
			Value:   u.Email,
			Type:    "work",
			Primary: true,
		}},
		Active: u.Active,
		Meta: scimMeta{
			ResourceType: "User",
			Created:      u.CreatedAt,
			LastModified: u.UpdatedAt,
		},
	}
}

func groupToResponse(g SCIMGroup) scimGroupResponse {
	members := g.Members
	if members == nil {
		members = []SCIMGroupMember{}
	}
	return scimGroupResponse{
		Schemas:     []string{schemaGroup},
		ID:          g.ID,
		ExternalID:  g.ExternalID,
		DisplayName: g.DisplayName,
		Members:     members,
		Meta: scimMeta{
			ResourceType: "Group",
			Created:      g.CreatedAt,
			LastModified: g.UpdatedAt,
		},
	}
}
