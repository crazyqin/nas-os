// Package aiassistant 提供 AI 智能助手功能，支持自然语言查询系统状态、故障诊断和文件搜索。
// 对标群晖 AI Assistant，为 NAS 系统提供智能化运维体验。
package aiassistant

import "time"

// QueryType 查询类型
type QueryType string

const (
	QueryTypeSystem   QueryType = "system"   // 系统状态查询
	QueryTypeDisk     QueryType = "disk"     // 磁盘信息查询
	QueryTypeMemory   QueryType = "memory"   // 内存信息查询
	QueryTypeCPU      QueryType = "cpu"      // CPU 信息查询
	QueryTypeFile     QueryType = "file"     // 文件搜索
	QueryTypeDiag     QueryType = "diagnosis" // 故障诊断
	QueryTypeGeneral  QueryType = "general"  // 通用查询
)

// QueryStatus 查询状态
type QueryStatus string

const (
	QueryStatusPending    QueryStatus = "pending"
	QueryStatusProcessing QueryStatus = "processing"
	QueryStatusCompleted  QueryStatus = "completed"
	QueryStatusFailed     QueryStatus = "failed"
)

// QueryRequest 查询请求
type QueryRequest struct {
	Query     string            `json:"query" binding:"required"`
	QueryType QueryType         `json:"query_type,omitempty"`
	Context   map[string]string `json:"context,omitempty"`
	SessionID string            `json:"session_id,omitempty"`
}

// QueryResponse 查询响应
type QueryResponse struct {
	ID        string        `json:"id"`
	Query     string        `json:"query"`
	Answer    string        `json:"answer"`
	QueryType QueryType     `json:"query_type"`
	Status    QueryStatus   `json:"status"`
	Data      interface{}   `json:"data,omitempty"`      // 结构化数据
	Suggestions []string    `json:"suggestions,omitempty"` // 建议操作
	Metadata  *QueryMetadata `json:"metadata,omitempty"`
	CreatedAt time.Time     `json:"created_at"`
	Duration  time.Duration `json:"duration"`
}

// QueryMetadata 查询元数据
type QueryMetadata struct {
	Model       string  `json:"model,omitempty"`       // 使用的 AI 模型
	TokensUsed  int     `json:"tokens_used,omitempty"` // 消耗的 token 数
	Confidence  float64 `json:"confidence,omitempty"`  // 置信度
	Sources     []string `json:"sources,omitempty"`    // 数据来源
}

// SystemStatus 系统状态信息
type SystemStatus struct {
	Hostname    string        `json:"hostname"`
	Uptime      time.Duration `json:"uptime"`
	OS          string        `json:"os"`
	Kernel      string        `json:"kernel"`
	Arch        string        `json:"arch"`
	CPU         CPUInfo       `json:"cpu"`
	Memory      MemoryInfo    `json:"memory"`
	Disks       []DiskInfo    `json:"disks"`
	Network     []NetworkInfo `json:"network"`
	LoadAverage [3]float64    `json:"load_average"`
	Timestamp   time.Time     `json:"timestamp"`
}

// CPUInfo CPU 信息
type CPUInfo struct {
	Model      string  `json:"model"`
	Cores      int     `json:"cores"`
	Threads    int     `json:"threads"`
	Usage      float64 `json:"usage"`       // 百分比
	Temperature float64 `json:"temperature"` // 摄氏度
	Frequency  float64 `json:"frequency"`   // MHz
}

// MemoryInfo 内存信息
type MemoryInfo struct {
	Total     int64   `json:"total"`      // 字节
	Used      int64   `json:"used"`
	Available int64   `json:"available"`
	SwapTotal int64   `json:"swap_total"`
	SwapUsed  int64   `json:"swap_used"`
	Usage     float64 `json:"usage"` // 百分比
}

// DiskInfo 磁盘信息
type DiskInfo struct {
	Device      string  `json:"device"`
	MountPoint  string  `json:"mount_point"`
	FSType      string  `json:"fs_type"`
	Total       int64   `json:"total"`
	Used        int64   `json:"used"`
	Available   int64   `json:"available"`
	Usage       float64 `json:"usage"` // 百分比
	Health      string  `json:"health"`
	Temperature float64 `json:"temperature,omitempty"`
}

