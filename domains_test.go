package namecheap

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dal-go/dalgo/dal"
)

// mockQuery implements dal.Query for testing.
type mockQuery struct {
	limit  int
	offset int
}

func (q mockQuery) String() string { return "mock" }
func (q mockQuery) Offset() int    { return q.offset }
func (q mockQuery) Limit() int     { return q.limit }
func (q mockQuery) GetRecordsReader(ctx context.Context, qe dal.QueryExecutor) (dal.RecordsReader, error) {
	return qe.ExecuteQueryToRecordsReader(ctx, q)
}
func (q mockQuery) GetRecordsetReader(ctx context.Context, qe dal.QueryExecutor) (dal.RecordsetReader, error) {
	return qe.ExecuteQueryToRecordsetReader(ctx, q)
}

// helper to build a test client pointing at a given httptest server URL
func testClient(t *testing.T, serverURL string) *Client {
	t.Helper()
	c, err := New(
		WithAPIUser("testuser"),
		WithAPIKey("testkey"),
		WithClientIP("1.2.3.4"),
	)
	if err != nil {
		t.Fatalf("testClient: %v", err)
	}
	c.cfg.baseURL = serverURL
	return c
}

// successXML wraps content in a valid OK ApiResponse.
func successXML(commandResponse string) string {
	return fmt.Sprintf(`<?xml version="1.0"?>
<ApiResponse Status="OK" xmlns="https://api.namecheap.com/xml.response">
  <Errors/>
  <CommandResponse>%s</CommandResponse>
</ApiResponse>`, commandResponse)
}

// errorXML builds an ERROR ApiResponse.
func errorXML(code int, msg string) string {
	return fmt.Sprintf(`<?xml version="1.0"?>
<ApiResponse Status="ERROR">
  <Errors><Error Number="%d">%s</Error></Errors>
  <CommandResponse/>
</ApiResponse>`, code, msg)
}

// ---------- New() error cases ----------

func TestNew_MissingUser(t *testing.T) {
	_, err := New(WithAPIKey("k"), WithClientIP("1.2.3.4"))
	if err == nil || !strings.Contains(err.Error(), "APIUser") {
		t.Errorf("expected APIUser error, got %v", err)
	}
}

func TestNew_MissingKey(t *testing.T) {
	_, err := New(WithAPIUser("u"), WithClientIP("1.2.3.4"))
	if err == nil || !strings.Contains(err.Error(), "APIKey") {
		t.Errorf("expected APIKey error, got %v", err)
	}
}

func TestNew_MissingIP(t *testing.T) {
	_, err := New(WithAPIUser("u"), WithAPIKey("k"))
	if err == nil || !strings.Contains(err.Error(), "WithClientIP") {
		t.Errorf("expected IP error, got %v", err)
	}
}

func TestNew_MutuallyExclusiveIP(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "1.2.3.4")
	}))
	defer srv.Close()

	_, err := New(
		WithAPIUser("u"),
		WithAPIKey("k"),
		WithClientIP("1.2.3.4"),
		WithClientIPAutodetection(),
		WithIPDetectionURL(srv.URL),
	)
	if err == nil || !strings.Contains(err.Error(), "mutually exclusive") {
		t.Errorf("expected mutually exclusive error, got %v", err)
	}
}

func TestNew_AutodetectSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "  5.6.7.8  ")
	}))
	defer srv.Close()

	c, err := New(
		WithAPIUser("u"),
		WithAPIKey("k"),
		WithClientIPAutodetection(),
		WithIPDetectionURL(srv.URL),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c.cfg.clientIP != "5.6.7.8" {
		t.Errorf("expected IP 5.6.7.8, got %q", c.cfg.clientIP)
	}
}

func TestNew_AutodetectFailure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// hijack and close connection
		hj, ok := w.(http.Hijacker)
		if ok {
			conn, _, _ := hj.Hijack()
			conn.Close()
			return
		}
		w.WriteHeader(500)
	}))
	defer srv.Close()

	_, err := New(
		WithAPIUser("u"),
		WithAPIKey("k"),
		WithClientIPAutodetection(),
		WithIPDetectionURL(srv.URL),
	)
	if err == nil || !strings.Contains(err.Error(), "IP auto-detection") {
		t.Errorf("expected autodetect failure error, got %v", err)
	}
}

