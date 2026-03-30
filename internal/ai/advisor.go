// Package ai provides AI Advisor service for NAS-OS
// Inspired by Synology AI Console - intelligent assistant for system queries
package ai

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"
)

// AdvisorConfig AI Advisor 配置
type AdvisorConfig struct {
	// AI 服务配置
	Manager *Manager // AI Manager 实例

	// 系统信息获取接口
	SystemInfoProvider  SystemInfoProvider
	HealthInfoProvider  HealthInfoProvider
	StorageInfoProvider StorageInfoProvider
	ServiceInfoProvider ServiceInfoProvider

	// 配置项
	MaxContextTokens   int     // 最大上下文 token 数
	ResponseMaxTokens  int     // 响应最大 token 数
	Temperature        float64 // 响应创造性
	EnableHistory      bool    // 启用对话历史
	MaxHistoryMessages int     // 最大历史消息数
}

// SystemInfoProvider 系统信息提供者接口
type SystemInfoProvider interface {
	// GetSystemStatus 获取系统状态概览
	GetSystemStatus() (*SystemStatus, error)
	// GetCPUInfo 获取 CPU 信息
	GetCPUInfo() (*CPUInfo, error)
	// GetMemoryInfo 获取内存信息
	GetMemoryInfo() (*MemoryInfo, error)
	// GetNetworkInfo 获取网络信息
	GetNetworkInfo() (*NetworkInfo, error)
	// GetUptime 获取运行时间
	GetUptime() (time.Duration, error)
}

// HealthInfoProvider 健康信息提供者接口
type HealthInfoProvider interface {
	// GetHealthScore 获取健康评分
	GetHealthScore() (*HealthScore, error)
	// GetDiskHealth 获取磁盘健康状态
	GetDiskHealth() ([]DiskHealth, error)
	// GetServiceHealth 获取服务健康状态
	GetServiceHealth() ([]ServiceHealth, error)
	// GetAlerts 获取告警列表
	GetAlerts() ([]Alert, error)
}

// StorageInfoProvider 存储信息提供者接口
type StorageInfoProvider interface {
	// GetStorageOverview 获取存储概览
	GetStorageOverview() (*StorageOverview, error)
	// GetVolumes 获取卷列表
	GetVolumes() ([]VolumeInfo, error)
	// GetPools 获取存储池列表
	GetPools() ([]PoolInfo, error)
	// GetSnapshots 获取快照列表
	GetSnapshots(volume string) ([]SnapshotInfo, error)
}

// ServiceInfoProvider 服务信息提供者接口
type ServiceInfoProvider interface {
	// GetServices 获取服务列表
	GetServices() ([]ServiceInfo, error)
	// GetServiceStatus 获取服务状态
	GetServiceStatus(name string) (*ServiceStatus, error)
	// GetServiceLogs 获取服务日志
	GetServiceLogs(name string, lines int) ([]string, error)
}

// ===== 数据类型定义 =====

// SystemStatus 系统状态概览
type SystemStatus struct {
	CPUUsage    float64   `json:"cpu_usage"`
	MemoryUsage float64   `json:"memory_usage"`
	SwapUsage   float64   `json:"swap_usage"`
	Uptime      string    `json:"uptime"`
	LoadAvg     []float64 `json:"load_avg"`
	Hostname    string    `json:"hostname"`
	Kernel      string    `json:"kernel"`
	TimeStamp   time.Time `json:"timestamp"`
}

// CPUInfo CPU 信息
type CPUInfo struct {
	Model     string  `json:"model"`
	Cores     int     `json:"cores"`
	Threads   int     `json:"threads"`
	Frequency float64 `json:"frequency_mhz"` // MHz
	Usage     float64 `json:"usage_percent"`
	Temp      float64 `json:"temperature_c"` // Celsius, -1 if unavailable
}

