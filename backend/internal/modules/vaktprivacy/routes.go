package vaktprivacy

import (
	"github.com/labstack/echo/v4"
	"github.com/matharnica/vakt/internal/shared/platform/features"
)

// Register wires PrivacyOps routes under the provided group.
func Register(g *echo.Group, h ...*Handler) {
	var handler *Handler
	if len(h) > 0 && h[0] != nil {
		handler = h[0]
	} else {
		handler = &Handler{}
	}
	registerRoutes(g, handler)
}

// RegisterPublic wires unauthenticated DSR portal routes (no Bearer auth required).
func RegisterPublic(g *echo.Group, h *Handler) {
	g.GET("/dsr-portal/status/:token", h.PortalGetDSRStatus)
	g.GET("/dsr-portal/:slug/info", h.PortalGetInfo)
	g.POST("/dsr-portal/:slug/submit", h.PortalSubmitDSR)
}

func registerRoutes(g *echo.Group, h *Handler) {
	// VVT (Art. 30 DSGVO)
	g.GET("/vvt", h.ListVVT)
	g.POST("/vvt", h.CreateVVT)
	g.GET("/vvt/export", h.ExportVVT) // must be before /vvt/:id
	g.GET("/vvt/:id", h.GetVVT)
	g.PUT("/vvt/:id", h.UpdateVVT)
	g.DELETE("/vvt/:id", h.DeleteVVT)

	// DPIA (Art. 35 DSGVO)
	g.GET("/dpias", h.ListDPIAs)
	g.POST("/dpias", h.CreateDPIA)
	// DPIA PDF export is a Pro feature — basic DPIA management remains Community.
	g.GET("/dpias/export", h.ExportDPIA, features.Require(features.FeatureAuditPDF)) // must be before /dpias/:id
	g.GET("/dpias/:id", h.GetDPIA)
	g.PUT("/dpias/:id", h.UpdateDPIA)
	g.POST("/dpias/:id/approve", h.ApproveDPIA)
	g.DELETE("/dpias/:id", h.DeleteDPIA)

	// AVV (Art. 28 DSGVO) — static routes must come before /:id
	g.GET("/avv-templates", h.ListAVVTemplates)
	g.GET("/scc-modules", h.ListSCCModules)
	g.GET("/avvs", h.ListAVVs)
	g.POST("/avvs", h.CreateAVV)
	g.POST("/avvs/from-template", h.CreateAVVFromTemplate) // must be before /avvs/:id
	g.GET("/avvs/:id", h.GetAVV)
	g.PUT("/avvs/:id", h.UpdateAVV)
	g.DELETE("/avvs/:id", h.DeleteAVV)
	// AVV and SCC PDF export are Pro features — basic AVV management remains Community.
	g.GET("/avvs/:id/pdf", h.ExportAVVPDF, features.Require(features.FeatureAuditPDF))
	g.PATCH("/avvs/:id/scc", h.UpdateAVVSCC)
	g.GET("/avvs/:id/scc.pdf", h.ExportSCCPDF, features.Require(features.FeatureAuditPDF))

	// Breach Notifications (Art. 33/34 DSGVO)
	g.GET("/breaches", h.ListBreaches)
	g.POST("/breaches", h.CreateBreach)
	g.GET("/breaches/:id", h.GetBreach)
	g.PUT("/breaches/:id", h.UpdateBreach)
	g.DELETE("/breaches/:id", h.DeleteBreach)
	g.POST("/breaches/:id/notify-authority", h.MarkAuthorityNotified)
	g.GET("/breaches/:id/notification-pdf", h.ExportBreachNotification)

	// DSR — Data Subject Requests (Art. 15-21 DSGVO)
	g.GET("/dsr", h.ListDSRs)
	g.POST("/dsr", h.CreateDSR)
	// CRITICAL: static sub-paths must come before /dsr/:id to avoid param capture
	g.GET("/dsrs/export.csv", h.ExportDSRsCSV)
	g.GET("/dsr/summary", h.GetDSRSummary)
	g.GET("/dsr/export", h.ExportDSRLog)
	g.PUT("/dsr/:id", h.UpdateDSR)
	g.DELETE("/dsr/:id", h.DeleteDSR)
	g.POST("/dsr/:id/resolve", h.ResolveDSR)
	g.PATCH("/dsr/:id/assign", h.AssignDSR)

	// Retention / deletion reminders (S68-5)
	g.GET("/retention/summary", h.GetRetentionSummary)
	g.GET("/retention-templates", h.ListRetentionTemplates)
	g.GET("/deletion-reminders", h.ListDeletionReminders)
	g.POST("/deletion-reminders", h.CreateDeletionReminder)
	g.PATCH("/deletion-reminders/:id/complete", h.CompleteDeletionReminder)
	g.GET("/processing-activities/:id/retention", h.GetRetentionInfo)
	g.PUT("/processing-activities/:id/retention", h.UpdateRetentionInfo)

	// S69-6: Transfer Impact Assessment (TIA / Schrems II)
	if h.tia != nil {
		g.GET("/adequacy-decisions", h.ListAdequacyDecisions)
		// CRITICAL: /transfers/compliance must be registered BEFORE /transfers/:id to avoid param conflict.
		g.GET("/transfers/compliance", h.GetTransferComplianceStatus)
		g.GET("/transfers", h.ListDataTransfers)
		g.POST("/transfers", h.CreateDataTransfer)
		g.GET("/transfers/:id/tia", h.ListTIAs)
		g.POST("/transfers/:id/tia", h.CreateTIA)
	}

	// S70-3: Privacy by Design (Art. 25 DSGVO)
	// CRITICAL: /privacy-design/summary must be registered BEFORE /processing-activities/:id/privacy-design.
	g.GET("/privacy-design/summary", h.GetPrivacyDesignSummary)
	g.GET("/processing-activities/:id/privacy-design", h.GetPrivacyDesign)
	g.POST("/processing-activities/:id/privacy-design", h.CreateOrUpdatePrivacyDesign)
}
