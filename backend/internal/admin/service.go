package admin

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"

	"github.com/matharnica/vakt/internal/shared/crypto"
	"github.com/matharnica/vakt/internal/shared/notify"
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

// InviteInput is the request body for POST /admin/users/invite.
type InviteInput struct {
	Email string `json:"email" validate:"required,email"`
	Role  string `json:"role"  validate:"required,oneof=Admin SecurityAnalyst Viewer AuditorReadOnly"`
}

// CreateUserInput is the request body for POST /admin/users (direct creation, no SMTP needed).
type CreateUserInput struct {
	Email    string `json:"email"    validate:"required,email"`
	Password string `json:"password" validate:"required,min=10"`
	Role     string `json:"role"     validate:"required,oneof=Admin SecurityAnalyst Viewer AuditorReadOnly"`
}

// CreateUserResult carries the new user's ID and the one-time plaintext password.
type CreateUserResult struct {
	UserID string `json:"user_id"`
}

// OIDCConfigInput is the request body for PUT /admin/org/oidc-config.
type OIDCConfigInput struct {
	ProviderURL  string `json:"provider_url"  validate:"required,url"`
	ClientID     string `json:"client_id"     validate:"required"`
	ClientSecret string `json:"client_secret" validate:"required"`
	Enabled      bool   `json:"enabled"`
}

// ModuleStatus describes the enabled/disabled state of a platform module.
type ModuleStatus struct {
	Name    string `json:"name"`
	Enabled bool   `json:"enabled"`
}

// Service implements admin business logic.
type Service struct {
	db             *pgxpool.Pool
	repo           *Repository
	modulesEnabled string
	notifySvc      *notify.Service
	masterKey      []byte
}

// NewService constructs an admin Service.
func NewService(db *pgxpool.Pool, modulesEnabled string) *Service {
	return &Service{
		db:             db,
		repo:           NewRepository(db),
		modulesEnabled: modulesEnabled,
	}
}

// WithMasterKey attaches the encryption master key to the Service (for OIDC secret encryption).
func (s *Service) WithMasterKey(key []byte) *Service {
	s.masterKey = key
	return s
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

	// Fixed-placeholder query: optional filters use IS NULL short-circuit so no
	// dynamic SQL or fmt.Sprintf is needed. NULL means "no filter on this column".
	var nullUserID, nullAction, nullResourceType *string
	if userID != "" {
		nullUserID = &userID
	}
	if action != "" {
		nullAction = &action
	}
	if resourceType != "" {
		nullResourceType = &resourceType
	}

	// Query audit_log directly (not the audit_logs VIEW) so that created_at is
	// available as a real column.
	// S121-C2 (P2): $3/$4 are optional *string filters passed as nil when the
	// caller omits them. Without an explicit ::text cast, Postgres cannot infer
	// the parameter type of a bare nil ($3) and rejects the query with 42P08
	// ("could not determine data type of parameter"), so the default audit-log
	// view (no filters) returned 500. The ::text casts make the type explicit.
	var total int
	if err := s.db.QueryRow(ctx, `
		SELECT COUNT(*) FROM audit_log
		WHERE org_id = $1::uuid
		  AND deleted_at IS NULL
		  AND ($2::uuid IS NULL OR user_id = $2::uuid)
		  AND ($3::text IS NULL OR action = $3::text)
		  AND ($4::text IS NULL OR resource_type = $4::text)`,
		orgID, nullUserID, nullAction, nullResourceType,
	).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count audit logs: %w", err)
	}

	rows, err := s.db.Query(ctx, `
		SELECT id::text, org_id::text, user_id::text, action, resource_type,
		       resource_id, ip_address, created_at
		FROM audit_log
		WHERE org_id = $1::uuid
		  AND deleted_at IS NULL
		  AND ($2::uuid IS NULL OR user_id = $2::uuid)
		  AND ($3::text IS NULL OR action = $3::text)
		  AND ($4::text IS NULL OR resource_type = $4::text)
		ORDER BY created_at DESC
		LIMIT $5 OFFSET $6`,
		orgID, nullUserID, nullAction, nullResourceType, limit, offset)
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

// CreateUser directly creates an active user in the org without requiring SMTP.
// The caller receives the userID; password is already supplied by the admin.
func (s *Service) CreateUser(ctx context.Context, orgID, createdByID string, input CreateUserInput) (*CreateUserResult, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(input.Password), 12)
	if err != nil {
		return nil, fmt.Errorf("create user: hash password: %w", err)
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("create user: begin tx: %w", err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback(ctx)
		}
	}()

	var userID string
	err = tx.QueryRow(ctx, `
		INSERT INTO users (email, password_hash, is_active)
		VALUES ($1, $2, TRUE)
		RETURNING id::text`,
		input.Email, string(hash),
	).Scan(&userID)
	if err != nil {
		if strings.Contains(err.Error(), "unique") || strings.Contains(err.Error(), "duplicate") {
			return nil, fmt.Errorf("create user: email already exists")
		}
		return nil, fmt.Errorf("create user: insert user: %w", err)
	}

	var roleID string
	err = tx.QueryRow(ctx, `SELECT id::text FROM roles WHERE name = $1`, input.Role).Scan(&roleID)
	if err != nil {
		return nil, fmt.Errorf("create user: lookup role %q: %w", input.Role, err)
	}

	var inviterID *string
	if createdByID != "" {
		inviterID = &createdByID
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO org_members (org_id, user_id, role_id, invited_by)
		VALUES ($1::uuid, $2::uuid, $3::uuid, $4::uuid)
		ON CONFLICT (org_id, user_id) DO NOTHING`,
		orgID, userID, roleID, inviterID)
	if err != nil {
		return nil, fmt.Errorf("create user: insert org member: %w", err)
	}

	if err = tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("create user: commit: %w", err)
	}
	return &CreateUserResult{UserID: userID}, nil
}

// GetOIDCConfig returns the OIDC config for the org (secret not included).
func (s *Service) GetOIDCConfig(ctx context.Context, orgID string) (*OrgOIDCConfig, error) {
	return s.repo.GetOrgOIDCConfig(ctx, orgID)
}

// UpsertOIDCConfig stores or updates the OIDC configuration.
// The client_secret is encrypted before storage.
func (s *Service) UpsertOIDCConfig(ctx context.Context, orgID string, input OIDCConfigInput) error {
	if len(s.masterKey) == 0 {
		return fmt.Errorf("oidc config: VAKT_SECRET_KEY not configured; refusing to store client_secret without encryption")
	}
	ct, err := crypto.Encrypt(s.masterKey, []byte(input.ClientSecret))
	if err != nil {
		return fmt.Errorf("oidc config: encrypt secret: %w", err)
	}
	secretEnc := []byte("enc:v1:" + base64.URLEncoding.EncodeToString(ct))
	return s.repo.UpsertOrgOIDCConfig(ctx, orgID, input.ProviderURL, input.ClientID, secretEnc, input.Enabled)
}

// DisableOIDCConfig disables OIDC for the org without deleting the config.
func (s *Service) DisableOIDCConfig(ctx context.Context, orgID string) error {
	return s.repo.DisableOrgOIDCConfig(ctx, orgID)
}

// ListModules returns the enabled/disabled state for each known module.
func (s *Service) ListModules() []ModuleStatus {
	known := []string{"vaktscan", "vaktcomply", "vaktvault", "vaktaware", "vaktprivacy"}
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
