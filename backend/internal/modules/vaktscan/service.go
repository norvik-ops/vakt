// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package vaktscan

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"

	"github.com/matharnica/vakt/internal/services/crossevidence"
	"github.com/matharnica/vakt/internal/services/evidence_auto"
	"github.com/matharnica/vakt/internal/shared/dashboard"
	"github.com/matharnica/vakt/internal/shared/notify"
	"github.com/matharnica/vakt/internal/shared/platform/events"
	"github.com/matharnica/vakt/internal/shared/platform/webhooks"
	"github.com/matharnica/vakt/internal/shared/safego"
)

// webhookTrigger abstracts the webhook delivery dependency for testability.
type webhookTrigger interface {
	TriggerEvent(ctx context.Context, orgID, eventType string, payload any)
}

// Service handles VulnBoard business logic.
type Service struct {
	repo        *Repository
	db          *pgxpool.Pool
	rdb         *redis.Client
	asynqClient *asynq.Client
	webhookSvc  webhookTrigger
}

// NewService creates a new VulnBoard service.
// Pass a zero-value asynq.RedisClientOpt{} if Redis is not available.
func NewService(db *pgxpool.Pool, asynqOpt asynq.RedisClientOpt) *Service {
	var client *asynq.Client
	if asynqOpt.Addr != "" {
		client = asynq.NewClient(asynqOpt)
	}
	return &Service{
		repo:        NewRepository(db),
		db:          db,
		asynqClient: client,
	}
}

// WithRedis sets the Redis client used for dashboard cache invalidation.
func (s *Service) WithRedis(rdb *redis.Client) {
	s.rdb = rdb
}

// WithWebhooks sets the webhook service used to fire outgoing events.
func (s *Service) WithWebhooks(svc *webhooks.WebhookService) {
	s.webhookSvc = svc
}

// triggerWebhook fires a webhook event in a background goroutine so the caller
// is never blocked by network latency or a slow endpoint.
// ADR-0018: läuft über safego.Run; parentCtx ist der Request-/Job-Context.
// WithoutCancel-Wrapper bewahrt Fire-and-Forget-Semantik beim Client-Disconnect.
func (s *Service) triggerWebhook(parentCtx context.Context, orgID, eventType string, payload map[string]any) {
	if s.webhookSvc == nil {
		return
	}
	safego.Run(parentCtx, "vaktscan.webhook.trigger", func(parent context.Context) error {
		ctx, cancel := context.WithTimeout(context.WithoutCancel(parent), 15*time.Second)
		defer cancel()
		s.webhookSvc.TriggerEvent(ctx, orgID, eventType, payload)
		return nil
	})
}

// invalidateDashboardCache deletes the cached dashboard aggregate for the given
// org from Redis. It is a no-op when Redis is not configured.
func (s *Service) invalidateDashboardCache(ctx context.Context, orgID string) {
	if err := dashboard.InvalidateDashboardCache(ctx, s.rdb, orgID); err != nil {
		log.Warn().Err(err).Str("org_id", orgID).Msg("vaktscan: dashboard cache invalidation failed")
	}
}

// CreateAsset validates and creates a new asset.
func (s *Service) CreateAsset(ctx context.Context, orgID, _ string, input CreateAssetInput) (*Asset, error) {
	if input.Criticality == "" {
		input.Criticality = "medium"
	}
	return s.repo.CreateAsset(ctx, orgID, input)
}

// ListAssets returns a paginated list of assets for the org.
func (s *Service) ListAssets(ctx context.Context, orgID string, page, limit int, tag string) ([]Asset, int, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 500 {
		limit = 25
	}
	return s.repo.ListAssets(ctx, orgID, page, limit, tag)
}

// GetAsset fetches a single asset by ID.
func (s *Service) GetAsset(ctx context.Context, orgID, assetID string) (*Asset, error) {
	return s.repo.GetAsset(ctx, orgID, assetID)
}

// UpdateAsset applies a partial update.
func (s *Service) UpdateAsset(ctx context.Context, orgID, assetID string, input UpdateAssetInput) (*Asset, error) {
	return s.repo.UpdateAsset(ctx, orgID, assetID, input)
}

// GetAssetProtectionNeedID returns the protection_need_id soft-link for an asset, or nil if unlinked.
func (s *Service) GetAssetProtectionNeedID(ctx context.Context, orgID, assetID string) (*string, error) {
	return s.repo.GetAssetProtectionNeedID(ctx, orgID, assetID)
}

