package scim

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"
)

// dbPool is a minimal subset of pgxpool.Pool used by Service.
// The interface enables DB-free unit tests without an external mock library.
type dbPool interface {
	Begin(ctx context.Context) (pgx.Tx, error)
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// ─── Domain types ─────────────────────────────────────────────────────────────

// SCIMUser is the internal representation of a SCIM User resource.
type SCIMUser struct {
	ID          string
	UserName    string
	DisplayName string
	FirstName   string
	LastName    string
	Email       string
	Active      bool
	ExternalID  string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// SCIMGroup is the internal representation of a SCIM Group resource.
type SCIMGroup struct {
	ID          string
	DisplayName string
	ExternalID  string
	Members     []SCIMGroupMember
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// SCIMGroupMember is a member reference inside a SCIMGroup.
type SCIMGroupMember struct {
	Value   string `json:"value"`
	Display string `json:"display"`
}

// SessionRevoker revokes all active sessions for a user. Implemented by auth.Service.
type SessionRevoker interface {
	RevokeAllSessions(ctx context.Context, userID string) error
}

// Service provides SCIM provisioning operations.
type Service struct {
	db             dbPool
	sessionRevoker SessionRevoker
}

// NewService constructs a SCIM Service.
func NewService(db *pgxpool.Pool) *Service {
	return &Service{db: db}
}

// WithSessionRevoker injects a session revoker so that SCIM-driven
// deactivations immediately invalidate the user's active tokens.
func (s *Service) WithSessionRevoker(r SessionRevoker) *Service {
	s.sessionRevoker = r
	return s
}

// ─── User operations ──────────────────────────────────────────────────────────

// ListUsers returns org-scoped SCIM users, with optional filter by userName or email.
// Only the first "userName eq <value>" clause is evaluated (minimal SCIM filter DSL).
func (s *Service) ListUsers(ctx context.Context, orgID, filter string) ([]SCIMUser, error) {
	baseQuery := `
		SELECT u.id::text, u.email,
		       COALESCE(u.display_name, ''),
		       u.is_active,
		       COALESCE(u.scim_external_id, ''),
		       u.created_at, u.updated_at
		FROM users u
		JOIN org_members om ON om.user_id = u.id
		WHERE om.org_id = $1::uuid`

	args := []any{orgID}

	// Minimal SCIM filter: "userName eq <value>"
	if f := parseEqFilter(filter, "userName"); f != "" {
		args = append(args, f)
		baseQuery += fmt.Sprintf(" AND u.email = $%d", len(args))
	}
	baseQuery += " ORDER BY u.created_at ASC"

	rows, err := s.db.Query(ctx, baseQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("list scim users: %w", err)
	}
	defer rows.Close()

	return scanUsers(rows)
}

// GetUser returns a single user scoped to the org.
func (s *Service) GetUser(ctx context.Context, orgID, userID string) (*SCIMUser, error) {
	var u SCIMUser
	err := s.db.QueryRow(ctx, `
		SELECT u.id::text, u.email,
		       COALESCE(u.display_name, ''),
		       u.is_active,
		       COALESCE(u.scim_external_id, ''),
		       u.created_at, u.updated_at
		FROM users u
		JOIN org_members om ON om.user_id = u.id
		WHERE om.org_id = $1::uuid AND u.id = $2::uuid`,
		orgID, userID,
	).Scan(&u.ID, &u.Email, &u.DisplayName, &u.Active, &u.ExternalID,
		&u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		return nil, err
	}
	u.UserName = u.Email
	return &u, nil
}

// CreateUser provisions a new user into the org.
// If a user with that email already exists, they are added to the org (upserted).
func (s *Service) CreateUser(ctx context.Context, orgID string, u SCIMUser) (*SCIMUser, error) {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback(ctx)
		}
	}()

	displayName := u.DisplayName
	if displayName == "" && (u.FirstName != "" || u.LastName != "") {
		displayName = strings.TrimSpace(u.FirstName + " " + u.LastName)
	}

	var userID string
	err = tx.QueryRow(ctx, `
		INSERT INTO users (email, display_name, is_active, scim_external_id, scim_provisioned)
		VALUES ($1, NULLIF($2,''), $3, NULLIF($4,''), TRUE)
		ON CONFLICT (email) DO UPDATE
		    SET display_name     = COALESCE(NULLIF($2,''), users.display_name),
		        is_active        = $3,
		        scim_external_id = COALESCE(NULLIF($4,''), users.scim_external_id),
		        scim_provisioned = TRUE,
		        updated_at       = NOW()
		RETURNING id::text`,
		u.Email, displayName, u.Active, u.ExternalID,
	).Scan(&userID)
	if err != nil {
		return nil, fmt.Errorf("upsert user: %w", err)
	}

	// Link to org with the default Viewer role; skip if already a member.
	_, err = tx.Exec(ctx, `
		INSERT INTO org_members (org_id, user_id, role_id)
		SELECT $1::uuid, $2::uuid, r.id FROM roles r WHERE r.name = 'Viewer'
		ON CONFLICT (org_id, user_id) DO NOTHING`,
		orgID, userID)
	if err != nil {
		return nil, fmt.Errorf("link org member: %w", err)
	}

	if err = tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}

	return s.GetUser(ctx, orgID, userID)
}

