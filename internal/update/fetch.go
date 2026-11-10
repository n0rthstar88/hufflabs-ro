package update

import (
	"context"
	"crypto/ed25519"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"hufflabs-ro/internal/manifest"
)

// ErrUnavailable means the update server couldn't be reached or didn't answer with
// a valid signed manifest.
var ErrUnavailable = errors.New("update server unavailable")

const maxManifest = 16 << 20

// FetchManifest downloads and verifies <siteURL>/update/manifest.json.
func FetchManifest(ctx context.Context, c *http.Client, siteURL string, trusted []ed25519.PublicKey) (manifest.Manifest, error) {
	if !strings.HasPrefix(siteURL, "https://") {
		return manifest.Manifest{}, fmt.Errorf("%w: site URL must be https", ErrUnavailable)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(siteURL, "/")+"/update/manifest.json", nil)
	if err != nil {
		return manifest.Manifest{}, fmt.Errorf("%w: %v", ErrUnavailable, err)
	}
	req.Header.Set("Cache-Control", "no-cache")
	resp, err := c.Do(req)
	if err != nil {
		return manifest.Manifest{}, fmt.Errorf("%w: %v", ErrUnavailable, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return manifest.Manifest{}, fmt.Errorf("%w: HTTP %d", ErrUnavailable, resp.StatusCode)
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, maxManifest+1))
	if err != nil {
		return manifest.Manifest{}, fmt.Errorf("%w: %v", ErrUnavailable, err)
	}
	if len(b) > maxManifest {
		return manifest.Manifest{}, fmt.Errorf("%w: manifest too large", ErrUnavailable)
	}
	m, err := manifest.Verify(b, trusted)
	if err != nil {
		return manifest.Manifest{}, fmt.Errorf("%w: %v", ErrUnavailable, err)
	}
	return m, nil
}
