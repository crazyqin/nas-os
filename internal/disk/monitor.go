// Package disk 提供磁盘监控接口定义
package disk

import "time"

// Monitor 定义 SMART 监控器接口
type Monitor interface {
	GetAllDisks() []*DiskInfo
	GetDiskInfo(device string) (*DiskInfo, error)
	GetAlerts(device string, acknowledged bool) []*SMARTAlert
	AcknowledgeAlert(id string) error
	GetAlertRules() []*AlertRule
	SetAlertRule(rule *AlertRule)
	SetScoreWeights(w *ScoreWeights)
	ScanDisks() error
	CheckAllDisks() error
	GetHistory(device string, duration time.Duration) []*SMARTHistoryPoint
	ExportJSON() ([]byte, error)
	ImportJSON(data []byte) error
}
