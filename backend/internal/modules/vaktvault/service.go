// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package vaktvault

import (
	"bufio"
	"context"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"crypto/rand"
	"crypto/sha256"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"

	"github.com/matharnica/vakt/internal/db"
	"github.com/matharnica/vakt/internal/services/crossevidence"
	"github.com/matharnica/vakt/internal/shared/platform/events"
)

// Service handles SecVault business logic.
type Service struct {
	db        *pgxpool.Pool
	repo      *Repository
	masterKey []byte
	queue     *asynq.Client
}

// NewService creates a new SecVault service.
func NewService(db *pgxpool.Pool, masterKey []byte, queue *asynq.Client) *Service {
	return &Service{
		db:        db,
		repo:      NewRepository(db),
		masterKey: masterKey,
		queue:     queue,
	}
}

// --- Projects ---

// CreateProject creates a new secret project under the given organisation.
func (s *Service) CreateProject(ctx context.Context, orgID, userID, name, desc string) (*Project, error) {
	slug := slugify(name)
	return s.repo.CreateProject(ctx, orgID, userID, name, slug, desc)
}

// ListProjects returns all projects for an organisation.
func (s *Service) ListProjects(ctx context.Context, orgID string) ([]Project, error) {
	return s.repo.ListProjects(ctx, orgID)
}

// GetProject retrieves a single project by ID within an organisation.
func (s *Service) GetProject(ctx context.Context, orgID, projectID string) (*Project, error) {
	return s.repo.GetProject(ctx, orgID, projectID)
}

// DeleteProject removes a project and all its child data (cascade in DB).
func (s *Service) DeleteProject(ctx context.Context, orgID, projectID string) error {
	return s.repo.DeleteProject(ctx, orgID, projectID)
}

// --- Environments ---

// CreateEnvironment adds a named environment to a project.
func (s *Service) CreateEnvironment(ctx context.Context, orgID, projectID, name string) (*Environment, error) {
	return s.repo.CreateEnvironment(ctx, orgID, projectID, name)
}

// ListEnvironments returns all environments for a project within the caller's org.
func (s *Service) ListEnvironments(ctx context.Context, orgID, projectID string) ([]Environment, error) {
	return s.repo.ListEnvironments(ctx, orgID, projectID)
}

// --- Secrets ---

// SetSecret encrypts and stores (or updates) a secret value.
// The per-project key is derived from the master key so that a project deletion
// also renders its ciphertext unrecoverable.
func (s *Service) SetSecret(ctx context.Context, orgID, envID, userID, key, value string) (*Secret, error) {
	// We need the project ID to derive the per-project key.
	projectID, err := s.getProjectIDForEnv(ctx, envID, orgID)
	if err != nil {
		return nil, fmt.Errorf("resolve project for env: %w", err)
	}

	projectKey, err := DeriveProjectKey(s.masterKey, projectID)
	if err != nil {
		return nil, fmt.Errorf("derive project key: %w", err)
	}

	// Phase 1: upsert to obtain the stable secret_id. The secret_id is part of
	// the Associated Data (S90-3), but is only known after the INSERT/UPDATE, so
	// we first store a (still AES-256-GCM-encrypted) non-AAD ciphertext.
	encrypted, err := Encrypt(projectKey, []byte(value))
	if err != nil {
		return nil, fmt.Errorf("encrypt secret: %w", err)
	}
	sec, err := s.repo.UpsertSecret(ctx, orgID, envID, userID, key, encrypted)
	if err != nil {
		return nil, err
	}

	// Phase 2: re-encrypt binding the ciphertext to org_id + secret_id and
	// overwrite the value (no version bump). If this fails the row still holds a
	// valid legacy ciphertext that decrypts via the backward-compatible path, so
	// the secret is never lost — it self-heals (AAD-binds) on the next write.
	bound, err := EncryptWithAAD(projectKey, []byte(value), secretAAD(orgID, sec.ID))
	if err != nil {
		return nil, fmt.Errorf("encrypt secret (aad): %w", err)
	}
	if err := s.repo.UpdateSecretCiphertext(ctx, orgID, sec.ID, bound); err != nil {
		return nil, fmt.Errorf("bind secret ciphertext: %w", err)
	}

	return sec, nil
}

