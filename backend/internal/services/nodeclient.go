// The HTTP client the console uses to talk to wgkeeper nodes.

package services

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"slices"
	"strings"
	"syscall"
	"time"
)

// ErrBlockedAddress is returned when a node address resolves to a link-local
// or cloud metadata address, which no wgkeeper node listens on.
var ErrBlockedAddress = errors.New("address not allowed for nodes")

// metadataAddrs are cloud metadata endpoints outside the link-local ranges:
// AWS over IPv6 and Alibaba Cloud.
var metadataAddrs = []netip.Addr{
	netip.MustParseAddr("fd00:ec2::254"),
	netip.MustParseAddr("100.100.100.200"),
}

// IsBlockedNodeAddr reports whether the console refuses to connect to addr:
// link-local (169.254.0.0/16, which holds the usual cloud metadata endpoint,
// and fe80::/10), multicast, unspecified, and the metadata addresses above.
// Loopback and private ranges stay allowed: nodes run on them.
func IsBlockedNodeAddr(addr netip.Addr) bool {
	addr = addr.Unmap()
	return addr.IsLinkLocalUnicast() || addr.IsMulticast() || addr.IsUnspecified() ||
		slices.Contains(metadataAddrs, addr)
}

// refuseBlockedAddress runs after DNS resolution, right before each connect,
// so a hostname that resolves (or is rebound) to a blocked address is caught
// too. With an outbound proxy configured this checks the proxy's address; the
// proxy's own policy then governs where requests go.
func refuseBlockedAddress(_, address string, _ syscall.RawConn) error {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return fmt.Errorf("%w: %s", ErrBlockedAddress, address)
	}
	addr, err := netip.ParseAddr(host)
	if err != nil || IsBlockedNodeAddr(addr) {
		return fmt.Errorf("%w: %s", ErrBlockedAddress, host)
	}
	return nil
}

var sharedHTTPTransport = &http.Transport{
	Proxy: http.ProxyFromEnvironment,
	DialContext: (&net.Dialer{
		Timeout:   3 * time.Second,
		KeepAlive: 30 * time.Second,
		Control:   refuseBlockedAddress,
	}).DialContext,
	ForceAttemptHTTP2:     true,
	MaxIdleConns:          100,
	MaxIdleConnsPerHost:   20,
	IdleConnTimeout:       90 * time.Second,
	TLSHandshakeTimeout:   3 * time.Second,
	ExpectContinueTimeout: time.Second,
}

var sharedHTTPRoundTripper http.RoundTripper = sharedHTTPTransport

// WithTransport makes the service send node requests through rt, for example
// a metrics-instrumented wrapper around the shared transport.
func (s *DashboardService) WithTransport(rt http.RoundTripper) *DashboardService {
	s.transport = rt
	return s
}

// SharedTransport is the default transport for node requests: pooled
// connections, short dial and TLS timeouts.
func SharedTransport() http.RoundTripper {
	return sharedHTTPRoundTripper
}

func (s *DashboardService) httpClient(timeout time.Duration) *http.Client {
	rt := s.transport
	if rt == nil {
		rt = sharedHTTPRoundTripper
	}
	return newHTTPClient(timeout, rt)
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

func newHTTPClient(timeout time.Duration, rt http.RoundTripper) *http.Client {
	return &http.Client{
		Timeout:   timeout,
		Transport: rt,
		// Never follow redirects: Go would resend X-API-KEY to the new host
		// (it only strips Authorization and Cookie), and a 307 would replay
		// a POST with peer data to wherever the node points.
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}

type FetchResponse struct {
	OK         bool
	StatusCode int
	Data       *StatsData
	ErrorData  any
}

func (s *DashboardService) fetchWithTimeout(ctx context.Context, urlStr string, apiKey string, timeout time.Duration, method string, body []byte, contentType string) (*FetchResponse, error) {
	client := s.httpClient(timeout)
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
	client := s.httpClient(timeout)
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

func deref(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

func derefInt(p *int) int {
	if p == nil {
		return 0
	}
	return *p
}

func (s *DashboardService) fetchPeerConfig(ctx context.Context, urlStr string, apiKey string, timeout time.Duration, body []byte) (*PeerConfigFetchResponse, error) {
	client := s.httpClient(timeout)
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

func (s *DashboardService) deletePeerData(ctx context.Context, address string, apiKey string, peerID string) (*DeletePeerResponse, error) {
	endpoint := fmt.Sprintf("/peers/%s", url.PathEscape(peerID))

	client := s.httpClient(2500 * time.Millisecond)
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

// getNodeJSON GETs a node endpoint and decodes a JSON body into T. Failures
// are an *UpstreamError for non-2xx answers or wrap ErrNodeUnreachable.
func getNodeJSON[T any](ctx context.Context, client *http.Client, urlStr string, apiKey string) (*T, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, urlStr, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-API-KEY", apiKey)

	resp, err := client.Do(req)
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
