package nids

import "errors"

var (
	ErrRuleNotFound       = errors.New("nids: rule not found")
	ErrMaxRulesReached    = errors.New("nids: max rules limit reached")
	ErrInvalidRule        = errors.New("nids: invalid rule")
	ErrInvalidCIDR        = errors.New("nids: invalid CIDR")
	ErrAlertNotFound      = errors.New("nids: alert not found")
	ErrNIDSDisabled       = errors.New("nids: NIDS is disabled")
	ErrIPInBlacklist      = errors.New("nids: IP is in blacklist")
	ErrIPInWhitelist      = errors.New("nids: IP is in whitelist")
	ErrForensicNotFound   = errors.New("nids: forensic record not found")
	ErrInvalidThreshold   = errors.New("nids: invalid threshold config")
	ErrDetectorNotReady   = errors.New("nids: detector not ready")
)
