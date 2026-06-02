// Package smartaiassistant 提供统一AI助手面板功能
// 集成所有AI模块的统一入口，支持自然语言NAS管理、智能问答、故障诊断、操作建议
package smartaiassistant

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

// AIAction 表示AI助手识别的操作类型
type AIAction string

const (
	ActionQuery     AIAction = "query"     // 查询操作
	ActionDiagnose  AIAction = "diagnose"  // 诊断操作
	ActionSuggest   AIAction = "suggest"   // 建议操作
	ActionExecute   AIAction = "execute"   // 执行操作
	ActionStatus    AIAction = "status"    // 状态查询
	ActionHelp      AIAction = "help"      // 帮助信息
)

// Message 表示对话消息
type Message struct {
	Role      string    // 角色：user/assistant/system
	Content   string    // 消息内容
	Timestamp time.Time // 时间戳
	Action    AIAction  // 关联的操作类型
}

// SystemStatus 表示NAS系统状态
type SystemStatus struct {
	CPUUsage    float64 // CPU使用率
	MemoryUsage float64 // 内存使用率
	DiskUsage   float64 // 磁盘使用率
	Temperature float64 // 系统温度
	Uptime      int64   // 运行时间（秒）
	NetworkUp   bool    // 网络状态
	ServicesOK  bool    // 服务状态
}

// StorageStatus 表示存储状态
type StorageStatus struct {
	TotalSpace   int64  // 总空间（字节）
	UsedSpace    int64  // 已用空间（字节）
	FreeSpace    int64  // 可用空间（字节）
	RAIDStatus   string // RAID状态
	DiskCount    int    // 磁盘数量
	HealthStatus string // 健康状态
}

// DiagnosisResult 诊断结果
type DiagnosisResult struct {
	IssueType    string   // 问题类型
	Severity     string   // 严重程度：info/warning/error/critical
	Description  string   // 问题描述
	Suggestions  []string // 建议措施
	RelatedLogs  []string // 相关日志
	Timestamp    time.Time // 诊断时间
}

// Suggestion 操作建议
type Suggestion struct {
	Title       string   // 建议标题
	Description string   // 建议描述
	Category    string   // 分类
	Priority    int      // 优先级（1-5，5最高）
	Steps       []string // 操作步骤
}

// AIResult AI处理结果
type AIResult struct {
	Action    AIAction           // 操作类型
	Response  string             // AI响应内容
	Diagnosis *DiagnosisResult   // 诊断结果（如有）
	Suggestions []*Suggestion    // 建议列表（如有）
	Context   map[string]string  // 上下文信息
	Timestamp time.Time          // 处理时间
}

// AIProvider AI后端提供者接口
type AIProvider interface {
	// Name 返回提供者名称
	Name() string
	// Process 处理查询请求
	Process(query string, context map[string]string) (string, error)
	// IsAvailable 检查提供者是否可用
	IsAvailable() bool
}

// LocalProvider 本地LLM提供者
type LocalProvider struct {
	name    string
	model   string
	available bool
}

// NewLocalProvider 创建本地LLM提供者
func NewLocalProvider(name, model string) *LocalProvider {
	return &LocalProvider{
		name:      name,
		model:     model,
		available: true,
	}
}

// Name 返回提供者名称
func (lp *LocalProvider) Name() string {
	return lp.name
}

// Process 处理查询请求
func (lp *LocalProvider) Process(query string, context map[string]string) (string, error) {
	if !lp.available {
		return "", fmt.Errorf("本地LLM不可用")
	}
	// 模拟本地LLM处理
	response := fmt.Sprintf("[本地LLM %s] 处理查询: %s", lp.model, query)
	if ctx, ok := context["system_status"]; ok {
		response += fmt.Sprintf(" (系统状态: %s)", ctx)
	}
	return response, nil
}

// IsAvailable 检查提供者是否可用
func (lp *LocalProvider) IsAvailable() bool {
	return lp.available
}