// MemoryInfo 内存信息
type MemoryInfo struct {
	Total     uint64  `json:"total_bytes"`
	Used      uint64  `json:"used_bytes"`
	Available uint64  `json:"available_bytes"`
	SwapTotal uint64  `json:"swap_total_bytes"`
	SwapUsed  uint64  `json:"swap_used_bytes"`
	Usage     float64 `json:"usage_percent"`
}

// NetworkInfo 网络信息
type NetworkInfo struct {
	Interfaces []NetworkInterface `json:"interfaces"`
	DefaultIP  string             `json:"default_ip"`
}

// NetworkInterface 网络接口
type NetworkInterface struct {
	Name      string `json:"name"`
	IP        string `json:"ip"`
	MAC       string `json:"mac"`
	RxBytes   uint64 `json:"rx_bytes"`
	TxBytes   uint64 `json:"tx_bytes"`
	RxPackets uint64 `json:"rx_packets"`
	TxPackets uint64 `json:"tx_packets"`
	Up        bool   `json:"up"`
}

// HealthScore 健康评分
type HealthScore struct {
	TotalScore      int               `json:"total_score"` // 0-100
	Grade           string            `json:"grade"`       // A/B/C/D/F
	Trend           string            `json:"trend"`       // up/down/stable
	Components      []HealthComponent `json:"components"`
	Recommendations []string          `json:"recommendations"`
}

// HealthComponent 健康组件
type HealthComponent struct {
	Name        string `json:"name"`
	Score       int    `json:"score"`
	Status      string `json:"status"` // healthy/warning/critical
	Description string `json:"description"`
}

// DiskHealth 磁盘健康
type DiskHealth struct {
	Device       string   `json:"device"`
	Model        string   `json:"model"`
	Serial       string   `json:"serial"`
	Status       string   `json:"status"` // healthy/warning/critical/unknown
	SmartStatus  string   `json:"smart_status"`
	Temperature  float64  `json:"temperature_c"`
	PowerOnHours int      `json:"power_on_hours"`
	ReallocSect  int      `json:"realloc_sectors"`
	PendingSect  int      `json:"pending_sectors"`
	Lifetime     int      `json:"estimated_lifetime_days"` // 预估剩余寿命
	Warnings     []string `json:"warnings"`
}

// ServiceHealth 服务健康
type ServiceHealth struct {
	Name        string  `json:"name"`
	Status      string  `json:"status"` // running/stopped/failed
	Health      string  `json:"health"` // healthy/unhealthy
	Uptime      string  `json:"uptime"`
	CPUUsage    float64 `json:"cpu_usage"`
	MemoryUsage float64 `json:"memory_usage"`
	LastError   string  `json:"last_error"`
}

// Alert 告警
type Alert struct {
	ID         string    `json:"id"`
	Type       string    `json:"type"`  // disk/service/storage/network/security
	Level      string    `json:"level"` // info/warning/critical
	Message    string    `json:"message"`
	Source     string    `json:"source"`
	Timestamp  time.Time `json:"timestamp"`
	Resolved   bool      `json:"resolved"`
	Suggestion string    `json:"suggestion"`
}

// StorageOverview 存储概览
type StorageOverview struct {
	TotalCapacity uint64  `json:"total_capacity_bytes"`
	UsedCapacity  uint64  `json:"used_capacity_bytes"`
	FreeCapacity  uint64  `json:"free_capacity_bytes"`
	UsagePercent  float64 `json:"usage_percent"`
	VolumeCount   int     `json:"volume_count"`
	PoolCount     int     `json:"pool_count"`
	SnapshotCount int     `json:"snapshot_count"`
}

// VolumeInfo 卷信息
type VolumeInfo struct {
	Name     string  `json:"name"`
	Path     string  `json:"path"`
	FSType   string  `json:"fs_type"` // btrfs/ext4/zfs
	Total    uint64  `json:"total_bytes"`
	Used     uint64  `json:"used_bytes"`
	Free     uint64  `json:"free_bytes"`
	Usage    float64 `json:"usage_percent"`
	Profile  string  `json:"profile"` // raid0/raid1/raid5/raid6/raid10/single
	Mounted  bool    `json:"mounted"`
	Compress string  `json:"compression"`
	Encrypt  bool    `json:"encrypted"`
}

