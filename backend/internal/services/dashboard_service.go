package services

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"wg-keeper-backend/internal/database"
	"wg-keeper-backend/internal/models"

	"github.com/google/uuid"
	"golang.org/x/sync/errgroup"
	"golang.org/x/sync/singleflight"
	"gorm.io/gorm"
)

const (
	// releaseRetryAfterFailure keeps a failed GitHub lookup (no internet,
	// rate limit) from slowing down every node list request.
	releaseRetryAfterFailure = 5 * time.Minute
	// refreshTimeout bounds a status sweep below the default 30s HTTP write
	// timeout, so the client always gets an answer.
	refreshTimeout = 20 * time.Second
	// refreshConcurrency is how many nodes a sweep checks at once.
	refreshConcurrency = 10
)

var sharedHTTPTransport = &http.Transport{
	Proxy: http.ProxyFromEnvironment,
	DialContext: (&net.Dialer{
		Timeout:   3 * time.Second,
		KeepAlive: 30 * time.Second,
	}).DialContext,
	ForceAttemptHTTP2:     true,
	MaxIdleConns:          100,
	MaxIdleConnsPerHost:   20,
	IdleConnTimeout:       90 * time.Second,
	TLSHandshakeTimeout:   3 * time.Second,
	ExpectContinueTimeout: time.Second,
}

var sharedHTTPRoundTripper http.RoundTripper = sharedHTTPTransport

type DashboardService struct {
	db               *gorm.DB
	releaseCacheMu   sync.Mutex
	latestRelease    *LatestNodeRelease
	latestReleaseAt  time.Time
	latestReleaseTTL time.Duration
	releaseFailedAt  time.Time
	releaseGroup     singleflight.Group
	refreshGroup     singleflight.Group
	// sweepTimeout overrides refreshTimeout in tests.
	sweepTimeout time.Duration
}

func NewDashboardService() *DashboardService {
	return &DashboardService{
		db:               database.DB,
		latestReleaseTTL: 10 * time.Minute,
	}
}

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

func parseJSONErrorBody(body []byte) any {
	if len(body) == 0 {
		return nil
	}

	var errorData any
	if err := json.Unmarshal(body, &errorData); err != nil {
		return nil
	}
	return errorData
}

func stripTrailingSlash(address string) string {
	return strings.TrimRight(address, "/")
}

