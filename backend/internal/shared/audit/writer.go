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

	"github.com/jackc/pgx/v5/pgxpool"
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
}

// Write writes one audit entry to the audit_log table.
// Errors are intentionally swallowed — audit logging must never cause a business
// operation to fail.  If db is nil the function is a no-op.
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

	_, _ = db.Exec(ctx, `
		INSERT INTO audit_log
		  (org_id, user_id, user_email, action, resource_type, resource_id, resource_name, details, ip_address)
		VALUES ($1::uuid, $2::uuid, $3, $4, $5, $6, $7, $8, $9)`,
		e.OrgID, userID, userEmail, e.Action,
		e.ResourceType, resourceID, resourceName,
		details, ipAddress,
	)
}

// toNullableString returns nil when s is empty so that the DB column stores NULL
// rather than an empty string.
func toNullableString(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
