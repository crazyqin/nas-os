package smartredundancy

import "errors"

var (
	ErrInvalidNodeID     = errors.New("invalid node ID")
	ErrNodeNotFound      = errors.New("node not found")
	ErrInsufficientNodes = errors.New("insufficient online nodes")
	ErrNoAvailableTarget = errors.New("no available failover target")
)
