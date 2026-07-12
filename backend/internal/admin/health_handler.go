// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package admin

import (
	"net/http"
	"runtime"
	"time"

	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"

	"github.com/matharnica/vakt/internal/config"
)

// healthStartTime is set once at package init and used to compute uptime.
var healthStartTime = time.Now()

// HealthHandler handles GET /api/v1/admin/health.
type HealthHandler struct {
	db            *pgxpool.Pool
	rdb           *redis.Client
	redisAddr     string
	redisPassword string
	version       string
}

// NewHealthHandler constructs a HealthHandler.
func NewHealthHandler(db *pgxpool.Pool, rdb *redis.Client, cfg *config.Config) *HealthHandler {
	// Extract the plain host:port from the full Redis URL so that
	// asynq.NewInspector can be constructed independently of go-redis.
	// S122-B2 (NOAUTH): carry the password too — without it the inspector
	// cannot authenticate against a --requirepass Redis and queue stats fall
	// back to zeros silently (the same NOAUTH class as the enqueue clients).
	return &HealthHandler{
		db:            db,
		rdb:           rdb,
		redisAddr:     rdb.Options().Addr,
		redisPassword: rdb.Options().Password,
		version:       cfg.Version,
	}
}

// dbCheck pings the database with a SELECT 1 and returns latency in ms.
func (h *HealthHandler) dbCheck(c echo.Context) (ok bool, latencyMs int64) {
	start := time.Now()
	_, err := h.db.Exec(c.Request().Context(), "SELECT 1")
	latencyMs = time.Since(start).Milliseconds()
	if err != nil {
		log.Warn().Err(err).Msg("admin health: db check failed")
		return false, latencyMs
	}
	return true, latencyMs
}

// redisCheck pings Redis and returns latency in ms.
func (h *HealthHandler) redisCheck(c echo.Context) (ok bool, latencyMs int64) {
	start := time.Now()
	err := h.rdb.Ping(c.Request().Context()).Err()
	latencyMs = time.Since(start).Milliseconds()
	if err != nil {
		log.Warn().Err(err).Msg("admin health: redis check failed")
		return false, latencyMs
	}
	return true, latencyMs
}

// QueueStats holds the queue metrics returned in the health response.
type QueueStats struct {
	Pending int `json:"pending"`
	Active  int `json:"active"`
	Failed  int `json:"failed"`
}

// queueStats returns queue depth for the "default" asynq queue.
// Returns zero values and logs a warning when the inspector call fails.
func (h *HealthHandler) queueStats() QueueStats {
	inspector := asynq.NewInspector(asynq.RedisClientOpt{Addr: h.redisAddr, Password: h.redisPassword})
	defer func() { _ = inspector.Close() }()

	info, err := inspector.GetQueueInfo("default")
	if err != nil {
		log.Warn().Err(err).Msg("admin health: queue stats failed")
		return QueueStats{}
	}
	return QueueStats{
		Pending: info.Pending,
		Active:  info.Active,
		Failed:  info.Archived,
	}
}

// HandleHealth handles GET /api/v1/admin/health.
func (h *HealthHandler) HandleHealth(c echo.Context) error {
	dbOK, dbLatency := h.dbCheck(c)
	redisOK, redisLatency := h.redisCheck(c)
	queue := h.queueStats()

	return c.JSON(http.StatusOK, map[string]any{
		"db": map[string]any{
			"ok":         dbOK,
			"latency_ms": dbLatency,
		},
		"redis": map[string]any{
			"ok":         redisOK,
			"latency_ms": redisLatency,
		},
		"queue":          queue,
		"version":        h.version,
		"uptime_seconds": int64(time.Since(healthStartTime).Seconds()),
		"goroutines":     runtime.NumGoroutine(),
	})
}
