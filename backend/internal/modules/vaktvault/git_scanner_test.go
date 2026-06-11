// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package vaktvault

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ── sanitizeGitURL ────────────────────────────────────────────────────────────

func TestSanitizeGitURL_RemovesCredentials(t *testing.T) {
	out := sanitizeGitURL("https://user:password@github.com/org/repo")
	assert.Equal(t, "https://<redacted>@github.com/org/repo", out)
	assert.NotContains(t, out, "password")
	assert.NotContains(t, out, "user:")
}

func TestSanitizeGitURL_NoCredentials_Unchanged(t *testing.T) {
	url := "https://github.com/org/repo"
	assert.Equal(t, url, sanitizeGitURL(url))
}

func TestSanitizeGitURL_TokenInURL_Redacted(t *testing.T) {
	out := sanitizeGitURL("https://oauth2:ghp_abc123@gitlab.com/org/repo")
	assert.NotContains(t, out, "ghp_abc123")
	assert.Contains(t, out, "<redacted>")
}

// ── validateBranch ────────────────────────────────────────────────────────────

func TestValidateBranch_Valid(t *testing.T) {
	for _, b := range []string{"main", "feature/my-branch", "release-1.0", "fix_typo", "v1.2.3"} {
		assert.NoError(t, validateBranch(b), "branch=%q", b)
	}
}

func TestValidateBranch_LeadingHyphen_Rejected(t *testing.T) {
	err := validateBranch("-dangerous")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "must not start with '-'")
}

func TestValidateBranch_CommandInjection_Rejected(t *testing.T) {
	// Semicolons, $, backticks, spaces — characters not in the allowlist
	for _, b := range []string{"main; rm -rf /", "branch$(whoami)", "branch`id`", "a b c"} {
		assert.Error(t, validateBranch(b), "branch=%q should be rejected", b)
	}
}

func TestValidateBranch_TooLong_Rejected(t *testing.T) {
	long := make([]byte, 256)
	for i := range long {
		long[i] = 'a'
	}
	assert.Error(t, validateBranch(string(long)))
}

// ── redactMatch ───────────────────────────────────────────────────────────────

func TestRedactMatch_ShortInput_Masked(t *testing.T) {
	assert.Equal(t, "****", redactMatch("short"))
	assert.Equal(t, "****", redactMatch("1234567"))
}

func TestRedactMatch_LongInput_ShowsFirstAndLast(t *testing.T) {
	out := redactMatch("AKIAIOSFODNN7EXAMPLE")
	assert.Less(t, len(out), len("AKIAIOSFODNN7EXAMPLE"))
	assert.Contains(t, out, "...")
	assert.True(t, len(out) >= 4 && out[:4] == "AKIA", "should preserve first 4 chars")
}

func TestRedactMatch_ExactlyEightChars(t *testing.T) {
	out := redactMatch("abcd1234")
	assert.Contains(t, out, "...")
	assert.NotEqual(t, "****", out)
}

// ── shannonEntropy ────────────────────────────────────────────────────────────

func TestShannonEntropy_EmptyString(t *testing.T) {
	assert.Equal(t, 0.0, shannonEntropy(""))
}

func TestShannonEntropy_SingleChar_Zero(t *testing.T) {
	assert.Equal(t, 0.0, shannonEntropy("aaaa"))
}

func TestShannonEntropy_BroadAlphabet_HighEntropy(t *testing.T) {
	// 62 distinct printable ASCII chars — entropy approaches log2(62) ≈ 5.95 bits
	s := "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"
	e := shannonEntropy(s)
	assert.Greater(t, e, 5.5, "62 distinct chars should yield > 5.5 bits entropy")
}

func TestShannonEntropy_HighEntropySecret(t *testing.T) {
	// A typical API key / random token should score > 4.5
	e := shannonEntropy("AKIAIOSFODNN7EXAMPLE1234")
	assert.Greater(t, e, 4.0)
}

func TestShannonEntropy_LowEntropyWord(t *testing.T) {
	e := shannonEntropy("aaaaaabbbb")
	assert.Less(t, e, 2.0)
}

// ── shouldSkipEntropyCheck ────────────────────────────────────────────────────

func TestShouldSkipEntropyCheck_LockFiles(t *testing.T) {
	for _, p := range []string{"package-lock.json", "go.sum", "yarn.lock", "Gemfile.lock", "Pipfile.lock", "composer.lock"} {
		assert.True(t, shouldSkipEntropyCheck(p), "path=%q should be skipped", p)
	}
}

func TestShouldSkipEntropyCheck_BinaryExtensions(t *testing.T) {
	for _, p := range []string{"image.png", "logo.jpg", "font.woff2", "file.bin", "bundle.min.js"} {
		assert.True(t, shouldSkipEntropyCheck(p), "path=%q should be skipped", p)
	}
}

func TestShouldSkipEntropyCheck_SourceFiles_NotSkipped(t *testing.T) {
	for _, p := range []string{"main.go", "service.ts", "handler.py", "config.yaml"} {
		assert.False(t, shouldSkipEntropyCheck(p), "path=%q should NOT be skipped", p)
	}
}

// ── scanLineForEntropy ────────────────────────────────────────────────────────

func TestScanLineForEntropy_HighEntropyToken_Found(t *testing.T) {
	// 31-char token with 31 distinct chars → entropy ≈ log2(31) ≈ 4.95 bits > 4.5 threshold
	line := "SECRET_KEY=ABCDEFGHIJKLMNOPQRSTUVWXYZabcde"
	findings := scanLineForEntropy(line)
	assert.NotEmpty(t, findings, "high-entropy token should be detected")
}

func TestScanLineForEntropy_LowEntropyLine_NotDetected(t *testing.T) {
	findings := scanLineForEntropy("username=admin")
	assert.Empty(t, findings)
}

func TestScanLineForEntropy_ShortToken_NotDetected(t *testing.T) {
	// Tokens shorter than 20 chars are skipped even if high entropy
	findings := scanLineForEntropy(`key="abc123def456"`)
	assert.Empty(t, findings)
}
