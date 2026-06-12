// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package dashboard

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"
	"golang.org/x/sync/errgroup"
)

// Service encapsulates all database access for the dashboard package.
// Handlers call Service methods exclusively — no direct pgxpool usage in handlers.
type Service struct {
	db *pgxpool.Pool
}

// NewService creates a Service backed by the provided connection pool.
func NewService(db *pgxpool.Pool) *Service {
	return &Service{db: db}
}

// ScoreInputs holds the raw counts used to compute the security health score.
type ScoreInputs struct {
	Cfg         ScoreConfig
	CritCount   int64
	HighCount   int64
	BreachCount int64
	FwCount     int64
}

// LoadScoreConfig loads the org's score weights from the DB, returning defaults on no-row.
func (s *Service) LoadScoreConfig(ctx context.Context, orgID string) (ScoreConfig, error) {
	var cfg ScoreConfig
	err := s.db.QueryRow(ctx,
		`SELECT base_score, crit_penalty, crit_penalty_cap, high_penalty, high_penalty_cap,
		        breach_penalty, breach_penalty_cap, fw_bonus, fw_bonus_cap
		   FROM score_config WHERE org_id=$1::uuid`, orgID).Scan(
		&cfg.BaseScore,
		&cfg.CritPenalty,
		&cfg.CritPenaltyCap,
		&cfg.HighPenalty,
		&cfg.HighPenaltyCap,
		&cfg.BreachPenalty,
		&cfg.BreachPenaltyCap,
		&cfg.FwBonus,
		&cfg.FwBonusCap,
	)
	if err != nil {
		if !errors.Is(err, pgx.ErrNoRows) {
			return defaultScoreConfig(), err
		}
		return defaultScoreConfig(), nil
	}
	return cfg, nil
}

// UpsertScoreConfig saves (inserts or updates) the org's score configuration.
func (s *Service) UpsertScoreConfig(ctx context.Context, orgID string, cfg ScoreConfig) error {
	_, err := s.db.Exec(ctx,
		`INSERT INTO score_config
		   (org_id, base_score, crit_penalty, crit_penalty_cap, high_penalty, high_penalty_cap,
		    breach_penalty, breach_penalty_cap, fw_bonus, fw_bonus_cap, updated_at)
		 VALUES ($1::uuid, $2, $3, $4, $5, $6, $7, $8, $9, $10, now())
		 ON CONFLICT (org_id) DO UPDATE SET
		   base_score        = EXCLUDED.base_score,
		   crit_penalty      = EXCLUDED.crit_penalty,
		   crit_penalty_cap  = EXCLUDED.crit_penalty_cap,
		   high_penalty      = EXCLUDED.high_penalty,
		   high_penalty_cap  = EXCLUDED.high_penalty_cap,
		   breach_penalty    = EXCLUDED.breach_penalty,
		   breach_penalty_cap = EXCLUDED.breach_penalty_cap,
		   fw_bonus          = EXCLUDED.fw_bonus,
		   fw_bonus_cap      = EXCLUDED.fw_bonus_cap,
		   updated_at        = now()`,
		orgID,
		cfg.BaseScore, cfg.CritPenalty, cfg.CritPenaltyCap,
		cfg.HighPenalty, cfg.HighPenaltyCap,
		cfg.BreachPenalty, cfg.BreachPenaltyCap,
		cfg.FwBonus, cfg.FwBonusCap,
	)
	return err
}

// LoadScoreInputs fetches the raw finding / breach / framework counts used in the score formula.
func (s *Service) LoadScoreInputs(ctx context.Context, orgID string, cfg ScoreConfig) ScoreInputs {
	inp := ScoreInputs{Cfg: cfg}
	if err := s.db.QueryRow(ctx,
		`SELECT COUNT(*) FROM vb_findings WHERE org_id=$1::uuid AND severity='critical' AND status NOT IN ('resolved','false_positive')`,
		orgID).Scan(&inp.CritCount); err != nil {
		log.Error().Err(err).Msg("dashboard: count critical findings")
	}
	if err := s.db.QueryRow(ctx,
		`SELECT COUNT(*) FROM vb_findings WHERE org_id=$1::uuid AND severity='high' AND status NOT IN ('resolved','false_positive')`,
		orgID).Scan(&inp.HighCount); err != nil {
		log.Error().Err(err).Msg("dashboard: count high findings")
	}
	if err := s.db.QueryRow(ctx,
		`SELECT COUNT(*) FROM po_breaches WHERE org_id=$1::uuid AND status='open'`,
		orgID).Scan(&inp.BreachCount); err != nil {
		log.Error().Err(err).Msg("dashboard: count breaches")
	}
	if err := s.db.QueryRow(ctx,
		`SELECT COUNT(*) FROM ck_frameworks WHERE org_id=$1::uuid`,
		orgID).Scan(&inp.FwCount); err != nil {
		log.Error().Err(err).Msg("dashboard: count frameworks")
	}
	return inp
}

