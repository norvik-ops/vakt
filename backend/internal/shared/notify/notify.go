// Package notify provides two notification paths used by all Vakt modules.
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
	"time"

	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"

	"github.com/matharnica/vakt/internal/config"
	"github.com/matharnica/vakt/internal/shared/queuemetrics"
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
	// Parse the full Redis URL (redis://:password@host:port) — asynq expects "host:port".
	redisOpt := asynq.RedisClientOpt{Addr: "localhost:6379"}
	if cfg != nil && cfg.RedisUrl != "" {
		if parsed, err := redis.ParseURL(cfg.RedisUrl); err == nil {
			redisOpt = asynq.RedisClientOpt{
				Addr:     parsed.Addr,
				Password: parsed.Password,
				DB:       parsed.DB,
			}
		} else {
			log.Warn().Err(err).Str("url", cfg.RedisUrl).Msg("notify: invalid Redis URL, falling back to localhost:6379")
		}
	}
	client := asynq.NewClient(redisOpt)
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
		queuemetrics.RecordError("default")
		log.Error().Err(err).Str("org_id", msg.OrgID).Msg("failed to enqueue notification")
	}

	return nil
}

// pubClient is an optional Redis client used to push a wakeup signal to open
// SSE notification streams (S98-5). Set once at startup via SetPublisher.
// nil → no push (the SSE safety-poll still delivers within 30 s).
var pubClient *redis.Client

// SetPublisher wires the Redis client used by Send to push SSE wakeups.
// Call once during API/worker startup. Safe to leave unset (push becomes no-op).
func SetPublisher(rdb *redis.Client) { pubClient = rdb }

// Send inserts a single row into user_notifications for in-app display.
// It is intentionally non-fatal: any database error is logged via zerolog and
// silently discarded so that callers (scanner workers, training completions,
// breach events, etc.) are never blocked by a notification failure.
//
// S124-6 (E2E-03): Send detaches from the caller's context. It is best-effort
// and frequently fired at the tail of a request handler; if it ran on the
// request context, a client that already received its response (context
// canceled) would spuriously fail the notification write with "context
// canceled". The write is bounded by its own 5s timeout instead.
func Send(ctx context.Context, db *pgxpool.Pool, orgID, title, body, notifType, module string) {
	sendCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	_, err := db.Exec(sendCtx,
		`INSERT INTO user_notifications (org_id, title, body, type, module)
		 VALUES ($1::uuid, $2, $3, $4, $5)`,
		orgID, title, body, notifType, module)
	if err != nil {
		log.Error().Err(err).Str("module", module).Msg("notify.Send failed")
		return
	}
	// S98-5: push a wakeup to open SSE streams. Channel key MUST match
	// dashboard.notifyChannel ("notify:<org_id>"). Best-effort.
	if pubClient != nil {
		if perr := pubClient.Publish(sendCtx, "notify:"+orgID, "1").Err(); perr != nil {
			log.Warn().Err(perr).Str("org_id", orgID).Msg("notify.Send: SSE publish failed")
		}
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
