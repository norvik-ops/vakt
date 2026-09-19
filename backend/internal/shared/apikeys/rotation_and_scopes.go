package apikeys

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"time"
)

// Sprint 20 / S20-2: API-Key-Rotation.
//
// Hinweis (2026-09-18): Die zweite, „Punkt-Grammatik"-Scope-Prüfung
// (RequireScope-Middleware + ScopeAllows-Helper, `vaktcomply.*` / exakt / `*`)
// wurde entfernt (R1-W7C-N2). Sie war an keine Route gemountet — die aktive
// Scope-Autorisierung läuft ausschließlich über die Pfad-Präfix-Grammatik in
// internal/auth/middleware.go (`scopePathPrefixes`, `:ro`/`.*`-Suffixe). Zwei
// divergierende Grammatiken für dasselbe Feld sind eine stille Fehlerquelle.

// RotateKey generiert einen neuen Schlüssel-Hash, schreibt den alten in
// `previous_key_hash` mit `previous_key_grace_expires_at = NOW() + 24h`.
// Beide Hashes werden für die Grace-Period akzeptiert.
//
// userID muss dem created_by-Wert in der DB entsprechen — Rotation fremder
// Keys durch andere Nutzer der gleichen Org ist damit ausgeschlossen (IDOR).
//
// Sprint 20 S20-2. Returns the NEW raw key (one-shot disclosure, wie bei
// CreateKey — nie erneut abrufbar).
func (s *Service) RotateKey(ctx context.Context, orgID, userID, keyID string) (*CreateResult, error) {
	// 1. Neuen Raw-Key generieren (gleicher Algorithmus wie CreateKey).
	rawBytes := make([]byte, 24)
	if _, err := rand.Read(rawBytes); err != nil {
		return nil, fmt.Errorf("rand: %w", err)
	}
	rawKey := "sk_" + base64.URLEncoding.WithPadding(base64.NoPadding).EncodeToString(rawBytes)
	hash := sha256.Sum256([]byte(rawKey))
	newHash := hex.EncodeToString(hash[:])
	prefix := rawKey[:10]

	// 2. UPDATE: alten Hash in previous_key_hash schieben, neuen Hash
	//    aktivieren, Grace 24 h. AND created_by = $6 verhindert IDOR.
	grace := time.Now().UTC().Add(24 * time.Hour)
	var key APIKey
	if err := s.db.QueryRow(ctx, `
		UPDATE api_keys
		SET previous_key_hash               = key_hash,
		    previous_key_grace_expires_at   = $1,
		    key_hash                        = $2,
		    key_prefix                      = $3,
		    rotated_at                      = NOW()
		WHERE id = $4::uuid AND org_id = $5::uuid AND created_by = $6::uuid
		RETURNING id::text, name, key_prefix, scopes, last_used_at, expires_at, created_at`,
		grace, newHash, prefix, keyID, orgID, userID,
	).Scan(&key.ID, &key.Name, &key.KeyPrefix, &key.Scopes, &key.LastUsedAt, &key.ExpiresAt, &key.CreatedAt); err != nil {
		if err.Error() == "no rows in result set" {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("rotate: %w", err)
	}

	return &CreateResult{APIKey: key, RawKey: rawKey}, nil
}

// LoginHistoryWriter persistiert einen Login-Versuch. Sprint 20 S20-6.
// Schwirrt durch ALLE Login-Pfade (password, OIDC, magic-link, api-key
// success). Best-Effort — Fehler werden ge-loggt aber nie propagiert.
type LoginAttempt struct {
	OrgID     string
	UserID    string
	Email     string
	IP        string
	UserAgent string
	Source    string // 'password' | 'oidc' | 'magic_link' | 'api_key'
	Result    string // 'ok' | 'bad_password' | 'locked' | 'mfa_failed' | 'oidc_failed'
}

// RecordLoginAttempt schreibt ein Audit-Event in login_history.
func (s *Service) RecordLoginAttempt(ctx context.Context, a LoginAttempt) {
	// Truncate UA für defensiv-konservative Storage.
	if len(a.UserAgent) > 200 {
		a.UserAgent = a.UserAgent[:200]
	}
	_, err := s.db.Exec(ctx, `
		INSERT INTO login_history (org_id, user_id, email, ip, user_agent, source, result)
		VALUES (NULLIF($1, '')::uuid, NULLIF($2, '')::uuid, NULLIF($3, ''), NULLIF($4, ''),
		        NULLIF($5, ''), $6, $7)`,
		a.OrgID, a.UserID, a.Email, a.IP, a.UserAgent, a.Source, a.Result,
	)
	if err != nil {
		// Best-Effort. Login-History-Failures dürfen nie den Login blockieren.
		// Logging zentral — fmt-Pkg nicht ziehen, log.Warn nicht hier weil
		// Aufrufer das schon macht.
		_ = err
	}
}

// LoginHistoryEntry ist das Read-Modell für die Account-Settings-Page.
type LoginHistoryEntry struct {
	TS        time.Time `json:"ts"`
	IP        string    `json:"ip,omitempty"`
	UserAgent string    `json:"user_agent,omitempty"`
	Source    string    `json:"source"`
	Result    string    `json:"result"`
}

// ListLoginHistoryForUser lädt die letzten 50 Login-Versuche eines Users.
// Sprint 20 S20-7.
func (s *Service) ListLoginHistoryForUser(ctx context.Context, userID string) ([]LoginHistoryEntry, error) {
	// orgid-lint: global — caller's own login history, scoped by user_id from the auth token
	rows, err := s.db.Query(ctx, `
		SELECT ts, ip, user_agent, source, result
		FROM login_history
		WHERE user_id = $1::uuid
		ORDER BY ts DESC
		LIMIT 50`, userID,
	)
	if err != nil {
		return nil, fmt.Errorf("query login_history: %w", err)
	}
	defer rows.Close()
	out := make([]LoginHistoryEntry, 0)
	for rows.Next() {
		var e LoginHistoryEntry
		var ip, ua *string
		if err := rows.Scan(&e.TS, &ip, &ua, &e.Source, &e.Result); err != nil {
			continue
		}
		if ip != nil {
			e.IP = *ip
		}
		if ua != nil {
			e.UserAgent = *ua
		}
		out = append(out, e)
	}
	return out, nil
}