// ReplaceUser performs a full PUT replacement of a SCIM user.
func (s *Service) ReplaceUser(ctx context.Context, orgID, userID string, u SCIMUser) (*SCIMUser, error) {
	displayName := u.DisplayName
	if displayName == "" && (u.FirstName != "" || u.LastName != "") {
		displayName = strings.TrimSpace(u.FirstName + " " + u.LastName)
	}

	tag, err := s.db.Exec(ctx, `
		UPDATE users
		   SET email            = $3,
		       display_name     = NULLIF($4,''),
		       is_active        = $5,
		       scim_external_id = NULLIF($6,''),
		       updated_at       = NOW()
		 FROM org_members om
		WHERE users.id = $2::uuid
		  AND om.user_id = users.id
		  AND om.org_id  = $1::uuid`,
		orgID, userID, u.Email, displayName, u.Active, u.ExternalID,
	)
	if err != nil {
		return nil, fmt.Errorf("replace user: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return nil, pgx.ErrNoRows
	}
	return s.GetUser(ctx, orgID, userID)
}

// PatchUser applies SCIM PATCH operations to a user.
// Only "replace" op on "active", "displayName", "userName", and "externalId" is supported.
func (s *Service) PatchUser(ctx context.Context, orgID, userID string, ops []PatchOp) (*SCIMUser, error) {
	existing, err := s.GetUser(ctx, orgID, userID)
	if err != nil {
		return nil, err
	}

	for _, op := range ops {
		if !strings.EqualFold(op.Op, "replace") {
			continue // only replace is supported
		}
		switch strings.ToLower(op.Path) {
		case "active":
			if b, ok := op.Value.(bool); ok {
				existing.Active = b
			}
		case "displayname":
			if v, ok := op.Value.(string); ok {
				existing.DisplayName = v
			}
		case "username", "emails[type eq \"work\"].value":
			if v, ok := op.Value.(string); ok {
				existing.Email = v
				existing.UserName = v
			}
		case "externalid":
			if v, ok := op.Value.(string); ok {
				existing.ExternalID = v
			}
		}
	}

	return s.ReplaceUser(ctx, orgID, userID, *existing)
}

// DeactivateUser soft-deletes a SCIM-provisioned user from the org.
// The user row is set is_active=false; the org_members row is removed.
func (s *Service) DeactivateUser(ctx context.Context, orgID, userID string) error {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback(ctx)
		}
	}()

	// Remove from org.
	_, err = tx.Exec(ctx, `
		DELETE FROM org_members WHERE org_id = $1::uuid AND user_id = $2::uuid`,
		orgID, userID)
	if err != nil {
		return fmt.Errorf("remove org member: %w", err)
	}

	// Deactivate the user account (soft-delete for SCIM-provisioned users).
	_, err = tx.Exec(ctx, `
		UPDATE users SET is_active = FALSE, updated_at = NOW()
		 WHERE id = $1::uuid AND scim_provisioned = TRUE`,
		userID)
	if err != nil {
		return fmt.Errorf("deactivate user: %w", err)
	}

	if err = tx.Commit(ctx); err != nil {
		return err
	}
	log.Info().Str("org_id", orgID).Str("user_id", userID).Msg("scim: user deactivated")
	// Revoke active sessions so deprovisioned users lose access immediately (AUTH-007).
	if s.sessionRevoker != nil {
		if rErr := s.sessionRevoker.RevokeAllSessions(ctx, userID); rErr != nil {
			log.Warn().Err(rErr).Str("user_id", userID).Msg("scim: session revocation failed after deactivation")
		}
	}
	return nil
}

// ─── Group operations ─────────────────────────────────────────────────────────