// GetClassificationSummary returns asset counts grouped by classification level (S67-3).
func (s *Service) GetClassificationSummary(ctx context.Context, orgID string) (*ClassificationSummary, error) {
	return s.repo.GetClassificationSummary(ctx, orgID)
}

// DeleteAsset soft-deletes an asset.
func (s *Service) DeleteAsset(ctx context.Context, orgID, assetID string) error {
	return s.repo.SoftDeleteAsset(ctx, orgID, assetID)
}

// GetSLAConfig returns the org's SLA configuration (defaults if none set).
func (s *Service) GetSLAConfig(ctx context.Context, orgID string) (*SLAConfig, error) {
	return s.repo.GetSLAConfig(ctx, orgID)
}

// UpdateSLAConfig saves the org's SLA configuration.
func (s *Service) UpdateSLAConfig(ctx context.Context, orgID string, input SLAConfig) error {
	input.OrgID = orgID
	return s.repo.UpsertSLAConfig(ctx, orgID, input)
}

// GetSLADashboard returns open findings enriched with SLA status for the compliance dashboard.
// It reads per-severity thresholds from vb_sla_config (falling back to DB defaults when no
// row exists), then computes Overdue and SLADays for each finding returned by the repository.
func (s *Service) GetSLADashboard(ctx context.Context, orgID string) ([]SLAEntry, error) {
	cfg, _ := s.repo.GetSLAConfig(ctx, orgID)

	rows, err := s.repo.GetSLADashboard(ctx, orgID)
	if err != nil {
		return nil, err
	}

	entries := make([]SLAEntry, 0, len(rows))
	for _, dbRow := range rows {
		slaDays := slaDaysForSeverity(cfg, dbRow.Severity)
		entries = append(entries, SLAEntry{
			AssetID:      dbRow.AssetID,
			AssetName:    dbRow.AssetName,
			FindingID:    dbRow.FindingID,
			FindingTitle: dbRow.FindingTitle,
			Severity:     dbRow.Severity,
			Status:       dbRow.Status,
			DaysOpen:     dbRow.DaysOpen,
			SLADays:      slaDays,
			Overdue:      dbRow.DaysOpen > slaDays,
		})
	}
	return entries, nil
}

// slaDaysForSeverity maps a severity label to the org's configured remediation window in days.
// Unrecognised severity values fall back to 90 days (the BSI-Grundschutz baseline).
func slaDaysForSeverity(cfg *SLAConfig, severity string) int {
	switch severity {
	case "critical":
		return cfg.CriticalDays
	case "high":
		return cfg.HighDays
	case "medium":
		return cfg.MediumDays
	case "low":
		return cfg.LowDays
	default:
		return 90
	}
}

// ImportAssetsCSV parses a CSV reader and bulk-inserts assets.
// Returns (inserted, errored, errorMessages, error).
// The CSV must have a header row: name,type,criticality,tags,external_url
// Tags are comma-separated within a quoted field.
func (s *Service) ImportAssetsCSV(ctx context.Context, orgID, _ string, r io.Reader) (int, int, []string, error) {
	reader := csv.NewReader(r)
	reader.TrimLeadingSpace = true

	header, err := reader.Read()
	if err != nil {
		return 0, 0, nil, fmt.Errorf("read CSV header: %w", err)
	}

	// Build column index map for flexible column ordering.
	colIdx := make(map[string]int, len(header))
	for i, h := range header {
		colIdx[strings.ToLower(strings.TrimSpace(h))] = i
	}

	required := []string{"name", "type"}
	for _, col := range required {
		if _, ok := colIdx[col]; !ok {
			return 0, 0, nil, fmt.Errorf("CSV missing required column %q", col)
		}
	}

	var rows []CSVAssetRow
	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return 0, 0, nil, fmt.Errorf("read CSV row: %w", err)
		}

		row := CSVAssetRow{}

		if i, ok := colIdx["name"]; ok && i < len(record) {
			row.Name = strings.TrimSpace(record[i])
		}
		if i, ok := colIdx["type"]; ok && i < len(record) {
			row.Type = strings.TrimSpace(record[i])
		}
		if i, ok := colIdx["criticality"]; ok && i < len(record) {
			row.Criticality = strings.TrimSpace(record[i])
		}
		if row.Criticality == "" {
			row.Criticality = "medium"
		}
		if i, ok := colIdx["tags"]; ok && i < len(record) {
			raw := strings.TrimSpace(record[i])
			if raw != "" {
				for _, t := range strings.Split(raw, ",") {
					if tag := strings.TrimSpace(t); tag != "" {
						row.Tags = append(row.Tags, tag)
					}
				}
			}
		}
		if i, ok := colIdx["external_url"]; ok && i < len(record) {
			row.ExternalURL = strings.TrimSpace(record[i])
		}

		rows = append(rows, row)
	}

	if len(rows) == 0 {
		return 0, 0, nil, fmt.Errorf("CSV file contains no data rows")
	}

	inserted, errored, errs := s.repo.BulkCreateAssets(ctx, orgID, rows)
	return inserted, errored, errs, nil
}

