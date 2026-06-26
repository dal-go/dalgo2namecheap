# dalgo2namecheap

[![Go Reference](https://pkg.go.dev/badge/github.com/dal-go/dalgo2namecheap.svg)](https://pkg.go.dev/github.com/dal-go/dalgo2namecheap)
[![License](https://img.shields.io/badge/license-Apache%202.0-blue.svg)](LICENSE)

NameCheap API adapter for [dalgo](https://github.com/dal-go/dalgo) — exposes domain registrations and DNS host records as typed dalgo Collections.

## Features

- **`DomainsCollection`** — implements `dal.Getter` (getInfo) and `dal.QueryExecutor` (getList with pagination)
- **`DNSHostsCollection`** — implements `dal.Getter` (getHosts) and `dal.Setter` (setHosts)
- Functional-options constructor with IP auto-detection via [ipify](https://www.ipify.org/)
- Credential loading from env vars or `~/.namecheap-api` file
- Sandbox mode for testing writes without affecting production
- Two-tier tests: httptest unit tests + build-tagged integration tests

## Installation

```bash
go get github.com/dal-go/dalgo2namecheap
```

## Credential Setup

Create `~/.namecheap-api` (chmod 600):

```bash
touch ~/.namecheap-api && chmod 600 ~/.namecheap-api
```

Add your credentials:

```
NAMECHEAP_API_USER="your_username"
NAMECHEAP_API_KEY="your_api_key"
NAMECHEAP_CLIENT_IP="your_whitelisted_ip"
```

Enable API access in your NameCheap account: **Profile → Tools → Business & Dev Tools → NameCheap API Access**, and whitelist your IP address.

Alternatively, set env vars directly:

```bash
export NAMECHEAP_API_USER=your_username
export NAMECHEAP_API_KEY=your_api_key
```

## Usage

### Initialize the client

```go
import "github.com/dal-go/dalgo2namecheap"

// Load credentials from env / ~/.namecheap-api
opts, err := namecheap.ConfigFromEnv()
if err != nil {
    log.Fatal(err)
}

// Add client IP (explicit or auto-detected)
opts = append(opts, namecheap.WithClientIPAutodetection())

client, err := namecheap.New(opts...)
if err != nil {
    log.Fatal(err)
}
```

Or supply everything explicitly:

```go
client, err := namecheap.New(
    namecheap.WithAPIUser("myuser"),
    namecheap.WithAPIKey("mykey"),
    namecheap.WithClientIP("1.2.3.4"),
)
```

### List domains

```go
ctx := context.Background()
col := client.DomainsCollection()

q := dal.From("domains").Limit(20)
reader, err := col.ExecuteQueryToRecordsReader(ctx, q)
if err != nil {
    log.Fatal(err)
}
defer reader.Close()

for {
    var info namecheap.DomainInfo
    rec := dal.NewRecordWithData(dal.NewKeyWithID("domains", ""), &info)
    err := reader.Next(ctx, rec)
    if errors.Is(err, dal.ErrNoMoreRecords) {
        break
    }
    if err != nil {
        log.Fatal(err)
    }
    fmt.Printf("%s  expires=%s  autoRenew=%v\n", info.DomainName, info.Expires.Format("2006-01-02"), info.AutoRenew)
}
```

### Get a single domain

```go
var info namecheap.DomainInfo
rec := dal.NewRecordWithData(dal.NewKeyWithID("domains", "example.com"), &info)
if err := client.DomainsCollection().Get(ctx, rec); err != nil {
    log.Fatal(err)
}
fmt.Println("Expires:", info.Expires)
fmt.Println("Nameservers:", info.Nameservers)
```

### Get DNS hosts

```go
var hosts namecheap.DNSHosts
rec := dal.NewRecordWithData(dal.NewKeyWithID("dns", "example.com"), &hosts)
if err := client.DNSHostsCollection().Get(ctx, rec); err != nil {
    log.Fatal(err)
}
for _, h := range hosts.Hosts {
    fmt.Printf("%s %s %s\n", h.HostName, h.RecordType, h.Address)
}
```

### Set DNS hosts

```go
hosts := namecheap.DNSHosts{
    Hosts: []namecheap.HostRecord{
        {HostName: "@",   RecordType: "A",     Address: "1.2.3.4",     TTL: "1800"},
        {HostName: "www", RecordType: "CNAME", Address: "example.com.", TTL: "1800"},
        {HostName: "@",   RecordType: "MX",    Address: "mail.example.com.", TTL: "1800", MXPref: 10},
    },
}
rec := dal.NewRecordWithData(dal.NewKeyWithID("dns", "example.com"), &hosts)
rec.SetError(nil) // mark as loaded before setting
if err := client.DNSHostsCollection().Set(ctx, rec); err != nil {
    log.Fatal(err)
}
```

### Sandbox mode

For integration tests that write DNS records, use the sandbox environment:

```go
client, err := namecheap.New(
    namecheap.WithAPIUser("myuser"),
    namecheap.WithAPIKey("mykey"),
    namecheap.WithClientIP("1.2.3.4"),
    namecheap.WithSandbox(),
)
```

## Running integration tests

```bash
export NAMECHEAP_API_USER=your_username
export NAMECHEAP_API_KEY=your_api_key
export NAMECHEAP_CLIENT_IP=your_ip  # or use WithClientIPAutodetection()
export NAMECHEAP_SANDBOX_DOMAIN=sandbox-domain.com  # for DNS write tests

go test -v -tags=integration ./...
```

## Error handling

```go
import "errors"

err := client.DomainsCollection().Get(ctx, rec)
if dal.IsNotFound(err) {
    // domain not found (NameCheap error 2019166)
}
if errors.Is(err, namecheap.ErrRateLimited) {
    // rate limited (NameCheap error 2030280)
}
var apiErr namecheap.APIError
if errors.As(err, &apiErr) {
    fmt.Printf("NameCheap API error %d: %s\n", apiErr.Code, apiErr.Message)
}
```

## License

Apache 2.0 — see [LICENSE](LICENSE).
