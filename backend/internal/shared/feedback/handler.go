package feedback

import (
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
)

type Handler struct {
	db *pgxpool.Pool
}

func NewHandler(db *pgxpool.Pool) *Handler {
	return &Handler{db: db}
}

type submitRequest struct {
	Rating  int    `json:"rating"`
	Message string `json:"message"`
	Name    string `json:"name"`
	Email   string `json:"email"`
	Page    string `json:"page"`
}

type feedbackRow struct {
	ID        string    `json:"id"`
	Rating    int16     `json:"rating"`
	Message   string    `json:"message"`
	Name      *string   `json:"name,omitempty"`
	Email     *string   `json:"email,omitempty"`
	Page      *string   `json:"page,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

func (h *Handler) Submit(c echo.Context) error {
	var req submitRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}
	if req.Rating < 1 || req.Rating > 5 {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "rating must be between 1 and 5"})
	}
	if req.Message == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "message is required"})
	}
	_, err := h.db.Exec(c.Request().Context(),
		`INSERT INTO demo_feedback (rating, message, name, email, page)
		 VALUES ($1, $2, NULLIF($3,''), NULLIF($4,''), NULLIF($5,''))`,
		req.Rating, req.Message, req.Name, req.Email, req.Page,
	)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to save feedback"})
	}
	return c.JSON(http.StatusCreated, map[string]string{"status": "ok"})
}

func (h *Handler) List(c echo.Context) error {
	rows, err := h.db.Query(c.Request().Context(),
		`SELECT id, rating, message, name, email, page, created_at
		 FROM demo_feedback ORDER BY created_at DESC LIMIT 500`,
	)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to fetch feedback"})
	}
	defer rows.Close()

	results := []feedbackRow{}
	for rows.Next() {
		var f feedbackRow
		if err := rows.Scan(&f.ID, &f.Rating, &f.Message, &f.Name, &f.Email, &f.Page, &f.CreatedAt); err != nil {
			continue
		}
		results = append(results, f)
	}
	return c.JSON(http.StatusOK, results)
}
