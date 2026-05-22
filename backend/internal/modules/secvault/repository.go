package secvault

import (
	"context"
	"fmt"
	"strconv"
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
	row, err := r.q.GetSVProject(ctx, db.GetSVProjectParams{ID: projectID, OrgID: orgID})
	if err != nil {
		return nil, fmt.Errorf("get project: %w", err)
	}
	return &Project{
		ID:          row.ID,
		OrgID:       row.OrgID,
		Name:        row.Name,
		Slug:        row.Slug,
		Description: row.Description.String,
		CreatedAt:   row.CreatedAt.Time,
	}, nil
}

func (r *Repository) DeleteProject(ctx context.Context, orgID, projectID string) error {
	n, err := r.q.DeleteSVProject(ctx, db.DeleteSVProjectParams{ID: projectID, OrgID: orgID})
	if err != nil {
		return fmt.Errorf("delete project: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("project not found")
	}
	return nil
}

// --- Environments ---

func (r *Repository) CreateEnvironment(ctx context.Context, orgID, projectID, name string) (*Environment, error) {
	row, err := r.q.CreateSVEnvironment(ctx, db.CreateSVEnvironmentParams{
		ProjectID: projectID,
		OrgID:     orgID,
		Name:      name,
	})
	if err != nil {
		return nil, fmt.Errorf("create environment: %w", err)
	}
	return &Environment{
		ID:        row.ID,
		ProjectID: row.ProjectID,
		Name:      row.Name,
		CreatedAt: row.CreatedAt.Time,
	}, nil
}

func (r *Repository) ListEnvironments(ctx context.Context, orgID, projectID string) ([]Environment, error) {
	rows, err := r.q.ListSVEnvironments(ctx, db.ListSVEnvironmentsParams{
		ProjectID: projectID,
		OrgID:     orgID,
	})
	if err != nil {
		return nil, fmt.Errorf("list environments: %w", err)
	}
	out := make([]Environment, 0, len(rows))
	for _, row := range rows {
		out = append(out, Environment{
			ID:        row.ID,
			ProjectID: row.ProjectID,
			Name:      row.Name,
			CreatedAt: row.CreatedAt.Time,
		})
	}
	return out, nil
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
	return r.q.UpdateSVSecretAccess(ctx, secretID)
}

func (r *Repository) ListSecretKeys(ctx context.Context, orgID, envID string) ([]Secret, error) {
	rows, err := r.q.ListSVSecretKeys(ctx, db.ListSVSecretKeysParams{
		EnvironmentID: envID,
		OrgID:         orgID,
	})
	if err != nil {
		return nil, fmt.Errorf("list secret keys: %w", err)
	}
	return mapSecretKeyRows(rows), nil
}

func (r *Repository) DeleteSecret(ctx context.Context, orgID, envID, key string) error {
	n, err := r.q.DeleteSVSecret(ctx, db.DeleteSVSecretParams{
		EnvironmentID: envID,
		OrgID:         orgID,
		Key:           key,
	})
	if err != nil {
		return fmt.Errorf("delete secret: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("secret not found")
	}
	return nil
}

// --- Access log ---

func (r *Repository) LogAccess(ctx context.Context, secretID, orgID string, accessedBy *string, accessVia, ip, userAgent string) error {
	var byUUID pgtype.UUID
	if accessedBy != nil && *accessedBy != "" {
		if err := byUUID.Scan(*accessedBy); err != nil {
			return fmt.Errorf("parse accessed_by uuid: %w", err)
		}
	}
	return r.q.InsertSVAccessLog(ctx, db.InsertSVAccessLogParams{
		SecretID:   secretID,
		OrgID:      orgID,
		AccessedBy: byUUID,
		AccessVia:  accessVia,
		IpAddress:  optionalText(ip),
		UserAgent:  optionalText(userAgent),
	})
}

func (r *Repository) GetAccessLog(ctx context.Context, secretID, orgID string, limit, offset int) ([]AccessLogEntry, error) {
	rows, err := r.q.GetSVAccessLog(ctx, db.GetSVAccessLogParams{
		SecretID: secretID,
		OrgID:    orgID,
		Limit:    int32(limit),
		Offset:   int32(offset),
	})
	if err != nil {
		return nil, fmt.Errorf("get access log: %w", err)
	}
	out := make([]AccessLogEntry, 0, len(rows))
	for _, row := range rows {
		e := AccessLogEntry{
			ID:         row.ID,
			SecretID:   row.SecretID,
			AccessVia:  row.AccessVia,
			AccessedAt: row.AccessedAt.Time,
		}
		if row.AccessedBy.Valid {
			s := row.AccessedBy.String
			e.AccessedBy = &s
		}
		if row.IpAddress.Valid {
			s := row.IpAddress.String
			e.IPAddress = &s
		}
		if row.UserAgent.Valid {
			s := row.UserAgent.String
			e.UserAgent = &s
		}
		out = append(out, e)
	}
	return out, nil
}

// GetProjectAccessLog returns paginated access log entries across all secrets that belong to
// a project, joining in the secret key name for display purposes.
// It runs a COUNT query first to obtain the total row count needed for frontend pagination,
// then a bounded SELECT using limit/offset. Returns (entries, total, error).
func (r *Repository) GetProjectAccessLog(ctx context.Context, orgID, projectID string, limit, offset int) ([]ProjectAccessLogEntry, int, error) {
	total, err := r.q.CountSVProjectAccessLog(ctx, db.CountSVProjectAccessLogParams{
		ProjectID: projectID,
		OrgID:     orgID,
	})
	if err != nil {
		return nil, 0, fmt.Errorf("count project access log: %w", err)
	}

	rows, err := r.q.ListSVProjectAccessLog(ctx, db.ListSVProjectAccessLogParams{
		ProjectID: projectID,
		OrgID:     orgID,
		Limit:     int32(limit),
		Offset:    int32(offset),
	})
	if err != nil {
		return nil, 0, fmt.Errorf("get project access log: %w", err)
	}
	out := make([]ProjectAccessLogEntry, 0, len(rows))
	for _, row := range rows {
		e := ProjectAccessLogEntry{
			ID:         row.ID,
			SecretKey:  row.SecretKey,
			AccessVia:  row.AccessVia,
			AccessedAt: row.AccessedAt.Time,
		}
		if row.AccessedBy.Valid {
			s := row.AccessedBy.String
			e.AccessedBy = &s
		}
		if row.IpAddress.Valid {
			s := row.IpAddress.String
			e.IPAddress = &s
		}
		out = append(out, e)
	}
	return out, int(total), nil
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
	row, err := r.q.CreateSVShareLink(ctx, db.CreateSVShareLinkParams{
		SecretID:  secretID,
		OrgID:     orgID,
		TokenHash: tokenHash,
		ExpiresAt: expiresAt,
		CreatedBy: userID,
	})
	if err != nil {
		return nil, fmt.Errorf("create share link: %w", err)
	}
	return &ShareLink{
		ID:        row.ID,
		SecretID:  row.SecretID,
		ExpiresAt: row.ExpiresAt.Time,
		CreatedAt: row.CreatedAt.Time,
	}, nil
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
	row, err := r.q.GetSVShareLink(ctx, tokenHash)
	if err != nil {
		return nil, fmt.Errorf("get share link: %w", err)
	}
	sl := &ShareLink{
		ID:        row.ID,
		SecretID:  row.SecretID,
		ExpiresAt: row.ExpiresAt.Time,
		CreatedAt: row.CreatedAt.Time,
	}
	if row.UsedAt.Valid {
		t := row.UsedAt.Time
		sl.UsedAt = &t
	}
	return sl, nil
}

func (r *Repository) MarkShareLinkUsed(ctx context.Context, linkID, orgID string) error {
	// org_id filter prevents a caller who knows the link UUID from burning
	// another organisation's share link (DoS / IDOR defense).
	return r.q.MarkSVShareLinkUsed(ctx, db.MarkSVShareLinkUsedParams{ID: linkID, OrgID: orgID})
}

// --- API tokens (uses shared api_keys table) ---

func (r *Repository) CreateAPIToken(ctx context.Context, orgID, userID, name, keyHash, keyPrefix string, scopes []string, expiresAt *time.Time) (*APIToken, error) {
	var expiresAtPG pgtype.Timestamptz
	if expiresAt != nil {
		expiresAtPG = pgtype.Timestamptz{Time: *expiresAt, Valid: true}
	}
	row, err := r.q.CreateSVAPIToken(ctx, db.CreateSVAPITokenParams{
		OrgID:     orgID,
		CreatedBy: userID,
		Name:      name,
		KeyHash:   keyHash,
		KeyPrefix: keyPrefix,
		Scopes:    scopes,
		ExpiresAt: expiresAtPG,
	})
	if err != nil {
		return nil, fmt.Errorf("create api token: %w", err)
	}
	return mapAPITokenRow(row), nil
}

func (r *Repository) ListAPITokens(ctx context.Context, orgID, userID string) ([]APIToken, error) {
	rows, err := r.q.ListSVAPITokens(ctx, db.ListSVAPITokensParams{
		OrgID:     orgID,
		CreatedBy: userID,
		Scope:     "secvault",
	})
	if err != nil {
		return nil, fmt.Errorf("list api tokens: %w", err)
	}
	out := make([]APIToken, 0, len(rows))
	for _, row := range rows {
		out = append(out, *mapAPITokenRow(row))
	}
	return out, nil
}

func (r *Repository) RevokeAPIToken(ctx context.Context, orgID, userID, tokenID string) error {
	n, err := r.q.RevokeSVAPIToken(ctx, db.RevokeSVAPITokenParams{
		ID:        tokenID,
		OrgID:     orgID,
		CreatedBy: userID,
		Scope:     "secvault",
	})
	if err != nil {
		return fmt.Errorf("revoke api token: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("token not found or already revoked")
	}
	return nil
}

// ListProjectSecrets returns all secrets (with metadata) for health scoring.
func (r *Repository) ListProjectSecrets(ctx context.Context, orgID, projectID string) ([]Secret, error) {
	rows, err := r.q.ListSVProjectSecrets(ctx, db.ListSVProjectSecretsParams{
		ProjectID: projectID,
		OrgID:     orgID,
	})
	if err != nil {
		return nil, fmt.Errorf("list project secrets: %w", err)
	}
	return mapSecretKeyRows(rows), nil
}

// GetSecretIDByKey returns the secret ID for an access-log query given key + env + org.
func (r *Repository) GetSecretIDByKey(ctx context.Context, orgID, envID, key string) (string, error) {
	id, err := r.q.GetSVSecretIDByKey(ctx, db.GetSVSecretIDByKeyParams{
		EnvironmentID: envID,
		OrgID:         orgID,
		Key:           key,
	})
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
	row, err := r.q.GetSVSecretByKey(ctx, db.GetSVSecretByKeyParams{
		EnvironmentID: envID,
		Key:           key,
		OrgID:         orgID,
	})
	if err != nil {
		return nil, fmt.Errorf("get secret by key: %w", err)
	}
	return mapSecretByKeyRow(row), nil
}

// --- Git Scanner ---

func (r *Repository) CreateGitScan(ctx context.Context, orgID, repoURL, branch string) (*GitScan, error) {
	row, err := r.q.CreateSVGitScan(ctx, db.CreateSVGitScanParams{
		OrgID:   orgID,
		RepoURL: repoURL,
		Branch:  branch,
	})
	if err != nil {
		return nil, fmt.Errorf("create git scan: %w", err)
	}
	return mapGitScanRow(row), nil
}

func (r *Repository) GetGitScan(ctx context.Context, orgID, scanID string) (*GitScan, error) {
	row, err := r.q.GetSVGitScan(ctx, db.GetSVGitScanParams{ID: scanID, OrgID: orgID})
	if err != nil {
		return nil, fmt.Errorf("get git scan: %w", err)
	}
	return mapGitScanRow(row), nil
}

func (r *Repository) ListGitScans(ctx context.Context, orgID string) ([]GitScan, error) {
	rows, err := r.q.ListSVGitScans(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("list git scans: %w", err)
	}
	out := make([]GitScan, 0, len(rows))
	for _, row := range rows {
		out = append(out, *mapGitScanRow(row))
	}
	return out, nil
}

func (r *Repository) UpdateGitScanStatus(ctx context.Context, scanID, orgID, status string, findingCount, openCount, dismissedCount int, errMsg string, scannedAt *time.Time) error {
	var scannedAtPG pgtype.Timestamptz
	if scannedAt != nil {
		scannedAtPG = pgtype.Timestamptz{Time: *scannedAt, Valid: true}
	}
	return r.q.UpdateSVGitScanStatus(ctx, db.UpdateSVGitScanStatusParams{
		Status:         status,
		FindingCount:   int32(findingCount),
		OpenCount:      int32(openCount),
		DismissedCount: int32(dismissedCount),
		ErrorMessage:   optionalText(errMsg),
		ScannedAt:      scannedAtPG,
		ID:             scanID,
		OrgID:          orgID,
	})
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
	rows, err := r.q.GetSVScanResults(ctx, db.GetSVScanResultsParams{ScanID: scanID, OrgID: orgID})
	if err != nil {
		return nil, fmt.Errorf("get scan results: %w", err)
	}
	out := make([]ScanResult, 0, len(rows))
	for _, row := range rows {
		out = append(out, ScanResult{
			ID:            row.ID,
			OrgID:         row.OrgID,
			ScanID:        row.ScanID,
			RepoURL:       row.RepoURL,
			CommitHash:    row.CommitHash,
			FilePath:      row.FilePath,
			LineNumber:    int(row.LineNumber),
			PatternName:   row.PatternName,
			MatchPreview:  row.MatchPreview,
			Severity:      row.Severity,
			Status:        row.Status,
			DismissReason: row.DismissReason,
			DismissCount:  int(row.DismissCount),
			CreatedAt:     row.CreatedAt.Time,
		})
	}
	return out, nil
}

func (r *Repository) DismissScanResult(ctx context.Context, orgID, resultID, reason string) error {
	n, err := r.q.DismissSVScanResult(ctx, db.DismissSVScanResultParams{
		Reason: reason,
		ID:     resultID,
		OrgID:  orgID,
	})
	if err != nil {
		return fmt.Errorf("dismiss scan result: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("scan result not found")
	}
	return nil
}

func (r *Repository) CountDismissals(ctx context.Context, orgID, patternName, filePath string) (int, error) {
	count, err := r.q.CountSVDismissals(ctx, db.CountSVDismissalsParams{
		OrgID:       orgID,
		PatternName: patternName,
		FilePath:    filePath,
	})
	return int(count), err
}

// --- Rotation policies ---

func (r *Repository) UpsertRotationPolicy(ctx context.Context, orgID, secretID string, intervalDays int) (*RotationPolicy, error) {
	nextRotation := time.Now().AddDate(0, 0, intervalDays)
	row, err := r.q.UpsertSVRotationPolicy(ctx, db.UpsertSVRotationPolicyParams{
		OrgID:          orgID,
		SecretID:       secretID,
		IntervalDays:   int32(intervalDays),
		NextRotationAt: nextRotation,
	})
	if err != nil {
		return nil, fmt.Errorf("upsert rotation policy: %w", err)
	}
	return mapRotationPolicyRow(row), nil
}

func (r *Repository) GetRotationPolicy(ctx context.Context, orgID, secretID string) (*RotationPolicy, error) {
	row, err := r.q.GetSVRotationPolicy(ctx, db.GetSVRotationPolicyParams{
		SecretID: secretID,
		OrgID:    orgID,
	})
	if err != nil {
		return nil, fmt.Errorf("get rotation policy: %w", err)
	}
	return mapRotationPolicyRow(row), nil
}

func (r *Repository) UpdateRotationAfterRotate(ctx context.Context, orgID, secretID string, intervalDays int) error {
	nextRotation := time.Now().AddDate(0, 0, intervalDays)
	return r.q.UpdateSVRotationAfterRotate(ctx, db.UpdateSVRotationAfterRotateParams{
		NextRotationAt: nextRotation,
		SecretID:       secretID,
		OrgID:          orgID,
	})
}

// ── Mapping helpers ───────────────────────────────────────────────────────────

// mapSecretKeyRows converts sqlc ListSVSecretKeysRow rows to domain Secret slices.
func mapSecretKeyRows(rows []db.ListSVSecretKeysRow) []Secret {
	out := make([]Secret, 0, len(rows))
	for _, row := range rows {
		s := Secret{
			ID:          row.ID,
			Key:         row.Key,
			Version:     int(row.Version),
			AccessCount: row.AccessCount,
			CreatedAt:   row.CreatedAt.Time,
			UpdatedAt:   row.UpdatedAt.Time,
		}
		if row.RotationDueAt.Valid {
			t := row.RotationDueAt.Time
			s.RotationDueAt = &t
		}
		if row.LastRotatedAt.Valid {
			t := row.LastRotatedAt.Time
			s.LastRotatedAt = &t
		}
		if row.LastAccessedAt.Valid {
			t := row.LastAccessedAt.Time
			s.LastAccessedAt = &t
		}
		out = append(out, s)
	}
	return out
}

// mapSecretByKeyRow converts a GetSVSecretByKeyRow to a domain Secret.
func mapSecretByKeyRow(row db.GetSVSecretByKeyRow) *Secret {
	s := &Secret{
		ID:          row.ID,
		Key:         row.Key,
		Version:     int(row.Version),
		AccessCount: row.AccessCount,
		CreatedAt:   row.CreatedAt.Time,
		UpdatedAt:   row.UpdatedAt.Time,
	}
	if row.RotationDueAt.Valid {
		t := row.RotationDueAt.Time
		s.RotationDueAt = &t
	}
	if row.LastRotatedAt.Valid {
		t := row.LastRotatedAt.Time
		s.LastRotatedAt = &t
	}
	if row.LastAccessedAt.Valid {
		t := row.LastAccessedAt.Time
		s.LastAccessedAt = &t
	}
	return s
}

// mapGitScanRow converts a SVGitScanRow to a domain GitScan.
func mapGitScanRow(row db.SVGitScanRow) *GitScan {
	gs := &GitScan{
		ID:             row.ID,
		OrgID:          row.OrgID,
		RepoURL:        row.RepoURL,
		Branch:         row.Branch,
		Status:         row.Status,
		FindingCount:   int(row.FindingCount),
		OpenCount:      int(row.OpenCount),
		DismissedCount: int(row.DismissedCount),
		ErrorMessage:   row.ErrorMessage,
		CreatedAt:      row.CreatedAt.Time,
	}
	if row.ScannedAt.Valid {
		t := row.ScannedAt.Time
		gs.ScannedAt = &t
	}
	return gs
}

// mapAPITokenRow converts a CreateSVAPITokenRow to a domain APIToken.
func mapAPITokenRow(row db.CreateSVAPITokenRow) *APIToken {
	t := &APIToken{
		ID:        row.ID,
		Name:      row.Name,
		KeyPrefix: row.KeyPrefix,
		Scopes:    row.Scopes,
		CreatedAt: row.CreatedAt.Time,
	}
	if row.ExpiresAt.Valid {
		ts := row.ExpiresAt.Time
		t.ExpiresAt = &ts
	}
	if row.LastUsedAt.Valid {
		ts := row.LastUsedAt.Time
		t.LastUsedAt = &ts
	}
	if row.RevokedAt.Valid {
		ts := row.RevokedAt.Time
		t.RevokedAt = &ts
	}
	return t
}

// mapRotationPolicyRow converts a SVRotationPolicyRow to a domain RotationPolicy.
func mapRotationPolicyRow(row db.SVRotationPolicyRow) *RotationPolicy {
	p := &RotationPolicy{
		ID:           row.ID,
		OrgID:        row.OrgID,
		SecretID:     row.SecretID,
		IntervalDays: int(row.IntervalDays),
		IsActive:     row.IsActive,
		CreatedAt:    row.CreatedAt.Time,
	}
	if row.LastRotatedAt.Valid {
		t := row.LastRotatedAt.Time
		p.LastRotatedAt = &t
	}
	if row.NextRotationAt.Valid {
		t := row.NextRotationAt.Time
		p.NextRotationAt = &t
	}
	return p
}

// ListGitScansCursor returns git scans for orgID using keyset pagination on (created_at DESC, id DESC).
func (r *Repository) ListGitScansCursor(ctx context.Context, orgID string, cursorID string, cursorTS time.Time, limit int) ([]GitScan, error) {
	args := []any{orgID}
	q := `SELECT id::text, org_id, repo_url, branch, status,
	             finding_count, open_count, dismissed_count,
	             COALESCE(error_message, ''), scanned_at, created_at
	      FROM so_git_scans
	      WHERE org_id = $1::uuid`
	if !cursorTS.IsZero() {
		q += ` AND (created_at < $2 OR (created_at = $2 AND id::text < $3))`
		args = append(args, cursorTS, cursorID)
	}
	q += ` ORDER BY created_at DESC, id DESC LIMIT $` + strconv.Itoa(len(args)+1)
	args = append(args, limit+1)
	rows, err := r.db.Query(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("list git scans cursor: %w", err)
	}
	defer rows.Close()
	var out []GitScan
	for rows.Next() {
		var gs GitScan
		var fc, oc, dc int32
		var scannedAt, createdAt pgtype.Timestamptz
		if err := rows.Scan(&gs.ID, &gs.OrgID, &gs.RepoURL, &gs.Branch, &gs.Status,
			&fc, &oc, &dc, &gs.ErrorMessage, &scannedAt, &createdAt); err != nil {
			return nil, fmt.Errorf("scan git scan cursor row: %w", err)
		}
		gs.FindingCount = int(fc)
		gs.OpenCount = int(oc)
		gs.DismissedCount = int(dc)
		if scannedAt.Valid {
			t := scannedAt.Time
			gs.ScannedAt = &t
		}
		gs.CreatedAt = createdAt.Time
		out = append(out, gs)
	}
	return out, rows.Err()
}
