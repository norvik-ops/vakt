// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package logsafe

// SanitizeField strips ANSI escape codes and non-printable control characters
// from untrusted strings before they are written to structured logs or stored,
// preventing log-injection / terminal-escape attacks. It also truncates to
// maxLen. Moved out of cmd/api/main.go in S90-2 so it can be reused.
func SanitizeField(s string, maxLen int) string {
	if len(s) > maxLen {
		s = s[:maxLen]
	}
	out := make([]byte, 0, len(s))
	i := 0
	for i < len(s) {
		b := s[i]
		if b == 0x1b && i+1 < len(s) && s[i+1] == '[' {
			// Skip ANSI CSI sequence: ESC [ ... <final byte 0x40–0x7E>
			i += 2
			for i < len(s) && (s[i] < 0x40 || s[i] > 0x7e) {
				i++
			}
			i++ // consume final byte
			continue
		}
		if b >= 0x20 || b == '\n' || b == '\r' || b == '\t' {
			out = append(out, b)
		}
		i++
	}
	return string(out)
}
