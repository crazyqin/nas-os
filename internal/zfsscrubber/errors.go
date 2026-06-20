package zfsscrubber

import "errors"

// 调度相关错误
var (
	// ErrScheduleIDRequired 调度 ID 不能为空
	ErrScheduleIDRequired = errors.New("调度ID不能为空")
	// ErrPoolIDRequired 池 ID 不能为空
	ErrPoolIDRequired = errors.New("池ID不能为空")
	// ErrScheduleExists 调度已存在
	ErrScheduleExists = errors.New("调度已存在")
	// ErrScheduleNotFound 调度未找到
	ErrScheduleNotFound = errors.New("调度未找到")
	// ErrInvalidFrequency 无效的频率
	ErrInvalidFrequency = errors.New("无效的频率，支持: daily, weekly, monthly")
	// ErrInvalidDayOfWeek 无效的星期
	ErrInvalidDayOfWeek = errors.New("无效的星期，范围: 0-6 (0=周日)")
	// ErrInvalidDayOfMonth 无效的日期
	ErrInvalidDayOfMonth = errors.New("无效的日期，范围: 1-31")
	// ErrInvalidHour 无效的小时
	ErrInvalidHour = errors.New("无效的小时，范围: 0-23")
)

// 任务相关错误
var (
	// ErrJobNotFound 任务未找到
	ErrJobNotFound = errors.New("任务未找到")
	// ErrJobNotRunning 任务未运行
	ErrJobNotRunning = errors.New("任务未在运行中")
	// ErrScrubAlreadyRunning 已有运行中的清洗任务
	ErrScrubAlreadyRunning = errors.New("已有运行中的清洗任务")
)

// 报告相关错误
var (
	// ErrReportNotFound 报告未找到
	ErrReportNotFound = errors.New("报告未找到")
)

// 健康监控相关错误
var (
	// ErrDiskNotFound 磁盘未找到
	ErrDiskNotFound = errors.New("磁盘未找到")
	// ErrAlertNotFound 告警未找到
	ErrAlertNotFound = errors.New("告警未找到")
	// ErrRepairActionNotFound 修复动作未找到
	ErrRepairActionNotFound = errors.New("修复动作未找到")
)

// 数据完整性相关错误
var (
	// ErrBlockNotFound 数据块未找到
	ErrBlockNotFound = errors.New("数据块未找到")
	// ErrChecksumMismatch 校验和不匹配
	ErrChecksumMismatch = errors.New("校验和不匹配")
	// ErrRepairFailed 修复失败
	ErrRepairFailed = errors.New("修复失败")
	// ErrNoRedundancy 没有冗余副本
	ErrNoRedundancy = errors.New("没有冗余副本可用于修复")
)
