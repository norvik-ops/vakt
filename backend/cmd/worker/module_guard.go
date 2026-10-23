// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package main

import (
	"context"
	"strings"

	"github.com/hibiken/asynq"
	"github.com/rs/zerolog/log"
)

// archivedTaskPrefixes are the task type prefixes of the modules archived with
// docs/stories/module-archivieren.md (Scan, Vault, Aware, Privacy, HR; tag
// archiv/module-vollstaendig). Task names grew with both the package name
// ("vaktscan:") and a short form ("privacy:"), so both spellings are listed.
var archivedTaskPrefixes = []string{
	"vaktscan:", "bsi:", // the CERT-Bund feed wrote vb_findings
	"vaktvault:", "vault:",
	"vaktaware:", "aware:",
	"vaktprivacy:", "privacy:",
	"hr:",
}

// archivedTaskTypes are platform- or ISMS-named task types whose only producers
// were archived modules: the alerting checks read vb_findings / po_dsr, cross-
// module evidence and the per-finding AI suggestion were enqueued by the modules.
var archivedTaskTypes = map[string]bool{
	"alerting:sla_overdue_check":        true,
	"alerting:dsr_overdue_check":        true,
	"vaktcomply:record_evidence":        true,
	"vaktcomply:ai_evidence_suggestion": true,
}

// isArchivedTask reports whether a task type belongs to an archived module.
func isArchivedTask(taskType string) bool {
	if archivedTaskTypes[taskType] {
		return true
	}
	for _, p := range archivedTaskPrefixes {
		if strings.HasPrefix(taskType, p) {
			return true
		}
	}
	return false
}

// archivedTaskMiddleware drops tasks of archived modules. They can still sit in
// Redis after an update — queued, scheduled or waiting for a retry — and no
// handler exists for them any more; without this they would fail and retry up
// to 25 times. The task is acknowledged so it does not come back. main.go keeps
// serving the archived queues at the lowest weight so they drain.
func archivedTaskMiddleware(next asynq.Handler) asynq.Handler {
	return asynq.HandlerFunc(func(ctx context.Context, t *asynq.Task) error {
		if isArchivedTask(t.Type()) {
			log.Warn().Str("task", t.Type()).
				Msg("worker: Aufgabe eines archivierten Moduls verworfen")
			return nil
		}
		return next.ProcessTask(ctx, t)
	})
}
