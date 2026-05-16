package notify

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// NotificationChannel represents an admin-configured delivery channel.
type NotificationChannel struct {
	ID        string          `json:"id"`
	OrgID     string          `json:"org_id"`
	Name      string          `json:"name"`
	Channel   Channel         `json:"channel"`
	Config    json.RawMessage `json:"config"`
	IsActive  bool            `json:"is_active"`
	CreatedAt time.Time       `json:"created_at"`
	UpdatedAt time.Time       `json:"updated_at"`
}

// CreateChannelInput is the request body for creating a notification channel.
type CreateChannelInput struct {
	Name    string          `json:"name"    validate:"required,min=1,max=128"`
	Channel Channel         `json:"channel" validate:"required,oneof=slack teams email webhook"`
	Config  json.RawMessage `json:"config"  validate:"required"`
}

// ListNotificationChannels returns all notification channels for the given org.
func (s *Service) ListNotificationChannels(ctx context.Context, orgID string) ([]NotificationChannel, error) {
	rows, err := s.db.Query(ctx, `
		SELECT id::text, org_id::text, name, channel, config, is_active, created_at, updated_at
		FROM notification_channels
		WHERE org_id = $1::uuid
		ORDER BY created_at ASC`,
		orgID,
	)
	if err != nil {
		return nil, fmt.Errorf("query notification channels: %w", err)
	}
	defer rows.Close()

	var channels []NotificationChannel
	for rows.Next() {
		var ch NotificationChannel
		var configRaw []byte
		if err := rows.Scan(
			&ch.ID, &ch.OrgID, &ch.Name, &ch.Channel,
			&configRaw, &ch.IsActive, &ch.CreatedAt, &ch.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan notification channel: %w", err)
		}
		ch.Config = json.RawMessage(configRaw)
		channels = append(channels, ch)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate notification channel rows: %w", err)
	}
	return channels, nil
}

// CreateNotificationChannel inserts a new notification channel for the org.
func (s *Service) CreateNotificationChannel(ctx context.Context, orgID string, input CreateChannelInput) (*NotificationChannel, error) {
	configJSON, err := json.Marshal(input.Config)
	if err != nil {
		return nil, fmt.Errorf("marshal channel config: %w", err)
	}

	var ch NotificationChannel
	var configRaw []byte
	err = s.db.QueryRow(ctx, `
		INSERT INTO notification_channels (org_id, name, channel, config)
		VALUES ($1::uuid, $2, $3, $4::jsonb)
		RETURNING id::text, org_id::text, name, channel, config, is_active, created_at, updated_at`,
		orgID, input.Name, string(input.Channel), string(configJSON),
	).Scan(
		&ch.ID, &ch.OrgID, &ch.Name, &ch.Channel,
		&configRaw, &ch.IsActive, &ch.CreatedAt, &ch.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("insert notification channel: %w", err)
	}
	ch.Config = json.RawMessage(configRaw)
	return &ch, nil
}

// DeleteNotificationChannel removes a notification channel by ID within the org.
// Returns an error if the channel does not exist or belongs to a different org.
func (s *Service) DeleteNotificationChannel(ctx context.Context, orgID, channelID string) error {
	tag, err := s.db.Exec(ctx, `
		DELETE FROM notification_channels
		WHERE id = $1::uuid AND org_id = $2::uuid`,
		channelID, orgID,
	)
	if err != nil {
		return fmt.Errorf("delete notification channel: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("notification channel not found")
	}
	return nil
}
