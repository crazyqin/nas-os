package ransomware_honeypot

import (
	"sync"
	"time"
)

// HoneypotState 蜜罐状态.
type HoneypotState string

const (
	StateActive   HoneypotState = "active"
	StateTriggered HoneypotState = "triggered"
	StateDisabled HoneypotState = "disabled"
)

// FileType 诱饵文件类型.
type FileType string

const (
	FileTypeOffice FileType = "office"
	FileTypePDF    FileType = "pdf"
	FileTypeImage  FileType = "image"
	FileTypeText   FileType = "text"
)

// Honeypot 蜜罐实例.
type Honeypot struct {
	ID          string        `json:"id"`
	Name        string        `json:"name"`
	SharePath   string        `json:"share_path"`
	State       HoneypotState `json:"state"`
	FileCount   int           `json:"file_count"`
	CreatedAt   time.Time     `json:"created_at"`
	UpdatedAt   time.Time     `json:"updated_at"`
	TriggeredAt *time.Time    `json:"triggered_at,omitempty"`
}

// DecoyFile 诱饵文件.
type DecoyFile struct {
	ID          string    `json:"id"`
	HoneypotID  string    `json:"honeypot_id"`
	FilePath    string    `json:"file_path"`
	FileType    FileType  `json:"file_type"`
	SizeBytes   int64     `json:"size_bytes"`
	Entropy     float64   `json:"entropy"`
	Hash        string    `json:"hash"`
	CreatedAt   time.Time `json:"created_at"`
}

// AlertLevel 告警级别.
type AlertLevel string

const (
	AlertLevelInfo     AlertLevel = "info"
	AlertLevelWarning  AlertLevel = "warning"
	AlertLevelCritical AlertLevel = "critical"
)

// AlertType 告警类型.
type AlertType string

const (
	AlertTypeEntropyChange   AlertType = "entropy_change"
	AlertTypeMassRename      AlertType = "mass_rename"
	AlertTypeFileAccess      AlertType = "file_access"
	AlertTypeExtensionChange AlertType = "extension_change"
)

// Alert 告警信息.
type Alert struct {
	ID          string      `json:"id"`
	HoneypotID  string      `json:"honeypot_id"`
	Level       AlertLevel  `json:"level"`
	Type        AlertType   `json:"type"`
	Message     string      `json:"message"`
	FilePath    string      `json:"file_path,omitempty"`
	OldValue    string      `json:"old_value,omitempty"`
	NewValue    string      `json:"new_value,omitempty"`
	Responded   bool        `json:"responded"`
	Response    string      `json:"response,omitempty"`
	CreatedAt   time.Time   `json:"created_at"`
	RespondedAt *time.Time  `json:"responded_at,omitempty"`
}

// ScanResult 扫描结果.
type ScanResult struct {
	HoneypotID    string    `json:"honeypot_id"`
	FilesScanned  int       `json:"files_scanned"`
	AlertsRaised  int       `json:"alerts_raised"`
	EntropyChanges int      `json:"entropy_changes"`
	RenameEvents  int       `json:"rename_events"`
	ScanDuration  string    `json:"scan_duration"`
	ScannedAt     time.Time `json:"scanned_at"`
}

// ResponseAction 响应动作.
type ResponseAction string

const (
	ActionIsolate  ResponseAction = "isolate"
	ActionQuarantine ResponseAction = "quarantine"
	ActionIgnore   ResponseAction = "ignore"
	ActionRestore  ResponseAction = "restore"
)

// AlertResponse 告警响应请求.
type AlertResponse struct {
	Action  ResponseAction `json:"action"`
	Comment string         `json:"comment,omitempty"`
}

// CreateHoneypotRequest 创建蜜罐请求.
type CreateHoneypotRequest struct {
	Name      string `json:"name"`
	SharePath string `json:"share_path"`
	FileTypes []FileType `json:"file_types,omitempty"`
}

// DetectionThresholds 检测阈值配置.
type DetectionThresholds struct {
	EntropyChangeThreshold float64 `json:"entropy_change_threshold"`
	MassRenameThreshold    int     `json:"mass_rename_threshold"`
	MassRenameWindowSec    int     `json:"mass_rename_window_sec"`
	AccessFrequencyLimit   int     `json:"access_frequency_limit"`
	AccessFrequencyWindowSec int   `json:"access_frequency_window_sec"`
}

// DefaultThresholds 默认检测阈值.
func DefaultThresholds() DetectionThresholds {
	return DetectionThresholds{
		EntropyChangeThreshold: 1.5,
		MassRenameThreshold:    5,
		MassRenameWindowSec:    60,
		AccessFrequencyLimit:   10,
		AccessFrequencyWindowSec: 30,
	}
}

// AccessEvent 文件访问事件.
type AccessEvent struct {
	FilePath  string    `json:"file_path"`
	EventType string    `json:"event_type"` // read, write, rename, delete
	Timestamp time.Time `json:"timestamp"`
}

// DetectorState 检测器内部状态（线程安全）.
type DetectorState struct {
	mu           sync.RWMutex
	recentEvents []AccessEvent
	renameCount  int
	renameWindow time.Time
}
