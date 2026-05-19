// Package dashboard aggregates cross-module metrics into a single security
// score and manages in-app notifications stored in the user_notifications
// table.
package dashboard

import "time"

// UserNotification is the JSON shape returned by the notifications endpoints.
// The Module field identifies which SecHealth module originated the event so
// the frontend can route the user to the right detail view.
type UserNotification struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	Body      string    `json:"body"`
	Type      string    `json:"type"`
	Module    string    `json:"module"`
	Read      bool      `json:"read"`
	CreatedAt time.Time `json:"created_at"`
}