// ---------------------------------------------------------------------------
// Scans
// ---------------------------------------------------------------------------

// TriggerScan creates a scan record and enqueues the appropriate Asynq task.
func (s *Service) TriggerScan(ctx context.Context, orgID, assetID string, input CreateScanInput) (*Scan, error) {
	scan, err := s.repo.CreateScan(ctx, orgID, input, assetID)
	if err != nil {
		return nil, fmt.Errorf("create scan record: %w", err)
	}

	asset, err := s.repo.GetAsset(ctx, orgID, assetID)
	if err != nil {
		return nil, fmt.Errorf("get asset for scan: %w", err)
	}

	payload := ScanPayload{
		ScanID:    scan.ID,
		OrgID:     orgID,
		AssetID:   assetID,
		AssetName: asset.Name,
		Scanner:   input.Scanner,
		TargetURL: input.TargetURL,
		TargetIP:  input.TargetIP,
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal scan payload: %w", err)
	}

	taskType := taskTypeForScanner(input.Scanner)

	if s.asynqClient != nil {
		task := asynq.NewTask(taskType, payloadBytes)
		if _, err := s.asynqClient.EnqueueContext(ctx, task, asynq.Queue(QueueScans)); err != nil {
			return nil, fmt.Errorf("enqueue scan task: %w", err)
		}
	}

	return scan, nil
}

// GetScan fetches a scan record.
func (s *Service) GetScan(ctx context.Context, orgID, scanID string) (*Scan, error) {
	return s.repo.GetScan(ctx, orgID, scanID)
}

// taskTypeForScanner returns the Asynq task type string for the given scanner.
func taskTypeForScanner(scanner string) string {
	switch scanner {
	case "trivy":
		return TaskScanTrivy
	case "nuclei":
		return TaskScanNuclei
	case "openvas":
		return TaskScanOpenVAS
	default:
		return "vaktscan:scan:unknown"
	}
}

// ---------------------------------------------------------------------------
// Findings
// ---------------------------------------------------------------------------

// UpsertFinding upserts a single finding and fires the finding.created webhook.
// It is used by scanner import flows that create findings one-by-one.
func (s *Service) UpsertFinding(ctx context.Context, orgID string, f Finding) (*Finding, error) {
	result, err := s.repo.UpsertFinding(ctx, orgID, f)
	if err != nil {
		return nil, err
	}
	s.triggerWebhook(ctx, orgID, "finding.created", map[string]any{
		"id":       result.ID,
		"title":    result.Title,
		"severity": result.Severity,
		"org_id":   orgID,
	})
	// S88-8: emit a cross-module finding-created event for critical/high findings
	// so the Scan→Comply bridge attaches it as evidence on A.8.8/A.8.9 (idempotent,
	// consumed by the worker — no direct cross-module call here).
	if s.asynqClient != nil && (result.Severity == "critical" || result.Severity == "high") {
		if task, taskErr := crossevidence.NewRecordEvidenceTask(
			events.FindingCreated(orgID, result.ID, result.Title, result.Severity),
		); taskErr == nil {
			if _, enqErr := s.asynqClient.EnqueueContext(ctx, task, asynq.Queue(crossevidence.Queue)); enqErr != nil {
				log.Warn().Err(enqErr).Str("finding_id", result.ID).Msg("vaktscan: finding-created event enqueue failed")
			}
		}
	}

	// Notify org on critical/high severity findings.
	if result.Severity == "critical" || result.Severity == "high" {
		capturedOrgID := orgID
		capturedTitle := result.Title
		capturedSev := result.Severity
		// ADR-0018: notify-fanout über safego.Run mit Parent-Context + WithoutCancel.
		safego.Run(ctx, "vaktscan.finding.notify.severity", func(parent context.Context) error {
			notifyCtx, notifyCancel := context.WithTimeout(context.WithoutCancel(parent), 10*time.Second)
			defer notifyCancel()
			sev := "Kritisch"
			if capturedSev == "high" {
				sev = "Hoch"
			}
			notify.Send(notifyCtx, s.db, capturedOrgID,
				"Neues "+sev+"-Finding",
				"Das Finding \""+capturedTitle+"\" wurde als "+sev+" eingestuft.",
				"warning", "vaktscan")
			return nil
		})
	}
	return result, nil
}

