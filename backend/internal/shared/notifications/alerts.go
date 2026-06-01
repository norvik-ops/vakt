package notifications

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"

	"github.com/matharnica/vakt/internal/shared/logsafe"
	"github.com/matharnica/vakt/internal/shared/notify"
)

// Notification type constants — must match the CHECK / UNIQUE key in notification_log.
const (
	NotifBreachWarning = "breach_72h_warning"
	NotifDSROverdue    = "dsr_overdue"
	NotifAVVExpiring   = "avv_expiring"
	NotifCCMFailed     = "ccm_check_failed"
)

// breachRow carries the data we need from the breach + org join.
type breachRow struct {
	ID           string
	Title        string
	DeadlineAt   time.Time
	DiscoveredAt time.Time
	OrgID        string
	OrgName      string
	DPOEmail     string
}

// dsrRow carries the data we need from po_dsr + org join.
type dsrRow struct {
	ID            string
	RequesterName string
	RequestType   string
	DueDate       time.Time
	OrgID         string
	DPOEmail      string
}

// avvRow carries the data we need from po_avvs + org join.
type avvRow struct {
	ID            string
	ProcessorName string
	ReviewDate    time.Time
	OrgID         string
	DPOEmail      string
}

// CheckBreachDeadlines finds open breaches whose authority_deadline_at is within
// 24 hours and sends a warning email to the org's DPO email address.
func CheckBreachDeadlines(ctx context.Context, db *pgxpool.Pool, m *Mailer) error {
	rows, err := db.Query(ctx, `
		SELECT b.id::text,
		       b.title,
		       b.authority_deadline_at,
		       b.discovered_at,
		       b.org_id::text,
		       o.name AS org_name,
		       COALESCE(o.dsr_dpo_email, '') AS dpo_email
		FROM   po_breaches b
		JOIN   organizations o ON o.id = b.org_id
		WHERE  b.status = 'open'
		  AND  b.authority_deadline_at BETWEEN now() AND now() + INTERVAL '24 hours'
		  AND  b.authority_notified_at IS NULL
	`)
	if err != nil {
		return fmt.Errorf("CheckBreachDeadlines: query: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var r breachRow
		if err := rows.Scan(
			&r.ID, &r.Title, &r.DeadlineAt, &r.DiscoveredAt,
			&r.OrgID, &r.OrgName, &r.DPOEmail,
		); err != nil {
			log.Error().Err(err).Msg("CheckBreachDeadlines: scan row")
			continue
		}
		if r.DPOEmail == "" {
			log.Debug().Str("breach_id", r.ID).Msg("CheckBreachDeadlines: no DPO email configured, skipping")
			continue
		}
		if alreadySent(ctx, db, r.OrgID, NotifBreachWarning, r.ID, r.DPOEmail) {
			continue
		}

		subject := "⚠ Datenpanne-Meldepflicht: Frist läuft in weniger als 24 Stunden ab"
		body := fmt.Sprintf(`Sehr geehrte Datenschutzbeauftragte/r,

die folgende Datenpanne erfordert eine Meldung an die Aufsichtsbehörde:

Titel:          %s
Entdeckt:       %s
Meldepflicht bis: %s

Gemäß Art. 33 DSGVO muss die Aufsichtsbehörde innerhalb von 72 Stunden
nach Bekanntwerden informiert werden.

Bitte melden Sie den Vorfall umgehend in Vakt unter Datenpannen > %s.

Diese E-Mail wurde automatisch von Vakt generiert.`,
			r.Title,
			r.DiscoveredAt.UTC().Format("02.01.2006 15:04 UTC"),
			r.DeadlineAt.UTC().Format("02.01.2006 15:04 UTC"),
			r.Title,
		)

		if err := m.Send(r.DPOEmail, subject, body); err != nil {
			log.Error().Err(err).Str("breach_id", r.ID).Str("to_redacted", logsafe.RedactEmail(r.DPOEmail)).Msg("CheckBreachDeadlines: send failed")
			continue
		}
		markSent(ctx, db, r.OrgID, NotifBreachWarning, r.ID, r.DPOEmail)
		log.Info().Str("breach_id", r.ID).Str("to_redacted", logsafe.RedactEmail(r.DPOEmail)).Msg("CheckBreachDeadlines: alert sent")
	}
	return rows.Err()
}

// CheckDSRDeadlines finds DSRs that are overdue (due_date < today) and still
// open or in_progress, then sends a warning email to the org's DPO address.
func CheckDSRDeadlines(ctx context.Context, db *pgxpool.Pool, m *Mailer) error {
	rows, err := db.Query(ctx, `
		SELECT d.id::text,
		       d.requester_name,
		       d.type,
		       d.due_date,
		       d.org_id::text,
		       COALESCE(o.dsr_dpo_email, '') AS dpo_email
		FROM   po_dsr d
		JOIN   organizations o ON o.id = d.org_id
		WHERE  d.status IN ('open', 'in_progress')
		  AND  d.due_date < CURRENT_DATE
	`)
	if err != nil {
		return fmt.Errorf("CheckDSRDeadlines: query: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var r dsrRow
		if err := rows.Scan(
			&r.ID, &r.RequesterName, &r.RequestType, &r.DueDate,
			&r.OrgID, &r.DPOEmail,
		); err != nil {
			log.Error().Err(err).Msg("CheckDSRDeadlines: scan row")
			continue
		}
		if r.DPOEmail == "" {
			log.Debug().Str("dsr_id", r.ID).Msg("CheckDSRDeadlines: no DPO email configured, skipping")
			continue
		}
		if alreadySent(ctx, db, r.OrgID, NotifDSROverdue, r.ID, r.DPOEmail) {
			continue
		}

		subject := fmt.Sprintf("⚠ Betroffenenanfrage überfällig: %s", r.RequesterName)
		body := fmt.Sprintf(`Die folgende Betroffenenanfrage (Art. 12 DSGVO) ist überfällig:

Antragsteller: %s
Anfragetyp:    %s
Fälligkeit:    %s (heute oder früher)

Die 30-Tage-Frist nach Art. 12 Abs. 3 DSGVO ist abgelaufen.
Bitte bearbeiten Sie die Anfrage umgehend in Vakt.

Diese E-Mail wurde automatisch von Vakt generiert.`,
			r.RequesterName,
			r.RequestType,
			r.DueDate.Format("02.01.2006"),
		)

		if err := m.Send(r.DPOEmail, subject, body); err != nil {
			log.Error().Err(err).Str("dsr_id", r.ID).Str("to_redacted", logsafe.RedactEmail(r.DPOEmail)).Msg("CheckDSRDeadlines: send failed")
			continue
		}
		markSent(ctx, db, r.OrgID, NotifDSROverdue, r.ID, r.DPOEmail)
		log.Info().Str("dsr_id", r.ID).Str("to_redacted", logsafe.RedactEmail(r.DPOEmail)).Msg("CheckDSRDeadlines: alert sent")
	}
	return rows.Err()
}

// CheckAVVExpiry finds AVVs expiring within 30 days and sends a warning email.
func CheckAVVExpiry(ctx context.Context, db *pgxpool.Pool, m *Mailer) error {
	rows, err := db.Query(ctx, `
		SELECT a.id::text,
		       a.processor_name,
		       a.review_date,
		       a.org_id::text,
		       COALESCE(o.dsr_dpo_email, '') AS dpo_email
		FROM   po_avvs a
		JOIN   organizations o ON o.id = a.org_id
		WHERE  a.status = 'active'
		  AND  a.review_date IS NOT NULL
		  AND  a.review_date BETWEEN CURRENT_DATE AND CURRENT_DATE + INTERVAL '30 days'
	`)
	if err != nil {
		return fmt.Errorf("CheckAVVExpiry: query: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var r avvRow
		if err := rows.Scan(
			&r.ID, &r.ProcessorName, &r.ReviewDate,
			&r.OrgID, &r.DPOEmail,
		); err != nil {
			log.Error().Err(err).Msg("CheckAVVExpiry: scan row")
			continue
		}
		if r.DPOEmail == "" {
			log.Debug().Str("avv_id", r.ID).Msg("CheckAVVExpiry: no DPO email configured, skipping")
			continue
		}
		if alreadySent(ctx, db, r.OrgID, NotifAVVExpiring, r.ID, r.DPOEmail) {
			continue
		}

		daysLeft := int(time.Until(r.ReviewDate.UTC()).Hours() / 24)
		subject := fmt.Sprintf("⚠ Auftragsverarbeitungsvertrag läuft ab: %s", r.ProcessorName)
		body := fmt.Sprintf(`Der folgende Auftragsverarbeitungsvertrag (Art. 28 DSGVO) läuft bald ab:

Auftragsverarbeiter:  %s
Überprüfungsdatum:    %s
Verbleibende Tage:    %d

Bitte erneuern oder überprüfen Sie den AVV rechtzeitig in Vakt unter
Datenschutz > Auftragsverarbeitungsverträge.

Diese E-Mail wurde automatisch von Vakt generiert.`,
			r.ProcessorName,
			r.ReviewDate.Format("02.01.2006"),
			daysLeft,
		)

		if err := m.Send(r.DPOEmail, subject, body); err != nil {
			log.Error().Err(err).Str("avv_id", r.ID).Str("to_redacted", logsafe.RedactEmail(r.DPOEmail)).Msg("CheckAVVExpiry: send failed")
			continue
		}
		markSent(ctx, db, r.OrgID, NotifAVVExpiring, r.ID, r.DPOEmail)
		log.Info().Str("avv_id", r.ID).Str("to_redacted", logsafe.RedactEmail(r.DPOEmail)).Msg("CheckAVVExpiry: alert sent")
	}
	return rows.Err()
}

// CheckCCMFailures finds CCM checks whose last_status is 'fail' and sends a
// warning email once per failure cycle (deduplication via notification_log).
// Note: this function is a no-op if the ck_ccm_checks table does not exist yet.
func CheckCCMFailures(ctx context.Context, db *pgxpool.Pool, m *Mailer) error {
	// Check whether the table exists before querying — it may not be deployed yet.
	var tableExists bool
	err := db.QueryRow(ctx, `
		SELECT EXISTS (
		    SELECT 1 FROM information_schema.tables
		    WHERE table_schema = 'public' AND table_name = 'ck_ccm_checks'
		)
	`).Scan(&tableExists)
	if err != nil || !tableExists {
		return nil
	}

	rows, err := db.Query(ctx, `
		SELECT c.id::text,
		       c.name,
		       COALESCE(c.last_output, '') AS last_output,
		       c.org_id::text,
		       COALESCE(o.dsr_dpo_email, '') AS dpo_email
		FROM   ck_ccm_checks c
		JOIN   organizations o ON o.id = c.org_id
		WHERE  c.last_status = 'fail'
		  AND  c.enabled = true
	`)
	if err != nil {
		return fmt.Errorf("CheckCCMFailures: query: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var (
			id, name, lastOutput, orgID, dpoEmail string
		)
		if err := rows.Scan(&id, &name, &lastOutput, &orgID, &dpoEmail); err != nil {
			log.Error().Err(err).Msg("CheckCCMFailures: scan row")
			continue
		}
		if dpoEmail == "" {
			log.Debug().Str("ccm_check_id", id).Msg("CheckCCMFailures: no DPO email configured, skipping")
			continue
		}
		if alreadySent(ctx, db, orgID, NotifCCMFailed, id, dpoEmail) {
			continue
		}

		subject := fmt.Sprintf("⚠ Compliance-Prüfung fehlgeschlagen: %s", name)
		body := fmt.Sprintf(`Die folgende automatische Compliance-Prüfung ist fehlgeschlagen:

Prüfung: %s
Status:  fehlgeschlagen

Details:
%s

Bitte überprüfen Sie den Befund umgehend in Vakt unter
Compliance > Automatische Prüfungen.

Diese E-Mail wurde automatisch von Vakt generiert.`,
			name, lastOutput,
		)

		if err := m.Send(dpoEmail, subject, body); err != nil {
			log.Error().Err(err).Str("ccm_check_id", id).Str("to_redacted", logsafe.RedactEmail(dpoEmail)).Msg("CheckCCMFailures: send failed")
			continue
		}
		markSent(ctx, db, orgID, NotifCCMFailed, id, dpoEmail)
		log.Info().Str("ccm_check_id", id).Str("to_redacted", logsafe.RedactEmail(dpoEmail)).Msg("CheckCCMFailures: alert sent")
	}
	return rows.Err()
}

// CheckCertificationDeadlines finds upcoming audit milestones in ck_audit_milestones
// within the next 30 days and sends in-app notifications to the relevant orgs.
// If the ck_audit_milestones table does not exist the function is a no-op.
func CheckCertificationDeadlines(ctx context.Context, db *pgxpool.Pool, _ *Mailer) error {
	// Guard: skip gracefully if the table does not exist yet.
	var tableExists bool
	if err := db.QueryRow(ctx, `
		SELECT EXISTS (
		    SELECT 1 FROM information_schema.tables
		    WHERE table_schema = 'public' AND table_name = 'ck_audit_milestones'
		)
	`).Scan(&tableExists); err != nil || !tableExists {
		if err != nil {
			log.Debug().Err(err).Msg("CheckCertificationDeadlines: table-existence check failed, skipping")
		} else {
			log.Debug().Msg("CheckCertificationDeadlines: ck_audit_milestones does not exist, skipping")
		}
		return nil
	}

	// Step 1: find distinct orgs with approaching milestones.
	orgRows, err := db.Query(ctx, `
		SELECT DISTINCT org_id::text FROM ck_audit_milestones
		WHERE milestone_date BETWEEN NOW() AND NOW() + INTERVAL '30 days'
		  AND status != 'completed'
	`)
	if err != nil {
		return fmt.Errorf("CheckCertificationDeadlines: org query: %w", err)
	}
	defer orgRows.Close()

	var orgIDs []string
	for orgRows.Next() {
		var id string
		if err := orgRows.Scan(&id); err != nil {
			log.Error().Err(err).Msg("CheckCertificationDeadlines: scan org_id")
			continue
		}
		orgIDs = append(orgIDs, id)
	}
	if err := orgRows.Err(); err != nil {
		return fmt.Errorf("CheckCertificationDeadlines: org rows: %w", err)
	}

	// Step 2: for each org, load upcoming milestones and send in-app notifications.
	for _, orgID := range orgIDs {
		milestoneRows, err := db.Query(ctx, `
			SELECT title, milestone_date, (milestone_date - NOW()::date)::int AS days_until
			FROM ck_audit_milestones
			WHERE org_id = $1::uuid
			  AND milestone_date BETWEEN NOW() AND NOW() + INTERVAL '30 days'
			  AND status != 'completed'
			ORDER BY milestone_date
		`, orgID)
		if err != nil {
			log.Error().Err(err).Str("org_id", orgID).Msg("CheckCertificationDeadlines: milestone query")
			continue
		}

		for milestoneRows.Next() {
			var (
				title      string
				targetDate time.Time
				daysUntil  int
			)
			if err := milestoneRows.Scan(&title, &targetDate, &daysUntil); err != nil {
				log.Error().Err(err).Msg("CheckCertificationDeadlines: scan milestone")
				continue
			}

			notifType := "info"
			if daysUntil <= 14 {
				notifType = "warning"
			}

			notifyTitle := fmt.Sprintf("Zertifizierungs-Deadline in %d Tagen", daysUntil)
			notifyBody := fmt.Sprintf("Milestone \"%s\" ist am %s fällig.",
				title, targetDate.Format("02.01.2006"))

			notify.Send(ctx, db, orgID, notifyTitle, notifyBody, notifType, "vaktcomply")
			log.Info().Str("org_id", orgID).Str("milestone", title).Int("days_until", daysUntil).
				Msg("CheckCertificationDeadlines: in-app notification sent")
		}
		milestoneRows.Close()
		if err := milestoneRows.Err(); err != nil {
			log.Error().Err(err).Str("org_id", orgID).Msg("CheckCertificationDeadlines: milestone rows error")
		}
	}
	return nil
}

// alreadySent returns true if a notification for the given (org, type, resource, email)
// tuple already exists in notification_log. Prevents duplicate sends across daily runs.
func alreadySent(ctx context.Context, db *pgxpool.Pool, orgID, notifType, resourceID, email string) bool {
	var exists bool
	err := db.QueryRow(ctx, `
		SELECT EXISTS (
		    SELECT 1 FROM notification_log
		    WHERE org_id          = $1::uuid
		      AND notification_type = $2
		      AND resource_id     = $3
		      AND recipient_email = $4
		)
	`, orgID, notifType, resourceID, email).Scan(&exists)
	if err != nil {
		log.Error().Err(err).Msg("alreadySent: query failed")
		return false // send on error to avoid silent drops
	}
	return exists
}

// markSent inserts a row into notification_log. Uses ON CONFLICT DO NOTHING
// so concurrent workers cannot produce duplicates.
func markSent(ctx context.Context, db *pgxpool.Pool, orgID, notifType, resourceID, email string) {
	_, err := db.Exec(ctx, `
		INSERT INTO notification_log (org_id, notification_type, resource_id, recipient_email)
		VALUES ($1::uuid, $2, $3, $4)
		ON CONFLICT (org_id, notification_type, resource_id, recipient_email) DO NOTHING
	`, orgID, notifType, resourceID, email)
	if err != nil {
		log.Error().Err(err).
			Str("org_id", orgID).
			Str("type", notifType).
			Str("resource_id", resourceID).
			Msg("markSent: insert failed")
	}
}
