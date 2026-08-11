package fetch

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
)

// githubBlobRe matches GitHub blob URLs and captures the owner, repository,
// ref, and file path needed to build the corresponding raw URL.
var githubBlobRe = regexp.MustCompile(`^https?://github\.com/([^/]+)/([^/]+)/blob/([^/]+)/(.+)$`)

// NormalizeGitHubURL converts a GitHub blob URL to its raw content URL
// URLs that don't match the expected GitHub format are returned unchanged
func NormalizeGitHubURL(rawURL string) string {
	m := githubBlobRe.FindStringSubmatch(rawURL)
	if m == nil {
		return rawURL
	}
	owner, repo, ref, path := m[1], m[2], m[3], m[4]
	return fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/%s/%s", owner, repo, ref, path)
}

// FetchURL retrieves the content at the given URL transparently
// converting GitHub blob URLs to raw content URLs first
func FetchURL(client *http.Client, rawURL string) ([]byte, error) {
	target := NormalizeGitHubURL(rawURL)

	resp, err := client.Get(target)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch %s: %w", target, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch %s: server returned status %d", target, resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body from %s: %w", target, err)
	}
	return body, nil
}

// FileNameFromURL returns the last path segment of a URL. The filename is
// used to infer the target ecosystem when no registry is specified
func FileNameFromURL(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
	// Fall back to splitting the raw string if URL parsing fails
		parts := strings.Split(rawURL, "/")
		return parts[len(parts)-1]
	}
	parts := strings.Split(strings.TrimSuffix(u.Path, "/"), "/")
	return parts[len(parts)-1]
}