func newHTTPClient(timeout time.Duration) *http.Client {
	return &http.Client{
		Timeout:   timeout,
		Transport: sharedHTTPRoundTripper,
		// Never follow redirects: Go would resend X-API-KEY to the new host
		// (it only strips Authorization and Cookie), and a 307 would replay
		// a POST with peer data to wherever the node points.
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}

func (s *DashboardService) GetNodes(ctx context.Context, page, limit int) ([]models.Node, int64, error) {
	var nodes []models.Node
	var total int64

	if err := s.db.WithContext(ctx).Model(&models.Node{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * limit
	err := s.db.WithContext(ctx).Order("created_at DESC").Offset(offset).Limit(limit).Find(&nodes).Error
	return nodes, total, err
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

	client := newHTTPClient(2500 * time.Millisecond)
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

func (s *DashboardService) FindNodeByID(ctx context.Context, id string) (*models.Node, error) {
	return s.findNode(ctx, "id = ?", id)
}

func (s *DashboardService) FindNodeByAddress(ctx context.Context, address string) (*models.Node, error) {
	return s.findNode(ctx, "address = ?", address)
}

func (s *DashboardService) findNode(ctx context.Context, query string, arg string) (*models.Node, error) {
	var node models.Node
	err := s.db.WithContext(ctx).Where(query, arg).First(&node).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNodeNotFound
	}
	if err != nil {
		return nil, err
	}
	return &node, nil
}

func (s *DashboardService) CreateNode(ctx context.Context, input struct {
	Address string
	APIKey  string
}) (*models.Node, error) {
	address := stripTrailingSlash(input.Address)
	name := fmt.Sprintf("wg-%s", address)

	health, err := s.checkHealth(ctx, address, input.APIKey)
	if err != nil {
		return nil, err
	}

	node := models.Node{
		ID:      uuid.New().String(),
		Name:    name,
		Address: address,
		APIKey:  models.SecretString(input.APIKey),
		Status:  health.Status,
		Version: health.Version,
	}

	err = s.db.WithContext(ctx).Create(&node).Error
	if errors.Is(err, gorm.ErrDuplicatedKey) {
		// Another request added the same address after the handler's check.
		return nil, ErrNodeExists
	}
	if err != nil {
		return nil, err
	}

	return &node, nil
}

func (s *DashboardService) DeleteNode(ctx context.Context, id string) (*models.Node, error) {
	node, err := s.FindNodeByID(ctx, id)
	if err != nil {
		return nil, err
	}
	err = s.db.WithContext(ctx).Delete(node).Error
	return node, err
}

// RefreshStatuses checks every node and stores its status and version.
// Concurrent calls (several tabs, several admins) share one sweep, which runs
// for at most refreshTimeout even if the request that started it goes away.
func (s *DashboardService) RefreshStatuses(ctx context.Context) error {
	_, err, _ := s.refreshGroup.Do("refresh", func() (any, error) {
		timeout := refreshTimeout
		if s.sweepTimeout > 0 {
			timeout = s.sweepTimeout
		}
		sweepCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), timeout)
		defer cancel()
		return nil, s.refreshStatuses(sweepCtx)
	})
	return err
}

func (s *DashboardService) refreshStatuses(ctx context.Context) error {
	var nodes []models.Node
	if err := s.db.WithContext(ctx).Find(&nodes).Error; err != nil {
		return err
	}

	results := make([]*HealthResult, len(nodes))
	var g errgroup.Group
	g.SetLimit(refreshConcurrency)
	for i := range nodes {
		g.Go(func() error {
			health, err := s.checkHealth(ctx, nodes[i].Address, string(nodes[i].APIKey))
			// A check cut off by the sweep deadline says nothing about the
			// node: keep its last known status instead of marking it offline.
			if err == nil && ctx.Err() == nil {
				results[i] = health
			}
			return nil
		})
	}
	_ = g.Wait() //nolint:errcheck // the workers never return an error

	// The sweep context may have just expired; give the write its own short one.
	writeCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	return s.db.WithContext(writeCtx).Transaction(func(tx *gorm.DB) error {
		for i, health := range results {
			if health == nil {
				continue
			}
			updates := map[string]any{"status": health.Status}
			if health.Version != nil {
				updates["version"] = health.Version
			}
			if err := tx.Model(&models.Node{}).Where("id = ?", nodes[i].ID).Updates(updates).Error; err != nil {
				return fmt.Errorf("update node %s: %w", nodes[i].ID, err)
			}
		}
		return nil
	})
}

func (s *DashboardService) GetNodeStats(ctx context.Context, id string) (*StatsResponse, error) {
	node, err := s.FindNodeByID(ctx, id)
	if err != nil {
		return nil, err
	}

	return s.fetchStatsData(ctx, node.Address, string(node.APIKey))
}

func (s *DashboardService) CheckNode(ctx context.Context, address string, apiKey string) (*HealthResult, error) {
	return s.checkHealth(ctx, stripTrailingSlash(address), apiKey)
}

type HealthResult struct {
	Status      models.NodeStatus
	Version     *string
	ServiceName *string
	Error       string
}

func (s *DashboardService) checkHealth(ctx context.Context, address string, apiKey string) (*HealthResult, error) {
	// Check health endpoint
	healthResp, err := s.fetchWithTimeout(ctx, address+"/readyz", apiKey, 2*time.Second, "GET", nil, "")
	if err != nil {
		return &HealthResult{Status: models.NodeStatusOffline}, nil
	}

	if healthResp.StatusCode == 401 || healthResp.StatusCode == 403 {
		return &HealthResult{
			Status: models.NodeStatusOffline,
			Error:  "invalid_api_key",
		}, nil
	}

	if !healthResp.OK {
		return &HealthResult{Status: models.NodeStatusOffline}, nil
	}

	// Get stats to extract version
	statsResp, err := s.fetchStatsData(ctx, address, apiKey)
	if err != nil {
		return &HealthResult{Status: models.NodeStatusOffline}, nil
	}

	if statsResp.StatusCode != nil && (*statsResp.StatusCode == 401 || *statsResp.StatusCode == 403) {
		return &HealthResult{
			Status: models.NodeStatusOffline,
			Error:  "invalid_api_key",
		}, nil
	}

	var version *string
	var serviceName *string
	if statsResp.Data != nil {
		if statsResp.Data.Service != nil {
			version = statsResp.Data.Service.Version
			serviceName = statsResp.Data.Service.Name
		}
	}

	return &HealthResult{
		Status:      models.NodeStatusOnline,
		Version:     version,
		ServiceName: serviceName,
	}, nil
}

type StatsResponse struct {
	Data       *StatsData `json:"data"`
	StatusCode *int       `json:"status,omitempty"`
	Endpoint   *string    `json:"endpoint,omitempty"`
}

type StatsData struct {
	Service   *ServiceData   `json:"service,omitempty"`
	Wireguard *WireguardData `json:"wireguard,omitempty"`
	Peers     *PeersData     `json:"peers,omitempty"`
	StartedAt *string        `json:"startedAt,omitempty"`
}

type ServiceData struct {
	Name    *string `json:"name,omitempty"`
	Version *string `json:"version,omitempty"`
}

type WireguardData struct {
	Interface       *string  `json:"interface,omitempty"`
	ListenPort      *int     `json:"listenPort,omitempty"`
	Subnets         []string `json:"subnets,omitempty"`
	ServerIPs       []string `json:"serverIps,omitempty"`
	AddressFamilies []string `json:"addressFamilies,omitempty"`
}

type PeersData struct {
	Possible *int `json:"possible,omitempty"`
	Issued   *int `json:"issued,omitempty"`
	Active   *int `json:"active,omitempty"`
}

func (s *DashboardService) fetchStatsData(ctx context.Context, address string, apiKey string) (*StatsResponse, error) {
	endpoints := []string{"/stats", "/stats/"}

	var lastStatus *int
	var lastEndpoint *string

	for _, endpoint := range endpoints {
		resp, err := s.fetchJSONWithTimeout(ctx, address+endpoint, apiKey, 2500*time.Millisecond, true, "GET", nil, "")
		if err != nil {
			// The node is unreachable; trying another path would only double
			// the wait.
			break
		}

		status := resp.StatusCode
		lastStatus = &status
		lastEndpoint = &endpoint

		if resp.OK {
			return &StatsResponse{
				Data:       resp.Data,
				StatusCode: &status,
				Endpoint:   &endpoint,
			}, nil
		}

		// Only a missing /stats is worth retrying as /stats/ (older nodes).
		if status != http.StatusNotFound {
			break
		}
	}

	return &StatsResponse{
		Data:       nil,
		StatusCode: lastStatus,
		Endpoint:   lastEndpoint,
	}, nil
}

type FetchResponse struct {
	OK         bool
	StatusCode int
	Data       *StatsData
	ErrorData  any
}

func (s *DashboardService) fetchWithTimeout(ctx context.Context, urlStr string, apiKey string, timeout time.Duration, method string, body []byte, contentType string) (*FetchResponse, error) {
	client := newHTTPClient(timeout)
	req, err := http.NewRequestWithContext(ctx, method, urlStr, bytes.NewReader(body))
	if err != nil {
		return &FetchResponse{OK: false}, err
	}

	req.Header.Set("X-API-KEY", apiKey)
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}

	resp, err := client.Do(req)
	if err != nil {
		return &FetchResponse{OK: false}, err
	}
	defer closeResponseBody(resp)

	return &FetchResponse{
		OK:         resp.StatusCode >= 200 && resp.StatusCode < 300,
		StatusCode: resp.StatusCode,
	}, nil
}

