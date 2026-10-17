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
}

func TestRegisterCrons_SkipsDisabledModules(t *testing.T) {
	got := scheduledTypes(&config.Config{ModulesEnabled: "vaktcomply"})
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
	got := scheduledTypes(&config.Config{ModulesEnabled: "vaktcomply,vaktscan,vaktvault,vaktaware,vaktprivacy,vakthr"})
	for _, task := range optionalModuleTasks {
		if !got[task] {
			t.Errorf("cron %q is missing although its module is enabled", task)
		}
	}
}
