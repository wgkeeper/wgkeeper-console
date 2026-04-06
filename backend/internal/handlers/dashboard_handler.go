package handlers

import (
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"wg-keeper-backend/internal/models"
	"wg-keeper-backend/internal/services"

	"github.com/gin-gonic/gin"
)

// NodeItem represents a node in the list
type NodeItem struct {
	ID               string            `json:"id" example:"550e8400-e29b-41d4-a716-446655440000"`
	Name             string            `json:"name" example:"wg-https://192.168.1.1:51821"`
	Address          string            `json:"address" example:"https://192.168.1.1:51821"`
	Status           models.NodeStatus `json:"status" example:"online"`
	Version          *string           `json:"version,omitempty" example:"1.0.0"`
	IsOutdated       bool              `json:"isOutdated" example:"false"`
	LatestVersion    *string           `json:"latestVersion,omitempty" example:"1.1.0"`
	LatestVersionURL *string           `json:"latestVersionUrl,omitempty" example:"https://github.com/wgkeeper/wgkeeper-node/releases/tag/v1.1.0"`
	CreatedAt        string            `json:"createdAt" example:"2024-01-01T00:00:00.000Z"`
	UpdatedAt        string            `json:"updatedAt" example:"2024-01-01T00:00:00.000Z"`
}

// CreateNodeRequest represents create node request
type CreateNodeRequest struct {
	Address string `json:"address" example:"https://192.168.1.1:51821" binding:"required"`
	APIKey  string `json:"apiKey" example:"your-api-key" binding:"required"`
}

// CreateNodeResponse represents create node response
type CreateNodeResponse struct {
	OK bool   `json:"ok" example:"true"`
	ID string `json:"id" example:"550e8400-e29b-41d4-a716-446655440000"`
}

// CheckNodeRequest represents check node request
type CheckNodeRequest struct {
	Address string `json:"address" example:"https://192.168.1.1:51821" binding:"required"`
	APIKey  string `json:"apiKey" example:"your-api-key" binding:"required"`
}

// CheckNodeResponse represents check node response
type CheckNodeResponse struct {
	OK          bool              `json:"ok" example:"true"`
	Status      models.NodeStatus `json:"status" example:"online"`
	Version     *string           `json:"version,omitempty" example:"1.0.0"`
	ServiceName *string           `json:"serviceName,omitempty" example:"wg-keeper"`
}

type DashboardHandler struct {
	service *services.DashboardService
}

func NewDashboardHandler() *DashboardHandler {
	return &DashboardHandler{
		service: services.NewDashboardService(),
	}
}

func parseNodeAddress(raw string) (string, bool) {
	address := strings.TrimRight(strings.TrimSpace(raw), "/")
	u, err := url.Parse(address)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return "", false
	}
	return address, true
}

// GetNodes godoc
// @Summary      Get all nodes
// @Description  Retrieve list of all WireGuard nodes
// @Tags         nodes
// @Security     CookieAuth
// @Success      200  {array}   NodeItem
// @Failure      401  {object}  map[string]interface{}  "Unauthorized"
// @Router       /nodes [get]
func (h *DashboardHandler) GetNodes(c *gin.Context) {
	page, err := strconv.Atoi(c.DefaultQuery("page", "1"))
	if err != nil || page < 1 {
		page = 1
	}

	limit, err := strconv.Atoi(c.DefaultQuery("limit", "12"))
	if err != nil || limit < 1 {
		limit = 12
	}
	if limit > 100 {
		limit = 100
	}

	nodes, total, err := h.service.GetNodes(page, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"ok": false})
		return
	}

	latestRelease, _ := h.service.GetLatestNodeRelease()
	var latestVersion *string
	var latestVersionURL *string
	if latestRelease != nil {
		latestVersion = &latestRelease.Version
		latestVersionURL = &latestRelease.URL
	}

	result := make([]gin.H, len(nodes))
	for i, node := range nodes {
		result[i] = gin.H{
			"id":               node.ID,
			"name":             node.Name,
			"address":          node.Address,
			"status":           node.Status,
			"version":          node.Version,
			"isOutdated":       services.IsVersionOutdated(node.Version, latestRelease),
			"latestVersion":    latestVersion,
			"latestVersionUrl": latestVersionURL,
			"createdAt":        node.CreatedAt.Format("2006-01-02T15:04:05.000Z"),
			"updatedAt":        node.UpdatedAt.Format("2006-01-02T15:04:05.000Z"),
		}
	}

	totalPages := int(math.Ceil(float64(total) / float64(limit)))
	if totalPages < 1 {
		totalPages = 1
	}

	c.JSON(http.StatusOK, gin.H{
		"nodes":      result,
		"total":      total,
		"page":       page,
		"pageSize":   limit,
		"totalPages": totalPages,
	})
}

