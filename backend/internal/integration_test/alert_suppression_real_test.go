//go:build integration

package integration_test

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go/modules/postgres"

	"github.com/matharnica/vakt/internal/services/alerting"
	shareddb "github.com/matharnica/vakt/internal/shared/db"
)

// TestAlertSuppressionFollowsDelivery pinnt R1-W7A-N1 (ADR-0083).
//
// Die drei taeglichen Alarm-Cronjobs (SLA-Frist ueberschritten, AVV abgelaufen,
// Betroffenenanfrage ueberfaellig) setzten ihre 24-Stunden-Sperre in
// notification_alert_state, sobald alerting.Fire zurueckkehrte. Fire hatte
// weder einen Rueckgabewert noch wartete es auf sein eigenes Auffaechern — es
// kehrte zurueck, BEVOR irgendetwas zugestellt war. Bei kaputtem Zustellweg
// kaufte jeder Lauf damit 24 Stunden Stille; bei dauerhaftem Fehler kam die
// Meldung nie wieder.
//
// Der eigentliche Schaden ist nicht die eine verpasste Meldung, sondern die
// dauerhafte Unterdrueckung. Der Test prueft deshalb beides: nach einem
// Fehlschlag steht keine Sperre in der Tabelle, UND der naechste Lauf stellt
// wirklich erneut zu (gezaehlt am Mailserver, nicht am Rueckgabewert).
func TestAlertSuppressionFollowsDelivery(t *testing.T) {
	if testing.Short() {
		t.Skip("integration: -short mode")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	pgC, err := postgres.Run(ctx,
		imagePostgres,
		postgres.WithDatabase("vakt_test"),
		postgres.WithUsername("vakt"),
		postgres.WithPassword("vakt"),
		postgres.WithSQLDriver("pgx"),
		postgres.BasicWaitStrategies(),
	)
	if err != nil {
		if strings.Contains(err.Error(), "permission denied") ||
			strings.Contains(err.Error(), "Cannot connect to the Docker daemon") {
			t.Skipf("integration: Docker unavailable (%v)", err)
		}
		t.Fatalf("postgres container: %v", err)
	}
	defer func() { _ = pgC.Terminate(ctx) }()

	dsn, err := pgC.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)
	require.NoError(t, shareddb.RunMigrations(dsn, migrationsDir(t)))

	pool, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)
	defer pool.Close()

	var orgID string
	require.NoError(t, pool.QueryRow(ctx, `
		INSERT INTO organizations (name, slug) VALUES ('AlarmOrg', 'alarmorg')
		RETURNING id::text`).Scan(&orgID))

	mail := startFakeSMTP(t)
	defer mail.Close()

	// 32 Byte Schluessel — AES-256 wie in Produktion.
	key := []byte("0123456789abcdef0123456789abcdef")
	svc := alerting.NewService(pool, key, alerting.SMTPConfig{
		Host: "127.0.0.1",
		Port: mail.Port(),
		From: "alarm@vakt.test",
	}).WithDispatchBudget(5 * time.Second)

	// Ein E-Mail-Kanal: der einzige Kanaltyp, den CreateChannel ohne
	// SSRF-Pruefung annimmt und der sich damit gegen einen lokalen Testserver
	// richten darf. Fuer die zu pruefende Invariante ist der Kanaltyp egal —
	// entscheidend ist nur, ob eine Zustellung bestaetigt wurde.
	event := alerting.EventFindingSLAOverdue
	_, _, err = svc.CreateChannel(ctx, orgID, alerting.CreateChannelInput{
		Name:   "Bereitschaft",
		Type:   "email",
		URL:    "bereitschaft@example.org",
		Events: []string{event},
	})
	require.NoError(t, err)

	// Wortgleich mit der NOT-EXISTS-Bedingung der drei Cronjobs
	// (cmd/worker/handlers_shared.go, handlers_secprivacy.go): genau dieser
	// Ausdruck entscheidet, ob eine Organisation still bleibt.
	suppressed := func() bool {
		var yes bool
		require.NoError(t, pool.QueryRow(ctx, `
			SELECT EXISTS (
			  SELECT 1 FROM notification_alert_state s
			  WHERE s.org_id = $1::uuid
			    AND s.event_type = $2
			    AND s.last_fired_at > NOW() - INTERVAL '24 hours'
			)`, orgID, event).Scan(&yes))
		return yes
	}
	countLog := func(status string) int {
		var n int
		require.NoError(t, pool.QueryRow(ctx,
			`SELECT COUNT(*) FROM alert_delivery_log WHERE org_id = $1::uuid AND event = $2 AND status = $3`,
			orgID, event, status).Scan(&n))
		return n
	}
	payload := map[string]any{"message": "One or more findings have exceeded their SLA deadline."}

	t.Run("Zustellung scheitert — keine Sperre, naechster Lauf versucht es erneut", func(t *testing.T) {
		mail.SetReject(true)

		res := svc.FireAndMark(ctx, orgID, event, payload)
		require.Equal(t, 1, res.Channels, "der Kanal muss gefunden worden sein — sonst prueft der Test nichts")
		assert.False(t, res.Delivered(), "der Mailserver hat abgelehnt, das darf nicht als zugestellt gelten")
		assert.Equal(t, 1, res.Failed)
		assert.False(t, suppressed(), "ohne Zustellung darf keine 24-Stunden-Sperre stehen")
		assert.Equal(t, 1, mail.Attempts(), "erster Zustellversuch")

		// Der eigentliche Defekt: bleibt die Sperre aus, muss der naechste
		// Lauf wirklich wieder zustellen — nicht nur formal duerfen.
		res = svc.FireAndMark(ctx, orgID, event, payload)
		assert.False(t, res.Delivered())
		assert.False(t, suppressed(), "auch nach dem zweiten Fehlschlag keine Sperre")
		assert.Equal(t, 2, mail.Attempts(), "der naechste Lauf muss erneut zustellen")
		assert.Equal(t, 2, countLog("failed"), "beide Fehlschlaege stehen im Zustellprotokoll")
	})

	t.Run("Mailserver wieder da — die unterdrueckte Meldung kommt an", func(t *testing.T) {
		mail.SetReject(false)

		res := svc.FireAndMark(ctx, orgID, event, payload)
		assert.True(t, res.Delivered(), "der Mailserver nimmt wieder an")
		assert.Equal(t, 1, res.Sent)
		assert.Equal(t, 3, mail.Attempts())
		assert.True(t, suppressed(), "nach bestaetigter Zustellung greift die Sperre wie bisher")
		assert.Equal(t, 1, countLog("sent"))
	})

	t.Run("kein Kanal konfiguriert ist keine Zustellung", func(t *testing.T) {
		var otherOrg string
		require.NoError(t, pool.QueryRow(ctx, `
			INSERT INTO organizations (name, slug) VALUES ('StummOrg', 'stummorg')
			RETURNING id::text`).Scan(&otherOrg))

		res := svc.FireAndMark(ctx, otherOrg, event, payload)
		assert.Equal(t, 0, res.Channels)
		assert.False(t, res.Delivered(), "niemand wurde benachrichtigt — das ist keine Zustellung")

		var n int
		require.NoError(t, pool.QueryRow(ctx,
			`SELECT COUNT(*) FROM notification_alert_state WHERE org_id = $1::uuid`, otherOrg).Scan(&n))
		assert.Zero(t, n, "ohne Empfaenger darf nichts als erledigt markiert werden")
	})
}

