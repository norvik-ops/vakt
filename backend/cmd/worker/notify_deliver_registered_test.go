// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package main

import (
	"testing"

	"github.com/hibiken/asynq"
	"github.com/stretchr/testify/require"

	"github.com/matharnica/vakt/internal/shared/notify"
)

// TestNotificationsDeliverHandlerRegistered guards R1-19-W01: notify.Service.Notify
// enqueues a "notifications:deliver" task, but for years cmd/worker registered NO
// handler for it, so every external notification (Slack/Teams/email/webhook) hit
// asynq's "handler not found", retried 25×, was archived, and its notifications
// row stayed 'pending' forever.
//
// asynq.ServeMux.Handler returns an empty pattern when no handler matches a task
// type (servemux.go: NotFoundHandler + pattern ""). This test asserts the mux the
// worker actually builds routes notifications:deliver to a real handler.
//
// Red-on-revert: delete the mux.HandleFunc(notify.NotificationJobType, ...) line
// in main.go and this test fails with an empty pattern.
func TestNotificationsDeliverHandlerRegistered(t *testing.T) {
	_, mux, enqueueClient := buildServer(nil)
	t.Cleanup(func() { _ = enqueueClient.Close() })

	_, pattern := mux.Handler(asynq.NewTask(notify.NotificationJobType, nil))
	require.Equal(t, notify.NotificationJobType, pattern,
		"notifications:deliver must resolve to a registered handler — the enqueue side (notify.Service.Notify) has no consumer otherwise")
}
