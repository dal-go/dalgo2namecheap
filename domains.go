package namecheap

import (
	"context"
	"encoding/xml"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/dal-go/dalgo/dal"
	"github.com/dal-go/dalgo/recordset"
)

// DomainInfo holds information about a registered domain.
type DomainInfo struct {
	DomainName  string
	Expires     time.Time
	IsExpired   bool
	AutoRenew   bool
	WhoisGuard  string
	Nameservers []string
}

// DomainsCollection provides dalgo-style access to NameCheap domain records.
type DomainsCollection struct {
	client *Client
}

var _ dal.Getter = (*DomainsCollection)(nil)
var _ dal.QueryExecutor = (*DomainsCollection)(nil)
var _ dal.Inserter = (*DomainsCollection)(nil)
var _ dal.MultiInserter = (*DomainsCollection)(nil)

// Get retrieves domain information for the domain named by record.Key().ID.
func (d *DomainsCollection) Get(ctx context.Context, record dal.Record) error {
	domainName := fmt.Sprintf("%v", record.Key().ID)

	params := url.Values{}
	params.Set("Command", "namecheap.domains.getInfo")
	params.Set("DomainName", domainName)

	resp, err := d.client.doRequest(ctx, params)
	if err != nil {
		record.SetError(err)
		return err
	}

	result := resp.CommandResponse.DomainGetInfoResult
	if result == nil {
		err = fmt.Errorf("namecheap: domains.getInfo: missing DomainGetInfoResult in response")
		record.SetError(err)
		return err
	}

	info, err := result.toDomainInfo()
	if err != nil {
		record.SetError(err)
		return err
	}

	record.SetError(nil)
	// Update the data pointer (record was created with NewRecordWithData)
	if data, ok := record.Data().(*DomainInfo); ok {
		*data = *info
	}
	return nil
}

