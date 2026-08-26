// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package admin

import (
	"net/http"

	"github.com/hibiken/asynq"
	"github.com/labstack/echo/v4"
	"github.com/redis/go-redis/v9"

	"github.com/matharnica/vakt/internal/shared/redisopt"
)

// JobsHandler serves job queue statistics using the asynq Inspector.
type JobsHandler struct {
	inspector *asynq.Inspector
}

// NewJobsHandler creates a JobsHandler.
//
// S122-B2 (NOAUTH): the password must be threaded through — without it the
// Asynq inspector cannot authenticate against a --requirepass Redis and every
// queue-stats call errors, leaving the admin jobs panel blind.
//
// R1-14b-01: this takes the FULL parsed Redis options, not (addr, password)
// strings. Through two strings the database number cannot pass, so the
// inspector always reported on the queues in DB 0 while the worker ran in DB N.
// A panel that says "0 pending" about a queue it is not looking at is worse
// than no panel.
func NewJobsHandler(opts *redis.Options) *JobsHandler {
	return &JobsHandler{
		inspector: asynq.NewInspector(redisopt.Asynq(opts)),
	}
}

// GetQueueStats handles GET /admin/jobs — returns queue statistics.
func (h *JobsHandler) GetQueueStats(c echo.Context) error {
	queues, err := h.inspector.Queues()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to list job queues"})
	}

	type QueueStat struct {
		Queue   string `json:"queue"`
		Active  int    `json:"active"`
		Pending int    `json:"pending"`
		Retry   int    `json:"retry"`
		Failed  int    `json:"failed"`
		Size    int    `json:"size"`
	}

	var stats []QueueStat
	for _, q := range queues {
		info, err := h.inspector.GetQueueInfo(q)
		if err != nil {
			continue
		}
		stats = append(stats, QueueStat{
			Queue:   q,
			Active:  info.Active,
			Pending: info.Pending,
			Retry:   info.Retry,
			Failed:  info.Failed,
			Size:    info.Size,
		})
	}
	return c.JSON(http.StatusOK, stats)
}