// ListFindings returns findings for an org with optional filtering.
func (s *Service) ListFindings(ctx context.Context, orgID string, filter FindingFilter) ([]Finding, error) {
	return s.repo.ListFindings(ctx, orgID, filter)
}

// CountFindings returns total number of findings matching the filter (without pagination).
func (s *Service) CountFindings(ctx context.Context, orgID string, filter FindingFilter) (int, error) {
	return s.repo.CountFindings(ctx, orgID, filter)
}

// GetFinding fetches a single finding.
func (s *Service) GetFinding(ctx context.Context, orgID, findingID string) (*Finding, error) {
	return s.repo.GetFinding(ctx, orgID, findingID)
}

// UpdateFinding applies a partial update.  Justification is required when
// setting status to "accepted_risk".  When status is set to "resolved" an
// auto-evidence job is enqueued so that related SecVitals patch controls are
// updated automatically.
func (s *Service) UpdateFinding(ctx context.Context, orgID, findingID string, input UpdateFindingInput) (*Finding, error) {
	if input.Status != nil && *input.Status == "accepted_risk" {
		if input.Justification == nil || strings.TrimSpace(*input.Justification) == "" {
			return nil, fmt.Errorf("justification is required when setting status to accepted_risk")
		}
	}

	finding, err := s.repo.UpdateFinding(ctx, orgID, findingID, input)
	if err != nil {
		return nil, err
	}

	// Notify org when a finding is assigned. ADR-0018: safego.Run + WithoutCancel.
	if input.AssignedTo != nil && *input.AssignedTo != "" {
		capturedOrgID := orgID
		capturedTitle := finding.Title
		safego.Run(ctx, "vaktscan.finding.notify.assigned", func(parent context.Context) error {
			notifyCtx, notifyCancel := context.WithTimeout(context.WithoutCancel(parent), 10*time.Second)
			defer notifyCancel()
			notify.Send(notifyCtx, s.db, capturedOrgID,
				"Finding zugewiesen",
				"Das Finding \""+capturedTitle+"\" wurde einem Bearbeiter zugewiesen.",
				"info", "vaktscan")
			return nil
		})
	}

	// Enqueue auto-evidence job when a finding is resolved.
	if input.Status != nil && *input.Status == "resolved" && s.asynqClient != nil {
		cve := ""
		if finding.CVEID != nil {
			cve = *finding.CVEID
		}
		payload := AutoEvidencePayload{
			OrgID:     orgID,
			FindingID: findingID,
			CVE:       cve,
			Title:     finding.Title,
		}
		payloadBytes, marshalErr := json.Marshal(payload)
		if marshalErr == nil {
			task := asynq.NewTask(TaskAutoEvidence, payloadBytes)
			if _, enqErr := s.asynqClient.EnqueueContext(ctx, task, asynq.Queue(crossevidence.Queue)); enqErr != nil {
				log.Warn().Err(enqErr).Str("finding_id", findingID).Msg("vaktscan: auto-evidence enqueue failed (evidence may be missed on Redis outage)")
			}
		}
	}

	// Enqueue AI evidence-suggestion check when finding is resolved (S52-5).
	if input.Status != nil && *input.Status == "resolved" && s.asynqClient != nil {
		suggPayload := struct {
			FindingID    string `json:"finding_id"`
			OrgID        string `json:"org_id"`
			Severity     string `json:"severity"`
			FindingTitle string `json:"finding_title"`
		}{
			FindingID:    findingID,
			OrgID:        orgID,
			Severity:     finding.Severity,
			FindingTitle: finding.Title,
		}
		if suggBytes, marshalErr := json.Marshal(suggPayload); marshalErr == nil {
			suggTask := asynq.NewTask("vaktcomply:ai_evidence_suggestion", suggBytes, asynq.MaxRetry(1))
			if _, enqErr := s.asynqClient.EnqueueContext(ctx, suggTask, asynq.Queue(crossevidence.Queue)); enqErr != nil {
				log.Warn().Err(enqErr).Str("finding_id", findingID).Msg("vaktscan: ai-suggestion enqueue failed (evidence may be missed on Redis outage)")
			}
		}
	}

	// Collect auto-evidence into the unassigned inbox when finding is resolved
	// (best-effort). ADR-0018: safego.Run + WithoutCancel.
	if input.Status != nil && *input.Status == "resolved" {
		capturedOrgID := orgID
		capturedFindingID := findingID
		safego.Run(ctx, "vaktscan.finding.evidence.collect", func(parent context.Context) error {
			evidCtx, evidCancel := context.WithTimeout(context.WithoutCancel(parent), 30*time.Second)
			defer evidCancel()
			if autoErr := evidence_auto.CollectSecPulseEvidence(evidCtx, s.db, capturedOrgID, capturedFindingID); autoErr != nil {
				log.Warn().Err(autoErr).Msg("auto-evidence collection failed")
			}
			return nil
		})
	}

	// Trigger outgoing webhook for severity change (non-blocking).
	if input.Severity != nil {
		s.triggerWebhook(ctx, orgID, "finding.severity_changed", map[string]any{
			"id":       finding.ID,
			"title":    finding.Title,
			"severity": finding.Severity,
			"org_id":   orgID,
		})
	}

	s.invalidateDashboardCache(ctx, orgID)
	return finding, nil
}