// Exists returns true if the domain exists in the NameCheap account.
func (d *DomainsCollection) Exists(ctx context.Context, key *dal.Key) (bool, error) {
	record := dal.NewRecordWithData(key, &DomainInfo{})
	err := d.Get(ctx, record)
	if err != nil {
		if dal.IsNotFound(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// ExecuteQueryToRecordsReader lists domains matching the query.
func (d *DomainsCollection) ExecuteQueryToRecordsReader(ctx context.Context, query dal.Query) (dal.RecordsReader, error) {
	limit := query.Limit()
	offset := query.Offset()

	if limit > 100 {
		return nil, fmt.Errorf("namecheap: domains list: Limit must not exceed 100, got %d", limit)
	}

	pageSize := limit
	if pageSize == 0 {
		pageSize = 20
	}

	if offset%pageSize != 0 {
		return nil, fmt.Errorf("namecheap: domains list: Offset (%d) must be a multiple of PageSize (%d)", offset, pageSize)
	}

	page := (offset / pageSize) + 1

	params := url.Values{}
	params.Set("Command", "namecheap.domains.getList")
	params.Set("PageSize", fmt.Sprintf("%d", pageSize))
	params.Set("Page", fmt.Sprintf("%d", page))

	resp, err := d.client.doRequest(ctx, params)
	if err != nil {
		return nil, err
	}

	result := resp.CommandResponse.DomainGetListResult
	if result == nil {
		return newDomainRecordsReader(nil), nil
	}

	var infos []*DomainInfo
	for _, xmlDomain := range result.Domains {
		info, err := xmlDomain.toDomainInfo()
		if err != nil {
			return nil, fmt.Errorf("namecheap: domains list: failed to parse domain: %w", err)
		}
		infos = append(infos, info)
	}

	return newDomainRecordsReader(infos), nil
}

// ExecuteQueryToRecordsetReader is not implemented for NameCheap.
func (d *DomainsCollection) ExecuteQueryToRecordsetReader(_ context.Context, _ dal.Query, _ ...recordset.Option) (dal.RecordsetReader, error) {
	return nil, dal.ErrNotImplementedYet
}

// Insert is not supported for domain registration.
func (d *DomainsCollection) Insert(_ context.Context, _ dal.Record, _ ...dal.InsertOption) error {
	return fmt.Errorf("%w: domain registration is not supported in this version", dal.ErrNotImplementedYet)
}

// InsertMulti is not supported for domain registration.
func (d *DomainsCollection) InsertMulti(_ context.Context, _ []dal.Record, _ ...dal.InsertOption) error {
	return fmt.Errorf("%w: domain registration is not supported in this version", dal.ErrNotImplementedYet)
}

// xmlDomainGetInfoResult is the XML structure for namecheap.domains.getInfo response.
type xmlDomainGetInfoResult struct {
	XMLName       xml.Name         `xml:"DomainGetInfoResult"`
	DomainName    string           `xml:"DomainName,attr"`
	IsExpired     bool             `xml:"IsExpired,attr,omitempty"`
	AutoRenew     bool             `xml:"AutoRenew,attr,omitempty"`
	DomainDetails xmlDomainDetails `xml:"DomainDetails"`
	Whoisguard    xmlWhoisguard    `xml:"Whoisguard"`
	DnsDetails    xmlDnsDetails    `xml:"DnsDetails"`
}

type xmlDomainDetails struct {
	ExpiredDate string `xml:"ExpiredDate"`
	IsExpired   string `xml:"IsExpired,omitempty"`
	AutoRenew   string `xml:"AutoRenew,omitempty"`
}

type xmlWhoisguard struct {
	Enabled string `xml:"Enabled,attr"`
}

type xmlDnsDetails struct {
	Nameservers []string `xml:"Nameserver"`
}

func (r *xmlDomainGetInfoResult) toDomainInfo() (*DomainInfo, error) {
	info := &DomainInfo{
		DomainName:  r.DomainName,
		AutoRenew:   r.AutoRenew,
		IsExpired:   r.IsExpired,
		WhoisGuard:  r.Whoisguard.Enabled,
		Nameservers: r.DnsDetails.Nameservers,
	}

	// Parse expiry date (various formats)
	if r.DomainDetails.ExpiredDate != "" {
		t, err := parseNamecheapDate(r.DomainDetails.ExpiredDate)
		if err != nil {
			return nil, fmt.Errorf("failed to parse domain expiry date %q: %w", r.DomainDetails.ExpiredDate, err)
		}
		info.Expires = t
	}

	return info, nil
}

// xmlDomainGetListResult holds the list response.
type xmlDomainGetListResult struct {
	XMLName xml.Name          `xml:"DomainGetListResult"`
	Domains []xmlDomainInList `xml:"Domain"`
}

type xmlDomainInList struct {
	Name       string `xml:"Name,attr"`
	Expires    string `xml:"Expires,attr"`
	IsExpired  string `xml:"IsExpired,attr"`
	AutoRenew  string `xml:"AutoRenew,attr"`
	WhoisGuard string `xml:"WhoisGuard,attr"`
}

func (d *xmlDomainInList) toDomainInfo() (*DomainInfo, error) {
	info := &DomainInfo{
		DomainName: d.Name,
		WhoisGuard: d.WhoisGuard,
	}

	if d.Expires != "" {
		t, err := parseNamecheapDate(d.Expires)
		if err != nil {
			return nil, fmt.Errorf("failed to parse domain list expiry date %q: %w", d.Expires, err)
		}
		info.Expires = t
	}

	info.IsExpired = strings.EqualFold(d.IsExpired, "true")
	info.AutoRenew = strings.EqualFold(d.AutoRenew, "true")

	return info, nil
}

// parseNamecheapDate parses a NameCheap date string, trying multiple formats.
func parseNamecheapDate(s string) (time.Time, error) {
	formats := []string{
		"01/02/2006",          // MM/DD/YYYY
		"01/02/2006 15:04:05", // MM/DD/YYYY HH:MM:SS
		"2006-01-02",          // YYYY-MM-DD
		"2006-01-02T15:04:05", // ISO 8601
	}
	for _, f := range formats {
		if t, err := time.Parse(f, s); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("unrecognized date format: %q", s)
}
