// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hibiken/asynq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakePinger implements dbPinger with a configurable response.  It lets
// each test exercise the success/failure branch of the readiness probe
// without booting a real Postgres.
type fakePinger struct{ err error }

func (f fakePinger) Ping(context.Context) error { return f.err }

// fakeInspector implements queueInspector with canned data.  Returning
// (nil, err) from Queues simulates a Redis outage; the canned QueueInfo
// values make /health/queue exercise the JSON encoding path.
type fakeInspector struct {
	queues    []string
	queuesErr error
	infos     map[string]*asynq.QueueInfo
	infoErr   error
}

func (f *fakeInspector) Queues() ([]string, error) { return f.queues, f.queuesErr }
func (f *fakeInspector) GetQueueInfo(q string) (*asynq.QueueInfo, error) {
	if f.infoErr != nil {
		return nil, f.infoErr
	}
	return f.infos[q], nil
}

// TestLiveness_AlwaysOK pins the liveness contract: while the process is
// running, /health returns 200 regardless of dependent-system state. This
// is intentional — Docker should not kill a worker just because Redis
// hiccupped; the orchestrator uses /health/ready for that.
func TestLiveness_AlwaysOK(t *testing.T) {
	mux := buildHealthHandlers(fakePinger{err: errors.New("db is angry")}, &fakeInspector{queuesErr: errors.New("redis down")})
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)
}

// TestReadiness_OKWhenAllReachable covers the green path.
func TestReadiness_OKWhenAllReachable(t *testing.T) {
	mux := buildHealthHandlers(fakePinger{}, &fakeInspector{queues: []string{"default"}})
	req := httptest.NewRequest(http.MethodGet, "/health/ready", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)
}

// TestReadiness_FailsOnDB ensures DB unreachability is the FIRST signal
// surfaced — "database" component, 503.
func TestReadiness_FailsOnDB(t *testing.T) {
	mux := buildHealthHandlers(fakePinger{err: errors.New("db unreachable")}, &fakeInspector{})
	req := httptest.NewRequest(http.MethodGet, "/health/ready", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
	assert.Contains(t, rec.Body.String(), "database")
}

// TestReadiness_FailsOnQueue is the core regression for audit P1-5: when
// Asynq cannot talk to Redis the readiness probe must fail. The pre-fix
// /health was blind to this.
func TestReadiness_FailsOnQueue(t *testing.T) {
	mux := buildHealthHandlers(fakePinger{}, &fakeInspector{queuesErr: errors.New("dial tcp: connection refused")})
	req := httptest.NewRequest(http.MethodGet, "/health/ready", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
	assert.Contains(t, rec.Body.String(), "queue")
}

// TestQueueStats_RendersCounts feeds two queues with realistic counts and
// asserts the JSON shape used by dashboards.
func TestQueueStats_RendersCounts(t *testing.T) {
	mux := buildHealthHandlers(fakePinger{}, &fakeInspector{
		queues: []string{"default", "vaktscan:scans"},
		infos: map[string]*asynq.QueueInfo{
			"default":        {Size: 5, Pending: 3, Active: 2, Retry: 0, Archived: 0, Scheduled: 0},
			"vaktscan:scans": {Size: 12, Pending: 10, Active: 2, Retry: 1, Archived: 4, Scheduled: 0},
		},
	})
	req := httptest.NewRequest(http.MethodGet, "/health/queue", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	var got map[string]map[string]int
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	assert.Equal(t, 3, got["default"]["pending"])
	assert.Equal(t, 10, got["vaktscan:scans"]["pending"])
	assert.Equal(t, 1, got["vaktscan:scans"]["retry"])
}

// TestQueueStats_FailsOnUnreachable: when the inspector cannot enumerate
// queues, /health/queue must surface that as 503 (dashboard alarms).
func TestQueueStats_FailsOnUnreachable(t *testing.T) {
	mux := buildHealthHandlers(fakePinger{}, &fakeInspector{queuesErr: errors.New("redis: timeout")})
	req := httptest.NewRequest(http.MethodGet, "/health/queue", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
}
