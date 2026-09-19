// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/matharnica/vakt/internal/modules/vaktscan"
)

func TestComputeNextScanRun(t *testing.T) {
	base := time.Date(2026, 1, 1, 12, 2, 0, 0, time.UTC)

	next, err := computeNextScanRun("*/5 * * * *", base)
	if err != nil {
		t.Fatalf("valid cron returned error: %v", err)
	}
	// Next 5-minute boundary strictly after 12:02 is 12:05.
	want := time.Date(2026, 1, 1, 12, 5, 0, 0, time.UTC)
	if !next.Equal(want) {
		t.Errorf("next run = %s, want %s", next, want)
	}
	if !next.After(base) {
		t.Errorf("next run %s not strictly after base %s", next, base)
	}

	if _, err := computeNextScanRun("not-a-cron", base); err == nil {
		t.Error("malformed cron expression did not return an error")
	}
}

// --- fakes ---------------------------------------------------------------------

type fakeScanStore struct {
	due     []dueScanSchedule
	created []string // scanner names createScanRow was called with
	nextID  int
	marked  map[string]time.Time
}

func (f *fakeScanStore) listDueScanSchedules(_ context.Context) ([]dueScanSchedule, error) {
	return f.due, nil
}

func (f *fakeScanStore) createScanRow(_ context.Context, _, _, scanner string) (string, error) {
	f.nextID++
	id := fmt.Sprintf("scan-%d", f.nextID)
	f.created = append(f.created, scanner)
	return id, nil
}

func (f *fakeScanStore) markScanScheduleRun(_ context.Context, id string, nextRun time.Time) error {
	if f.marked == nil {
		f.marked = map[string]time.Time{}
	}
	f.marked[id] = nextRun
	return nil
}

type enqCall struct {
	taskType string
	payload  vaktscan.ScanPayload
}

type fakeEnqueuer struct {
	calls   []enqCall
	failFor map[string]bool // payload.ScanID -> return error
}

func (f *fakeEnqueuer) enqueueScan(taskType string, payload []byte) error {
	var p vaktscan.ScanPayload
	if err := json.Unmarshal(payload, &p); err != nil {
		return err
	}
	if f.failFor[p.ScanID] {
		return fmt.Errorf("enqueue failed for %s", p.ScanID)
	}
	f.calls = append(f.calls, enqCall{taskType: taskType, payload: p})
	return nil
}

// TestProcessDueScanSchedules verifies the three behaviours the executor must
// have and that a naive loop would get wrong:
//   - a due schedule is enqueued with the right task type + payload and its
//     next_run is advanced into the future;
//   - a schedule whose enqueue fails is NOT marked, so it retries next tick,
//     and its failure does NOT abort the remaining schedules (error isolation);
//   - a schedule with an invalid cron is skipped entirely — no scan row, no
//     enqueue, no next_run write.
func TestProcessDueScanSchedules(t *testing.T) {
	sFail := dueScanSchedule{ID: "sched-fail", OrgID: "org-1", AssetID: "asset-1",
		Scanner: "trivy", CronExpr: "*/5 * * * *", AssetName: "web-1", TargetURL: "https://a.example"}
	sOK := dueScanSchedule{ID: "sched-ok", OrgID: "org-2", AssetID: "asset-2",
		Scanner: "nuclei", CronExpr: "*/10 * * * *", AssetName: "web-2", TargetURL: "https://b.example"}
	sBadCron := dueScanSchedule{ID: "sched-bad", OrgID: "org-3", AssetID: "asset-3",
		Scanner: "openvas", CronExpr: "totally invalid", AssetName: "web-3"}

	store := &fakeScanStore{due: []dueScanSchedule{sFail, sOK, sBadCron}}
	// sFail is created first → scan-1; sOK second → scan-2. Fail the first.
	enq := &fakeEnqueuer{failFor: map[string]bool{"scan-1": true}}

	if err := ProcessDueScanSchedules(context.Background(), store, enq); err != nil {
		t.Fatalf("ProcessDueScanSchedules returned error: %v", err)
	}

	// Invalid cron: skipped before any scan row is created → only two rows made.
	if len(store.created) != 2 {
		t.Fatalf("createScanRow called %d times, want 2 (bad-cron schedule must be skipped)", len(store.created))
	}

	// Exactly one enqueue succeeded (sOK); sFail errored, sBadCron skipped.
	if len(enq.calls) != 1 {
		t.Fatalf("enqueued %d scans, want 1", len(enq.calls))
	}
	got := enq.calls[0]
	if got.taskType != vaktscan.TaskScanNuclei {
		t.Errorf("task type = %q, want %q", got.taskType, vaktscan.TaskScanNuclei)
	}
	if got.payload.ScanID != "scan-2" || got.payload.OrgID != "org-2" ||
		got.payload.AssetName != "web-2" || got.payload.TargetURL != "https://b.example" {
		t.Errorf("payload mismatch: %+v", got.payload)
	}

	// sOK advanced; sFail and sBadCron not marked.
	if _, ok := store.marked["sched-fail"]; ok {
		t.Error("sched-fail was marked despite enqueue failure — it must retry next tick")
	}
	if _, ok := store.marked["sched-bad"]; ok {
		t.Error("sched-bad (invalid cron) was marked — it must be skipped")
	}
	nr, ok := store.marked["sched-ok"]
	if !ok {
		t.Fatal("sched-ok was not marked with a new next_run")
	}
	if !nr.After(time.Now()) {
		t.Errorf("sched-ok next_run %s is not in the future", nr)
	}
}