// PoolInfo 存储池信息
type PoolInfo struct {
	Name       string   `json:"name"`
	Profile    string   `json:"profile"`
	Devices    []string `json:"devices"`
	Total      uint64   `json:"total_bytes"`
	Used       uint64   `json:"used_bytes"`
	Free       uint64   `json:"free_bytes"`
	Status     string   `json:"status"` // healthy/degraded/error
	Rebuilding bool     `json:"rebuilding"`
}

// SnapshotInfo 快照信息
type SnapshotInfo struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Volume    string    `json:"volume"`
	CreatedAt time.Time `json:"created_at"`
	Size      uint64    `json:"size_bytes"`
	Readonly  bool      `json:"readonly"`
}

// ServiceInfo 服务信息
type ServiceInfo struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Status      string `json:"status"`
	Enabled     bool   `json:"enabled"`
	Port        int    `json:"port"`
	Protocol    string `json:"protocol"`
}

// ServiceStatus 服务状态
type ServiceStatus struct {
	Name        string    `json:"name"`
	Status      string    `json:"status"`
	Running     bool      `json:"running"`
	Enabled     bool      `json:"enabled"`
	Uptime      string    `json:"uptime"`
	CPUUsage    float64   `json:"cpu_usage"`
	MemoryUsage float64   `json:"memory_usage"`
	LastStart   time.Time `json:"last_start"`
	LastStop    time.Time `json:"last_stop"`
	LogPath     string    `json:"log_path"`
}

// ===== AI Advisor 核心 =====

// Advisor AI 智能助手
type Advisor struct {
	config *AdvisorConfig
	mu     sync.RWMutex

	// 对话历史
	history map[string][]Message // sessionID -> messages

	// 系统提示模板
	systemPrompt string

	// 知识库（故障排查、配置建议等）
	knowledgeBase *AdvisorKnowledge
}

// NewAdvisor 创建 AI Advisor
func NewAdvisor(config *AdvisorConfig) (*Advisor, error) {
	if config.Manager == nil {
		return nil, fmt.Errorf("AI Manager is required")
	}

	if config.MaxContextTokens == 0 {
		config.MaxContextTokens = 2048
	}
	if config.ResponseMaxTokens == 0 {
		config.ResponseMaxTokens = 1024
	}
	if config.Temperature == 0 {
		config.Temperature = 0.3 // 低温度，更精确的回答
	}
	if config.MaxHistoryMessages == 0 {
		config.MaxHistoryMessages = 10
	}

	return &Advisor{
		config:        config,
		history:       make(map[string][]Message),
		systemPrompt:  buildSystemPrompt(),
		knowledgeBase: NewAdvisorKnowledge(),
	}, nil
}

// buildSystemPrompt 构建系统提示
func buildSystemPrompt() string {
	return `你是 NAS-OS 系统的智能助手，帮助用户快速获取系统信息和解决问题。

## 你的职责

1. **系统状态查询**: 帮助用户快速了解 CPU、内存、磁盘、网络等系统状态
2. **健康诊断**: 分析系统健康状态，提供问题诊断和建议
3. **配置建议**: 根据系统状态提供配置优化建议
4. **故障排查**: 根据错误信息提供排查步骤和解决方案

## 响应原则

- **简洁准确**: 直接回答问题，避免冗长
- **数据驱动**: 基于实际系统数据，不要臆测
- **分级建议**: 优先给出最重要的建议
- **中文回复**: 默认使用中文，除非用户使用英文提问

## 你可以查询的信息

- 系统状态：CPU、内存、网络、运行时间
- 存储状态：卷、存储池、快照、容量使用
- 服务状态：运行的服务、状态、资源占用
- 健康状态：系统评分、磁盘 SMART、告警信息

## 回答格式示例

用户问："系统状态怎么样？"
回答："当前系统状态良好：
- CPU 使用率: 15%
- 内存使用率: 42% (8GB/16GB)
- 磁盘使用率: 65% (650GB/1TB)
- 系统运行时间: 32天

💡 建议: 内存使用正常，磁盘空间建议清理或扩容。"

用户问："磁盘有问题吗？"
回答："检测到以下磁盘状态：
- /dev/sda: 健康 (温度 38°C)
- /dev/sdb: 偊告 - 有3个待处理扇区

⚠️ 建议: 尽快备份 /dev/sdb 数据，考虑更换磁盘。"`
}

