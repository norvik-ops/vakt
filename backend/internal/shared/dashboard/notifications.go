// Package dashboard aggregates cross-module metrics into a single security
// score and manages in-app notifications stored in the user_notifications
// table. It queries SecPulse findings, SecPrivacy breaches, and SecVitals
// frameworks directly via raw SQL so it remains decoupled from each module's
// service layer.
package dashboard

import (
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog/log"
)

// UserNotification is the JSON shape returned by the notifications endpoints.
// The Module field identifies which SecHealth module originated the event so
// the frontend can route the user to the right detail view.
type UserNotification struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	Body      string    `json:"body"`
	Type      string    `json:"type"`
	Module    string    `json:"module"`
	Read      bool      `json:"read"`
	CreatedAt time.Time `json:"created_at"`
}

// ListNotifications returns the 50 most recent notifications for the
// authenticated organisation, ordered newest first. An empty slice (never
// null) is returned when no rows exist so the frontend can iterate safely.
func (h *Handler) ListNotifications(c echo.Context) error {
	orgID, ok := c.Get("org_id").(string)
	if !ok || orgID == "" {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
	}
	rows, err := h.db.Query(c.Request().Context(),
		`SELECT id, title, body, type, module, read, created_at
         FROM user_notifications
         WHERE org_id=$1::uuid
         ORDER BY created_at DESC
         LIMIT 50`,
		orgID)
	if err != nil {
		log.Error().Err(err).Msg("list notifications")
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "db error"})
	}
	defer rows.Close()

	var result []UserNotification
	for rows.Next() {
		var n UserNotification
		if err := rows.Scan(&n.ID, &n.Title, &n.Body, &n.Type, &n.Module, &n.Read, &n.CreatedAt); err != nil {
			continue
		}
		result = append(result, n)
	}
	if result == nil {
		result = []UserNotification{}
	}
	return c.JSON(http.StatusOK, result)
}

// MarkNotificationRead marks a single notification as read. The update is
// scoped to the caller's org_id so users cannot mutate other organisations'
// records. Responds 204 No Content on success.
func (h *Handler) MarkNotificationRead(c echo.Context) error {
	orgID, ok := c.Get("org_id").(string)
	if !ok || orgID == "" {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
	}
	id := c.Param("id")
	_, err := h.db.Exec(c.Request().Context(),
		`UPDATE user_notifications SET read=true WHERE id=$1::uuid AND org_id=$2::uuid`,
		id, orgID)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "db error"})
	}
	return c.NoContent(http.StatusNoContent)
}

// MarkAllRead marks every unread notification for the organisation as read in
// a single UPDATE. Responds 204 No Content on success.
func (h *Handler) MarkAllRead(c echo.Context) error {
	orgID, ok := c.Get("org_id").(string)
	if !ok || orgID == "" {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
	}
	_, err := h.db.Exec(c.Request().Context(),
		`UPDATE user_notifications SET read=true WHERE org_id=$1::uuid AND read=false`,
		orgID)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "db error"})
	}
	return c.NoContent(http.StatusNoContent)
}