// RemoteProvider 远程API提供者
type RemoteProvider struct {
	name      string
	endpoint  string
	apiKey    string
	available bool
}

// NewRemoteProvider 创建远程API提供者
func NewRemoteProvider(name, endpoint, apiKey string) *RemoteProvider {
	return &RemoteProvider{
		name:      name,
		endpoint:  endpoint,
		apiKey:    apiKey,
		available: true,
	}
}

// Name 返回提供者名称
func (rp *RemoteProvider) Name() string {
	return rp.name
}

// Process 处理查询请求
func (rp *RemoteProvider) Process(query string, context map[string]string) (string, error) {
	if !rp.available {
		return "", fmt.Errorf("远程API不可用")
	}
	// 模拟远程API处理
	response := fmt.Sprintf("[远程API %s] 处理查询: %s", rp.name, query)
	if ctx, ok := context["system_status"]; ok {
		response += fmt.Sprintf(" (系统状态: %s)", ctx)
	}
	return response, nil
}

// IsAvailable 检查提供者是否可用
func (rp *RemoteProvider) IsAvailable() bool {
	return rp.available
}

// UnifiedAIAssistant 统一AI助手
type UnifiedAIAssistant struct {
	mu             sync.RWMutex
	providers      []AIProvider          // AI后端列表
	conversations  map[string][]Message  // 对话历史（按会话ID）
	systemStatus   *SystemStatus         // 系统状态
	storageStatus  *StorageStatus        // 存储状态
	maxHistory     int                   // 最大历史记录数
	defaultProvider AIProvider           // 默认AI后端
}

// NewUnifiedAIAssistant 创建统一AI助手实例
func NewUnifiedAIAssistant() *UnifiedAIAssistant {
	return &UnifiedAIAssistant{
		providers:     make([]AIProvider, 0),
		conversations: make(map[string][]Message),
		maxHistory:    100,
	}
}

// RegisterProvider 注册AI后端提供者
func (a *UnifiedAIAssistant) RegisterProvider(provider AIProvider) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.providers = append(a.providers, provider)
	if a.defaultProvider == nil {
		a.defaultProvider = provider
	}
}

// SetDefaultProvider 设置默认AI后端
func (a *UnifiedAIAssistant) SetDefaultProvider(provider AIProvider) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.defaultProvider = provider
}

// GetProviders 获取所有AI后端
func (a *UnifiedAIAssistant) GetProviders() []AIProvider {
	a.mu.RLock()
	defer a.mu.RUnlock()
	providers := make([]AIProvider, len(a.providers))
	copy(providers, a.providers)
	return providers
}

// UpdateSystemStatus 更新系统状态
func (a *UnifiedAIAssistant) UpdateSystemStatus(status *SystemStatus) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.systemStatus = status
}

// UpdateStorageStatus 更新存储状态
func (a *UnifiedAIAssistant) UpdateStorageStatus(status *StorageStatus) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.storageStatus = status
}

// getSystemContext 获取系统上下文信息
func (a *UnifiedAIAssistant) getSystemContext() map[string]string {
	a.mu.RLock()
	defer a.mu.RUnlock()
	
	context := make(map[string]string)
	if a.systemStatus != nil {
		context["system_status"] = fmt.Sprintf("CPU:%.1f%% 内存:%.1f%% 磁盘:%.1f%% 温度:%.1f°C",
			a.systemStatus.CPUUsage, a.systemStatus.MemoryUsage,
			a.systemStatus.DiskUsage, a.systemStatus.Temperature)
		context["uptime"] = fmt.Sprintf("%d秒", a.systemStatus.Uptime)
		context["network"] = fmt.Sprintf("%v", a.systemStatus.NetworkUp)
	}
	if a.storageStatus != nil {
		context["storage"] = fmt.Sprintf("总空间:%dGB 已用:%dGB 可用:%dGB RAID:%s",
			a.storageStatus.TotalSpace/1073741824,
			a.storageStatus.UsedSpace/1073741824,
			a.storageStatus.FreeSpace/1073741824,
			a.storageStatus.RAIDStatus)
		context["disk_count"] = fmt.Sprintf("%d", a.storageStatus.DiskCount)
		context["health"] = a.storageStatus.HealthStatus
	}
	return context
}

