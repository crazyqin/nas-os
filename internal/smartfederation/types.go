package smartfederation

import "errors"

var (
	ErrInvalidClusterID = errors.New("invalid cluster ID")
	ErrInvalidPolicyID  = errors.New("invalid policy ID")
	ErrClusterNotFound  = errors.New("cluster not found")
)
