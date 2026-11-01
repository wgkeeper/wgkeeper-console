package handlers

import (
	"net/url"
	"testing"
)

// FuzzParseNodeAddress checks that an accepted node address is a plain
// http(s) base URL with no credentials, path, query or fragment, and that
// parsing it again gives the same result.
func FuzzParseNodeAddress(f *testing.F) {
	for _, seed := range []string{
		"https://node.example:51821",
		"http://10.0.0.5:51821/",
		"https://[fd00::1]:51821/peers?x=1#y",
		"https://user:pw@node.example",
		"ftp://node.example",
		"node.example",
		" https://node.example/ ",
		"https://",
		"http://%zz",
	} {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, raw string) {
		got, ok := parseNodeAddress(raw)
		if !ok {
			return
		}
		u, err := url.Parse(got)
		if err != nil {
			t.Fatalf("accepted %q but the result %q does not parse: %v", raw, got, err)
		}
		if u.Scheme != "http" && u.Scheme != "https" {
			t.Fatalf("accepted scheme %q from %q", u.Scheme, raw)
		}
		if u.Host == "" || u.User != nil || u.Path != "" || u.RawQuery != "" || u.Fragment != "" {
			t.Fatalf("accepted %q as %q: want only scheme and host", raw, got)
		}
		again, ok := parseNodeAddress(got)
		if !ok || again != got {
			t.Fatalf("not idempotent: %q -> %q -> (%q, %v)", raw, got, again, ok)
		}
	})
}
