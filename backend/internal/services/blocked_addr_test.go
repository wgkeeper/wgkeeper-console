package services

import (
	"errors"
	"net/netip"
	"testing"
)

func TestIsBlockedNodeAddr(t *testing.T) {
	tests := []struct {
		addr    string
		blocked bool
	}{
		{"169.254.169.254", true}, // AWS, GCP and Azure metadata
		{"169.254.0.1", true},     // link-local
		{"fe80::1", true},         // IPv6 link-local
		{"fd00:ec2::254", true},   // AWS metadata over IPv6
		{"100.100.100.200", true}, // Alibaba Cloud metadata
		{"::ffff:169.254.169.254", true},
		{"0.0.0.0", true},
		{"::", true},
		{"224.0.0.1", true},
		{"127.0.0.1", false}, // console and node on one host
		{"::1", false},
		{"10.0.0.5", false},
		{"192.168.1.10", false},
		{"100.64.0.7", false}, // CGNAT and Tailscale
		{"203.0.113.10", false},
		{"2001:db8::10", false},
	}
	for _, tt := range tests {
		if got := IsBlockedNodeAddr(netip.MustParseAddr(tt.addr)); got != tt.blocked {
			t.Errorf("IsBlockedNodeAddr(%s) = %v, want %v", tt.addr, got, tt.blocked)
		}
	}
}

func TestSharedTransportRefusesBlockedAddresses(t *testing.T) {
	for _, address := range []string{"169.254.169.254:80", "[fd00:ec2::254]:80", "[fe80::1]:51821"} {
		conn, err := sharedHTTPTransport.DialContext(t.Context(), "tcp", address)
		if conn != nil {
			_ = conn.Close()
		}
		if !errors.Is(err, ErrBlockedAddress) {
			t.Errorf("dial %s: err = %v, want ErrBlockedAddress", address, err)
		}
	}
}
