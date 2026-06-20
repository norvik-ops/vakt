// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package dashboard

import (
	"context"
	"testing"
)

// TestNotifyChannelKeyFormat pins the Redis Pub/Sub channel key so the
// publish side (PublishNotification) and the subscribe side (StreamNotifications)
// can never drift apart. (S98-5)
func TestNotifyChannelKeyFormat(t *testing.T) {
	cases := []struct {
		orgID string
		want  string
	}{
		{"a3f2b1c9", "notify:a3f2b1c9"},
		{"00000000-0000-0000-0000-000000000001", "notify:00000000-0000-0000-0000-000000000001"},
		{"", "notify:"},
	}
	for _, tc := range cases {
		if got := notifyChannel(tc.orgID); got != tc.want {
			t.Errorf("notifyChannel(%q) = %q, want %q", tc.orgID, got, tc.want)
		}
	}
}

// TestPublishNotificationNilRedisIsNoOp verifies the fallback path: when Redis
// is not configured, publishing must not panic and must silently no-op so the
// write path stays unaffected. (S98-5 — fail-open fallback)
func TestPublishNotificationNilRedisIsNoOp(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("PublishNotification panicked with nil redis: %v", r)
		}
	}()
	PublishNotification(context.Background(), nil, "org-1")
}
