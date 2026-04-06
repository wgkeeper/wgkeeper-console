package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
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
	"gorm.io/gorm"
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

func closeResponseBody(resp *http.Response) {
	if resp == nil || resp.Body == nil {
		return
	}
	if err := resp.Body.Close(); err != nil {
		slog.Warn("failed to close response body", "err", err)
	}
}

func parseJSONErrorBody(body []byte) interface{} {
	if len(body) == 0 {
		return nil
	}

	var errorData interface{}
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
	}
}

func (s *DashboardService) GetNodes(page, limit int) ([]models.Node, int64, error) {
	var nodes []models.Node
	var total int64

	if err := s.db.Model(&models.Node{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * limit
	err := s.db.Order("created_at DESC").Offset(offset).Limit(limit).Find(&nodes).Error
	return nodes, total, err
}

func (s *DashboardService) GetLatestNodeRelease() (*LatestNodeRelease, error) {
	mocked := previewLatestNodeReleaseOverride()
	if mocked != nil {
		cloned := *mocked
		return &cloned, nil
	}

	s.releaseCacheMu.Lock()
	if s.latestRelease != nil && time.Since(s.latestReleaseAt) < s.latestReleaseTTL {
		release := *s.latestRelease
		s.releaseCacheMu.Unlock()
		return &release, nil
	}
	s.releaseCacheMu.Unlock()

	release, err := s.fetchLatestNodeRelease()
	if err != nil {
		s.releaseCacheMu.Lock()
		defer s.releaseCacheMu.Unlock()
		if s.latestRelease != nil {
			cached := *s.latestRelease
			return &cached, nil
		}
		return nil, err
	}

	s.releaseCacheMu.Lock()
	s.latestRelease = release
	s.latestReleaseAt = time.Now()
	s.releaseCacheMu.Unlock()

	cloned := *release
	return &cloned, nil
}

func (s *DashboardService) fetchLatestNodeRelease() (*LatestNodeRelease, error) {
	const releasesLatestURL = "https://api.github.com/repos/wgkeeper/wgkeeper-node/releases/latest"

	client := newHTTPClient(2500 * time.Millisecond)
	req, err := http.NewRequest(http.MethodGet, releasesLatestURL, nil)
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
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
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

func (s *DashboardService) FindNodeByID(id string) (*models.Node, error) {
	var node models.Node
	err := s.db.Where("id = ?", id).First(&node).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	return &node, err
}

func (s *DashboardService) FindNodeByAddress(address string) (*models.Node, error) {
	var node models.Node
	err := s.db.Where("address = ?", address).First(&node).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	return &node, err
}

func (s *DashboardService) CreateNode(input struct {
	Address string
	APIKey  string
}) (*models.Node, error) {
	address := stripTrailingSlash(input.Address)
	name := fmt.Sprintf("wg-%s", address)

	health, err := s.checkHealth(address, input.APIKey)
	if err != nil {
		return nil, err
	}

	node := models.Node{
		ID:      uuid.New().String(),
		Name:    name,
		Address: address,
		APIKey:  input.APIKey,
		Status:  health.Status,
		Version: health.Version,
	}

	err = s.db.Create(&node).Error
	if err != nil {
		return nil, err
	}

	return &node, nil
}

func (s *DashboardService) DeleteNode(id string) (*models.Node, error) {
	node, err := s.FindNodeByID(id)
	if err != nil || node == nil {
		return nil, err
	}
	err = s.db.Delete(node).Error
	return node, err
}

func (s *DashboardService) RefreshStatuses() error {
	var nodes []models.Node
	if err := s.db.Find(&nodes).Error; err != nil {
		return err
	}

	const maxConcurrent = 10
	type result struct {
		node   *models.Node
		health *HealthResult
		err    error
	}
	results := make(chan result, len(nodes))
	sem := make(chan struct{}, maxConcurrent)

	for _, node := range nodes {
		sem <- struct{}{}
		go func(n models.Node) {
			defer func() { <-sem }()
			health, err := s.checkHealth(n.Address, n.APIKey)
			results <- result{node: &n, health: health, err: err}
		}(node)
	}

	for i := 0; i < len(nodes); i++ {
		res := <-results
		if res.err == nil && res.health != nil {
			updates := map[string]interface{}{
				"status": res.health.Status,
			}
			if res.health.Version != nil {
				updates["version"] = res.health.Version
			}
			if err := s.db.Model(&models.Node{}).Where("id = ?", res.node.ID).Updates(updates).Error; err != nil {
				slog.Error("failed to update node status", "node_id", res.node.ID, "err", err)
			}
		}
	}

	return nil
}

func (s *DashboardService) GetNodeStats(id string) (*StatsResponse, error) {
	node, err := s.FindNodeByID(id)
	if err != nil || node == nil {
		return nil, err
	}

	return s.fetchStatsData(node.Address, node.APIKey)
}

func (s *DashboardService) CheckNode(address string, apiKey string) (*HealthResult, error) {
	return s.checkHealth(stripTrailingSlash(address), apiKey)
}

type HealthResult struct {
	Status      models.NodeStatus
	Version     *string
	ServiceName *string
	Error       string
}

func (s *DashboardService) checkHealth(address string, apiKey string) (*HealthResult, error) {
	// Check health endpoint
	healthResp, err := s.fetchWithTimeout(address+"/readyz", apiKey, 2*time.Second, "GET", nil, "")
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
	statsResp, err := s.fetchStatsData(address, apiKey)
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

func (s *DashboardService) fetchStatsData(address string, apiKey string) (*StatsResponse, error) {
	endpoints := []string{"/stats", "/stats/"}

	var lastStatus *int
	var lastEndpoint *string

	for _, endpoint := range endpoints {
		resp, err := s.fetchJSONWithTimeout(address+endpoint, apiKey, 2500*time.Millisecond, true, "GET", nil, "")
		if err != nil {
			continue
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

		if status == 404 {
			continue
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
	ErrorData  interface{}
}

func (s *DashboardService) fetchWithTimeout(urlStr string, apiKey string, timeout time.Duration, method string, body []byte, contentType string) (*FetchResponse, error) {
	client := newHTTPClient(timeout)
	req, err := http.NewRequest(method, urlStr, bytes.NewReader(body))
	if err != nil {
		return &FetchResponse{OK: false}, err
	}

	req.Header.Set("X-API-KEY", apiKey)
	req.Header.Set("X_API_KEY", apiKey)
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

func (s *DashboardService) fetchJSONWithTimeout(urlStr string, apiKey string, timeout time.Duration, allowNonJSON bool, method string, body []byte, contentType string) (*FetchResponse, error) {
	client := newHTTPClient(timeout)
	req, err := http.NewRequest(method, urlStr, bytes.NewReader(body))
	if err != nil {
		return &FetchResponse{OK: false}, err
	}

	req.Header.Set("X-API-KEY", apiKey)
	req.Header.Set("X_API_KEY", apiKey)
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}

	resp, err := client.Do(req)
	if err != nil {
		return &FetchResponse{OK: false}, err
	}
	defer closeResponseBody(resp)

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		bodyBytes, _ := io.ReadAll(resp.Body)
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

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return &FetchResponse{OK: false, StatusCode: resp.StatusCode}, err
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

func (s *DashboardService) GetNodeConfig(id string, peerID string, dns []string, expiresAt *string, addressFamilies []string) (*ConfigResponse, error) {
	node, err := s.FindNodeByID(id)
	if err != nil || node == nil {
		return nil, err
	}

	return s.fetchConfigData(node.Address, node.APIKey, peerID, dns, expiresAt, addressFamilies)
}

type ConfigResponse struct {
	Data       *string
	StatusCode *int
	Endpoint   *string
	ErrorData  interface{}
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

func (s *DashboardService) fetchConfigData(address string, apiKey string, peerID string, dns []string, expiresAt *string, addressFamilies []string) (*ConfigResponse, error) {
	parsedURL, _ := url.Parse(address)
	host := parsedURL.Hostname()

	endpoints := []string{"/peers"}

	var lastStatus *int
	var lastEndpoint *string
	var lastErrorData interface{}

	bodyObj := map[string]interface{}{
		"peerId": peerID,
	}
	if expiresAt != nil && *expiresAt != "" {
		bodyObj["expiresAt"] = *expiresAt
	}
	if len(addressFamilies) > 0 {
		bodyObj["addressFamilies"] = addressFamilies
	}

	bodyBytes, _ := json.Marshal(bodyObj)

	for _, endpoint := range endpoints {
		resp, err := s.fetchPeerConfig(address+endpoint, apiKey, 2500*time.Millisecond, bodyBytes)
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
			if len(allowedIPs) > 0 {
				var filtered []string
				for _, ip := range allowedIPs {
					trimmed := strings.TrimSpace(ip)
					if trimmed != "" {
						filtered = append(filtered, trimmed)
					}
				}
				allowedIP = strings.Join(filtered, ", ")
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

			effectiveDNS := dns
			if len(effectiveDNS) == 0 {
				effectiveDNS = []string{"1.1.1.1", "1.0.0.1", "2606:4700:4700::1111", "2606:4700:4700::1001"}
			}

			config := fmt.Sprintf(`[Interface]
PrivateKey = %s
Address = %s
DNS = %s

[Peer]
PublicKey = %s
PresharedKey = %s
AllowedIPs = 0.0.0.0/0, ::/0
Endpoint = %s:%d
PersistentKeepalive = 25

`, privateKey, allowedIP, strings.Join(effectiveDNS, ", "), serverPublicKey, presharedKey, host, listenPort)

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

func (s *DashboardService) fetchPeerConfig(urlStr string, apiKey string, timeout time.Duration, body []byte) (*PeerConfigFetchResponse, error) {
	client := newHTTPClient(timeout)
	req, err := http.NewRequest("POST", urlStr, bytes.NewReader(body))
	if err != nil {
		return &PeerConfigFetchResponse{OK: false}, err
	}

	req.Header.Set("X-API-KEY", apiKey)
	req.Header.Set("X_API_KEY", apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return &PeerConfigFetchResponse{OK: false}, err
	}
	defer closeResponseBody(resp)

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return &PeerConfigFetchResponse{OK: false, StatusCode: resp.StatusCode}, err
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return &PeerConfigFetchResponse{
			OK:         false,
			StatusCode: resp.StatusCode,
			ErrorData:  parseJSONErrorBody(bodyBytes),
		}, nil
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
	ErrorData  interface{}
}

func (s *DashboardService) DeletePeer(id string, peerID string) (*DeletePeerResponse, error) {
	node, err := s.FindNodeByID(id)
	if err != nil || node == nil {
		return nil, err
	}

	return s.deletePeerData(node.Address, node.APIKey, peerID)
}

type DeletePeerResponse struct {
	OK         bool
	StatusCode *int
	Endpoint   *string
}

func (s *DashboardService) deletePeerData(address string, apiKey string, peerID string) (*DeletePeerResponse, error) {
	endpoint := fmt.Sprintf("/peers/%s", url.PathEscape(peerID))

	client := newHTTPClient(2500 * time.Millisecond)
	req, err := http.NewRequest("DELETE", address+endpoint, nil)
	if err != nil {
		return &DeletePeerResponse{OK: false}, err
	}

	req.Header.Set("X-API-KEY", apiKey)
	req.Header.Set("X_API_KEY", apiKey)

	resp, err := client.Do(req)
	if err != nil {
		return &DeletePeerResponse{OK: false}, err
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

func (s *DashboardService) GetNodePeers(id string, offset int, limit int) (*PeersListResponse, error) {
	node, err := s.FindNodeByID(id)
	if err != nil || node == nil {
		return nil, err
	}

	raw, err := s.fetchPeersList(node.Address, node.APIKey, offset, limit)
	if err != nil || raw == nil {
		return &PeersListResponse{
			Data: []NodePeerListItem{},
			Meta: PeersMeta{Offset: offset, Limit: limit},
		}, nil
	}

	return raw, nil
}

func (s *DashboardService) fetchPeersList(address string, apiKey string, offset int, limit int) (*PeersListResponse, error) {
	url := fmt.Sprintf("%s/peers?offset=%d&limit=%d", address, offset, limit)
	resp, err := s.fetchPeersListJSON(url, apiKey, 10*time.Second)
	if err != nil || !resp.OK || resp.Data == nil {
		return nil, err
	}
	return resp.Data, nil
}

func (s *DashboardService) fetchPeersListJSON(urlStr string, apiKey string, timeout time.Duration) (*PeersListFetchResponse, error) {
	client := newHTTPClient(timeout)
	req, err := http.NewRequest("GET", urlStr, nil)
	if err != nil {
		return &PeersListFetchResponse{OK: false}, err
	}

	req.Header.Set("X-API-KEY", apiKey)
	req.Header.Set("X_API_KEY", apiKey)

	resp, err := client.Do(req)
	if err != nil {
		return &PeersListFetchResponse{OK: false}, err
	}
	defer closeResponseBody(resp)

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return &PeersListFetchResponse{OK: false, StatusCode: resp.StatusCode}, err
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return &PeersListFetchResponse{
			OK:         false,
			StatusCode: resp.StatusCode,
		}, nil
	}

	var data PeersListResponse
	if err := json.Unmarshal(bodyBytes, &data); err != nil {
		return &PeersListFetchResponse{OK: false, StatusCode: resp.StatusCode}, err
	}

	return &PeersListFetchResponse{
		OK:         true,
		StatusCode: resp.StatusCode,
		Data:       &data,
	}, nil
}

type PeersListFetchResponse struct {
	OK         bool
	StatusCode int
	Data       *PeersListResponse
}

func (s *DashboardService) GetNodePeerDetail(id string, peerID string) (*NodePeerDetail, error) {
	node, err := s.FindNodeByID(id)
	if err != nil || node == nil {
		return nil, err
	}

	return s.fetchPeerDetail(node.Address, node.APIKey, peerID)
}

func (s *DashboardService) fetchPeerDetail(address string, apiKey string, peerID string) (*NodePeerDetail, error) {
	path := fmt.Sprintf("/peers/%s", url.PathEscape(peerID))
	resp, err := s.fetchPeerDetailJSON(address+path, apiKey, 5*time.Second)
	if err != nil || !resp.OK || resp.Data == nil {
		return nil, err
	}
	return resp.Data, nil
}

func (s *DashboardService) fetchPeerDetailJSON(urlStr string, apiKey string, timeout time.Duration) (*PeerDetailFetchResponse, error) {
	client := newHTTPClient(timeout)
	req, err := http.NewRequest("GET", urlStr, nil)
	if err != nil {
		return &PeerDetailFetchResponse{OK: false}, err
	}

	req.Header.Set("X-API-KEY", apiKey)
	req.Header.Set("X_API_KEY", apiKey)

	resp, err := client.Do(req)
	if err != nil {
		return &PeerDetailFetchResponse{OK: false}, err
	}
	defer closeResponseBody(resp)

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return &PeerDetailFetchResponse{OK: false, StatusCode: resp.StatusCode}, err
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return &PeerDetailFetchResponse{
			OK:         false,
			StatusCode: resp.StatusCode,
		}, nil
	}

	var data NodePeerDetail
	if err := json.Unmarshal(bodyBytes, &data); err != nil {
		return &PeerDetailFetchResponse{OK: false, StatusCode: resp.StatusCode}, err
	}

	return &PeerDetailFetchResponse{
		OK:         true,
		StatusCode: resp.StatusCode,
		Data:       &data,
	}, nil
}

type PeerDetailFetchResponse struct {
	OK         bool
	StatusCode int
	Data       *NodePeerDetail
}