func TestNew_WithIPDetectionURLIgnoredWithoutAutodetect(t *testing.T) {
	// WithIPDetectionURL alone (without autodetect) should not cause any errors
	c, err := New(
		WithAPIUser("u"),
		WithAPIKey("k"),
		WithClientIP("1.2.3.4"),
		WithIPDetectionURL("http://example.com/ip"),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c.cfg.clientIP != "1.2.3.4" {
		t.Errorf("expected IP 1.2.3.4, got %q", c.cfg.clientIP)
	}
}

// ---------- Sandbox ----------

func TestNew_SandboxURL(t *testing.T) {
	c, err := New(WithAPIUser("u"), WithAPIKey("k"), WithClientIP("1.2.3.4"), WithSandbox())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c.cfg.baseURL != sandboxBaseURL {
		t.Errorf("expected sandbox URL, got %q", c.cfg.baseURL)
	}
}

// ---------- ConfigFromEnv ----------

func TestConfigFromEnv_FromEnvVars(t *testing.T) {
	t.Setenv("NAMECHEAP_API_USER", "envuser")
	t.Setenv("NAMECHEAP_API_KEY", "envkey")

	opts, err := ConfigFromEnv()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(opts) != 2 {
		t.Fatalf("expected 2 options, got %d", len(opts))
	}
}

func TestConfigFromEnv_FromFile(t *testing.T) {
	// Use a temp home dir
	dir := t.TempDir()
	content := "NAMECHEAP_API_USER=fileuser\nNAMECHEAP_API_KEY=filekey\n"
	if err := os.WriteFile(filepath.Join(dir, namecheapAPIFile), []byte(content), 0600); err != nil {
		t.Fatal(err)
	}

	t.Setenv("HOME", dir)
	t.Setenv("NAMECHEAP_API_USER", "")
	t.Setenv("NAMECHEAP_API_KEY", "")

	// Temporarily override UserHomeDir by pointing HOME
	opts, err := ConfigFromEnv()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(opts) != 2 {
		t.Fatalf("expected 2 options, got %d", len(opts))
	}
	// Verify options by building a client
	c, err := New(append(opts, WithClientIP("1.2.3.4"))...)
	if err != nil {
		t.Fatalf("client build error: %v", err)
	}
	if c.cfg.apiUser != "fileuser" || c.cfg.apiKey != "filekey" {
		t.Errorf("unexpected creds: user=%q key=%q", c.cfg.apiUser, c.cfg.apiKey)
	}
}

func TestConfigFromEnv_Hybrid(t *testing.T) {
	// env has USER, file has KEY
	dir := t.TempDir()
	content := "NAMECHEAP_API_KEY=hybridkey\n"
	if err := os.WriteFile(filepath.Join(dir, namecheapAPIFile), []byte(content), 0600); err != nil {
		t.Fatal(err)
	}

	t.Setenv("HOME", dir)
	t.Setenv("NAMECHEAP_API_USER", "hybriduser")
	t.Setenv("NAMECHEAP_API_KEY", "")

	opts, err := ConfigFromEnv()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	c, err := New(append(opts, WithClientIP("1.2.3.4"))...)
	if err != nil {
		t.Fatalf("client build: %v", err)
	}
	if c.cfg.apiUser != "hybriduser" || c.cfg.apiKey != "hybridkey" {
		t.Errorf("unexpected creds: user=%q key=%q", c.cfg.apiUser, c.cfg.apiKey)
	}
}

func TestConfigFromEnv_Missing(t *testing.T) {
	dir := t.TempDir() // empty dir, no .namecheap-api
	t.Setenv("HOME", dir)
	t.Setenv("NAMECHEAP_API_USER", "")
	t.Setenv("NAMECHEAP_API_KEY", "")

	_, err := ConfigFromEnv()
	if err == nil {
		t.Error("expected error for missing credentials")
	}
}

func TestConfigFromEnv_UnreadableFile(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("running as root, file permissions are not enforced")
	}

	dir := t.TempDir()
	path := filepath.Join(dir, namecheapAPIFile)
	if err := os.WriteFile(path, []byte("NAMECHEAP_API_KEY=x\n"), 0000); err != nil {
		t.Fatal(err)
	}

	t.Setenv("HOME", dir)
	t.Setenv("NAMECHEAP_API_USER", "")
	t.Setenv("NAMECHEAP_API_KEY", "")

	_, err := ConfigFromEnv()
	if err == nil {
		t.Error("expected error for unreadable file")
	}
}

// ---------- Request params ----------

func TestDoRequest_RequiredParams(t *testing.T) {
	var gotQuery url.Values
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.Query()
		fmt.Fprint(w, successXML(`<DomainGetInfoResult DomainName="example.com">
			<DomainDetails><ExpiredDate>12/31/2025</ExpiredDate></DomainDetails>
			<Whoisguard Enabled="ENABLED"/>
			<DnsDetails/>
		</DomainGetInfoResult>`))
	}))
	defer srv.Close()

	c := testClient(t, srv.URL)
	rec := dal.NewRecordWithData(dal.NewKeyWithID("domains", "example.com"), &DomainInfo{})
	_ = c.DomainsCollection().Get(context.Background(), rec)

	for _, param := range []string{"ApiUser", "ApiKey", "UserName", "ClientIp"} {
		if gotQuery.Get(param) == "" {
			t.Errorf("required param %q missing from request", param)
		}
	}
}

