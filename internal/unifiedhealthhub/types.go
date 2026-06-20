package unifiedhealthhub

import "errors"

var (
	ErrSubsystemNotFound    = errors.New("subsystem not found")
	ErrSubsystemExists      = errors.New("subsystem already exists")
	ErrAlertNotFound        = errors.New("alert not found")
	ErrAlertAlreadyAcked    = errors.New("alert already acknowledged")
	ErrAlertAlreadyResolved = errors.New("alert already resolved")
	ErrIncidentNotFound     = errors.New("incident not found")
	ErrIncidentAlreadyClosed = errors.New("incident already closed")
	ErrRuleNotFound         = errors.New("health rule not found")
	ErrInvalidCheckType     = errors.New("invalid check type")
	ErrHubNotRunning        = errors.New("health hub is not running")
	ErrPredictionDisabled   = errors.New("prediction is disabled")
	ErrInvalidConfig        = errors.New("invalid configuration")
)