// classifyQuery 分类查询意图
func (a *UnifiedAIAssistant) classifyQuery(query string) AIAction {
	query = strings.ToLower(query)
	
	// 诊断关键词
	diagnoseKeywords := []string{"故障", "问题", "错误", "异常", "报错", "失败", "无法", "不能", "坏了", "出问题"}
	for _, kw := range diagnoseKeywords {
		if strings.Contains(query, kw) {
			return ActionDiagnose
		}
	}
	
	// 建议关键词
	suggestKeywords := []string{"建议", "推荐", "优化", "改进", "怎么做", "如何", "怎样"}
	for _, kw := range suggestKeywords {
		if strings.Contains(query, kw) {
			return ActionSuggest
		}
	}
	
	// 状态关键词
	statusKeywords := []string{"状态", "运行", "监控", "查看", "检查"}
	for _, kw := range statusKeywords {
		if strings.Contains(query, kw) {
			return ActionStatus
		}
	}
	
	// 帮助关键词
	helpKeywords := []string{"帮助", "怎么用", "使用方法", "功能", "能做什么"}
	for _, kw := range helpKeywords {
		if strings.Contains(query, kw) {
			return ActionHelp
		}
	}
	
	// 执行关键词
	executeKeywords := []string{"执行", "运行", "启动", "停止", "重启", "关闭"}
	for _, kw := range executeKeywords {
		if strings.Contains(query, kw) {
			return ActionExecute
		}
	}
	
	// 默认为查询
	return ActionQuery
}

// addMessage 添加消息到对话历史
func (a *UnifiedAIAssistant) addMessage(sessionID string, msg Message) {
	a.mu.Lock()
	defer a.mu.Unlock()
	
	history, exists := a.conversations[sessionID]
	if !exists {
		history = make([]Message, 0)
	}
	
	history = append(history, msg)
	
	// 限制历史记录数量
	if len(history) > a.maxHistory {
		history = history[len(history)-a.maxHistory:]
	}
	
	a.conversations[sessionID] = history
}

// GetConversationHistory 获取对话历史
func (a *UnifiedAIAssistant) GetConversationHistory(sessionID string) []Message {
	a.mu.RLock()
	defer a.mu.RUnlock()
	
	history, exists := a.conversations[sessionID]
	if !exists {
		return []Message{}
	}
	
	result := make([]Message, len(history))
	copy(result, history)
	return result
}

// ClearConversationHistory 清空对话历史
func (a *UnifiedAIAssistant) ClearConversationHistory(sessionID string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	delete(a.conversations, sessionID)
}

// Query 处理自然语言查询
func (a *UnifiedAIAssistant) Query(sessionID, query string) (*AIResult, error) {
	if query == "" {
		return nil, fmt.Errorf("查询内容不能为空")
	}
	
	// 记录用户消息
	a.addMessage(sessionID, Message{
		Role:      "user",
		Content:   query,
		Timestamp: time.Now(),
		Action:    ActionQuery,
	})
	
	// 分类查询意图
	action := a.classifyQuery(query)
	
	// 获取系统上下文
	context := a.getSystemContext()
	
	// 选择AI后端
	a.mu.RLock()
	provider := a.defaultProvider
	a.mu.RUnlock()
	
	if provider == nil {
		return nil, fmt.Errorf("未配置AI后端")
	}
	
	// 处理查询
	response, err := provider.Process(query, context)
	if err != nil {
		return nil, fmt.Errorf("AI处理失败: %v", err)
	}
	
	// 构建结果
	result := &AIResult{
		Action:    action,
		Response:  response,
		Context:   context,
		Timestamp: time.Now(),
	}
	
	// 根据操作类型处理
	switch action {
	case ActionDiagnose:
		diagnosis := a.performDiagnosis(query)
		result.Diagnosis = diagnosis
		result.Response = a.formatDiagnosisResponse(diagnosis)
	case ActionSuggest:
		suggestions := a.generateSuggestions(query)
		result.Suggestions = suggestions
		result.Response = a.formatSuggestionsResponse(suggestions)
	case ActionStatus:
		result.Response = a.formatStatusResponse()
	case ActionHelp:
		result.Response = a.formatHelpResponse()
	}
	
	// 记录助手回复
	a.addMessage(sessionID, Message{
		Role:      "assistant",
		Content:   result.Response,
		Timestamp: time.Now(),
		Action:    action,
	})
	
	return result, nil
}

