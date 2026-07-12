// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package vaktscan

import (
	"encoding/json"
	"fmt"

	"github.com/hibiken/asynq"

	"github.com/matharnica/vakt/internal/shared/queuemetrics"
)

// Job type constants for SecPulse Asynq tasks.
const (
	TaskScanTrivy         = "vaktscan:scan:trivy"
	TaskScanNuclei        = "vaktscan:scan:nuclei"
	TaskScanOpenVAS       = "vaktscan:scan:openvas"
	TaskEPSSEnrich        = "vaktscan:epss_enrich"
	TaskGenerateReport    = "vaktscan:generate_report"
	TaskAutoEvidence      = "vaktscan:auto_evidence"
	TaskSBOMGenerate      = "vaktscan:sbom:generate"
	TaskEOLCheck          = "vaktscan:eol:check"
	TaskRiskTrendSnapshot = "vaktscan:risk_trend_snapshot"
	TaskCertScan          = "vaktscan:cert_scan"
	TaskSLACheck          = "vaktscan:sla_check"

	// QueueScans is the dedicated Asynq queue for scanner jobs.
	// Higher concurrency than other module queues to avoid starving user-facing scans.
	QueueScans = "vaktscan"
	// QueueMaintenance is the queue for background SBOM/EOL maintenance tasks.
	QueueMaintenance = "maintenance"
)

// AutoEvidencePayload is the Asynq task payload for the auto-evidence job.
// It is enqueued when a Finding is resolved so that related patch-management
// controls in SecVitals receive an automated evidence entry.
type AutoEvidencePayload struct {
	OrgID     string `json:"org_id"`
	FindingID string `json:"finding_id"`
	CVE       string `json:"cve,omitempty"`
	Title     string `json:"title"`
}

// ScanPayload is the Asynq task payload for a scan job.
type ScanPayload struct {
	ScanID    string `json:"scan_id"`
	OrgID     string `json:"org_id"`
	AssetID   string `json:"asset_id"`
	AssetName string `json:"asset_name"`
	Scanner   string `json:"scanner"`
	TargetURL string `json:"target_url,omitempty"`
	TargetIP  string `json:"target_ip,omitempty"`
}

// GenerateReportPayload is the Asynq task payload for a report generation job.
type GenerateReportPayload struct {
	ReportID string      `json:"report_id"`
	OrgID    string      `json:"org_id"`
	Scope    ReportScope `json:"scope"`
}

// SBOMGeneratePayload is the Asynq task payload for a syft SBOM generation job.
type SBOMGeneratePayload struct {
	OrgID   string `json:"org_id"`
	AssetID string `json:"asset_id"`
	Target  string `json:"target"`
}

// EOLCheckPayload is the Asynq task payload for an EOL check job.
type EOLCheckPayload struct {
	OrgID  string `json:"org_id"`
	SBOMID string `json:"sbom_id"`
}

// EnqueueSBOMGenerate enqueues a syft SBOM generation job for the given asset.
// SBOM generation is a background maintenance task and runs on the "maintenance"
// queue so that it does not compete with user-facing scan jobs on "default".
func EnqueueSBOMGenerate(client *asynq.Client, payload SBOMGeneratePayload) error {
	b, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal SBOMGeneratePayload: %w", err)
	}
	task := asynq.NewTask(TaskSBOMGenerate, b)
	if _, err = client.Enqueue(task, asynq.Queue(QueueMaintenance)); err != nil {
		queuemetrics.RecordError(QueueMaintenance)
	}
	return err
}

// EnqueueEOLCheck enqueues an EOL check job for the given SBOM.
// EOL checks are background maintenance tasks and run on the "maintenance" queue
// so that they do not compete with user-facing scan jobs on "default".
func EnqueueEOLCheck(client *asynq.Client, payload EOLCheckPayload) error {
	b, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal EOLCheckPayload: %w", err)
	}
	task := asynq.NewTask(TaskEOLCheck, b)
	if _, err = client.Enqueue(task, asynq.Queue(QueueMaintenance)); err != nil {
		queuemetrics.RecordError(QueueMaintenance)
	}
	return err
}

// CertScanPayload is the Asynq task payload for the daily TLS certificate rescan.
type CertScanPayload struct {
	OrgID string `json:"org_id"`
}
