package auth

import (
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog/log"
)

const traceIDKey = "trace_id"

// TraceMiddleware generates a unique trace ID per request, attaches it to
// the context logger, and adds it to the response as X-Trace-ID.
func TraceMiddleware() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			traceID := uuid.New().String()
			c.Set(traceIDKey, traceID)
			c.Response().Header().Set("X-Trace-ID", traceID)

			// Enrich zerolog context for all subsequent log calls in this request
			logger := log.With().
				Str("trace_id", traceID).
				Str("method", c.Request().Method).
				Str("path", c.Request().URL.Path).
				Logger()
			c.Set("logger", logger)

			return next(c)
		}
	}
}

// TraceID returns the trace ID from the echo context (empty string if not set).
func TraceID(c echo.Context) string {
	if v, ok := c.Get(traceIDKey).(string); ok {
		return v
	}
	return ""
}
