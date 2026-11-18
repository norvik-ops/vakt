// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package dashboard

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"
	"golang.org/x/sync/errgroup"

	"github.com/matharnica/vakt/internal/config"
	"github.com/matharnica/vakt/internal/shared/percent"
	"github.com/matharnica/vakt/internal/shared/risklevel"
)

// FrameworkReadiness is one readiness tally (ADR-0086): Covered = effectively
// implemented/covered, Partial = in_progress/partial (half weight),
// Applicable = all controls except not_applicable, Score = 0–100.
type FrameworkReadiness struct {
	Covered    int
	Partial    int
	Applicable int
	Score      float64
}

// ReadinessSummary is what the dashboard needs from the compliance module:
// the tally per framework ID and the org-wide pooled value.
type ReadinessSummary struct {
	ByFramework map[string]FrameworkReadiness
	Overall     FrameworkReadiness
}

// ReadinessSource computes the canonical readiness (ADR-0086). The dashboard
// does not compute it itself and does not import the module that owns the
// formula (shared must not depend on a module, ADR-0004/ADR-0066): cmd/api
// injects the implementation from vaktcomply (R1-KW-6).
type ReadinessSource interface {
	ReadinessSummary(ctx context.Context, orgID string) (ReadinessSummary, error)
}

// Service encapsulates all database access for the dashboard package.
// Handlers call Service methods exclusively — no direct pgxpool usage in handlers.
type Service struct {
	db        *pgxpool.Pool
	readiness ReadinessSource // nil → framework scores stay empty (logged)
}

// NewService creates a Service backed by the provided connection pool and the
// readiness source for the framework progress widgets.
func NewService(db *pgxpool.Pool, readiness ReadinessSource) *Service {
	return &Service{db: db, readiness: readiness}
}

// ScoreResponse is the payload of GET /api/v1/dashboard/score.
//
// Score is the org-wide readiness (ADR-0086), rounded to a whole percent — the
// same number the "Compliance-Fortschritt" card (aggregate overall_readiness),
// the executive summary PDF and the daily score snapshot show. It replaced the
// old "Security Score" (base 70 + 10 per framework, penalties only from
// archived modules), which showed 70/100 on an empty instance and 100/100 with
// three frameworks and 1 of 214 controls implemented (UX-37).
//
// HasData is false while no applicable control exists; the UI then says
// "noch keine Daten" instead of 0 %. Components stays for older clients:
// active_frameworks is still counted, the finding/breach counters of the
// archived modules are always 0.
type ScoreResponse struct {
	Score      int              `json:"score"`
	HasData    bool             `json:"has_data"`
	Readiness  OverallReadiness `json:"readiness"`
	Components map[string]int64 `json:"components"`
}

// ScoreFromReadiness builds the score payload from the pooled readiness tally.
// It does not compute readiness itself: the tally comes from the injected
// ReadinessSource (policy.ReadinessScore, covered full, partial half).
func ScoreFromReadiness(overall FrameworkReadiness, activeFrameworks int64) ScoreResponse {
	return ScoreResponse{
		Score:   percent.Round(overall.Score),
		HasData: overall.Applicable > 0,
		Readiness: OverallReadiness{
			Covered:    overall.Covered,
			Partial:    overall.Partial,
			Applicable: overall.Applicable,
			ScorePct:   overall.Score,
		},
		Components: map[string]int64{
			"critical_findings": 0,
			"high_findings":     0,
			"open_breaches":     0,
			"active_frameworks": activeFrameworks,
		},
	}
}

// LoadScore returns the dashboard score: the canonical org-wide readiness from
// the ReadinessSource plus the number of active frameworks.
func (s *Service) LoadScore(ctx context.Context, orgID string) (ScoreResponse, error) {
	if s.readiness == nil {
		return ScoreResponse{}, errors.New("dashboard score: no readiness source wired")
	}
	summary, err := s.readiness.ReadinessSummary(ctx, orgID)
	if err != nil {
		return ScoreResponse{}, fmt.Errorf("dashboard score: readiness: %w", err)
	}
	var fwCount int64
	if err := s.db.QueryRow(ctx,
		`SELECT COUNT(*) FROM ck_frameworks WHERE org_id=$1::uuid`,
		orgID).Scan(&fwCount); err != nil {
		// Only the legacy component counter depends on this; the score itself
		// is already known, so log and carry on.
		log.Error().Err(err).Msg("dashboard score: count frameworks")
	}
	return ScoreFromReadiness(summary.Overall, fwCount), nil
}

