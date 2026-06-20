package smartredundancy

import "errors"

// ErrInvalidNodeID 无效的节点ID.
var ErrInvalidNodeID = errors.New("invalid node ID")

// ErrNodeNotFound 节点未找到.
var ErrNodeNotFound = errors.New("node not found")

// ErrInsufficientNodes 在线节点不足.
var ErrInsufficientNodes = errors.New("insufficient online nodes")

// ErrNoAvailableTarget 无可用故障转移目标.
var ErrNoAvailableTarget = errors.New("no available failover target")
