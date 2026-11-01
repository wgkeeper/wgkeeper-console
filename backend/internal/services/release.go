// Checking GitHub for the latest wgkeeper-node release.

package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

type LatestNodeRelease struct {
	Tag         string `json:"tag"`
	Version     string `json:"version"`
	URL         string `json:"url"`
	PublishedAt string `json:"publishedAt"`
}

func previewLatestNodeReleaseOverride() *LatestNodeRelease {
	version := strings.TrimSpace(os.Getenv("LATEST_NODE_RELEASE_OVERRIDE"))
	if version == "" {
		return nil
	}

	normalized := normalizeVersionLabel(version)
	if normalized == "" {
		return nil
	}

	return &LatestNodeRelease{
		Tag:         "v" + normalized,
		Version:     normalized,
		URL:         "https://github.com/wgkeeper/wgkeeper-node/releases",
		PublishedAt: time.Now().UTC().Format(time.RFC3339),
	}
}

func (s *DashboardService) GetLatestNodeRelease(ctx context.Context) (*LatestNodeRelease, error) {
	mocked := previewLatestNodeReleaseOverride()
	if mocked != nil {
		cloned := *mocked
		return &cloned, nil
	}

	s.releaseCacheMu.Lock()
	fresh := s.latestRelease != nil && time.Since(s.latestReleaseAt) < s.latestReleaseTTL
	backingOff := time.Since(s.releaseFailedAt) < releaseRetryAfterFailure
	if fresh || backingOff {
		cached := s.cachedReleaseLocked()
		s.releaseCacheMu.Unlock()
		if cached == nil {
			return nil, errors.New("latest node release unavailable, retrying later")
		}
		return cached, nil
	}
	s.releaseCacheMu.Unlock()

	// Concurrent requests share one lookup, and it isn't cut short when the
	// request that started it goes away.
	result, err, _ := s.releaseGroup.Do("latest", func() (any, error) {
		return s.fetchLatestNodeRelease(context.WithoutCancel(ctx))
	})

	s.releaseCacheMu.Lock()
	defer s.releaseCacheMu.Unlock()
	if err != nil {
		s.releaseFailedAt = time.Now()
		if cached := s.cachedReleaseLocked(); cached != nil {
			return cached, nil
		}
		return nil, err
	}
	release, ok := result.(*LatestNodeRelease)
	if !ok {
		return nil, fmt.Errorf("unexpected release lookup result %T", result)
	}
	s.latestRelease = release
	s.latestReleaseAt = time.Now()
	s.releaseFailedAt = time.Time{}
	return s.cachedReleaseLocked(), nil
}

func (s *DashboardService) cachedReleaseLocked() *LatestNodeRelease {
	if s.latestRelease == nil {
		return nil
	}
	cloned := *s.latestRelease
	return &cloned
}

func (s *DashboardService) fetchLatestNodeRelease(ctx context.Context) (*LatestNodeRelease, error) {
	const releasesLatestURL = "https://api.github.com/repos/wgkeeper/wgkeeper-node/releases/latest"

	client := s.httpClient(2500 * time.Millisecond)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, releasesLatestURL, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "wgkeeper-console")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer closeResponseBody(resp)

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("github latest release returned status %d", resp.StatusCode)
	}

	var payload struct {
		TagName     string `json:"tag_name"`
		HTMLURL     string `json:"html_url"`
		PublishedAt string `json:"published_at"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&payload); err != nil {
		return nil, err
	}

	version := normalizeVersionLabel(payload.TagName)
	if version == "" {
		return nil, fmt.Errorf("github latest release did not include a valid tag")
	}

	return &LatestNodeRelease{
		Tag:         strings.TrimSpace(payload.TagName),
		Version:     version,
		URL:         strings.TrimSpace(payload.HTMLURL),
		PublishedAt: strings.TrimSpace(payload.PublishedAt),
	}, nil
}

func normalizeVersionLabel(raw string) string {
	trimmed := strings.TrimSpace(raw)
	trimmed = strings.TrimPrefix(trimmed, "v")
	trimmed = strings.TrimPrefix(trimmed, "V")
	return trimmed
}

func parseComparableVersion(raw string) ([3]int, bool) {
	var parsed [3]int

	normalized := normalizeVersionLabel(raw)
	if normalized == "" {
		return parsed, false
	}

	if idx := strings.IndexAny(normalized, "-+"); idx >= 0 {
		normalized = normalized[:idx]
	}

	parts := strings.Split(normalized, ".")
	if len(parts) == 0 || len(parts) > 3 {
		return parsed, false
	}

	for i, part := range parts {
		if part == "" {
			return parsed, false
		}
		value, err := strconv.Atoi(part)
		if err != nil {
			return parsed, false
		}
		parsed[i] = value
	}

	return parsed, true
}

func IsVersionOutdated(currentVersion *string, latestRelease *LatestNodeRelease) bool {
	if currentVersion == nil || latestRelease == nil {
		return false
	}

	current, ok := parseComparableVersion(*currentVersion)
	if !ok {
		return false
	}

	latest, ok := parseComparableVersion(latestRelease.Version)
	if !ok {
		return false
	}

	for i := range current {
		if current[i] < latest[i] {
			return true
		}
		if current[i] > latest[i] {
			return false
		}
	}

	return false
}
