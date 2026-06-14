// Package logging configures the global zerolog level from the environment.
package logging

import (
	"os"
	"strings"

	"github.com/rs/zerolog"
)

// ApplyLevelFromEnv sets the global zerolog level from VAKT_LOG_LEVEL
// (trace|debug|info|warn|error|fatal|panic). Unset or invalid → InfoLevel.
// Call this early in main() so every logger honours it.
func ApplyLevelFromEnv() {
	zerolog.SetGlobalLevel(ParseLevelOr(os.Getenv("VAKT_LOG_LEVEL"), zerolog.InfoLevel))
}

// ParseLevelOr parses a zerolog level string, falling back to def when the
// string is empty or unparseable.
func ParseLevelOr(s string, def zerolog.Level) zerolog.Level {
	s = strings.ToLower(strings.TrimSpace(s))
	if s == "" {
		return def
	}
	if lvl, err := zerolog.ParseLevel(s); err == nil {
		return lvl
	}
	return def
}