func TestDoRequest_APIKeyNotLeaked(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// close connection to trigger HTTP error
		hj, ok := w.(http.Hijacker)
		if ok {
			conn, _, _ := hj.Hijack()
			conn.Close()
			return
		}
		w.WriteHeader(500)
	}))
	defer srv.Close()

	c, err := New(WithAPIUser("u"), WithAPIKey("supersecretkey"), WithClientIP("1.2.3.4"))
	if err != nil {
		t.Fatal(err)
	}
	c.cfg.baseURL = srv.URL

	rec := dal.NewRecordWithData(dal.NewKeyWithID("domains", "example.com"), &DomainInfo{})
	err = c.DomainsCollection().Get(context.Background(), rec)
	if err == nil {
		t.Fatal("expected error")
	}
	if strings.Contains(err.Error(), "supersecretkey") {
		t.Errorf("API key leaked in error: %v", err)
	}
}

// ---------- DomainsCollection.Get ----------

func TestDomainsCollection_Get_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, successXML(`<DomainGetInfoResult DomainName="example.com">
			<DomainDetails><ExpiredDate>12/31/2025</ExpiredDate></DomainDetails>
			<Whoisguard Enabled="ENABLED"/>
			<DnsDetails>
				<Nameserver>ns1.example.com</Nameserver>
				<Nameserver>ns2.example.com</Nameserver>
			</DnsDetails>
		</DomainGetInfoResult>`))
	}))
	defer srv.Close()

	c := testClient(t, srv.URL)
	data := &DomainInfo{}
	rec := dal.NewRecordWithData(dal.NewKeyWithID("domains", "example.com"), data)
	if err := c.DomainsCollection().Get(context.Background(), rec); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if data.DomainName != "example.com" {
		t.Errorf("DomainName: want example.com, got %q", data.DomainName)
	}
	if data.WhoisGuard != "ENABLED" {
		t.Errorf("WhoisGuard: want ENABLED, got %q", data.WhoisGuard)
	}
	if len(data.Nameservers) != 2 {
		t.Errorf("Nameservers: want 2, got %d", len(data.Nameservers))
	}
	if data.Expires.IsZero() {
		t.Error("Expires should not be zero")
	}
}

// ---------- DomainsCollection list ----------

func TestDomainsCollection_List_ThreeDomains(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, successXML(`<DomainGetListResult>
			<Domain Name="a.com" Expires="12/31/2025" IsExpired="false" AutoRenew="true" WhoisGuard="ENABLED"/>
			<Domain Name="b.com" Expires="01/01/2026" IsExpired="false" AutoRenew="false" WhoisGuard="NOTPRESENT"/>
			<Domain Name="c.com" Expires="06/15/2024" IsExpired="true" AutoRenew="false" WhoisGuard="ENABLED"/>
		</DomainGetListResult>`))
	}))
	defer srv.Close()

	c := testClient(t, srv.URL)
	reader, err := c.DomainsCollection().ExecuteQueryToRecordsReader(context.Background(), mockQuery{limit: 10})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer reader.Close()

	var names []string
	for {
		rec, err := reader.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatalf("Next: %v", err)
		}
		names = append(names, fmt.Sprintf("%v", rec.Key().ID))
	}
	if len(names) != 3 {
		t.Errorf("expected 3 domains, got %d: %v", len(names), names)
	}
}

func TestDomainsCollection_List_PaginationParams(t *testing.T) {
	var gotQuery url.Values
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.Query()
		fmt.Fprint(w, successXML(`<DomainGetListResult/>`))
	}))
	defer srv.Close()

	c := testClient(t, srv.URL)
	// offset=20 with limit=10 -> page=3
	_, err := c.DomainsCollection().ExecuteQueryToRecordsReader(context.Background(), mockQuery{limit: 10, offset: 20})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotQuery.Get("PageSize") != "10" {
		t.Errorf("PageSize: want 10, got %q", gotQuery.Get("PageSize"))
	}
	if gotQuery.Get("Page") != "3" {
		t.Errorf("Page: want 3, got %q", gotQuery.Get("Page"))
	}
}

