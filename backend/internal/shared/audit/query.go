package audit

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
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

// ListFilters holds the optional filter parameters for List.
type ListFilters struct {
	From      *time.Time // created_at >= From
	To        *time.Time // created_at <= To
	UserEmail string     // ILIKE match on user_email
	Action    string     // exact match on action
	Limit     int        // default 100, max 500
	Offset    int        // for server-side pagination
}

// ListResult wraps the entries and total count for the current filter set.
type ListResult struct {
	Entries []LogEntry `json:"entries"`
	Total   int        `json:"total"`
}

// List returns audit log entries for the given organisation, honouring the
// supplied filters.  limit is capped at 500 to prevent runaway queries.
func List(ctx context.Context, db *pgxpool.Pool, orgID string, filters ListFilters) (ListResult, error) {
	if filters.Limit <= 0 {
		filters.Limit = 100
	}
	if filters.Limit > 500 {
		filters.Limit = 500
	}

	// Build WHERE clause dynamically.
	args := []any{orgID} // $1 = org_id
	argIdx := 2

	var conditions []string
	conditions = append(conditions, "org_id = $1::uuid")
	conditions = append(conditions, "deleted_at IS NULL")

	if filters.From != nil {
		conditions = append(conditions, fmt.Sprintf("created_at >= $%d", argIdx))
		args = append(args, *filters.From)
		argIdx++
	}
	if filters.To != nil {
		conditions = append(conditions, fmt.Sprintf("created_at <= $%d", argIdx))
		args = append(args, *filters.To)
		argIdx++
	}
	if filters.UserEmail != "" {
		conditions = append(conditions, fmt.Sprintf("user_email ILIKE $%d", argIdx))
		args = append(args, "%"+filters.UserEmail+"%")
		argIdx++
	}
	if filters.Action != "" {
		conditions = append(conditions, fmt.Sprintf("action = $%d", argIdx))
		args = append(args, filters.Action)
		argIdx++
	}

	where := strings.Join(conditions, " AND ")

	// Count query (same WHERE, no LIMIT/OFFSET).
	countSQL := "SELECT COUNT(*) FROM audit_log WHERE " + where
	var total int
	if err := db.QueryRow(ctx, countSQL, args...).Scan(&total); err != nil {
		return ListResult{}, err
	}

	// Data query.
	dataArgs := append(args, filters.Limit, filters.Offset) //nolint:gocritic // intentional append to copy
	dataSQL := fmt.Sprintf(`
		SELECT id, org_id, user_id, user_email, action, resource_type,
		       resource_id, resource_name, details, ip_address, created_at
		FROM audit_log
		WHERE %s
		ORDER BY created_at DESC
		LIMIT $%d OFFSET $%d`,
		where, argIdx, argIdx+1,
	)

	rows, err := db.Query(ctx, dataSQL, dataArgs...)
	if err != nil {
		return ListResult{}, err
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
			return ListResult{}, err
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
		return ListResult{}, err
	}

	if entries == nil {
		entries = []LogEntry{}
	}

	return ListResult{Entries: entries, Total: total}, nil
}
