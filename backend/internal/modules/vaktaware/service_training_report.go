package vaktaware

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/go-pdf/fpdf"
	"github.com/hibiken/asynq"
	"github.com/rs/zerolog/log"

	"github.com/matharnica/vakt/internal/services/crossevidence"
	"github.com/matharnica/vakt/internal/shared/platform/events"
	"github.com/matharnica/vakt/internal/shared/queuemetrics"
)

// GenerateTrainingMatrixReport builds the structured report for the given org and period.
func (s *Service) GenerateTrainingMatrixReport(ctx context.Context, orgID string, from, to time.Time) (*TrainingMatrixReport, error) {
	campaigns, err := s.repo.ListCampaignSummariesForReport(ctx, orgID, from, to)
	if err != nil {
		return nil, fmt.Errorf("campaign summaries: %w", err)
	}
	trainings, err := s.repo.CountCompletedTrainingsInPeriod(ctx, orgID, from, to)
	if err != nil {
		return nil, fmt.Errorf("training count: %w", err)
	}

	orgName := s.repo.GetOrganizationName(ctx, orgID)

	var totalParticipants int
	var sumClickRate float64
	for _, c := range campaigns {
		totalParticipants += c.RecipientCount
		sumClickRate += c.ClickRate
	}
	var avgClickRate float64
	if len(campaigns) > 0 {
		avgClickRate = sumClickRate / float64(len(campaigns))
	}

	orp3 := s.computeORP3Compliance(ctx, orgID, from, to)

	report := &TrainingMatrixReport{
		Period:      ReportPeriod{From: from, To: to},
		OrgName:     orgName,
		Campaigns:   campaigns,
		GeneratedAt: time.Now().UTC(),
		TotalStats: AwareStats{
			TotalCampaigns:          len(campaigns),
			TotalParticipants:       totalParticipants,
			AvgClickRate:            avgClickRate,
			TotalTrainingsCompleted: trainings,
		},
		BSICompliance: orp3,
	}
	return report, nil
}

// ExportTrainingMatrixPDF renders a training-matrix audit report as PDF.
func (s *Service) ExportTrainingMatrixPDF(ctx context.Context, orgID string, from, to time.Time) ([]byte, error) {
	report, err := s.GenerateTrainingMatrixReport(ctx, orgID, from, to)
	if err != nil {
		return nil, err
	}
	pdfBytes, err := generateTrainingMatrixPDF(report)
	if err != nil {
		return nil, err
	}
	// Auto-evidence: record the export in vaktcomply (best-effort).
	s.recordTrainingReportEvidence(ctx, orgID, report)
	return pdfBytes, nil
}

// ExportTrainingMatrixCSV returns a CSV representation of the training matrix.
func (s *Service) ExportTrainingMatrixCSV(ctx context.Context, orgID string, from, to time.Time) ([]byte, error) {
	report, err := s.GenerateTrainingMatrixReport(ctx, orgID, from, to)
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	buf.WriteString("Kampagne,Zeitraum Start,Zeitraum Ende,Teilnehmer,Klickrate\n")
	for _, c := range report.Campaigns {
		fmt.Fprintf(&buf, "%q,%s,%s,%d,%.2f\n",
			c.Name, c.StartedAt, c.CompletedAt, c.RecipientCount, c.ClickRate)
	}
	s.recordTrainingReportEvidence(ctx, orgID, report)
	return buf.Bytes(), nil
}

// recordTrainingReportEvidence enqueues a cross-module evidence record.
func (s *Service) recordTrainingReportEvidence(ctx context.Context, orgID string, report *TrainingMatrixReport) {
	if s.asynqClient == nil {
		return
	}
	resourceID := fmt.Sprintf("training-report-%d", report.GeneratedAt.Year())
	title := fmt.Sprintf("Awareness-Training-Report %d exportiert — %d Kampagnen, %d Teilnehmer",
		report.GeneratedAt.Year(), report.TotalStats.TotalCampaigns, report.TotalStats.TotalParticipants)
	ev := events.CrossModuleEvent{
		OrgID:        orgID,
		Source:       events.SourceSecreflex,
		ResourceType: "vakt-aware/training-report-exported",
		ResourceID:   resourceID,
		Title:        title,
		Description:  fmt.Sprintf("BSI ORP.3: %d/%d Anforderungen erfüllt", report.BSICompliance.FulfilledCount, report.BSICompliance.TotalCount),
		OccurredAt:   report.GeneratedAt,
	}
	if task, err := crossevidence.NewRecordEvidenceTask(ev); err == nil {
		if _, err := s.asynqClient.EnqueueContext(ctx, task); err != nil {
			queuemetrics.RecordError(crossevidence.Queue)
			log.Warn().Err(err).Str("org_id", orgID).Msg("training report evidence enqueue failed")
		}
	}
}

