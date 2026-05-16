package secpulse

import "time"

// scanUpdateOptions holds optional fields for UpdateScanStatus.
type scanUpdateOptions struct {
	errorMessage *string
	findingCount *int
	durationMs   *int64
	startedAt    *time.Time
	completedAt  *time.Time
}

// ScanUpdateOpt is a functional option for UpdateScanStatus.
type ScanUpdateOpt func(*scanUpdateOptions)

// WithErrorMessage sets the error_message field.
func WithErrorMessage(msg string) ScanUpdateOpt {
	return func(o *scanUpdateOptions) {
		o.errorMessage = &msg
	}
}

// WithFindingCount sets the finding_count field.
func WithFindingCount(n int) ScanUpdateOpt {
	return func(o *scanUpdateOptions) {
		o.findingCount = &n
	}
}

// WithDurationMs sets the duration_ms field.
func WithDurationMs(ms int64) ScanUpdateOpt {
	return func(o *scanUpdateOptions) {
		o.durationMs = &ms
	}
}

// WithStartedAt sets the started_at field.
func WithStartedAt(t time.Time) ScanUpdateOpt {
	return func(o *scanUpdateOptions) {
		o.startedAt = &t
	}
}

// WithCompletedAt sets the completed_at field.
func WithCompletedAt(t time.Time) ScanUpdateOpt {
	return func(o *scanUpdateOptions) {
		o.completedAt = &t
	}
}
