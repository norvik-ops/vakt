package pagination

import (
	"encoding/base64"
	"encoding/json"
	"time"

	"github.com/labstack/echo/v4"
)

// CursorMeta is the pagination metadata returned by cursor-based endpoints.
// The deprecated offset-based fields are omitted; callers use next_cursor for iteration.
type CursorMeta struct {
	Limit      int    `json:"limit"`
	NextCursor string `json:"next_cursor,omitempty"` // empty when last page
	HasMore    bool   `json:"has_more"`
}

// CursorResponse wraps a typed slice with cursor pagination metadata.
type CursorResponse[T any] struct {
	Data       []T        `json:"data"`
	Pagination CursorMeta `json:"pagination"`
}

// cursorPayload is the internal structure encoded in the opaque cursor string.
type cursorPayload struct {
	ID string    `json:"i"`
	TS time.Time `json:"t"`
}

// EncodeCursor returns an opaque, URL-safe cursor string from an item's UUID and timestamp.
func EncodeCursor(id string, ts time.Time) string {
	b, _ := json.Marshal(cursorPayload{ID: id, TS: ts.UTC()})
	return base64.RawURLEncoding.EncodeToString(b)
}

// DecodeCursor parses an opaque cursor into its component parts.
// Returns empty id and zero time on invalid input (treated as "first page").
func DecodeCursor(encoded string) (id string, ts time.Time) {
	if encoded == "" {
		return "", time.Time{}
	}
	b, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil {
		return "", time.Time{}
	}
	var p cursorPayload
	if err := json.Unmarshal(b, &p); err != nil {
		return "", time.Time{}
	}
	return p.ID, p.TS
}

// CursorParams holds parsed cursor pagination parameters from an HTTP request.
type CursorParams struct {
	Cursor string
	Limit  int
}

// CursorFromRequest extracts cursor and limit from query params.
// Falls back to DefaultLimit and empty cursor (first page) when absent.
func CursorFromRequest(c echo.Context) CursorParams {
	lim, _ := parseInt(c.QueryParam("limit"), DefaultLimit)
	if lim < 1 || lim > MaxLimit {
		lim = DefaultLimit
	}
	return CursorParams{
		Cursor: c.QueryParam("cursor"),
		Limit:  lim,
	}
}

// WrapCursor returns a CursorResponse. rows must have len == limit+1 to detect
// HasMore; the extra row is stripped before returning.
func WrapCursor[T any](rows []T, params CursorParams, cursorOf func(T) string) CursorResponse[T] {
	hasMore := len(rows) > params.Limit
	if hasMore {
		rows = rows[:params.Limit]
	}
	var nextCursor string
	if hasMore && len(rows) > 0 {
		nextCursor = cursorOf(rows[len(rows)-1])
	}
	if rows == nil {
		rows = []T{}
	}
	return CursorResponse[T]{
		Data: rows,
		Pagination: CursorMeta{
			Limit:      params.Limit,
			NextCursor: nextCursor,
			HasMore:    hasMore,
		},
	}
}

// EncodeControlCursor returns an opaque cursor string for control list pagination.
// Controls are ordered by (control_id ASC, id ASC), so both are needed.
func EncodeControlCursor(controlID, id string) string {
	b, _ := json.Marshal(struct {
		CID string `json:"c"`
		ID  string `json:"i"`
	}{CID: controlID, ID: id})
	return base64.RawURLEncoding.EncodeToString(b)
}

// DecodeControlCursor parses a control cursor into its component parts.
func DecodeControlCursor(encoded string) (controlID, id string) {
	if encoded == "" {
		return "", ""
	}
	b, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil {
		return "", ""
	}
	var p struct {
		CID string `json:"c"`
		ID  string `json:"i"`
	}
	if err := json.Unmarshal(b, &p); err != nil {
		return "", ""
	}
	return p.CID, p.ID
}

func parseInt(s string, def int) (int, bool) {
	if s == "" {
		return def, false
	}
	n := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			return def, false
		}
		n = n*10 + int(c-'0')
	}
	return n, true
}
