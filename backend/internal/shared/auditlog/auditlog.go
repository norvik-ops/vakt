// Package auditlog provides a lightweight, fire-and-forget audit log writer.
// Every write operation (create/update/delete/approve/export) on compliance-relevant
// resources is appended to the audit_log table.  Errors are intentionally swallowed
// so that a failing audit write never causes a business operation to fail.
package auditlog

import (
	"context"
	"encoding/json"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Entry describes a single compliance audit event.
type Entry struct {
	OrgID        string
	UserID       string            // empty string = system / anonymous
	UserEmail    string
	Action       string            // create | update | delete | approve | export
	ResourceType string            // vvt | dpia | avv | breach | dsr | control | policy | risk | incident
	ResourceID   string
	ResourceName string
	Details      map[string]string // optional extra info (changed fields, event markers, …)
	IPAddress    string
}

// Log writes one audit entry to the audit_log table.
// Errors are intentionally swallowed — audit logging must never cause a business
// operation to fail.  If db is nil the function is a no-op.
func Log(ctx context.Context, db *pgxpool.Pool, e Entry) {
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