// Diagnose 智能故障诊断
func (a *UnifiedAIAssistant) Diagnose(sessionID, symptom string) (*DiagnosisResult, error) {
	if symptom == "" {
		return nil, fmt.Errorf("症状描述不能为空")
	}
	
	// 记录用户消息
	a.addMessage(sessionID, Message{
		Role:      "user",
		Content:   symptom,
		Timestamp: time.Now(),
		Action:    ActionDiagnose,
	})
	
	// 执行诊断
	diagnosis := a.performDiagnosis(symptom)
	
	// 记录诊断结果
	a.addMessage(sessionID, Message{
		Role:      "assistant",
		Content:   a.formatDiagnosisResponse(diagnosis),
		Timestamp: time.Now(),
		Action:    ActionDiagnose,
	})
	
	return diagnosis, nil
}

// performDiagnosis 执行诊断
func (a *UnifiedAIAssistant) performDiagnosis(symptom string) *DiagnosisResult {
	a.mu.RLock()
	defer a.mu.RUnlock()
	
	diagnosis := &DiagnosisResult{
		Timestamp: time.Now(),
	}
	
	symptom = strings.ToLower(symptom)
	
	// 分析系统状态
	if a.systemStatus != nil {
		if a.systemStatus.CPUUsage > 90 {
			diagnosis.IssueType = "cpu"
			diagnosis.Severity = "warning"
			diagnosis.Description = "CPU使用率过高"
			diagnosis.Suggestions = []string{
				"检查是否有异常进程占用CPU",
				"考虑升级CPU或优化应用",
				"检查是否有挖矿病毒",
			}
			diagnosis.RelatedLogs = []string{"top", "ps aux --sort=-%cpu"}
			return diagnosis
		}
		
		if a.systemStatus.MemoryUsage > 90 {
			diagnosis.IssueType = "memory"
			diagnosis.Severity = "warning"
			diagnosis.Description = "内存使用率过高"
			diagnosis.Suggestions = []string{
				"检查内存泄漏的应用",
				"增加Swap空间",
				"考虑增加物理内存",
			}
			diagnosis.RelatedLogs = []string{"free -h", "cat /proc/meminfo"}
			return diagnosis
		}
		
		if a.systemStatus.Temperature > 80 {
			diagnosis.IssueType = "temperature"
			diagnosis.Severity = "error"
			diagnosis.Description = "系统温度过高"
			diagnosis.Suggestions = []string{
				"检查散热风扇是否正常",
				"清理灰尘",
				"改善通风环境",
				"降低系统负载",
			}
			diagnosis.RelatedLogs = []string{"sensors", "cat /sys/class/thermal/thermal_zone*/temp"}
			return diagnosis
		}
		
		if !a.systemStatus.NetworkUp {
			diagnosis.IssueType = "network"
			diagnosis.Severity = "error"
			diagnosis.Description = "网络连接异常"
			diagnosis.Suggestions = []string{
				"检查网线是否插好",
				"检查路由器状态",
				"检查网络配置",
				"重启网络服务",
			}
			diagnosis.RelatedLogs = []string{"ip addr", "ping -c 3 8.8.8.8", "systemctl status networking"}
			return diagnosis
		}
	}
	
	// 分析存储状态
	if a.storageStatus != nil {
		if a.storageStatus.HealthStatus != "healthy" {
			diagnosis.IssueType = "storage"
			diagnosis.Severity = "warning"
			diagnosis.Description = "存储健康状态异常"
			diagnosis.Suggestions = []string{
				"检查SMART状态",
				"备份重要数据",
				"考虑更换硬盘",
			}
			diagnosis.RelatedLogs = []string{"smartctl -a /dev/sda", "mdadm --detail /dev/md0"}
			return diagnosis
		}
		
		usagePercent := float64(a.storageStatus.UsedSpace) / float64(a.storageStatus.TotalSpace) * 100
		if usagePercent > 90 {
			diagnosis.IssueType = "disk"
			diagnosis.Severity = "warning"
			diagnosis.Description = "磁盘空间不足"
			diagnosis.Suggestions = []string{
				"清理临时文件",
				"删除不需要的日志",
				"扩展存储空间",
			}
			diagnosis.RelatedLogs = []string{"df -h", "du -sh /*"}
			return diagnosis
		}
	}
	
	// 基于症状关键词的诊断
	if strings.Contains(symptom, "慢") || strings.Contains(symptom, "卡") {
		diagnosis.IssueType = "performance"
		diagnosis.Severity = "info"
		diagnosis.Description = "系统性能问题"
		diagnosis.Suggestions = []string{
			"检查系统资源使用情况",
			"优化启动服务",
			"检查是否有异常进程",
		}
		diagnosis.RelatedLogs = []string{"top", "iotop", "iftop"}
		return diagnosis
	}
	
	if strings.Contains(symptom, "噪音") || strings.Contains(symptom, "响") {
		diagnosis.IssueType = "hardware"
		diagnosis.Severity = "warning"
		diagnosis.Description = "硬件噪音问题"
		diagnosis.Suggestions = []string{
			"检查风扇是否正常",
			"检查硬盘是否有坏道",
			"检查电源是否稳定",
		}
		diagnosis.RelatedLogs = []string{"smartctl -a /dev/sda"}
		return diagnosis
	}
	
	// 默认诊断
	diagnosis.IssueType = "unknown"
	diagnosis.Severity = "info"
	diagnosis.Description = "未发现明显问题"
	diagnosis.Suggestions = []string{
		"请提供更详细的症状描述",
		"检查系统日志获取更多信息",
		"运行系统自检",
	}
	diagnosis.RelatedLogs = []string{"journalctl -xe", "dmesg | tail -100"}
	
	return diagnosis
}

