package auditreport

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"
	"golang.org/x/sync/errgroup"
)

// ReportData is the full dataset gathered for an audit PDF.
type ReportData struct {
	OrgName        string
	GeneratedAt    time.Time
	Frameworks     []FrameworkSection
	Risks          []RiskRow
	OpenIncidents  []IncidentRow
	ActivePolicies []PolicyRow
	OpenCAPAs      []CAPARow
	EvidenceCount  int
	ControlStats   ControlStats
}

// ControlStats contains aggregate control implementation totals.
type ControlStats struct {
	Total       int
	Implemented int
	InProgress  int
	NotStarted  int
}

// FrameworkSection describes one compliance framework.
type FrameworkSection struct {
	Name          string
	TotalControls int
	Implemented   int
	InProgress    int
	NotStarted    int
	ScorePct      float64
	Domains       []DomainSection
}

// DomainSection groups controls by domain within a framework.
type DomainSection struct {
	Name     string
	Controls []ControlRow
}

// ControlRow is a single control entry in the report.
type ControlRow struct {
	ControlID     string
	Title         string
	Status        string
	EvidenceCount int
}

// RiskRow is a risk register entry.
type RiskRow struct {
	Title       string
	Likelihood  int
	Impact      int
	Score       int
	Status      string
}

// IncidentRow is an open incident entry.
type IncidentRow struct {
	Title     string
	Severity  string
	Status    string
	CreatedAt time.Time
}

// PolicyRow is an active policy entry.
type PolicyRow struct {
	Title     string
	Version   string
	Status    string
	UpdatedAt time.Time
}

// CAPARow is an open corrective action entry.
type CAPARow struct {
	Title      string
	SourceType string
	Status     string
	DueDate    *time.Time
}

