package manifest

import (
	"context"
	"fmt"
	"io"
	"net/http"
)

// HTTPClient is the http client used by Fetch. Tests may override.
var HTTPClient = http.DefaultClient

// Fetch downloads an encrypted manifest from url and decrypts it with the
// given age identity.
func Fetch(ctx context.Context, url, identity string) (*Manifest, error) {
	ciphertext, err := download(ctx, url)
	if err != nil {
		return nil, err
	}
	return Decrypt(ciphertext, identity)
}

// download fetches the raw bytes at url.
func download(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	resp, err := HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch %s: %w", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch %s: %s", url, resp.Status)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}
	return body, nil
}

// DownloadAsset fetches binary content at url. Used by install to pull a
// release asset directly from the URL named in the manifest.
func DownloadAsset(ctx context.Context, url string) (io.ReadCloser, int64, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, 0, fmt.Errorf("build request: %w", err)
	}
	resp, err := HTTPClient.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("fetch %s: %w", url, err)
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, 0, fmt.Errorf("fetch %s: %s", url, resp.Status)
	}
	return resp.Body, resp.ContentLength, nil
}
