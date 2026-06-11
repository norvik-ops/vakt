package alerting

import "time"

// Channel is the public representation of a notification channel (no encrypted URL).
type Channel struct {
	ID             string    `json:"id"`
	OrgID          string    `json:"org_id"`
	Name           string    `json:"name"`
	Type           string    `json:"type"`   // slack | teams | webhook | email
	Events         []string  `json:"events"` // e.g. ["finding.sla_overdue","breach.created"]
	Enabled        bool      `json:"enabled"`
	CreatedAt      time.Time `json:"created_at"`
	HasHmacSecret  bool      `json:"has_hmac_secret"`
	HmacSecretHint string    `json:"hmac_secret_hint,omitempty"` // last 8 hex chars of secret, shown as hint
}

// CreateChannelResponse is returned once at creation time and includes the plaintext HMAC secret.
type CreateChannelResponse struct {
	Channel
	HmacSecret string `json:"hmac_secret"` // shown once at creation, plaintext hex
}

// CreateChannelInput is the validated request body for creating a channel.
// For email-type channels, URL holds the recipient email address.
type CreateChannelInput struct {
	Name   string   `json:"name"   validate:"required,min=1,max=80"`
	Type   string   `json:"type"   validate:"required,oneof=slack teams webhook email"`
	URL    string   `json:"url"    validate:"required,url"`
	Events []string `json:"events" validate:"required,min=1"`
}

// DeliveryLogEntry is one row from alert_delivery_log.
type DeliveryLogEntry struct {
	ID           string    `json:"id"`
	ChannelID    *string   `json:"channel_id,omitempty"`
	Event        string    `json:"event"`
	Status       string    `json:"status"`
	ResponseCode *int      `json:"response_code,omitempty"`
	SentAt       time.Time `json:"sent_at"`
}

// Known trigger events — used as constants across the codebase.
const (
	EventFindingSLAOverdue  = "finding.sla_overdue"
	EventBreachCreated      = "breach.created"
	EventDSROverdue         = "dsr.overdue"
	EventAVVExpired         = "avv.expired"
	EventScanFailed         = "scan.failed"
	EventFindingNewCritical = "finding.new_critical"
)
