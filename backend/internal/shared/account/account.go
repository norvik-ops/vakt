// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

// Package account provides per-user self-service endpoints required by DSGVO
// Articles 17 (right to erasure) and 20 (right to data portability).
//
// All endpoints are mounted under /account on the authenticated API surface —
// they always operate on the calling user (no path parameter), so a user can
// never act on another user's data through these routes.
package account

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog/log"
	"golang.org/x/crypto/bcrypt"

	"github.com/matharnica/vakt/internal/shared/audit"
)

// Service orchestrates DSGVO Art. 17 and Art. 20 actions for the calling user.
//
// Why the service owns audit.Write calls (P2-19): the audit-log entry for an
// account deletion or export MUST be written even if the API is consumed
// outside of an HTTP context — e.g. a future CLI or scheduled job. Keeping
// auditing in the handler couples it to Echo and creates a gap when the
// service is used as an SDK. The service writes the audit entry; the handler
// only enriches the context with caller metadata (IP, user-agent).
type Service struct {
	db *pgxpool.Pool
}

// NewService creates an account service backed by the given DB pool.
func NewService(db *pgxpool.Pool) *Service { return &Service{db: db} }

// ── Data Export (DSGVO Art. 20) ──────────────────────────────────────────────

// ExportUserData assembles a ZIP archive of all data the calling user owns or
// is identified in. The archive contains:
//
//   - profile.json          — user record (no password hash)
//   - sessions.json         — refresh-session metadata (no token hashes)
//   - api_keys.json         — own API-key metadata (no key values)
//   - notification_prefs    — notification preferences
//   - audit_log.json        — audit-log entries where user_id = me
//   - comments.json         — comments authored by me (across modules)
//   - meta.json             — export metadata (date, app version, user id)
//
// Returns the ZIP bytes ready to stream as application/zip.
func (s *Service) ExportUserData(ctx context.Context, userID, userEmail, orgID string) ([]byte, error) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)

	// 1. Profile
	type userProfile struct {
		ID          string     `json:"id"`
		Email       string     `json:"email"`
		DisplayName *string    `json:"display_name,omitempty"`
		AvatarURL   *string    `json:"avatar_url,omitempty"`
		OIDCSubject *string    `json:"oidc_subject,omitempty"`
		OIDCProvi   *string    `json:"oidc_provider,omitempty"`
		IsActive    bool       `json:"is_active"`
		LastLoginAt *time.Time `json:"last_login_at,omitempty"`
		CreatedAt   time.Time  `json:"created_at"`
		UpdatedAt   time.Time  `json:"updated_at"`
	}
	var profile userProfile
	err := s.db.QueryRow(ctx, `
		SELECT id::text, email, display_name, avatar_url, oidc_subject, oidc_provider,
		       is_active, last_login_at, created_at, updated_at
		FROM users WHERE id = $1::uuid
	`, userID).Scan(
		&profile.ID, &profile.Email, &profile.DisplayName, &profile.AvatarURL,
		&profile.OIDCSubject, &profile.OIDCProvi, &profile.IsActive,
		&profile.LastLoginAt, &profile.CreatedAt, &profile.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("load profile: %w", err)
	}
	if err := writeJSON(zw, "profile.json", profile); err != nil {
		return nil, err
	}

	// 2-N. User-scoped tables — best effort: missing tables in the schema do
	// not abort the export, they just write an empty array.
	tableExports := []struct {
		file  string
		query string
	}{
		{"sessions.json", `SELECT id::text, device_hint, created_at, last_used, expires_at, revoked_at
		                    FROM refresh_sessions WHERE user_id = $1::uuid ORDER BY created_at`},
		{"api_keys.json", `SELECT id::text, name, key_prefix, scopes, last_used_at, expires_at, revoked_at, created_at
		                    FROM api_keys WHERE created_by = $1::uuid ORDER BY created_at`},
		{"audit_log.json", `SELECT id::text, action, resource_type, resource_id, ip_address, created_at
		                     FROM audit_log WHERE user_id = $1::uuid ORDER BY created_at`},
		{"notification_preferences.json", `SELECT * FROM notification_preferences WHERE user_id = $1::uuid`},
	}
	for _, e := range tableExports {
		data, err := queryToJSON(ctx, s.db, userID, e.query)
		if err != nil {
			log.Debug().Err(err).Str("file", e.file).Msg("export: skipping table (likely absent)")
			data = []byte("[]")
		}
		if err := writeRaw(zw, e.file, data); err != nil {
			return nil, fmt.Errorf("%s: %w", e.file, err)
		}
	}

	// Comments use author_email rather than author_id in some places — query
	// by email AND user_id where available.
	commentsData, err := queryCommentsByUser(ctx, s.db, userID, userEmail)
	if err != nil {
		commentsData = []byte("[]")
	}
	if err := writeRaw(zw, "comments.json", commentsData); err != nil {
		return nil, fmt.Errorf("comments: %w", err)
	}

	// meta.json with export-time metadata.
	meta := map[string]string{
		"export_date":   time.Now().UTC().Format(time.RFC3339),
		"user_id":       userID,
		"user_email":    userEmail,
		"org_id":        orgID,
		"dsgvo_article": "Art. 20 — Right to data portability",
		"machine_readable": "JSON files inside a ZIP. Each file is a JSON array " +
			"of records, except profile.json (single object) and meta.json (this file).",
	}
	if err := writeJSON(zw, "meta.json", meta); err != nil {
		return nil, err
	}

	if err := zw.Close(); err != nil {
		return nil, fmt.Errorf("close zip: %w", err)
	}
	return buf.Bytes(), nil
}