// GetSecret retrieves and decrypts a secret value, writing an access log entry.
func (s *Service) GetSecret(ctx context.Context, orgID, envID, key, accessVia, ip string) (*Secret, error) {
	projectID, err := s.getProjectIDForEnv(ctx, envID, orgID)
	if err != nil {
		return nil, fmt.Errorf("resolve project for env: %w", err)
	}

	projectKey, err := DeriveProjectKey(s.masterKey, projectID)
	if err != nil {
		return nil, fmt.Errorf("derive project key: %w", err)
	}

	sec, encryptedValue, err := s.repo.GetSecretRaw(ctx, orgID, envID, key)
	if err != nil {
		return nil, err
	}

	// DecryptWithAAD verifies the org_id+secret_id binding for enc:v2: values and
	// transparently falls back to the no-AAD path for legacy ciphertexts (S90-3).
	plaintext, err := DecryptWithAAD(projectKey, encryptedValue, secretAAD(orgID, sec.ID))
	if err != nil {
		return nil, fmt.Errorf("decrypt secret: %w", err)
	}
	sec.Value = string(plaintext)

	// Update access metadata (best-effort, non-blocking).
	_ = s.repo.UpdateSecretAccess(ctx, sec.ID)

	// Log the access.
	_ = s.repo.LogAccess(ctx, sec.ID, orgID, nil, accessVia, ip, "")

	return sec, nil
}

// ListSecretKeys returns metadata for all secrets in an environment (values omitted).
// Returns an error wrapping pgx.ErrNoRows when the environment does not belong to orgID.
func (s *Service) ListSecretKeys(ctx context.Context, orgID, envID string) ([]Secret, error) {
	if _, err := s.getProjectIDForEnv(ctx, envID, orgID); err != nil {
		return nil, err
	}
	return s.repo.ListSecretKeys(ctx, orgID, envID)
}

// DeleteSecret removes a secret from an environment.
func (s *Service) DeleteSecret(ctx context.Context, orgID, envID, key string) error {
	return s.repo.DeleteSecret(ctx, orgID, envID, key)
}

// --- Access log ---

// GetAccessLog returns paginated access log entries for a secret.
func (s *Service) GetAccessLog(ctx context.Context, orgID, envID, key string, limit, offset int) ([]AccessLogEntry, error) {
	secretID, err := s.repo.GetSecretIDByKey(ctx, orgID, envID, key)
	if err != nil {
		return nil, err
	}
	return s.repo.GetAccessLog(ctx, secretID, orgID, limit, offset)
}

// AccessLogPage is the standard pagination envelope returned by GetProjectAccessLog.
// Total is the unfiltered row count (needed so the frontend can compute page count).
// Page and Limit echo back the normalised values actually used for the query.
type AccessLogPage struct {
	// Entries holds the access log rows for the current page.
	Entries []ProjectAccessLogEntry `json:"entries"`
	// Total is the total number of log entries across all pages.
	Total int `json:"total"`
	// Page is the 1-based current page number.
	Page int `json:"page"`
	// Limit is the number of entries per page (clamped to [1, 100]).
	Limit int `json:"limit"`
}

// GetProjectAccessLog returns a page of access log entries for every secret in a project.
// Page is normalised to ≥ 1 and limit is clamped to [1, 100] (default 25 when out of range).
// The DB offset is computed as (page-1)*limit before the query is issued.
func (s *Service) GetProjectAccessLog(ctx context.Context, orgID, projectID string, page, limit int) (*AccessLogPage, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 25
	}
	offset := (page - 1) * limit
	entries, total, err := s.repo.GetProjectAccessLog(ctx, orgID, projectID, limit, offset)
	if err != nil {
		return nil, err
	}
	if entries == nil {
		entries = []ProjectAccessLogEntry{}
	}
	return &AccessLogPage{Entries: entries, Total: total, Page: page, Limit: limit}, nil
}

// --- Health ---

// GetProjectHealth computes the aggregate health score for a project: the
// average of each secret's health score, plus every secret's issues in one
// flat, key-labelled list.
func (s *Service) GetProjectHealth(ctx context.Context, orgID, projectID string) (*ProjectHealth, error) {
	secrets, err := s.repo.ListProjectSecrets(ctx, orgID, projectID)
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	perSecret := make([]SecretHealth, 0, len(secrets))
	for _, sec := range secrets {
		perSecret = append(perSecret, computeHealth(sec, now))
	}
	agg := aggregateProjectHealth(perSecret)
	return &agg, nil
}

