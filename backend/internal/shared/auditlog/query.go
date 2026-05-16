package auditlog

import (
	"context"
	"encoding/json"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// LogEntry is the read model for a single audit log row.
type LogEntry struct {
	ID           string            `json:"id"`
	OrgID        string            `json:"org_id"`
	UserID       *string           `json:"user_id,omitempty"`
	UserEmail    string            `json:"user_email,omitempty"`
	Action       string            `json:"action"`
	ResourceType string            `json:"resource_type"`
	ResourceID   string            `json:"resource_id,omitempty"`
	ResourceName string            `json:"resource_name,omitempty"`
	Details      map[string]string `json:"details,omitempty"`
	IPAddress    string            `json:"ip_address,omitempty"`
	CreatedAt    time.Time         `json:"created_at"`
}

// List returns the last limit entries for the given organisation, ordered by
// created_at descending.  limit is capped at 500 to prevent runaway queries.
func List(ctx context.Context, db *pgxpool.Pool, orgID string, limit int) ([]LogEntry, error) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 500 {
		limit = 500
	}

	rows, err := db.Query(ctx, `
		SELECT id, org_id, user_id, user_email, action, resource_type,
		       resource_id, resource_name, details, ip_address, created_at
		FROM audit_log
		WHERE org_id = $1::uuid
		ORDER BY created_at DESC
		LIMIT $2`,
		orgID, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []LogEntry
	for rows.Next() {
		var e LogEntry
		var userEmail *string
		var resourceID *string
		var resourceName *string
		var ipAddress *string
		var rawDetails []byte

		if err := rows.Scan(
			&e.ID, &e.OrgID, &e.UserID, &userEmail, &e.Action,
			&e.ResourceType, &resourceID, &resourceName,
			&rawDetails, &ipAddress, &e.CreatedAt,
		); err != nil {
			return nil, err
		}

		if userEmail != nil {
			e.UserEmail = *userEmail
		}
		if resourceID != nil {
			e.ResourceID = *resourceID
		}
		if resourceName != nil {
			e.ResourceName = *resourceName
		}
		if ipAddress != nil {
			e.IPAddress = *ipAddress
		}
		if len(rawDetails) > 0 {
			_ = json.Unmarshal(rawDetails, &e.Details)
		}

		entries = append(entries, e)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return entries, nil
}
