package namecheap

import (
	"context"
	"encoding/xml"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/dal-go/dalgo/dal"
)

// HostRecord represents a single DNS host entry.
type HostRecord struct {
	HostName   string
	RecordType string
	Address    string
	TTL        string
	MXPref     int
}

// DNSHosts holds a slice of DNS host records for a domain.
type DNSHosts struct {
	Hosts []HostRecord
}

// DNSHostsCollection provides dalgo-style access to NameCheap DNS host records.
type DNSHostsCollection struct {
	client *Client
}

var _ dal.Getter = (*DNSHostsCollection)(nil)
var _ dal.Setter = (*DNSHostsCollection)(nil)

// Get retrieves DNS host records for the domain named by record.Key().ID.
func (d *DNSHostsCollection) Get(ctx context.Context, record dal.Record) error {
	domainName := fmt.Sprintf("%v", record.Key().ID)
	sld, tld := splitDomain(domainName)

	params := url.Values{}
	params.Set("Command", "namecheap.domains.dns.getHosts")
	params.Set("SLD", sld)
	params.Set("TLD", tld)

	resp, err := d.client.doRequest(ctx, params)
	if err != nil {
		record.SetError(err)
		return err
	}

	result := resp.CommandResponse.DNSGetHostsResult
	if result == nil {
		err = fmt.Errorf("namecheap: dns.getHosts: missing DomainDNSGetHostsResult in response")
		record.SetError(err)
		return err
	}

	hosts := make([]HostRecord, 0, len(result.Hosts))
	for _, h := range result.Hosts {
		mxPref, _ := strconv.Atoi(h.MXPref)
		hosts = append(hosts, HostRecord{
			HostName:   h.Name,
			RecordType: h.Type,
			Address:    h.Address,
			TTL:        h.TTL,
			MXPref:     mxPref,
		})
	}

	record.SetError(nil)
	if data, ok := record.Data().(*DNSHosts); ok {
		data.Hosts = hosts
	}
	return nil
}

// Exists returns true if the domain's DNS hosts record exists.
func (d *DNSHostsCollection) Exists(ctx context.Context, key *dal.Key) (bool, error) {
	record := dal.NewRecordWithData(key, &DNSHosts{})
	err := d.Get(ctx, record)
	if err != nil {
		if dal.IsNotFound(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// Set atomically replaces all DNS host records for the domain named by record.Key().ID.
func (d *DNSHostsCollection) Set(ctx context.Context, record dal.Record) error {
	domainName := fmt.Sprintf("%v", record.Key().ID)
	sld, tld := splitDomain(domainName)

	// dalgo's record state machine requires SetError(nil) before Data() is accessible.
	record.SetError(nil)
	data, ok := record.Data().(*DNSHosts)
	if !ok || data == nil {
		return fmt.Errorf("namecheap: dns.setHosts: record data must be *DNSHosts")
	}

	params := url.Values{}
	params.Set("Command", "namecheap.domains.dns.setHosts")
	params.Set("SLD", sld)
	params.Set("TLD", tld)

	for i, host := range data.Hosts {
		idx := strconv.Itoa(i + 1)
		params.Set("HostName"+idx, host.HostName)
		params.Set("RecordType"+idx, host.RecordType)
		params.Set("Address"+idx, host.Address)
		params.Set("TTL"+idx, host.TTL)
		if strings.EqualFold(host.RecordType, "MX") {
			params.Set("MXPref"+idx, strconv.Itoa(host.MXPref))
		} else {
			params.Set("MXPref"+idx, "10")
		}
	}

	_, err := d.client.doRequest(ctx, params)
	return err
}

// splitDomain splits a domain name into SLD and TLD.
// e.g. "example.com" -> "example", "com"
// e.g. "example.co.uk" -> "example", "co.uk"
func splitDomain(domain string) (sld, tld string) {
	idx := strings.IndexByte(domain, '.')
	if idx < 0 {
		return domain, ""
	}
	return domain[:idx], domain[idx+1:]
}

// xmlDNSGetHostsResult is the XML structure for dns.getHosts response.
type xmlDNSGetHostsResult struct {
	XMLName xml.Name     `xml:"DomainDNSGetHostsResult"`
	Hosts   []xmlDNSHost `xml:"host"`
}

type xmlDNSHost struct {
	Name    string `xml:"Name,attr"`
	Type    string `xml:"Type,attr"`
	Address string `xml:"Address,attr"`
	TTL     string `xml:"TTL,attr"`
	MXPref  string `xml:"MXPref,attr"`
}

// xmlDNSSetHostsResult is the XML structure for dns.setHosts response.
type xmlDNSSetHostsResult struct {
	XMLName   xml.Name `xml:"DomainDNSSetHostsResult"`
	IsSuccess string   `xml:"IsSuccess,attr"`
}
