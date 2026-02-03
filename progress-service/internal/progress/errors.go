package progress

import "errors"

var (
	// ErrMissingUserID indicates a required user id was absent.
	ErrMissingUserID = errors.New("user id is required")
	// ErrInvalidSummaryRange indicates an unsupported summary range.
	ErrInvalidSummaryRange = errors.New("invalid summary range")
	// ErrNotPremium indicates the user is not premium (recovery requires premium).
	ErrNotPremium = errors.New("recovery requires premium")
	// ErrStreakNotRecoverable indicates streak is not in grace/expired state or already recovered.
	ErrStreakNotRecoverable = errors.New("streak is not recoverable")
	// ErrRecoveryQuotaExceeded indicates monthly recovery quota (5) is exceeded.
	ErrRecoveryQuotaExceeded = errors.New("recovery quota exceeded for this month")
)
