// Package notify provides two notification paths used by all Vakt modules.
//
// The Service type persists a notification to the notifications table and then
// enqueues an Asynq delivery task so a worker can fan it out over Slack,
// Teams, email, or a webhook. This path is used for user-configured alert
// channels and is retry-safe: if enqueue fails the DB record survives and can
// be swept by a background job.
//
// The package-level Send function writes directly to user_notifications for
// in-app display and returns whether that write succeeded.
//
// # Wie Aufrufer mit dem Fehler umgehen (R1-W4A-N1)
//
// Send gab frueher nichts zurueck: jeder Datenbankfehler wurde geloggt und
// verworfen, und kein Aufrufer konnte wissen, ob er gerade etwas zugestellt
// hat. Fuenfzehn Dateien haben danach einen Erfolg protokolliert, den sie
// nicht kennen konnten — und zwei haben zusaetzlich eine Marke gesetzt, die
// jeden weiteren Versuch ausschliesst. Ein Fehler, der niemanden erreicht,
// ist kein Fehler, sondern eine Luege.
//
// Der Rueckgabewert bricht bewusst NICHT den Geschaeftsvorgang ab. Ein
// Kommentar, ein Risiko, eine Datenpanne sind bereits gespeichert, wenn Send
// laeuft; eine fehlgeschlagene Benachrichtigung darf sie nicht zuruecknehmen.
// Es gelten drei Regeln, je nachdem, wofuer der Versand da ist:
//
//  1. Der Versand IST der Zweck und danach wird eine Marke gesetzt, die
//     kuenftige Versuche ausschliesst (reminder_sent_at, expiry_notified_at,
//     notified_warn_*): Bei einem Fehler wird die Marke NICHT gesetzt, damit
//     der naechste Lauf es erneut versucht. Sonst macht ein einmaliger
//     Ausfall aus einer verpassten Meldung eine dauerhaft unterdrueckte.
//  2. Der Versand begleitet einen bereits gespeicherten Geschaeftsvorgang:
//     Der Fehler bricht nichts ab, wird aber am Aufrufort mit dem
//     Geschaeftskontext geloggt — und es darf danach kein Erfolg
//     protokolliert werden.
//  3. Innerhalb von safego.Run: den Fehler zurueckgeben, safego loggt ihn.
//
// Send loggt einen Schreibfehler zusaetzlich selbst. Das ist Absicht: ein
// Aufrufer, der den Fehler bewusst mit `_ =` verwirft, soll die Meldung
// trotzdem in den Betriebslogs hinterlassen.
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
	"github.com/matharnica/vakt/internal/shared/redisopt"
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

