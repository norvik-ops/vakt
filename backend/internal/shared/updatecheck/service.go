package updatecheck

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"
)

const (
	cacheKey          = "system:latest_version"
	overrideKey       = "system:update_check_enabled"
	cacheTTL          = 24 * time.Hour
	githubReleasesURL = "https://api.github.com/repos/norvik-ops/vatk/releases/latest"
)

type UpdateInfo struct {
	Enabled         bool   `json:"check_enabled"`
	CurrentVersion  string `json:"current_version"`
	LatestVersion   string `json:"latest_version,omitempty"`
	UpdateAvailable bool   `json:"update_available"`
	ReleaseURL      string `json:"release_url,omitempty"`
}

type Service struct {
	enabled        bool
	currentVersion string
	rdb            *redis.Client
	client         *http.Client
}

func NewService(enabled bool, currentVersion string, rdb *redis.Client) *Service {
	return &Service{
		enabled:        enabled,
		currentVersion: normalizeVersion(currentVersion),
		rdb:            rdb,
		client:         &http.Client{Timeout: 10 * time.Second},
	}
}

// isEnabled returns true if update checks are active.
// Redis override (set via PUT /system/update) takes precedence over the env var.
func (s *Service) isEnabled(ctx context.Context) bool {
	v, err := s.rdb.Get(ctx, overrideKey).Result()
	if err == nil {
		return v == "true"
	}
	return s.enabled
}

// SetEnabled persists the toggle to Redis (survives restarts until explicitly changed).
func (s *Service) SetEnabled(ctx context.Context, enabled bool) error {
	v := "false"
	if enabled {
		v = "true"
	}
	return s.rdb.Set(ctx, overrideKey, v, 0).Err()
}

// GetUpdateInfo returns current update status, reading from Redis cache first.
// If the cache is empty or stale, it fetches from the GitHub releases API.
func (s *Service) GetUpdateInfo(ctx context.Context) UpdateInfo {
	if !s.isEnabled(ctx) {
		return UpdateInfo{Enabled: false, CurrentVersion: s.currentVersion}
	}

	latest := s.cachedLatest(ctx)
	if latest == "" {
		latest = s.fetchLatest(ctx)
		if latest != "" {
			s.rdb.Set(ctx, cacheKey, latest, cacheTTL)
		}
	}

	info := UpdateInfo{
		Enabled:        true,
		CurrentVersion: s.currentVersion,
		LatestVersion:  latest,
	}
	if latest != "" {
		info.UpdateAvailable = isNewer(latest, s.currentVersion)
		info.ReleaseURL = "https://github.com/norvik-ops/vatk/releases/tag/" + latest
	}
	return info
}

// StartBackgroundRefresh runs a daily goroutine that refreshes the cached latest version.
// The goroutine always starts so that toggling the check on via the UI takes effect
// at the next tick without requiring a restart.
func (s *Service) StartBackgroundRefresh(ctx context.Context) {
	go func() {
		// Initial fetch after 30s (let the app fully start up first).
		select {
		case <-time.After(30 * time.Second):
		case <-ctx.Done():
			return
		}
		if s.isEnabled(ctx) {
			s.refresh(ctx)
		}

		ticker := time.NewTicker(cacheTTL)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				if s.isEnabled(ctx) {
					s.refresh(ctx)
				}
			case <-ctx.Done():
				return
			}
		}
	}()
}

func (s *Service) refresh(ctx context.Context) {
	latest := s.fetchLatest(ctx)
	if latest != "" {
		if err := s.rdb.Set(ctx, cacheKey, latest, cacheTTL).Err(); err != nil {
			log.Warn().Err(err).Msg("updatecheck: failed to cache latest version")
		}
	}
}

func (s *Service) cachedLatest(ctx context.Context) string {
	v, err := s.rdb.Get(ctx, cacheKey).Result()
	if err != nil {
		return ""
	}
	return v
}

func (s *Service) fetchLatest(ctx context.Context) string {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, githubReleasesURL, nil)
	if err != nil {
		return ""
	}
	req.Header.Set("User-Agent", "vakt-update-check/1.0")
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := s.client.Do(req)
	if err != nil {
		log.Debug().Err(err).Msg("updatecheck: GitHub API unreachable")
		return ""
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Debug().Int("status", resp.StatusCode).Msg("updatecheck: GitHub API returned non-200")
		return ""
	}

	var release struct {
		TagName string `json:"tag_name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return ""
	}
	return normalizeVersion(release.TagName)
}

// normalizeVersion strips a leading "v" so "v1.2.3" and "1.2.3" compare equally.
func normalizeVersion(v string) string {
	return strings.TrimPrefix(strings.TrimSpace(v), "v")
}

// isNewer returns true if candidate is strictly newer than current using simple semver comparison.
func isNewer(candidate, current string) bool {
	return candidate != "" && current != "" && candidate != current && semverGT(candidate, current)
}

// semverGT compares two "X.Y.Z" version strings and returns true if a > b.
func semverGT(a, b string) bool {
	ap := parseSemver(a)
	bp := parseSemver(b)
	for i := 0; i < 3; i++ {
		if ap[i] > bp[i] {
			return true
		}
		if ap[i] < bp[i] {
			return false
		}
	}
	return false
}

func parseSemver(v string) [3]int {
	var parts [3]int
	fmt.Sscanf(v, "%d.%d.%d", &parts[0], &parts[1], &parts[2])
	return parts
}
