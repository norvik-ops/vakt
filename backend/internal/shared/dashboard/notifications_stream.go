package dashboard

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog/log"
	"go.opentelemetry.io/otel"
)

// StreamNotifications ist der SSE-Endpoint, der neue Notifications für die
// aktive Org pushed.
//
// Sprint 17 / S17-1. Pattern: server-side poll-and-push.
//   - Alle pollIntervalSec Sekunden: lade neue Notifications aus user_notifications,
//     deren created_at > letzter gesendeter Cursor liegt
//   - Pro neue Row: emit "data: {...}\n\n"-Frame mit JSON-Body
//   - Alle heartbeatSec Sekunden ohne neue Daten: emit "event: ping\ndata: {}\n\n"
//     (verhindert nginx-Idle-Timeout)
//
// Warum kein Postgres LISTEN/NOTIFY: jeder Listen würde eine dedizierte
// Postgres-Connection halten. 100 aktive User = 100 Idle-Connections am
// Pool-Limit. Server-side-poll skaliert besser, kostet 1 Query/2s/User —
// das ist im Cache-Pfad eines kleinen Index trivial. Für > 1000 gleichzeitige
// User wäre LISTEN-via-Single-Dispatcher der nächste Schritt (Sprint 22+).
//
// nginx-Anforderung: `X-Accel-Buffering: no` ist gesetzt; siehe
// docs/wiki/reverse-proxy.md für die nginx-Location-Block-Empfehlung.
//
// ADR-0019: nutzt das gleiche SSE-Pattern wie der AI-Streaming-Endpoint.
const (
	notificationStreamPollInterval = 2 * time.Second
	notificationStreamHeartbeat    = 30 * time.Second
)

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

	// Initialer Snapshot wird nicht über den Stream geliefert — der Client
	// holt sich /notifications einmal via GET, dann hängt er sich an den
	// Stream für Deltas. cursor = jetzt → wir streamen nur was DANACH kommt.
	cursor := time.Now().UTC()

	pollTicker := time.NewTicker(notificationStreamPollInterval)
	defer pollTicker.Stop()
	heartbeatTicker := time.NewTicker(notificationStreamHeartbeat)
	defer heartbeatTicker.Stop()

	for {
		select {
		case <-streamCtx.Done():
			// Client disconnect oder Server-Shutdown — kontrollierter Exit.
			return nil

		case <-pollTicker.C:
			items, newCursor, err := h.fetchNotificationsSince(streamCtx, orgID, cursor)
			if err != nil {
				log.Warn().Err(err).Str("org_id", orgID).Msg("notification-stream poll failed")
				continue
			}
			for _, item := range items {
				payload, err := json.Marshal(item)
				if err != nil {
					continue
				}
				if _, werr := fmt.Fprintf(resp.Writer, "data: %s\n\n", payload); werr != nil {
					return nil
				}
			}
			if len(items) > 0 {
				resp.Flush()
				cursor = newCursor
			}

		case <-heartbeatTicker.C:
			if _, werr := fmt.Fprint(resp.Writer, "event: ping\ndata: {}\n\n"); werr != nil {
				return nil
			}
			resp.Flush()
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
