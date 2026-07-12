package queuemetrics

import "testing"

func TestRecordError_CountsPerQueue(t *testing.T) {
	// Note: package-level state; this test asserts deltas, not absolutes, so it
	// is robust to other tests in the same package.
	before := Snapshot()
	RecordError("vaktscan")
	RecordError("vaktscan")
	RecordError("vaktprivacy")
	RecordError("") // normalises to "default"

	after := Snapshot()
	if got := after["vaktscan"] - before["vaktscan"]; got != 2 {
		t.Errorf("vaktscan delta = %d, want 2", got)
	}
	if got := after["vaktprivacy"] - before["vaktprivacy"]; got != 1 {
		t.Errorf("vaktprivacy delta = %d, want 1", got)
	}
	if got := after["default"] - before["default"]; got != 1 {
		t.Errorf("default delta = %d, want 1", got)
	}
}

func TestSnapshot_IsCopy(t *testing.T) {
	RecordError("isolated")
	snap := Snapshot()
	snap["isolated"] = 9999 // mutating the copy must not affect internal state
	if Snapshot()["isolated"] == 9999 {
		t.Error("Snapshot returned a live reference, not a copy")
	}
}