// CreateNode godoc
// @Summary      Create a new node
// @Description  Add a new WireGuard node to the system
// @Tags         nodes
// @Security     CookieAuth
// @Accept       json
// @Produce      json
// @Param        request  body      CreateNodeRequest  true  "Node information"
// @Success      200      {object}  CreateNodeResponse
// @Failure      400      {object}  map[string]interface{}  "Bad request"
// @Failure      401      {object}  map[string]interface{}  "Unauthorized"
// @Failure      409      {object}  map[string]interface{}  "Node already exists"
// @Router       /nodes [post]
func (h *DashboardHandler) CreateNode(c *gin.Context) {
	var body CreateNodeRequest

	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"ok": false})
		return
	}

	address, ok := parseNodeAddress(body.Address)
	apiKey := strings.TrimSpace(body.APIKey)

	if !ok || apiKey == "" {
		c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": "invalid_input"})
		return
	}

	existingNode, err := h.service.FindNodeByAddress(address)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"ok": false})
		return
	}
	if existingNode != nil {
		c.JSON(http.StatusConflict, gin.H{"ok": false, "error": "node_exists"})
		return
	}

	node, err := h.service.CreateNode(struct {
		Address string
		APIKey  string
	}{
		Address: address,
		APIKey:  apiKey,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"ok": false})
		return
	}

	c.JSON(http.StatusOK, gin.H{"ok": true, "id": node.ID})
}

// CheckNode godoc
// @Summary      Check node connection
// @Description  Check if a WireGuard node is accessible and get its status
// @Tags         nodes
// @Security     CookieAuth
// @Accept       json
// @Produce      json
// @Param        request  body      CheckNodeRequest  true  "Node connection info"
// @Success      200      {object}  CheckNodeResponse
// @Failure      400      {object}  map[string]interface{}  "Bad request"
// @Failure      401      {object}  map[string]interface{}  "Unauthorized or invalid API key"
// @Router       /nodes/check [post]
func (h *DashboardHandler) CheckNode(c *gin.Context) {
	var body CheckNodeRequest

	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": "invalid_body"})
		return
	}

	address, ok := parseNodeAddress(body.Address)
	apiKey := strings.TrimSpace(body.APIKey)

	if !ok || apiKey == "" {
		c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": "invalid_input"})
		return
	}

	health, err := h.service.CheckNode(address, apiKey)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "error": "service_error"})
		return
	}

	if health.Error == "invalid_api_key" {
		c.JSON(http.StatusUnauthorized, gin.H{"ok": false, "error": "invalid_api_key"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"ok":          health.Status == models.NodeStatusOnline,
		"status":      health.Status,
		"version":     health.Version,
		"serviceName": health.ServiceName,
	})
}

// GetNodeStats godoc
// @Summary      Get node statistics
// @Description  Retrieve statistics for a specific WireGuard node
// @Tags         nodes
// @Security     CookieAuth
// @Produce      json
// @Param        id   path      string  true  "Node ID"
// @Success      200  {object}  map[string]interface{}  "Node statistics"
// @Failure      401   {object}  map[string]interface{}  "Unauthorized"
// @Failure      502   {object}  map[string]interface{}  "Stats unavailable"
// @Router       /nodes/{id}/stats [get]
func (h *DashboardHandler) GetNodeStats(c *gin.Context) {
	id := strings.TrimSpace(c.Param("id"))
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"ok": false})
		return
	}

	stats, err := h.service.GetNodeStats(id)
	if err != nil || stats == nil || stats.Data == nil {
		statusCode := http.StatusBadGateway
		var statusPtr *int
		var endpointPtr *string
		if stats != nil {
			if stats.StatusCode != nil && *stats.StatusCode >= 400 && *stats.StatusCode < 600 {
				statusCode = *stats.StatusCode
			}
			statusPtr = stats.StatusCode
			endpointPtr = stats.Endpoint
		}
		c.JSON(statusCode, gin.H{
			"ok":       false,
			"error":    "stats_unavailable",
			"status":   statusPtr,
			"endpoint": endpointPtr,
		})
		return
	}

	c.JSON(http.StatusOK, stats.Data)
}

