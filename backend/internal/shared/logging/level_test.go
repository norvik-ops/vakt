package logging

import (
	"testing"

	"github.com/rs/zerolog"
)

func TestParseLevelOr(t *testing.T) {
	cases := []struct {
		in   string
		want zerolog.Level
	}{
		{"", zerolog.InfoLevel}, // unset → default
		{"info", zerolog.InfoLevel},
		{"DEBUG", zerolog.DebugLevel},  // case-insensitive
		{"  warn ", zerolog.WarnLevel}, // trimmed
		{"error", zerolog.ErrorLevel},
		{"trace", zerolog.TraceLevel},
		{"bogus", zerolog.InfoLevel}, // unparseable → default
	}
	for _, c := range cases {
		if got := ParseLevelOr(c.in, zerolog.InfoLevel); got != c.want {
			t.Errorf("ParseLevelOr(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}