// DeleteFinding removes a finding by ID within the org.
func (s *Service) DeleteFinding(ctx context.Context, orgID, findingID string) error {
	if err := s.repo.DeleteFinding(ctx, orgID, findingID); err != nil {
		return err
	}
	s.invalidateDashboardCache(ctx, orgID)
	return nil
}

// BulkUpdateFindings applies a bulk status/assignee update.
func (s *Service) BulkUpdateFindings(ctx context.Context, orgID string, input BulkFindingInput) (int, error) {
	return s.repo.BulkUpdateFindings(ctx, orgID, input)
}

// ---------------------------------------------------------------------------
// Suppression Rules
// ---------------------------------------------------------------------------

// ListSuppressionRules returns all suppression rules for an org.
func (s *Service) ListSuppressionRules(ctx context.Context, orgID string) ([]SuppressionRule, error) {
	return s.repo.ListSuppressionRules(ctx, orgID)
}

// CreateSuppressionRule creates a new suppression rule.
func (s *Service) CreateSuppressionRule(ctx context.Context, orgID, userID string, input CreateSuppressionInput) (*SuppressionRule, error) {
	return s.repo.CreateSuppressionRule(ctx, orgID, userID, input)
}

// DeleteSuppressionRule removes a suppression rule.
func (s *Service) DeleteSuppressionRule(ctx context.Context, orgID, ruleID string) error {
	return s.repo.DeleteSuppressionRule(ctx, orgID, ruleID)
}

// ---------------------------------------------------------------------------
// Scan Schedules
// ---------------------------------------------------------------------------

// ListScanSchedules returns all scan schedules for an asset.
func (s *Service) ListScanSchedules(ctx context.Context, orgID, assetID string) ([]ScanSchedule, error) {
	return s.repo.ListScanSchedules(ctx, orgID, assetID)
}

// CreateScanSchedule creates a new scan schedule for an asset.
func (s *Service) CreateScanSchedule(ctx context.Context, orgID, assetID string, input CreateScanScheduleInput) (*ScanSchedule, error) {
	return s.repo.CreateScanSchedule(ctx, orgID, assetID, input)
}

// DeleteScanSchedule removes a scan schedule.
func (s *Service) DeleteScanSchedule(ctx context.Context, orgID, scheduleID string) error {
	return s.repo.DeleteScanSchedule(ctx, orgID, scheduleID)
}