func (s *DashboardService) fetchJSONWithTimeout(ctx context.Context, urlStr string, apiKey string, timeout time.Duration, allowNonJSON bool, method string, body []byte, contentType string) (*FetchResponse, error) {
	client := newHTTPClient(timeout)
	req, err := http.NewRequestWithContext(ctx, method, urlStr, bytes.NewReader(body))
	if err != nil {
		return &FetchResponse{OK: false}, err
	}

	req.Header.Set("X-API-KEY", apiKey)
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}

	resp, err := client.Do(req)
	if err != nil {
		return &FetchResponse{OK: false}, err
	}
	defer closeResponseBody(resp)

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		bodyBytes, err := readBody(resp, maxErrorBodyBytes)
		if err != nil {
			// Best effort: the status code alone still describes the failure.
			bodyBytes = nil
		}
		return &FetchResponse{
			OK:         false,
			StatusCode: resp.StatusCode,
			ErrorData:  parseJSONErrorBody(bodyBytes),
		}, nil
	}

	contentTypeHeader := resp.Header.Get("Content-Type")
	if !allowNonJSON && !strings.Contains(contentTypeHeader, "application/json") {
		return &FetchResponse{
			OK:         false,
			StatusCode: resp.StatusCode,
		}, nil
	}

	bodyBytes, err := readBody(resp, maxNodeResponseBytes)
	if err != nil {
		return &FetchResponse{OK: false, StatusCode: resp.StatusCode}, unreachable(err)
	}

	var data StatsData
	if err := json.Unmarshal(bodyBytes, &data); err != nil {
		return &FetchResponse{OK: false, StatusCode: resp.StatusCode}, err
	}

	return &FetchResponse{
		OK:         true,
		StatusCode: resp.StatusCode,
		Data:       &data,
	}, nil
}