// ── Account Deletion (DSGVO Art. 17) ─────────────────────────────────────────

// ErrInvalidPassword is returned when a deletion confirmation password fails.
var ErrInvalidPassword = fmt.Errorf("invalid password")

// ErrLastAdmin is returned when deleting the last admin in an org — that would
// orphan the org and is disallowed without a transfer-of-ownership flow.
var ErrLastAdmin = fmt.Errorf("cannot delete the last admin of an organisation")

// DeleteUserAccount anonymizes the user account in place rather than hard-
// deleting it. The strategy is deliberate:
//
//   - Audit-log integrity (ISO 27001 A.5.28, BSI ORP.2): historical entries
//     must remain attributable to *some* identifier, even if the human is
//     gone. We replace email and display_name with "deleted-<uuid>@vakt.local"
//     and "[gelöscht]" so the user_id remains stable for joins.
//   - Sessions and API keys are revoked immediately.
//   - The user is marked inactive, the password hash is wiped, and any OIDC
//     subject mapping is cleared (prevents zombie SSO logins).
//
// Hard-delete is the wrong answer for a compliance product — DSGVO Art. 17 (3)
// allows retention "for the establishment, exercise or defence of legal claims"
// and Art. 5 (1)(e) is met by anonymisation. If a customer specifically demands
// hard-delete, they run `DELETE FROM users WHERE id = …` themselves on the DB.
func (s *Service) DeleteUserAccount(ctx context.Context, userID, suppliedPassword, ipAddress string) error {
	// 1. Re-authenticate: load the hash and verify the supplied password.
	var hash string
	var oidcSubject *string
	if err := s.db.QueryRow(ctx,
		`SELECT password_hash, oidc_subject FROM users WHERE id = $1::uuid`, userID,
	).Scan(&hash, &oidcSubject); err != nil {
		return fmt.Errorf("load user: %w", err)
	}
	// SSO-only users (no password) can delete without a password — they
	// already proved identity via OIDC for this session.
	if hash != "" {
		if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(suppliedPassword)); err != nil {
			return ErrInvalidPassword
		}
	}

	// 2. Prevent orphaning an organisation: if this user is the sole admin,
	// refuse and ask them to transfer ownership first.
	if err := s.guardLastAdmin(ctx, userID); err != nil {
		return err
	}

	// 3. Perform the anonymisation in a transaction so partial failures do
	// not leave a half-deleted account behind.
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	anonEmail := fmt.Sprintf("deleted-%s@vakt.local", userID)
	if _, err := tx.Exec(ctx, `
		UPDATE users
		SET email          = $1,
		    password_hash  = NULL,
		    display_name   = '[gelöscht]',
		    avatar_url     = NULL,
		    oidc_subject   = NULL,
		    oidc_provider  = NULL,
		    is_active      = FALSE,
		    updated_at     = NOW()
		WHERE id = $2::uuid
	`, anonEmail, userID); err != nil {
		return fmt.Errorf("anonymise user: %w", err)
	}
	// Revoke sessions + api keys (failure is non-fatal — already in tx).
	if _, err := tx.Exec(ctx,
		`UPDATE refresh_sessions SET revoked_at = NOW() WHERE user_id = $1::uuid AND revoked_at IS NULL`,
		userID,
	); err != nil {
		log.Warn().Err(err).Msg("delete: revoke sessions")
	}
	if _, err := tx.Exec(ctx,
		`UPDATE api_keys SET revoked_at = NOW() WHERE created_by = $1::uuid AND revoked_at IS NULL`,
		userID,
	); err != nil {
		log.Warn().Err(err).Msg("delete: revoke api keys")
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit: %w", err)
	}

	// 4. Audit-log AFTER the commit so a rolled-back deletion does not
	// produce a misleading "deleted" entry.
	audit.Write(ctx, s.db, audit.WriteEntry{
		UserID:       userID,
		Action:       "delete",
		ResourceType: "account",
		ResourceID:   userID,
		IPAddress:    ipAddress,
	})

	return nil
}