// GetORP3Status returns the BSI ORP.3 compliance overview for the past 12 months.
func (s *Service) GetORP3Status(ctx context.Context, orgID string) (*BSIOrp3Compliance, error) {
	to := time.Now().UTC()
	from := to.AddDate(-1, 0, 0)
	orp3 := s.computeORP3Compliance(ctx, orgID, from, to)
	return &orp3, nil
}

// computeORP3Compliance evaluates all 8 ORP.3 requirements for the given period.
func (s *Service) computeORP3Compliance(ctx context.Context, orgID string, from, to time.Time) BSIOrp3Compliance {
	hasCampaign, _ := s.repo.HasCampaignInPeriod(ctx, orgID, from, to)
	hasNewEmpRule, _ := s.repo.HasActiveNewEmployeeRule(ctx, orgID)

	trainings, _ := s.repo.CountCompletedTrainingsInPeriod(ctx, orgID, from, to)
	hasTrainings := trainings > 0

	reqs := []ORP3Requirement{
		{ID: "ORP.3.A1", Title: "Sensibilisierung der Mitarbeiter für Informationssicherheit", Fulfilled: hasCampaign},
		{ID: "ORP.3.A2", Title: "Schulungsplan für Informationssicherheit", Fulfilled: hasNewEmpRule},
		{ID: "ORP.3.A3", Title: "Einweisung neuer Mitarbeiter", Fulfilled: hasNewEmpRule},
		{ID: "ORP.3.A4", Title: "Schulungsmaßnahmen zu Informationssicherheit", Fulfilled: hasTrainings},
		{ID: "ORP.3.A5", Title: "Analyse des Schulungsbedarfs", Fulfilled: hasNewEmpRule},
		{ID: "ORP.3.A6", Title: "Schulung auf Leitungsebene", Fulfilled: hasCampaign},
		{ID: "ORP.3.A7", Title: "Regelmäßige Überprüfung der Schulungsmaßnahmen", Fulfilled: hasCampaign && hasTrainings},
		{ID: "ORP.3.A8", Title: "Messung und Dokumentation der Schulungsmetriken", Fulfilled: hasCampaign},
	}

	fulfilled := 0
	for _, r := range reqs {
		if r.Fulfilled {
			fulfilled++
		}
	}
	return BSIOrp3Compliance{
		FulfilledCount: fulfilled,
		TotalCount:     len(reqs),
		Requirements:   reqs,
	}
}

// ── Asynq task names for new sprint-65 tasks ─────────────────────────────

const TaskORP3EvidenceSync = "aware:orp3_evidence_sync"

// ORP3EvidenceSyncPayload is the payload for the daily ORP.3 evidence job.
type ORP3EvidenceSyncPayload struct {
	OrgID string `json:"org_id"`
}

// RunORP3EvidenceSync evaluates ORP.3 for the org and writes evidence for
// fulfilled requirements into vaktcomply (best-effort).
func (s *Service) RunORP3EvidenceSync(ctx context.Context, orgID string) error {
	to := time.Now().UTC()
	from := to.AddDate(-1, 0, 0)
	orp3 := s.computeORP3Compliance(ctx, orgID, from, to)
	if s.asynqClient == nil {
		return nil
	}
	for _, req := range orp3.Requirements {
		if !req.Fulfilled {
			continue
		}
		ev := events.CrossModuleEvent{
			OrgID:        orgID,
			Source:       events.SourceSecreflex,
			ResourceType: "vakt-aware/bsi-orp3-evidence",
			ResourceID:   req.ID,
			Title:        fmt.Sprintf("BSI %s erfüllt: %s", req.ID, req.Title),
			Description:  fmt.Sprintf("Vakt Aware erfüllt BSI IT-Grundschutz %s automatisch.", req.ID),
			OccurredAt:   to,
		}
		if task, err := crossevidence.NewRecordEvidenceTask(ev); err == nil {
			if _, enqErr := s.asynqClient.EnqueueContext(ctx, task, asynq.Queue(Queue)); enqErr != nil {
				queuemetrics.RecordError(Queue)
				log.Warn().Err(enqErr).Str("req_id", req.ID).Msg("orp3 evidence enqueue failed")
			}
		}
	}
	return nil
}

