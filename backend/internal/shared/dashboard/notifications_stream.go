package dashboard

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"
	"go.opentelemetry.io/otel"
)

// StreamNotifications ist der SSE-Endpoint, der neue Notifications für die
// aktive Org pushed.
//
// S98-5: Push-first via Redis Pub/Sub with safety-poll fallback.
//   - When Redis is available: subscribe to "notify:<org_id>"; on message,
//     fetch and flush all rows since last cursor. DB is only hit on events.
//   - Safety-poll (30 s) catches any missed pub/sub events (e.g. during Redis
//     failover) so the stream never goes stale.
//   - When Redis is unavailable: falls back to 2 s poll (prior behaviour).
//   - Heartbeat every 30 s prevents nginx idle-timeout.
//
// nginx-Anforderung: `X-Accel-Buffering: no` ist gesetzt; siehe
// docs/wiki/reverse-proxy.md für die nginx-Location-Block-Empfehlung.
//
// ADR-0019: nutzt das gleiche SSE-Pattern wie der AI-Streaming-Endpoint.
const (
	// ponytail: legacy 2 s poll kept as Redis-unavailable fallback (S98-5)
	notificationStreamPollInterval = 2 * time.Second
	notificationStreamHeartbeat    = 30 * time.Second
	notificationStreamSafetyPoll   = 30 * time.Second
)

// notifyChannel returns the Redis Pub/Sub channel key for an org.
func notifyChannel(orgID string) string { return "notify:" + orgID }

// StreamNotifications serves an SSE stream of new notifications.
// Publish a message to notifyChannel(orgID) from any write path to trigger
// a push without waiting for the next poll cycle.
func (h *Handler) StreamNotifications(c echo.Context) error {
	orgID, _ := c.Get("org_id").(string)
	if orgID == "" {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
	}

	resp := c.Response()
	resp.Header().Set(echo.HeaderContentType, "text/event-stream")
	resp.Header().Set("Cache-Control", "no-cache")
	resp.Header().Set("Connection", "keep-alive")
	resp.Header().Set("X-Accel-Buffering", "no")
	resp.WriteHeader(http.StatusOK)

	tracer := otel.Tracer("vakt.dashboard.notifications.stream")
	streamCtx, span := tracer.Start(c.Request().Context(), "notifications.stream")
	defer span.End()

	cursor := time.Now().UTC()

	flush := func() bool {
		items, newCursor, err := h.fetchNotificationsSince(streamCtx, orgID, cursor)
		if err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return false
			}
			log.Warn().Err(err).Str("org_id", orgID).Msg("notification-stream poll failed")
			return true
		}
		for _, item := range items {
			payload, merr := json.Marshal(item)
			if merr != nil {
				continue
			}
			if _, werr := fmt.Fprintf(resp.Writer, "data: %s\n\n", payload); werr != nil {
				return false
			}
		}
		if len(items) > 0 {
			resp.Flush()
			cursor = newCursor
		}
		return true
	}

	heartbeat := func() bool {
		_, werr := fmt.Fprint(resp.Writer, "event: ping\ndata: {}\n\n")
		if werr == nil {
			resp.Flush()
		}
		return werr == nil
	}

	// Attempt Redis pub/sub push path.
	if h.rdb != nil {
		sub := h.rdb.Subscribe(streamCtx, notifyChannel(orgID))
		subCh := sub.Channel()
		defer func() { _ = sub.Close() }()

		safetyTicker := time.NewTicker(notificationStreamSafetyPoll)
		heartbeatTicker := time.NewTicker(notificationStreamHeartbeat)
		defer safetyTicker.Stop()
		defer heartbeatTicker.Stop()

		for {
			select {
			case <-streamCtx.Done():
				return nil
			case msg, ok := <-subCh:
				if !ok {
					// Redis pub/sub broke — fall through to poll below.
					goto fallbackPoll
				}
				_ = msg
				if !flush() {
					return nil
				}
			case <-safetyTicker.C:
				if !flush() {
					return nil
				}
			case <-heartbeatTicker.C:
				if !heartbeat() {
					return nil
				}
			}
		}
	}

fallbackPoll:
	// Redis unavailable — legacy 2 s poll.
	pollTicker := time.NewTicker(notificationStreamPollInterval)
	heartbeatTicker := time.NewTicker(notificationStreamHeartbeat)
	defer pollTicker.Stop()
	defer heartbeatTicker.Stop()

	for {
		select {
		case <-streamCtx.Done():
			return nil
		case <-pollTicker.C:
			if !flush() {
				return nil
			}
		case <-heartbeatTicker.C:
			if !heartbeat() {
				return nil
			}
		}
	}
}

// fetchNotificationsSince lädt UserNotifications mit created_at > since.
// Returnt zusätzlich den neuen Cursor (max created_at + 1µs, damit der
// nächste Aufruf strikt darüber liest und nichts doppelt sendet).
func (h *Handler) fetchNotificationsSince(ctx context.Context, orgID string, since time.Time) ([]UserNotification, time.Time, error) {
	rows, err := h.svc.db.Query(ctx, `
		SELECT id::text, title, body, type, module, created_at, read
		FROM user_notifications
		WHERE org_id = $1::uuid AND created_at > $2
		ORDER BY created_at ASC
		LIMIT 50`,
		orgID, since,
	)
	if err != nil {
		return nil, since, fmt.Errorf("query notifications: %w", err)
	}
	defer rows.Close()

	var out []UserNotification
	newCursor := since
	for rows.Next() {
		var n UserNotification
		if err := rows.Scan(&n.ID, &n.Title, &n.Body, &n.Type, &n.Module, &n.CreatedAt, &n.Read); err != nil {
			return nil, since, fmt.Errorf("scan notification: %w", err)
		}
		out = append(out, n)
		if n.CreatedAt.After(newCursor) {
			newCursor = n.CreatedAt
		}
	}
	if err := rows.Err(); err != nil {
		return nil, since, fmt.Errorf("iterate notifications: %w", err)
	}
	// Mikrosekunde drauf, damit der nächste Cursor strikt > ist und das
	// gleiche row nicht erneut liefert.
	if !newCursor.Equal(since) {
		newCursor = newCursor.Add(time.Microsecond)
	}
	return out, newCursor, nil
}

// PublishNotification publishes a wakeup signal on the Redis Pub/Sub channel
// for the given org. Connected SSE streams will flush immediately instead of
// waiting for the next safety poll. No-op when rdb is nil.
func PublishNotification(ctx context.Context, rdb *redis.Client, orgID string) {
	if rdb == nil {
		return
	}
	if err := rdb.Publish(ctx, notifyChannel(orgID), "1").Err(); err != nil {
		log.Warn().Err(err).Str("org_id", orgID).Msg("publish notification: redis error")
	}
}
