// Package onboardingwizard 提供开箱引导向导功能
// 对标群晖 DSM 初始设置向导，支持首次使用引导、功能推荐、快速配置模板
package onboardingwizard

import (
	"time"
)

// StepStatus 步骤状态.
type StepStatus string

const (
	StepStatusPending    StepStatus = "pending"
	StepStatusInProgress StepStatus = "in_progress"
	StepStatusCompleted  StepStatus = "completed"
	StepStatusSkipped    StepStatus = "skipped"
)

// StepType 步骤类型.
type StepType string

const (
	StepTypeNetwork      StepType = "network"
	StepTypeStoragePool  StepType = "storage_pool"
	StepTypeUserCreation StepType = "user_creation"
	StepTypeAppInstall   StepType = "app_install"
	StepTypeRecommend    StepType = "recommend"
)

// TemplateType 配置模板类型.
type TemplateType string

const (
	TemplateTypeHome       TemplateType = "home"
	TemplateTypeEnterprise TemplateType = "enterprise"
	TemplateTypeDeveloper  TemplateType = "developer"
)

// Step 引导步骤.
type Step struct {
	ID          string     `json:"id"`
	Type        StepType   `json:"type"`
	Name        string     `json:"name"`
	Description string     `json:"description"`
	Required    bool       `json:"required"`
	Status      StepStatus `json:"status"`
	Order       int        `json:"order"`
	Data        any        `json:"data,omitempty"`
	SkippedAt   *time.Time `json:"skipped_at,omitempty"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
}

// Progress 引导进度.
type Progress struct {
	TotalSteps     int        `json:"total_steps"`
	CompletedSteps int        `json:"completed_steps"`
	SkippedSteps   int        `json:"skipped_steps"`
	CurrentStep    *Step      `json:"current_step,omitempty"`
	Percentage     float64    `json:"percentage"`
	StartedAt      time.Time  `json:"started_at"`
	CompletedAt    *time.Time `json:"completed_at,omitempty"`
	IsCompleted    bool       `json:"is_completed"`
}

// Session 引导会话.
type Session struct {
	ID           string         `json:"id"`
	TemplateType TemplateType   `json:"template_type"`
	Steps        []*Step        `json:"steps"`
	Progress     *Progress      `json:"progress"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
	CompletedAt  *time.Time     `json:"completed_at,omitempty"`
	IsCompleted  bool           `json:"is_completed"`
	CustomData   map[string]any `json:"custom_data,omitempty"`
}

// NetworkConfig 网络配置数据.
type NetworkConfig struct {
	Hostname   string   `json:"hostname"`
	IPAddress  string   `json:"ip_address,omitempty"`
	Netmask    string   `json:"netmask,omitempty"`
	Gateway    string   `json:"gateway,omitempty"`
	DNSServers []string `json:"dns_servers,omitempty"`
	DHCP       bool     `json:"dhcp"`
}

// StoragePoolConfig 存储池配置数据.
type StoragePoolConfig struct {
	Name       string   `json:"name"`
	RAIDType   string   `json:"raid_type"` // single, mirror, raid5, raid6, raid10
	Disks      []string `json:"disks"`
	FileSystem string   `json:"file_system"` // ext4, btrfs, xfs
}

// UserConfig 用户配置数据.
type UserConfig struct {
	Username string `json:"username"`
	Password string `json:"password,omitempty"`
	FullName string `json:"full_name"`
	Email    string `json:"email,omitempty"`
	IsAdmin  bool   `json:"is_admin"`
}

// RecommendedApp 推荐应用.
type RecommendedApp struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Icon        string   `json:"icon"`
	Category    string   `json:"category"`
	Reason      string   `json:"reason"`
	Tags        []string `json:"tags,omitempty"`
}

// Template 配置模板.
type Template struct {
	Type        TemplateType `json:"type"`
	Name        string       `json:"name"`
	Description string       `json:"description"`
	Steps       []StepConfig `json:"steps"`
	Apps        []string     `json:"recommended_apps"`
}

// StepConfig 步骤配置.
type StepConfig struct {
	Type     StepType `json:"type"`
	Required bool     `json:"required"`
	Order    int      `json:"order"`
}

// CreateSessionRequest 创建引导会话请求.
type CreateSessionRequest struct {
	TemplateType TemplateType `json:"template_type" binding:"required"`
}

// CompleteStepRequest 完成步骤请求.
type CompleteStepRequest struct {
	Data any `json:"data,omitempty"`
}

// SkipStepRequest 跳过步骤请求.
type SkipStepRequest struct {
	Reason string `json:"reason,omitempty"`
}

// GetRecommendationsRequest 获取推荐请求.
type GetRecommendationsRequest struct {
	Scenario string `json:"scenario" binding:"required"` // home, office, development, media, backup
}
