// Package sysmigrate 提供系统迁移向导功能
// 引导用户完成 NAS 系统从源平台到当前平台的完整迁移流程
package sysmigrate

import (
	"time"
)

// ========== 迁移状态定义 ==========

// MigrationPhase 迁移阶段.
type MigrationPhase string

const (
	PhaseAssess   MigrationPhase = "assess"   // 评估中
	PhasePlan     MigrationPhase = "plan"     // 计划中
	PhaseExecute  MigrationPhase = "execute"  // 执行中
	PhaseRollback MigrationPhase = "rollback" // 回滚中
	PhaseDone     MigrationPhase = "done"     // 完成
	PhaseFailed   MigrationPhase = "failed"   // 失败
)

// MigrationStepStatus 单个步骤状态.
type MigrationStepStatus string

const (
	StepPending   MigrationStepStatus = "pending"
	StepRunning   MigrationStepStatus = "running"
	StepCompleted MigrationStepStatus = "completed"
	StepFailed    MigrationStepStatus = "failed"
	StepSkipped   MigrationStepStatus = "skipped"
)

// ========== 源系统类型 ==========

// SourceSystemType 源 NAS 系统类型.
type SourceSystemType string

const (
	SourceSynology SourceSystemType = "synology"
	SourceQNAP     SourceSystemType = "qnap"
	SourceTrueNAS  SourceSystemType = "truenas"
	SourceUnraid   SourceSystemType = "unraid"
	SourceGeneric  SourceSystemType = "generic"
)

// ========== 迁移类别 ==========

// MigrationCategory 迁移数据类别.
type MigrationCategory string

const (
	CategoryData       MigrationCategory = "data"       // 用户数据
	CategoryConfig     MigrationCategory = "config"     // 系统配置
	CategoryUsers      MigrationCategory = "users"      // 用户和权限
	CategoryServices   MigrationCategory = "services"   // 服务和应用
	CategoryNetwork    MigrationCategory = "network"    // 网络配置
	CategoryShared     MigrationCategory = "shared"     // 共享文件夹
	CategoryCert       MigrationCategory = "certificates" // 证书
	CategorySchedule   MigrationCategory = "schedule"   // 计划任务
)

// ========== 核心请求/响应类型 ==========

// AssessRequest 迁移评估请求.
type AssessRequest struct {
	SourceType SourceSystemType `json:"sourceType" binding:"required"`
	SourceHost string           `json:"sourceHost" binding:"required"`
	SourcePort int              `json:"sourcePort" binding:"omitempty,min=1,max=65535"`
	SourceUser string           `json:"sourceUser" binding:"required"`
	SourcePass string           `json:"sourcePass"`
	TargetPath string           `json:"targetPath" binding:"required"`
}

// AssessResult 迁移评估结果.
type AssessResult struct {
	TaskID            string             `json:"taskId"`
	Compatible        bool               `json:"compatible"`
	SourceInfo        *SourceSystemInfo  `json:"sourceInfo"`
	Warnings          []string           `json:"warnings,omitempty"`
	Blockers          []string           `json:"blockers,omitempty"`
	EstimatedDuration string             `json:"estimatedDuration"`
	EstimatedDataSize int64              `json:"estimatedDataSize"` // 字节
	AssessedAt        time.Time          `json:"assessedAt"`
}

// SourceSystemInfo 源系统信息.
type SourceSystemInfo struct {
	Type         SourceSystemType `json:"type"`
	Version      string           `json:"version"`
	Hostname     string           `json:"hostname"`
	TotalStorage int64            `json:"totalStorage"` // 字节
	UsedStorage  int64            `json:"usedStorage"`  // 字节
	UserCount    int              `json:"userCount"`
	ShareCount   int              `json:"shareCount"`
	ServiceCount int              `json:"serviceCount"`
}

// PlanRequest 迁移计划请求.
type PlanRequest struct {
	TaskID     string             `json:"taskId" binding:"required"`
	Categories []MigrationCategory `json:"categories" binding:"required,min=1"`
}

// PlanResult 迁移计划结果.
type PlanResult struct {
	TaskID      string           `json:"taskId"`
	Steps       []MigrationStep  `json:"steps"`
	Timeline    string           `json:"timeline"`    // 预计时间线描述
	TotalSteps  int              `json:"totalSteps"`
	CreatedAt   time.Time        `json:"createdAt"`
}

// MigrationStep 迁移步骤.
type MigrationStep struct {
	ID         string             `json:"id"`
	Order      int                `json:"order"`
	Category   MigrationCategory  `json:"category"`
	Name       string             `json:"name"`
	Status     MigrationStepStatus `json:"status"`
	StartedAt  time.Time          `json:"startedAt,omitempty"`
	FinishedAt time.Time          `json:"finishedAt,omitempty"`
	Error      string             `json:"error,omitempty"`
}

// ExecuteRequest 迁移执行请求.
type ExecuteRequest struct {
	TaskID    string `json:"taskId" binding:"required"`
	DryRun    bool   `json:"dryRun"`
	SkipSteps []string `json:"skipSteps,omitempty"`
}

// ExecuteResult 迁移执行结果.
type ExecuteResult struct {
	TaskID     string            `json:"taskId"`
	Phase      MigrationPhase    `json:"phase"`
	Progress   float64           `json:"progress"` // 0-100
	Steps      []MigrationStep   `json:"steps"`
	StartedAt  time.Time         `json:"startedAt"`
	FinishedAt time.Time         `json:"finishedAt,omitempty"`
	Error      string            `json:"error,omitempty"`
}

// RollbackRequest 回滚请求.
type RollbackRequest struct {
	TaskID   string `json:"taskId" binding:"required"`
	StepID   string `json:"stepId"` // 回滚到指定步骤，为空则全部回滚
}

// RollbackResult 回滚结果.
type RollbackResult struct {
	TaskID     string         `json:"taskId"`
	Success    bool           `json:"success"`
	Steps      []MigrationStep `json:"steps"`
	Message    string         `json:"message"`
	RolledAt   time.Time      `json:"rolledAt"`
}

// MigrationStatus 迁移整体状态.
type MigrationStatus struct {
	TaskID       string          `json:"taskId"`
	Phase        MigrationPhase  `json:"phase"`
	Progress     float64        `json:"progress"`
	CurrentStep  *MigrationStep `json:"currentStep,omitempty"`
	Steps        []MigrationStep `json:"steps"`
	CreatedAt    time.Time       `json:"createdAt"`
	UpdatedAt    time.Time       `json:"updatedAt"`
	Error        string          `json:"error,omitempty"`
}

// ========== 内部任务模型 ==========

// migrationTask 内部迁移任务状态.
type migrationTask struct {
	id          string
	phase       MigrationPhase
	sourceInfo  *SourceSystemInfo
	steps       []MigrationStep
	currentIdx  int
	createdAt   time.Time
	updatedAt   time.Time
	backupPath  string
	error       string
}
