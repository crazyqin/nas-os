// Package updatecoord 提供系统更新协调器功能
// 编排 NAS 系统更新的完整流程：检查→预检→下载→备份→安装→验证→切换
package updatecoord

import (
	"time"
)

// ========== 更新阶段定义 ==========

// UpdatePhase 更新阶段.
type UpdatePhase string

const (
	PhaseCheck    UpdatePhase = "check"    // 检查更新
	PhasePreCheck UpdatePhase = "precheck" // 更新前预检
	PhaseDownload UpdatePhase = "download" // 下载更新
	PhaseBackup   UpdatePhase = "backup"   // 备份
	PhaseInstall  UpdatePhase = "install"  // 安装
	PhaseVerify   UpdatePhase = "verify"   // 验证
	PhaseSwitch   UpdatePhase = "switch"   // 切换
	PhaseRollback UpdatePhase = "rollback" // 回滚
	PhaseDone     UpdatePhase = "done"     // 完成
	PhaseFailed   UpdatePhase = "failed"   // 失败
)

// UpdateStepStatus 单个步骤状态.
type UpdateStepStatus string

const (
	StepPending   UpdateStepStatus = "pending"
	StepRunning   UpdateStepStatus = "running"
	StepCompleted UpdateStepStatus = "completed"
	StepFailed    UpdateStepStatus = "failed"
	StepSkipped   UpdateStepStatus = "skipped"
)

// ========== 更新渠道 ==========

// UpdateChannel 更新渠道.
type UpdateChannel string

const (
	ChannelStable UpdateChannel = "stable"
	ChannelBeta   UpdateChannel = "beta"
	ChannelLTS    UpdateChannel = "lts"
	ChannelNightly UpdateChannel = "nightly"
)

// ========== 核心类型 ==========

// UpdateInfo 可用更新信息.
type UpdateInfo struct {
	Version       string        `json:"version"`
	Channel       UpdateChannel `json:"channel"`
	ReleaseNotes  string        `json:"releaseNotes"`
	Size          int64         `json:"size"` // 字节
	ReleasedAt    time.Time     `json:"releasedAt"`
	CriticalLevel string        `json:"criticalLevel"` // none, low, medium, high, critical
	Checksum      string        `json:"checksum"`
	Available     bool          `json:"available"`
}

// PreCheckRequest 更新前预检请求.
type PreCheckRequest struct {
	Version string `json:"version" binding:"required"`
}

// PreCheckResult 预检结果.
type PreCheckResult struct {
	Version      string            `json:"version"`
	Passed       bool              `json:"passed"`
	Checks       []PreCheckItem   `json:"checks"`
	Warnings     []string         `json:"warnings,omitempty"`
	CheckedAt    time.Time        `json:"checkedAt"`
}

// PreCheckItem 单项预检结果.
type PreCheckItem struct {
	Name     string `json:"name"`
	Category string `json:"category"` // disk, service, backup, network, etc.
	Passed   bool   `json:"passed"`
	Message  string `json:"message"`
	Detail   string `json:"detail,omitempty"`
}

// ApplyRequest 应用更新请求.
type ApplyRequest struct {
	Version     string `json:"version" binding:"required"`
	DryRun      bool   `json:"dryRun"`
	SkipBackup  bool   `json:"skipBackup"`
	AutoRollback bool  `json:"autoRollback"`
}

// ApplyResult 应用更新结果.
type ApplyResult struct {
	Version     string           `json:"version"`
	Phase       UpdatePhase      `json:"phase"`
	Progress    float64          `json:"progress"`
	Steps       []UpdateStep     `json:"steps"`
	StartedAt   time.Time        `json:"startedAt"`
	FinishedAt  time.Time        `json:"finishedAt,omitempty"`
	Error       string           `json:"error,omitempty"`
}

// UpdateStep 更新步骤.
type UpdateStep struct {
	ID         string            `json:"id"`
	Order      int               `json:"order"`
	Name       string            `json:"name"`
	Phase      UpdatePhase       `json:"phase"`
	Status     UpdateStepStatus  `json:"status"`
	StartedAt  time.Time         `json:"startedAt,omitempty"`
	FinishedAt time.Time         `json:"finishedAt,omitempty"`
	Error      string            `json:"error,omitempty"`
}

// HistoryEntry 更新历史记录.
type HistoryEntry struct {
	ID         string       `json:"id"`
	Version    string       `json:"version"`
	FromVersion string     `json:"fromVersion"`
	Phase      UpdatePhase  `json:"phase"`
	Success    bool         `json:"success"`
	Steps      []UpdateStep `json:"steps"`
	StartedAt  time.Time    `json:"startedAt"`
	FinishedAt time.Time    `json:"finishedAt,omitempty"`
	Error      string       `json:"error,omitempty"`
}

// RollbackRequest 回滚请求.
type RollbackRequest struct {
	Version    string `json:"version" binding:"required"`
	HistoryID  string `json:"historyId"`
}

// RollbackResult 回滚结果.
type RollbackResult struct {
	Version    string       `json:"version"`
	Success    bool         `json:"success"`
	Steps      []UpdateStep `json:"steps"`
	Message    string       `json:"message"`
	RolledAt   time.Time    `json:"rolledAt"`
}

// ========== 内部任务模型 ==========

// updateTask 内部更新任务状态.
type updateTask struct {
	id           string
	version      string
	fromVersion  string
	phase        UpdatePhase
	steps        []UpdateStep
	currentIdx   int
	createdAt    time.Time
	updatedAt    time.Time
	backupPath   string
	autoRollback bool
	error        string
}
