package secvault

// NotificationJobType is the Asynq task type for notification delivery.
// It mirrors notify.NotificationJobType; kept here so the secvault worker
// namespace can reference it without importing the shared notify package
// (module isolation rule).
const NotificationJobType = "notifications:deliver"
