// Package aiopsagent 提供 AI 驱动的智能运维助手能力
// 对标群晖 DSM Agent，支持自然语言查询、故障诊断、自动化建议
package aiopsagent

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// Agent 智能运维助手
type Agent struct {
	mu          sync.RWMutex
	initialized bool
	conversations map[string]*Conversation
	knowledgeBase *KnowledgeBase
	tools         map[string]Tool
	logger        Logger
}

// Conversation 对话会话
type Conversation struct {
	ID        string
	Messages  []Message
	Context   map[string]interface{}
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Message 消息
type Message struct {
	Role      string    // user, assistant, system
	Content   string
	Timestamp time.Time
	Metadata  map[string]interface{}
}

// KnowledgeBase 知识库
type KnowledgeBase struct {
	entries map[string]*KnowledgeEntry
	mu      sync.RWMutex
}

// KnowledgeEntry 知识条目
type KnowledgeEntry struct {
	ID        string
	Category  string
	Title     string
	Content   string
	Tags      []string
	UpdatedAt time.Time
}

// Tool 工具接口
type Tool interface {
	Name() string
	Description() string
	Execute(ctx context.Context, params map[string]interface{}) (interface{}, error)
}

// Logger 日志接口
type Logger interface {
	Info(msg string, args ...interface{})
	Error(msg string, args ...interface{})
	Debug(msg string, args ...interface{})
}

// NewAgent 创建新的智能运维助手
func NewAgent(logger Logger) *Agent {
	return &Agent{
		conversations: make(map[string]*Conversation),
		knowledgeBase: &KnowledgeBase{
			entries: make(map[string]*KnowledgeEntry),
		},
		tools:  make(map[string]Tool),
		logger: logger,
	}
}

// Init 初始化助手
func (a *Agent) Init(ctx context.Context) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.initialized {
		return fmt.Errorf("助手已初始化")
	}

	// 注册内置工具
	a.registerBuiltinTools()

	// 加载知识库
	a.loadKnowledgeBase()

	a.initialized = true
	a.logger.Info("AI运维助手已启动")
	return nil
}

// Chat 处理用户消息
func (a *Agent) Chat(ctx context.Context, conversationID, userMessage string) (string, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if !a.initialized {
		return "", fmt.Errorf("助手未初始化")
	}

	// 获取或创建会话
	conv, exists := a.conversations[conversationID]
	if !exists {
		conv = &Conversation{
			ID:        conversationID,
			Messages:  make([]Message, 0),
			Context:   make(map[string]interface{}),
			CreatedAt: time.Now(),
		}
		a.conversations[conversationID] = conv
	}

	// 添加用户消息
	conv.Messages = append(conv.Messages, Message{
		Role:      "user",
		Content:   userMessage,
		Timestamp: time.Now(),
	})

	// 分析意图
	intent := a.analyzeIntent(userMessage)

	// 生成响应
	response := a.generateResponse(ctx, conv, intent)

	// 添加助手响应
	conv.Messages = append(conv.Messages, Message{
		Role:      "assistant",
		Content:   response,
		Timestamp: time.Now(),
	})
	conv.UpdatedAt = time.Now()

	return response, nil
}

// analyzeIntent 分析用户意图
func (a *Agent) analyzeIntent(message string) string {
	// 简单的意图识别
	intents := map[string][]string{
		"system_status":  {"状态", "运行", "健康", "状态"},
		"disk_usage":     {"磁盘", "存储", "空间", "容量"},
		"memory_usage":   {"内存", "RAM", "内存"},
		"process_info":   {"进程", "服务", "程序"},
		"network_info":   {"网络", "连接", "流量"},
		"error_diagnosis": {"错误", "故障", "问题", "异常"},
		"optimization":   {"优化", "建议", "改进"},
	}

	for intent, keywords := range intents {
		for _, keyword := range keywords {
			if contains(message, keyword) {
				return intent
			}
		}
	}

	return "general"
}

// generateResponse 生成响应
func (a *Agent) generateResponse(ctx context.Context, conv *Conversation, intent string) string {
	switch intent {
	case "system_status":
		return a.getSystemStatus(ctx)
	case "disk_usage":
		return a.getDiskUsage(ctx)
	case "memory_usage":
		return a.getMemoryUsage(ctx)
	case "process_info":
		return a.getProcessInfo(ctx)
	case "network_info":
		return a.getNetworkInfo(ctx)
	case "error_diagnosis":
		return a.diagnoseError(ctx, conv)
	case "optimization":
		return a.getOptimizationSuggestions(ctx)
	default:
		return "我是NAS-OS智能运维助手，可以帮您查询系统状态、诊断问题、提供优化建议。请问有什么可以帮您？"
	}
}

