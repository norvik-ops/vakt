package hr

import (
	"net/http"

	"github.com/go-playground/validator/v10"
	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog/log"

	"github.com/matharnica/vakt/internal/shared/pagination"
)

// Handler handles HTTP requests for the HR module.
type Handler struct {
	Service  *Service
	validate *validator.Validate
}

// NewHandler creates a new HR handler with an initialised validator.
func NewHandler(svc *Service) *Handler {
	return &Handler{
		Service:  svc,
		validate: validator.New(),
	}
}

// actorFrom assembles the Actor record the service uses to attribute audit-log
// entries. Audit-write now lives in the service (P2-19 / ADR pattern) so that
// non-HTTP callers (workers, CLI, SDK) get the same audit trail.
func actorFrom(c echo.Context) Actor {
	return Actor{
		OrgID:     orgID(c),
		UserID:    userID(c),
		UserEmail: userEmail(c),
		IPAddress: c.RealIP(),
	}
}

func userID(c echo.Context) string {
	v, _ := c.Get("user_id").(string)
	return v
}

func userEmail(c echo.Context) string {
	v, _ := c.Get("user_email").(string)
	return v
}

// orgID extracts the authenticated organisation ID from the Echo context.
func orgID(c echo.Context) string {
	v, _ := c.Get("org_id").(string)
	return v
}

// errResp writes a consistent JSON error response.
func errResp(c echo.Context, code int, msg, errCode string) error {
	return c.JSON(code, map[string]string{"error": msg, "code": errCode})
}

// --- Employees ---

// ListEmployees handles GET /api/v1/hr/employees.
func (h *Handler) ListEmployees(c echo.Context) error {
	offset, limit, meta := pagination.FromRequest(c)
	employees, total, err := h.Service.ListEmployeesPaged(c.Request().Context(), orgID(c), offset, limit)
	if err != nil {
		log.Error().Err(err).Msg("list employees")
		return errResp(c, http.StatusInternalServerError, "failed to list employees", "HR_LIST_EMPLOYEES_FAILED")
	}
	pagination.Complete(&meta, total)
	return c.JSON(http.StatusOK, pagination.Wrap(employees, meta))
}

// GetEmployee handles GET /api/v1/hr/employees/:id.
func (h *Handler) GetEmployee(c echo.Context) error {
	employee, err := h.Service.GetEmployee(c.Request().Context(), orgID(c), c.Param("id"))
	if err != nil {
		return errResp(c, http.StatusNotFound, "employee not found", "HR_EMPLOYEE_NOT_FOUND")
	}
	return c.JSON(http.StatusOK, employee)
}

// CreateEmployee handles POST /api/v1/hr/employees.
func (h *Handler) CreateEmployee(c echo.Context) error {
	var in CreateEmployeeInput
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "HR_INVALID_BODY")
	}
	if err := h.validate.Struct(in); err != nil {
		return errResp(c, http.StatusUnprocessableEntity, "Ungültige Eingabe", "VALIDATION_ERROR")
	}
	employee, err := h.Service.CreateEmployee(c.Request().Context(), actorFrom(c), in)
	if err != nil {
		log.Error().Err(err).Msg("create employee")
		return errResp(c, http.StatusInternalServerError, "failed to create employee", "HR_CREATE_EMPLOYEE_FAILED")
	}
	return c.JSON(http.StatusCreated, employee)
}

// UpdateEmployee handles PUT /api/v1/hr/employees/:id.
func (h *Handler) UpdateEmployee(c echo.Context) error {
	var in UpdateEmployeeInput
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "HR_INVALID_BODY")
	}
	if err := h.validate.Struct(in); err != nil {
		return errResp(c, http.StatusUnprocessableEntity, "Ungültige Eingabe", "VALIDATION_ERROR")
	}
	employee, err := h.Service.UpdateEmployee(c.Request().Context(), actorFrom(c), c.Param("id"), in)
	if err != nil {
		log.Error().Err(err).Msg("update employee")
		return errResp(c, http.StatusInternalServerError, "failed to update employee", "HR_UPDATE_EMPLOYEE_FAILED")
	}
	return c.JSON(http.StatusOK, employee)
}