// ListGroups returns org-scoped SCIM groups, with optional displayName filter.
func (s *Service) ListGroups(ctx context.Context, orgID, filter string) ([]SCIMGroup, error) {
	baseQuery := `
		SELECT id::text, display_name, COALESCE(external_id,''), created_at, updated_at
		FROM scim_groups WHERE org_id = $1::uuid`
	args := []any{orgID}

	if f := parseEqFilter(filter, "displayName"); f != "" {
		args = append(args, f)
		baseQuery += fmt.Sprintf(" AND display_name = $%d", len(args))
	}
	baseQuery += " ORDER BY created_at ASC"

	rows, err := s.db.Query(ctx, baseQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("list scim groups: %w", err)
	}
	defer rows.Close()

	var groups []SCIMGroup
	for rows.Next() {
		var g SCIMGroup
		if err := rows.Scan(&g.ID, &g.DisplayName, &g.ExternalID, &g.CreatedAt, &g.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan scim group: %w", err)
		}
		groups = append(groups, g)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate scim groups: %w", err)
	}

	for i := range groups {
		members, err := s.loadGroupMembers(ctx, groups[i].ID)
		if err != nil {
			return nil, err
		}
		groups[i].Members = members
	}
	return groups, nil
}

// GetGroup returns a single SCIM group scoped to the org.
func (s *Service) GetGroup(ctx context.Context, orgID, groupID string) (*SCIMGroup, error) {
	var g SCIMGroup
	err := s.db.QueryRow(ctx, `
		SELECT id::text, display_name, COALESCE(external_id,''), created_at, updated_at
		FROM scim_groups WHERE org_id = $1::uuid AND id = $2::uuid`,
		orgID, groupID,
	).Scan(&g.ID, &g.DisplayName, &g.ExternalID, &g.CreatedAt, &g.UpdatedAt)
	if err != nil {
		return nil, err
	}
	members, err := s.loadGroupMembers(ctx, g.ID)
	if err != nil {
		return nil, err
	}
	g.Members = members
	return &g, nil
}

// CreateGroup creates a new SCIM group in the org.
func (s *Service) CreateGroup(ctx context.Context, orgID string, g SCIMGroup) (*SCIMGroup, error) {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback(ctx)
		}
	}()

	var groupID string
	err = tx.QueryRow(ctx, `
		INSERT INTO scim_groups (org_id, display_name, external_id)
		VALUES ($1::uuid, $2, NULLIF($3,''))
		ON CONFLICT (org_id, display_name) DO UPDATE
		    SET external_id = COALESCE(NULLIF($3,''), scim_groups.external_id),
		        updated_at  = NOW()
		RETURNING id::text`,
		orgID, g.DisplayName, g.ExternalID,
	).Scan(&groupID)
	if err != nil {
		return nil, fmt.Errorf("upsert scim group: %w", err)
	}

	for _, m := range g.Members {
		if _, err = tx.Exec(ctx, `
			INSERT INTO scim_group_members (group_id, user_id)
			VALUES ($1::uuid, $2::uuid)
			ON CONFLICT DO NOTHING`,
			groupID, m.Value,
		); err != nil {
			log.Warn().Err(err).Str("user_id", m.Value).Msg("scim: skip member that does not exist")
		}
	}

	if err = tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}
	return s.GetGroup(ctx, orgID, groupID)
}

// ReplaceGroup performs a full PUT replacement of a SCIM group.
func (s *Service) ReplaceGroup(ctx context.Context, orgID, groupID string, g SCIMGroup) (*SCIMGroup, error) {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback(ctx)
		}
	}()

	tag, err := tx.Exec(ctx, `
		UPDATE scim_groups SET display_name = $3,
		                       external_id  = NULLIF($4,''),
		                       updated_at   = NOW()
		 WHERE id = $2::uuid AND org_id = $1::uuid`,
		orgID, groupID, g.DisplayName, g.ExternalID,
	)
	if err != nil {
		return nil, fmt.Errorf("update scim group: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return nil, pgx.ErrNoRows
	}

	// Replace members: delete all, then re-insert.
	if _, err = tx.Exec(ctx,
		`DELETE FROM scim_group_members WHERE group_id = $1::uuid`, groupID,
	); err != nil {
		return nil, fmt.Errorf("clear group members: %w", err)
	}
	for _, m := range g.Members {
		if _, err = tx.Exec(ctx, `
			INSERT INTO scim_group_members (group_id, user_id)
			VALUES ($1::uuid, $2::uuid) ON CONFLICT DO NOTHING`,
			groupID, m.Value,
		); err != nil {
			log.Warn().Err(err).Str("user_id", m.Value).Msg("scim: skip member that does not exist")
		}
	}

	if err = tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}
	return s.GetGroup(ctx, orgID, groupID)
}

