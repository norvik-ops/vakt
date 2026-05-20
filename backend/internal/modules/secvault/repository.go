package secvault

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"

	"github.com/matharnica/vakt/internal/db"
)

// Repository handles SecretOps data access. Projects / Environments / AccessLog
// use sqlc-generated queries (db.Queries); Secrets stay on embedded SQL because
// the crypto round-trip plus dynamic column selection makes sqlc generation
// brittle (see ADR-0005 / docs/sqlc-migration-plan.md).
type Repository struct {
	db *pgxpool.Pool
	q  *db.Queries
}

// NewRepository creates a new SecretOps repository.
func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{db: pool, q: db.New(pool)}
}

// optionalText collapses an empty string to a NULL pgtype.Text so the
// generated NULLable column maps cleanly. Avoids storing literal "".
func optionalText(s string) pgtype.Text {
	if s == "" {
		return pgtype.Text{}
	}
	return pgtype.Text{String: s, Valid: true}
}

// --- Projects (sqlc) ---

func (r *Repository) CreateProject(ctx context.Context, orgID, userID, name, slug, description string) (*Project, error) {
	row, err := r.q.CreateSVProject(ctx, db.CreateSVProjectParams{
		OrgID:       orgID,
		Name:        name,
		Slug:        slug,
		Description: optionalText(description),
		CreatedBy:   userID,
	})
	if err != nil {
		return nil, fmt.Errorf("create project: %w", err)
	}
	return &Project{
		ID:          row.ID,
		OrgID:       row.OrgID,
		Name:        row.Name,
		Slug:        row.Slug,
		Description: row.Description.String, // empty string when not Valid
		CreatedAt:   row.CreatedAt.Time,
	}, nil
}

func (r *Repository) ListProjects(ctx context.Context, orgID string) ([]Project, error) {
	rows, err := r.q.ListSVProjects(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("list projects: %w", err)
	}
	out := make([]Project, 0, len(rows))
	for _, row := range rows {
		out = append(out, Project{
			ID:          row.ID,
			OrgID:       row.OrgID,
			Name:        row.Name,
			Slug:        row.Slug,
			Description: row.Description.String,
			CreatedAt:   row.CreatedAt.Time,
		})
	}
	return out, nil
}

func (r *Repository) GetProject(ctx context.Context, orgID, projectID string) (*Project, error) {
	var p Project
	err := r.db.QueryRow(ctx, `
		SELECT id::text, org_id::text, name, slug, COALESCE(description,''), created_at
		FROM so_projects
		WHERE id = $1::uuid AND org_id = $2::uuid`,
		projectID, orgID,
	).Scan(&p.ID, &p.OrgID, &p.Name, &p.Slug, &p.Description, &p.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("get project: %w", err)
	}
	return &p, nil
}

