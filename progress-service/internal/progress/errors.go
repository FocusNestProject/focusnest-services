package progress

import "errors"

var (
	// ErrMissingUserID indicates a required user id was absent.
	ErrMissingUserID = errors.New("user id is required")
	// ErrInvalidSummaryRange indicates an unsupported summary range.
	ErrInvalidSummaryRange = errors.New("invalid summary range")
)
