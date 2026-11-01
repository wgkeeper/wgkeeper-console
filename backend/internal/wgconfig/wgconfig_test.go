package wgconfig

import (
	"errors"
	"strings"
	"testing"
)

func base() Params {
	return Params{
		PrivateKey:      "priv",
		PresharedKey:    "psk",
		Addresses:       []string{"10.0.0.2/32", "fd00::2/128"},
		ServerPublicKey: "spk",
		Host:            "vpn.example.com",
		ListenPort:      51820,
	}
}

func TestRender(t *testing.T) {
	tests := []struct {
		name     string
		edit     func(*Params)
		contains []string
		absent   []string
	}{
		{"dual stack", func(*Params) {},
			[]string{"Address = 10.0.0.2/32, fd00::2/128", "AllowedIPs = 0.0.0.0/0, ::/0", "DNS = 1.1.1.1, 1.0.0.1, 2606:4700:4700::1111, 2606:4700:4700::1001", "Endpoint = vpn.example.com:51820\n", "PersistentKeepalive = 25"}, nil},
		{"IPv4 only drops IPv6 routes and DNS", func(p *Params) { p.Addresses = []string{"10.0.0.2/32"} },
			[]string{"AllowedIPs = 0.0.0.0/0\n", "DNS = 1.1.1.1, 1.0.0.1\n"}, []string{"::/0", "2606:"}},
		{"IPv6 only drops IPv4 routes and DNS", func(p *Params) { p.Addresses = []string{"fd00::2/128"} },
			[]string{"AllowedIPs = ::/0\n", "DNS = 2606:4700:4700::1111, 2606:4700:4700::1001\n"}, []string{"0.0.0.0/0", "1.1.1.1"}},
		{"custom DNS filtered by family", func(p *Params) {
			p.Addresses = []string{"10.0.0.2/32"}
			p.DNS = []string{"9.9.9.9", "2620:fe::fe"}
		}, []string{"DNS = 9.9.9.9\n"}, []string{"2620:fe::fe"}},
		{"IPv6 node host is bracketed", func(p *Params) { p.Host = "fd00::1" },
			[]string{"Endpoint = [fd00::1]:51820\n"}, nil},
		{"whitespace and empty addresses are cleaned", func(p *Params) {
			p.PrivateKey, p.Addresses = "  priv  ", []string{" 10.0.0.2/32 ", "", "  "}
		}, []string{"PrivateKey = priv\n", "Address = 10.0.0.2/32\n"}, nil},
		{"unrecognised addresses route both families", func(p *Params) { p.Addresses = []string{"peer-address"} },
			[]string{"AllowedIPs = 0.0.0.0/0, ::/0"}, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := base()
			tt.edit(&p)
			got, err := Render(p)
			if err != nil {
				t.Fatalf("Render() error = %v", err)
			}
			for _, want := range tt.contains {
				if !strings.Contains(got, want) {
					t.Errorf("config lacks %q:\n%s", want, got)
				}
			}
			for _, bad := range tt.absent {
				if strings.Contains(got, bad) {
					t.Errorf("config contains %q:\n%s", bad, got)
				}
			}
		})
	}
}

func TestRenderIncomplete(t *testing.T) {
	for name, edit := range map[string]func(*Params){
		"no private key":   func(p *Params) { p.PrivateKey = " " },
		"no preshared key": func(p *Params) { p.PresharedKey = "" },
		"no server key":    func(p *Params) { p.ServerPublicKey = "" },
		"no addresses":     func(p *Params) { p.Addresses = []string{"", " "} },
		"no host":          func(p *Params) { p.Host = "" },
		"no port":          func(p *Params) { p.ListenPort = 0 },
	} {
		t.Run(name, func(t *testing.T) {
			p := base()
			edit(&p)
			if _, err := Render(p); !errors.Is(err, ErrIncomplete) {
				t.Fatalf("Render() error = %v, want ErrIncomplete", err)
			}
		})
	}
}

// FuzzRender checks that Render never panics and that a rendered config
// always names the endpoint it was given, with IPv6 hosts bracketed.
func FuzzRender(f *testing.F) {
	f.Add("vpn.example.com", 51820, "10.0.0.2/32,fd00::2/128")
	f.Add("fd00::1", 1, "fd00::2/128")
	f.Add("", 0, "")
	f.Fuzz(func(t *testing.T, host string, port int, addresses string) {
		p := base()
		p.Host, p.ListenPort, p.Addresses = host, port, strings.Split(addresses, ",")
		got, err := Render(p)
		if err != nil {
			return
		}
		if strings.Contains(host, ":") && !strings.Contains(got, "Endpoint = ["+host+"]:") {
			t.Fatalf("IPv6 host %q not bracketed:\n%s", host, got)
		}
	})
}
