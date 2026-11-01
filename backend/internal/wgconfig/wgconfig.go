// Package wgconfig renders WireGuard client configuration files.
package wgconfig

import (
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
)

// DefaultDNS is used when the admin picks no DNS servers: Cloudflare over
// IPv4 and IPv6. Servers of a family the peer has no address in are dropped.
var DefaultDNS = []string{"1.1.1.1", "1.0.0.1", "2606:4700:4700::1111", "2606:4700:4700::1001"}

// ErrIncomplete means the node's answer lacked a field a working config needs.
var ErrIncomplete = errors.New("incomplete peer data")

// Params is what a client config is built from.
type Params struct {
	PrivateKey      string
	PresharedKey    string
	Addresses       []string // the peer's tunnel addresses, e.g. 10.0.0.2/32, fd00::2/128
	ServerPublicKey string
	Host            string // node host as the client should dial it; IPv6 without brackets
	ListenPort      int
	DNS             []string // nil or empty means DefaultDNS
}

// Render builds the client config. Traffic is routed through the tunnel for
// every address family the peer has an address in; DNS servers of other
// families are dropped. It returns ErrIncomplete if a required field is empty.
func Render(p Params) (string, error) {
	privateKey := strings.TrimSpace(p.PrivateKey)
	presharedKey := strings.TrimSpace(p.PresharedKey)
	serverPublicKey := strings.TrimSpace(p.ServerPublicKey)

	var addresses []string
	hasV4, hasV6 := false, false
	for _, a := range p.Addresses {
		a = strings.TrimSpace(a)
		if a == "" {
			continue
		}
		addresses = append(addresses, a)
		ip, _, _ := strings.Cut(a, "/")
		if strings.Contains(ip, ":") {
			hasV6 = true
		} else if strings.Contains(ip, ".") {
			hasV4 = true
		}
	}
	if privateKey == "" || presharedKey == "" || serverPublicKey == "" || len(addresses) == 0 ||
		p.Host == "" || p.ListenPort <= 0 {
		return "", ErrIncomplete
	}
	// Defensive: if family detection failed, route both.
	if !hasV4 && !hasV6 {
		hasV4, hasV6 = true, true
	}

	dns := p.DNS
	if len(dns) == 0 {
		dns = DefaultDNS
	}
	var effectiveDNS []string
	for _, d := range dns {
		if strings.Contains(d, ":") {
			if hasV6 {
				effectiveDNS = append(effectiveDNS, d)
			}
		} else if hasV4 {
			effectiveDNS = append(effectiveDNS, d)
		}
	}

	var routes []string
	if hasV4 {
		routes = append(routes, "0.0.0.0/0")
	}
	if hasV6 {
		routes = append(routes, "::/0")
	}

	return fmt.Sprintf(`[Interface]
PrivateKey = %s
Address = %s
DNS = %s

[Peer]
PublicKey = %s
PresharedKey = %s
AllowedIPs = %s
Endpoint = %s
PersistentKeepalive = 25

`, privateKey, strings.Join(addresses, ", "), strings.Join(effectiveDNS, ", "),
		serverPublicKey, presharedKey, strings.Join(routes, ", "),
		// JoinHostPort brackets IPv6 hosts: [fd00::1]:51820.
		net.JoinHostPort(p.Host, strconv.Itoa(p.ListenPort))), nil
}
