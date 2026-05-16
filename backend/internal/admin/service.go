package admin

import (
	"context"
	"fmt"
	"time"

	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/sechealth-app/sechealth/internal/shared/notify"
)

// AuditLog is a single audit log entry as returned by the admin API.
// Fields are sourced from the audit_log table (migration 064). The former
// audit_logs table (migration 004) was consolidated into audit_log by
// migration 082; user_agent and status_code no longer exist in the schema.
type AuditLog struct {
	ID           string    `json:"id"`
	OrgID        string    `json:"org_id"`
	UserID       *string   `json:"user_id,omitempty"`
	Action       string    `json:"action"`
	ResourceType string    `json:"resource_type"`
	ResourceID   *string   `json:"resource_id,omitempty"`
	IPAddress    *string   `json:"ip_address,omitempty"`
	Timestamp    time.Time `json:"timestamp"`
}

// OrgMember represents a user within an organisation.
type OrgMember struct {
	UserID      string    `json:"user_id"`
	Email       string    `json:"email"`
	DisplayName string    `json:"display_name"`
	Role        string    `json:"role"`
	JoinedAt    time.Time `json:"joined_at"`
}

// InviteInput is the request body for POST /admin/users/invite.
type InviteInput struct {
	Email string `json:"email" validate:"required,email"`
	Role  string `json:"role"  validate:"required,oneof=Admin SecurityAnalyst Viewer AuditorReadOnly"`
}

// RoleUpdateInput is the request body for PATCH /admin/users/:id/role.
type RoleUpdateInput struct {
	Role string `json:"role" validate:"required,oneof=Admin SecurityAnalyst Viewer AuditorReadOnly"`
}

// ModuleStatus describes the enabled/disabled state of a platform module.
type ModuleStatus struct {
	Name    string `json:"name"`
	Enabled bool   `json:"enabled"`
}

// Service implements admin business logic.
type Service struct {
	db             *pgxpool.Pool
	modulesEnabled string // comma-separated list from config
	MSP            *MSPService
	notifySvc      *notify.Service
}

// NewService constructs an admin Service.
// asynqClient may be nil when Redis is not available; MSP deletion degrades gracefully.
func NewService(db *pgxpool.Pool, modulesEnabled string, asynqClient *asynq.Client) *Service {
	repo := NewRepository(db)
	return &Service{
		db:             db,
		modulesEnabled: modulesEnabled,
		MSP:            newMSPService(repo, asynqClient),
	}
}

// WithNotifyService attaches a notify.Service to the admin Service for channel management.
func (s *Service) WithNotifyService(ns *notify.Service) *Service {
	s.notifySvc = ns
	return s
}

// ListNotificationChannels delegates to the notify service.
func (s *Service) ListNotificationChannels(ctx context.Context, orgID string) ([]notify.NotificationChannel, error) {
	if s.notifySvc == nil {
		return nil, fmt.Errorf("notification service not configured")
	}
	return s.notifySvc.ListNotificationChannels(ctx, orgID)
}

// CreateNotificationChannel delegates to the notify service.
func (s *Service) CreateNotificationChannel(ctx context.Context, orgID string, input notify.CreateChannelInput) (*notify.NotificationChannel, error) {
	if s.notifySvc == nil {
		return nil, fmt.Errorf("notification service not configured")
	}
	return s.notifySvc.CreateNotificationChannel(ctx, orgID, input)
}

// DeleteNotificationChannel delegates to the notify service.
func (s *Service) DeleteNotificationChannel(ctx context.Context, orgID, channelID string) error {
	if s.notifySvc == nil {
		return fmt.Errorf("notification service not configured")
	}
	return s.notifySvc.DeleteNotificationChannel(ctx, orgID, channelID)
}

// ListAuditLogs returns a paginated, filterable list of audit entries for the org.
func (s *Service) ListAuditLogs(ctx context.Context, orgID string, page, limit int, userID, action, resourceType string) ([]AuditLog, int, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 500 {
		limit = 25
	}
	offset := (page - 1) * limit

	// Build the WHERE clause dynamically to avoid excessive SQL branching.
	args := []interface{}{orgID}
	where := "org_id = $1::uuid"
	argN := 2

	if userID != "" {
		where += fmt.Sprintf(" AND user_id = $%d::uuid", argN)
		args = append(args, userID)
		argN++
	}
	if action != "" {
		where += fmt.Sprintf(" AND action = $%d", argN)
		args = append(args, action)
		argN++
	}
	if resourceType != "" {
		where += fmt.Sprintf(" AND resource_type = $%d", argN)
		args = append(args, resourceType)
		argN++
	}

	// Count total.
	// Query audit_log directly (not the audit_logs VIEW) so that created_at is
	// available as a real column; the VIEW aliased it as "timestamp" and exposed
	// user_agent / status_code as NULLs that no longer have meaning.
	var total int
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM audit_log WHERE %s", where)
	if err := s.db.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count audit logs: %w", err)
	}

	// Fetch page.
	dataArgs := append(args, limit, offset)
	rows, err := s.db.Query(ctx, fmt.Sprintf(`
		SELECT id::text, org_id::text, user_id::text, action, resource_type,
		       resource_id, ip_address, created_at
		FROM audit_log
		WHERE %s
		ORDER BY created_at DESC
		LIMIT $%d OFFSET $%d`, where, argN, argN+1), dataArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("query audit logs: %w", err)
	}
	defer rows.Close()

	var logs []AuditLog
	for rows.Next() {
		var l AuditLog
		if err := rows.Scan(
			&l.ID, &l.OrgID, &l.UserID, &l.Action, &l.ResourceType,
			&l.ResourceID, &l.IPAddress, &l.Timestamp,
		); err != nil {
			return nil, 0, fmt.Errorf("scan audit log row: %w", err)
		}
		logs = append(logs, l)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate audit log rows: %w", err)
	}

	return logs, total, nil
}