func (s *Service) guardLastAdmin(ctx context.Context, userID string) error {
	// For each org this user is an Admin of, check whether at least one other
	// active Admin exists. If any org would be orphaned, refuse.
	rows, err := s.db.Query(ctx, `
		SELECT om.org_id::text FROM org_members om
		JOIN roles r ON r.id = om.role_id
		WHERE om.user_id = $1::uuid AND r.name = 'Admin'
	`, userID)
	if err != nil {
		// Fail open on lookup errors — better an extra stranded org than
		// locking a user out of their right to deletion.
		return nil //nolint:nilerr
	}
	defer rows.Close()

	var adminOrgIDs []string
	for rows.Next() {
		var orgID string
		if err := rows.Scan(&orgID); err != nil {
			continue
		}
		adminOrgIDs = append(adminOrgIDs, orgID)
	}

	for _, orgID := range adminOrgIDs {
		var otherAdmins int
		_ = s.db.QueryRow(ctx, `
			SELECT COUNT(*) FROM org_members om
			JOIN roles r ON r.id = om.role_id
			JOIN users u ON u.id = om.user_id
			WHERE om.org_id = $1::uuid
			  AND om.user_id != $2::uuid
			  AND u.is_active = TRUE
			  AND r.name = 'Admin'
		`, orgID, userID).Scan(&otherAdmins)
		if otherAdmins == 0 {
			return ErrLastAdmin
		}
	}
	return nil
}

// ── helpers ──────────────────────────────────────────────────────────────────