func (s *DashboardService) GetNodeConfig(ctx context.Context, id string, peerID string, dns []string, expiresAt *string, addressFamilies []string) (*ConfigResponse, error) {
	node, err := s.FindNodeByID(ctx, id)
	if err != nil {
		return nil, err
	}

	return s.fetchConfigData(ctx, node.Address, string(node.APIKey), peerID, dns, expiresAt, addressFamilies)
}

type ConfigResponse struct {
	Data       *string
	StatusCode *int
	Endpoint   *string
	ErrorData  any
}

type PeerConfigResponse struct {
	Server *ServerData `json:"server,omitempty"`
	Peer   *PeerData   `json:"peer,omitempty"`
}

type ServerData struct {
	PublicKey  *string `json:"publicKey,omitempty"`
	ListenPort *int    `json:"listenPort,omitempty"`
}

type PeerData struct {
	PeerID          *string  `json:"peerId,omitempty"`
	PublicKey       *string  `json:"publicKey,omitempty"`
	PrivateKey      *string  `json:"privateKey,omitempty"`
	PresharedKey    *string  `json:"presharedKey,omitempty"`
	AllowedIPs      []string `json:"allowedIPs,omitempty"`
	AddressFamilies []string `json:"addressFamilies,omitempty"`
}

func (s *DashboardService) fetchConfigData(ctx context.Context, address string, apiKey string, peerID string, dns []string, expiresAt *string, addressFamilies []string) (*ConfigResponse, error) {
	parsedURL, err := url.Parse(address)
	if err != nil {
		return nil, fmt.Errorf("parse node address: %w", err)
	}
	host := parsedURL.Hostname()

	endpoints := []string{"/peers"}

	var lastStatus *int
	var lastEndpoint *string
	var lastErrorData any

	bodyObj := map[string]any{
		"peerId": peerID,
	}
	if expiresAt != nil && *expiresAt != "" {
		bodyObj["expiresAt"] = *expiresAt
	}
	if len(addressFamilies) > 0 {
		bodyObj["addressFamilies"] = addressFamilies
	}

	bodyBytes, err := json.Marshal(bodyObj)
	if err != nil {
		return nil, fmt.Errorf("encode peer request: %w", err)
	}

	for _, endpoint := range endpoints {
		resp, err := s.fetchPeerConfig(ctx, address+endpoint, apiKey, 2500*time.Millisecond, bodyBytes)
		if err != nil {
			continue
		}

		status := resp.StatusCode
		lastStatus = &status
		lastEndpoint = &endpoint
		lastErrorData = resp.ErrorData

		if resp.OK && resp.Data != nil {
			peer := resp.Data.Peer
			server := resp.Data.Server

			if peer == nil || server == nil {
				return &ConfigResponse{
					Data:       nil,
					StatusCode: &status,
					Endpoint:   &endpoint,
				}, nil
			}

			privateKey := ""
			if peer.PrivateKey != nil {
				privateKey = strings.TrimSpace(*peer.PrivateKey)
			}
			allowedIPs := peer.AllowedIPs
			allowedIP := ""
			hasV4 := false
			hasV6 := false
			if len(allowedIPs) > 0 {
				var filtered []string
				for _, ip := range allowedIPs {
					trimmed := strings.TrimSpace(ip)
					if trimmed == "" {
						continue
					}
					filtered = append(filtered, trimmed)
					addr := trimmed
					if idx := strings.Index(addr, "/"); idx >= 0 {
						addr = addr[:idx]
					}
					if strings.Contains(addr, ":") {
						hasV6 = true
					} else if strings.Contains(addr, ".") {
						hasV4 = true
					}
				}
				allowedIP = strings.Join(filtered, ", ")
			}
			// Defensive: if family detection failed, route both (preserves prior behavior).
			if !hasV4 && !hasV6 {
				hasV4 = true
				hasV6 = true
			}
			presharedKey := ""
			if peer.PresharedKey != nil {
				presharedKey = strings.TrimSpace(*peer.PresharedKey)
			}
			serverPublicKey := ""
			if server.PublicKey != nil {
				serverPublicKey = strings.TrimSpace(*server.PublicKey)
			}
			listenPort := 0
			if server.ListenPort != nil {
				listenPort = *server.ListenPort
			}

			if privateKey == "" || allowedIP == "" || presharedKey == "" || serverPublicKey == "" || listenPort == 0 {
				return &ConfigResponse{
					Data:       nil,
					StatusCode: &status,
					Endpoint:   &endpoint,
				}, nil
			}

			candidateDNS := dns
			if len(candidateDNS) == 0 {
				candidateDNS = []string{"1.1.1.1", "1.0.0.1", "2606:4700:4700::1111", "2606:4700:4700::1001"}
			}
			var effectiveDNS []string
			for _, d := range candidateDNS {
				if strings.Contains(d, ":") {
					if hasV6 {
						effectiveDNS = append(effectiveDNS, d)
					}
				} else if hasV4 {
					effectiveDNS = append(effectiveDNS, d)
				}
			}

			var allowedIPsParts []string
			if hasV4 {
				allowedIPsParts = append(allowedIPsParts, "0.0.0.0/0")
			}
			if hasV6 {
				allowedIPsParts = append(allowedIPsParts, "::/0")
			}
			allowedIPsLine := strings.Join(allowedIPsParts, ", ")

			config := fmt.Sprintf(`[Interface]
PrivateKey = %s
Address = %s
DNS = %s

[Peer]
PublicKey = %s
PresharedKey = %s
AllowedIPs = %s
Endpoint = %s
PersistentKeepalive = 25

`, privateKey, allowedIP, strings.Join(effectiveDNS, ", "), serverPublicKey, presharedKey, allowedIPsLine,
				// JoinHostPort brackets IPv6 hosts: [fd00::1]:51820.
				net.JoinHostPort(host, strconv.Itoa(listenPort)))

			return &ConfigResponse{
				Data:       &config,
				StatusCode: &status,
				Endpoint:   &endpoint,
			}, nil
		}

		if status == 404 {
			continue
		}
	}

	return &ConfigResponse{
		Data:       nil,
		StatusCode: lastStatus,
		Endpoint:   lastEndpoint,
		ErrorData:  lastErrorData,
	}, nil
}

