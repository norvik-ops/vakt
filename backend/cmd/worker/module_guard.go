// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package main

import (
	"context"
	"strings"

	"github.com/hibiken/asynq"
	"github.com/matharnica/vakt/internal/config"
	"github.com/matharnica/vakt/internal/services/alerting"
	"github.com/rs/zerolog/log"
)

// taskModulePrefixes maps task type prefixes to the module that owns them. Task
// names grew over time with both the package name ("vaktscan:") and a short form
// ("privacy:"), so both spellings are listed.
var taskModulePrefixes = []struct {
	prefix string
	module string
}{
	{"vaktscan:", "vaktscan"},
	{"bsi:", "vaktscan"}, // the CERT-Bund feed writes vb_findings
	{"vaktvault:", "vaktvault"},
	{"vault:", "vaktvault"},
	{"vaktaware:", "vaktaware"},
	{"aware:", "vaktaware"},
	{"vaktprivacy:", "vaktprivacy"},
	{"privacy:", "vaktprivacy"},
	{"hr:", "vakthr"},
}

// taskModuleExact maps platform-named task types that read a module's data.
// The alerting checks run under "alerting:" but read vb_findings / po_dsr.
var taskModuleExact = map[string]string{
	alerting.TaskSLAOverdueCheck: "vaktscan",
	alerting.TaskDSROverdueCheck: "vaktprivacy",
}

// taskModule returns the module owning a task type, or "" for platform and ISMS
// tasks, which always run.
func taskModule(taskType string) string {
	if m, ok := taskModuleExact[taskType]; ok {
		return m
	}
	for _, p := range taskModulePrefixes {
		if strings.HasPrefix(taskType, p.prefix) {
			return p.module
		}
	}
	return ""
}

// moduleGuardMiddleware drops tasks of modules switched off via
// VAKT_MODULES_ENABLED. Without it a task enqueued before the switch — a phishing
// campaign, a scan, a Git scan — would still run after the module is gone. The
// task is acknowledged (not retried) so it does not come back.
func moduleGuardMiddleware(cfg *config.Config) asynq.MiddlewareFunc {
	return func(next asynq.Handler) asynq.Handler {
		return asynq.HandlerFunc(func(ctx context.Context, t *asynq.Task) error {
			if m := taskModule(t.Type()); m != "" && cfg != nil && !cfg.IsModuleEnabled(m) {
				log.Warn().Str("task", t.Type()).Str("module", m).
					Msg("worker: Aufgabe verworfen — Modul ist per VAKT_MODULES_ENABLED abgeschaltet")
				return nil
			}
			return next.ProcessTask(ctx, t)
		})
	}
}