// Query 自然语言查询
func (a *Advisor) Query(ctx context.Context, sessionID, question string) (*AdvisorResponse, error) {
	a.mu.Lock()
	if _, exists := a.history[sessionID]; !exists {
		a.history[sessionID] = []Message{}
	}
	a.mu.Unlock()

	// 1. 收集系统上下文
	context, err := a.collectContext(question)
	if err != nil {
		// 上下文获取失败时继续，但记录警告
		context = &AdvisorContext{
			Timestamp: time.Now(),
			Note:      "部分系统信息获取失败",
		}
	}

	// 2. 构建消息
	messages := a.buildMessages(sessionID, question, context)

	// 3. 调用 AI
	req := &Request{
		Messages:    messages,
		MaxTokens:   a.config.ResponseMaxTokens,
		Temperature: a.config.Temperature,
		Stream:      false,
	}

	// 使用默认 provider（第一个可用）
	providers := a.config.Manager.GetAvailableProviders()
	if len(providers) == 0 {
		return nil, fmt.Errorf("没有可用的 AI 提供商")
	}

	resp, err := a.config.Manager.Chat(ctx, providers[0], req)
	if err != nil {
		return nil, fmt.Errorf("AI 请求失败: %w", err)
	}

	// 4. 更新历史
	if a.config.EnableHistory {
		a.addToHistory(sessionID, Message{Role: "user", Content: question})
		a.addToHistory(sessionID, Message{Role: "assistant", Content: resp.Content})
	}

	// 5. 构建响应
	return &AdvisorResponse{
		Answer:     resp.Content,
		Context:    context,
		TokensUsed: resp.TokensUsed,
		Model:      resp.Model,
		Provider:   resp.Provider,
		Timestamp:  time.Now(),
		SessionID:  sessionID,
	}, nil
}

// QuickQuery 快速查询（无历史，单轮）
func (a *Advisor) QuickQuery(ctx context.Context, question string) (*AdvisorResponse, error) {
	return a.Query(ctx, "quick-"+time.Now().Format("20060102150405"), question)
}

// collectContext 收集系统上下文
func (a *Advisor) collectContext(question string) (*AdvisorContext, error) {
	context := &AdvisorContext{
		Timestamp: time.Now(),
	}

	// 分析问题类型，只收集相关上下文
	qtype := a.analyzeQuestionType(question)

	// 系统状态
	if qtype.system || qtype.general {
		if a.config.SystemInfoProvider != nil {
			status, err := a.config.SystemInfoProvider.GetSystemStatus()
			if err == nil {
				context.SystemStatus = status
			}
			cpu, err := a.config.SystemInfoProvider.GetCPUInfo()
			if err == nil {
				context.CPU = cpu
			}
			mem, err := a.config.SystemInfoProvider.GetMemoryInfo()
			if err == nil {
				context.Memory = mem
			}
			net, err := a.config.SystemInfoProvider.GetNetworkInfo()
			if err == nil {
				context.Network = net
			}
		}
	}

	// 健康状态
	if qtype.health || qtype.general {
		if a.config.HealthInfoProvider != nil {
			score, err := a.config.HealthInfoProvider.GetHealthScore()
			if err == nil {
				context.HealthScore = score
			}
			diskHealth, err := a.config.HealthInfoProvider.GetDiskHealth()
			if err == nil {
				context.DiskHealth = diskHealth
			}
			alerts, err := a.config.HealthInfoProvider.GetAlerts()
			if err == nil {
				context.Alerts = alerts
			}
		}
	}

	// 存储状态
	if qtype.storage || qtype.general {
		if a.config.StorageInfoProvider != nil {
			overview, err := a.config.StorageInfoProvider.GetStorageOverview()
			if err == nil {
				context.StorageOverview = overview
			}
			volumes, err := a.config.StorageInfoProvider.GetVolumes()
			if err == nil {
				context.Volumes = volumes
			}
			pools, err := a.config.StorageInfoProvider.GetPools()
			if err == nil {
				context.Pools = pools
			}
		}
	}

	// 服务状态
	if qtype.service || qtype.general {
		if a.config.ServiceInfoProvider != nil {
			services, err := a.config.ServiceInfoProvider.GetServices()
			if err == nil {
				context.Services = services
			}
		}
	}

	// 添加知识库建议
	if qtype.troubleshoot {
		context.RelevantKnowledge = a.knowledgeBase.FindRelevant(question)
	}

	return context, nil
}

