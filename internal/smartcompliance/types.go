package smartcompliance

import "errors"

var (
	ErrInvalidRuleID   = errors.New("invalid rule ID")
	ErrInvalidPolicyID = errors.New("invalid policy ID")
)
