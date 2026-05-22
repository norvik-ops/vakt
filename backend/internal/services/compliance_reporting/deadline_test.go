package compliance_reporting_test

import (
	"testing"
	"time"

	cr "github.com/matharnica/vakt/internal/services/compliance_reporting"
)

// reference point used throughout the tests
var epoch = time.Date(2026, 1, 15, 10, 0, 0, 0, time.UTC)

// ─── ComputeDeadlines ────────────────────────────────────────────────────────

func TestComputeDeadlines_DORA(t *testing.T) {
	windows := cr.ComputeDeadlines(epoch, cr.DORADeadlines)

	if len(windows) != 4 {
		t.Fatalf("expected 4 windows, got %d", len(windows))
	}

	cases := []struct {
		label    string
		wantAt   time.Time
	}{
		{"4h", epoch.Add(4 * time.Hour)},
		{"24h", epoch.Add(24 * time.Hour)},
		{"72h", epoch.Add(72 * time.Hour)},
		{"30d", epoch.Add(720 * time.Hour)},
	}
	for i, c := range cases {
		if windows[i].Label != c.label {
			t.Errorf("[%d] label: want %q, got %q", i, c.label, windows[i].Label)
		}
		if !windows[i].DeadlineAt.Equal(c.wantAt) {
			t.Errorf("[%d] DeadlineAt: want %v, got %v", i, c.wantAt, windows[i].DeadlineAt)
		}
		if windows[i].ReportedAt != nil {
			t.Errorf("[%d] ReportedAt should be nil on fresh windows", i)
		}
		if windows[i].WarnSent {
			t.Errorf("[%d] WarnSent should be false on fresh windows", i)
		}
	}
}

func TestComputeDeadlines_NIS2(t *testing.T) {
	windows := cr.ComputeDeadlines(epoch, cr.NIS2Deadlines)
	if len(windows) != 3 {
		t.Fatalf("expected 3 windows, got %d", len(windows))
	}
	if windows[0].Label != "24h" || !windows[0].DeadlineAt.Equal(epoch.Add(24*time.Hour)) {
		t.Errorf("NIS2 first window wrong: %+v", windows[0])
	}
}

func TestComputeDeadlines_DSGVO(t *testing.T) {
	windows := cr.ComputeDeadlines(epoch, cr.DSGVODeadlines)
	if len(windows) != 1 {
		t.Fatalf("expected 1 window, got %d", len(windows))
	}
	if windows[0].Label != "72h" || !windows[0].DeadlineAt.Equal(epoch.Add(72*time.Hour)) {
		t.Errorf("DSGVO window wrong: %+v", windows[0])
	}
}

func TestComputeDeadlines_EmptySet(t *testing.T) {
	windows := cr.ComputeDeadlines(epoch, cr.DeadlineSet{})
	if len(windows) != 0 {
		t.Errorf("expected empty slice, got %d elements", len(windows))
	}
}

func TestComputeDeadlines_PreservesOrder(t *testing.T) {
	custom := cr.DeadlineSet{
		{Label: "1h", Duration: time.Hour},
		{Label: "2h", Duration: 2 * time.Hour},
		{Label: "3h", Duration: 3 * time.Hour},
	}
	windows := cr.ComputeDeadlines(epoch, custom)
	for i, w := range windows {
		if w.Label != custom[i].Label {
			t.Errorf("order mismatch at index %d: want %q, got %q", i, custom[i].Label, w.Label)
		}
	}
}

// ─── AmpelStatus ─────────────────────────────────────────────────────────────

func TestAmpelStatus_AllReported_Green(t *testing.T) {
	reported := epoch
	windows := cr.ComputeDeadlines(epoch, cr.NIS2Deadlines)
	// Mark all as reported.
	for i := range windows {
		windows[i].ReportedAt = &reported
	}
	got := cr.AmpelStatus(windows, epoch.Add(48*time.Hour))
	if got != "green" {
		t.Errorf("all reported: want green, got %q", got)
	}
}

