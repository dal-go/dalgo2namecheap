package namecheap

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dal-go/dalgo/dal"
	"github.com/dal-go/record"
)

func TestCoverage_Errors(t *testing.T) {
	err := mapAPIError(99999, "custom message")
	apiErr, ok := err.(APIError)
	if !ok {
		t.Fatalf("expected APIError, got %T", err)
	}
	if apiErr.Error() != "namecheap: API error 99999: custom message" {
		t.Errorf("unexpected Error() string: %s", apiErr.Error())
	}
}

func TestCoverage_DomainsReader(t *testing.T) {
	r := newDomainRecordsReader(nil)
	cur, err := r.Cursor()
	if cur != "" || err != nil {
		t.Errorf("Cursor() = (%q, %v), want (\"\", nil)", cur, err)
	}
}

func TestCoverage_Domains_Exists(t *testing.T) {
	t.Run("exists_true", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = fmt.Fprint(w, successXML(`<DomainGetInfoResult DomainName="example.com"/>`))
		}))
		defer srv.Close()

		c := testClient(t, srv.URL)
		exists, err := c.DomainsCollection().Exists(context.Background(), record.NewKeyWithID("domains", "example.com"))
		if err != nil || !exists {
			t.Errorf("Exists = (%v, %v), want (true, nil)", exists, err)
		}
	})

	t.Run("exists_false_not_found", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = fmt.Fprint(w, errorXML(2019166, "Domain not found"))
		}))
		defer srv.Close()

		c := testClient(t, srv.URL)
		exists, err := c.DomainsCollection().Exists(context.Background(), record.NewKeyWithID("domains", "notfound.com"))
		if err != nil || exists {
			t.Errorf("Exists = (%v, %v), want (false, nil)", exists, err)
		}
	})

	t.Run("exists_error", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = fmt.Fprint(w, errorXML(999, "internal error"))
		}))
		defer srv.Close()

		c := testClient(t, srv.URL)
		exists, err := c.DomainsCollection().Exists(context.Background(), record.NewKeyWithID("domains", "error.com"))
		if err == nil || exists {
			t.Errorf("Exists = (%v, %v), want (false, error)", exists, err)
		}
	})
}

func TestCoverage_Domains_Misc(t *testing.T) {
	c := testClient(t, "http://localhost")
	_, err := c.DomainsCollection().ExecuteQueryToRecordsetReader(context.Background(), nil)
	if !errors.Is(err, dal.ErrNotImplementedYet) {
		t.Errorf("expected ErrNotImplementedYet, got %v", err)
	}

	t.Run("get_missing_info_result", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = fmt.Fprint(w, successXML(""))
		}))
		defer srv.Close()

		client := testClient(t, srv.URL)
		rec := record.NewRecordWithData(record.NewKeyWithID("domains", "example.com"), &DomainInfo{})
		err := client.DomainsCollection().Get(context.Background(), rec)
		if err == nil || !strings.Contains(err.Error(), "missing DomainGetInfoResult") {
			t.Errorf("expected missing DomainGetInfoResult error, got %v", err)
		}
	})

	t.Run("get_invalid_expiry_date", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = fmt.Fprint(w, successXML(`<DomainGetInfoResult DomainName="example.com">
				<DomainDetails><ExpiredDate>invalid-date-format</ExpiredDate></DomainDetails>
			</DomainGetInfoResult>`))
		}))
		defer srv.Close()

		client := testClient(t, srv.URL)
		rec := record.NewRecordWithData(record.NewKeyWithID("domains", "example.com"), &DomainInfo{})
		err := client.DomainsCollection().Get(context.Background(), rec)
		if err == nil || !strings.Contains(err.Error(), "failed to parse domain expiry date") {
			t.Errorf("expected parse error, got %v", err)
		}
	})

	t.Run("list_missing_result", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = fmt.Fprint(w, successXML(""))
		}))
		defer srv.Close()

		client := testClient(t, srv.URL)
		q := mockQuery{limit: 10, offset: 0}
		reader, err := client.DomainsCollection().ExecuteQueryToRecordsReader(context.Background(), q)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		rec, err := reader.Next()
		if rec != nil || !errors.Is(err, dal.ErrNoMoreRecords) && err.Error() != "EOF" {
			t.Errorf("expected EOF, got (%v, %v)", rec, err)
		}
	})

	t.Run("list_invalid_expiry_date", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = fmt.Fprint(w, successXML(`<DomainGetListResult>
				<Domain Name="example.com" Expires="bad-date"/>
			</DomainGetListResult>`))
		}))
		defer srv.Close()

		client := testClient(t, srv.URL)
		q := mockQuery{limit: 10, offset: 0}
		_, err := client.DomainsCollection().ExecuteQueryToRecordsReader(context.Background(), q)
		if err == nil || !strings.Contains(err.Error(), "failed to parse domain") {
			t.Errorf("expected parse domain error, got %v", err)
		}
	})

	t.Run("list_doRequest_error", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer srv.Close()

		client := testClient(t, srv.URL)
		q := mockQuery{limit: 10, offset: 0}
		_, err := client.DomainsCollection().ExecuteQueryToRecordsReader(context.Background(), q)
		if err == nil {
			t.Fatal("expected request error, got nil")
		}
	})

	t.Run("parseNamecheapDate_invalid", func(t *testing.T) {
		_, err := parseNamecheapDate("totally-invalid")
		if err == nil || !strings.Contains(err.Error(), "unrecognized date format") {
			t.Errorf("expected unrecognized date format, got %v", err)
		}
	})
}