// aggregateProjectHealth folds per-secret health entries into the
// project-level summary: the average score (100 when there are no secrets
// yet — nothing to flag), and every secret's issues in one flat,
// key-labelled list.
func aggregateProjectHealth(perSecret []SecretHealth) ProjectHealth {
	if len(perSecret) == 0 {
		return ProjectHealth{Score: 100, Issues: []string{}}
	}
	totalScore := 0
	issues := make([]string, 0)
	for _, h := range perSecret {
		totalScore += h.HealthScore
		for _, issue := range h.Issues {
			issues = append(issues, fmt.Sprintf("%s: %s", h.Key, issue))
		}
	}
	return ProjectHealth{Score: totalScore / len(perSecret), Issues: issues}
}

// computeHealth calculates a 0-100 health score for a single secret.
func computeHealth(s Secret, now time.Time) SecretHealth {
	score := 100
	var issues []string

	ageInDays := int(now.Sub(s.CreatedAt).Hours() / 24)

	daysSinceRotation := ageInDays
	if s.LastRotatedAt != nil {
		daysSinceRotation = int(now.Sub(*s.LastRotatedAt).Hours() / 24)
	}

	if ageInDays > 90 {
		score -= 20
		issues = append(issues, "secret older than 90 days")
	}
	if daysSinceRotation > 90 {
		score -= 20
		issues = append(issues, "not rotated in over 90 days")
	}
	if s.AccessCount == 0 && ageInDays > 7 {
		score -= 10
		issues = append(issues, "never accessed since creation")
	}

	if score < 0 {
		score = 0
	}

	return SecretHealth{
		SecretID:          s.ID,
		Key:               s.Key,
		AgeInDays:         ageInDays,
		DaysSinceRotation: daysSinceRotation,
		AccessCount:       s.AccessCount,
		HealthScore:       score,
		Issues:            issues,
	}
}

// --- Share links ---

// CreateShareLink generates a time-limited token for sharing a single secret.
func (s *Service) CreateShareLink(ctx context.Context, orgID, envID, userID, key string, expiresInHours int) (*ShareLink, error) {
	if expiresInHours < 1 || expiresInHours > 168 {
		return nil, fmt.Errorf("expires_in_hours must be between 1 and 168")
	}

	secretID, err := s.repo.GetSecretIDByKey(ctx, orgID, envID, key)
	if err != nil {
		return nil, err
	}

	rawToken, tokenHash, err := generateToken()
	if err != nil {
		return nil, err
	}

	expiresAt := time.Now().UTC().Add(time.Duration(expiresInHours) * time.Hour)

	sl, err := s.repo.CreateShareLink(ctx, secretID, orgID, userID, tokenHash, expiresAt)
	if err != nil {
		return nil, err
	}
	sl.ShareURL = "/api/v1/vaktvault/share/" + rawToken
	return sl, nil
}

// UseShareLink validates a share token, decrypts the secret, and marks the link used.
func (s *Service) UseShareLink(ctx context.Context, rawToken string) (string, error) {
	sum := sha256.Sum256([]byte(rawToken))
	tokenHash := hex.EncodeToString(sum[:])

	sl, err := s.repo.GetShareLink(ctx, tokenHash)
	if err != nil {
		return "", fmt.Errorf("share link not found")
	}

	if sl.UsedAt != nil {
		return "", fmt.Errorf("share link already used")
	}
	if time.Now().UTC().After(sl.ExpiresAt) {
		return "", fmt.Errorf("share link expired")
	}

	// Fetch the org_id once up-front so we can pass it to MarkShareLinkUsed
	// (which requires org_id for the IDOR-prevention WHERE clause) and to
	// LogAccess, avoiding two separate DB round-trips later.
	slOrgID, err := s.getOrgIDForShareLink(ctx, sl.ID)
	if err != nil {
		return "", fmt.Errorf("resolve share link org: %w", err)
	}

	// Fetch the secret using the org_id stored on the share link row.
	// This avoids an unscoped lookup with an empty org_id and keeps org isolation intact.
	secWithOrg, encVal, err := s.getSecretByShareLinkOrg(ctx, sl, slOrgID)
	if err != nil {
		return "", fmt.Errorf("resolve secret: %w", err)
	}

	projectID, err := s.getProjectIDForEnvBySecretID(ctx, secWithOrg.ID)
	if err != nil {
		return "", fmt.Errorf("resolve project: %w", err)
	}

	projectKey, err := DeriveProjectKey(s.masterKey, projectID)
	if err != nil {
		return "", fmt.Errorf("derive key: %w", err)
	}

	plaintext, err := DecryptWithAAD(projectKey, encVal, secretAAD(slOrgID, secWithOrg.ID))
	if err != nil {
		return "", fmt.Errorf("decrypt: %w", err)
	}

	// org_id is included in the UPDATE so that a caller who learns the link UUID
	// cannot burn another organisation's share link (DoS / IDOR prevention).
	_ = s.repo.MarkShareLinkUsed(ctx, sl.ID, slOrgID)
	_ = s.repo.LogAccess(ctx, sl.SecretID, slOrgID, nil, "share_link", "", "")

	return string(plaintext), nil
}

