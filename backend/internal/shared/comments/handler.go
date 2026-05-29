// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package comments

import (
	"fmt"
	"net/http"
	"regexp"
	"strings"

	"github.com/go-playground/validator/v10"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog/log"

	"github.com/matharnica/vakt/internal/shared/notify"
)

// Handler handles HTTP requests for the shared comments endpoint.
type Handler struct {
	repo     *Repository
	db       *pgxpool.Pool
	validate *validator.Validate
}

// NewHandler creates a new comments handler.
func NewHandler(repo *Repository, db *pgxpool.Pool) *Handler {
	return &Handler{
		repo:     repo,
		db:       db,
		validate: validator.New(),
	}
}

// createCommentInput is the request body for POST /api/v1/comments.
type createCommentInput struct {
	EntityType string `json:"entity_type" validate:"required,oneof=finding control"`
	EntityID   string `json:"entity_id"   validate:"required,uuid"`
	Content    string `json:"content"     validate:"required,min=1,max=4000"`
}

// isAdmin returns true when the authenticated user holds the Admin role.
// Roles are injected by the Paseto auth middleware as []string.
func isAdmin(c echo.Context) bool {
	roles, _ := c.Get("roles").([]string)
	for _, r := range roles {
		if r == "Admin" {
			return true
		}
	}
	return false
}

// ListComments handles GET /api/v1/comments?entity_type=<type>&entity_id=<uuid>.
func (h *Handler) ListComments(c echo.Context) error {
	orgID, _ := c.Get("org_id").(string)
	userID, _ := c.Get("user_id").(string)
	if orgID == "" || userID == "" {
		return c.JSON(http.StatusUnauthorized, map[string]string{
			"error": "unauthorized",
			"code":  "AUTH_MISSING_TOKEN",
		})
	}

	entityType := c.QueryParam("entity_type")
	entityID := c.QueryParam("entity_id")
	if entityType == "" || entityID == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "entity_type and entity_id are required",
			"code":  "COMMENTS_BAD_REQUEST",
		})
	}
	if entityType != "finding" && entityType != "control" {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "entity_type must be 'finding' or 'control'",
			"code":  "COMMENTS_INVALID_ENTITY_TYPE",
		})
	}

	cmts, err := h.repo.ListComments(c.Request().Context(), orgID, entityType, entityID, userID, isAdmin(c))
	if err != nil {
		log.Error().Err(err).Str("org_id", orgID).Msg("list comments failed")
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "failed to list comments",
			"code":  "COMMENTS_LIST_ERROR",
		})
	}
	if cmts == nil {
		cmts = []Comment{}
	}
	return c.JSON(http.StatusOK, cmts)
}

// CreateComment handles POST /api/v1/comments.
func (h *Handler) CreateComment(c echo.Context) error {
	orgID, _ := c.Get("org_id").(string)
	userID, _ := c.Get("user_id").(string)
	if orgID == "" || userID == "" {
		return c.JSON(http.StatusUnauthorized, map[string]string{
			"error": "unauthorized",
			"code":  "AUTH_MISSING_TOKEN",
		})
	}

	var in createCommentInput
	if err := c.Bind(&in); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "invalid request body",
			"code":  "COMMENTS_BAD_REQUEST",
		})
	}
	if err := h.validate.Struct(in); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, map[string]string{
			"error": "Ungültige Eingabe",
			"code":  "VALIDATION_ERROR",
		})
	}

	cmt, err := h.repo.CreateComment(c.Request().Context(), orgID, in.EntityType, in.EntityID, userID, in.Content)
	if err != nil {
		log.Error().Err(err).Str("org_id", orgID).Msg("create comment failed")
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "failed to create comment",
			"code":  "COMMENTS_CREATE_ERROR",
		})
	}

	// Send in-app notification — non-fatal, comment was already persisted.
	module := "vaktscan"
	if in.EntityType == "control" {
		module = "vaktcomply"
	}
	userName := cmt.AuthorName
	if userName == "" {
		userName = userID
	}
	notify.Send(
		c.Request().Context(),
		h.db,
		orgID,
		"Neuer Kommentar",
		fmt.Sprintf("%s hat einen Kommentar hinterlassen", userName),
		"comment_added",
		module,
	)

	// Parse @mentions and send targeted notifications.
	mentionRe := regexp.MustCompile(`@(\S+)`)
	matches := mentionRe.FindAllStringSubmatch(in.Content, -1)
	seen := make(map[string]struct{})
	trailingPunct := strings.NewReplacer(".", "", ",", "", "!", "", "?", "")
	for _, m := range matches {
		fragment := trailingPunct.Replace(m[1])
		if fragment == "" {
			continue
		}
		rows, queryErr := h.db.Query(
			c.Request().Context(),
			`SELECT u.id::text, u.display_name
			 FROM org_members om
			 JOIN users u ON u.id = om.user_id
			 WHERE om.org_id = $1::uuid
			   AND u.is_active = true
			   AND u.display_name ILIKE $2 || '%'
			 LIMIT 10`,
			orgID, fragment,
		)
		if queryErr != nil {
			log.Error().Err(queryErr).Str("org_id", orgID).Msg("mention lookup failed")
			continue
		}
		func() {
			defer rows.Close()
			for rows.Next() {
				var uid, displayName string
				if scanErr := rows.Scan(&uid, &displayName); scanErr != nil {
					continue
				}
				if _, alreadySent := seen[uid]; alreadySent {
					continue
				}
				seen[uid] = struct{}{}
				body := cmt.AuthorName + " hat Sie in einem Kommentar erwähnt"
				notify.Send(
					c.Request().Context(),
					h.db,
					orgID,
					"Sie wurden erwähnt",
					body,
					"mention",
					module,
				)
			}
		}()
	}

	return c.JSON(http.StatusCreated, cmt)
}

// DeleteComment handles DELETE /api/v1/comments/:id.
func (h *Handler) DeleteComment(c echo.Context) error {
	orgID, _ := c.Get("org_id").(string)
	userID, _ := c.Get("user_id").(string)
	if orgID == "" || userID == "" {
		return c.JSON(http.StatusUnauthorized, map[string]string{
			"error": "unauthorized",
			"code":  "AUTH_MISSING_TOKEN",
		})
	}

	commentID := c.Param("id")
	if err := h.repo.DeleteComment(c.Request().Context(), orgID, commentID, userID, isAdmin(c)); err != nil {
		errMsg := err.Error()
		if errMsg == "permission denied" {
			return c.JSON(http.StatusForbidden, map[string]string{
				"error": "not allowed to delete this comment",
				"code":  "COMMENTS_FORBIDDEN",
			})
		}
		return c.JSON(http.StatusNotFound, map[string]string{
			"error": "comment not found",
			"code":  "COMMENTS_NOT_FOUND",
		})
	}
	return c.NoContent(http.StatusNoContent)
}
