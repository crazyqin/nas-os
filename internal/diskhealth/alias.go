// Package diskhealth 提供 SMART 磁盘健康监测和故障预测功能
package diskhealth

import (
	"nas-os/internal/monitor"
)

// DiskHealthMonitor 磁盘健康监控器类型别名
// 注意: 不能为别名类型定义方法，使用 SmartReader 包装

// SmartReader SMART 数据读取器（独立结构体，可定义方法）
type SmartReader struct{}

// DiskHealthMonitorAlias 保留别名供其他模块使用
type DiskHealthMonitorAlias = monitor.DiskHealthMonitor
