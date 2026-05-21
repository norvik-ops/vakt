// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package secvault

import (
	"bufio"
	"context"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// credentialInURLRegex matches embedded credentials in URLs of the form
// scheme://user:password@host — used to redact them from log/error output.
var credentialInURLRegex = regexp.MustCompile(`://[^:@\s]+:[^@\s]+@`)

// sanitizeGitURL removes embedded credentials from a git URL so that it is safe
// to include in error messages and log output.
func sanitizeGitURL(s string) string {
	return credentialInURLRegex.ReplaceAllString(s, "://<redacted>@")
}

// validBranchRegex defines an allowlist for branch names.
// It intentionally excludes leading hyphens to prevent arg-injection into git.
var validBranchRegex = regexp.MustCompile(`^[a-zA-Z0-9/_.\-]{1,255}$`)

// validateBranch returns an error if the branch name is not safe to pass as a
// git CLI argument.
func validateBranch(branch string) error {
	if strings.HasPrefix(branch, "-") {
		return fmt.Errorf("invalid branch name: must not start with '-'")
	}
	if !validBranchRegex.MatchString(branch) {
		return fmt.Errorf("invalid branch name: only alphanumerics, '/', '_', '.', and '-' are allowed")
	}
	return nil
}

// Pattern represents a secret-detection regex pattern with associated metadata.
type Pattern struct {
	Name     string
	Regex    *regexp.Regexp
	Severity string
}

// builtinPatterns contains the default set of secret-detection patterns.
var builtinPatterns = []Pattern{
	{
		Name:     "aws_access_key",
		Regex:    regexp.MustCompile(`AKIA[0-9A-Z]{16}`),
		Severity: "critical",
	},
	{
		Name:     "github_token",
		Regex:    regexp.MustCompile(`ghp_[A-Za-z0-9]{36}`),
		Severity: "critical",
	},
	{
		Name:     "stripe_key",
		Regex:    regexp.MustCompile(`sk_live_[A-Za-z0-9]{24}`),
		Severity: "critical",
	},
	{
		Name:     "private_key",
		Regex:    regexp.MustCompile(`-----BEGIN (?:RSA |EC |OPENSSH )?PRIVATE KEY`),
		Severity: "critical",
	},
	{
		Name:     "generic_api_key",
		Regex:    regexp.MustCompile(`(?i)api[_-]?key\s*[:=]\s*["']?([A-Za-z0-9+/]{32,})`),
		Severity: "high",
	},
	{
		Name:     "password_assignment",
		Regex:    regexp.MustCompile(`(?i)password\s*[:=]\s*["']([^"'\n]{8,})`),
		Severity: "high",
	},
	{
		Name:     "jwt_token",
		Regex:    regexp.MustCompile(`eyJ[A-Za-z0-9_-]{10,}\.eyJ[A-Za-z0-9_-]{10,}`),
		Severity: "medium",
	},
}

// extensionsToSkipEntropy lists file extensions for which entropy analysis is skipped
// because the content is expected to have high entropy but is not secrets.
var extensionsToSkipEntropy = map[string]bool{
	".lock":             true,
	".min.js":           true,
	".png":              true,
	".jpg":              true,
	".jpeg":             true,
	".gif":              true,
	".pdf":              true,
	".svg":              true,
	".ico":              true,
	".woff":             true,
	".woff2":            true,
	".ttf":              true,
	".eot":              true,
	".map":              true,
	".bin":              true,
	"package-lock.json": true,
	"yarn.lock":         true,
	"go.sum":            true,
	"composer.lock":     true,
	"Gemfile.lock":      true,
	"Pipfile.lock":      true,
}

// redactMatch returns a preview string of the form "first4...last4".
// If the input is shorter than 8 characters the entire string is replaced with "****".
func redactMatch(s string) string {
	runes := []rune(s)
	if len(runes) < 8 {
		return "****"
	}
	return string(runes[:4]) + "..." + string(runes[len(runes)-4:])
}

// shannonEntropy calculates the Shannon entropy (in bits) of s.
func shannonEntropy(s string) float64 {
	if len(s) == 0 {
		return 0
	}
	freq := make(map[rune]float64)
	for _, r := range s {
		freq[r]++
	}
	entropy := 0.0
	l := float64(len([]rune(s)))
	for _, f := range freq {
		p := f / l
		if p > 0 {
			entropy -= p * math.Log2(p)
		}
	}
	return entropy
}

// shouldSkipEntropyCheck returns true for file paths/names that are known to
// contain high-entropy content which is not secrets (minified JS, lock files, binary assets, etc.).
func shouldSkipEntropyCheck(filePath string) bool {
	base := filepath.Base(filePath)
	// Check exact file name first (e.g. "package-lock.json", "go.sum")
	if extensionsToSkipEntropy[base] {
		return true
	}
	// Check extension
	ext := strings.ToLower(filepath.Ext(filePath))
	if extensionsToSkipEntropy[ext] {
		return true
	}
	// Check combined suffix for .min.js
	lower := strings.ToLower(filePath)
	if strings.HasSuffix(lower, ".min.js") || strings.HasSuffix(lower, ".min.css") {
		return true
	}
	return false
}

// entropyFinding holds the token and line from an entropy scan.
type entropyFinding struct {
	token string
	line  string
}

// scanLineForEntropy checks a line for high-entropy tokens (> 4.5 bits, length >= 20).
// Only base64-like (A-Za-z0-9+/=) and hex-like tokens are considered.
func scanLineForEntropy(line string) []entropyFinding {
	const minLen = 20
	const minEntropy = 4.5

	var findings []entropyFinding

	// Tokenise by splitting on common delimiters
	tokens := regexp.MustCompile(`[\s"'=:,{}\[\]()]`).Split(line, -1)
	for _, tok := range tokens {
		if len(tok) < minLen {
			continue
		}
		// Only consider base64 / hex alphabet
		if !regexp.MustCompile(`^[A-Za-z0-9+/=_\-]+$`).MatchString(tok) {
			continue
		}
		if shannonEntropy(tok) > minEntropy {
			findings = append(findings, entropyFinding{token: tok, line: line})
		}
	}
	return findings
}

// RunGitScan clones the repository into a temporary directory, scans every file for
// secrets using both regex patterns and entropy analysis, then returns all findings.
//
// Credentials are used only for the git clone subprocess and are never persisted.
func RunGitScan(ctx context.Context, input TriggerGitScanInput) ([]ScanResult, error) {
	// Re-validate URL here as defense-in-depth: the Asynq payload could be crafted
	// by a malicious operator with Redis access, bypassing the handler-layer check.
	if err := ValidateRepoURL(input.RepoURL); err != nil {
		return nil, fmt.Errorf("repo url validation failed in worker: %w", err)
	}

	tmpDir, err := os.MkdirTemp("", "vakt-gitscan-*")
	if err != nil {
		return nil, fmt.Errorf("create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	// Validate branch name before passing it to the git CLI.
	if err := validateBranch(input.Branch); err != nil {
		return nil, err
	}

	cloneURL := input.RepoURL
	if input.Credentials != nil {
		cloneURL, err = injectCredentials(input.RepoURL, input.Credentials)
		if err != nil {
			return nil, fmt.Errorf("inject credentials: %w", err)
		}
	}

	//nolint:gosec // URL is constructed from validated input; tmpDir is OS temp
	cloneCmd := exec.CommandContext(ctx, "git", "clone", "--depth=1",
		"--branch", input.Branch, cloneURL, tmpDir)
	cloneOut, cloneErr := cloneCmd.CombinedOutput()
	if cloneErr != nil {
		// Sanitize cloneOut before including it in the error — it may contain
		// embedded credentials from the clone URL.
		safeOut := sanitizeGitURL(string(cloneOut))
		return nil, fmt.Errorf("git clone failed: %w\n%s", cloneErr, safeOut)
	}

	var results []ScanResult

	walkErr := filepath.WalkDir(tmpDir, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil // skip unreadable entries
		}
		// Skip the .git directory entirely
		if d.IsDir() && d.Name() == ".git" {
			return filepath.SkipDir
		}
		if d.IsDir() {
			return nil
		}

		relPath, _ := filepath.Rel(tmpDir, path)
		findings, scanErr := scanFile(path, relPath, input.RepoURL, shouldSkipEntropyCheck(path))
		if scanErr != nil {
			return nil // skip unreadable files
		}
		results = append(results, findings...)
		return nil
	})
	if walkErr != nil {
		return nil, fmt.Errorf("walk repo: %w", walkErr)
	}

	return results, nil
}

// scanFile reads a single file and returns ScanResult entries for every finding.
func scanFile(absPath, relPath, repoURL string, skipEntropy bool) ([]ScanResult, error) {
	f, err := os.Open(absPath) //nolint:gosec
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var results []ScanResult
	lineNum := 0
	scanner := bufio.NewScanner(f)

	// Increase buffer for long lines (e.g. minified files)
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)

	for scanner.Scan() {
		lineNum++
		line := scanner.Text()

		// Regex pattern scan
		for _, p := range builtinPatterns {
			if p.Regex.MatchString(line) {
				match := p.Regex.FindString(line)
				results = append(results, ScanResult{
					RepoURL:      repoURL,
					FilePath:     relPath,
					LineNumber:   lineNum,
					PatternName:  p.Name,
					MatchPreview: redactMatch(match),
					Severity:     p.Severity,
					Status:       "open",
					CreatedAt:    time.Now().UTC(),
				})
			}
		}

		// Entropy analysis (skip for binary/minified/lock files)
		if !skipEntropy {
			for _, ef := range scanLineForEntropy(line) {
				results = append(results, ScanResult{
					RepoURL:      repoURL,
					FilePath:     relPath,
					LineNumber:   lineNum,
					PatternName:  "high_entropy_string",
					MatchPreview: redactMatch(ef.token),
					Severity:     "medium",
					Status:       "open",
					CreatedAt:    time.Now().UTC(),
				})
			}
		}
	}

	return results, scanner.Err()
}

// injectCredentials embeds authentication into the clone URL so that git can
// clone private repositories without interactive prompts.
// Credentials are NEVER stored — they exist only in the subprocess environment.
func injectCredentials(repoURL string, creds *GitScanCredentials) (string, error) {
	switch creds.Type {
	case "github_token":
		// For GitHub: https://x-access-token:<token>@github.com/...
		if !strings.HasPrefix(repoURL, "https://") {
			return "", fmt.Errorf("github_token credential requires an HTTPS URL")
		}
		return strings.Replace(repoURL, "https://", "https://x-access-token:"+creds.Token+"@", 1), nil
	case "basic":
		if !strings.HasPrefix(repoURL, "https://") && !strings.HasPrefix(repoURL, "http://") {
			return "", fmt.Errorf("basic credential requires an HTTP(S) URL")
		}
		scheme := "https://"
		if strings.HasPrefix(repoURL, "http://") {
			scheme = "http://"
		}
		rest := strings.TrimPrefix(strings.TrimPrefix(repoURL, "https://"), "http://")
		return scheme + creds.User + ":" + creds.Pass + "@" + rest, nil
	default:
		return "", fmt.Errorf("unsupported credential type: %s", creds.Type)
	}
}
