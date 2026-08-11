package registry

import (
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"
)

const pypiBaseURL = "https://pypi.org/pypi/"

var pep503SeparatorRe = regexp.MustCompile(`[-_.]+`)

// Normalize the name according to PEP 503.
func normalizePyPIName(name string) string {
	name = strings.ToLower(strings.TrimSpace(name))
	return pep503SeparatorRe.ReplaceAllString(name, "-")
}

// CheckPyPI checks whether the package exists on PyPI.
func CheckPyPI(client *http.Client, name string) (claimed bool, err error) {
	normalized := normalizePyPIName(name)
	if normalized == "" {
		return false, fmt.Errorf("empty package name")
	}

	reqURL := pypiBaseURL + url.PathEscape(normalized) + "/json"

	resp, err := client.Get(reqURL)
	if err != nil {
		return false, fmt.Errorf("pypi registry request failed: %w", err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
		return true, nil
	case http.StatusNotFound:
		return false, nil
	default:
		return false, fmt.Errorf("pypi registry returned unexpected status %d for %q", resp.StatusCode, name)
	}
}