func (s *DashboardService) fetchPeerConfig(ctx context.Context, urlStr string, apiKey string, timeout time.Duration, body []byte) (*PeerConfigFetchResponse, error) {
	client := newHTTPClient(timeout)
	req, err := http.NewRequestWithContext(ctx, "POST", urlStr, bytes.NewReader(body))
	if err != nil {
		return &PeerConfigFetchResponse{OK: false}, err
	}

	req.Header.Set("X-API-KEY", apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return &PeerConfigFetchResponse{OK: false}, err
	}
	defer closeResponseBody(resp)

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		bodyBytes, err := readBody(resp, maxErrorBodyBytes)
		if err != nil {
			bodyBytes = nil
		}
		return &PeerConfigFetchResponse{
			OK:         false,
			StatusCode: resp.StatusCode,
			ErrorData:  parseJSONErrorBody(bodyBytes),
		}, nil
	}

	bodyBytes, err := readBody(resp, maxNodeResponseBytes)
	if err != nil {
		return &PeerConfigFetchResponse{OK: false, StatusCode: resp.StatusCode}, unreachable(err)
	}

	var data PeerConfigResponse
	if err := json.Unmarshal(bodyBytes, &data); err != nil {
		return &PeerConfigFetchResponse{OK: false, StatusCode: resp.StatusCode}, err
	}

	return &PeerConfigFetchResponse{
		OK:         true,
		StatusCode: resp.StatusCode,
		Data:       &data,
	}, nil
}

type PeerConfigFetchResponse struct {
	OK         bool
	StatusCode int
	Data       *PeerConfigResponse
	ErrorData  any
}

func (s *DashboardService) DeletePeer(ctx context.Context, id string, peerID string) (*DeletePeerResponse, error) {
	node, err := s.FindNodeByID(ctx, id)
	if err != nil {
		return nil, err
	}

	return s.deletePeerData(ctx, node.Address, string(node.APIKey), peerID)
}

type DeletePeerResponse struct {
	OK         bool
	StatusCode *int
	Endpoint   *string
}