// formatDiagnosisResponse 格式化诊断响应
func (a *UnifiedAIAssistant) formatDiagnosisResponse(diagnosis *DiagnosisResult) string {
	var sb strings.Builder
	
	sb.WriteString(fmt.Sprintf("🔍 诊断结果\n"))
	sb.WriteString(fmt.Sprintf("━━━━━━━━━━━━━━━━━━━━\n"))
	sb.WriteString(fmt.Sprintf("问题类型: %s\n", diagnosis.IssueType))
	sb.WriteString(fmt.Sprintf("严重程度: %s\n", diagnosis.Severity))
	sb.WriteString(fmt.Sprintf("问题描述: %s\n", diagnosis.Description))
	
	if len(diagnosis.Suggestions) > 0 {
		sb.WriteString("\n💡 建议措施:\n")
		for i, s := range diagnosis.Suggestions {
			sb.WriteString(fmt.Sprintf("  %d. %s\n", i+1, s))
		}
	}
	
	if len(diagnosis.RelatedLogs) > 0 {
		sb.WriteString("\n📋 相关命令:\n")
		for _, log := range diagnosis.RelatedLogs {
			sb.WriteString(fmt.Sprintf("  - %s\n", log))
		}
	}
	
	return sb.String()
}

// Suggest 生成操作建议
func (a *UnifiedAIAssistant) Suggest(sessionID, scenario string) ([]*Suggestion, error) {
	if scenario == "" {
		return nil, fmt.Errorf("场景描述不能为空")
	}
	
	// 记录用户消息
	a.addMessage(sessionID, Message{
		Role:      "user",
		Content:   scenario,
		Timestamp: time.Now(),
		Action:    ActionSuggest,
	})
	
	// 生成建议
	suggestions := a.generateSuggestions(scenario)
	
	// 记录建议
	a.addMessage(sessionID, Message{
		Role:      "assistant",
		Content:   a.formatSuggestionsResponse(suggestions),
		Timestamp: time.Now(),
		Action:    ActionSuggest,
	})
	
	return suggestions, nil
}

