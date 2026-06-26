package namecheap

import (
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// xmlApiResponse represents the top-level NameCheap API XML response.
type xmlApiResponse struct {
	XMLName xml.Name `xml:"ApiResponse"`
	Status  string   `xml:"Status,attr"`
	Errors  struct {
		Errors []xmlApiError `xml:"Error"`
	} `xml:"Errors"`
	CommandResponse xmlCommandResponse `xml:"CommandResponse"`
}

type xmlApiError struct {
	Number  int    `xml:"Number,attr"`
	Message string `xml:",chardata"`
}

type xmlCommandResponse struct {
	// Domains getInfo
	DomainGetInfoResult *xmlDomainGetInfoResult `xml:"DomainGetInfoResult"`
	// Domains getList
	DomainGetListResult *xmlDomainGetListResult `xml:"DomainGetListResult"`
	Paging              *xmlPaging              `xml:"Paging"`
	// DNS getHosts
	DNSGetHostsResult *xmlDNSGetHostsResult `xml:"DomainDNSGetHostsResult"`
	// DNS setHosts
	DNSSetHostsResult *xmlDNSSetHostsResult `xml:"DomainDNSSetHostsResult"`
}

type xmlPaging struct {
	TotalItems  int `xml:"TotalItems"`
	CurrentPage int `xml:"CurrentPage"`
	PageSize    int `xml:"PageSize"`
}

// doRequest builds and executes an authenticated GET request to the NameCheap API.
// The apiKey is never included in any returned error message.
func (c *Client) doRequest(ctx context.Context, params url.Values) (*xmlApiResponse, error) {
	params.Set("ApiUser", c.cfg.apiUser)
	params.Set("ApiKey", c.cfg.apiKey)
	params.Set("UserName", c.cfg.apiUser)
	params.Set("ClientIp", c.cfg.clientIP)

	reqURL := c.cfg.baseURL + "?" + params.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, c.sanitizeErr(fmt.Errorf("namecheap: failed to build request: %w", err))
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, c.sanitizeErr(fmt.Errorf("namecheap: HTTP request failed: %w", err))
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("namecheap: failed to read response body: %w", err)
	}

	var apiResp xmlApiResponse
	if err := xml.Unmarshal(body, &apiResp); err != nil {
		return nil, fmt.Errorf("namecheap: failed to parse XML response: %w", err)
	}
	if apiResp.Status == "" {
		return nil, fmt.Errorf("namecheap: empty or invalid XML response")
	}

	if apiResp.Status == "ERROR" {
		if len(apiResp.Errors.Errors) > 0 {
			e := apiResp.Errors.Errors[0]
			return nil, mapAPIError(e.Number, strings.TrimSpace(e.Message))
		}
		return nil, fmt.Errorf("namecheap: API returned ERROR status without error details")
	}

	return &apiResp, nil
}

// sanitizeErr replaces the API key with [REDACTED] in error messages to prevent key leakage.
func (c *Client) sanitizeErr(err error) error {
	if err == nil || c.cfg.apiKey == "" {
		return err
	}
	msg := err.Error()
	if strings.Contains(msg, c.cfg.apiKey) {
		return &sanitizedError{msg: strings.ReplaceAll(msg, c.cfg.apiKey, "[REDACTED]")}
	}
	return err
}

type sanitizedError struct {
	msg string
}

func (e *sanitizedError) Error() string { return e.msg }