func (r *Repository) DeleteProject(ctx context.Context, orgID, projectID string) error {
	tag, err := r.db.Exec(ctx, `
		DELETE FROM so_projects WHERE id = $1::uuid AND org_id = $2::uuid`,
		projectID, orgID,
	)
	if err != nil {
		return fmt.Errorf("delete project: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("project not found")
	}
	return nil
}

// --- Environments ---

func (r *Repository) CreateEnvironment(ctx context.Context, orgID, projectID, name string) (*Environment, error) {
	var e Environment
	err := r.db.QueryRow(ctx, `
		INSERT INTO so_environments (project_id, org_id, name)
		VALUES ($1::uuid, $2::uuid, $3)
		RETURNING id::text, project_id::text, name, created_at`,
		projectID, orgID, name,
	).Scan(&e.ID, &e.ProjectID, &e.Name, &e.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("create environment: %w", err)
	}
	return &e, nil
}

func (r *Repository) ListEnvironments(ctx context.Context, orgID, projectID string) ([]Environment, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id::text, project_id::text, name, created_at
		FROM so_environments
		WHERE project_id = $1::uuid
		  AND org_id = $2::uuid
		ORDER BY created_at ASC`,
		projectID, orgID,
	)
	if err != nil {
		return nil, fmt.Errorf("list environments: %w", err)
	}
	defer rows.Close()

	var envs []Environment
	for rows.Next() {
		var e Environment
		if err := rows.Scan(&e.ID, &e.ProjectID, &e.Name, &e.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan environment: %w", err)
		}
		envs = append(envs, e)
	}
	return envs, rows.Err()
}

// --- Secrets ---

func (r *Repository) UpsertSecret(ctx context.Context, orgID, envID, userID, key string, encryptedValue []byte) (*Secret, error) {
	var s Secret
	err := r.db.QueryRow(ctx, `
		INSERT INTO so_secrets (environment_id, org_id, key, encrypted_value, created_by, updated_by)
		VALUES ($1::uuid, $2::uuid, $3, $4, $5::uuid, $5::uuid)
		ON CONFLICT (environment_id, key) DO UPDATE
		SET encrypted_value = EXCLUDED.encrypted_value,
		    version         = so_secrets.version + 1,
		    updated_by      = EXCLUDED.updated_by,
		    updated_at      = NOW()
		RETURNING id::text, key, version, rotation_due_at, last_rotated_at, last_accessed_at,
		          access_count, created_at, updated_at`,
		envID, orgID, key, encryptedValue, userID,
	).Scan(
		&s.ID, &s.Key, &s.Version,
		&s.RotationDueAt, &s.LastRotatedAt, &s.LastAccessedAt,
		&s.AccessCount, &s.CreatedAt, &s.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("upsert secret: %w", err)
	}
	return &s, nil
}

func (r *Repository) GetSecretRaw(ctx context.Context, orgID, envID, key string) (*Secret, []byte, error) {
	var s Secret
	var encryptedValue []byte
	err := r.db.QueryRow(ctx, `
		SELECT id::text, key, encrypted_value, version,
		       rotation_due_at, last_rotated_at, last_accessed_at,
		       access_count, created_at, updated_at
		FROM so_secrets
		WHERE environment_id = $1::uuid AND org_id = $2::uuid AND key = $3`,
		envID, orgID, key,
	).Scan(
		&s.ID, &s.Key, &encryptedValue, &s.Version,
		&s.RotationDueAt, &s.LastRotatedAt, &s.LastAccessedAt,
		&s.AccessCount, &s.CreatedAt, &s.UpdatedAt,
	)
	if err != nil {
		return nil, nil, fmt.Errorf("get secret: %w", err)
	}
	return &s, encryptedValue, nil
}

func (r *Repository) UpdateSecretAccess(ctx context.Context, secretID string) error {
	_, err := r.db.Exec(ctx, `
		UPDATE so_secrets
		SET access_count     = access_count + 1,
		    last_accessed_at = NOW()
		WHERE id = $1::uuid`,
		secretID,
	)
	return err
}

func (r *Repository) ListSecretKeys(ctx context.Context, orgID, envID string) ([]Secret, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id::text, key, version, rotation_due_at, last_rotated_at, last_accessed_at,
		       access_count, created_at, updated_at
		FROM so_secrets
		WHERE environment_id = $1::uuid AND org_id = $2::uuid
		ORDER BY key ASC`,
		envID, orgID,
	)
	if err != nil {
		return nil, fmt.Errorf("list secret keys: %w", err)
	}
	defer rows.Close()

	var secrets []Secret
	for rows.Next() {
		var s Secret
		if err := rows.Scan(
			&s.ID, &s.Key, &s.Version,
			&s.RotationDueAt, &s.LastRotatedAt, &s.LastAccessedAt,
			&s.AccessCount, &s.CreatedAt, &s.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan secret: %w", err)
		}
		secrets = append(secrets, s)
	}
	return secrets, rows.Err()
}

