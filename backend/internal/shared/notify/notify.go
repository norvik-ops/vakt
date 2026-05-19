// Package notify provides two notification paths used by all SecHealth modules.
//
// The Service type persists a notification to the notifications table and then
// enqueues an Asynq delivery task so a worker can fan it out over Slack,
// Teams, email, or a webhook. This path is used for user-configured alert
// channels and is retry-safe: if enqueue fails the DB record survives and can
// be swept by a background job.
//
// The package-level Send function is a thin, fire-and-forget helper that
// writes directly to user_notifications for in-app display. It never returns
// an error — failures are logged and swallowed so callers can remain
// non-fatal.
package notify

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"

	"github.com/matharnica/vakt/internal/config"
)

// Channel identifies the external delivery channel for a notification.
// The value is stored in the notifications table and matched by the worker
// to select the appropriate Sender adapter.
type Channel string

const (
	// ChannelSlack routes delivery through the configured Slack integration.
	ChannelSlack Channel = "slack"
	// ChannelTeams routes delivery through Microsoft Teams incoming webhooks.
	ChannelTeams Channel = "teams"
	// ChannelEmail routes delivery via the configured SMTP server.
	ChannelEmail Channel = "email"
	// ChannelWebhook delivers a generic HTTP POST to an arbitrary URL.
	ChannelWebhook Channel = "webhook"
)

// NotificationJobType is the Asynq task type string for notification delivery.
// It is exported so the worker package can register a matching handler without
// creating an import cycle.
const NotificationJobType = "notifications:deliver"

// Message is the notification payload passed to Service.Notify and serialised
// into the Asynq task. Target interpretation depends on Channel: a URL for
// webhooks, an email address for email, or a channel name for Slack/Teams.
type Message struct {
	Title   string  `json:"title"`
	Body    string  `json:"body"`
	OrgID   string  `json:"org_id"`
	Channel Channel `json:"channel"`
	Target  string  `json:"target"` // webhook URL, email address, Slack channel, etc.
}

// Sender is the interface that delivery adapters must satisfy. Each Channel
// constant has a corresponding Sender registered in the worker process.
type Sender interface {
	Send(ctx context.Context, msg Message) error
}

// Service persists notifications to the database and enqueues them for
// asynchronous delivery over the configured external channels (Slack, Teams,
// email, webhook). Use NewService to construct a ready-to-use instance.
type Service struct {
	db    *pgxpool.Pool
	cfg   *config.Config
	queue *asynq.Client
}

// NewService constructs a Service, creating an Asynq client connected to the
// Redis address specified in cfg. The caller owns the db pool lifecycle;
// the Service does not close it.
func NewService(db *pgxpool.Pool, cfg *config.Config) *Service {
	client := asynq.NewClient(asynq.RedisClientOpt{Addr: cfg.RedisUrl})
	return &Service{
		db:    db,
		cfg:   cfg,
		queue: client,
	}
}

// Notify persists msg to the notifications table and then enqueues an Asynq
// delivery task. If enqueue fails the error is logged but not returned —
// the persisted record can be retried by a background sweep job. A persist
// failure is returned as a wrapped error.
func (s *Service) Notify(ctx context.Context, msg Message) error {
	// Persist to notifications table.
	if err := s.persist(ctx, msg); err != nil {
		return fmt.Errorf("notify persist: %w", err)
	}

	// Enqueue delivery task.
	payload, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("notify marshal payload: %w", err)
	}

	task := asynq.NewTask(NotificationJobType, payload)
	if _, err := s.queue.EnqueueContext(ctx, task); err != nil {
		// Enqueue failure is non-fatal: the record is already in the DB and
		// can be retried by a sweep job.
		log.Error().Err(err).Str("org_id", msg.OrgID).Msg("failed to enqueue notification")
	}

	return nil
}

// Send inserts a single row into user_notifications for in-app display.
// It is intentionally non-fatal: any database error is logged via zerolog and
// silently discarded so that callers (scanner workers, training completions,
// breach events, etc.) are never blocked by a notification failure.
func Send(ctx context.Context, db *pgxpool.Pool, orgID, title, body, notifType, module string) {
	_, err := db.Exec(ctx,
		`INSERT INTO user_notifications (org_id, title, body, type, module)
		 VALUES ($1::uuid, $2, $3, $4, $5)`,
		orgID, title, body, notifType, module)
	if err != nil {
		log.Error().Err(err).Str("module", module).Msg("notify.Send failed")
	}
}

// persist inserts a pending notification row.
func (s *Service) persist(ctx context.Context, msg Message) error {
	payload, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("marshal notification payload: %w", err)
	}

	_, err = s.db.Exec(ctx, `
		INSERT INTO notifications (org_id, type, channel, payload, status)
		VALUES ($1::uuid, $2, $3, $4::jsonb, 'pending')`,
		msg.OrgID, NotificationJobType, string(msg.Channel), string(payload),
	)
	if err != nil {
		return fmt.Errorf("insert notification: %w", err)
	}
	return nil
}