// deliveryEnvelope is the Asynq task payload for a notification delivery job. It
// carries the notifications-row id alongside the Message so the worker handler
// can (a) deliver over the channel and (b) advance the exact DB row's status
// from 'pending' to 'sent'/'failed'. The row id must travel in the task because
// the notifications table stores no Asynq task id to correlate against.
type deliveryEnvelope struct {
	NotificationID string  `json:"notification_id"`
	Message        Message `json:"message"`
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
	// R1-14b-01: die Ableitung aus VAKT_REDIS_URL liegt in redisopt, nicht hier.
	// Sie stand frueher an sechs Stellen als Handarbeit, an vier davon ohne die
	// Datenbanknummer — und eine Struktur, die man sechsmal von Hand baut, baut
	// man beim siebten Mal wieder falsch.
	var redisURL string
	if cfg != nil {
		redisURL = cfg.RedisUrl
	}
	client := asynq.NewClient(redisopt.AsynqFromURL(redisURL))
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
	// Persist to notifications table and capture the row id so the delivery
	// task can advance this exact row's status.
	id, err := s.persist(ctx, msg)
	if err != nil {
		return fmt.Errorf("notify persist: %w", err)
	}

	// Enqueue delivery task. The payload is an envelope carrying the row id so
	// the worker handler can flip status='sent'/'failed' on the right row.
	payload, err := json.Marshal(deliveryEnvelope{NotificationID: id, Message: msg})
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

// SendOnce verhaelt sich wie Send, legt die Benachrichtigung aber nur an, wenn
// zu demselben dedupeKey noch keine existiert.
//
// L3-01: der DORA-Ampel-Cron laeuft alle fuenf Minuten und rief Send fuer jede
// ueberschrittene Frist erneut auf — 288 Laeufe am Tag mal drei Fristen sind 864
// identische Meldungen pro Tag und Vorfall. Eine Meldung, die 864-mal am Tag
// kommt, wird nicht gelesen; sie verdeckt die, die gelesen werden muss.
//
// Der Schluessel liegt in user_notifications.module. Dieselbe Doppelnutzung
// benutzt bereits der Framework-Meilenstein (CountCKFrameworkMilestoneNotifs
// mit "<frameworkID>:<threshold>"); die Spalte traegt sonst den Modulnamen und
// wird von keiner Abfrage als Fremdschluessel gelesen.
//
// Einfuegen und Pruefen stecken in EINER Anweisung. Zwei Cron-Laeufe, die sich
// ueberholen, koennen so hoechstens dann doppelt schreiben, wenn beide
// Anweisungen exakt gleichzeitig laufen — ohne Unique-Index bleibt das offen,
// und ein Unique-Index ueber (org_id, type, module) wuerde alle uebrigen
// Send-Aufrufer brechen, die sich module teilen.
// SendOnce gibt nil zurueck, wenn die Meldung geschrieben wurde ODER bereits
// vorlag — beides heisst „der Nutzer sieht sie". Nur ein Schreibfehler ist ein
// Fehler. Die Regeln aus dem Paketkommentar gelten unveraendert.
func SendOnce(ctx context.Context, db *pgxpool.Pool, orgID, title, body, notifType, dedupeKey string) error {
	sendCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	tag, err := db.Exec(sendCtx,
		`INSERT INTO user_notifications (org_id, title, body, type, module)
		 SELECT $1::uuid, $2, $3, $4, $5
		  WHERE NOT EXISTS (
		      SELECT 1 FROM user_notifications
		       WHERE org_id = $1::uuid AND type = $4 AND module = $5
		  )`,
		orgID, title, body, notifType, dedupeKey)
	if err != nil {
		log.Error().Err(err).Str("dedupe_key", dedupeKey).Msg("notify.SendOnce failed")
		return fmt.Errorf("notify.SendOnce: insert user notification: %w", err)
	}
	if tag.RowsAffected() == 0 {
		// Schon gemeldet — kein Fehler, aber auch kein SSE-Weckruf.
		return nil
	}
	if pubClient != nil {
		if perr := pubClient.Publish(sendCtx, "notify:"+orgID, "1").Err(); perr != nil {
			log.Warn().Err(perr).Str("org_id", orgID).Msg("notify.SendOnce: SSE publish failed")
		}
	}
	return nil
}

// Send inserts a single row into user_notifications for in-app display and
// returns nil exactly when that row was written. Siehe den Paketkommentar,
// wie Aufrufer mit dem Fehler umzugehen haben — kurz: er bricht den
// Geschaeftsvorgang nicht ab, aber er darf auch nicht als Erfolg
// protokolliert werden, und er verhindert jede Marke, die kuenftige Versuche
// ausschliesst.
//
// S124-6 (E2E-03): Send detaches from the caller's context. It is frequently
// fired at the tail of a request handler; if it ran on the request context, a
// client that already received its response (context canceled) would
// spuriously fail the notification write with "context canceled". The write is
// bounded by its own 5s timeout instead.
func Send(ctx context.Context, db *pgxpool.Pool, orgID, title, body, notifType, module string) error {
	sendCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	_, err := db.Exec(sendCtx,
		`INSERT INTO user_notifications (org_id, title, body, type, module)
		 VALUES ($1::uuid, $2, $3, $4, $5)`,
		orgID, title, body, notifType, module)
	if err != nil {
		log.Error().Err(err).Str("module", module).Msg("notify.Send failed")
		return fmt.Errorf("notify.Send: insert user notification: %w", err)
	}
	// S98-5: push a wakeup to open SSE streams. Channel key MUST match
	// dashboard.notifyChannel ("notify:<org_id>"). Best-effort — und bewusst
	// NICHT im Rueckgabewert: die Benachrichtigung steht bereits in der
	// Datenbank, die SSE-Sicherheitsabfrage holt sie binnen 30 s nach. Wer
	// den Publish-Fehler zurueckgaebe, liesse den Aufrufer eine zugestellte
	// Benachrichtigung als fehlgeschlagen behandeln — und im Fall 1 oben die
	// Marke nie setzen, also dieselbe Meldung endlos wiederholen.
	if pubClient != nil {
		if perr := pubClient.Publish(sendCtx, "notify:"+orgID, "1").Err(); perr != nil {
			log.Warn().Err(perr).Str("org_id", orgID).Msg("notify.Send: SSE publish failed")
		}
	}
	return nil
}

// persist inserts a pending notification row and returns its generated id so the
// caller can reference the exact row in the enqueued delivery task.
func (s *Service) persist(ctx context.Context, msg Message) (string, error) {
	payload, err := json.Marshal(msg)
	if err != nil {
		return "", fmt.Errorf("marshal notification payload: %w", err)
	}

	var id string
	err = s.db.QueryRow(ctx, `
		INSERT INTO notifications (org_id, type, channel, payload, status)
		VALUES ($1::uuid, $2, $3, $4::jsonb, 'pending')
		RETURNING id::text`,
		msg.OrgID, NotificationJobType, string(msg.Channel), string(payload),
	).Scan(&id)
	if err != nil {
		return "", fmt.Errorf("insert notification: %w", err)
	}
	return id, nil
}