// questionType 问题类型分析
type questionType struct {
	system       bool
	health       bool
	storage      bool
	service      bool
	general      bool
	troubleshoot bool
}

// analyzeQuestionType 分析问题类型
func (a *Advisor) analyzeQuestionType(question string) questionType {
	q := strings.ToLower(question)

	return questionType{
		system: strings.Contains(q, "cpu") ||
			strings.Contains(q, "内存") ||
			strings.Contains(q, "memory") ||
			strings.Contains(q, "网络") ||
			strings.Contains(q, "network") ||
			strings.Contains(q, "负载") ||
			strings.Contains(q, "load") ||
			strings.Contains(q, "运行") ||
			strings.Contains(q, "uptime"),

		health: strings.Contains(q, "健康") ||
			strings.Contains(q, "health") ||
			strings.Contains(q, "状态") ||
			strings.Contains(q, "状态怎么样") ||
			strings.Contains(q, "评分") ||
			strings.Contains(q, "告警") ||
			strings.Contains(q, "alert"),

		storage: strings.Contains(q, "存储") ||
			strings.Contains(q, "storage") ||
			strings.Contains(q, "磁盘") ||
			strings.Contains(q, "disk") ||
			strings.Contains(q, "卷") ||
			strings.Contains(q, "volume") ||
			strings.Contains(q, "空间") ||
			strings.Contains(q, "容量") ||
			strings.Contains(q, "快照") ||
			strings.Contains(q, "snapshot") ||
			strings.Contains(q, "raid"),

		service: strings.Contains(q, "服务") ||
			strings.Contains(q, "service") ||
			strings.Contains(q, "smb") ||
			strings.Contains(q, "nfs") ||
			strings.Contains(q, "docker") ||
			strings.Contains(q, "容器"),

		troubleshoot: strings.Contains(q, "问题") ||
			strings.Contains(q, "error") ||
			strings.Contains(q, "错误") ||
			strings.Contains(q, "失败") ||
			strings.Contains(q, "fail") ||
			strings.Contains(q, "无法") ||
			strings.Contains(q, "怎么解决") ||
			strings.Contains(q, "排查") ||
			strings.Contains(q, "troubleshoot"),

		general: strings.Contains(q, "怎么样") ||
			strings.Contains(q, "overview") ||
			strings.Contains(q, "概况") ||
			strings.Contains(q, "总览") ||
			strings.Contains(q, "情况") ||
			len(q) < 10, // 短问题通常是概览请求
	}
}

