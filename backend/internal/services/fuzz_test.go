package services

import "testing"

// FuzzParseComparableVersion checks that version labels from nodes and from
// GitHub never panic the parser, and that a version is never "outdated"
// compared with itself.
func FuzzParseComparableVersion(f *testing.F) {
	for _, seed := range []string{"1.2.3", "v2.0.0", "2.0.0-rc.1", "10", "1.2", "", "v", "1.2.3.4", "-1.0.0", "99999999999999999999.0.0"} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, raw string) {
		parts, ok := parseComparableVersion(raw)
		if ok {
			for i, p := range parts {
				if p < 0 {
					t.Fatalf("parseComparableVersion(%q) part %d = %d, want >= 0", raw, i, p)
				}
			}
		}
		if IsVersionOutdated(&raw, &LatestNodeRelease{Version: raw}) {
			t.Fatalf("%q is outdated compared with itself", raw)
		}
	})
}