func TestDomainsCollection_List_DefaultPageSize(t *testing.T) {
	var gotQuery url.Values
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.Query()
		fmt.Fprint(w, successXML(`<DomainGetListResult/>`))
	}))
	defer srv.Close()

	c := testClient(t, srv.URL)
	_, err := c.DomainsCollection().ExecuteQueryToRecordsReader(context.Background(), mockQuery{limit: 0})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotQuery.Get("PageSize") != "20" {
		t.Errorf("default PageSize: want 20, got %q", gotQuery.Get("PageSize"))
	}
}

func TestDomainsCollection_List_NonMultipleOffset(t *testing.T) {
	c, _ := New(WithAPIUser("u"), WithAPIKey("k"), WithClientIP("1.2.3.4"))
	_, err := c.DomainsCollection().ExecuteQueryToRecordsReader(context.Background(), mockQuery{limit: 10, offset: 7})
	if err == nil || !strings.Contains(err.Error(), "multiple") {
		t.Errorf("expected multiple-of error, got %v", err)
	}
}

func TestDomainsCollection_List_LimitOver100(t *testing.T) {
	c, _ := New(WithAPIUser("u"), WithAPIKey("k"), WithClientIP("1.2.3.4"))
	_, err := c.DomainsCollection().ExecuteQueryToRecordsReader(context.Background(), mockQuery{limit: 101})
	if err == nil || !strings.Contains(err.Error(), "100") {
		t.Errorf("expected limit-100 error, got %v", err)
	}
}

// ---------- Insert / InsertMulti ----------

func TestDomainsCollection_Insert_NotImplemented(t *testing.T) {
	c, _ := New(WithAPIUser("u"), WithAPIKey("k"), WithClientIP("1.2.3.4"))
	err := c.DomainsCollection().Insert(context.Background(), dal.NewRecordWithData(dal.NewKeyWithID("domains", "x.com"), &DomainInfo{}))
	if !errors.Is(err, dal.ErrNotImplementedYet) {
		t.Errorf("expected ErrNotImplementedYet, got %v", err)
	}
}

func TestDomainsCollection_InsertMulti_NotImplemented(t *testing.T) {
	c, _ := New(WithAPIUser("u"), WithAPIKey("k"), WithClientIP("1.2.3.4"))
	err := c.DomainsCollection().InsertMulti(context.Background(), nil)
	if !errors.Is(err, dal.ErrNotImplementedYet) {
		t.Errorf("expected ErrNotImplementedYet, got %v", err)
	}
}

// ---------- XML edge cases ----------

func TestDoRequest_MalformedXML(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "<this is not valid xml <<>>")
	}))
	defer srv.Close()

	c := testClient(t, srv.URL)
	rec := dal.NewRecordWithData(dal.NewKeyWithID("domains", "example.com"), &DomainInfo{})
	err := c.DomainsCollection().Get(context.Background(), rec)
	if err == nil {
		t.Error("expected error for malformed XML")
	}
}

func TestDoRequest_EmptyBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// write empty body
	}))
	defer srv.Close()

	c := testClient(t, srv.URL)
	rec := dal.NewRecordWithData(dal.NewKeyWithID("domains", "example.com"), &DomainInfo{})
	err := c.DomainsCollection().Get(context.Background(), rec)
	if err == nil {
		t.Error("expected error for empty body")
	}
}

// ---------- Error mapping ----------

func TestMapAPIError_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, errorXML(2019166, "Domain not found"))
	}))
	defer srv.Close()

	c := testClient(t, srv.URL)
	rec := dal.NewRecordWithData(dal.NewKeyWithID("domains", "example.com"), &DomainInfo{})
	err := c.DomainsCollection().Get(context.Background(), rec)
	if !dal.IsNotFound(err) {
		t.Errorf("expected IsNotFound, got %v", err)
	}
}

func TestMapAPIError_RateLimited(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, errorXML(2030280, "Too many requests"))
	}))
	defer srv.Close()

	c := testClient(t, srv.URL)
	rec := dal.NewRecordWithData(dal.NewKeyWithID("domains", "example.com"), &DomainInfo{})
	err := c.DomainsCollection().Get(context.Background(), rec)
	if !errors.Is(err, ErrRateLimited) {
		t.Errorf("expected ErrRateLimited, got %v", err)
	}
}

func TestMapAPIError_Unknown(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, errorXML(9999, "Something went wrong"))
	}))
	defer srv.Close()

	c := testClient(t, srv.URL)
	rec := dal.NewRecordWithData(dal.NewKeyWithID("domains", "example.com"), &DomainInfo{})
	err := c.DomainsCollection().Get(context.Background(), rec)

	var apiErr APIError
	if !errors.As(err, &apiErr) {
		t.Errorf("expected APIError, got %T: %v", err, err)
	}
	if apiErr.Code != 9999 {
		t.Errorf("Code: want 9999, got %d", apiErr.Code)
	}
}