// buildMessages 构建消息列表
func (a *Advisor) buildMessages(sessionID, question string, context *AdvisorContext) []Message {
	messages := []Message{
		{Role: "system", Content: a.systemPrompt},
	}

	// 添加历史（如果启用）
	if a.config.EnableHistory {
		a.mu.RLock()
		hist := a.history[sessionID]
		a.mu.RUnlock()

		// 只保留最近的消息
		start := 0
		if len(hist) > a.config.MaxHistoryMessages {
			start = len(hist) - a.config.MaxHistoryMessages
		}
		messages = append(messages, hist[start:]...)
	}

	// 构建上下文消息
	contextMsg := a.formatContext(context)
	messages = append(messages, Message{
		Role:    "system",
		Content: fmt.Sprintf("当前系统上下文信息：\n%s", contextMsg),
	})

	// 用户问题
	messages = append(messages, Message{
		Role:    "user",
		Content: question,
	})

	return messages
}

// formatContext 格式化上下文为文本
func (a *Advisor) formatContext(context *AdvisorContext) string {
	var parts []string

	// 系统状态
	if context.SystemStatus != nil {
		parts = append(parts, fmt.Sprintf(`【系统状态】
- CPU使用率: %.1f%%
- 内存使用率: %.1f%%
- Swap使用率: %.1f%%
- 运行时间: %s
- 主机名: %s`,
			context.SystemStatus.CPUUsage,
			context.SystemStatus.MemoryUsage,
			context.SystemStatus.SwapUsage,
			context.SystemStatus.Uptime,
			context.SystemStatus.Hostname))
	}

	// 健康评分
	if context.HealthScore != nil {
		compStr := ""
		for _, c := range context.HealthScore.Components {
			compStr += fmt.Sprintf("\n  - %s: %d分 (%s)", c.Name, c.Score, c.Status)
		}
		parts = append(parts, fmt.Sprintf(`【健康评分】
- 总分: %d/100
- 等级: %s
- 趋势: %s%s`,
			context.HealthScore.TotalScore,
			context.HealthScore.Grade,
			context.HealthScore.Trend,
			compStr))
	}

	// 磁盘健康
	if len(context.DiskHealth) > 0 {
		diskStr := ""
		for _, d := range context.DiskHealth {
			diskStr += fmt.Sprintf("\n  - %s: %s (温度 %.0f°C)", d.Device, d.Status, d.Temperature)
			if len(d.Warnings) > 0 {
				diskStr += " ⚠️ " + strings.Join(d.Warnings, ", ")
			}
		}
		parts = append(parts, fmt.Sprintf(`【磁盘健康】%s`, diskStr))
	}

	// 存储
	if context.StorageOverview != nil {
		parts = append(parts, fmt.Sprintf(`【存储概览】
- 总容量: %.2f GB
- 已用: %.2f GB (%.1f%%)
- 卷数量: %d
- 快照数量: %d`,
			float64(context.StorageOverview.TotalCapacity)/1e9,
			float64(context.StorageOverview.UsedCapacity)/1e9,
			context.StorageOverview.UsagePercent,
			context.StorageOverview.VolumeCount,
			context.StorageOverview.SnapshotCount))
	}

	// 告警
	if len(context.Alerts) > 0 {
		alertStr := ""
		for _, al := range context.Alerts {
			if !al.Resolved {
				alertStr += fmt.Sprintf("\n  - [%s] %s: %s", al.Level, al.Source, al.Message)
			}
		}
		if alertStr != "" {
			parts = append(parts, fmt.Sprintf(`【活跃告警】%s`, alertStr))
		}
	}

	// 知识库建议
	if len(context.RelevantKnowledge) > 0 {
		kbStr := ""
		for _, kb := range context.RelevantKnowledge {
			kbStr += fmt.Sprintf("\n  - %s", kb.Solution)
		}
		parts = append(parts, fmt.Sprintf(`【相关解决方案】%s`, kbStr))
	}

	if len(parts) == 0 {
		return "（暂无系统数据）"
	}

	return strings.Join(parts, "\n\n")
}

