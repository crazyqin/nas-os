package datalifecycleai

import "errors"

// 策略相关错误
var (
	ErrPolicyNotFound      = errors.New("policy not found")
	ErrPolicyAlreadyExists = errors.New("policy already exists")
	ErrPolicyInvalid       = errors.New("invalid policy")
	ErrPolicyDisabled      = errors.New("policy is disabled")
)

// 规则相关错误
var (
	ErrRuleNotFound      = errors.New("rule not found")
	ErrRuleAlreadyExists = errors.New("rule already exists")
	ErrRuleInvalid       = errors.New("invalid rule")
	ErrRuleDisabled      = errors.New("rule is disabled")
	ErrConditionInvalid  = errors.New("invalid condition")
)

// 资产相关错误
var (
	ErrAssetNotFound      = errors.New("asset not found")
	ErrAssetAlreadyExists = errors.New("asset already exists")
	ErrAssetInvalid       = errors.New("invalid asset")
)

// 操作相关错误
var (
	ErrActionFailed      = errors.New("action failed")
	ErrActionCancelled   = errors.New("action cancelled")
	ErrActionTimeout     = errors.New("action timeout")
	ErrActionNotAllowed  = errors.New("action not allowed")
)

// 引擎相关错误
var (
	ErrEngineNotRunning     = errors.New("engine not running")
	ErrEngineAlreadyRunning = errors.New("engine already running")
	ErrConfigInvalid        = errors.New("invalid configuration")
	ErrConcurrentLimit      = errors.New("concurrent limit reached")
)

// AI相关错误
var (
	ErrModelNotFound   = errors.New("model not found")
	ErrModelInvalid    = errors.New("invalid model")
	ErrDecisionFailed  = errors.New("decision failed")
	ErrInsufficientData = errors.New("insufficient data for decision")
)

// 合规相关错误
var (
	ErrComplianceViolation = errors.New("compliance violation")
	ErrRetentionConflict   = errors.New("retention period conflict")
	ErrEncryptionRequired  = errors.New("encryption required")
)