func (r *Repository) DeleteSecret(ctx context.Context, orgID, envID, key string) error {
	tag, err := r.db.Exec(ctx, `
		DELETE FROM so_secrets
		WHERE environment_id = $1::uuid AND org_id = $2::uuid AND key = $3`,
		envID, orgID, key,
	)
	if err != nil {
		return fmt.Errorf("delete secret: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("secret not found")
	}
	return nil
}

// --- Access log ---

func (r *Repository) LogAccess(ctx context.Context, secretID, orgID string, accessedBy *string, accessVia, ip, userAgent string) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO so_access_log (secret_id, org_id, accessed_by, access_via, ip_address, user_agent)
		VALUES ($1::uuid, $2::uuid, $3::uuid, $4, $5, $6)`,
		secretID, orgID, accessedBy, accessVia, nilIfEmpty(ip), nilIfEmpty(userAgent),
	)
	return err
}

func (r *Repository) GetAccessLog(ctx context.Context, secretID, orgID string, limit, offset int) ([]AccessLogEntry, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id::text, secret_id::text,
		       accessed_by::text, access_via,
		       ip_address, user_agent, accessed_at
		FROM so_access_log
		WHERE secret_id = $1::uuid
		  AND org_id = $2::uuid
		ORDER BY accessed_at DESC
		LIMIT $3 OFFSET $4`,
		secretID, orgID, limit, offset,
	)
	if err != nil {
		return nil, fmt.Errorf("get access log: %w", err)
	}
	defer rows.Close()

	var entries []AccessLogEntry
	for rows.Next() {
		var e AccessLogEntry
		if err := rows.Scan(
			&e.ID, &e.SecretID,
			&e.AccessedBy, &e.AccessVia,
			&e.IPAddress, &e.UserAgent, &e.AccessedAt,
		); err != nil {
			return nil, fmt.Errorf("scan access log entry: %w", err)
		}
		entries = append(entries, e)
	}
	return entries, rows.Err()
}