func TestCoverage_DNS(t *testing.T) {
	t.Run("splitDomain_no_dot", func(t *testing.T) {
		sld, tld := splitDomain("localhost")
		if sld != "localhost" || tld != "" {
			t.Errorf("splitDomain(localhost) = (%q, %q), want (localhost, \"\")", sld, tld)
		}
	})

	t.Run("exists_true", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = fmt.Fprint(w, successXML(`<DomainDNSGetHostsResult/>`))
		}))
		defer srv.Close()

		c := testClient(t, srv.URL)
		exists, err := c.DNSHostsCollection().Exists(context.Background(), record.NewKeyWithID("dns", "example.com"))
		if err != nil || !exists {
			t.Errorf("Exists = (%v, %v), want (true, nil)", exists, err)
		}
	})

	t.Run("exists_false_not_found", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = fmt.Fprint(w, errorXML(2019166, "Domain not found"))
		}))
		defer srv.Close()

		c := testClient(t, srv.URL)
		exists, err := c.DNSHostsCollection().Exists(context.Background(), record.NewKeyWithID("dns", "notfound.com"))
		if err != nil || exists {
			t.Errorf("Exists = (%v, %v), want (false, nil)", exists, err)
		}
	})

	t.Run("exists_error", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = fmt.Fprint(w, errorXML(999, "server error"))
		}))
		defer srv.Close()

		c := testClient(t, srv.URL)
		exists, err := c.DNSHostsCollection().Exists(context.Background(), record.NewKeyWithID("dns", "error.com"))
		if err == nil || exists {
			t.Errorf("Exists = (%v, %v), want (false, error)", exists, err)
		}
	})

	t.Run("get_missing_result", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = fmt.Fprint(w, successXML(""))
		}))
		defer srv.Close()

		c := testClient(t, srv.URL)
		rec := record.NewRecordWithData(record.NewKeyWithID("dns", "example.com"), &DNSHosts{})
		err := c.DNSHostsCollection().Get(context.Background(), rec)
		if err == nil || !strings.Contains(err.Error(), "missing DomainDNSGetHostsResult") {
			t.Errorf("expected missing result error, got %v", err)
		}
	})

	t.Run("set_invalid_record_data", func(t *testing.T) {
		c := testClient(t, "http://localhost")
		rec := record.NewRecordWithData(record.NewKeyWithID("dns", "example.com"), "not-a-dns-hosts")
		err := c.DNSHostsCollection().Set(context.Background(), rec)
		if err == nil || !strings.Contains(err.Error(), "record data must be *DNSHosts") {
			t.Errorf("expected *DNSHosts error, got %v", err)
		}
	})

	t.Run("set_mx_and_non_mx", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			q := r.URL.Query()
			if q.Get("MXPref1") != "20" {
				t.Errorf("MXPref1 = %q, want 20", q.Get("MXPref1"))
			}
			if q.Get("MXPref2") != "10" {
				t.Errorf("MXPref2 = %q, want 10", q.Get("MXPref2"))
			}
			_, _ = fmt.Fprint(w, successXML(`<DomainDNSSetHostsResult IsSuccess="true"/>`))
		}))
		defer srv.Close()

		c := testClient(t, srv.URL)
		data := &DNSHosts{
			Hosts: []HostRecord{
				{HostName: "@", RecordType: "MX", Address: "mail.example.com.", TTL: "1800", MXPref: 20},
				{HostName: "www", RecordType: "CNAME", Address: "example.com.", TTL: "1800"},
			},
		}
		rec := record.NewRecordWithData(record.NewKeyWithID("dns", "example.com"), data)
		if err := c.DNSHostsCollection().Set(context.Background(), rec); err != nil {
			t.Fatalf("Set failed: %v", err)
		}
	})
}