// fakeSMTP ist ein minimaler SMTP-Server fuer den Test. Er zaehlt
// Zustellversuche und kann sie auf Kommando ablehnen — beides brauchen wir,
// weil der Rueckgabewert des Dienstes genau das behaupten soll, was hier
// wirklich passiert ist.
type fakeSMTP struct {
	ln       net.Listener
	mu       sync.Mutex
	reject   bool
	attempts int
}

func startFakeSMTP(t *testing.T) *fakeSMTP {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	f := &fakeSMTP{ln: ln}
	go f.serve()
	return f
}

func (f *fakeSMTP) Port() string {
	_, port, _ := net.SplitHostPort(f.ln.Addr().String())
	return port
}

func (f *fakeSMTP) Close() { _ = f.ln.Close() }

func (f *fakeSMTP) SetReject(v bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.reject = v
}

func (f *fakeSMTP) Attempts() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.attempts
}

func (f *fakeSMTP) serve() {
	for {
		conn, err := f.ln.Accept()
		if err != nil {
			return // Listener geschlossen
		}
		go f.handle(conn)
	}
}

func (f *fakeSMTP) handle(conn net.Conn) {
	defer func() { _ = conn.Close() }()
	_ = conn.SetDeadline(time.Now().Add(20 * time.Second))

	f.mu.Lock()
	f.attempts++
	reject := f.reject
	f.mu.Unlock()

	w := bufio.NewWriter(conn)
	r := bufio.NewReader(conn)
	say := func(s string) bool {
		if _, err := fmt.Fprintf(w, "%s\r\n", s); err != nil {
			return false
		}
		return w.Flush() == nil
	}

	if !say("220 fake ESMTP") {
		return
	}
	inData := false
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			return
		}
		line = strings.TrimRight(line, "\r\n")

		if inData {
			if line == "." {
				inData = false
				if reject {
					// Ablehnung erst nach DATA: der Client hat vollstaendig
					// gesprochen, der Versand ist trotzdem gescheitert.
					if !say("550 rejected by policy") {
						return
					}
					continue
				}
				if !say("250 queued") {
					return
				}
			}
			continue
		}

		verb := strings.ToUpper(line)
		switch {
		case strings.HasPrefix(verb, "EHLO"), strings.HasPrefix(verb, "HELO"):
			if !say("250 fake") {
				return
			}
		case strings.HasPrefix(verb, "MAIL FROM"), strings.HasPrefix(verb, "RCPT TO"):
			if !say("250 ok") {
				return
			}
		case strings.HasPrefix(verb, "DATA"):
			inData = true
			if !say("354 go ahead") {
				return
			}
		case strings.HasPrefix(verb, "QUIT"):
			_ = say("221 bye")
			return
		case strings.HasPrefix(verb, "RSET"), strings.HasPrefix(verb, "NOOP"):
			if !say("250 ok") {
				return
			}
		default:
			if !say("502 not implemented") {
				return
			}
		}
	}
}
