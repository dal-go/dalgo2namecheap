package namecheap

import (
	"errors"
	"fmt"

	"github.com/dal-go/dalgo/dal"
)

// ErrRateLimited is returned when the NameCheap API rate limit is exceeded (error code 2030280).
var ErrRateLimited = errors.New("namecheap: rate limit exceeded")

// APIError represents a NameCheap API error with a numeric code.
type APIError struct {
	Code    int
	Message string
}

func (e APIError) Error() string {
	return fmt.Sprintf("namecheap: API error %d: %s", e.Code, e.Message)
}

// mapAPIError maps a NameCheap error code and message to the appropriate error type.
func mapAPIError(code int, message string) error {
	switch code {
	case 2019166:
		return fmt.Errorf("%w: %s", dal.ErrRecordNotFound, message)
	case 2030280:
		return ErrRateLimited
	default:
		return APIError{Code: code, Message: message}
	}
}
