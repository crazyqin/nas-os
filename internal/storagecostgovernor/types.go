package storagecostgovernor

import "errors"

// 错误定义
var (
	// ErrPoolNotFound 存储池未找到
	ErrPoolNotFound = errors.New("storage pool not found")

	// ErrPoolAlreadyExists 存储池已存在
	ErrPoolAlreadyExists = errors.New("storage pool already exists")

	// ErrBudgetNotFound 预算未找到
	ErrBudgetNotFound = errors.New("budget not found")

	// ErrInvalidPoolID 无效的存储池ID
	ErrInvalidPoolID = errors.New("invalid pool ID")

	// ErrInvalidBudgetID 无效的预算ID
	ErrInvalidBudgetID = errors.New("invalid budget ID")

	// ErrInsufficientData 数据不足
	ErrInsufficientData = errors.New("insufficient data for calculation")

	// ErrInvalidConfig 无效配置
	ErrInvalidConfig = errors.New("invalid configuration")

	// ErrGovernorStopped 治理引擎已停止
	ErrGovernorStopped = errors.New("governor is stopped")

	// ErrBudgetExceeded 预算超支
	ErrBudgetExceeded = errors.New("budget exceeded")

	// ErrForecastFailed 预测失败
	ErrForecastFailed = errors.New("forecast calculation failed")
)
