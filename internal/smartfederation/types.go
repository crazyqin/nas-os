package smartfederation

import "errors"

// ErrInvalidClusterID 无效的集群ID.
var ErrInvalidClusterID = errors.New("invalid cluster ID")

// ErrInvalidPolicyID 无效的策略ID.
var ErrInvalidPolicyID = errors.New("invalid policy ID")

// ErrClusterNotFound 集群未找到.
var ErrClusterNotFound = errors.New("cluster not found")
