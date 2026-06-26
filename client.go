package namecheap

import (
	"fmt"
	"net/http"
)

const (
	productionBaseURL    = "https://api.namecheap.com/xml.response"
	sandboxBaseURL       = "https://api.sandbox.namecheap.com/xml.response"
	defaultIPDetectionURL = "https://api.ipify.org"
)

type config struct {
	apiUser  string
	apiKey   string
	clientIP string
	baseURL  string
}

// Client is the NameCheap API client. Create it with New().
type Client struct {
	cfg        config
	httpClient *http.Client
}

// New creates a new NameCheap API client.
// Required options: WithAPIUser, WithAPIKey, and either WithClientIP or WithClientIPAutodetection.
func New(opts ...Option) (*Client, error) {
	cfg := config{
		baseURL: productionBaseURL,
	}
	var hasClientIP bool
	var hasAutodetect bool
	ipDetectionURL := defaultIPDetectionURL

	for _, opt := range opts {
		if err := opt(&cfg, &hasClientIP, &hasAutodetect, &ipDetectionURL); err != nil {
			return nil, err
		}
	}

	if cfg.apiUser == "" {
		return nil, fmt.Errorf("namecheap: APIUser is required")
	}
	if cfg.apiKey == "" {
		return nil, fmt.Errorf("namecheap: APIKey is required")
	}
	if hasClientIP && hasAutodetect {
		return nil, fmt.Errorf("namecheap: WithClientIP and WithClientIPAutodetection are mutually exclusive")
	}
	if !hasClientIP && !hasAutodetect {
		return nil, fmt.Errorf("namecheap: either WithClientIP or WithClientIPAutodetection is required")
	}

	if hasAutodetect {
		ip, err := detectClientIP(ipDetectionURL)
		if err != nil {
			return nil, fmt.Errorf("namecheap: IP auto-detection failed: %w", err)
		}
		cfg.clientIP = ip
	}

	return &Client{cfg: cfg, httpClient: http.DefaultClient}, nil
}

// DomainsCollection returns a collection for managing domains.
func (c *Client) DomainsCollection() *DomainsCollection {
	return &DomainsCollection{client: c}
}

// DNSHostsCollection returns a collection for managing DNS host records.
func (c *Client) DNSHostsCollection() *DNSHostsCollection {
	return &DNSHostsCollection{client: c}
}
