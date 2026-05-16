// Package pagination provides helpers for offset-based pagination.
package pagination

import (
	"math"
	"strconv"

	"github.com/labstack/echo/v4"
)

// DefaultLimit is the page size used when none is specified.
const DefaultLimit = 25

// MaxLimit is the maximum page size a caller may request.
const MaxLimit = 100

// Meta holds pagination metadata returned alongside list data.
type Meta struct {
	Page       int `json:"page"`
	Limit      int `json:"limit"`
	Total      int `json:"total"`
	TotalPages int `json:"total_pages"`
}

// Response wraps a typed slice with pagination metadata.
type Response[T any] struct {
	Data       []T  `json:"data"`
	Pagination Meta `json:"pagination"`
}

// FromRequest extracts and validates page/limit from query params.
// Returns the SQL OFFSET, the validated LIMIT, and a partially-filled Meta
// (Total and TotalPages are set by Complete after the count query).
func FromRequest(c echo.Context) (offset, limit int, meta Meta) {
	page, _ := strconv.Atoi(c.QueryParam("page"))
	lim, _ := strconv.Atoi(c.QueryParam("limit"))
	if page < 1 {
		page = 1
	}
	if lim < 1 || lim > MaxLimit {
		lim = DefaultLimit
	}
	return (page - 1) * lim, lim, Meta{Page: page, Limit: lim}
}

// Complete fills in Total and TotalPages from the count query result.
func Complete(m *Meta, total int) {
	m.Total = total
	m.TotalPages = int(math.Ceil(float64(total) / float64(m.Limit)))
	if m.TotalPages < 1 {
		m.TotalPages = 1
	}
}

// Wrap returns a paginated Response. A nil slice becomes an empty slice so
// JSON serialisation always produces an array rather than null.
func Wrap[T any](data []T, m Meta) Response[T] {
	if data == nil {
		data = []T{}
	}
	return Response[T]{Data: data, Pagination: m}
}
