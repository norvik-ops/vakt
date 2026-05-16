package hr

import "github.com/labstack/echo/v4"

// Register mounts all HR routes on the given Echo group.
// The group is expected to be /api/v1/hr and should already have the auth middleware applied.
func Register(g *echo.Group, h *Handler) {
	// Employees
	g.GET("/employees", h.ListEmployees)
	g.POST("/employees", h.CreateEmployee)
	g.GET("/employees/:id", h.GetEmployee)
	g.PUT("/employees/:id", h.UpdateEmployee)
	g.DELETE("/employees/:id", h.DeleteEmployee)
	// Checklists
	g.GET("/checklists", h.ListChecklists)
	g.POST("/checklists", h.CreateChecklist)
	g.DELETE("/checklists/:id", h.DeleteChecklist)
	// Checklist runs
	g.POST("/checklist-runs", h.StartChecklistRun)
	g.GET("/checklist-runs/:id", h.GetChecklistRun)
	g.GET("/employees/:id/checklist-runs", h.ListChecklistRuns)
	g.PUT("/checklist-runs/:id", h.UpdateChecklistRun)
}
