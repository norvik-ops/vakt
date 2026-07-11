package vakthr

import (
	"github.com/labstack/echo/v4"

	"github.com/matharnica/vakt/internal/auth"
)

// Register mounts all HR routes on the given Echo group.
// The group is expected to be /api/v1/hr and should already have the auth middleware applied.
func Register(g *echo.Group, h *Handler) {
	rw := auth.RequireRole("Admin", "SecurityAnalyst")
	admin := auth.RequireRole("Admin")

	// Employees — PII; read: analyst+; write/delete: admin only
	g.GET("/employees", h.ListEmployees, rw)
	g.POST("/employees", h.CreateEmployee, admin)
	g.GET("/employees/:id", h.GetEmployee, rw)
	g.PUT("/employees/:id", h.UpdateEmployee, admin)
	g.DELETE("/employees/:id", h.DeleteEmployee, admin)

	// Convenience: start onboarding/offboarding runs for a specific employee
	g.POST("/employees/:id/onboard", h.StartOnboarding, admin)
	g.POST("/employees/:id/offboard", h.StartOffboarding, admin)

	// Checklists — templates; admin only
	g.GET("/checklists", h.ListChecklists, rw)
	// S121-C4 (C7): single-template read for the checklist-run page.
	g.GET("/checklists/:id", h.GetChecklist, rw)
	g.POST("/checklists", h.CreateChecklist, admin)
	g.DELETE("/checklists/:id", h.DeleteChecklist, admin)

	// Checklist runs — admin manages; analyst can view/update progress
	g.POST("/checklist-runs", h.StartChecklistRun, admin)
	g.GET("/checklist-runs/:id", h.GetChecklistRun, rw)
	g.GET("/employees/:id/checklist-runs", h.ListChecklistRuns, rw)
	g.PUT("/checklist-runs/:id", h.UpdateChecklistRun, rw)

	// Step completion + audit trail
	g.POST("/checklist-runs/:id/steps/:step_id", h.CompleteStep, rw)
	g.GET("/checklist-runs/:id/events", h.ListRunEvents, rw)

	// Berechtigungskonzept (S60)
	// CRITICAL: static sub-paths (/roles, /versions) must be registered BEFORE bare /:id
	g.GET("/access-concepts", h.ListAccessConcepts, rw)
	g.POST("/access-concepts", h.CreateAccessConcept, admin)
	g.GET("/access-concepts/:id/roles", h.ListAccessRoles, rw)
	g.POST("/access-concepts/:id/roles", h.AddAccessRole, admin)
	g.PATCH("/access-concepts/:id/roles/:rid", h.UpdateAccessRole, admin)
	g.DELETE("/access-concepts/:id/roles/:rid", h.DeleteAccessRole, admin)
	g.POST("/access-concepts/:id/versions", h.SnapshotAccessConceptVersion, admin)
	g.GET("/access-concepts/:id/versions", h.ListAccessConceptVersions, rw)
	g.GET("/access-concepts/:id", h.GetAccessConcept, rw)
	g.PATCH("/access-concepts/:id", h.UpdateAccessConcept, admin)
	g.DELETE("/access-concepts/:id", h.DeleteAccessConcept, admin)

	// S69-4: JML Mover Workflow
	g.GET("/mover-events", h.ListMoverEvents, rw)
	g.POST("/mover-events", h.CreateMoverEvent, rw)
	g.GET("/mover-events/:id", h.GetMoverEvent, rw)
	g.PATCH("/mover-events/:id/status", h.UpdateMoverEventStatus, rw)
	g.GET("/mover-templates", h.ListMoverTemplates, rw)

	// S70-4: Contractor/Freelancer lifecycle
	g.GET("/contractors", h.ListContractors, rw)
	g.POST("/contractors", h.CreateContractor, admin)
	g.GET("/contractors/:id", h.GetContractor, rw)
	g.PUT("/contractors/:id", h.UpdateContractor, admin)
}