func queryToJSON(ctx context.Context, db *pgxpool.Pool, userID, query string) ([]byte, error) {
	rows, err := db.Query(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	fields := rows.FieldDescriptions()
	results := []map[string]any{}
	for rows.Next() {
		vals, err := rows.Values()
		if err != nil {
			return nil, err
		}
		row := make(map[string]any, len(fields))
		for i, f := range fields {
			row[string(f.Name)] = vals[i]
		}
		results = append(results, row)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return json.Marshal(results)
}

func queryCommentsByUser(ctx context.Context, db *pgxpool.Pool, userID, userEmail string) ([]byte, error) {
	rows, err := db.Query(ctx, `
		SELECT id::text, resource_type, resource_id, author_email, body, created_at
		FROM ck_comments
		WHERE author_id = $1::uuid OR author_email = $2
		ORDER BY created_at
	`, userID, userEmail)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	type comment struct {
		ID           string    `json:"id"`
		ResourceType string    `json:"resource_type"`
		ResourceID   string    `json:"resource_id"`
		AuthorEmail  string    `json:"author_email"`
		Body         string    `json:"body"`
		CreatedAt    time.Time `json:"created_at"`
	}
	out := []comment{}
	for rows.Next() {
		var c comment
		if err := rows.Scan(&c.ID, &c.ResourceType, &c.ResourceID, &c.AuthorEmail, &c.Body, &c.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return json.Marshal(out)
}

func writeJSON(zw *zip.Writer, name string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return writeRaw(zw, name, data)
}

func writeRaw(zw *zip.Writer, name string, data []byte) error {
	f, err := zw.Create(name)
	if err != nil {
		return err
	}
	_, err = f.Write(data)
	return err
}

// ── HTTP layer ───────────────────────────────────────────────────────────────

// Handler exposes the account service over HTTP.
type Handler struct{ svc *Service }

func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

// ExportData handles GET /api/v1/account/data-export and streams a ZIP of the
// caller's personal data (DSGVO Art. 20).
func (h *Handler) ExportData(c echo.Context) error {
	userID, _ := c.Get("user_id").(string)
	userEmail, _ := c.Get("user_email").(string)
	orgID, _ := c.Get("org_id").(string)
	if userID == "" {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
	}

	data, err := h.svc.ExportUserData(c.Request().Context(), userID, userEmail, orgID)
	if err != nil {
		log.Error().Err(err).Msg("account: export failed")
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "export failed", "code": "ACCOUNT_EXPORT_FAILED",
		})
	}

	filename := fmt.Sprintf("vakt-personal-data-%s.zip", time.Now().UTC().Format("2006-01-02"))
	c.Response().Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
	return c.Blob(http.StatusOK, "application/zip", data)
}

// DeleteAccountInput is the request body for self-service account deletion.
type DeleteAccountInput struct {
	Password string `json:"password"`
	// Confirm must literally be the string "LÖSCHEN" so a misclicked button
	// or stale request body cannot silently wipe an account.
	Confirm string `json:"confirm"`
}

// DeleteAccount handles POST /api/v1/account/delete (DSGVO Art. 17).
func (h *Handler) DeleteAccount(c echo.Context) error {
	userID, _ := c.Get("user_id").(string)
	if userID == "" {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
	}

	var in DeleteAccountInput
	if err := c.Bind(&in); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "invalid body", "code": "ACCOUNT_BAD_BODY",
		})
	}
	if in.Confirm != "LÖSCHEN" {
		return c.JSON(http.StatusUnprocessableEntity, map[string]string{
			"error": "Bitte 'LÖSCHEN' in das Bestätigungsfeld eintragen.",
			"code":  "ACCOUNT_CONFIRM_REQUIRED",
		})
	}

	err := h.svc.DeleteUserAccount(c.Request().Context(), userID, in.Password, c.RealIP())
	switch err {
	case nil:
		// On success, clear cookies so the now-anonymised account cannot be re-used.
		c.SetCookie(&http.Cookie{Name: "access_token", Value: "", Path: "/api/v1", MaxAge: -1})
		c.SetCookie(&http.Cookie{Name: "csrf_token", Value: "", Path: "/api/v1", MaxAge: -1})
		return c.JSON(http.StatusOK, map[string]string{"status": "deleted"})
	case ErrInvalidPassword:
		return c.JSON(http.StatusUnauthorized, map[string]string{
			"error": "Passwort falsch.", "code": "ACCOUNT_INVALID_PASSWORD",
		})
	case ErrLastAdmin:
		return c.JSON(http.StatusConflict, map[string]string{
			"error": "Du bist der letzte Admin dieser Organisation. Bitte zuerst einen anderen Admin ernennen.",
			"code":  "ACCOUNT_LAST_ADMIN",
		})
	default:
		log.Error().Err(err).Msg("account: delete failed")
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "delete failed", "code": "ACCOUNT_DELETE_FAILED",
		})
	}
}

// Register mounts the account routes on the given protected group.
func Register(g *echo.Group, h *Handler) {
	g.GET("/account/data-export", h.ExportData)
	g.POST("/account/delete", h.DeleteAccount)
}
