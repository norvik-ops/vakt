// WriteEntry + Write sind der lightweight fire-and-forget Audit-Log-Writer
// (vorher Package auditlog, in Sprint 14.6 als Teil von audit konsolidiert).
// Jede mutierende Operation auf compliance-relevante Ressourcen
// (create/update/delete/approve/export) sollte einen WriteEntry über
// Write(ctx, db, e) anhängen. Fehler werden bewusst verschluckt — ein
// gescheiterter Audit-Write darf NIEMALS eine Business-Operation zum
// Scheitern bringen.
//
// Die Middleware-Variante (AuditMiddleware + Logger.Log) ist für
// Echo-Request-Handler gedacht; Write ist für service-layer fire-and-forget
// aus Business-Logik heraus. Beide Pfade schreiben in dieselbe
// audit_log-Tabelle.
package audit

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"
)

// WriteEntry describes a single compliance audit event.
type WriteEntry struct {
	OrgID        string
	UserID       string // empty string = system / anonymous
	UserEmail    string
	Action       string // create | update | delete | approve | export
	ResourceType string // vvt | dpia | avv | breach | dsr | control | policy | risk | incident
	ResourceID   string
	ResourceName string
	Details      map[string]string // optional extra info (changed fields, event markers, …)
	IPAddress    string
	// CreatedAt overrides the timestamp used for both the DB column and the
	// chain hash. Zero value (default) means time.Now(). Only set this in
	// tests — production code must never back-date entries.
	CreatedAt time.Time
}

// Write writes one audit entry to the audit_log table.
// Errors are intentionally swallowed — audit logging must never cause a business
// operation to fail.  If db is nil the function is a no-op.
//
// Each entry extends the per-org hash chain: prev_hash is the entry_hash of
// the most-recent existing row for this org (NULL for the first row), and
// entry_hash is SHA-256(prev_hash || canonical(this_row)).  The select-for-
// update keeps the chain serialisable under concurrent writers in the same
// org — at the cost of a per-write row-lock, which is acceptable given audit
// inserts are already on a fire-and-forget goroutine.  See ADR-0040.
func Write(ctx context.Context, db *pgxpool.Pool, e WriteEntry) {
	if db == nil {
		return
	}

	userID := toNullableString(e.UserID)
	userEmail := toNullableString(e.UserEmail)
	resourceID := toNullableString(e.ResourceID)
	resourceName := toNullableString(e.ResourceName)
	ipAddress := toNullableString(e.IPAddress)

	var details []byte
	if len(e.Details) > 0 {
		details, _ = json.Marshal(e.Details)
	}

	tx, err := db.Begin(ctx)
	if err != nil {
		log.Error().Err(err).Str("org_id", e.OrgID).Str("action", e.Action).Msg("audit: begin tx failed")
		return
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// Lock the chain tail for this org and read its hash.
	var prevHash []byte
	err = tx.QueryRow(ctx, `
		SELECT entry_hash FROM audit_log
		WHERE org_id = $1::uuid AND entry_hash IS NOT NULL
		ORDER BY created_at DESC, id DESC
		LIMIT 1
		FOR UPDATE`,
		e.OrgID,
	).Scan(&prevHash)
	if err != nil && err.Error() != "no rows in result set" {
		// pgx returns ErrNoRows for empty chain — that is the expected case
		// for the very first chained entry of an org. Any other error is real.
		if !isNoRows(err) {
			log.Error().Err(err).Str("org_id", e.OrgID).Msg("audit: read chain tail failed")
			return
		}
		prevHash = nil
	}

	id := uuid.NewString()
	now := e.CreatedAt
	if now.IsZero() {
		now = time.Now().UTC()
	}
	chainInput := ChainInput{
		ID:           id,
		OrgID:        e.OrgID,
		UserID:       e.UserID,
		UserEmail:    e.UserEmail,
		Action:       e.Action,
		ResourceType: e.ResourceType,
		ResourceID:   e.ResourceID,
		ResourceName: e.ResourceName,
		Details:      e.Details,
		IPAddress:    e.IPAddress,
		CreatedAt:    now,
	}
	entryHash := EntryHash(prevHash, chainInput)

	if _, err := tx.Exec(ctx, `
		INSERT INTO audit_log
		  (id, org_id, user_id, user_email, action, resource_type, resource_id, resource_name, details, ip_address, created_at, prev_hash, entry_hash)
		VALUES ($1::uuid, $2::uuid, $3::uuid, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)`,
		id, e.OrgID, userID, userEmail, e.Action,
		e.ResourceType, resourceID, resourceName,
		details, ipAddress, now, prevHash, entryHash,
	); err != nil {
		log.Error().Err(err).Str("org_id", e.OrgID).Str("action", e.Action).Msg("audit: failed to write audit log entry")
		return
	}
	if err := tx.Commit(ctx); err != nil {
		log.Error().Err(err).Str("org_id", e.OrgID).Msg("audit: commit failed")
		return
	}

	// S88-6: opt-in outbound forward to a customer Syslog/SIEM sink. Non-blocking;
	// only runs after the entry is durably committed. No-op when not configured.
	if f := currentForwarder(); f != nil {
		f.Forward(e)
	}
}

// isNoRows reports whether err signals pgx.ErrNoRows. We compare on the
// error message instead of importing pgx here to keep the writer's import
// graph minimal — the package is also used in tests that swap out pool
// implementations.
func isNoRows(err error) bool {
	return err != nil && err.Error() == "no rows in result set"
}

// toNullableString returns nil when s is empty so that the DB column stores NULL
// rather than an empty string.
func toNullableString(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
