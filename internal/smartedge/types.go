package smartedge

import "errors"

var (
	ErrInvalidDeviceID   = errors.New("invalid device ID")
	ErrDeviceNotFound    = errors.New("device not found")
	ErrInvalidRuleID     = errors.New("invalid rule ID")
	ErrInvalidPipelineID = errors.New("invalid pipeline ID")
)
