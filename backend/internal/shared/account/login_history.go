package account

import (
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
)

// Sprint 22 / S22-11: GET /api/v1/account/login-history
//
// Liefert die letzten 50 Login-Versuche des authentifizierten Users.
// Nutzt die login_history-Tabelle (Sprint 20 Migration 126), populiert
// von auth.recordLogin in password/oidc/saml/register-Pfaden (S22-3).

// LoginHistoryItem ist das JSON-Read-Modell.
type LoginHistoryItem struct {
	TS        time.Time `json:"ts"`
	IP        string    `json:"ip,omitempty"`
	UserAgent string    `json:"user_agent,omitempty"`
	Source    string    `json:"source"`            // password | oidc | saml | register | magic_link | api_key
	Result    string    `json:"result"`            // ok | bad_password | locked | mfa_failed | oidc_failed
}

// LoginHistoryHandler bindet einen pgx-Pool an den Endpoint.
type LoginHistoryHandler struct {
	db *pgxpool.Pool
}

// NewLoginHistoryHandler baut den Handler.
func NewLoginHistoryHandler(db *pgxpool.Pool) *LoginHistoryHandler {
	return &LoginHistoryHandler{db: db}
}

// RegisterLoginHistory mountet GET /account/login-history.
func RegisterLoginHistory(g *echo.Group, h *LoginHistoryHandler) {
	g.GET("/account/login-history", h.List)
}

// List liefert die letzten 50 Einträge.
func (h *LoginHistoryHandler) List(c echo.Context) error {
	userID, _ := c.Get("user_id").(string)
	if userID == "" {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
	}
	rows, err := h.db.Query(c.Request().Context(), `
		SELECT ts, ip, user_agent, source, result
		FROM login_history
		WHERE user_id = $1::uuid
		ORDER BY ts DESC
		LIMIT 50`, userID)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "db error"})
	}
	defer rows.Close()

	out := make([]LoginHistoryItem, 0, 50)
	for rows.Next() {
		var item LoginHistoryItem
		var ip, ua *string
		if err := rows.Scan(&item.TS, &ip, &ua, &item.Source, &item.Result); err != nil {
			continue
		}
		if ip != nil {
			item.IP = *ip
		}
		if ua != nil {
			item.UserAgent = *ua
		}
		out = append(out, item)
	}
	return c.JSON(http.StatusOK, out)
}
