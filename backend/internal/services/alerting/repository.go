package alerting

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// rawChannel includes the url_encrypted field for internal use by the service.
type rawChannel struct {
	ID                  string
	OrgID               string
	Name                string
	Type                string
	URLEncrypted        []byte
	Events              []string
	Enabled             bool
	HmacSecretEncrypted []byte
}

// Repository handles all database access for the alerting package.
type Repository struct {
	db *pgxpool.Pool
}

// NewRepository returns a new Repository backed by the given connection pool.
func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

// ListChannels returns all notification channels for the given org, newest first.
func (r *Repository) ListChannels(ctx context.Context, orgID string) ([]Channel, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, org_id, name, type, events, enabled, created_at,
		       hmac_secret_encrypted IS NOT NULL AS has_hmac_secret
		FROM notification_channels
		WHERE org_id = $1::uuid
		ORDER BY created_at DESC
	`, orgID)
	if err != nil {
		return nil, fmt.Errorf("list channels: %w", err)
	}
	defer rows.Close()

	var channels []Channel
	for rows.Next() {
		var ch Channel
		if err := rows.Scan(&ch.ID, &ch.OrgID, &ch.Name, &ch.Type, &ch.Events, &ch.Enabled, &ch.CreatedAt, &ch.HasHmacSecret); err != nil {
			return nil, fmt.Errorf("scan channel: %w", err)
		}
		channels = append(channels, ch)
	}
	return channels, rows.Err()
}

// CreateChannel inserts a new notification channel and returns it.
func (r *Repository) CreateChannel(ctx context.Context, orgID string, in CreateChannelInput, encryptedURL []byte, encryptedHmacSecret []byte) (*Channel, error) {
	var ch Channel
	err := r.db.QueryRow(ctx, `
		INSERT INTO notification_channels (org_id, name, type, url_encrypted, events, enabled, hmac_secret_encrypted)
		VALUES ($1::uuid, $2, $3, $4, $5, true, $6)
		RETURNING id, org_id, name, type, events, enabled, created_at,
		          hmac_secret_encrypted IS NOT NULL AS has_hmac_secret
	`, orgID, in.Name, in.Type, encryptedURL, in.Events, encryptedHmacSecret).
		Scan(&ch.ID, &ch.OrgID, &ch.Name, &ch.Type, &ch.Events, &ch.Enabled, &ch.CreatedAt, &ch.HasHmacSecret)
	if err != nil {
		return nil, fmt.Errorf("create channel: %w", err)
	}
	return &ch, nil
}

// DeleteChannel removes a notification channel by id, scoped to the org.
func (r *Repository) DeleteChannel(ctx context.Context, orgID, id string) error {
	ct, err := r.db.Exec(ctx, `
		DELETE FROM notification_channels WHERE id = $1::uuid AND org_id = $2::uuid
	`, id, orgID)
	if err != nil {
		return fmt.Errorf("delete channel: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return fmt.Errorf("channel not found")
	}
	return nil
}

// ToggleChannel updates the enabled flag on a channel.
func (r *Repository) ToggleChannel(ctx context.Context, orgID, id string, enabled bool) error {
	ct, err := r.db.Exec(ctx, `
		UPDATE notification_channels SET enabled = $3, updated_at = now()
		WHERE id = $1::uuid AND org_id = $2::uuid
	`, id, orgID, enabled)
	if err != nil {
		return fmt.Errorf("toggle channel: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return fmt.Errorf("channel not found")
	}
	return nil
}

// GetEnabledChannelsForEvent returns channels that have the event in their events array
// and are currently enabled.
func (r *Repository) GetEnabledChannelsForEvent(ctx context.Context, orgID, event string) ([]rawChannel, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, org_id, name, type, url_encrypted, events, enabled, hmac_secret_encrypted
		FROM notification_channels
		WHERE org_id = $1::uuid
		  AND enabled = true
		  AND events @> ARRAY[$2]::text[]
	`, orgID, event)
	if err != nil {
		return nil, fmt.Errorf("get channels for event: %w", err)
	}
	defer rows.Close()

	var channels []rawChannel
	for rows.Next() {
		var ch rawChannel
		if err := rows.Scan(&ch.ID, &ch.OrgID, &ch.Name, &ch.Type, &ch.URLEncrypted, &ch.Events, &ch.Enabled, &ch.HmacSecretEncrypted); err != nil {
			return nil, fmt.Errorf("scan raw channel: %w", err)
		}
		channels = append(channels, ch)
	}
	return channels, rows.Err()
}