// addToHistory 添加到历史
func (a *Advisor) addToHistory(sessionID string, msg Message) {
	a.mu.Lock()
	defer a.mu.Unlock()

	hist := a.history[sessionID]
	hist = append(hist, msg)

	// 限制历史长度
	if len(hist) > a.config.MaxHistoryMessages*2 {
		hist = hist[len(hist)-a.config.MaxHistoryMessages:]
	}

	a.history[sessionID] = hist
}

// ClearHistory 清除历史
func (a *Advisor) ClearHistory(sessionID string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	delete(a.history, sessionID)
}

// ===== AdvisorContext =====

// AdvisorContext Advisor 上下文
type AdvisorContext struct {
	Timestamp         time.Time
	SystemStatus      *SystemStatus
	CPU               *CPUInfo
	Memory            *MemoryInfo
	Network           *NetworkInfo
	HealthScore       *HealthScore
	DiskHealth        []DiskHealth
	ServiceHealth     []ServiceHealth
	Alerts            []Alert
	StorageOverview   *StorageOverview
	Volumes           []VolumeInfo
	Pools             []PoolInfo
	Services          []ServiceInfo
	RelevantKnowledge []KnowledgeEntry
	Note              string
}

// ===== AdvisorResponse =====

// AdvisorResponse Advisor 响应
type AdvisorResponse struct {
	Answer     string          `json:"answer"`
	Context    *AdvisorContext `json:"context,omitempty"`
	TokensUsed int             `json:"tokens_used"`
	Model      string          `json:"model"`
	Provider   Provider        `json:"provider"`
	Timestamp  time.Time       `json:"timestamp"`
	SessionID  string          `json:"session_id"`
}

// ===== Knowledge Base =====

// AdvisorKnowledge 知识库
type AdvisorKnowledge struct {
	entries []KnowledgeEntry
}

// KnowledgeEntry 知识条目
type KnowledgeEntry struct {
	Keywords []string `json:"keywords"`
	Problem  string   `json:"problem"`
	Solution string   `json:"solution"`
	Category string   `json:"category"` // storage/network/service/system
	Priority int      `json:"priority"` // 1-5, 5最高
}

// NewAdvisorKnowledge 创建知识库
func NewAdvisorKnowledge() *AdvisorKnowledge {
	return &AdvisorKnowledge{
		entries: []KnowledgeEntry{
			// 存储问题
			{
				Keywords: []string{"磁盘满", "空间不足", "no space"},
				Problem:  "磁盘空间不足",
				Solution: "建议清理无用文件、删除旧快照、或扩容存储卷",
				Category: "storage",
				Priority: 4,
			},
			{
				Keywords: []string{"磁盘故障", "smart错误", "pending sectors"},
				Problem:  "磁盘SMART异常",
				Solution: "立即备份该磁盘数据，准备替换磁盘。可运行 scrub 检查数据完整性",
				Category: "storage",
				Priority: 5,
			},
			{
				Keywords: []string{"raid降级", "degraded", "重建"},
				Problem:  "RAID阵列降级",
				Solution: "检查故障磁盘，替换后启动阵列重建。重建期间避免高负载操作",
				Category: "storage",
				Priority: 5,
			},

			// 网络问题
			{
				Keywords: []string{"无法访问", "连接失败", "timeout"},
				Problem:  "网络连接问题",
				Solution: "检查网络配置、防火墙规则、服务端口是否开放",
				Category: "network",
				Priority: 3,
			},
			{
				Keywords: []string{"smb失败", "samba错误", "445端口"},
				Problem:  "SMB服务问题",
				Solution: "确认 smbd 服务运行、检查 smb.conf 配置、开放445端口",
				Category: "service",
				Priority: 4,
			},

			// 服务问题
			{
				Keywords: []string{"服务启动失败", "服务停止", "service failed"},
				Problem:  "服务运行异常",
				Solution: "查看服务日志定位原因，检查依赖服务、配置文件权限",
				Category: "service",
				Priority: 3,
			},
			{
				Keywords: []string{"内存不足", "oom", "out of memory"},
				Problem:  "内存耗尽",
				Solution: "检查内存占用大户进程，考虑增加 swap 或限制服务内存",
				Category: "system",
				Priority: 4,
			},
			{
				Keywords: []string{"cpu高", "负载高", "系统慢"},
				Problem:  "系统负载过高",
				Solution: "检查 CPU 占用进程，可能是后台任务（balance/scrub）或异常进程",
				Category: "system",
				Priority: 3,
			},

			// 性能优化
			{
				Keywords: []string{"性能优化", "加速", "提速"},
				Problem:  "性能优化需求",
				Solution: "考虑添加 SSD 缓存、调整 RAID 配置、启用压缩、定期 balance",
				Category: "system",
				Priority: 2,
			},
			{
				Keywords: []string{"配置建议", "推荐配置", "最佳实践"},
				Problem:  "配置建议请求",
				Solution: "家用推荐 raid1+定期快照；企业推荐 raid6+异地备份；高性能场景 raid10+SSD缓存",
				Category: "storage",
				Priority: 2,
			},
		},
	}
}

