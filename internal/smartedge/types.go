package smartedge

import "errors"

// ErrInvalidDeviceID 无效的设备ID.
var ErrInvalidDeviceID = errors.New("invalid device ID")

// ErrDeviceNotFound 设备未找到.
var ErrDeviceNotFound = errors.New("device not found")

// ErrInvalidRuleID 无效的规则ID.
var ErrInvalidRuleID = errors.New("invalid rule ID")

// ErrInvalidPipelineID 无效的管道ID.
var ErrInvalidPipelineID = errors.New("invalid pipeline ID")