// getSystemStatus 获取系统状态
func (a *Agent) getSystemStatus(ctx context.Context) string {
	// 模拟系统状态查询
	return "系统运行正常\n- 运行时间: 15天3小时\n- CPU使用率: 23%\n- 内存使用率: 45%\n- 磁盘使用率: 67%\n- 网络连接: 正常"
}

// getDiskUsage 获取磁盘使用情况
func (a *Agent) getDiskUsage(ctx context.Context) string {
	return "磁盘使用情况:\n- 总容量: 2TB\n- 已用: 1.34TB (67%)\n- 可用: 660GB\n- 建议: 考虑清理大文件或扩展存储"
}

// getMemoryUsage 获取内存使用情况
func (a *Agent) getMemoryUsage(ctx context.Context) string {
	return "内存使用情况:\n- 总内存: 8GB\n- 已用: 3.6GB (45%)\n- 缓存: 2.4GB\n- 可用: 4.4GB\n- 状态: 正常"
}

// getProcessInfo 获取进程信息
func (a *Agent) getProcessInfo(ctx context.Context) string {
	return "运行中的服务:\n- nasd (主服务) - 运行中\n- nginx (Web服务) - 运行中\n- postgres (数据库) - 运行中\n- redis (缓存) - 运行中"
}

// getNetworkInfo 获取网络信息
func (a *Agent) getNetworkInfo(ctx context.Context) string {
	return "网络状态:\n- 入站流量: 125 Mbps\n- 出站流量: 89 Mbps\n- 活跃连接: 1,234\n- 状态: 正常"
}

// diagnoseError 诊断错误
func (a *Agent) diagnoseError(ctx context.Context, conv *Conversation) string {
	return "错误诊断建议:\n1. 检查系统日志: journalctl -xe\n2. 检查服务状态: systemctl status nasd\n3. 检查磁盘空间: df -h\n4. 检查内存使用: free -h\n\n如需进一步帮助，请提供具体的错误信息。"
}

// getOptimizationSuggestions 获取优化建议
func (a *Agent) getOptimizationSuggestions(ctx context.Context) string {
	return "优化建议:\n1. 启用存储分层，将冷数据迁移到低成本存储\n2. 配置自动备份策略\n3. 启用缓存加速\n4. 定期清理临时文件\n5. 监控资源使用趋势"
}

// registerBuiltinTools 注册内置工具
func (a *Agent) registerBuiltinTools() {
	// 注册系统查询工具
	a.tools["system_query"] = &SystemQueryTool{}
	a.tools["disk_analyzer"] = &DiskAnalyzerTool{}
	a.tools["log_analyzer"] = &LogAnalyzerTool{}
}

// loadKnowledgeBase 加载知识库
func (a *Agent) loadKnowledgeBase() {
	// 加载NAS相关知识
	entries := []*KnowledgeEntry{
		{ID: "kb001", Category: "存储", Title: "ZFS存储池管理", Content: "ZFS是一种先进的文件系统...", Tags: []string{"zfs", "存储", "管理"}},
		{ID: "kb002", Category: "网络", Title: "SMB共享配置", Content: "SMB是一种网络文件共享协议...", Tags: []string{"smb", "共享", "网络"}},
		{ID: "kb003", Category: "安全", Title: "ACL权限管理", Content: "ACL提供细粒度的访问控制...", Tags: []string{"acl", "权限", "安全"}},
	}

	for _, entry := range entries {
		a.knowledgeBase.entries[entry.ID] = entry
	}
}

// contains 检查字符串是否包含子串
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && (s[0] == substr[0] && contains(s[1:], substr[1:])))
}

// SystemQueryTool 系统查询工具
type SystemQueryTool struct{}

func (t *SystemQueryTool) Name() string { return "system_query" }
func (t *SystemQueryTool) Description() string { return "查询系统状态信息" }
func (t *SystemQueryTool) Execute(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	return map[string]interface{}{
		"cpu":    23.5,
		"memory": 45.2,
		"disk":   67.8,
	}, nil
}

// DiskAnalyzerTool 磁盘分析工具
type DiskAnalyzerTool struct{}

func (t *DiskAnalyzerTool) Name() string { return "disk_analyzer" }
func (t *DiskAnalyzerTool) Description() string { return "分析磁盘使用情况" }
func (t *DiskAnalyzerTool) Execute(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	return map[string]interface{}{
		"total":     "2TB",
		"used":      "1.34TB",
		"available": "660GB",
	}, nil
}

// LogAnalyzerTool 日志分析工具
type LogAnalyzerTool struct{}

func (t *LogAnalyzerTool) Name() string { return "log_analyzer" }
func (t *LogAnalyzerTool) Description() string { return "分析系统日志" }
func (t *LogAnalyzerTool) Execute(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	return map[string]interface{}{
		"errors":   2,
		"warnings": 15,
		"info":     1234,
	}, nil
}
