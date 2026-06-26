package namecheap

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/dal-go/dalgo/dal"
)

func TestDNSHostsCollection_Get_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, successXML(`<DomainDNSGetHostsResult>
			<host Name="@" Type="A" Address="1.2.3.4" TTL="1800" MXPref="10"/>
			<host Name="www" Type="CNAME" Address="example.com." TTL="1800" MXPref="10"/>
		</DomainDNSGetHostsResult>`))
	}))
	defer srv.Close()

	c := testClient(t, srv.URL)
	data := &DNSHosts{}
	rec := dal.NewRecordWithData(dal.NewKeyWithID("dns", "example.com"), data)
	if err := c.DNSHostsCollection().Get(context.Background(), rec); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(data.Hosts) != 2 {
		t.Fatalf("expected 2 hosts, got %d", len(data.Hosts))
	}
	if data.Hosts[0].HostName != "@" {
		t.Errorf("Host[0].HostName: want @, got %q", data.Hosts[0].HostName)
	}
	if data.Hosts[0].RecordType != "A" {
		t.Errorf("Host[0].RecordType: want A, got %q", data.Hosts[0].RecordType)
	}
	if data.Hosts[1].HostName != "www" {
		t.Errorf("Host[1].HostName: want www, got %q", data.Hosts[1].HostName)
	}
}

func TestDNSHostsCollection_Set_Params(t *testing.T) {
	var gotQuery url.Values
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.Query()
		fmt.Fprint(w, successXML(`<DomainDNSSetHostsResult IsSuccess="true"/>`))
	}))
	defer srv.Close()

	c := testClient(t, srv.URL)
	data := &DNSHosts{
		Hosts: []HostRecord{
			{HostName: "@", RecordType: "A", Address: "1.2.3.4", TTL: "1800"},
		},
	}
	rec := dal.NewRecordWithData(dal.NewKeyWithID("dns", "example.com"), data)
	if err := c.DNSHostsCollection().Set(context.Background(), rec); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if gotQuery.Get("HostName1") != "@" {
		t.Errorf("HostName1: want @, got %q", gotQuery.Get("HostName1"))
	}
	if gotQuery.Get("RecordType1") != "A" {
		t.Errorf("RecordType1: want A, got %q", gotQuery.Get("RecordType1"))
	}
	if gotQuery.Get("Address1") != "1.2.3.4" {
		t.Errorf("Address1: want 1.2.3.4, got %q", gotQuery.Get("Address1"))
	}
	if gotQuery.Get("TTL1") != "1800" {
		t.Errorf("TTL1: want 1800, got %q", gotQuery.Get("TTL1"))
	}
	// For A record, MXPref should be "10" (default)
	if gotQuery.Get("MXPref1") != "10" {
		t.Errorf("MXPref1 for A record: want 10, got %q", gotQuery.Get("MXPref1"))
	}
}

func TestDNSHostsCollection_Set_MultiRecords(t *testing.T) {
	var gotQuery url.Values
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.Query()
		fmt.Fprint(w, successXML(`<DomainDNSSetHostsResult IsSuccess="true"/>`))
	}))
	defer srv.Close()

	c := testClient(t, srv.URL)
	data := &DNSHosts{
		Hosts: []HostRecord{
			{HostName: "@", RecordType: "A", Address: "1.2.3.4", TTL: "1800"},
			{HostName: "www", RecordType: "CNAME", Address: "example.com.", TTL: "3600"},
		},
	}
	rec := dal.NewRecordWithData(dal.NewKeyWithID("dns", "example.com"), data)
	if err := c.DNSHostsCollection().Set(context.Background(), rec); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if gotQuery.Get("HostName2") != "www" {
		t.Errorf("HostName2: want www, got %q", gotQuery.Get("HostName2"))
	}
	if gotQuery.Get("RecordType2") != "CNAME" {
		t.Errorf("RecordType2: want CNAME, got %q", gotQuery.Get("RecordType2"))
	}
}

func TestDNSHostsCollection_Set_SLDTLDSplit(t *testing.T) {
	var gotQuery url.Values
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.Query()
		fmt.Fprint(w, successXML(`<DomainDNSSetHostsResult IsSuccess="true"/>`))
	}))
	defer srv.Close()

	c := testClient(t, srv.URL)
	data := &DNSHosts{
		Hosts: []HostRecord{{HostName: "@", RecordType: "A", Address: "1.2.3.4", TTL: "1800"}},
	}
	rec := dal.NewRecordWithData(dal.NewKeyWithID("dns", "example.co.uk"), data)
	if err := c.DNSHostsCollection().Set(context.Background(), rec); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if gotQuery.Get("SLD") != "example" {
		t.Errorf("SLD: want example, got %q", gotQuery.Get("SLD"))
	}
	if gotQuery.Get("TLD") != "co.uk" {
		t.Errorf("TLD: want co.uk, got %q", gotQuery.Get("TLD"))
	}
}

func TestDNSHostsCollection_Set_MXRecord(t *testing.T) {
	var gotQuery url.Values
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.Query()
		fmt.Fprint(w, successXML(`<DomainDNSSetHostsResult IsSuccess="true"/>`))
	}))
	defer srv.Close()

	c := testClient(t, srv.URL)
	data := &DNSHosts{
		Hosts: []HostRecord{
			{HostName: "@", RecordType: "MX", Address: "mail.example.com.", TTL: "1800", MXPref: 5},
		},
	}
	rec := dal.NewRecordWithData(dal.NewKeyWithID("dns", "example.com"), data)
	if err := c.DNSHostsCollection().Set(context.Background(), rec); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if gotQuery.Get("MXPref1") != "5" {
		t.Errorf("MXPref1 for MX record: want 5, got %q", gotQuery.Get("MXPref1"))
	}
}

func TestDNSHostsCollection_Set_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, successXML(`<DomainDNSSetHostsResult IsSuccess="true"/>`))
	}))
	defer srv.Close()

	c := testClient(t, srv.URL)
	data := &DNSHosts{
		Hosts: []HostRecord{{HostName: "@", RecordType: "A", Address: "1.2.3.4", TTL: "1800"}},
	}
	rec := dal.NewRecordWithData(dal.NewKeyWithID("dns", "example.com"), data)
	err := c.DNSHostsCollection().Set(context.Background(), rec)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}