// NetworkInfo 网络信息
type NetworkInfo struct {
	Interface   string `json:"interface"`
	IPAddress   string `json:"ip_address"`
	MACAddress  string `json:"mac_address"`
	Speed       string `json:"speed"`       // 1Gbps, 10Gbps 等
	Status      string `json:"status"`      // up/down
	RxBytes     int64  `json:"rx_bytes"`
	TxBytes     int64  `json:"tx_bytes"`
}

// DiagnosisResult 诊断结果
type DiagnosisResult struct {
	ID          string             `json:"id"`
	Problem     string             `json:"problem"`
	Severity    Severity           `json:"severity"`
	Category    string             `json:"category"`
	Symptoms    []string           `json:"symptoms"`
	Causes      []string           `json:"causes"`
	Solutions   []Solution         `json:"solutions"`
	References  []string           `json:"references,omitempty"`
	CreatedAt   time.Time          `json:"created_at"`
}

// Severity 严重程度
type Severity string

const (
	SeverityLow      Severity = "low"
	SeverityMedium   Severity = "medium"
	SeverityHigh     Severity = "high"
	SeverityCritical Severity = "critical"
)

// Solution 解决方案
type Solution struct {
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Steps       []string `json:"steps"`
	Commands    []string `json:"commands,omitempty"`
	Risk        Severity `json:"risk"`
	Automated   bool     `json:"automated"` // 是否可自动执行
}

// FileSearchRequest 文件搜索请求
type FileSearchRequest struct {
	Query       string   `json:"query" binding:"required"`
	Path        string   `json:"path,omitempty"`        // 搜索路径
	FileTypes   []string `json:"file_types,omitempty"`   // 文件类型过滤
	MaxResults  int      `json:"max_results,omitempty"`  // 最大结果数
	SearchMode  string   `json:"search_mode,omitempty"`  // name, content, both
}

// FileSearchResult 文件搜索结果
type FileSearchResult struct {
	TotalFound int            `json:"total_found"`
	Files      []FileInfo     `json:"files"`
	Query      string         `json:"query"`
	Duration   time.Duration  `json:"duration"`
}

// FileInfo 文件信息
type FileInfo struct {
	Path         string    `json:"path"`
	Name         string    `json:"name"`
	Size         int64     `json:"size"`
	IsDir        bool      `json:"is_dir"`
	ModifiedAt   time.Time `json:"modified_at"`
	ContentType  string    `json:"content_type,omitempty"`
	MatchedLine  string    `json:"matched_line,omitempty"` // 匹配的行内容
	Relevance    float64   `json:"relevance"`              // 相关度评分
}

// DiagnosisRequest 诊断请求
type DiagnosisRequest struct {
	Problem     string   `json:"problem" binding:"required"`
	Category    string   `json:"category,omitempty"`    // 硬件、软件、网络等
	Symptoms    []string `json:"symptoms,omitempty"`
	SystemLogs  bool     `json:"system_logs"`           // 是否分析系统日志
}

// ConversationMessage 对话消息
type ConversationMessage struct {
	Role      string    `json:"role"`      // user, assistant, system
	Content   string    `json:"content"`
	Timestamp time.Time `json:"timestamp"`
}

// Conversation 对话会话
type Conversation struct {
	ID        string                 `json:"id"`
	Messages  []ConversationMessage  `json:"messages"`
	Context   map[string]interface{} `json:"context,omitempty"`
	CreatedAt time.Time              `json:"created_at"`
	UpdatedAt time.Time              `json:"updated_at"`
}

// AIConfig AI 助手配置
type AIConfig struct {
	Enabled         bool     `json:"enabled"`
	DefaultModel    string   `json:"default_model"`
	MaxTokens       int      `json:"max_tokens"`
	Temperature     float64  `json:"temperature"`
	SystemPrompt    string   `json:"system_prompt,omitempty"`
	AllowedActions  []string `json:"allowed_actions,omitempty"` // 允许自动执行的操作
	MaxHistory      int      `json:"max_history"`               // 最大对话历史数
	CacheEnabled    bool     `json:"cache_enabled"`
	CacheTTLMinutes int      `json:"cache_ttl_minutes"`
}

// DefaultAIConfig 默认配置
func DefaultAIConfig() *AIConfig {
	return &AIConfig{
		Enabled:         true,
		DefaultModel:    "local",
		MaxTokens:       2048,
		Temperature:     0.7,
		MaxHistory:      50,
		CacheEnabled:    true,
		CacheTTLMinutes: 30,
		AllowedActions:  []string{"status", "search", "diagnosis"},
	}
}
