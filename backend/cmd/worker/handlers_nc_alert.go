// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0
// See LICENSE file in the project root for terms.

package main

import (
	"context"
	"fmt"

	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"

	"github.com/matharnica/vakt/internal/modules/vaktcomply"
	"github.com/matharnica/vakt/internal/shared/notify"
)

// taskEffectivenessCheckOverdueAlert is the Asynq task name for the daily
// overdue effectiveness-check alert job (S61-3).
const taskEffectivenessCheckOverdueAlert = "vaktcomply:effectiveness_check_overdue_alert"

// handleEffectivenessCheckOverdueAlert queries for all major_nc CAPAs whose
// effectiveness_check_date has passed without confirmation and sends an
// in-app notification per org.
func handleEffectivenessCheckOverdueAlert(pool *pgxpool.Pool) asynq.HandlerFunc {
	return func(ctx context.Context, _ *asynq.Task) error {
		repo := vaktcomply.NewRepository(pool)
		items, err := repo.ListOverdueEffectivenessChecks(ctx)
		if err != nil {
			return fmt.Errorf("effectiveness overdue alert: list: %w", err)
		}

		if len(items) == 0 {
			return nil
		}

		log.Info().Int("count", len(items)).Msg("effectiveness_check_overdue_alert: found overdue CAPAs")

		for _, item := range items {
			notify.Send(ctx, pool, item.OrgID,
				"Wirksamkeitsprüfung überfällig",
				fmt.Sprintf("Eine Major-NC (CAPA %s) hat das Prüfdatum überschritten und wurde noch nicht als wirksam bestätigt.", item.CAPAID),
				"warning",
				"vaktcomply",
			)
		}

		return nil
	}
}