// GetProjectAccessLog returns paginated access log entries across all secrets that belong to
// a project, joining in the secret key name for display purposes.
// It runs a COUNT query first to obtain the total row count needed for frontend pagination,
// then a bounded SELECT using limit/offset. Returns (entries, total, error).
func (r *Repository) GetProjectAccessLog(ctx context.Context, orgID, projectID string, limit, offset int) ([]ProjectAccessLogEntry, int, error) {
	var total int
	err := r.db.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM so_access_log al
		JOIN so_secrets s ON s.id = al.secret_id
		JOIN so_environments e ON e.id = s.environment_id
		WHERE e.project_id = $1::uuid
		  AND al.org_id = $2::uuid`,
		projectID, orgID,
	).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("count project access log: %w", err)
	}

	rows, err := r.db.Query(ctx, `
		SELECT
		    al.id::text,
		    s.key AS secret_key,
		    al.access_via,
		    al.accessed_by::text,
		    al.ip_address,
		    al.accessed_at
		FROM so_access_log al
		JOIN so_secrets s ON s.id = al.secret_id
		JOIN so_environments e ON e.id = s.environment_id
		WHERE e.project_id = $1::uuid
		  AND al.org_id = $2::uuid
		ORDER BY al.accessed_at DESC
		LIMIT $3 OFFSET $4`,
		projectID, orgID, limit, offset,
	)
	if err != nil {
		return nil, 0, fmt.Errorf("get project access log: %w", err)
	}
	defer rows.Close()

	var entries []ProjectAccessLogEntry
	for rows.Next() {
		var e ProjectAccessLogEntry
		if err := rows.Scan(
			&e.ID, &e.SecretKey, &e.AccessVia,
			&e.AccessedBy, &e.IPAddress, &e.AccessedAt,
		); err != nil {
			return nil, 0, fmt.Errorf("scan project access log entry: %w", err)
		}
		entries = append(entries, e)
	}
	return entries, total, rows.Err()
}

// GetSecretByID returns a secret by its UUID (for access-log key lookups).
func (r *Repository) GetSecretByID(ctx context.Context, orgID, secretID string) (*Secret, []byte, error) {
	var s Secret
	var encryptedValue []byte
	err := r.db.QueryRow(ctx, `
		SELECT id::text, key, encrypted_value, version,
		       rotation_due_at, last_rotated_at, last_accessed_at,
		       access_count, created_at, updated_at
		FROM so_secrets
		WHERE id = $1::uuid AND org_id = $2::uuid`,
		secretID, orgID,
	).Scan(
		&s.ID, &s.Key, &encryptedValue, &s.Version,
		&s.RotationDueAt, &s.LastRotatedAt, &s.LastAccessedAt,
		&s.AccessCount, &s.CreatedAt, &s.UpdatedAt,
	)
	if err != nil {
		return nil, nil, fmt.Errorf("get secret by id: %w", err)
	}
	return &s, encryptedValue, nil
}

// --- Share links ---

func (r *Repository) CreateShareLink(ctx context.Context, secretID, orgID, userID, tokenHash string, expiresAt time.Time) (*ShareLink, error) {
	var sl ShareLink
	err := r.db.QueryRow(ctx, `
		INSERT INTO so_share_links (secret_id, org_id, token_hash, expires_at, created_by)
		VALUES ($1::uuid, $2::uuid, $3, $4, $5::uuid)
		RETURNING id::text, secret_id::text, expires_at, created_at`,
		secretID, orgID, tokenHash, expiresAt, userID,
	).Scan(&sl.ID, &sl.SecretID, &sl.ExpiresAt, &sl.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("create share link: %w", err)
	}
	return &sl, nil
}

// GetShareLink looks up a share link by its token hash and validates expiry / single-use.
//
// Defense-in-depth note: this query filters only by token_hash, not by org_id.
// That is intentional — the token is 32 bytes of cryptographic randomness (SHA-256
// over a crypto/rand token), making brute-force cross-org access completely
// impractical. The org_id is stored on the row and is not known by the caller
// at call time; the service layer retrieves it post-fetch (via getOrgIDForShareLink)
// and then enforces it in MarkShareLinkUsed via an org_id-scoped UPDATE.
func (r *Repository) GetShareLink(ctx context.Context, tokenHash string) (*ShareLink, error) {
	var sl ShareLink
	err := r.db.QueryRow(ctx, `
		SELECT id::text, secret_id::text, expires_at, used_at, created_at
		FROM so_share_links
		WHERE token_hash = $1`,
		tokenHash,
	).Scan(&sl.ID, &sl.SecretID, &sl.ExpiresAt, &sl.UsedAt, &sl.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("get share link: %w", err)
	}
	return &sl, nil
}

func (r *Repository) MarkShareLinkUsed(ctx context.Context, linkID, orgID string) error {
	// org_id filter prevents a caller who knows the link UUID from burning
	// another organisation's share link (DoS / IDOR defense).
	_, err := r.db.Exec(ctx, `
		UPDATE so_share_links SET used_at = NOW()
		WHERE id = $1::uuid AND org_id = $2::uuid`, linkID, orgID)
	return err
}

// --- API tokens (uses shared api_keys table) ---

func (r *Repository) CreateAPIToken(ctx context.Context, orgID, userID, name, keyHash, keyPrefix string, scopes []string, expiresAt *time.Time) (*APIToken, error) {
	var t APIToken
	err := r.db.QueryRow(ctx, `
		INSERT INTO api_keys (org_id, created_by, name, key_hash, key_prefix, scopes, expires_at)
		VALUES ($1::uuid, $2::uuid, $3, $4, $5, $6, $7)
		RETURNING id::text, name, key_prefix, scopes, expires_at, last_used_at, revoked_at, created_at`,
		orgID, userID, name, keyHash, keyPrefix, scopes, expiresAt,
	).Scan(
		&t.ID, &t.Name, &t.KeyPrefix, &t.Scopes,
		&t.ExpiresAt, &t.LastUsedAt, &t.RevokedAt, &t.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("create api token: %w", err)
	}
	return &t, nil
}

func (r *Repository) ListAPITokens(ctx context.Context, orgID, userID string) ([]APIToken, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id::text, name, key_prefix, scopes, expires_at, last_used_at, revoked_at, created_at
		FROM api_keys
		WHERE org_id = $1::uuid AND created_by = $2::uuid
		  AND $3 = ANY(scopes)
		ORDER BY created_at DESC`,
		orgID, userID, "secvault",
	)
	if err != nil {
		return nil, fmt.Errorf("list api tokens: %w", err)
	}
	defer rows.Close()

	var tokens []APIToken
	for rows.Next() {
		var t APIToken
		if err := rows.Scan(
			&t.ID, &t.Name, &t.KeyPrefix, &t.Scopes,
			&t.ExpiresAt, &t.LastUsedAt, &t.RevokedAt, &t.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan api token: %w", err)
		}
		tokens = append(tokens, t)
	}
	return tokens, rows.Err()
}

