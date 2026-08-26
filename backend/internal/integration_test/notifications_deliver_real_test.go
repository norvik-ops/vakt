//go:build integration

// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package integration_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/matharnica/vakt/internal/shared/notify"
)

// fakeSender is a hand-written notify.Sender: no socket, records the message it
// was handed, and returns a configurable error so both the delivered and the
// failed path can be driven against a real database.
type fakeSender struct {
	err error
	got notify.Message
}

func (f *fakeSender) Send(_ context.Context, m notify.Message) error {
	f.got = m
	return f.err
}

// insertPendingNotification mimics notify.Service.persist: it inserts a pending
// row and returns the id, exactly as the enqueued delivery task references it.
func insertPendingNotification(t *testing.T, pool *pgxpool.Pool, orgID string, ch notify.Channel) string {
	t.Helper()
	var id string
	require.NoError(t, pool.QueryRow(context.Background(), `
		INSERT INTO notifications (org_id, type, channel, payload, status)
		VALUES ($1::uuid, $2, $3, '{}'::jsonb, 'pending')
		RETURNING id::text`, orgID, notify.NotificationJobType, string(ch)).Scan(&id))
	return id
}

// deliverEnvelope builds the exact JSON the worker handler consumes. The envelope
// field names mirror notify's internal deliveryEnvelope / Message json tags.
func deliverEnvelope(t *testing.T, id string, msg notify.Message) *asynq.Task {
	t.Helper()
	payload, err := json.Marshal(map[string]any{
		"notification_id": id,
		"message": map[string]any{
			"title":   msg.Title,
			"body":    msg.Body,
			"org_id":  msg.OrgID,
			"channel": string(msg.Channel),
			"target":  msg.Target,
		},
	})
	require.NoError(t, err)
	return asynq.NewTask(notify.NotificationJobType, payload)
}

// TestNotificationsDeliver_DBEffect is the DB-effect regression for R1-19-W01.
// Before the fix, "notifications:deliver" had no worker handler at all, so a
// notifications row inserted by notify.Service.Notify stayed 'pending' forever
// (grep "UPDATE notifications" was empty in the whole repo). This drives the
// registered handler against a real Postgres and asserts the row advances.
func TestNotificationsDeliver_DBEffect(t *testing.T) {
	pool, orgID, cleanup := bootPostgresWithOrg(t)
	defer cleanup()
	ctx := context.Background()

	t.Run("success flips pending to sent", func(t *testing.T) {
		id := insertPendingNotification(t, pool, orgID, notify.ChannelEmail)
		fake := &fakeSender{}
		h := notify.DeliverHandler(pool, map[notify.Channel]notify.Sender{
			notify.ChannelEmail: fake,
		})

		msg := notify.Message{
			Title: "Deadline in 3 days", Body: "Control ABC is due", OrgID: orgID,
			Channel: notify.ChannelEmail, Target: "admin@example.org",
		}
		require.NoError(t, h(ctx, deliverEnvelope(t, id, msg)),
			"handler must accept the task and deliver without error")

		assert.Equal(t, "admin@example.org", fake.got.Target, "sender must receive the message")

		var status string
		var sentAt *string
		require.NoError(t, pool.QueryRow(ctx,
			`SELECT status, sent_at::text FROM notifications WHERE id = $1::uuid AND org_id = $2::uuid`, id, orgID).
			Scan(&status, &sentAt))
		assert.Equal(t, "sent", status, "delivered notification must advance to 'sent', not stay 'pending'")
		assert.NotNil(t, sentAt, "sent_at must be stamped")
	})

	t.Run("delivery error fails loudly and marks failed", func(t *testing.T) {
		id := insertPendingNotification(t, pool, orgID, notify.ChannelEmail)
		fake := &fakeSender{err: errors.New("smtp connection refused")}
		h := notify.DeliverHandler(pool, map[notify.Channel]notify.Sender{
			notify.ChannelEmail: fake,
		})

		msg := notify.Message{Title: "x", OrgID: orgID, Channel: notify.ChannelEmail, Target: "a@b.c"}
		err := h(ctx, deliverEnvelope(t, id, msg))
		require.Error(t, err, "a delivery failure must return an error so Asynq retries — never a silent nil")
		assert.Contains(t, err.Error(), "smtp connection refused")

		var status string
		var failedAt *string
		var retry int
		require.NoError(t, pool.QueryRow(ctx,
			`SELECT status, failed_at::text, retry_count FROM notifications WHERE id = $1::uuid AND org_id = $2::uuid`, id, orgID).
			Scan(&status, &failedAt, &retry))
		assert.Equal(t, "failed", status)
		assert.NotNil(t, failedAt, "failed_at must be stamped")
		assert.Equal(t, 1, retry, "retry_count must be incremented")
	})

	t.Run("unknown channel fails loudly", func(t *testing.T) {
		id := insertPendingNotification(t, pool, orgID, notify.ChannelSlack)
		// Empty registry: no sender for any channel.
		h := notify.DeliverHandler(pool, map[notify.Channel]notify.Sender{})

		msg := notify.Message{Title: "x", OrgID: orgID, Channel: notify.ChannelSlack, Target: "#ops"}
		err := h(ctx, deliverEnvelope(t, id, msg))
		require.Error(t, err, "a channel with no sender must fail the task, not swallow it")

		var status string
		require.NoError(t, pool.QueryRow(ctx,
			`SELECT status FROM notifications WHERE id = $1::uuid AND org_id = $2::uuid`, id, orgID).Scan(&status))
		assert.Equal(t, "failed", status)
	})
}