func TestCoverage_Client_Options(t *testing.T) {
	customErr := errors.New("custom option error")
	opt := func(cfg *config, hasClientIP *bool, hasAutodetect *bool, ipDetectionURL *string) error {
		return customErr
	}
	_, err := New(opt)
	if !errors.Is(err, customErr) {
		t.Errorf("expected customErr, got %v", err)
	}

	t.Run("detectClientIP_read_error", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Length", "500")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("short"))
		}))
		defer srv.Close()

		_, err := detectClientIP(srv.URL)
		if err == nil || !strings.Contains(err.Error(), "failed to read IP detection response") {
			t.Errorf("expected read IP response error, got %v", err)
		}
	})
}

func TestCoverage_ConfigEnv(t *testing.T) {
	t.Run("readNamecheapAPIFile_read_error", func(t *testing.T) {
		tmpDir := t.TempDir()
		t.Setenv("HOME", tmpDir)
		t.Setenv("NAMECHEAP_API_USER", "")
		t.Setenv("NAMECHEAP_API_KEY", "")

		// Create a directory named .namecheap-api so opening/reading fails
		credPath := filepath.Join(tmpDir, namecheapAPIFile)
		if err := os.Mkdir(credPath, 0000); err != nil {
			t.Fatal(err)
		}

		_, err := ConfigFromEnv()
		if err == nil || !strings.Contains(err.Error(), "failed to read credentials file") {
			t.Errorf("expected read credentials file error, got %v", err)
		}
	})

	t.Run("readNamecheapAPIFile_invalid_lines", func(t *testing.T) {
		tmpDir := t.TempDir()
		t.Setenv("HOME", tmpDir)
		t.Setenv("NAMECHEAP_API_USER", "")
		t.Setenv("NAMECHEAP_API_KEY", "")

		content := `# Comment line
LINE_WITHOUT_EQUALS
NAMECHEAP_API_USER="testuser"
NAMECHEAP_API_KEY="testkey"
`
		if err := os.WriteFile(filepath.Join(tmpDir, namecheapAPIFile), []byte(content), 0600); err != nil {
			t.Fatal(err)
		}

		opts, err := ConfigFromEnv()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(opts) != 2 {
			t.Errorf("expected 2 opts, got %d", len(opts))
		}
	})

	t.Run("readNamecheapAPIFile_home_dir_error", func(t *testing.T) {
		t.Setenv("HOME", "")
		t.Setenv("NAMECHEAP_API_USER", "")
		t.Setenv("NAMECHEAP_API_KEY", "")

		_, err := readNamecheapAPIFile()
		if err == nil || !strings.Contains(err.Error(), "failed to get home directory") {
			t.Errorf("expected home directory error, got %v", err)
		}
	})

	t.Run("apiKey_missing", func(t *testing.T) {
		tmpDir := t.TempDir()
		t.Setenv("HOME", tmpDir)
		t.Setenv("NAMECHEAP_API_USER", "only-user")
		t.Setenv("NAMECHEAP_API_KEY", "")

		_, err := ConfigFromEnv()
		if err == nil || !strings.Contains(err.Error(), "NAMECHEAP_API_KEY is not set") {
			t.Errorf("expected NAMECHEAP_API_KEY is not set, got %v", err)
		}
	})

	t.Run("scanner_error_too_long", func(t *testing.T) {
		tmpDir := t.TempDir()
		t.Setenv("HOME", tmpDir)
		t.Setenv("NAMECHEAP_API_USER", "")
		t.Setenv("NAMECHEAP_API_KEY", "")

		longLine := strings.Repeat("A", 70000)
		if err := os.WriteFile(filepath.Join(tmpDir, namecheapAPIFile), []byte(longLine), 0600); err != nil {
			t.Fatal(err)
		}

		_, err := ConfigFromEnv()
		if err == nil || !strings.Contains(err.Error(), "error reading credentials file") {
			t.Errorf("expected error reading credentials file, got %v", err)
		}
	})
}

