package retention

import "time"

// RetentionConfig holds the data-retention and digest settings for an organisation.
type RetentionConfig struct {
	OrgID                string    `json:"org_id"`
	AuditLogDays         int       `json:"audit_log_days"`
	FindingsResolvedDays int       `json:"findings_resolved_days"`
	NotificationsDays    int       `json:"notifications_days"`
	ScanHistoryDays      int       `json:"scan_history_days"`
	DigestEnabled        bool      `json:"digest_enabled"`
	DigestHour           int16     `json:"digest_hour"`
	UpdatedAt            time.Time `json:"updated_at"`
}

// UpdateRetentionConfigInput carries the fields that the API caller may change.
type UpdateRetentionConfigInput struct {
	AuditLogDays         *int   `json:"audit_log_days"          validate:"omitempty,min=0"`
	FindingsResolvedDays *int   `json:"findings_resolved_days"  validate:"omitempty,min=0"`
	NotificationsDays    *int   `json:"notifications_days"       validate:"omitempty,min=0"`
	ScanHistoryDays      *int   `json:"scan_history_days"       validate:"omitempty,min=0"`
	DigestEnabled        *bool  `json:"digest_enabled"`
	DigestHour           *int16 `json:"digest_hour"             validate:"omitempty,min=0,max=23"`
}
