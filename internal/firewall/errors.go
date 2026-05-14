package firewall

import "errors"

var (
	ErrRuleNotFound     = errors.New("firewall: rule not found")
	ErrMaxRulesReached  = errors.New("firewall: max rules limit reached")
	ErrInvalidRule      = errors.New("firewall: invalid rule")
	ErrInvalidCIDR      = errors.New("firewall: invalid CIDR")
	ErrFirewallDisabled = errors.New("firewall: firewall is disabled")
)