// DeleteEmployee handles DELETE /api/v1/hr/employees/:id.
func (h *Handler) DeleteEmployee(c echo.Context) error {
	id := c.Param("id")
	if err := h.Service.DeleteEmployee(c.Request().Context(), actorFrom(c), id); err != nil {
		log.Error().Err(err).Msg("delete employee")
		return errResp(c, http.StatusInternalServerError, "failed to delete employee", "HR_DELETE_EMPLOYEE_FAILED")
	}
	return c.NoContent(http.StatusNoContent)
}

// --- Checklists ---

// ListChecklists handles GET /api/v1/hr/checklists.
func (h *Handler) ListChecklists(c echo.Context) error {
	checklists, err := h.Service.ListChecklists(c.Request().Context(), orgID(c))
	if err != nil {
		log.Error().Err(err).Msg("list checklists")
		return errResp(c, http.StatusInternalServerError, "failed to list checklists", "HR_LIST_CHECKLISTS_FAILED")
	}
	return c.JSON(http.StatusOK, checklists)
}

// CreateChecklist handles POST /api/v1/hr/checklists.
func (h *Handler) CreateChecklist(c echo.Context) error {
	var in CreateChecklistInput
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "HR_INVALID_BODY")
	}
	if err := h.validate.Struct(in); err != nil {
		return errResp(c, http.StatusUnprocessableEntity, "Ungültige Eingabe", "VALIDATION_ERROR")
	}
	checklist, err := h.Service.CreateChecklist(c.Request().Context(), actorFrom(c), in)
	if err != nil {
		log.Error().Err(err).Msg("create checklist")
		return errResp(c, http.StatusInternalServerError, "failed to create checklist", "HR_CREATE_CHECKLIST_FAILED")
	}
	return c.JSON(http.StatusCreated, checklist)
}

// DeleteChecklist handles DELETE /api/v1/hr/checklists/:id.
func (h *Handler) DeleteChecklist(c echo.Context) error {
	if err := h.Service.DeleteChecklist(c.Request().Context(), actorFrom(c), c.Param("id")); err != nil {
		log.Error().Err(err).Msg("delete checklist")
		return errResp(c, http.StatusInternalServerError, "failed to delete checklist", "HR_DELETE_CHECKLIST_FAILED")
	}
	return c.NoContent(http.StatusNoContent)
}

// --- Checklist Runs ---

// StartChecklistRun handles POST /api/v1/hr/checklist-runs.
func (h *Handler) StartChecklistRun(c echo.Context) error {
	var in StartChecklistRunInput
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "HR_INVALID_BODY")
	}
	if err := h.validate.Struct(in); err != nil {
		return errResp(c, http.StatusUnprocessableEntity, "Ungültige Eingabe", "VALIDATION_ERROR")
	}
	run, err := h.Service.StartChecklistRun(c.Request().Context(), actorFrom(c), in)
	if err != nil {
		log.Error().Err(err).Msg("start checklist run")
		return errResp(c, http.StatusInternalServerError, "failed to start checklist run", "HR_START_RUN_FAILED")
	}
	return c.JSON(http.StatusCreated, run)
}

// GetChecklistRun handles GET /api/v1/hr/checklist-runs/:id.
func (h *Handler) GetChecklistRun(c echo.Context) error {
	run, err := h.Service.GetChecklistRun(c.Request().Context(), orgID(c), c.Param("id"))
	if err != nil {
		return errResp(c, http.StatusNotFound, "checklist run not found", "HR_RUN_NOT_FOUND")
	}
	return c.JSON(http.StatusOK, run)
}