// Collect queries all data needed for the audit PDF from the DB.
func Collect(ctx context.Context, db *pgxpool.Pool, orgID string) (*ReportData, error) {
	d := &ReportData{GeneratedAt: time.Now()}

	// Resolve org name (soft-fail).
	_ = db.QueryRow(ctx, `SELECT name FROM organizations WHERE id=$1::uuid`, orgID).Scan(&d.OrgName)
	if d.OrgName == "" {
		d.OrgName = orgID
	}

	g, gctx := errgroup.WithContext(ctx)

	// ── Frameworks + controls per domain ──────────────────────────────────────
	g.Go(func() error {
		fwRows, err := db.Query(gctx, `
			SELECT f.id::text, f.name,
			       COUNT(c.id)::int                                                                  AS total,
			       COUNT(c.id) FILTER (WHERE c.manual_status = 'implemented')::int                  AS implemented,
			       COUNT(c.id) FILTER (WHERE c.manual_status = 'in_progress')::int                  AS in_progress
			FROM ck_frameworks f
			LEFT JOIN ck_controls c ON c.framework_id = f.id AND c.org_id = f.org_id
			WHERE f.org_id = $1::uuid
			GROUP BY f.id, f.name
			ORDER BY f.name`, orgID)
		if err != nil {
			log.Error().Err(err).Msg("auditreport: framework query")
			return nil
		}
		defer fwRows.Close()

		type fwMeta struct {
			id          string
			name        string
			total       int
			implemented int
			inProgress  int
		}
		var frameworks []fwMeta
		for fwRows.Next() {
			var fm fwMeta
			if err := fwRows.Scan(&fm.id, &fm.name, &fm.total, &fm.implemented, &fm.inProgress); err != nil {
				log.Error().Err(err).Msg("auditreport: scan framework")
				continue
			}
			frameworks = append(frameworks, fm)
		}
		_ = fwRows.Err()

		for _, fm := range frameworks {
			notStarted := fm.total - fm.implemented - fm.inProgress
			if notStarted < 0 {
				notStarted = 0
			}
			scorePct := 0.0
			if fm.total > 0 {
				scorePct = float64(fm.implemented) / float64(fm.total) * 100
			}

			// Load domains + controls for this framework.
			ctrlRows, err := db.Query(gctx, `
				SELECT c.control_id, c.title, c.domain,
				       COALESCE(c.manual_status, '') AS manual_status,
				       COUNT(e.id)::int               AS evidence_count
				FROM ck_controls c
				LEFT JOIN ck_evidence e ON e.control_id = c.id
				WHERE c.framework_id = $1::uuid AND c.org_id = $2::uuid
				GROUP BY c.id, c.control_id, c.title, c.domain, c.manual_status
				ORDER BY c.domain, c.control_id`, fm.id, orgID)
			if err != nil {
				log.Error().Err(err).Str("framework", fm.name).Msg("auditreport: controls query")
				// Still add the framework summary without domain detail.
				d.Frameworks = append(d.Frameworks, FrameworkSection{
					Name:          fm.name,
					TotalControls: fm.total,
					Implemented:   fm.implemented,
					InProgress:    fm.inProgress,
					NotStarted:    notStarted,
					ScorePct:      scorePct,
				})
				continue
			}

			domainMap := make(map[string]*DomainSection)
			var domainOrder []string
			for ctrlRows.Next() {
				var cr ControlRow
				var domain string
				if err := ctrlRows.Scan(&cr.ControlID, &cr.Title, &domain, &cr.Status, &cr.EvidenceCount); err != nil {
					log.Error().Err(err).Msg("auditreport: scan control")
					continue
				}
				if _, ok := domainMap[domain]; !ok {
					domainMap[domain] = &DomainSection{Name: domain}
					domainOrder = append(domainOrder, domain)
				}
				domainMap[domain].Controls = append(domainMap[domain].Controls, cr)
			}
			ctrlRows.Close()

			var domains []DomainSection
			for _, dn := range domainOrder {
				domains = append(domains, *domainMap[dn])
			}

			d.Frameworks = append(d.Frameworks, FrameworkSection{
				Name:          fm.name,
				TotalControls: fm.total,
				Implemented:   fm.implemented,
				InProgress:    fm.inProgress,
				NotStarted:    notStarted,
				ScorePct:      scorePct,
				Domains:       domains,
			})
		}
		return nil
	})

	// ── Top 20 risks by score ─────────────────────────────────────────────────
	g.Go(func() error {
		rows, err := db.Query(gctx, `
			SELECT title,
			       likelihood::int,
			       impact::int,
			       (likelihood * impact)::int AS score,
			       status
			FROM ck_risks
			WHERE org_id = $1::uuid
			ORDER BY score DESC, updated_at DESC
			LIMIT 20`, orgID)
		if err != nil {
			log.Error().Err(err).Msg("auditreport: risks query")
			return nil
		}
		defer rows.Close()
		for rows.Next() {
			var r RiskRow
			if err := rows.Scan(&r.Title, &r.Likelihood, &r.Impact, &r.Score, &r.Status); err != nil {
				log.Error().Err(err).Msg("auditreport: scan risk")
				continue
			}
			d.Risks = append(d.Risks, r)
		}
		return rows.Err()
	})

	// ── Open incidents ────────────────────────────────────────────────────────
	g.Go(func() error {
		rows, err := db.Query(gctx, `
			SELECT title, severity, status, created_at
			FROM ck_incidents
			WHERE org_id = $1::uuid AND status IN ('open','investigating')
			ORDER BY
				CASE severity WHEN 'critical' THEN 0 WHEN 'high' THEN 1 WHEN 'medium' THEN 2 ELSE 3 END,
				created_at DESC`, orgID)
		if err != nil {
			log.Error().Err(err).Msg("auditreport: incidents query")
			return nil
		}
		defer rows.Close()
		for rows.Next() {
			var r IncidentRow
			if err := rows.Scan(&r.Title, &r.Severity, &r.Status, &r.CreatedAt); err != nil {
				log.Error().Err(err).Msg("auditreport: scan incident")
				continue
			}
			d.OpenIncidents = append(d.OpenIncidents, r)
		}
		return rows.Err()
	})

	// ── Active policies ───────────────────────────────────────────────────────
	g.Go(func() error {
		rows, err := db.Query(gctx, `
			SELECT title, version, status, updated_at
			FROM ck_policies
			WHERE org_id = $1::uuid
			ORDER BY updated_at DESC`, orgID)
		if err != nil {
			log.Error().Err(err).Msg("auditreport: policies query")
			return nil
		}
		defer rows.Close()
		for rows.Next() {
			var r PolicyRow
			if err := rows.Scan(&r.Title, &r.Version, &r.Status, &r.UpdatedAt); err != nil {
				log.Error().Err(err).Msg("auditreport: scan policy")
				continue
			}
			d.ActivePolicies = append(d.ActivePolicies, r)
		}
		return rows.Err()
	})

	// ── Open CAPAs ────────────────────────────────────────────────────────────
	g.Go(func() error {
		rows, err := db.Query(gctx, `
			SELECT title, source_type, status, due_date
			FROM ck_capas
			WHERE org_id = $1::uuid AND status NOT IN ('closed','verified')
			ORDER BY
				CASE priority WHEN 'critical' THEN 0 WHEN 'high' THEN 1 WHEN 'medium' THEN 2 ELSE 3 END,
				created_at DESC`, orgID)
		if err != nil {
			log.Error().Err(err).Msg("auditreport: capas query")
			return nil
		}
		defer rows.Close()
		for rows.Next() {
			var r CAPARow
			if err := rows.Scan(&r.Title, &r.SourceType, &r.Status, &r.DueDate); err != nil {
				log.Error().Err(err).Msg("auditreport: scan capa")
				continue
			}
			d.OpenCAPAs = append(d.OpenCAPAs, r)
		}
		return rows.Err()
	})

	// ── Evidence count ────────────────────────────────────────────────────────
	g.Go(func() error {
		var n int64
		if err := db.QueryRow(gctx,
			`SELECT COUNT(*)::bigint FROM ck_evidence WHERE org_id=$1::uuid`, orgID,
		).Scan(&n); err != nil {
			log.Error().Err(err).Msg("auditreport: evidence count")
		}
		d.EvidenceCount = int(n)
		return nil
	})

	// ── Global control stats ──────────────────────────────────────────────────
	g.Go(func() error {
		var total, implemented, inProgress int64
		if err := db.QueryRow(gctx, `
			SELECT
				COUNT(*)::bigint,
				COUNT(*) FILTER (WHERE manual_status = 'implemented')::bigint,
				COUNT(*) FILTER (WHERE manual_status = 'in_progress')::bigint
			FROM ck_controls WHERE org_id=$1::uuid`, orgID,
		).Scan(&total, &implemented, &inProgress); err != nil {
			log.Error().Err(err).Msg("auditreport: control stats")
		}
		d.ControlStats = ControlStats{
			Total:       int(total),
			Implemented: int(implemented),
			InProgress:  int(inProgress),
			NotStarted:  int(total) - int(implemented) - int(inProgress),
		}
		if d.ControlStats.NotStarted < 0 {
			d.ControlStats.NotStarted = 0
		}
		return nil
	})

	_ = g.Wait()
	return d, nil
}
