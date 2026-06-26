//go:build integration

package namecheap

import (
	"context"
	"io"
	"testing"

	"github.com/dal-go/dalgo/dal"
)

// Integration tests require a real NameCheap account.
// Set credentials via env vars NAMECHEAP_API_USER and NAMECHEAP_API_KEY,
// or via ~/.namecheap-api file.
// Run with: go test -tags integration ./...

func integrationClient(t *testing.T) *Client {
	t.Helper()
	opts, err := ConfigFromEnv()
	if err != nil {
		t.Skipf("skipping integration test: %v", err)
	}
	c, err := New(append(opts, WithClientIPAutodetection(), WithSandbox())...)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}
	return c
}

func TestIntegration_DomainsList(t *testing.T) {
	c := integrationClient(t)
	reader, err := c.DomainsCollection().ExecuteQueryToRecordsReader(context.Background(), mockQuery{limit: 20})
	if err != nil {
		t.Fatalf("list domains: %v", err)
	}
	defer reader.Close()

	count := 0
	for {
		_, err := reader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("Next: %v", err)
		}
		count++
	}
	t.Logf("found %d domains", count)
}

func TestIntegration_DomainGet(t *testing.T) {
	c := integrationClient(t)

	domainName := "namecheap.com" // sandbox public domain
	data := &DomainInfo{}
	rec := dal.NewRecordWithData(dal.NewKeyWithID("domains", domainName), data)
	if err := c.DomainsCollection().Get(context.Background(), rec); err != nil {
		if dal.IsNotFound(err) {
			t.Skipf("domain %q not found in sandbox account", domainName)
		}
		t.Fatalf("get domain: %v", err)
	}
	t.Logf("domain: %+v", data)
}
