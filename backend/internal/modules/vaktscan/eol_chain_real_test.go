//go:build integration

// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package vaktscan

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/hibiken/asynq"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	tcwait "github.com/testcontainers/testcontainers-go/wait"
)

// S5/W1: the EOL check used to be a born-dead chain — a handler and an enqueuer
// with no producer, so endoflife.date was never consulted and no component was
// ever reported as out of support. S5 wired it to run after an SBOM scan.
//
// That wiring is the only functional change of the story, and nothing covered
// it: cmd/worker has no test naming handleSBOMGenerate or EnqueueEOLCheck, and
// worker_test.go only asserts `enqueueClient != nil` — a constructor assertion,
// not a behaviour one.
//
// This test drives the real enqueue against a real Redis and asserts the task
// actually lands, on the queue the worker consumes, with a payload the handler
// can read. A wrong queue name is the specific way this silently breaks: the
// task is accepted, never processed, and nothing errors.
func TestEnqueueEOLCheck_LandsOnConsumedQueue(t *testing.T) {
	if testing.Short() {
		t.Skip("integration: -short mode")
	}
	ctx := context.Background()

	ctr, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image:        "redis:7.4.10-alpine@sha256:e7723ff73d963f5cc6d9c4643ea3d989527a402a319239054e9472a7fb9219a2",
			ExposedPorts: []string{"6379/tcp"},
			WaitingFor:   tcwait.ForListeningPort("6379/tcp").WithStartupTimeout(60 * time.Second),
		},
		Started: true,
	})
	if err != nil {
		if strings.Contains(err.Error(), "permission denied") ||
			strings.Contains(err.Error(), "Cannot connect to the Docker daemon") {
			t.Skipf("integration: Docker unavailable (%v)", err)
		}
		t.Fatalf("redis container: %v", err)
	}
	defer func() { _ = ctr.Terminate(ctx) }()

	host, err := ctr.Host(ctx)
	require.NoError(t, err)
	port, err := ctr.MappedPort(ctx, "6379/tcp")
	require.NoError(t, err)
	redisOpt := asynq.RedisClientOpt{Addr: host + ":" + port.Port()}

	client := asynq.NewClient(redisOpt)
	defer func() { _ = client.Close() }()

	require.NoError(t, EnqueueEOLCheck(client, EOLCheckPayload{
		OrgID:  "11111111-1111-1111-1111-111111111111",
		SBOMID: "22222222-2222-2222-2222-222222222222",
	}))

	insp := asynq.NewInspector(redisOpt)
	defer func() { _ = insp.Close() }()

	tasks, err := insp.ListPendingTasks(QueueMaintenance)
	require.NoError(t, err, "queue %q must exist — the worker consumes it", QueueMaintenance)
	require.Len(t, tasks, 1,
		"the EOL check must land on %q; a different queue name means the task is "+
			"accepted, never processed, and nothing errors", QueueMaintenance)

	require.Equal(t, TaskEOLCheck, tasks[0].Type,
		"task type must match the one cmd/worker registers a handler for")

	var got EOLCheckPayload
	require.NoError(t, json.Unmarshal(tasks[0].Payload, &got),
		"the handler unmarshals this payload — a shape change here breaks it silently")
	require.Equal(t, "22222222-2222-2222-2222-222222222222", got.SBOMID)
	require.Equal(t, "11111111-1111-1111-1111-111111111111", got.OrgID)
}
