package namecheap

import (
	"io"

	"github.com/dal-go/dalgo/dal"
)

// domainRecordsReader implements dal.RecordsReader for domain list results.
type domainRecordsReader struct {
	domains []*DomainInfo
	idx     int
}

var _ dal.RecordsReader = (*domainRecordsReader)(nil)

func newDomainRecordsReader(domains []*DomainInfo) *domainRecordsReader {
	return &domainRecordsReader{domains: domains}
}

// Next returns the next DomainInfo record, or (nil, io.EOF) when exhausted.
func (r *domainRecordsReader) Next() (dal.Record, error) {
	if r.idx >= len(r.domains) {
		return nil, io.EOF
	}
	info := r.domains[r.idx]
	r.idx++
	key := dal.NewKeyWithID("domains", info.DomainName)
	record := dal.NewRecordWithData(key, info)
	record.SetError(nil)
	return record, nil
}

// Cursor is not supported for NameCheap list results.
func (r *domainRecordsReader) Cursor() (string, error) {
	return "", nil
}

// Close is a no-op for in-memory readers.
func (r *domainRecordsReader) Close() error {
	return nil
}