// LoadScoreConfig loads the org's score weights from the DB, returning defaults on no-row.
//
// Deprecated: the weights belonged to the old "Security Score" formula, which
// the dashboard no longer uses (it shows the readiness, ADR-0086). The
// endpoints stay readable and writable for older clients; the values have no
// effect on any displayed number.
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

// disabledModules lists the archived modules. Notifications carry their module
// name; old hints of an archived module (scan alerts, DSR reminders) still sit
// in user_notifications and link to pages that no longer exist
// (docs/stories/module-archivieren.md).
func (s *Service) disabledModules() []string {
	return config.ArchivedModules
}

// LoadNotifications returns the 50 most recent notifications for the org,
// without those of archived modules.
func (s *Service) LoadNotifications(ctx context.Context, orgID string) ([]UserNotification, error) {
	rows, err := s.db.Query(ctx,
		`SELECT id, title, body, type, module, read, created_at
         FROM user_notifications
         WHERE org_id=$1::uuid AND NOT (module = ANY($2::text[]))
         ORDER BY created_at DESC
         LIMIT 50`,
		orgID, s.disabledModules())
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
		overall          OverallReadiness
		openCAPAs        int64
		overdueControls  int64
		overdueTasks     int64
		openRisks        int64
		criticalRisks    int64
		topRisks         []RiskSummary
		recentActivity   []ActivityEntry
		policiesTotal    int64
		policiesApproved int64
	)

	g, gctx := errgroup.WithContext(ctx)

	g.Go(func() error {
		// R1-KW-6 / ADR-0086: the percentages come from the one canonical
		// formula (injected ReadinessSource), the same the framework overview
		// and detail show. The dashboard used to count manual_status =
		// implemented over ALL controls in its own SQL — evidence ignored,
		// not_applicable in the denominator — and showed a different number as
		// soon as either occurred.
		// ImplementedControls = effectively implemented/covered,
		// TotalControls = applicable controls (the denominator of ScorePct).
		if s.readiness == nil {
			log.Error().Msg("dashboard aggregate: no readiness source wired — framework scores empty")
			return nil
		}
		summary, err := s.readiness.ReadinessSummary(gctx, orgID)
		if err != nil {
			log.Error().Err(err).Msg("dashboard aggregate: framework readiness")
			return nil
		}
		readiness := summary.ByFramework
		overall = OverallReadiness{
			Covered:    summary.Overall.Covered,
			Partial:    summary.Overall.Partial,
			Applicable: summary.Overall.Applicable,
			ScorePct:   summary.Overall.Score,
		}
		rows, err := s.db.Query(gctx, `
			SELECT f.id::text, f.name
			FROM ck_frameworks f
			WHERE f.org_id = $1::uuid
			ORDER BY f.name`, orgID)
		if err != nil {
			log.Error().Err(err).Msg("dashboard aggregate: framework scores")
			return nil
		}
		defer rows.Close()
		for rows.Next() {
			var fs FrameworkScore
			if err := rows.Scan(&fs.FrameworkID, &fs.FrameworkName); err != nil {
				log.Error().Err(err).Msg("dashboard aggregate: scan framework score")
				continue
			}
			r := readiness[fs.FrameworkID]
			fs.TotalControls = r.Applicable
			fs.ImplementedControls = r.Covered
			fs.PartialControls = r.Partial
			fs.ScorePct = r.Score
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
		// UX-38: the dashboard counted critical risks of every status (closed
		// ones too), the Comply overview open risks at level critical OR high,
		// from a paged list. Both read these two numbers now: open = status
		// 'open', critical = open AND level critical — the board report's rule.
		if err := s.db.QueryRow(gctx,
			`SELECT COUNT(*) FILTER (WHERE status = 'open')::bigint,
			        COUNT(*) FILTER (WHERE status = 'open' AND (likelihood * impact) >= $2)::bigint
			 FROM ck_risks
			 WHERE org_id=$1::uuid`,
			orgID, risklevel.CriticalFrom).Scan(&openRisks, &criticalRisks); err != nil {
			log.Error().Err(err).Msg("dashboard aggregate: open/critical risks")
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
		OverallReadiness: overall,
		OpenCAPAs:        int(openCAPAs),
		OverdueControls:  int(overdueControls),
		OverdueTasks:     int(overdueTasks),
		OpenRisks:        int(openRisks),
		CriticalRisks:    int(criticalRisks),
		TopRisks:         topRisks,
		RecentActivity:   recentActivity,
		PoliciesTotal:    int(policiesTotal),
		PoliciesApproved: int(policiesApproved),
	}, nil
}
