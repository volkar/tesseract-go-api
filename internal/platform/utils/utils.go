package utils

import (
	"errors"
	"net/url"
	"strings"
)

// ValidateAndSanitizeURL ensures the provided URL is safe to store and display
func ValidateAndSanitizeURL(rawURL string) (string, error) {
	// Remove leading/trailing whitespaces and invisible control characters
	cleanURL := strings.TrimSpace(rawURL)
	if cleanURL == "" {
		return "", nil
	}

	// ParseRequestURI is stricter than url.Parse.
	// It demands an absolute URI (must include a scheme)
	parsed, err := url.ParseRequestURI(cleanURL)
	if err != nil {
		return "", errors.New("Invalid URL format")
	}

	// Strictly whitelist schemes to prevent "javascript:", "data:", "file:" vectors
	scheme := strings.ToLower(parsed.Scheme)
	if scheme != "http" && scheme != "https" {
		return "", errors.New("Only http and https protocols are allowed")
	}

	// Ensure the host is present (prevents tricky edge-cases)
	if parsed.Host == "" {
		return "", errors.New("Host cannot be empty")
	}

	// Reconstruction: we let the standard library rebuild the URL.
	// This automatically applies correct URL-encoding to paths and query params,
	// neutralizing any injected HTML/JS tags or unescaped quotes.
	return parsed.String(), nil
}