// generateSuggestions 生成建议
func (a *UnifiedAIAssistant) generateSuggestions(scenario string) []*Suggestion {
	scenario = strings.ToLower(scenario)
	suggestions := make([]*Suggestion, 0)
	
	// 备份相关建议
	if strings.Contains(scenario, "备份") || strings.Contains(scenario, "数据安全") {
		suggestions = append(suggestions, &Suggestion{
			Title:       "配置自动备份",
			Description: "设置定时备份任务，确保数据安全",
			Category:    "backup",
			Priority:    5,
			Steps: []string{
				"选择备份源目录",
				"选择备份目标（本地/远程/云存储）",
				"设置备份计划（每日/每周）",
				"配置保留策略",
				"测试恢复流程",
			},
		})
	}
	
	// 性能优化建议
	if strings.Contains(scenario, "性能") || strings.Contains(scenario, "优化") || strings.Contains(scenario, "速度") {
		suggestions = append(suggestions, &Suggestion{
			Title:       "系统性能优化",
			Description: "优化系统配置，提升运行效率",
			Category:    "performance",
			Priority:    4,
			Steps: []string{
				"关闭不必要的启动服务",
				"调整内存分配策略",
				"启用SSD缓存（如有）",
				"优化网络配置",
				"定期清理临时文件",
			},
		})
	}
	
	// 安全相关建议
	if strings.Contains(scenario, "安全") || strings.Contains(scenario, "防护") {
		suggestions = append(suggestions, &Suggestion{
			Title:       "安全加固配置",
			Description: "增强系统安全性，防止未授权访问",
			Category:    "security",
			Priority:    5,
			Steps: []string{
				"启用防火墙",
				"配置访问控制列表",
				"启用双因素认证",
				"定期更新系统补丁",
				"监控异常登录",
			},
		})
	}
	
	// 存储管理建议
	if strings.Contains(scenario, "存储") || strings.Contains(scenario, "空间") || strings.Contains(scenario, "容量") {
		suggestions = append(suggestions, &Suggestion{
			Title:       "存储空间管理",
			Description: "优化存储使用，释放可用空间",
			Category:    "storage",
			Priority:    3,
			Steps: []string{
				"分析存储使用情况",
				"清理临时文件和日志",
				"压缩不常用文件",
				"配置存储配额",
				"考虑扩展存储容量",
			},
		})
	}
	
	// 默认建议
	if len(suggestions) == 0 {
		suggestions = append(suggestions, &Suggestion{
			Title:       "系统健康检查",
			Description: "定期检查系统状态，预防潜在问题",
			Category:    "maintenance",
			Priority:    3,
			Steps: []string{
				"检查系统日志",
				"验证存储健康状态",
				"测试网络连接",
				"检查更新",
				"备份重要数据",
			},
		})
	}
	
	return suggestions
}

// formatSuggestionsResponse 格式化建议响应
func (a *UnifiedAIAssistant) formatSuggestionsResponse(suggestions []*Suggestion) string {
	var sb strings.Builder
	
	sb.WriteString("💡 操作建议\n")
	sb.WriteString("━━━━━━━━━━━━━━━━━━━━\n")
	
	for i, s := range suggestions {
		sb.WriteString(fmt.Sprintf("\n%d. %s (优先级: %d/5)\n", i+1, s.Title, s.Priority))
		sb.WriteString(fmt.Sprintf("   分类: %s\n", s.Category))
		sb.WriteString(fmt.Sprintf("   说明: %s\n", s.Description))
		
		if len(s.Steps) > 0 {
			sb.WriteString("   步骤:\n")
			for j, step := range s.Steps {
				sb.WriteString(fmt.Sprintf("     %d.%d %s\n", i+1, j+1, step))
			}
		}
	}
	
	return sb.String()
}