// GetChannelRaw returns a single channel including url_encrypted, scoped to the org.
func (r *Repository) GetChannelRaw(ctx context.Context, orgID, id string) (*rawChannel, error) {
	var ch rawChannel
	err := r.db.QueryRow(ctx, `
		SELECT id, org_id, name, type, url_encrypted, events, enabled, hmac_secret_encrypted
		FROM notification_channels
		WHERE id = $1::uuid AND org_id = $2::uuid
	`, id, orgID).Scan(&ch.ID, &ch.OrgID, &ch.Name, &ch.Type, &ch.URLEncrypted, &ch.Events, &ch.Enabled, &ch.HmacSecretEncrypted)
	if err != nil {
		return nil, fmt.Errorf("get channel raw: %w", err)
	}
	return &ch, nil
}

// LogDelivery inserts a row into alert_delivery_log.
func (r *Repository) LogDelivery(ctx context.Context, orgID string, channelID *string, event, status string, responseCode *int, payload map[string]any) error {
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		payloadJSON = []byte("{}")
	}
	_, err = r.db.Exec(ctx, `
		INSERT INTO alert_delivery_log (org_id, channel_id, event, payload, status, response_code)
		VALUES ($1::uuid, $2::uuid, $3, $4, $5, $6)
	`, orgID, channelID, event, payloadJSON, status, responseCode)
	if err != nil {
		return fmt.Errorf("log delivery: %w", err)
	}
	return nil
}

// ListDeliveryLog returns the last 100 delivery log entries for an org.
func (r *Repository) ListDeliveryLog(ctx context.Context, orgID string, limit int) ([]DeliveryLogEntry, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := r.db.Query(ctx, `
		SELECT id, channel_id, event, status, response_code, sent_at
		FROM alert_delivery_log
		WHERE org_id = $1::uuid
		ORDER BY sent_at DESC
		LIMIT $2
	`, orgID, limit)
	if err != nil {
		return nil, fmt.Errorf("list delivery log: %w", err)
	}
	defer rows.Close()

	var entries []DeliveryLogEntry
	for rows.Next() {
		var e DeliveryLogEntry
		if err := rows.Scan(&e.ID, &e.ChannelID, &e.Event, &e.Status, &e.ResponseCode, &e.SentAt); err != nil {
			return nil, fmt.Errorf("scan delivery log entry: %w", err)
		}
		entries = append(entries, e)
	}
	return entries, rows.Err()
}

// ListChannelDeliveries returns the last 50 delivery log entries for a specific channel, scoped to the org.
func (r *Repository) ListChannelDeliveries(ctx context.Context, orgID, channelID string, limit int) ([]DeliveryLogEntry, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := r.db.Query(ctx, `
		SELECT id, channel_id, event, status, response_code, sent_at
		FROM alert_delivery_log
		WHERE org_id = $1::uuid AND channel_id = $2::uuid
		ORDER BY sent_at DESC
		LIMIT $3
	`, orgID, channelID, limit)
	if err != nil {
		return nil, fmt.Errorf("list channel deliveries: %w", err)
	}
	defer rows.Close()

	var entries []DeliveryLogEntry
	for rows.Next() {
		var e DeliveryLogEntry
		if err := rows.Scan(&e.ID, &e.ChannelID, &e.Event, &e.Status, &e.ResponseCode, &e.SentAt); err != nil {
			return nil, fmt.Errorf("scan delivery log entry: %w", err)
		}
		entries = append(entries, e)
	}
	return entries, rows.Err()
}
