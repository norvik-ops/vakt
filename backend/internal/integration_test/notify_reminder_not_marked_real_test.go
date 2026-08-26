//go:build integration

// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package integration_test

import (
	"context"
	"testing"

	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/matharnica/vakt/internal/modules/vaktprivacy"
)

// TestDeletionReminder_FailedSendIsNotMarkedAndRetries ist die Regressionsprobe
// fuer R1-W4A-N1.
//
// CheckDeletionReminders waehlt Loesch-Erinnerungen ueber
// `reminder_sent_at IS NULL` aus. Vorher rief die Schleife notify.Send auf —
// eine Funktion ohne Rueckgabewert, die jeden Datenbankfehler verschluckte —,
// setzte danach mit `_, _ =` bedingungslos reminder_sent_at = NOW() und
// protokollierte „deletion reminder notification sent".
//
// Damit macht ein EINMALIGER Schreibfehler aus einer verpassten Erinnerung eine
// DAUERHAFT unterdrueckte: die Zeile traegt die Marke, die Auswahlabfrage
// ueberspringt sie ab sofort, und die Loeschfrist nach Art. 17 DSGVO laeuft
// unbemerkt ab. Der Log behauptet dabei, alles sei zugestellt.
//
// Die Probe erzwingt den Fehler echt — ein BEFORE-INSERT-Trigger auf
// user_notifications, der nur fuer die Org dieses Tests feuert. Kein Mock: der
// Defekt lebt ausschliesslich im Zusammenspiel von Auswahlabfrage, Schreibpfad
// und Markierung gegen echtes Postgres.
//
// Geprueft werden beide Haelften, und die zweite ist der eigentliche Defekt:
//
//	(a) ein Fehlschlag wird nicht als Erfolg verbucht — keine Meldung, keine Marke;
//	(b) der naechste Lauf versucht es ERNEUT und stellt dann zu.
func TestDeletionReminder_FailedSendIsNotMarkedAndRetries(t *testing.T) {
	pool, orgID, cleanup := bootPostgresWithOrg(t)
	defer cleanup()
	ctx := context.Background()

	// Eine Loeschung, die in 7 Tagen faellig ist: offen, noch nie gemeldet.
	var reminderID string
	require.NoError(t, pool.QueryRow(ctx, `
		INSERT INTO po_deletion_reminders (org_id, description, deletion_due_date)
		VALUES ($1::uuid, 'Bewerberdaten Q1', CURRENT_DATE + 7)
		RETURNING id::text`, orgID).Scan(&reminderID))

	svc := vaktprivacy.NewService(pool, asynq.RedisClientOpt{})

	// ── Lauf 1: der Versand scheitert ────────────────────────────────────────
	installNotifyFailureTrigger(t, pool, orgID)

	require.NoError(t, svc.CheckDeletionReminders(ctx),
		"ein fehlgeschlagener Versand darf den Cron-Lauf nicht abbrechen")

	assert.Equal(t, 0, countUserNotifications(t, pool, orgID),
		"(a) es wurde nichts geschrieben — genau das ist der Fehlerfall")

	assert.False(t, reminderMarked(t, pool, reminderID),
		"(b) KERN DES DEFEKTS: reminder_sent_at darf ohne bestaetigten Versand NICHT "+
			"gesetzt werden — sonst waehlt die Abfrage diese Erinnerung nie wieder aus "+
			"und die Art.-17-Loeschfrist verstreicht dauerhaft unbemerkt")

	// ── Lauf 2: der Versand geht wieder ──────────────────────────────────────
	removeNotifyFailureTrigger(t, pool)

	require.NoError(t, svc.CheckDeletionReminders(ctx))

	assert.Equal(t, 1, countUserNotifications(t, pool, orgID),
		"(b) der naechste Lauf muss es ERNEUT versuchen und dann zustellen")

	assert.True(t, reminderMarked(t, pool, reminderID),
		"nach bestaetigtem Versand wird die Marke gesetzt — sonst wiederholt sich die Meldung endlos")

	// ── Lauf 3: Grundverhalten unveraendert — keine Dopplung ─────────────────
	require.NoError(t, svc.CheckDeletionReminders(ctx))
	assert.Equal(t, 1, countUserNotifications(t, pool, orgID),
		"eine markierte Erinnerung darf nicht erneut gemeldet werden")
}

// installNotifyFailureTrigger laesst jeden Schreibvorgang auf
// user_notifications fuer GENAU diese Org fehlschlagen. Org-gebunden, damit die
// Probe nichts ausserhalb ihres eigenen Datenbestands beeinflusst.
func installNotifyFailureTrigger(t *testing.T, pool *pgxpool.Pool, orgID string) {
	t.Helper()
	ctx := context.Background()
	_, err := pool.Exec(ctx, `
		CREATE OR REPLACE FUNCTION notify_probe_fail() RETURNS trigger AS $$
		BEGIN
			RAISE EXCEPTION 'notify probe: simulierter Schreibfehler';
		END;
		$$ LANGUAGE plpgsql`)
	require.NoError(t, err)

	_, err = pool.Exec(ctx, `
		CREATE TRIGGER notify_probe_fail_trg
		BEFORE INSERT ON user_notifications
		FOR EACH ROW WHEN (NEW.org_id = '`+orgID+`'::uuid)
		EXECUTE FUNCTION notify_probe_fail()`)
	require.NoError(t, err)
}

func removeNotifyFailureTrigger(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	_, err := pool.Exec(context.Background(),
		`DROP TRIGGER IF EXISTS notify_probe_fail_trg ON user_notifications`)
	require.NoError(t, err)
}

func countUserNotifications(t *testing.T, pool *pgxpool.Pool, orgID string) int {
	t.Helper()
	var n int
	require.NoError(t, pool.QueryRow(context.Background(),
		`SELECT count(*) FROM user_notifications
		  WHERE org_id = $1::uuid AND type = 'deletion_reminder_due'`, orgID).Scan(&n))
	return n
}

// reminderMarked liest die Marke, die die Auswahlabfrage von
// CheckDeletionReminders per `reminder_sent_at IS NULL` auswertet.
func reminderMarked(t *testing.T, pool *pgxpool.Pool, reminderID string) bool {
	t.Helper()
	var marked bool
	// orgid-lint: global — Probe liest genau die eine Zeile, deren ID sie selbst erzeugt hat
	require.NoError(t, pool.QueryRow(context.Background(),
		`SELECT reminder_sent_at IS NOT NULL FROM po_deletion_reminders WHERE id = $1::uuid`,
		reminderID).Scan(&marked))
	return marked
}