// getSecretByShareLinkOrg fetches a secret using the org_id already resolved for the share link.
func (s *Service) getSecretByShareLinkOrg(ctx context.Context, sl *ShareLink, orgID string) (*Secret, []byte, error) {
	return s.repo.GetSecretByID(ctx, orgID, sl.SecretID)
}

func (s *Service) getOrgIDForShareLink(ctx context.Context, linkID string) (string, error) {
	orgID, err := s.repo.q.GetSVShareLinkOrgID(ctx, linkID)
	if err != nil {
		return "", fmt.Errorf("get org for share link: %w", err)
	}
	return orgID, nil
}

// --- API tokens ---

// CreateToken creates a SecretOps-scoped API key.
func (s *Service) CreateToken(ctx context.Context, orgID, userID, name string, expiresAt *time.Time) (*APIToken, error) {
	rawKey, keyHash, err := generateAPIKey()
	if err != nil {
		return nil, err
	}

	prefix := "sk_so_" + rawKey[:6]
	scopes := []string{"vaktvault"}

	t, err := s.repo.CreateAPIToken(ctx, orgID, userID, name, keyHash, prefix, scopes, expiresAt)
	if err != nil {
		return nil, err
	}
	t.RawKey = rawKey
	return t, nil
}

// ListTokens returns all SecretOps-scoped API keys for a user.
func (s *Service) ListTokens(ctx context.Context, orgID, userID string) ([]APIToken, error) {
	return s.repo.ListAPITokens(ctx, orgID, userID)
}

// RevokeToken revokes a SecretOps-scoped API key.
func (s *Service) RevokeToken(ctx context.Context, orgID, userID, tokenID string) error {
	return s.repo.RevokeAPIToken(ctx, orgID, userID, tokenID)
}

// --- Helpers ---

// getProjectIDForEnv resolves the project UUID for a given environment ID.
func (s *Service) getProjectIDForEnv(ctx context.Context, envID, orgID string) (string, error) {
	projectID, err := s.repo.q.GetSVEnvProjectID(ctx, db.GetSVEnvProjectIDParams{ID: envID, OrgID: orgID})
	if err != nil {
		return "", fmt.Errorf("environment not found: %w", err)
	}
	return projectID, nil
}

// getProjectIDForEnvBySecretID resolves the project ID for a secret (via its environment).
func (s *Service) getProjectIDForEnvBySecretID(ctx context.Context, secretID string) (string, error) {
	projectID, err := s.repo.q.GetSVSecretProjectID(ctx, secretID)
	if err != nil {
		return "", fmt.Errorf("project not found for secret: %w", err)
	}
	return projectID, nil
}

// generateToken creates a random 32-byte hex token and its SHA-256 hash.
func generateToken() (rawToken, tokenHash string, err error) {
	b := make([]byte, 32)
	if _, err = rand.Read(b); err != nil {
		return "", "", fmt.Errorf("generate token: %w", err)
	}
	rawToken = hex.EncodeToString(b)
	sum := sha256.Sum256([]byte(rawToken))
	tokenHash = hex.EncodeToString(sum[:])
	return rawToken, tokenHash, nil
}

// generateAPIKey creates a SecretOps API key with "sk_so_" prefix.
func generateAPIKey() (rawKey, keyHash string, err error) {
	b := make([]byte, 24)
	if _, err = rand.Read(b); err != nil {
		return "", "", fmt.Errorf("generate api key: %w", err)
	}
	rawKey = "sk_so_" + hex.EncodeToString(b)
	sum := sha256.Sum256([]byte(rawKey))
	keyHash = hex.EncodeToString(sum[:])
	return rawKey, keyHash, nil
}

// --- Import ---

// parseDotenv parses a dotenv-format string into a key→value map.
// Lines starting with # and blank lines are ignored.
// Lines without '=' are ignored. Surrounding quotes are stripped.
func parseDotenv(content string) map[string]string {
	pairs := make(map[string]string)
	scanner := bufio.NewScanner(strings.NewReader(content))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		if len(value) >= 2 &&
			((value[0] == '"' && value[len(value)-1] == '"') ||
				(value[0] == '\'' && value[len(value)-1] == '\'')) {
			value = value[1 : len(value)-1]
		}
		pairs[key] = value
	}
	return pairs
}

