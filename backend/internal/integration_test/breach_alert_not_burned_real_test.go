//go:build integration

package integration_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go/modules/postgres"

	"github.com/matharnica/vakt/internal/config"
	shareddb "github.com/matharnica/vakt/internal/shared/db"
	"github.com/matharnica/vakt/internal/shared/notifications"
)

// TestBreachAlertIsNotBurnedWithoutSMTP pinnt R1-W9C-N1.
//
// Mailer.Send gab nil zurueck, wenn kein SMTP-Server eingerichtet war —
// "graceful no-op". Der Aufrufer in alerts.go liest nil als "zugestellt" und
// ruft danach markSent auf, das eine Zeile mit EINDEUTIGEM SCHLUESSEL UND OHNE
// ABLAUF in notification_log schreibt. alreadySent ueberspringt den Vorgang
// danach fuer immer.
//
// Fuer eine Instanz ohne SMTP hiess das: die Art.-33-Warnung zur Meldefrist
// einer Datenpanne galt als erledigt, ohne dass sie je jemand gesehen hat —
// und sie ging auch dann nicht mehr hinaus, als SMTP spaeter eingerichtet
// wurde. Dasselbe fuer ueberfaellige Betroffenenanfragen, ablaufende AVV und
// gescheiterte Compliance-Pruefungen.
//
// Der Test prueft deshalb nicht den Rueckgabewert, sondern die FOLGE:
//
//	Phase 1  ohne SMTP  -> notification_log bleibt LEER (nichts verbrannt)
//	Phase 2  mit SMTP   -> die Mail geht raus UND wird jetzt markiert
//	Phase 3  erneut     -> keine zweite Mail (die Sperre wirkt weiterhin)
//
// Phase 2 ist der eigentliche Beweis: die Meldung ueberlebt die Zeit ohne
// SMTP. Phase 3 belegt, dass der Fix die Unterdrueckung nicht kaputtgemacht
// hat — ohne sie wuerde jeder Lauf erneut mailen.
func TestBreachAlertIsNotBurnedWithoutSMTP(t *testing.T) {
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
		INSERT INTO organizations (name, slug, dsr_dpo_email)
		VALUES ('BreachOrg', 'breachorg', 'dpo@example.test')
		RETURNING id::text`).Scan(&orgID))

	// Eine offene Datenpanne, deren Meldefrist innerhalb der naechsten
	// 24 Stunden ablaeuft — genau das Fenster, das CheckBreachDeadlines liest.
	var breachID string
	require.NoError(t, pool.QueryRow(ctx, `
		INSERT INTO po_breaches (org_id, title, description, discovered_at, authority_deadline_at, status)
		VALUES ($1::uuid, 'Laptop verloren', 'Unverschluesselt', now() - INTERVAL '48 hours',
		        now() + INTERVAL '6 hours', 'open')
		RETURNING id::text`, orgID).Scan(&breachID))

	countLog := func() int {
		var n int
		require.NoError(t, pool.QueryRow(ctx,
			`SELECT count(*) FROM notification_log WHERE org_id = $1::uuid`, orgID).Scan(&n))
		return n
	}

	// ── Phase 1: kein SMTP eingerichtet ──────────────────────────────────
	// Der Lauf darf NICHTS markieren. Vor dem Fix stand hier eine Zeile.
	require.NoError(t, notifications.CheckBreachDeadlines(ctx, pool,
		notifications.NewMailer(&config.Config{})))
	require.Equal(t, 0, countLog(),
		"ohne SMTP wurde nichts gesendet — es darf auch nichts als gesendet vermerkt sein, "+
			"sonst ist die Art.-33-Warnung endgueltig verbrannt")

	// ── Phase 2: SMTP eingerichtet, die Meldung muss nachtraeglich rausgehen ─
	mail := startFakeSMTP(t)
	defer mail.Close()
	cfg := &config.Config{
		SMTPHost: "127.0.0.1",
		SMTPPort: mail.Port(),
		SMTPFrom: "vakt@example.test",
	}
	require.NoError(t, notifications.CheckBreachDeadlines(ctx, pool, notifications.NewMailer(cfg)))
	require.Equal(t, 1, mail.Attempts(),
		"die Warnung muss hinausgehen, sobald SMTP da ist — sie war in Phase 1 nur aufgeschoben")
	require.Equal(t, 1, countLog(), "jetzt — und erst jetzt — darf sie als gesendet gelten")

	// ── Phase 3: die Sperre wirkt weiterhin ──────────────────────────────
	require.NoError(t, notifications.CheckBreachDeadlines(ctx, pool, notifications.NewMailer(cfg)))
	require.Equal(t, 1, mail.Attempts(), "kein zweiter Versand fuer dieselbe Datenpanne")
	require.Equal(t, 1, countLog(), "und kein zweiter Eintrag")
}

// TestMailerReportsNotConfigured haelt die Wurzel fest: "nicht gesendet" darf
// nicht als nil zurueckkommen. Der Aufrufer kann den Unterschied sonst nicht
// sehen — und genau darauf beruhte der Defekt.
func TestMailerReportsNotConfigured(t *testing.T) {
	for _, tc := range []struct {
		name string
		cfg  *config.Config
	}{
		{"kein Config-Objekt", nil},
		{"leerer Host", &config.Config{}},
		{"localhost gilt als nicht eingerichtet", &config.Config{SMTPHost: "localhost"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := notifications.NewMailer(tc.cfg).Send("dpo@example.test", "Betreff", "Text")
			require.ErrorIs(t, err, notifications.ErrNotConfigured,
				"ein nil hier laesst den Aufrufer 'zugestellt' annehmen und die Meldung verbrennen")
		})
	}
}
