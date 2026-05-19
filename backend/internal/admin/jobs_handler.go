// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package admin

import (
	"net/http"

	"github.com/hibiken/asynq"
	"github.com/labstack/echo/v4"
)

// JobsHandler serves job queue statistics using the asynq Inspector.
type JobsHandler struct {
	inspector *asynq.Inspector
}

// NewJobsHandler creates a JobsHandler.
func NewJobsHandler(redisAddr string) *JobsHandler {
	return &JobsHandler{
		inspector: asynq.NewInspector(asynq.RedisClientOpt{Addr: redisAddr}),
	}
}

// GetQueueStats handles GET /admin/jobs — returns queue statistics.
func (h *JobsHandler) GetQueueStats(c echo.Context) error {
	queues, err := h.inspector.Queues()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
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
