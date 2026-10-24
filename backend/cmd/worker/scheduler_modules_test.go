package main

import (
	"testing"

	"github.com/hibiken/asynq"

	"github.com/matharnica/vakt/internal/config"
	"github.com/matharnica/vakt/internal/modules/vaktcomply"
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

// TestRegisterCrons_NoArchivedModuleTask ties the scheduler to the archived-task
// guard: no cron may schedule a task the worker would drop (isArchivedTask). A
// cron left over from an archived module fails here instead of being dropped
// silently every day at runtime.
func TestRegisterCrons_NoArchivedModuleTask(t *testing.T) {
	got := scheduledTypes(&config.Config{})
	for task := range got {
		if isArchivedTask(task) {
			t.Errorf("cron %q belongs to an archived module", task)
		}
	}
	// Gegenprobe: Kern-Jobs laufen weiter, sonst wäre der Test vakuos grün.
	if !got[vaktcomply.TaskEvidenceStalenessCheck] {
		t.Fatalf("core cron %q missing — registerCrons scheduled %d tasks", vaktcomply.TaskEvidenceStalenessCheck, len(got))
	}
}