// ImportSecrets bulk-imports secrets from a dotenv string into a project's environment.
func (s *Service) ImportSecrets(ctx context.Context, orgID, projectID, envName, content, source string) (*ImportResult, error) {
	if source != "dotenv" {
		return nil, fmt.Errorf("%s import not yet implemented", source)
	}

	// Resolve environment ID by name.
	envs, err := s.repo.ListEnvironments(ctx, orgID, projectID)
	if err != nil {
		return nil, err
	}
	var envID string
	for _, e := range envs {
		if e.Name == envName {
			envID = e.ID
			break
		}
	}
	if envID == "" {
		return nil, fmt.Errorf("environment %q not found", envName)
	}

	pairs := parseDotenv(content)
	if len(pairs) > 500 {
		return nil, fmt.Errorf("too many secrets: max 500 per import")
	}
	result := &ImportResult{}
	for key, value := range pairs {
		if _, err := s.SetSecret(ctx, orgID, envID, "", key, value); err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("failed to import %s: %v", key, err))
		} else {
			result.Imported++
		}
	}
	return result, nil
}

// ExportSecrets returns secrets in dotenv format for shell eval.
func (s *Service) ExportSecrets(ctx context.Context, orgID, projectID, envID string) (string, error) {
	secrets, err := s.repo.ListSecretKeys(ctx, orgID, envID)
	if err != nil {
		return "", err
	}

	var sb strings.Builder
	for _, sec := range secrets {
		plain, err := s.GetSecret(ctx, orgID, envID, sec.Key, "export", "")
		if err != nil {
			continue
		}
		fmt.Fprintf(&sb, "%s=%s\n", sec.Key, plain.Value)
	}
	return sb.String(), nil
}

// --- Rotation ---

// RotateSecret generates a new secret value for the given secret key.
func (s *Service) RotateSecret(ctx context.Context, orgID, envID, key string, input RotateInput) error {
	var newValue string
	switch input.Type {
	case "random_string":
		length := input.Length
		if length <= 0 {
			length = 32
		}
		b := make([]byte, length/2+1)
		if _, err := rand.Read(b); err != nil {
			return err
		}
		newValue = hex.EncodeToString(b)[:length]
	case "uuid":
		newValue = uuid.New().String()
	case "db_password":
		b := make([]byte, 16)
		if _, err := rand.Read(b); err != nil {
			return err
		}
		newValue = base64.URLEncoding.EncodeToString(b)
	default:
		return fmt.Errorf("unknown rotation type: %s", input.Type)
	}

	// Get current secret record for rotation policy update.
	secret, err := s.repo.GetSecretByKey(ctx, orgID, envID, key)
	if err != nil {
		return fmt.Errorf("secret not found: %w", err)
	}

	if _, err := s.SetSecret(ctx, orgID, envID, "", key, newValue); err != nil {
		return err
	}

	// Update rotation policy timestamps if one exists (best-effort).
	_ = s.repo.UpdateRotationAfterRotate(ctx, orgID, secret.ID, 90)

	// Enqueue cross-module evidence for SecVitals access-control controls.
	if s.queue != nil {
		if task, taskErr := crossevidence.NewRecordEvidenceTask(events.SecretRotated(orgID, key)); taskErr == nil {
			if _, enqErr := s.queue.EnqueueContext(ctx, task); enqErr != nil {
				log.Warn().Err(enqErr).Str("org_id", orgID).Msg("vaktvault: secret-rotation evidence enqueue failed (evidence may be missed on Redis outage)")
			}
		}
	}

	return nil
}

// SetRotationPolicy upserts a rotation policy for a secret.
func (s *Service) SetRotationPolicy(ctx context.Context, orgID, secretID string, intervalDays int) (*RotationPolicy, error) {
	return s.repo.UpsertRotationPolicy(ctx, orgID, secretID, intervalDays)
}

// GetRotationPolicy returns the rotation policy for a secret.
func (s *Service) GetRotationPolicy(ctx context.Context, orgID, secretID string) (*RotationPolicy, error) {
	return s.repo.GetRotationPolicy(ctx, orgID, secretID)
}

// --- Git Scanner ---

