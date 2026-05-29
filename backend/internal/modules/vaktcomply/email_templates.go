package vaktcomply

// Email template constants for supplier portal assessments (Story 29.3).
const (
	EmailSupplierInviteSubjectDE = "Sicherheitsfragebogen: Bitte um Ihre Mitwirkung"
	EmailSupplierInviteBodyDE    = "Sehr geehrte Damen und Herren,\n\nbitte füllen Sie den beigefügten Sicherheitsfragebogen unter folgendem Link aus:\n{{.ShareURL}}\n\nDer Link ist gültig bis: {{.ExpiresAt}}\n\nVielen Dank für Ihre Mitwirkung."

	EmailSupplierConfirmSubjectDE = "Fragebogen eingereicht – Danke"
	EmailSupplierConfirmBodyDE    = "Sehr geehrte Damen und Herren,\n\nIhr Fragebogen wurde erfolgreich eingereicht. Wir werden ihn prüfen und uns bei Bedarf melden."

	EmailComplianceNotifySubjectDE = "Lieferanten-Fragebogen eingereicht"
	EmailComplianceNotifyBodyDE    = "Ein Lieferant hat den Fragebogen eingereicht.\n\nAssessment-ID: {{.AssessmentID}}\nLieferant-ID: {{.SupplierID}}"
)