// ---------------------------------------------------------------------------
// Risk Trend & Reports
// ---------------------------------------------------------------------------

// GetRiskTrend returns daily aggregated risk data over the last N days.
func (s *Service) GetRiskTrend(ctx context.Context, orgID string, days int) ([]RiskTrendPoint, error) {
	return s.repo.GetRiskTrend(ctx, orgID, days)
}

// GenerateReport creates a report record and enqueues the Asynq generation job.
func (s *Service) GenerateReport(ctx context.Context, orgID, userID string, scope ReportScope) (*Report, error) {
	rpt, err := s.repo.CreateReport(ctx, orgID, userID, scope)
	if err != nil {
		return nil, fmt.Errorf("create report record: %w", err)
	}

	jobPayload := GenerateReportPayload{
		ReportID: rpt.ID,
		OrgID:    orgID,
		Scope:    scope,
	}
	payloadBytes, err := json.Marshal(jobPayload)
	if err != nil {
		return nil, fmt.Errorf("marshal report payload: %w", err)
	}

	if s.asynqClient != nil {
		task := asynq.NewTask(TaskGenerateReport, payloadBytes)
		if _, err := s.asynqClient.EnqueueContext(ctx, task, asynq.Queue(QueueScans)); err != nil {
			return nil, fmt.Errorf("enqueue report task: %w", err)
		}
	}

	return rpt, nil
}

// GetReport fetches a report by ID.
func (s *Service) GetReport(ctx context.Context, orgID, reportID string) (*Report, error) {
	return s.repo.GetReport(ctx, orgID, reportID)
}

// ListReports returns all reports for an org, newest first.
func (s *Service) ListReports(ctx context.Context, orgID string) ([]Report, error) {
	return s.repo.ListReports(ctx, orgID)
}

// GetReportContent returns the PDF bytes and title for a completed report.
func (s *Service) GetReportContent(ctx context.Context, orgID, reportID string) ([]byte, string, error) {
	return s.repo.GetReportContent(ctx, orgID, reportID)
}

// calculateRiskScore computes a numeric risk score from CVSS, EPSS percentile,
// and asset criticality.  It mirrors the logic in ComputeRiskScore but accepts
// individual parameters, making it straightforward to unit-test in isolation.
//
// Formula: cvss * (1 + epssPercentile) * criticalityMultiplier
// If cvss is nil, defaults to 5.0.  If epssPercent is nil, multiplier is 1.0.
func calculateRiskScore(cvss *float64, epssPercent *float64, criticality string) float64 {
	base := 5.0
	if cvss != nil {
		base = *cvss
	}

	epssMultiplier := 1.0
	if epssPercent != nil {
		epssMultiplier = 1.0 + *epssPercent
	}

	var critMultiplier float64
	switch criticality {
	case "critical":
		critMultiplier = 2.0
	case "high":
		critMultiplier = 1.5
	case "medium":
		critMultiplier = 1.0
	case "low":
		critMultiplier = 0.5
	default:
		critMultiplier = 0.25
	}

	return base * epssMultiplier * critMultiplier
}

// ExportFindings returns findings for an org as a CSV or JSON reader.
// Batches of 500 via keyset pagination (S120-9) — the previous OFFSET/page
// loop re-scanned all prior rows per batch (O(n²) on deep exports).
func (s *Service) ExportFindings(ctx context.Context, orgID, format string, filter FindingFilter) (io.Reader, error) {
	const batchSize = 500
	var (
		findings []Finding
		cursorID string
		cursorTS time.Time
	)
	for {
		batch, err := s.repo.ListFindingsCursor(ctx, orgID, filter, cursorID, cursorTS, batchSize)
		if err != nil {
			return nil, fmt.Errorf("list findings for export: %w", err)
		}
		// ListFindingsCursor fetches batchSize+1 rows for HasMore detection.
		hasMore := len(batch) > batchSize
		if hasMore {
			batch = batch[:batchSize]
		}
		findings = append(findings, batch...)
		if !hasMore {
			break
		}
		last := batch[len(batch)-1]
		cursorID, cursorTS = last.ID, last.CreatedAt
	}

	switch strings.ToLower(format) {
	case "json":
		data, err := json.Marshal(findings)
		if err != nil {
			return nil, fmt.Errorf("marshal findings JSON: %w", err)
		}
		return bytes.NewReader(data), nil

	default: // csv
		var buf bytes.Buffer
		w := csv.NewWriter(&buf)
		if err := w.Write([]string{
			"id", "org_id", "asset_id", "cve_id", "title", "severity",
			"status", "scanner", "risk_score", "last_seen_at", "created_at",
		}); err != nil {
			return nil, fmt.Errorf("write CSV header: %w", err)
		}
		for _, f := range findings {
			cveID := ""
			if f.CVEID != nil {
				cveID = *f.CVEID
			}
			riskScore := ""
			if f.RiskScore != nil {
				riskScore = fmt.Sprintf("%.4f", *f.RiskScore)
			}
			if err := w.Write([]string{
				f.ID, f.OrgID, f.AssetID, cveID, f.Title, f.Severity,
				f.Status, f.Scanner, riskScore,
				f.LastSeenAt.Format(time.RFC3339),
				f.CreatedAt.Format(time.RFC3339),
			}); err != nil {
				return nil, fmt.Errorf("write CSV row: %w", err)
			}
		}
		w.Flush()
		if err := w.Error(); err != nil {
			return nil, fmt.Errorf("flush CSV: %w", err)
		}
		return &buf, nil
	}
}

