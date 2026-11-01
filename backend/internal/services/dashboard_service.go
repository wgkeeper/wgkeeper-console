package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"wg-keeper-backend/internal/database"
	"wg-keeper-backend/internal/models"
	"wg-keeper-backend/internal/wgconfig"

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
	// transport carries every request to nodes and GitHub; nil means the
	// shared transport.
	transport http.RoundTripper
}

// CountNodesByStatus returns how many nodes are stored per status.
func (s *DashboardService) CountNodesByStatus() (map[string]int, error) {
	var rows []struct {
		Status string
		Count  int
	}
	if err := s.db.Model(&models.Node{}).Select("status, count(*) as count").Group("status").Scan(&rows).Error; err != nil {
		return nil, err
	}
	counts := make(map[string]int, len(rows))
	for _, r := range rows {
		counts[r.Status] = r.Count
	}
	return counts, nil
}

func NewDashboardService() *DashboardService {
	return &DashboardService{
		db:               database.DB,
		latestReleaseTTL: 10 * time.Minute,
	}
}

func stripTrailingSlash(address string) string {
	return strings.TrimRight(address, "/")
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
				return &ConfigResponse{StatusCode: &status, Endpoint: &endpoint}, nil
			}
			config, err := wgconfig.Render(wgconfig.Params{
				PrivateKey:      deref(peer.PrivateKey),
				PresharedKey:    deref(peer.PresharedKey),
				Addresses:       peer.AllowedIPs,
				ServerPublicKey: deref(server.PublicKey),
				Host:            host,
				ListenPort:      derefInt(server.ListenPort),
				DNS:             dns,
			})
			if err != nil {
				// The node answered 200 without a usable peer: the handler
				// reports incomplete_peer_response.
				return &ConfigResponse{StatusCode: &status, Endpoint: &endpoint}, nil
			}
			return &ConfigResponse{Data: &config, StatusCode: &status, Endpoint: &endpoint}, nil
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
	raw, err := getNodeJSON[PeersListResponse](ctx, s.httpClient(10*time.Second), urlStr, string(node.APIKey))
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
	return getNodeJSON[NodePeerDetail](ctx, s.httpClient(5*time.Second), urlStr, string(node.APIKey))
}