// ListUsers returns all members of the given organisation.
func (s *Service) ListUsers(ctx context.Context, orgID string) ([]OrgMember, error) {
	rows, err := s.db.Query(ctx, `
		SELECT u.id::text, u.email, COALESCE(u.display_name, ''), r.name, om.joined_at
		FROM org_members om
		JOIN users u ON u.id = om.user_id
		JOIN roles r ON r.id = om.role_id
		WHERE om.org_id = $1::uuid
		ORDER BY om.joined_at ASC`, orgID)
	if err != nil {
		return nil, fmt.Errorf("query org members: %w", err)
	}
	defer rows.Close()

	var members []OrgMember
	for rows.Next() {
		var m OrgMember
		if err := rows.Scan(&m.UserID, &m.Email, &m.DisplayName, &m.Role, &m.JoinedAt); err != nil {
			return nil, fmt.Errorf("scan org member: %w", err)
		}
		members = append(members, m)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate org member rows: %w", err)
	}
	return members, nil
}

// InviteUser creates a pending invitation for a new org member.
// The user row is created immediately with is_active=false; activation flow
// is handled outside this epic.
func (s *Service) InviteUser(ctx context.Context, orgID, invitedByID string, input InviteInput) error {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback(ctx)
		}
	}()

	// Upsert the invited user (may already exist in another org).
	var userID string
	err = tx.QueryRow(ctx, `
		INSERT INTO users (email, is_active)
		VALUES ($1, FALSE)
		ON CONFLICT (email) DO UPDATE SET email = EXCLUDED.email
		RETURNING id::text`, input.Email).Scan(&userID)
	if err != nil {
		return fmt.Errorf("upsert invited user: %w", err)
	}

	var roleID string
	err = tx.QueryRow(ctx, `SELECT id::text FROM roles WHERE name = $1`, input.Role).Scan(&roleID)
	if err != nil {
		return fmt.Errorf("lookup role %q: %w", input.Role, err)
	}

	var inviterID *string
	if invitedByID != "" {
		inviterID = &invitedByID
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO org_members (org_id, user_id, role_id, invited_by)
		VALUES ($1::uuid, $2::uuid, $3::uuid, $4::uuid)
		ON CONFLICT (org_id, user_id) DO NOTHING`,
		orgID, userID, roleID, inviterID)
	if err != nil {
		return fmt.Errorf("insert org member: %w", err)
	}

	return tx.Commit(ctx)
}

// UpdateUserRole changes the role of a user within the organisation.
func (s *Service) UpdateUserRole(ctx context.Context, orgID, targetUserID string, input RoleUpdateInput) error {
	var roleID string
	if err := s.db.QueryRow(ctx,
		`SELECT id::text FROM roles WHERE name = $1`, input.Role,
	).Scan(&roleID); err != nil {
		return fmt.Errorf("lookup role %q: %w", input.Role, err)
	}

	tag, err := s.db.Exec(ctx, `
		UPDATE org_members SET role_id = $1::uuid
		WHERE org_id = $2::uuid AND user_id = $3::uuid`,
		roleID, orgID, targetUserID)
	if err != nil {
		return fmt.Errorf("update member role: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("user not found in org")
	}
	return nil
}

// ListModules returns the enabled/disabled state for each known module.
func (s *Service) ListModules() []ModuleStatus {
	known := []string{"secpulse", "secvitals", "secvault", "secreflex", "secprivacy"}
	result := make([]ModuleStatus, 0, len(known))

	for _, name := range known {
		enabled := false
		for _, mod := range splitCSV(s.modulesEnabled) {
			if equalFold(mod, name) {
				enabled = true
				break
			}
		}
		result = append(result, ModuleStatus{Name: name, Enabled: enabled})
	}
	return result
}

func splitCSV(s string) []string {
	var out []string
	for _, part := range splitOn(s, ',') {
		trimmed := trimSpace(part)
		if trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

// Tiny string helpers to avoid importing "strings" at package init.
func splitOn(s string, sep rune) []string {
	var result []string
	start := 0
	for i, r := range s {
		if r == sep {
			result = append(result, s[start:i])
			start = i + 1
		}
	}
	result = append(result, s[start:])
	return result
}

func trimSpace(s string) string {
	for len(s) > 0 && (s[0] == ' ' || s[0] == '\t') {
		s = s[1:]
	}
	for len(s) > 0 && (s[len(s)-1] == ' ' || s[len(s)-1] == '\t') {
		s = s[:len(s)-1]
	}
	return s
}

func equalFold(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		ca, cb := a[i], b[i]
		if ca >= 'A' && ca <= 'Z' {
			ca += 'a' - 'A'
		}
		if cb >= 'A' && cb <= 'Z' {
			cb += 'a' - 'A'
		}
		if ca != cb {
			return false
		}
	}
	return true
}
