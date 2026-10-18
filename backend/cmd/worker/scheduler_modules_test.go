package main

import (
	"testing"

	"github.com/hibiken/asynq"

	"github.com/matharnica/vakt/internal/config"
	"github.com/matharnica/vakt/internal/modules/vaktcomply"
	"github.com/matharnica/vakt/internal/modules/vakthr"
	"github.com/matharnica/vakt/internal/modules/vaktprivacy"
	"github.com/matharnica/vakt/internal/modules/vaktscan"
	"github.com/matharnica/vakt/internal/modules/vaktvault"
	"github.com/matharnica/vakt/internal/services/alerting"
	"github.com/matharnica/vakt/internal/shared/bsi"
)

type recordingRegistrar struct{ types map[string]bool }

func (r *recordingRegistrar) Register(_ string, task *asynq.Task, _ ...asynq.Option) (string, error) {
	r.types[task.Type()] = true
	return task.Type(), nil
}

func scheduledTypes(cfg *config.Config) map[string]bool {
	r := &recordingRegistrar{types: map[string]bool{}}
	registerCrons(r, cfg)
	return r.types
}

// optionalModuleTasks are the crons that belong to a module other than the
// vaktcomply core. With only vaktcomply enabled none of them may be scheduled.
var optionalModuleTasks = []string{
	vaktscan.TaskEPSSEnrich, vaktscan.TaskScanScheduleDue, vaktscan.TaskRiskTrendSnapshot,
	vaktscan.TaskCertScan, vaktscan.TaskSLACheck, alerting.TaskSLAOverdueCheck,
	alerting.TaskDSROverdueCheck, vaktprivacy.TaskAVVExpiryCheck, vaktprivacy.TaskDSRDeadlineCheck,
	vaktprivacy.TaskDeletionReminderCheck, vakthr.TaskContractorExpiryCheck, vaktvault.TaskQuarterlyAccessReview,
	bsi.TaskBSIFeedSync,
}

func TestRegisterCrons_SkipsDisabledModules(t *testing.T) {
	got := scheduledTypes(&config.Config{ModulesEnabled: "vaktcomply", BSIFeedEnabled: true})
	for _, task := range optionalModuleTasks {
		if got[task] {
			t.Errorf("cron %q is scheduled although its module is disabled", task)
		}
	}
	// Gegenprobe: Kern-Jobs laufen weiter, sonst wäre der Test vakuos grün.
	if !got[vaktcomply.TaskEvidenceStalenessCheck] {
		t.Fatalf("core cron %q missing — registerCrons scheduled %d tasks", vaktcomply.TaskEvidenceStalenessCheck, len(got))
	}
}

func TestRegisterCrons_SchedulesEnabledModules(t *testing.T) {
	got := scheduledTypes(&config.Config{ModulesEnabled: "vaktcomply,vaktscan,vaktvault,vaktaware,vaktprivacy,vakthr", BSIFeedEnabled: true})
	for _, task := range optionalModuleTasks {
		if !got[task] {
			t.Errorf("cron %q is missing although its module is enabled", task)
		}
	}
}

// TestRegisterCrons_NoModuleTaskWithOnlyCore ties the scheduler to the worker's
// module guard: with only the core enabled, no scheduled task may belong to an
// optional module (taskModule). A new cron with a module prefix that forgets its
// moduleOn check fails here instead of being dropped silently at runtime.
func TestRegisterCrons_NoModuleTaskWithOnlyCore(t *testing.T) {
	for task := range scheduledTypes(&config.Config{ModulesEnabled: "vaktcomply", BSIFeedEnabled: true}) {
		if m := taskModule(task); m != "" {
			t.Errorf("cron %q (module %s) is scheduled with only vaktcomply enabled", task, m)
		}
	}
}