// GetNodeConfig godoc
// @Summary      Get WireGuard config
// @Description  Generate WireGuard configuration file for a peer
// @Tags         nodes
// @Security     CookieAuth
// @Produce      text/plain
// @Param        id              path      string  true   "Node ID"
// @Param        peerId          query     string  true   "Peer ID (UUID v4)"
// @Param        dns             query     string  false  "DNS servers (comma-separated)"
// @Param        expiresAt       query     string  false  "Expiration date (ISO 8601)"
// @Param        addressFamilies query     string  false  "Address families (IPv4,IPv6)"
// @Success      200             {string}  string  "WireGuard config file"
// @Failure      400             {object}  map[string]interface{}  "Bad request"
// @Failure      401             {object}  map[string]interface{}  "Unauthorized"
// @Failure      502             {object}  map[string]interface{}  "Config unavailable"
// @Router       /nodes/{id}/config [get]
func (h *DashboardHandler) GetNodeConfig(c *gin.Context) {
	id := strings.TrimSpace(c.Param("id"))
	peerID := strings.TrimSpace(c.Query("peerId"))
	if id == "" || peerID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"ok": false})
		return
	}

	var dns []string
	if dnsParam := c.Query("dns"); dnsParam != "" {
		parts := strings.Split(dnsParam, ",")
		for _, part := range parts {
			trimmed := strings.TrimSpace(part)
			if trimmed == "" {
				continue
			}
			if len(dns) >= 10 || len(trimmed) > 253 {
				c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": "invalid_dns"})
				return
			}
			dns = append(dns, trimmed)
		}
	}

	var expiresAt *string
	if expiresAtParam := strings.TrimSpace(c.Query("expiresAt")); expiresAtParam != "" {
		if _, err := time.Parse(time.RFC3339, expiresAtParam); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": "invalid_expires_at"})
			return
		}
		expiresAt = &expiresAtParam
	}

	var addressFamilies []string
	if afParam := c.Query("addressFamilies"); afParam != "" {
		parts := strings.Split(afParam, ",")
		allowedFamilies := map[string]bool{"IPv4": true, "IPv6": true}
		for _, part := range parts {
			trimmed := strings.TrimSpace(part)
			if allowedFamilies[trimmed] {
				addressFamilies = append(addressFamilies, trimmed)
			}
		}
	}

	config, err := h.service.GetNodeConfig(id, peerID, dns, expiresAt, addressFamilies)
	if err != nil || config == nil || config.Data == nil {
		statusCode := http.StatusBadGateway
		var statusPtr *int
		var endpointPtr *string
		errorCode := ""
		errorMessage := ""
		if config != nil {
			if config.StatusCode != nil && *config.StatusCode >= 400 && *config.StatusCode < 600 {
				statusCode = *config.StatusCode
			}
			statusPtr = config.StatusCode
			endpointPtr = config.Endpoint
			if config.ErrorData != nil {
				if errMap, ok := config.ErrorData.(map[string]interface{}); ok {
					if code, ok := errMap["code"].(string); ok {
						errorCode = code
					}
					if msg, ok := errMap["error"].(string); ok {
						errorMessage = msg
					}
				}
			}
		}

		response := gin.H{
			"ok":       false,
			"error":    "config_unavailable",
			"status":   statusPtr,
			"endpoint": endpointPtr,
		}
		if errorCode != "" {
			response["errorCode"] = errorCode
		}
		if errorMessage != "" {
			response["errorMessage"] = errorMessage
		}
		c.JSON(statusCode, response)
		return
	}

	c.Data(http.StatusOK, "text/plain", []byte(*config.Data))
}

// GetNodePeers godoc
// @Summary      Get node peers
// @Description  Retrieve paginated list of peers for a node
// @Tags         nodes
// @Security     CookieAuth
// @Produce      json
// @Param        id      path      string  true   "Node ID"
// @Param        offset  query     int     false  "Offset (default: 0)"
// @Param        limit   query     int     false  "Items per page (default: 50, max: 500)"
// @Success      200     {object}  map[string]interface{}  "Peers list with pagination"
// @Failure      401     {object}  map[string]interface{}  "Unauthorized"
// @Failure      502     {object}  map[string]interface{}  "Peers unavailable"
// @Router       /nodes/{id}/peers [get]
func (h *DashboardHandler) GetNodePeers(c *gin.Context) {
	id := strings.TrimSpace(c.Param("id"))
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"ok": false})
		return
	}

	offsetStr := c.DefaultQuery("offset", "0")
	limitStr := c.DefaultQuery("limit", "50")

	offset, err := strconv.Atoi(offsetStr)
	if err != nil || offset < 0 {
		offset = 0
	}

	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit < 1 {
		limit = 50
	}
	if limit > 500 {
		limit = 500
	}

	result, err := h.service.GetNodePeers(id, offset, limit)
	if err != nil || result == nil {
		c.JSON(http.StatusBadGateway, gin.H{"ok": false, "error": "peers_unavailable"})
		return
	}

	c.JSON(http.StatusOK, result)
}

