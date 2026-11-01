package services

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
)

var (
	// ErrNodeNotFound is returned when no node matches the lookup.
	ErrNodeNotFound = errors.New("node not found")
	// ErrNodeExists is returned when a node with the same address is already stored.
	ErrNodeExists = errors.New("node already exists")
	// ErrNodeUnreachable wraps transport failures talking to a node: connection
	// refused, timeouts, TLS errors, oversized or unreadable responses.
	ErrNodeUnreachable = errors.New("node unreachable")
)

// UpstreamError is a non-2xx answer from a node.
type UpstreamError struct {
	Status int
}

func (e *UpstreamError) Error() string {
	return fmt.Sprintf("node returned status %d", e.Status)
}

// Unauthorized reports whether the node rejected the stored API key.
func (e *UpstreamError) Unauthorized() bool {
	return e.Status == http.StatusUnauthorized || e.Status == http.StatusForbidden
}

func unreachable(err error) error {
	return fmt.Errorf("%w: %w", ErrNodeUnreachable, err)
}

const (
	// maxNodeResponseBytes bounds a successful node response. A list of 500
	// peers is well under 1 MiB; anything far beyond that is a broken or
	// hostile node, not data.
	maxNodeResponseBytes = 4 << 20
	// maxErrorBodyBytes bounds the error bodies kept for diagnostics.
	maxErrorBodyBytes = 64 << 10
	// maxDrainBytes is how much of an unread body is discarded so the
	// connection can be reused; larger leftovers just close it.
	maxDrainBytes = 64 << 10
)

var errResponseTooLarge = errors.New("node response too large")

// readBody reads at most limit bytes of the response body and fails if the
// node sent more.
func readBody(resp *http.Response, limit int64) ([]byte, error) {
	body, err := io.ReadAll(io.LimitReader(resp.Body, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(body)) > limit {
		return nil, errResponseTooLarge
	}
	return body, nil
}

// closeResponseBody drains a little of what is left of the body, so the
// HTTP/1.1 connection goes back to the pool, then closes it.
func closeResponseBody(resp *http.Response) {
	if resp == nil || resp.Body == nil {
		return
	}
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, maxDrainBytes)) //nolint:errcheck // best effort
	if err := resp.Body.Close(); err != nil {
		slog.Warn("failed to close response body", "err", err)
	}
}
