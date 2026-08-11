package registry

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

const npmRegistryBaseURL = "https://registry.npmjs.org/"

// npmjs web endpoint used for checking scopes.
const npmWebBaseURL = "https://www.npmjs.com/org/"

// IsScoped reports whether name is an npm scoped package name, e.g.
// "@myorg/some-package".
func IsScoped(name string) bool {
	return strings.HasPrefix(name, "@") && strings.Contains(name, "/")
}

// Return the scope from a scoped package name.
func ScopeOf(name string) string {
	if !IsScoped(name) {
		return ""
	}
	return name[:strings.Index(name, "/")]
}

// Check whether the package exists on npm.
func CheckNPM(client *http.Client, name string) (claimed bool, err error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return false, fmt.Errorf("empty package name")
	}

	// The npm registry API accepts scoped names directly in the path
	// (e.g. /@scope/name), but each path segment must be escaped
	// individually a raw url.QueryEscape would turn "/" into "%2F"
	// and break the route.
	segments := strings.Split(name, "/")
	for i, seg := range segments {
		segments[i] = url.PathEscape(seg)
	}
	escapedPath := strings.Join(segments, "/")

	reqURL := npmRegistryBaseURL + escapedPath

	resp, err := client.Get(reqURL)
	if err != nil {
		return false, fmt.Errorf("npm registry request failed: %w", err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
		return true, nil
	case http.StatusNotFound:
		return false, nil
	default:
		return false, fmt.Errorf("npm registry returned unexpected status %d for %q", resp.StatusCode, name)
	}
}

// Check whether an npm scope is registered.
func CheckNPMScope(client *http.Client, scope string) (registered bool, err error) {
	name := strings.TrimPrefix(scope, "@")
	if name == "" {
		return false, fmt.Errorf("empty scope name")
	}

	req, err := http.NewRequest(http.MethodGet, npmWebBaseURL+url.PathEscape(name), nil)
	if err != nil {
		return false, fmt.Errorf("failed to build scope check request: %w", err)
	}
	// npmjs.com's edge Cloudflare blocks requests that don't look like
	// a real browser these headers are the minimum needed to pass.
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; DepIsConfuse dependency-confusion-scanner)")
	req.Header.Set("Accept-Language", "en-US")

	// // a real browser these headers are the minimum needed to pass.
	noRedirectClient := &http.Client{
		Timeout: client.Timeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	resp, err := noRedirectClient.Do(req)
	if err != nil {
		return false, fmt.Errorf("npm scope check request failed: %w", err)
	}
	defer resp.Body.Close()

	switch {
	case resp.StatusCode == http.StatusOK:
		return true, nil
	case resp.StatusCode >= 300 && resp.StatusCode < 400:
	
		return true, nil
	case resp.StatusCode == http.StatusNotFound:
		return false, nil
	default:
		return false, fmt.Errorf("npm scope check returned unexpected status %d for %q", resp.StatusCode, scope)
	}
}
