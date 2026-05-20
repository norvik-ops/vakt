package hr

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
}