// FindRelevant 查找相关知识
func (kb *AdvisorKnowledge) FindRelevant(question string) []KnowledgeEntry {
	q := strings.ToLower(question)
	var results []KnowledgeEntry

	for _, entry := range kb.entries {
		for _, kw := range entry.Keywords {
			if strings.Contains(q, strings.ToLower(kw)) {
				results = append(results, entry)
				break
			}
		}
	}

	// 按优先级排序
	sortKnowledgeByPriority(results)

	return results
}

// sortKnowledgeByPriority 按优先级排序
func sortKnowledgeByPriority(entries []KnowledgeEntry) {
	// 简单冒泡排序
	for i := 0; i < len(entries)-1; i++ {
		for j := i + 1; j < len(entries); j++ {
			if entries[j].Priority > entries[i].Priority {
				entries[i], entries[j] = entries[j], entries[i]
			}
		}
	}
}

// AddEntry 添加知识条目
func (kb *AdvisorKnowledge) AddEntry(entry KnowledgeEntry) {
	kb.entries = append(kb.entries, entry)
}

// ===== 快捷查询方法 =====

// GetSystemOverview 快速获取系统概览
func (a *Advisor) GetSystemOverview(ctx context.Context) (*AdvisorResponse, error) {
	return a.QuickQuery(ctx, "请提供系统状态概览")
}

// GetHealthDiagnosis 快速获取健康诊断
func (a *Advisor) GetHealthDiagnosis(ctx context.Context) (*AdvisorResponse, error) {
	return a.QuickQuery(ctx, "系统健康状态如何？有什么问题或建议？")
}

// GetStorageSummary 快速获取存储摘要
func (a *Advisor) GetStorageSummary(ctx context.Context) (*AdvisorResponse, error) {
	return a.QuickQuery(ctx, "存储使用情况如何？")
}

// Troubleshoot 故障排查
func (a *Advisor) Troubleshoot(ctx context.Context, problem string) (*AdvisorResponse, error) {
	return a.QuickQuery(ctx, fmt.Sprintf("帮我排查问题：%s", problem))
}

// GetConfigAdvice 获取配置建议
func (a *Advisor) GetConfigAdvice(ctx context.Context, scenario string) (*AdvisorResponse, error) {
	return a.QuickQuery(ctx, fmt.Sprintf("对于%s场景，有什么配置建议？", scenario))
}

// ===== 批量查询 =====

// BatchQuery 批量查询
func (a *Advisor) BatchQuery(ctx context.Context, questions []string) ([]*AdvisorResponse, error) {
	results := make([]*AdvisorResponse, len(questions))

	for i, q := range questions {
		resp, err := a.QuickQuery(ctx, q)
		if err != nil {
			results[i] = &AdvisorResponse{
				Answer:    fmt.Sprintf("查询失败: %v", err),
				Timestamp: time.Now(),
			}
			continue
		}
		results[i] = resp
	}

	return results, nil
}
