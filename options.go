package namecheap

import (
	"fmt"
	"io"
	"net/http"
	"strings"
)

// optionFunc is a functional option for configuring the NameCheap client.
type optionFunc func(cfg *config, hasClientIP *bool, hasAutodetect *bool, ipDetectionURL *string) error

// Option is the type for client options.
type Option = optionFunc

// WithAPIUser sets the NameCheap API username.
func WithAPIUser(user string) Option {
	return func(cfg *config, _ *bool, _ *bool, _ *string) error {
		cfg.apiUser = user
		return nil
	}
}

// WithAPIKey sets the NameCheap API key.
func WithAPIKey(key string) Option {
	return func(cfg *config, _ *bool, _ *bool, _ *string) error {
		cfg.apiKey = key
		return nil
	}
}

// WithClientIP sets the client IP to send with every API request.
// Mutually exclusive with WithClientIPAutodetection.
func WithClientIP(ip string) Option {
	return func(cfg *config, hasClientIP *bool, _ *bool, _ *string) error {
		cfg.clientIP = ip
		*hasClientIP = true
		return nil
	}
}

// WithClientIPAutodetection detects the outbound IP at construction time.
// Mutually exclusive with WithClientIP.
func WithClientIPAutodetection() Option {
	return func(_ *config, _ *bool, hasAutodetect *bool, _ *string) error {
		*hasAutodetect = true
		return nil
	}
}

// WithIPDetectionURL overrides the IP detection endpoint.
// Only has effect when WithClientIPAutodetection is also supplied.
func WithIPDetectionURL(u string) Option {
	return func(_ *config, _ *bool, _ *bool, ipDetectionURL *string) error {
		*ipDetectionURL = u
		return nil
	}
}

// WithSandbox switches to the NameCheap sandbox API.
func WithSandbox() Option {
	return func(cfg *config, _ *bool, _ *bool, _ *string) error {
		cfg.baseURL = sandboxBaseURL
		return nil
	}
}

// detectClientIP calls the given URL and returns the response body as the IP address.
func detectClientIP(detectionURL string) (string, error) {
	resp, err := http.Get(detectionURL) //nolint:noctx
	if err != nil {
		return "", fmt.Errorf("failed to detect client IP: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read IP detection response: %w", err)
	}
	return strings.TrimSpace(string(body)), nil
}
