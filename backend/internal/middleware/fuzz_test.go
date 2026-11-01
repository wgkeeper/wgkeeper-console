package middleware

import (
	"net/netip"
	"testing"
)

// FuzzClientKey checks that the rate-limit bucket never panics, that an IPv4
// address and its IPv4-mapped IPv6 form share a bucket, and that every address
// in an IPv6 /64 lands in the same bucket.
func FuzzClientKey(f *testing.F) {
	for _, seed := range []string{"203.0.113.7", "::ffff:203.0.113.7", "2001:db8:1:2::1", "fe80::1%eth0", "", "garbage", "::"} {
		f.Add(seed, uint64(0))
	}
	f.Fuzz(func(t *testing.T, ip string, host uint64) {
		key := clientKey(ip)
		addr, err := netip.ParseAddr(ip)
		if err != nil {
			return
		}
		addr = addr.Unmap()
		if addr.Is4() {
			if mapped := netip.AddrFrom16(addr.As16()).String(); clientKey(mapped) != key {
				t.Fatalf("%s and %s are in different buckets", ip, mapped)
			}
			return
		}
		if addr.Zone() != "" {
			return
		}
		// Replace the interface identifier (low 64 bits) and expect the same bucket.
		b := addr.As16()
		for i := range 8 {
			b[15-i] = byte(host >> (8 * i))
		}
		if other := netip.AddrFrom16(b).String(); clientKey(other) != key {
			t.Fatalf("%s and %s share a /64 but got buckets %q and %q", ip, other, key, clientKey(other))
		}
	})
}
