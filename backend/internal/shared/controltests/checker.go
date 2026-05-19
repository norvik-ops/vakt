// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0
// See LICENSE file in the project root for terms.

// Package controltests creates CAPAs for controls whose test interval has elapsed.
package controltests

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"
)

// CheckOverdueControlTests queries all controls with a test_interval_days where
// next_test_due_at is in the past and no open CAPA exists yet, then creates CAPAs.
func CheckOverdueControlTests(ctx context.Context, db *pgxpool.Pool) error {
	type overdueControl struct {
		OrgID     string
		ControlID string
		Title     string
		DueAt     time.Time
	}

	rows, err := db.Query(ctx, `
        SELECT c.org_id::text, c.id::text, c.title, c.next_test_due_at
        FROM ck_controls c
        WHERE c.next_test_due_at IS NOT NULL
          AND c.next_test_due_at < NOW()
          AND c.manual_status != 'not_applicable'
          AND NOT EXISTS (
              SELECT 1 FROM ck_capas ca
              WHERE ca.org_id = c.org_id
                AND ca.source_type = 'control_test'
                AND ca.source_id = c.id::text
                AND ca.status IN ('open', 'in_progress')
          )
    `)
	if err != nil {
		return fmt.Errorf("controltests: query overdue: %w", err)
	}
	defer rows.Close()

	var overdue []overdueControl
	for rows.Next() {
		var o overdueControl
		if err := rows.Scan(&o.OrgID, &o.ControlID, &o.Title, &o.DueAt); err != nil {
			log.Error().Err(err).Msg("controltests: scan row")
			continue
		}
		overdue = append(overdue, o)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("controltests: rows: %w", err)
	}

	created := 0
	for _, o := range overdue {
		dueDate := time.Now().UTC().Add(14 * 24 * time.Hour)
		_, err := db.Exec(ctx, `
            INSERT INTO ck_capas (org_id, title, description, source_type, source_id, status, due_date, priority)
            VALUES ($1::uuid, $2, $3, 'control_test', $4, 'open', $5, 'medium')
            ON CONFLICT DO NOTHING
        `,
			o.OrgID,
			fmt.Sprintf("Kontrolle testen: %s", o.Title),
			fmt.Sprintf("Die Kontrolle '%s' wurde seit mehr als %s nicht getestet. Testnachweis erforderlich.", o.Title, o.DueAt.Format("2006-01-02")),
			o.ControlID,
			dueDate,
		)
		if err != nil {
			log.Error().Err(err).Str("control_id", o.ControlID).Msg("controltests: create capa")
			continue
		}
		created++
	}
	log.Info().Int("created_capas", created).Msg("controltests: check complete")
	return nil
}