// ---------------------------------------------------------------------------
// SBOM & EOL
// ---------------------------------------------------------------------------

// TriggerSBOMScan enqueues a syft SBOM generation job for the given asset.
// The asset must exist and have an external_url or name that serves as the scan target.
func (s *Service) TriggerSBOMScan(ctx context.Context, orgID, assetID string) error {
	asset, err := s.repo.GetAsset(ctx, orgID, assetID)
	if err != nil {
		return fmt.Errorf("get asset: %w", err)
	}

	target := ""
	if asset.ExternalURL != nil && *asset.ExternalURL != "" {
		target = *asset.ExternalURL
	}
	if target == "" {
		target = asset.Name
	}

	payload := SBOMGeneratePayload{
		OrgID:   orgID,
		AssetID: assetID,
		Target:  target,
	}

	if s.asynqClient != nil {
		if err := EnqueueSBOMGenerate(s.asynqClient, payload); err != nil {
			return fmt.Errorf("enqueue sbom generate: %w", err)
		}
	}
	return nil
}

// GetAssetSBOM returns the latest SBOM summary and its components for an asset.
func (s *Service) GetAssetSBOM(ctx context.Context, orgID, assetID string) (*SBOMSummary, []ComponentSummary, error) {
	sbom, err := s.repo.GetLatestSBOM(ctx, orgID, assetID)
	if err != nil {
		return nil, nil, fmt.Errorf("get latest SBOM: %w", err)
	}

	compRows, err := s.repo.q.ListSPComponentsBySBOMFull(ctx, sbom.ID)
	if err != nil {
		return nil, nil, fmt.Errorf("list SBOM components: %w", err)
	}
	components := make([]ComponentSummary, 0, len(compRows))
	for _, row := range compRows {
		components = append(components, ComponentSummary{
			ID:        row.ID,
			Name:      row.Name,
			Version:   row.Version,
			PURL:      row.PURL,
			EOLStatus: row.EOLStatus,
			EOLDate:   row.EOLDate,
			AssetID:   row.AssetID,
		})
	}
	return sbom, components, nil
}

// GetScanOrgID returns the org_id of a scan without requiring the caller to know the org.
// Used by handlers for ownership verification before streaming.
func (s *Service) GetScanOrgID(ctx context.Context, scanID string) (string, error) {
	return s.repo.q.GetSPScanOrgID(ctx, scanID)
}

// GetEOLDashboard returns paginated components with their EOL status for an org,
// optionally filtered to only EOL components. page is 1-based.
func (s *Service) GetEOLDashboard(ctx context.Context, orgID string, eolOnly bool, page int) ([]ComponentSummary, error) {
	return s.repo.ListComponentsWithEOL(ctx, orgID, eolOnly, page)
}

// ListFindingsCursor returns findings using keyset pagination.
// Returns limit+1 rows so the caller can detect HasMore.
func (s *Service) ListFindingsCursor(ctx context.Context, orgID string, filter FindingFilter, cursorID string, cursorTS time.Time, limit int) ([]Finding, error) {
	return s.repo.ListFindingsCursor(ctx, orgID, filter, cursorID, cursorTS, limit)
}