// PatchGroup applies SCIM PATCH operations to a group.
func (s *Service) PatchGroup(ctx context.Context, orgID, groupID string, ops []PatchOp) (*SCIMGroup, error) {
	existing, err := s.GetGroup(ctx, orgID, groupID)
	if err != nil {
		return nil, err
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback(ctx)
		}
	}()

	for _, op := range ops {
		switch strings.ToLower(op.Op) {
		case "replace":
			if strings.EqualFold(op.Path, "displayName") {
				if v, ok := op.Value.(string); ok {
					existing.DisplayName = v
				}
			}
		case "add":
			if strings.EqualFold(op.Path, "members") {
				members := parseMemberValues(op.Value)
				for _, uid := range members {
					if _, err = tx.Exec(ctx, `
						INSERT INTO scim_group_members (group_id, user_id)
						VALUES ($1::uuid, $2::uuid) ON CONFLICT DO NOTHING`,
						groupID, uid,
					); err != nil {
						log.Warn().Err(err).Str("user_id", uid).Msg("scim: skip member add")
					}
				}
			}
		case "remove":
			if strings.EqualFold(op.Path, "members") {
				members := parseMemberValues(op.Value)
				for _, uid := range members {
					if _, err = tx.Exec(ctx, `
						DELETE FROM scim_group_members
						WHERE group_id = $1::uuid AND user_id = $2::uuid`,
						groupID, uid,
					); err != nil {
						log.Warn().Err(err).Str("user_id", uid).Msg("scim: skip member remove")
					}
				}
			}
		}
	}

	if _, err = tx.Exec(ctx, `
		UPDATE scim_groups SET display_name = $3, updated_at = NOW()
		 WHERE id = $2::uuid AND org_id = $1::uuid`,
		orgID, groupID, existing.DisplayName,
	); err != nil {
		return nil, fmt.Errorf("patch group: %w", err)
	}

	if err = tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}
	return s.GetGroup(ctx, orgID, groupID)
}

// DeleteGroup removes a SCIM group and its member links.
func (s *Service) DeleteGroup(ctx context.Context, orgID, groupID string) error {
	tag, err := s.db.Exec(ctx, `
		DELETE FROM scim_groups WHERE id = $2::uuid AND org_id = $1::uuid`,
		orgID, groupID)
	if err != nil {
		return fmt.Errorf("delete scim group: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

func scanUsers(rows pgx.Rows) ([]SCIMUser, error) {
	var users []SCIMUser
	for rows.Next() {
		var u SCIMUser
		if err := rows.Scan(&u.ID, &u.Email, &u.DisplayName, &u.Active,
			&u.ExternalID, &u.CreatedAt, &u.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan scim user: %w", err)
		}
		u.UserName = u.Email
		users = append(users, u)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate scim user rows: %w", err)
	}
	return users, nil
}

func (s *Service) loadGroupMembers(ctx context.Context, groupID string) ([]SCIMGroupMember, error) {
	rows, err := s.db.Query(ctx, `
		SELECT u.id::text, COALESCE(u.display_name, u.email)
		FROM scim_group_members gm
		JOIN users u ON u.id = gm.user_id
		WHERE gm.group_id = $1::uuid`, groupID)
	if err != nil {
		return nil, fmt.Errorf("load group members: %w", err)
	}
	defer rows.Close()

	var members []SCIMGroupMember
	for rows.Next() {
		var m SCIMGroupMember
		if err := rows.Scan(&m.Value, &m.Display); err != nil {
			return nil, fmt.Errorf("scan group member: %w", err)
		}
		members = append(members, m)
	}
	return members, rows.Err()
}

// parseEqFilter extracts the value from a simple SCIM filter like:
// "userName eq \"alice\"" or "displayName eq \"Team X\"".
func parseEqFilter(filter, attr string) string {
	if filter == "" {
		return ""
	}
	// Accept: <attr> eq "<value>" or <attr> eq <value>
	prefix := strings.ToLower(attr) + " eq "
	lower := strings.ToLower(filter)
	idx := strings.Index(lower, prefix)
	if idx == -1 {
		return ""
	}
	val := filter[idx+len(prefix):]
	val = strings.TrimSpace(val)
	val = strings.Trim(val, `"`)
	return val
}

// parseMemberValues extracts user IDs from a PATCH members value.
// Value may be []any of map[string]any{"value": "<id>"} or a single such map.
func parseMemberValues(v any) []string {
	var ids []string
	switch typed := v.(type) {
	case []any:
		for _, item := range typed {
			if m, ok := item.(map[string]any); ok {
				if uid, ok := m["value"].(string); ok {
					ids = append(ids, uid)
				}
			}
		}
	case map[string]any:
		if uid, ok := typed["value"].(string); ok {
			ids = append(ids, uid)
		}
	}
	return ids
}