func (s *DashboardService) deletePeerData(ctx context.Context, address string, apiKey string, peerID string) (*DeletePeerResponse, error) {
	endpoint := fmt.Sprintf("/peers/%s", url.PathEscape(peerID))

	client := newHTTPClient(2500 * time.Millisecond)
	req, err := http.NewRequestWithContext(ctx, "DELETE", address+endpoint, nil)
	if err != nil {
		return &DeletePeerResponse{OK: false}, err
	}

	req.Header.Set("X-API-KEY", apiKey)

	resp, err := client.Do(req)
	if err != nil {
		return nil, unreachable(err)
	}
	defer closeResponseBody(resp)

	status := resp.StatusCode
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return &DeletePeerResponse{
			OK:         true,
			StatusCode: &status,
			Endpoint:   &endpoint,
		}, nil
	}

	return &DeletePeerResponse{
		OK:         false,
		StatusCode: &status,
		Endpoint:   &endpoint,
	}, nil
}

type NodePeerListItem struct {
	PeerID          string   `json:"peerId"`
	AllowedIPs      []string `json:"allowedIPs"`
	AddressFamilies []string `json:"addressFamilies,omitempty"`
	PublicKey       string   `json:"publicKey"`
	Active          bool     `json:"active"`
	LastHandshakeAt *string  `json:"lastHandshakeAt"`
	CreatedAt       string   `json:"createdAt"`
	ExpiresAt       *string  `json:"expiresAt,omitempty"`
}

type NodePeerDetail struct {
	NodePeerListItem
	ReceiveBytes  *int64 `json:"receiveBytes,omitempty"`
	TransmitBytes *int64 `json:"transmitBytes,omitempty"`
}

type PeersMeta struct {
	Offset     int  `json:"offset"`
	Limit      int  `json:"limit"`
	TotalItems int  `json:"totalItems"`
	HasPrev    bool `json:"hasPrev"`
	HasNext    bool `json:"hasNext"`
	PrevOffset *int `json:"prevOffset"`
	NextOffset *int `json:"nextOffset"`
}

type PeersListResponse struct {
	Data []NodePeerListItem `json:"data"`
	Meta PeersMeta          `json:"meta"`
}

type PeerDetailResponse struct {
	Peer NodePeerDetail `json:"peer"`
}

func (s *DashboardService) GetNodePeers(ctx context.Context, id string, offset int, limit int) (*PeersListResponse, error) {
	node, err := s.FindNodeByID(ctx, id)
	if err != nil {
		return nil, err
	}

	urlStr := fmt.Sprintf("%s/peers?offset=%d&limit=%d", node.Address, offset, limit)
	raw, err := getNodeJSON[PeersListResponse](ctx, urlStr, string(node.APIKey), 10*time.Second)
	if err != nil {
		return nil, err
	}

	// Drop rows missing required identifiers — the UI keys/labels rows by these,
	// and an empty value would render as a blank, click-into-nothing peer card.
	filtered := raw.Data[:0]
	for _, p := range raw.Data {
		if strings.TrimSpace(p.PeerID) == "" || strings.TrimSpace(p.PublicKey) == "" {
			continue
		}
		filtered = append(filtered, p)
	}
	raw.Data = filtered

	return raw, nil
}

func (s *DashboardService) GetNodePeerDetail(ctx context.Context, id string, peerID string) (*NodePeerDetail, error) {
	node, err := s.FindNodeByID(ctx, id)
	if err != nil {
		return nil, err
	}

	urlStr := node.Address + "/peers/" + url.PathEscape(peerID)
	return getNodeJSON[NodePeerDetail](ctx, urlStr, string(node.APIKey), 5*time.Second)
}

// getNodeJSON GETs a node endpoint and decodes a JSON body into T. Failures
// are an *UpstreamError for non-2xx answers or wrap ErrNodeUnreachable.
func getNodeJSON[T any](ctx context.Context, urlStr string, apiKey string, timeout time.Duration) (*T, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, urlStr, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-API-KEY", apiKey)

	resp, err := newHTTPClient(timeout).Do(req)
	if err != nil {
		return nil, unreachable(err)
	}
	defer closeResponseBody(resp)

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, &UpstreamError{Status: resp.StatusCode}
	}
	body, err := readBody(resp, maxNodeResponseBytes)
	if err != nil {
		return nil, unreachable(err)
	}
	var out T
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, unreachable(fmt.Errorf("decode response: %w", err))
	}
	return &out, nil
}
