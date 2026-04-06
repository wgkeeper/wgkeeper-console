package services

import "net/http"

// SetSharedHTTPRoundTripperForTests overrides the shared HTTP transport used by
// service clients. Intended for tests.
func SetSharedHTTPRoundTripperForTests(rt http.RoundTripper) http.RoundTripper {
	prev := sharedHTTPRoundTripper
	sharedHTTPRoundTripper = rt
	return prev
}