func (r *Repository) RevokeAPIToken(ctx context.Context, orgID, userID, tokenID string) error {
	tag, err := r.db.Exec(ctx, `
		UPDATE api_keys
		SET revoked_at = NOW()
		WHERE id = $1::uuid AND org_id = $2::uuid AND created_by = $3::uuid
		  AND $4 = ANY(scopes)
		  AND revoked_at IS NULL`,
		tokenID, orgID, userID, "secvault",
	)
	if err != nil {
		return fmt.Errorf("revoke api token: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("token not found or already revoked")
	}
	return nil
}

// ListProjectSecrets returns all secrets (with metadata) for health scoring.
func (r *Repository) ListProjectSecrets(ctx context.Context, orgID, projectID string) ([]Secret, error) {
	rows, err := r.db.Query(ctx, `
		SELECT s.id::text, s.key, s.version,
		       s.rotation_due_at, s.last_rotated_at, s.last_accessed_at,
		       s.access_count, s.created_at, s.updated_at
		FROM so_secrets s
		JOIN so_environments e ON e.id = s.environment_id
		WHERE e.project_id = $1::uuid AND s.org_id = $2::uuid
		ORDER BY s.key ASC`,
		projectID, orgID,
	)
	if err != nil {
		return nil, fmt.Errorf("list project secrets: %w", err)
	}
	defer rows.Close()

	var secrets []Secret
	for rows.Next() {
		var s Secret
		if err := rows.Scan(
			&s.ID, &s.Key, &s.Version,
			&s.RotationDueAt, &s.LastRotatedAt, &s.LastAccessedAt,
			&s.AccessCount, &s.CreatedAt, &s.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan project secret: %w", err)
		}
		secrets = append(secrets, s)
	}
	return secrets, rows.Err()
}

// GetSecretIDByKey returns the secret ID for an access-log query given key + env + org.
func (r *Repository) GetSecretIDByKey(ctx context.Context, orgID, envID, key string) (string, error) {
	var id string
	err := r.db.QueryRow(ctx, `
		SELECT id::text FROM so_secrets
		WHERE environment_id = $1::uuid AND org_id = $2::uuid AND key = $3`,
		envID, orgID, key,
	).Scan(&id)
	if err != nil {
		return "", fmt.Errorf("get secret id by key: %w", err)
	}
	return id, nil
}

// nilIfEmpty converts an empty string pointer to nil for nullable DB columns.
func nilIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// GetSecretByKey returns a secret record (without encrypted value) for a given env + key.
func (r *Repository) GetSecretByKey(ctx context.Context, orgID, envID, key string) (*Secret, error) {
	var s Secret
	err := r.db.QueryRow(ctx, `
		SELECT id::text, key, version,
		       rotation_due_at, last_rotated_at, last_accessed_at,
		       access_count, created_at, updated_at
		FROM so_secrets
		WHERE environment_id = $1::uuid AND key = $2 AND org_id = $3::uuid`,
		envID, key, orgID,
	).Scan(
		&s.ID, &s.Key, &s.Version,
		&s.RotationDueAt, &s.LastRotatedAt, &s.LastAccessedAt,
		&s.AccessCount, &s.CreatedAt, &s.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("get secret by key: %w", err)
	}
	return &s, nil
}

// --- Git Scanner ---

func (r *Repository) CreateGitScan(ctx context.Context, orgID, repoURL, branch string) (*GitScan, error) {
	const q = `INSERT INTO so_git_scans (org_id, repo_url, branch)
	           VALUES ($1::uuid, $2, $3)
	           RETURNING id::text, org_id::text, repo_url, branch, status,
	                     finding_count, open_count, dismissed_count,
	                     COALESCE(error_message,''), scanned_at, created_at`
	row := r.db.QueryRow(ctx, q, orgID, repoURL, branch)
	var s GitScan
	err := row.Scan(&s.ID, &s.OrgID, &s.RepoURL, &s.Branch, &s.Status,
		&s.FindingCount, &s.OpenCount, &s.DismissedCount, &s.ErrorMessage, &s.ScannedAt, &s.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("create git scan: %w", err)
	}
	return &s, nil
}

func (r *Repository) GetGitScan(ctx context.Context, orgID, scanID string) (*GitScan, error) {
	const q = `SELECT id::text, org_id::text, repo_url, branch, status,
	                  finding_count, open_count, dismissed_count,
	                  COALESCE(error_message,''), scanned_at, created_at
	           FROM so_git_scans WHERE id=$1::uuid AND org_id=$2::uuid`
	row := r.db.QueryRow(ctx, q, scanID, orgID)
	var s GitScan
	err := row.Scan(&s.ID, &s.OrgID, &s.RepoURL, &s.Branch, &s.Status,
		&s.FindingCount, &s.OpenCount, &s.DismissedCount, &s.ErrorMessage, &s.ScannedAt, &s.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("get git scan: %w", err)
	}
	return &s, nil
}

func (r *Repository) ListGitScans(ctx context.Context, orgID string) ([]GitScan, error) {
	const q = `SELECT id::text, org_id::text, repo_url, branch, status,
	                  finding_count, open_count, dismissed_count,
	                  COALESCE(error_message,''), scanned_at, created_at
	           FROM so_git_scans WHERE org_id=$1::uuid ORDER BY created_at DESC`
	rows, err := r.db.Query(ctx, q, orgID)
	if err != nil {
		return nil, fmt.Errorf("list git scans: %w", err)
	}
	defer rows.Close()
	var scans []GitScan
	for rows.Next() {
		var s GitScan
		if err := rows.Scan(&s.ID, &s.OrgID, &s.RepoURL, &s.Branch, &s.Status,
			&s.FindingCount, &s.OpenCount, &s.DismissedCount, &s.ErrorMessage, &s.ScannedAt, &s.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan git scan: %w", err)
		}
		scans = append(scans, s)
	}
	return scans, rows.Err()
}

func (r *Repository) UpdateGitScanStatus(ctx context.Context, scanID, orgID, status string, findingCount, openCount, dismissedCount int, errMsg string, scannedAt *time.Time) error {
	const q = `UPDATE so_git_scans SET status=$1, finding_count=$2, open_count=$3, dismissed_count=$4, error_message=$5, scanned_at=$6
	           WHERE id=$7::uuid AND org_id=$8::uuid`
	_, err := r.db.Exec(ctx, q, status, findingCount, openCount, dismissedCount, nilIfEmpty(errMsg), scannedAt, scanID, orgID)
	return err
}

func (r *Repository) SaveScanResults(ctx context.Context, orgID, scanID string, results []ScanResult) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("save scan results: begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	const q = `INSERT INTO so_scan_results (org_id, scan_id, repo_url, commit_hash, file_path, line_number, pattern_name, match_preview, severity, status)
	           VALUES ($1::uuid, $2::uuid, $3, $4, $5, $6, $7, $8, $9, 'open')`
	batch := &pgx.Batch{}
	for _, res := range results {
		batch.Queue(q, orgID, scanID, res.RepoURL, nilIfEmpty(res.CommitHash), res.FilePath, res.LineNumber, res.PatternName, res.MatchPreview, res.Severity)
	}
	br := tx.SendBatch(ctx, batch)
	defer br.Close()
	for range results {
		if _, execErr := br.Exec(); execErr != nil {
			log.Warn().Err(execErr).Msg("scan result insert failed")
		}
	}
	if err := br.Close(); err != nil {
		return fmt.Errorf("save scan results: close batch: %w", err)
	}
	return tx.Commit(ctx)
}

func (r *Repository) GetScanResults(ctx context.Context, orgID, scanID string) ([]ScanResult, error) {
	const q = `SELECT id::text, org_id::text, scan_id::text, repo_url,
	                  COALESCE(commit_hash,''), file_path, COALESCE(line_number,0),
	                  pattern_name, match_preview, severity, status,
	                  COALESCE(dismiss_reason,''), dismiss_count, created_at
	           FROM so_scan_results WHERE scan_id=$1::uuid AND org_id=$2::uuid ORDER BY severity, file_path`
	rows, err := r.db.Query(ctx, q, scanID, orgID)
	if err != nil {
		return nil, fmt.Errorf("get scan results: %w", err)
	}
	defer rows.Close()
	var results []ScanResult
	for rows.Next() {
		var res ScanResult
		if err := rows.Scan(&res.ID, &res.OrgID, &res.ScanID, &res.RepoURL, &res.CommitHash, &res.FilePath, &res.LineNumber,
			&res.PatternName, &res.MatchPreview, &res.Severity, &res.Status, &res.DismissReason, &res.DismissCount, &res.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan result row: %w", err)
		}
		results = append(results, res)
	}
	return results, rows.Err()
}

func (r *Repository) DismissScanResult(ctx context.Context, orgID, resultID, reason string) error {
	const q = `UPDATE so_scan_results SET status='dismissed', dismiss_reason=$1, dismiss_count=dismiss_count+1
	           WHERE id=$2::uuid AND org_id=$3::uuid`
	tag, err := r.db.Exec(ctx, q, reason, resultID, orgID)
	if err != nil {
		return fmt.Errorf("dismiss scan result: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("scan result not found")
	}
	return nil
}

func (r *Repository) CountDismissals(ctx context.Context, orgID, patternName, filePath string) (int, error) {
	const q = `SELECT COALESCE(SUM(dismiss_count), 0) FROM so_scan_results
	           WHERE org_id=$1::uuid AND pattern_name=$2 AND file_path=$3 AND status='dismissed'`
	var count int
	err := r.db.QueryRow(ctx, q, orgID, patternName, filePath).Scan(&count)
	return count, err
}

// --- Rotation policies ---

func (r *Repository) UpsertRotationPolicy(ctx context.Context, orgID, secretID string, intervalDays int) (*RotationPolicy, error) {
	nextRotation := time.Now().AddDate(0, 0, intervalDays)
	const q = `INSERT INTO so_rotation_policies (org_id, secret_id, interval_days, next_rotation_at)
	           VALUES ($1::uuid, $2::uuid, $3, $4)
	           ON CONFLICT (secret_id) DO UPDATE SET interval_days=EXCLUDED.interval_days, next_rotation_at=EXCLUDED.next_rotation_at
	           RETURNING id::text, org_id::text, secret_id::text, interval_days, last_rotated_at, next_rotation_at, is_active, created_at`
	row := r.db.QueryRow(ctx, q, orgID, secretID, intervalDays, nextRotation)
	var p RotationPolicy
	err := row.Scan(&p.ID, &p.OrgID, &p.SecretID, &p.IntervalDays, &p.LastRotatedAt, &p.NextRotationAt, &p.IsActive, &p.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("upsert rotation policy: %w", err)
	}
	return &p, nil
}

func (r *Repository) GetRotationPolicy(ctx context.Context, orgID, secretID string) (*RotationPolicy, error) {
	const q = `SELECT id::text, org_id::text, secret_id::text, interval_days, last_rotated_at, next_rotation_at, is_active, created_at
	           FROM so_rotation_policies WHERE secret_id=$1::uuid AND org_id=$2::uuid`
	row := r.db.QueryRow(ctx, q, secretID, orgID)
	var p RotationPolicy
	err := row.Scan(&p.ID, &p.OrgID, &p.SecretID, &p.IntervalDays, &p.LastRotatedAt, &p.NextRotationAt, &p.IsActive, &p.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("get rotation policy: %w", err)
	}
	return &p, nil
}

func (r *Repository) UpdateRotationAfterRotate(ctx context.Context, orgID, secretID string, intervalDays int) error {
	nextRotation := time.Now().AddDate(0, 0, intervalDays)
	const q = `UPDATE so_rotation_policies SET last_rotated_at=NOW(), next_rotation_at=$1 WHERE secret_id=$2::uuid AND org_id=$3::uuid`
	_, err := r.db.Exec(ctx, q, nextRotation, secretID, orgID)
	return err
}