// EnqueueORP3EvidenceSync schedules the daily ORP.3 evidence sync for an org.
func (s *Service) EnqueueORP3EvidenceSync(ctx context.Context, orgID string) error {
	if s.asynqClient == nil {
		return nil
	}
	data, _ := json.Marshal(ORP3EvidenceSyncPayload{OrgID: orgID})
	task := asynq.NewTask(TaskORP3EvidenceSync, data)
	if _, err := s.asynqClient.EnqueueContext(ctx, task, asynq.Queue(Queue)); err != nil {
		queuemetrics.RecordError(Queue)
		return err
	}
	return nil
}

// generateTrainingMatrixPDF renders the training matrix report as PDF using fpdf.
func generateTrainingMatrixPDF(r *TrainingMatrixReport) ([]byte, error) {
	pdf := fpdf.New("P", "mm", "A4", "")
	pdf.SetMargins(15, 15, 15)
	pdf.SetAutoPageBreak(true, 15)
	pdf.AliasNbPages("{nb}")
	pdf.AddPage()

	// ── Header ────────────────────────────────────────────────────────────────
	pdf.SetFillColor(37, 99, 235)
	pdf.Rect(0, 0, 210, 28, "F")
	pdf.SetTextColor(255, 255, 255)
	pdf.SetFont("Helvetica", "B", 14)
	pdf.SetXY(15, 8)
	pdf.CellFormat(180, 8, "Vakt Aware — Trainings-Evidence-Report (ISO 27001 / BSI ORP.3)", "", 1, "L", false, 0, "")
	pdf.SetFont("Helvetica", "", 9)
	pdf.SetXY(15, 17)
	pdf.CellFormat(180, 6, r.OrgName, "", 1, "L", false, 0, "")

	pdf.SetTextColor(30, 30, 40)
	pdf.SetXY(15, 35)
	pdf.SetFont("Helvetica", "B", 13)
	pdf.CellFormat(180, 8, fmt.Sprintf("Berichtszeitraum: %s – %s",
		r.Period.From.Format("02.01.2006"), r.Period.To.Format("02.01.2006")), "", 1, "L", false, 0, "")
	pdf.SetFont("Helvetica", "", 9)
	pdf.SetTextColor(100, 100, 120)
	pdf.CellFormat(180, 6, fmt.Sprintf("Erstellt am %s (anonymisiert gem. DSGVO/§87 BetrVG)", r.GeneratedAt.Format("02.01.2006 15:04")), "", 1, "L", false, 0, "")

	// ── Summary boxes ─────────────────────────────────────────────────────────
	pdf.SetY(pdf.GetY() + 6)
	pdf.SetFont("Helvetica", "B", 11)
	pdf.SetTextColor(30, 30, 40)
	pdf.CellFormat(180, 7, "Zusammenfassung", "", 1, "L", false, 0, "")

	type box struct {
		label, val string
		r, g, b    int
	}
	boxes := []box{
		{"Kampagnen", fmt.Sprintf("%d", r.TotalStats.TotalCampaigns), 37, 99, 235},
		{"Teilnehmer", fmt.Sprintf("%d", r.TotalStats.TotalParticipants), 55, 65, 81},
		{"Ø Klickrate", fmt.Sprintf("%.1f%%", r.TotalStats.AvgClickRate), 234, 88, 12},
		{"Trainings abgeschl.", fmt.Sprintf("%d", r.TotalStats.TotalTrainingsCompleted), 22, 163, 74},
	}
	const bw, gap, bsx = 40.0, 3.0, 15.0
	by := pdf.GetY() + 3
	for i, b := range boxes {
		bx := bsx + float64(i)*(bw+gap)
		pdf.SetFillColor(b.r, b.g, b.b)
		pdf.RoundedRect(bx, by, bw, 18, 2, "1234", "F")
		pdf.SetTextColor(255, 255, 255)
		pdf.SetFont("Helvetica", "B", 12)
		pdf.SetXY(bx, by+2)
		pdf.CellFormat(bw, 7, b.val, "", 1, "C", false, 0, "")
		pdf.SetFont("Helvetica", "", 7)
		pdf.SetXY(bx, by+10)
		pdf.CellFormat(bw, 5, b.label, "", 1, "C", false, 0, "")
	}

	// ── BSI ORP.3 compliance badge ────────────────────────────────────────────
	pdf.SetY(by + 25)
	pdf.SetTextColor(30, 30, 40)
	pdf.SetFont("Helvetica", "B", 11)
	pdf.CellFormat(180, 7, "BSI ORP.3 Compliance", "", 1, "L", false, 0, "")

	orp3Y := pdf.GetY()
	pdf.SetFillColor(240, 253, 244)
	pdf.Rect(15, orp3Y, 180, 12, "F")
	pdf.SetTextColor(22, 101, 52)
	pdf.SetFont("Helvetica", "B", 10)
	pdf.SetXY(18, orp3Y+2)
	pdf.CellFormat(174, 6,
		fmt.Sprintf("Anforderungen erfüllt: %d/%d", r.BSICompliance.FulfilledCount, r.BSICompliance.TotalCount),
		"", 1, "L", false, 0, "")
	pdf.SetFont("Helvetica", "", 8)
	pdf.SetXY(18, pdf.GetY())
	var fulfilled []string
	for _, req := range r.BSICompliance.Requirements {
		if req.Fulfilled {
			fulfilled = append(fulfilled, req.ID)
		}
	}
	if len(fulfilled) > 0 {
		listed := fulfilled
		if len(listed) > 6 {
			listed = listed[:6]
		}
		pdf.CellFormat(174, 5, "Erfüllt: "+joinStrings(listed, ", "), "", 1, "L", false, 0, "")
	}

	// ── Campaign table ────────────────────────────────────────────────────────
	pdf.SetY(pdf.GetY() + 5)
	pdf.SetTextColor(30, 30, 40)
	pdf.SetFont("Helvetica", "B", 11)
	pdf.CellFormat(180, 7, "Kampagnen-Übersicht (anonymisiert)", "", 1, "L", false, 0, "")

	// Table header
	pdf.SetFillColor(37, 99, 235)
	pdf.SetTextColor(255, 255, 255)
	pdf.SetFont("Helvetica", "B", 8)
	colW := []float64{70, 45, 30, 35}
	headers := []string{"Kampagne", "Abgeschlossen", "Teilnehmer", "Klickrate"}
	for i, h := range headers {
		pdf.CellFormat(colW[i], 6, h, "1", 0, "C", true, 0, "")
	}
	pdf.Ln(-1)

	// Table rows
	pdf.SetFont("Helvetica", "", 8)
	pdf.SetTextColor(30, 30, 40)
	fill := false
	for _, c := range r.Campaigns {
		if pdf.GetY() > 265 {
			pdf.AddPage()
		}
		if fill {
			pdf.SetFillColor(245, 247, 250)
		} else {
			pdf.SetFillColor(255, 255, 255)
		}
		name := c.Name
		if len(name) > 35 {
			name = name[:32] + "..."
		}
		completed := c.CompletedAt
		if len(completed) > 10 {
			completed = completed[:10]
		}
		pdf.CellFormat(colW[0], 6, name, "1", 0, "L", fill, 0, "")
		pdf.CellFormat(colW[1], 6, completed, "1", 0, "C", fill, 0, "")
		pdf.CellFormat(colW[2], 6, fmt.Sprintf("%d", c.RecipientCount), "1", 0, "C", fill, 0, "")
		pdf.CellFormat(colW[3], 6, fmt.Sprintf("%.1f%%", c.ClickRate), "1", 1, "C", fill, 0, "")
		fill = !fill
	}
	if len(r.Campaigns) == 0 {
		pdf.CellFormat(180, 6, "Keine abgeschlossenen Kampagnen im Berichtszeitraum", "1", 1, "C", false, 0, "")
	}

	// ── Footer ────────────────────────────────────────────────────────────────
	pdf.SetFooterFunc(func() {
		pdf.SetY(-12)
		pdf.SetFont("Helvetica", "I", 7)
		pdf.SetTextColor(150, 150, 160)
		pdf.CellFormat(0, 5,
			fmt.Sprintf("Vakt Aware Evidence Report — %s — Seite %d/{nb}", r.OrgName, pdf.PageNo()),
			"", 0, "C", false, 0, "")
	})

	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, fmt.Errorf("pdf output: %w", err)
	}
	return buf.Bytes(), nil
}

func joinStrings(ss []string, sep string) string {
	var b bytes.Buffer
	for i, s := range ss {
		if i > 0 {
			b.WriteString(sep)
		}
		b.WriteString(s)
	}
	return b.String()
}
