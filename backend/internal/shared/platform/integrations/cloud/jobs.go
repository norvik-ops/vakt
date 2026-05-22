// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package cloud

import "github.com/hibiken/asynq"

// TaskCloudSync is the Asynq task type for the daily cloud evidence sync.
const TaskCloudSync = "cloud:sync_all"

// NewCloudSyncTask creates a new cloud sync task for Asynq scheduling.
func NewCloudSyncTask() *asynq.Task {
	return asynq.NewTask(TaskCloudSync, nil)
}
