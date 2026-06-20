package containerresourcegov

import "errors"

// 容器相关错误
var (
	ErrInvalidContainer      = errors.New("invalid container")
	ErrContainerIDRequired   = errors.New("container ID is required")
	ErrContainerAlreadyExists = errors.New("container already exists")
	ErrContainerNotFound     = errors.New("container not found")
)

// 配置文件相关错误
var (
	ErrInvalidProfile       = errors.New("invalid profile")
	ErrProfileIDRequired    = errors.New("profile ID is required")
	ErrProfileAlreadyExists = errors.New("profile already exists")
	ErrProfileNotFound      = errors.New("profile not found")
)

// 策略相关错误
var (
	ErrInvalidPolicy       = errors.New("invalid policy")
	ErrPolicyIDRequired    = errors.New("policy ID is required")
	ErrPolicyAlreadyExists = errors.New("policy already exists")
	ErrPolicyNotFound      = errors.New("policy not found")
)

// 资源相关错误
var (
	ErrInsufficientData   = errors.New("insufficient data for prediction")
	ErrResourceLimitExceeded = errors.New("resource limit exceeded")
	ErrInvalidMetric      = errors.New("invalid metric name")
)

// 操作相关错误
var (
	ErrRemediationFailed = errors.New("remediation action failed")
	ErrDryRunMode        = errors.New("operation not allowed in dry run mode")
)