// ListChecklistRuns handles GET /api/v1/hr/employees/:id/checklist-runs.
func (h *Handler) ListChecklistRuns(c echo.Context) error {
	runs, err := h.Service.ListChecklistRuns(c.Request().Context(), orgID(c), c.Param("id"))
	if err != nil {
		log.Error().Err(err).Msg("list checklist runs")
		return errResp(c, http.StatusInternalServerError, "failed to list checklist runs", "HR_LIST_RUNS_FAILED")
	}
	return c.JSON(http.StatusOK, runs)
}

// UpdateChecklistRun handles PUT /api/v1/hr/checklist-runs/:id.
func (h *Handler) UpdateChecklistRun(c echo.Context) error {
	var in UpdateChecklistRunInput
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "HR_INVALID_BODY")
	}
	if err := h.validate.Struct(in); err != nil {
		return errResp(c, http.StatusUnprocessableEntity, "Ungültige Eingabe", "VALIDATION_ERROR")
	}
	run, err := h.Service.UpdateChecklistRun(c.Request().Context(), actorFrom(c), c.Param("id"), in)
	if err != nil {
		log.Error().Err(err).Msg("update checklist run")
		return errResp(c, http.StatusInternalServerError, "failed to update checklist run", "HR_UPDATE_RUN_FAILED")
	}
	return c.JSON(http.StatusOK, run)
}

// CompleteStep handles POST /api/v1/hr/checklist-runs/:id/steps/:step_id.
// Marks a single step as completed by the calling user. Idempotent.
func (h *Handler) CompleteStep(c echo.Context) error {
	runID := c.Param("id")
	stepID := c.Param("step_id")
	completedBy := userEmail(c)
	if completedBy == "" {
		completedBy = userID(c)
	}
	run, err := h.Service.CompleteStep(c.Request().Context(), actorFrom(c), runID, stepID, completedBy)
	if err != nil {
		log.Error().Err(err).Str("run_id", runID).Str("step_id", stepID).Msg("complete step")
		return errResp(c, http.StatusBadRequest, err.Error(), "HR_COMPLETE_STEP_FAILED")
	}
	return c.JSON(http.StatusOK, run)
}

// ListRunEvents handles GET /api/v1/hr/checklist-runs/:id/events.
// Returns the step-completion audit trail for a run.
func (h *Handler) ListRunEvents(c echo.Context) error {
	events, err := h.Service.ListRunEvents(c.Request().Context(), orgID(c), c.Param("id"))
	if err != nil {
		log.Error().Err(err).Msg("list run events")
		return errResp(c, http.StatusInternalServerError, "failed to list run events", "HR_LIST_RUN_EVENTS_FAILED")
	}
	return c.JSON(http.StatusOK, events)
}

// StartOnboarding handles POST /api/v1/hr/employees/:id/onboard.
// Finds the first onboarding checklist for the org and starts a run for the employee.
func (h *Handler) StartOnboarding(c echo.Context) error {
	employeeID := c.Param("id")
	run, err := h.Service.StartOnboarding(c.Request().Context(), actorFrom(c), employeeID)
	if err != nil {
		log.Error().Err(err).Str("employee_id", employeeID).Msg("start onboarding")
		return errResp(c, http.StatusBadRequest, err.Error(), "HR_START_ONBOARDING_FAILED")
	}
	return c.JSON(http.StatusCreated, run)
}

// StartOffboarding handles POST /api/v1/hr/employees/:id/offboard.
// Sets the employee's status to "offboarding" and starts an offboarding checklist run.
func (h *Handler) StartOffboarding(c echo.Context) error {
	employeeID := c.Param("id")
	run, err := h.Service.StartOffboarding(c.Request().Context(), actorFrom(c), employeeID)
	if err != nil {
		log.Error().Err(err).Str("employee_id", employeeID).Msg("start offboarding")
		return errResp(c, http.StatusBadRequest, err.Error(), "HR_START_OFFBOARDING_FAILED")
	}
	return c.JSON(http.StatusCreated, run)
}