func TestCoverage_Transport(t *testing.T) {
	t.Run("invalid_request_url", func(t *testing.T) {
		c := testClient(t, "http://localhost")
		c.cfg.baseURL = "http://\x7f"
		c.cfg.apiKey = "mysecret"
		_, err := c.doRequest(context.Background(), url.Values{})
		if err == nil {
			t.Fatal("expected request error, got nil")
		}
		if strings.Contains(err.Error(), "mysecret") {
			t.Errorf("apiKey leaked in error: %v", err)
		}
	})

	t.Run("body_read_error", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Length", "500")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("short"))
		}))
		defer srv.Close()

		c := testClient(t, srv.URL)
		_, err := c.doRequest(context.Background(), url.Values{})
		if err == nil || !strings.Contains(err.Error(), "failed to read response body") {
			t.Errorf("expected failed to read response body error, got %v", err)
		}
	})

	t.Run("empty_status", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = fmt.Fprint(w, `<ApiResponse xmlns="https://api.namecheap.com/xml.response"><Errors/></ApiResponse>`)
		}))
		defer srv.Close()

		c := testClient(t, srv.URL)
		_, err := c.doRequest(context.Background(), url.Values{})
		if err == nil || !strings.Contains(err.Error(), "empty or invalid XML response") {
			t.Errorf("expected empty or invalid XML error, got %v", err)
		}
	})

	t.Run("error_status_no_errors", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = fmt.Fprint(w, `<ApiResponse Status="ERROR" xmlns="https://api.namecheap.com/xml.response"><Errors/></ApiResponse>`)
		}))
		defer srv.Close()

		c := testClient(t, srv.URL)
		_, err := c.doRequest(context.Background(), url.Values{})
		if err == nil || !strings.Contains(err.Error(), "API returned ERROR status without error details") {
			t.Errorf("expected no error details error, got %v", err)
		}
	})

	t.Run("sanitizeErr_redaction", func(t *testing.T) {
		c := testClient(t, "http://localhost")
		c.cfg.apiKey = "secret123"
		err := fmt.Errorf("network error containing secret123 in url")
		sErr := c.sanitizeErr(err)
		if strings.Contains(sErr.Error(), "secret123") {
			t.Errorf("failed to redact: %v", sErr)
		}
		if !strings.Contains(sErr.Error(), "[REDACTED]") {
			t.Errorf("expected [REDACTED], got %v", sErr)
		}

		// nil error
		if c.sanitizeErr(nil) != nil {
			t.Error("expected nil for nil err")
		}

		// empty apiKey
		c.cfg.apiKey = ""
		if c.sanitizeErr(err) != err {
			t.Error("expected unchanged error when apiKey is empty")
		}

		// non-matching apiKey
		c.cfg.apiKey = "secret123"
		unrelatedErr := errors.New("unrelated error without key")
		if c.sanitizeErr(unrelatedErr) != unrelatedErr {
			t.Error("expected unchanged error when apiKey is not present in message")
		}
	})
}