// ComputeScore applies the penalty/bonus formula to the raw inputs and clamps to [0, 100].
func ComputeScore(inp ScoreInputs) (score int, components map[string]int64) {
	cfg := inp.Cfg

	critPenalty := int(inp.CritCount) * cfg.CritPenalty
	if critPenalty > cfg.CritPenaltyCap {
		critPenalty = cfg.CritPenaltyCap
	}
	highPenalty := int(inp.HighCount) * cfg.HighPenalty
	if highPenalty > cfg.HighPenaltyCap {
		highPenalty = cfg.HighPenaltyCap
	}
	breachPenalty := int(inp.BreachCount) * cfg.BreachPenalty
	if breachPenalty > cfg.BreachPenaltyCap {
		breachPenalty = cfg.BreachPenaltyCap
	}
	fwBonus := int(inp.FwCount) * cfg.FwBonus
	if fwBonus > cfg.FwBonusCap {
		fwBonus = cfg.FwBonusCap
	}

	s := cfg.BaseScore - critPenalty - highPenalty - breachPenalty + fwBonus
	if s < 0 {
		s = 0
	}
	if s > 100 {
		s = 100
	}
	return s, map[string]int64{
		"critical_findings": inp.CritCount,
		"high_findings":     inp.HighCount,
		"open_breaches":     inp.BreachCount,
		"active_frameworks": inp.FwCount,
	}
}

// LastBackupAt returns the most recent backup timestamp for the org (nil if none).
func (s *Service) LastBackupAt(ctx context.Context, orgID string) (*time.Time, error) {
	var lastBackup *time.Time
	err := s.db.QueryRow(ctx,
		`SELECT backed_up_at FROM backup_log WHERE org_id=$1::uuid ORDER BY backed_up_at DESC LIMIT 1`,
		orgID).Scan(&lastBackup)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return lastBackup, err
}

