package vaktaware

import (
	"crypto/sha256"
	"encoding/hex"
	"html"
	"strings"
	"time"
)

// Recipient carries the data used to fill template placeholders.
type Recipient struct {
	FirstName   string
	LastName    string
	Email       string
	CompanyName string
	SenderName  string
}

// RenderTemplate replaces all known placeholders in subject, body, and landing
// with recipient-specific values. Unknown placeholders are left unchanged.
// HTML content in recipient fields is escaped to prevent injection.
func RenderTemplate(tmpl Template, r Recipient, trackingURL string) (subject, body, landing string) {
	replacer := strings.NewReplacer(
		"{{first_name}}", html.EscapeString(r.FirstName),
		"{{last_name}}", html.EscapeString(r.LastName),
		"{{target_first_name}}", html.EscapeString(r.FirstName),
		"{{target_email}}", html.EscapeString(r.Email),
		"{{company}}", html.EscapeString(r.CompanyName),
		"{{company_name}}", html.EscapeString(r.CompanyName),
		"{{sender_name}}", html.EscapeString(r.SenderName),
		"{{current_date}}", time.Now().Format("02.01.2006"),
		"{{tracking_url}}", trackingURL,
	)
	subject = replacer.Replace(tmpl.Subject)
	body = replacer.Replace(tmpl.HTMLBody)
	return subject, body, landing
}

// anonymizeEmail returns a non-reversible 16-char hex digest of the lowercase email.
// Used for Betriebsrat-compliant training reports.
func anonymizeEmail(email string) string {
	h := sha256.Sum256([]byte(strings.ToLower(email)))
	return hex.EncodeToString(h[:8])
}

// FilterPresetTemplates returns only those preset templates matching the given
// category, difficulty, and language filters. Empty string means "no filter".
func FilterPresetTemplates(all []Template, category, difficulty, language string) []Template {
	out := make([]Template, 0, len(all))
	for _, t := range all {
		if category != "" && t.Category != category {
			continue
		}
		if difficulty != "" && t.Difficulty != difficulty {
			continue
		}
		if language != "" && t.Language != language {
			continue
		}
		out = append(out, t)
	}
	return out
}