// gitScanPayload is the Asynq task payload for a git scan job.
// EncryptedCredentials holds an AES-256-GCM-encrypted, base64-encoded JSON
// representation of the GitScanCredentials struct. The plaintext is never
// stored — encrypt before enqueue, decrypt in the worker.
type gitScanPayload struct {
	ScanID               string `json:"scan_id"`
	OrgID                string `json:"org_id"`
	RepoURL              string `json:"repo_url"`
	Branch               string `json:"branch"`
	EncryptedCredentials string `json:"encrypted_credentials,omitempty"`
}

// encryptPayloadField encrypts a plaintext string with AES-256-GCM and returns
// a base64-encoded ciphertext (nonce prepended, identical layout to Encrypt).
func encryptPayloadField(data string, key []byte) (string, error) {
	ct, err := Encrypt(key, []byte(data))
	if err != nil {
		return "", fmt.Errorf("encrypt payload field: %w", err)
	}
	return base64.StdEncoding.EncodeToString(ct), nil
}

// DecryptPayloadField is the inverse of encryptPayloadField. It is exported so
// the worker can call it without importing a separate helper package.
func DecryptPayloadField(encoded string, key []byte) (string, error) {
	ct, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", fmt.Errorf("base64 decode payload field: %w", err)
	}
	plain, err := Decrypt(key, ct)
	if err != nil {
		return "", fmt.Errorf("decrypt payload field: %w", err)
	}
	return string(plain), nil
}

// TriggerGitScan creates a scan record and enqueues a scan job.
// If credentials are provided they are AES-GCM-encrypted with the master key
// before being written to the Redis payload so that no plaintext tokens are
// stored in the job queue.
func (s *Service) TriggerGitScan(ctx context.Context, orgID string, input TriggerGitScanInput) (*GitScan, error) {
	scan, err := s.repo.CreateGitScan(ctx, orgID, input.RepoURL, input.Branch)
	if err != nil {
		return nil, err
	}

	if s.queue != nil {
		p := gitScanPayload{
			ScanID:  scan.ID,
			OrgID:   orgID,
			RepoURL: input.RepoURL,
			Branch:  input.Branch,
		}

		if input.Credentials != nil {
			credsJSON, marshalErr := json.Marshal(input.Credentials)
			if marshalErr != nil {
				return nil, fmt.Errorf("marshal credentials: %w", marshalErr)
			}
			encCreds, encErr := encryptPayloadField(string(credsJSON), s.masterKey)
			if encErr != nil {
				return nil, fmt.Errorf("encrypt credentials for queue: %w", encErr)
			}
			p.EncryptedCredentials = encCreds
		}

		payloadBytes, _ := json.Marshal(p)
		task := asynq.NewTask(TaskGitScan, payloadBytes)
		if _, err := s.queue.EnqueueContext(ctx, task); err != nil {
			return nil, fmt.Errorf("enqueue git scan: %w", err)
		}
	}

	return scan, nil
}

// GetGitScan returns a git scan by ID.
func (s *Service) GetGitScan(ctx context.Context, orgID, scanID string) (*GitScan, error) {
	return s.repo.GetGitScan(ctx, orgID, scanID)
}

// ListGitScans returns all git scans for the org.
func (s *Service) ListGitScans(ctx context.Context, orgID string) ([]GitScan, error) {
	return s.repo.ListGitScans(ctx, orgID)
}

// GetGitScanResults returns findings for a scan.
func (s *Service) GetGitScanResults(ctx context.Context, orgID, scanID string) ([]ScanResult, error) {
	return s.repo.GetScanResults(ctx, orgID, scanID)
}

// DismissScanResult marks a scan result as dismissed.
func (s *Service) DismissScanResult(ctx context.Context, orgID, resultID string, input DismissScanResultInput) error {
	return s.repo.DismissScanResult(ctx, orgID, resultID, input.Reason)
}

// slugify converts a human-readable name into a URL-safe slug.
func slugify(name string) string {
	s := strings.ToLower(name)
	s = strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			return r
		}
		if r == ' ' || r == '_' {
			return '-'
		}
		return -1
	}, s)
	for strings.Contains(s, "--") {
		s = strings.ReplaceAll(s, "--", "-")
	}
	return strings.Trim(s, "-")
}

// ListGitScansCursor returns git scans using keyset pagination.
func (s *Service) ListGitScansCursor(ctx context.Context, orgID string, cursorID string, cursorTS time.Time, limit int) ([]GitScan, error) {
	return s.repo.ListGitScansCursor(ctx, orgID, cursorID, cursorTS, limit)
}