// LoadNotifications returns the 50 most recent notifications for the org.
func (s *Service) LoadNotifications(ctx context.Context, orgID string) ([]UserNotification, error) {
	rows, err := s.db.Query(ctx,
		`SELECT id, title, body, type, module, read, created_at
         FROM user_notifications
         WHERE org_id=$1::uuid
         ORDER BY created_at DESC
         LIMIT 50`,
		orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := []UserNotification{}
	for rows.Next() {
		var n UserNotification
		if err := rows.Scan(&n.ID, &n.Title, &n.Body, &n.Type, &n.Module, &n.Read, &n.CreatedAt); err != nil {
			continue
		}
		result = append(result, n)
	}
	return result, rows.Err()
}

// MarkNotificationRead marks a single notification as read within the org.
func (s *Service) MarkNotificationRead(ctx context.Context, orgID, id string) error {
	_, err := s.db.Exec(ctx,
		`UPDATE user_notifications SET read=true WHERE id=$1::uuid AND org_id=$2::uuid`,
		id, orgID)
	return err
}

// MarkAllNotificationsRead marks all unread notifications for the org as read.
func (s *Service) MarkAllNotificationsRead(ctx context.Context, orgID string) error {
	_, err := s.db.Exec(ctx,
		`UPDATE user_notifications SET read=true WHERE org_id=$1::uuid AND read=false`,
		orgID)
	return err
}

// LoadAggregate runs all dashboard sub-queries concurrently and returns the aggregate payload.
func (s *Service) LoadAggregate(ctx context.Context, orgID string) (AggregateResponse, error) {
	var (
		fwScores         []FrameworkScore
		openCAPAs        int64
		overdueControls  int64
		overdueTasks     int64
		criticalRisks    int64
		topRisks         []RiskSummary
		recentActivity   []ActivityEntry
		policiesTotal    int64
		policiesApproved int64
	)

	g, gctx := errgroup.WithContext(ctx)

	g.Go(func() error {
		rows, err := s.db.Query(gctx, `
			SELECT f.id::text, f.name,
			       COUNT(c.id)::int                                                              AS total,
			       COUNT(c.id) FILTER (WHERE c.manual_status IN ('implemented','partially_implemented'))::int AS implemented
			FROM ck_frameworks f
			LEFT JOIN ck_controls c ON c.framework_id = f.id AND c.org_id = f.org_id
			WHERE f.org_id = $1::uuid
			GROUP BY f.id, f.name
			ORDER BY f.name`, orgID)
		if err != nil {
			log.Error().Err(err).Msg("dashboard aggregate: framework scores")
			return nil
		}
		defer rows.Close()
		for rows.Next() {
			var fs FrameworkScore
			if err := rows.Scan(&fs.FrameworkID, &fs.FrameworkName, &fs.TotalControls, &fs.ImplementedControls); err != nil {
				log.Error().Err(err).Msg("dashboard aggregate: scan framework score")
				continue
			}
			if fs.TotalControls > 0 {
				fs.ScorePct = float64(fs.ImplementedControls) / float64(fs.TotalControls) * 100
			}
			fwScores = append(fwScores, fs)
		}
		return rows.Err()
	})

	g.Go(func() error {
		if err := s.db.QueryRow(gctx,
			`SELECT COUNT(*)::bigint FROM ck_capas WHERE org_id=$1::uuid AND status != 'closed'`,
			orgID).Scan(&openCAPAs); err != nil {
			log.Error().Err(err).Msg("dashboard aggregate: open capas")
		}
		return nil
	})

	g.Go(func() error {
		if err := s.db.QueryRow(gctx,
			`SELECT COUNT(*)::bigint FROM ck_controls
			 WHERE org_id=$1::uuid AND next_review_due IS NOT NULL AND next_review_due < NOW()`,
			orgID).Scan(&overdueControls); err != nil {
			log.Error().Err(err).Msg("dashboard aggregate: overdue controls")
		}
		return nil
	})

	g.Go(func() error {
		if err := s.db.QueryRow(gctx,
			`SELECT COUNT(*)::bigint FROM ck_tasks
			 WHERE org_id=$1::uuid AND due_date IS NOT NULL AND due_date < NOW() AND status != 'done'`,
			orgID).Scan(&overdueTasks); err != nil {
			log.Error().Err(err).Msg("dashboard aggregate: overdue tasks")
		}
		return nil
	})

	g.Go(func() error {
		if err := s.db.QueryRow(gctx,
			`SELECT COUNT(*)::bigint FROM ck_risks
			 WHERE org_id=$1::uuid AND (likelihood * impact) >= 15`,
			orgID).Scan(&criticalRisks); err != nil {
			log.Error().Err(err).Msg("dashboard aggregate: critical risks")
		}
		return nil
	})

	g.Go(func() error {
		rows, err := s.db.Query(gctx, `
			SELECT id::text, title, likelihood::int, impact::int,
			       (likelihood * impact)::int AS score, status
			FROM ck_risks
			WHERE org_id = $1::uuid
			ORDER BY score DESC, updated_at DESC
			LIMIT 5`, orgID)
		if err != nil {
			log.Error().Err(err).Msg("dashboard aggregate: top risks")
			return nil
		}
		defer rows.Close()
		for rows.Next() {
			var r RiskSummary
			if err := rows.Scan(&r.ID, &r.Title, &r.Likelihood, &r.Impact, &r.Score, &r.Status); err != nil {
				log.Error().Err(err).Msg("dashboard aggregate: scan risk")
				continue
			}
			topRisks = append(topRisks, r)
		}
		return rows.Err()
	})

	g.Go(func() error {
		rows, err := s.db.Query(gctx, `
			SELECT id::text,
			       action,
			       resource_type,
			       COALESCE(user_email, '') AS user_email,
			       created_at
			FROM audit_log
			WHERE org_id = $1::uuid
			  AND deleted_at IS NULL
			ORDER BY created_at DESC
			LIMIT 10`, orgID)
		if err != nil {
			log.Error().Err(err).Msg("dashboard aggregate: recent activity")
			return nil
		}
		defer rows.Close()
		for rows.Next() {
			var e ActivityEntry
			if err := rows.Scan(&e.ID, &e.Action, &e.EntityType, &e.UserEmail, &e.CreatedAt); err != nil {
				log.Error().Err(err).Msg("dashboard aggregate: scan activity")
				continue
			}
			recentActivity = append(recentActivity, e)
		}
		return rows.Err()
	})

	g.Go(func() error {
		if err := s.db.QueryRow(gctx,
			`SELECT COUNT(*)::bigint,
			        COUNT(*) FILTER (WHERE status = 'active')::bigint
			 FROM ck_policies WHERE org_id=$1::uuid`,
			orgID).Scan(&policiesTotal, &policiesApproved); err != nil {
			log.Error().Err(err).Msg("dashboard aggregate: policies")
		}
		return nil
	})

	if err := g.Wait(); err != nil {
		// Partial-response: log at warn level and return what was collected.
		// Individual goroutines already log their own errors and always return nil,
		// so this branch fires only if a future goroutine is added that propagates.
		log.Warn().Err(err).Msg("dashboard aggregate: partial error")
	}

	if fwScores == nil {
		fwScores = []FrameworkScore{}
	}
	if topRisks == nil {
		topRisks = []RiskSummary{}
	}
	if recentActivity == nil {
		recentActivity = []ActivityEntry{}
	}

	return AggregateResponse{
		FrameworkScores:  fwScores,
		OpenCAPAs:        int(openCAPAs),
		OverdueControls:  int(overdueControls),
		OverdueTasks:     int(overdueTasks),
		CriticalRisks:    int(criticalRisks),
		TopRisks:         topRisks,
		RecentActivity:   recentActivity,
		PoliciesTotal:    int(policiesTotal),
		PoliciesApproved: int(policiesApproved),
	}, nil
}
