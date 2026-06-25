package upstreamurl

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
)

// NormalizeHTTPBaseURL returns the canonical HTTP(S) upstream base URL used by providers.
func NormalizeHTTPBaseURL(raw string) (string, error) {
	baseURL := strings.TrimRight(strings.TrimSpace(raw), "/")
	if baseURL == "" {
		return "", errors.New("base_url is required")
	}

	parsed, err := url.Parse(baseURL)
	if err != nil {
		return "", fmt.Errorf("base_url is invalid: %w", err)
	}
	if !isHTTPBaseURL(parsed) {
		return "", errors.New("base_url must use http or https scheme")
	}
	if hasQueryOrFragment(parsed) {
		return "", errors.New("base_url must not include query or fragment")
	}
	return baseURL, nil
}

func isHTTPBaseURL(u *url.URL) bool {
	if u == nil || u.Host == "" {
		return false
	}
	return u.Scheme == "http" || u.Scheme == "https"
}

func hasQueryOrFragment(u *url.URL) bool {
	return u != nil && (u.RawQuery != "" || u.Fragment != "")
}
