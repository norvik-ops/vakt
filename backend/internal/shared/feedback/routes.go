package feedback

import (
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"golang.org/x/time/rate"
)

func Register(api *echo.Group, db *pgxpool.Pool, authMiddleware echo.MiddlewareFunc) {
	h := NewHandler(db)

	feedbackLimiter := middleware.RateLimiter(middleware.NewRateLimiterMemoryStoreWithConfig(
		middleware.RateLimiterMemoryStoreConfig{Rate: rate.Limit(5.0 / 60.0), Burst: 10, ExpiresIn: 5 * time.Minute},
	))

	api.POST("/feedback", h.Submit, feedbackLimiter)
	api.GET("/feedback", h.List, authMiddleware)
}
