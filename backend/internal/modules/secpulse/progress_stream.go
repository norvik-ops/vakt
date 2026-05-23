package secpulse

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"
	"go.opentelemetry.io/otel"
)

// ProgressEvent ist ein Live-Update über einen laufenden Scan.
// Wird vom Worker via Redis-Pub/Sub publiziert und vom API-Handler an
// verbundene SSE-Clients weitergereicht.
//
// Sprint 17 / S17-2.
type ProgressEvent struct {
	ScanID    string    `json:"scan_id"`
	Phase     string    `json:"phase"`             // "started" | "fetching" | "scanning" | "parsing" | "finished" | "failed"
	Percent   int       `json:"percent,omitempty"` // 0-100, optional
	Message   string    `json:"message,omitempty"` // human-readable status, optional
	Timestamp time.Time `json:"ts"`
}

// progressChannel ist der Pub/Sub-Channel-Name pro Scan. Bewusst pro
// scan_id, damit Redis nur an verbundene Subscriber dieses Scans pushed
// (statt fan-out an alle).
func progressChannel(scanID string) string {
	return "scan:progress:" + scanID
}

// PublishProgress wird vom Worker bei jedem Phase-Übergang aufgerufen.
// Best-Effort: Redis-Fehler bricht den Scan nicht ab, nur Warn-Log.
func PublishProgress(ctx context.Context, rdb *redis.Client, evt ProgressEvent) {
	if rdb == nil || evt.ScanID == "" {
		return
	}
	if evt.Timestamp.IsZero() {
		evt.Timestamp = time.Now().UTC()
	}
	payload, err := json.Marshal(evt)
	if err != nil {
		log.Warn().Err(err).Str("scan_id", evt.ScanID).Msg("secpulse.progress: marshal failed")
		return
	}
	if err := rdb.Publish(ctx, progressChannel(evt.ScanID), payload).Err(); err != nil {
		log.Warn().Err(err).Str("scan_id", evt.ScanID).Msg("secpulse.progress: publish failed")
	}
}

// StreamScanProgress ist der SSE-Handler. Subscribed pro Request auf den
// Redis-Channel des angeforderten Scans, forwarded jedes Event als
// "data: {...}\n\n"-Frame. Beim Scan-Finish (phase=finished|failed) wird
// "data: [DONE]\n\n" gesendet + Connection geschlossen.
//
// Heartbeat alle 30 s ohne Daten als "event: ping\n", damit Reverse-Proxies
// die Connection nicht idle-killen.
//
// nginx-Pflicht: `X-Accel-Buffering: no` ist als Response-Header gesetzt;
// reverse-proxy.md im Wiki dokumentiert den nginx-Location-Block.
func (h *Handler) StreamScanProgress(c echo.Context) error {
	orgID, _ := c.Get("org_id").(string)
	if orgID == "" {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
	}
	scanID := c.Param("id")
	if scanID == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "scan id required"})
	}

	// Sicherheits-Check: Scan muss zur Org des Users gehören. Sonst kann ein
	// Angreifer mit gestohlener Cookie einen Scan einer anderen Org streamen.
	ownerOrgID, err := h.service.GetScanOrgID(c.Request().Context(), scanID)
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "scan not found"})
	}
	if ownerOrgID != orgID {
		// Bewusst gleiche Response wie not-found — verhindert Org-ID-Enumeration.
		return c.JSON(http.StatusNotFound, map[string]string{"error": "scan not found"})
	}

	if h.service.rdb == nil {
		return c.JSON(http.StatusServiceUnavailable, map[string]string{
			"error": "progress streaming requires Redis — set VAKT_REDIS_URL",
		})
	}

	resp := c.Response()
	resp.Header().Set(echo.HeaderContentType, "text/event-stream")
	resp.Header().Set("Cache-Control", "no-cache")
	resp.Header().Set("Connection", "keep-alive")
	resp.Header().Set("X-Accel-Buffering", "no")
	resp.WriteHeader(http.StatusOK)

	tracer := otel.Tracer("vakt.secpulse.scan.progress.stream")
	streamCtx, span := tracer.Start(c.Request().Context(), "scan.progress.stream")
	defer span.End()

	pubsub := h.service.rdb.Subscribe(streamCtx, progressChannel(scanID))
	defer pubsub.Close()
	msgCh := pubsub.Channel()

	heartbeat := time.NewTicker(30 * time.Second)
	defer heartbeat.Stop()

	for {
		select {
		case <-streamCtx.Done():
			return nil

		case msg, ok := <-msgCh:
			if !ok {
				return nil
			}
			if _, werr := fmt.Fprintf(resp.Writer, "data: %s\n\n", msg.Payload); werr != nil {
				return nil
			}
			resp.Flush()
			// Wenn Payload "phase":"finished" oder "phase":"failed" enthält,
			// terminiere den Stream. Wir matchen JSON-String-Substring statt
			// Full-Parse, da Payload immer gut-geformt ist (siehe
			// PublishProgress oben).
			if containsPhaseDone(msg.Payload) {
				_, _ = fmt.Fprint(resp.Writer, "data: [DONE]\n\n")
				resp.Flush()
				return nil
			}

		case <-heartbeat.C:
			if _, werr := fmt.Fprint(resp.Writer, "event: ping\ndata: {}\n\n"); werr != nil {
				return nil
			}
			resp.Flush()
		}
	}
}

func containsPhaseDone(payload string) bool {
	return contains(payload, `"phase":"finished"`) || contains(payload, `"phase":"failed"`)
}

// contains ist ein Mini-Helper ohne strings.Contains-Import (würden wir
// strings nutzen, müssten wir auch sortieren; ein einfacher Substring-Check
// reicht).
func contains(s, sub string) bool {
	if len(sub) == 0 {
		return true
	}
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
