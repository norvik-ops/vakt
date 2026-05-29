package vaktscan

import (
	"github.com/labstack/echo/v4"
	"github.com/matharnica/vakt/internal/auth"
	"github.com/matharnica/vakt/internal/shared/platform/features"
)

// Register wires VulnBoard routes under the provided group.
//
// Feature gating:
//   - Community: asset registry, basic findings list/update, scan triggers, suppressions, SLA dashboard
//   - Pro (FeatureSecPulse): SBOM scanning, EOL tracking, report generation and export,
//     findings bulk export/import, Wazuh import
func Register(g *echo.Group, h *Handler) {
	rw := auth.RequireRole("Admin", "SecurityAnalyst")
	ro := auth.RequireRole("Admin", "SecurityAnalyst", "Viewer", "AuditorReadOnly")

	assets := g.Group("", ro)

	// --- Community: Asset registry ---
	assets.POST("/assets", h.CreateAsset, rw)
	assets.GET("/assets", h.ListAssets)
	assets.GET("/assets/:id", h.GetAsset)
	assets.PUT("/assets/:id", h.UpdateAsset, rw)
	assets.DELETE("/assets/:id", h.DeleteAsset, rw)
	assets.POST("/assets/import", h.ImportAssets, rw)

	// --- Community: Scan triggers and schedules ---
	assets.POST("/assets/:id/scans", h.TriggerScan, rw)
	assets.GET("/assets/:id/schedules", h.ListScanSchedules)
	assets.POST("/assets/:id/schedules", h.CreateScanSchedule, rw)
	assets.DELETE("/assets/:id/schedules/:schedule_id", h.DeleteScanSchedule, rw)

	// --- Community: Scans ---
	assets.GET("/scans/:id", h.GetScan)
	// Sprint 17 S17-2: SSE-Progress-Stream pro Scan. Verbraucht Redis Pub/Sub
	// vom Worker (siehe progress_stream.go). Erfordert Redis — degradiert
	// 503 wenn nicht konfiguriert.
	assets.GET("/scans/:id/progress/stream", h.StreamScanProgress)

	// --- Pro: Findings bulk export/import and Wazuh import — must be before /:id routes ---
	assets.GET("/findings/export/xlsx", h.ExportFindingsXLSX, features.Require(features.FeatureSecPulse))
	assets.GET("/findings/export", h.ExportFindings, features.Require(features.FeatureSecPulse))
	assets.POST("/findings/import", h.ImportFindings, rw, features.Require(features.FeatureSecPulse))
	assets.POST("/findings/import/csv", h.ImportFindingsCSV, rw, features.Require(features.FeatureSecPulse))
	assets.POST("/import/wazuh", h.ImportWazuh, rw, features.Require(features.FeatureSecPulse))

	// --- Pro: Assets CSV import (extended format: name,type,ip,owner,criticality) ---
	assets.POST("/assets/import/csv", h.ImportAssetsCSVNew, rw, features.Require(features.FeatureSecPulse))
	// Community: basic findings list and individual finding management
	assets.GET("/findings/bulk", h.ListFindings) // keep as list
	assets.POST("/findings/bulk", h.BulkUpdateFindings, rw)
	assets.GET("/findings", h.ListFindings)
	assets.GET("/findings/:id", h.GetFinding)
	assets.PATCH("/findings/:id", h.UpdateFinding, rw)

	// --- Community: Suppression rules ---
	assets.GET("/suppressions", h.ListSuppressions)
	assets.POST("/suppressions", h.CreateSuppression, rw)
	assets.DELETE("/suppressions/:id", h.DeleteSuppression, rw)

	// --- Community: SLA configuration and dashboard ---
	assets.GET("/sla-dashboard", h.GetSLADashboard)
	assets.GET("/sla-config", h.GetSLAConfig)
	assets.PUT("/sla-config", h.UpdateSLAConfig, auth.RequireRole("Admin"))

	// --- Pro: Report generation and export ---
	assets.GET("/reports/risk-trend", h.GetRiskTrend, features.Require(features.FeatureSecPulse))
	assets.POST("/reports", h.GenerateReport, rw, features.Require(features.FeatureSecPulse))
	assets.GET("/reports", h.ListReports, features.Require(features.FeatureSecPulse))
	assets.GET("/reports/:id", h.GetReport, features.Require(features.FeatureSecPulse))
	assets.GET("/reports/:id/download", h.DownloadReport, features.Require(features.FeatureSecPulse))

	// --- Pro: SBOM generation and EOL tracking ---
	assets.POST("/assets/:id/sbom", h.TriggerSBOMScan, rw, features.Require(features.FeatureSecPulse))
	assets.GET("/assets/:id/sbom", h.GetAssetSBOM, features.Require(features.FeatureSecPulse))
	assets.GET("/sbom/eol", h.GetEOLDashboard, features.Require(features.FeatureSecPulse))

	// --- Community: CI/CD evidence webhook (push model for any CI system) ---
	assets.POST("/ci-evidence", h.ReceiveCIEvidence, rw)

	// --- Community: Scanner availability status ---
	assets.GET("/scanner-status", h.GetScannerStatus)
}