// GetNodePeerDetail godoc
// @Summary      Get peer details
// @Description  Retrieve detailed information about a specific peer
// @Tags         nodes
// @Security     CookieAuth
// @Produce      json
// @Param        id      path      string  true  "Node ID"
// @Param        peerId  path      string  true  "Peer ID"
// @Success      200     {object}  map[string]interface{}  "Peer details"
// @Failure      401     {object}  map[string]interface{}  "Unauthorized"
// @Failure      404     {object}  map[string]interface{}  "Peer not found"
// @Router       /nodes/{id}/peers/{peerId} [get]
func (h *DashboardHandler) GetNodePeerDetail(c *gin.Context) {
	id := strings.TrimSpace(c.Param("id"))
	peerID := strings.TrimSpace(c.Param("peerId"))
	if id == "" || peerID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"ok": false})
		return
	}

	peer, err := h.service.GetNodePeerDetail(id, peerID)
	if err != nil || peer == nil {
		c.JSON(http.StatusNotFound, gin.H{"ok": false, "error": "peer_not_found"})
		return
	}

	c.JSON(http.StatusOK, peer)
}

// DeletePeer godoc
// @Summary      Delete a peer
// @Description  Remove a peer from a WireGuard node
// @Tags         nodes
// @Security     CookieAuth
// @Produce      json
// @Param        id      path      string  true  "Node ID"
// @Param        peerId  query     string  true  "Peer ID"
// @Success      200     {object}  map[string]interface{}  "Delete result"
// @Failure      401     {object}  map[string]interface{}  "Unauthorized"
// @Failure      502     {object}  map[string]interface{}  "Delete failed"
// @Router       /nodes/{id}/peers [delete]
func (h *DashboardHandler) DeletePeer(c *gin.Context) {
	id := strings.TrimSpace(c.Param("id"))
	peerID := strings.TrimSpace(c.Query("peerId"))
	if id == "" || peerID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"ok": false})
		return
	}

	result, err := h.service.DeletePeer(id, peerID)
	if err != nil || result == nil {
		c.JSON(http.StatusBadGateway, gin.H{"ok": false, "error": "peer_delete_failed"})
		return
	}

	if !result.OK {
		if result.StatusCode != nil && *result.StatusCode == 404 {
			c.JSON(http.StatusOK, gin.H{"ok": true, "deleted": false, "notFound": true})
			return
		}
		c.JSON(http.StatusBadGateway, gin.H{
			"ok":       false,
			"error":    "peer_delete_failed",
			"status":   result.StatusCode,
			"endpoint": result.Endpoint,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{"ok": true, "deleted": true})
}

// DeleteNode godoc
// @Summary      Delete a node
// @Description  Remove a WireGuard node from the system
// @Tags         nodes
// @Security     CookieAuth
// @Produce      json
// @Param        id   path      string  true  "Node ID"
// @Success      200  {object}  map[string]interface{}  "Delete result"
// @Failure      401  {object}  map[string]interface{}  "Unauthorized"
// @Failure      404  {object}  map[string]interface{}  "Node not found"
// @Router       /nodes/{id} [delete]
func (h *DashboardHandler) DeleteNode(c *gin.Context) {
	id := strings.TrimSpace(c.Param("id"))
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"ok": false})
		return
	}

	deleted, err := h.service.DeleteNode(id)
	if err != nil || deleted == nil {
		c.JSON(http.StatusNotFound, gin.H{"ok": false, "error": "node_not_found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// RefreshNodes godoc
// @Summary      Refresh node statuses
// @Description  Check and update status for all nodes
// @Tags         nodes
// @Security     CookieAuth
// @Produce      json
// @Success      200  {object}  map[string]interface{}  "Refresh result"
// @Failure      401  {object}  map[string]interface{}  "Unauthorized"
// @Router       /nodes/refresh [post]
func (h *DashboardHandler) RefreshNodes(c *gin.Context) {
	if err := h.service.RefreshStatuses(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"ok": false})
		return
	}

	c.JSON(http.StatusOK, gin.H{"ok": true})
}