// GetStatus 获取系统状态概览
func (a *UnifiedAIAssistant) GetStatus() map[string]interface{} {
	a.mu.RLock()
	defer a.mu.RUnlock()
	
	status := make(map[string]interface{})
	
	if a.systemStatus != nil {
		status["system"] = map[string]interface{}{
			"cpu":        a.systemStatus.CPUUsage,
			"memory":     a.systemStatus.MemoryUsage,
			"disk":       a.systemStatus.DiskUsage,
			"temperature": a.systemStatus.Temperature,
			"uptime":     a.systemStatus.Uptime,
			"network":    a.systemStatus.NetworkUp,
			"services":   a.systemStatus.ServicesOK,
		}
	}
	
	if a.storageStatus != nil {
		status["storage"] = map[string]interface{}{
			"total":      a.storageStatus.TotalSpace,
			"used":       a.storageStatus.UsedSpace,
			"free":       a.storageStatus.FreeSpace,
			"raid":       a.storageStatus.RAIDStatus,
			"disk_count": a.storageStatus.DiskCount,
			"health":     a.storageStatus.HealthStatus,
		}
	}
	
	status["providers"] = len(a.providers)
	status["conversations"] = len(a.conversations)
	
	return status
}

// formatStatusResponse 格式化状态响应
func (a *UnifiedAIAssistant) formatStatusResponse() string {
	a.mu.RLock()
	defer a.mu.RUnlock()
	
	var sb strings.Builder
	
	sb.WriteString("📊 系统状态\n")
	sb.WriteString("━━━━━━━━━━━━━━━━━━━━\n")
	
	if a.systemStatus != nil {
		sb.WriteString("\n🖥️ 系统信息:\n")
		sb.WriteString(fmt.Sprintf("  CPU使用率: %.1f%%\n", a.systemStatus.CPUUsage))
		sb.WriteString(fmt.Sprintf("  内存使用率: %.1f%%\n", a.systemStatus.MemoryUsage))
		sb.WriteString(fmt.Sprintf("  磁盘使用率: %.1f%%\n", a.systemStatus.DiskUsage))
		sb.WriteString(fmt.Sprintf("  系统温度: %.1f°C\n", a.systemStatus.Temperature))
		sb.WriteString(fmt.Sprintf("  运行时间: %d秒\n", a.systemStatus.Uptime))
		sb.WriteString(fmt.Sprintf("  网络状态: %v\n", a.systemStatus.NetworkUp))
		sb.WriteString(fmt.Sprintf("  服务状态: %v\n", a.systemStatus.ServicesOK))
	}
	
	if a.storageStatus != nil {
		sb.WriteString("\n💾 存储信息:\n")
		sb.WriteString(fmt.Sprintf("  总空间: %dGB\n", a.storageStatus.TotalSpace/1073741824))
		sb.WriteString(fmt.Sprintf("  已用: %dGB\n", a.storageStatus.UsedSpace/1073741824))
		sb.WriteString(fmt.Sprintf("  可用: %dGB\n", a.storageStatus.FreeSpace/1073741824))
		sb.WriteString(fmt.Sprintf("  RAID状态: %s\n", a.storageStatus.RAIDStatus))
		sb.WriteString(fmt.Sprintf("  磁盘数量: %d\n", a.storageStatus.DiskCount))
		sb.WriteString(fmt.Sprintf("  健康状态: %s\n", a.storageStatus.HealthStatus))
	}
	
	sb.WriteString("\n🤖 AI后端:\n")
	sb.WriteString(fmt.Sprintf("  已注册: %d个\n", len(a.providers)))
	sb.WriteString(fmt.Sprintf("  活跃对话: %d个\n", len(a.conversations)))
	
	return sb.String()
}