func TestAmpelStatus_NoDeadlines_Green(t *testing.T) {
	got := cr.AmpelStatus([]cr.DeadlineWindow{}, time.Now())
	if got != "green" {
		t.Errorf("empty windows: want green, got %q", got)
	}
}

func TestAmpelStatus_DeadlineFarFuture_Green(t *testing.T) {
	windows := cr.ComputeDeadlines(epoch, cr.NIS2Deadlines)
	// Evaluate 6 hours after start — the nearest (24h) deadline is 18 hours away,
	// which is more than the 12h warning threshold.
	now := epoch.Add(6 * time.Hour)
	got := cr.AmpelStatus(windows, now)
	if got != "green" {
		t.Errorf("18h until 24h deadline: want green, got %q", got)
	}
}

func TestAmpelStatus_NearestDeadlineWithin12h_Yellow(t *testing.T) {
	windows := cr.ComputeDeadlines(epoch, cr.NIS2Deadlines)
	// 23 hours after start — the 24h deadline is only 1 hour away.
	now := epoch.Add(23 * time.Hour)
	got := cr.AmpelStatus(windows, now)
	if got != "yellow" {
		t.Errorf("1h until 24h deadline: want yellow, got %q", got)
	}
}

func TestAmpelStatus_ExactlyAt12hBoundary_Yellow(t *testing.T) {
	windows := cr.ComputeDeadlines(epoch, cr.NIS2Deadlines)
	// Exactly 12 hours before the 24h deadline.
	now := epoch.Add(12 * time.Hour)
	got := cr.AmpelStatus(windows, now)
	if got != "yellow" {
		t.Errorf("exactly 12h left: want yellow, got %q", got)
	}
}

func TestAmpelStatus_OverdueDeadline_Red(t *testing.T) {
	windows := cr.ComputeDeadlines(epoch, cr.NIS2Deadlines)
	// 25 hours after start — the 24h deadline is 1 hour overdue.
	now := epoch.Add(25 * time.Hour)
	got := cr.AmpelStatus(windows, now)
	if got != "red" {
		t.Errorf("24h deadline overdue: want red, got %q", got)
	}
}

func TestAmpelStatus_FirstReportedSecondOverdue_Red(t *testing.T) {
	reported := epoch.Add(20 * time.Hour)
	windows := cr.ComputeDeadlines(epoch, cr.NIS2Deadlines)
	// Mark the 24h window as reported but leave 72h and 30d open.
	windows[0].ReportedAt = &reported
	// 80 hours in — 72h is 8h overdue.
	now := epoch.Add(80 * time.Hour)
	got := cr.AmpelStatus(windows, now)
	if got != "red" {
		t.Errorf("72h overdue with 24h done: want red, got %q", got)
	}
}

func TestAmpelStatus_DORA_AllWindowsReported_Green(t *testing.T) {
	reported := epoch.Add(2 * time.Hour)
	windows := cr.ComputeDeadlines(epoch, cr.DORADeadlines)
	for i := range windows {
		windows[i].ReportedAt = &reported
	}
	now := epoch.Add(700 * time.Hour)
	got := cr.AmpelStatus(windows, now)
	if got != "green" {
		t.Errorf("all DORA windows reported: want green, got %q", got)
	}
}

func TestAmpelStatus_DORA_4hApproaching_Yellow(t *testing.T) {
	windows := cr.ComputeDeadlines(epoch, cr.DORADeadlines)
	// 3.5 hours in — 4h deadline is only 30 minutes away.
	now := epoch.Add(3*time.Hour + 30*time.Minute)
	got := cr.AmpelStatus(windows, now)
	if got != "yellow" {
		t.Errorf("30 min until 4h deadline: want yellow, got %q", got)
	}
}

func TestAmpelStatus_DSGVO_72hOverdue_Red(t *testing.T) {
	windows := cr.ComputeDeadlines(epoch, cr.DSGVODeadlines)
	now := epoch.Add(73 * time.Hour)
	got := cr.AmpelStatus(windows, now)
	if got != "red" {
		t.Errorf("DSGVO 72h overdue: want red, got %q", got)
	}
}
