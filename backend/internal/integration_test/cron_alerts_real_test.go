//go:build integration

package integration_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go/modules/postgres"

	"github.com/matharnica/vakt/internal/modules/vaktcomply"
	"github.com/matharnica/vakt/internal/modules/vaktcomply/risk"
	shareddb "github.com/matharnica/vakt/internal/shared/db"
)

// TestCronAlerts pinnt vier Befunde an den taeglichen bzw. 5-minuetlichen
// Hintergrundjobs der Comply-Flaeche:
//
//	L3-01 — der DORA-Ampel-Cron laeuft alle fuenf Minuten und meldete jede
//	        ueberschrittene Frist bei JEDEM Lauf neu: 288 Laeufe am Tag mal drei
//	        Fristen = 864 Meldungen pro Tag und Vorfall.
//	L3-04 — die Abfrage der ueberfaelligen Wirksamkeitspruefungen filterte den
//	        CAPA-Status nicht. Eine seit 400 Tagen GESCHLOSSENE Major-NC loeste
//	        damit taeglich weiter Alarm aus.
//	L1-04 — ck_supplier_answers.cert_expiry_date hatte keinen Schreiber.
//	        FindCKExpiringCerts verlangt cert_expiry_date IS NOT NULL, die
//	        Warnung vor ablaufenden Lieferanten-Zertifikaten konnte also nie
//	        ausloesen.
//	L3-02 — dieselbe Abfrage hat keine untere Grenze. Ein vor drei Jahren
//	        abgelaufenes Zertifikat kam mit und wurde als "laeuft in 30 Tagen
//	        ab" gemeldet.
func TestCronAlerts(t *testing.T) {
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
		INSERT INTO organizations (name, slug) VALUES ('CronOrg', 'cronorg')
		RETURNING id::text`).Scan(&orgID))

	svc := vaktcomply.NewService(pool)

	countNotifs := func(notifType string) int {
		var n int
		require.NoError(t, pool.QueryRow(ctx,
			`SELECT COUNT(*) FROM user_notifications WHERE org_id = $1::uuid AND type = $2`,
			orgID, notifType).Scan(&n))
		return n
	}

	t.Run("L3-01 DORA-Ampel meldet je Frist genau einmal", func(t *testing.T) {
		// Ein Vorfall mit ueberschrittener 24h-Frist, nicht gemeldet.
		var incID string
		require.NoError(t, pool.QueryRow(ctx, `
			INSERT INTO ck_incidents (org_id, title, severity, status, incident_type, deadline_24h)
			VALUES ($1::uuid, 'Ausfall Zahlungsverkehr', 'high', 'open', 'ikt_dora', NOW() - INTERVAL '2 days')
			RETURNING id::text`, orgID).Scan(&incID))

		// Zwoelf Laeufe entsprechen einer Stunde des 5-Minuten-Crons.
		for range 12 {
			require.NoError(t, svc.UpdateDORADeadlineStatus(ctx, orgID))
		}

		n := countNotifs("dora_deadline_overdue")
		assert.LessOrEqual(t, n, 3,
			"zwoelf Cron-Laeufe erzeugten je Frist eine eigene Meldung — "+
				"hochgerechnet 864 pro Tag und Vorfall")
		assert.Positive(t, n, "die Meldung muss weiterhin kommen, nur nicht dauernd")
	})

	t.Run("L3-04 geschlossene Major-NC loest keinen Alarm mehr aus", func(t *testing.T) {
		// Zwei Major-NCs, beide mit ueberschrittener Wirksamkeitspruefung und
		// nie bestaetigter Wirksamkeit — eine offen, eine seit langem geschlossen.
		for _, c := range []struct{ title, status string }{
			{"offen und ueberfaellig", "open"},
			{"seit 400 Tagen geschlossen", "closed"},
		} {
			_, err := pool.Exec(ctx, `
				INSERT INTO ck_capas (org_id, source_type, source_id, title, status,
				                      nc_classification, effectiveness_check_date)
				VALUES ($1::uuid, 'audit', 'a-1', $2, $3, 'major_nc', CURRENT_DATE - 400)`,
				orgID, c.title, c.status)
			require.NoError(t, err)
		}

		items, err := risk.NewRepository(pool).ListOverdueEffectivenessChecks(ctx)
		require.NoError(t, err)
		assert.Len(t, items, 1,
			"die geschlossene Major-NC wurde taeglich mit angemahnt")
	})

	t.Run("L1-04/L3-02 Zertifikatswarnung", func(t *testing.T) {
		// Kette aufbauen: Lieferant -> Fragebogen -> Frage -> Bewertung -> Antwort.
		var supplierID, qnID, questionID, asmID string
		require.NoError(t, pool.QueryRow(ctx, `
			INSERT INTO ck_suppliers (org_id, name) VALUES ($1::uuid, 'Rechenzentrum Nord')
			RETURNING id::text`, orgID).Scan(&supplierID))
		require.NoError(t, pool.QueryRow(ctx, `
			INSERT INTO ck_questionnaires (org_id, name) VALUES ($1::uuid, 'ISO-Nachweis')
			RETURNING id::text`, orgID).Scan(&qnID))
		require.NoError(t, pool.QueryRow(ctx, `
			INSERT INTO ck_questionnaire_questions (questionnaire_id, question_text, question_type, order_idx)
			VALUES ($1::uuid, 'ISO-27001-Zertifikat vorhanden?', 'file_upload', 1)
			RETURNING id::text`, qnID).Scan(&questionID))
		require.NoError(t, pool.QueryRow(ctx, `
			INSERT INTO ck_supplier_assessments (org_id, supplier_id, questionnaire_id, status, token_hash, expires_at)
			VALUES ($1::uuid, $2::uuid, $3::uuid, 'submitted', 'test-token-hash', NOW() + INTERVAL '30 days')
			RETURNING id::text`, orgID, supplierID, qnID).Scan(&asmID))

		var answerID string
		require.NoError(t, pool.QueryRow(ctx, `
			INSERT INTO ck_supplier_answers (assessment_id, question_id, answer_text, file_url)
			VALUES ($1::uuid, $2::uuid, 'ja', '/files/iso.pdf')
			RETURNING id::text`, asmID, questionID).Scan(&answerID))

		// L1-04: vor der Pruefung gibt es kein Ablaufdatum, die Warnung kann
		// nicht ausloesen — das war der Dauerzustand.
		before, err := svc.FindExpiringCertificates(ctx, orgID, 30)
		require.NoError(t, err)
		require.Empty(t, before, "ohne gesetztes Ablaufdatum darf nichts gefunden werden")

		// Der Pruefende traegt das Ablaufdatum ein.
		soon := time.Now().UTC().AddDate(0, 0, 10)
		_, err = svc.ReviewAnswer(ctx, orgID, asmID, answerID, vaktcomply.ReviewAnswerInput{
			ReviewStatus:   "accepted",
			CertExpiryDate: &soon,
		})
		require.NoError(t, err)

		after, err := svc.FindExpiringCertificates(ctx, orgID, 30)
		require.NoError(t, err)
		require.Len(t, after, 1,
			"cert_expiry_date hatte keinen Schreiber — die Warnung konnte nie ausloesen")

		// Eine erneute Pruefung ohne Datumsangabe darf den Wert nicht loeschen.
		_, err = svc.ReviewAnswer(ctx, orgID, asmID, answerID, vaktcomply.ReviewAnswerInput{
			ReviewStatus: "accepted",
		})
		require.NoError(t, err)
		still, err := svc.FindExpiringCertificates(ctx, orgID, 30)
		require.NoError(t, err)
		assert.Len(t, still, 1, "ein erfasstes Ablaufdatum darf nicht verlorengehen")

		// L3-02: ein laengst abgelaufenes Zertifikat kommt weiterhin mit — die
		// Abfrage hat bewusst keine untere Grenze, damit es nicht unsichtbar
		// wird. Der Cron muss es aber als abgelaufen melden, nicht als
		// "laeuft in 30 Tagen ab". Diese Unterscheidung faellt im Handler; hier
		// wird nur festgehalten, dass beide Faelle in derselben Liste stehen
		// und unterscheidbar sind.
		longGone := time.Now().UTC().AddDate(-3, 0, 0)
		_, err = pool.Exec(ctx,
			`UPDATE ck_supplier_answers SET cert_expiry_date = $1 WHERE id = $2::uuid`,
			longGone, answerID)
		require.NoError(t, err)

		expired, err := svc.FindExpiringCertificates(ctx, orgID, 30)
		require.NoError(t, err)
		require.Len(t, expired, 1)
		assert.True(t, expired[0].CertExpiryDate.Before(time.Now().UTC()),
			"das Ablaufdatum liegt in der Vergangenheit — der Cron darf das nicht "+
				"als bevorstehend melden")
	})
}