// formatHelpResponse 格式化帮助响应
func (a *UnifiedAIAssistant) formatHelpResponse() string {
	return `🤖 NAS智能助手 - 使用帮助
━━━━━━━━━━━━━━━━━━━━

我是一个集成AI能力的NAS管理助手，可以帮助您：

📋 主要功能：

1. 🔍 系统查询
   - 查看系统状态（CPU、内存、磁盘、温度）
   - 查询存储信息（容量、RAID状态、健康状态）
   - 检查网络连接状态

2. 🛠️ 故障诊断
   - 自动检测系统问题
   - 分析症状并提供建议
   - 生成诊断报告

3. 💡 操作建议
   - 性能优化建议
   - 安全加固建议
   - 存储管理建议
   - 备份策略建议

4. ⚡ 命令执行
   - 执行系统命令
   - 管理服务
   - 配置系统参数

📝 使用示例：
- "查看系统状态"
- "磁盘空间不足怎么办"
- "如何优化系统性能"
- "建议备份方案"
- "网络连接异常"

💬 提示：
- 您可以用自然语言描述问题
- 我会自动识别您的意图
- 支持多轮对话，我会记住上下文
`
}

// parseCommand 解析命令
func (a *UnifiedAIAssistant) parseCommand(input string) (AIAction, string) {
	input = strings.TrimSpace(input)
	
	// 命令前缀匹配
	commands := map[string]AIAction{
		"/query":    ActionQuery,
		"/diagnose": ActionDiagnose,
		"/suggest":  ActionSuggest,
		"/execute":  ActionExecute,
		"/status":   ActionStatus,
		"/help":     ActionHelp,
	}
	
	for prefix, action := range commands {
		if strings.HasPrefix(input, prefix) {
			content := strings.TrimSpace(strings.TrimPrefix(input, prefix))
			return action, content
		}
	}
	
	// 默认为自然语言查询
	return ActionQuery, input
}

// ExecuteCommand 执行命令
func (a *UnifiedAIAssistant) ExecuteCommand(sessionID, command string) (*AIResult, error) {
	action, content := a.parseCommand(command)
	
	switch action {
	case ActionQuery:
		return a.Query(sessionID, content)
	case ActionDiagnose:
		diagnosis, err := a.Diagnose(sessionID, content)
		if err != nil {
			return nil, err
		}
		return &AIResult{
			Action:    ActionDiagnose,
			Response:  a.formatDiagnosisResponse(diagnosis),
			Diagnosis: diagnosis,
			Timestamp: time.Now(),
		}, nil
	case ActionSuggest:
		suggestions, err := a.Suggest(sessionID, content)
		if err != nil {
			return nil, err
		}
		return &AIResult{
			Action:      ActionSuggest,
			Response:    a.formatSuggestionsResponse(suggestions),
			Suggestions: suggestions,
			Timestamp:   time.Now(),
		}, nil
	case ActionStatus:
		return &AIResult{
			Action:    ActionStatus,
			Response:  a.formatStatusResponse(),
			Timestamp: time.Now(),
		}, nil
	case ActionHelp:
		return &AIResult{
			Action:    ActionHelp,
			Response:  a.formatHelpResponse(),
			Timestamp: time.Now(),
		}, nil
	case ActionExecute:
		return nil, fmt.Errorf("命令执行功能需要权限验证")
	default:
		return a.Query(sessionID, content)
	}
}

// GetStats 获取助手统计信息
func (a *UnifiedAIAssistant) GetStats() map[string]interface{} {
	a.mu.RLock()
	defer a.mu.RUnlock()
	
	stats := make(map[string]interface{})
	stats["total_providers"] = len(a.providers)
	stats["active_conversations"] = len(a.conversations)
	stats["max_history"] = a.maxHistory
	
	totalMessages := 0
	for _, history := range a.conversations {
		totalMessages += len(history)
	}
	stats["total_messages"] = totalMessages
	
	return stats
}
