// Package aiassistant 提供 AI 智能助手核心逻辑
package aiassistant

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
)

// Manager AI 助手管理器
type Manager struct {
	mu            sync.RWMutex
	logger        *zap.Logger
	config        *AIConfig
	conversations map[string]*Conversation
	queryCache    map[string]*QueryResponse
	queryHistory  []*QueryResponse
	stopChan      chan struct{}
	running       bool
}

// NewManager 创建 AI 助手管理器
func NewManager(logger *zap.Logger, config *AIConfig) *Manager {
	if logger == nil {
		logger = zap.NewNop()
	}
	if config == nil {
		config = DefaultAIConfig()
	}
	return &Manager{
		logger:        logger,
		config:        config,
		conversations: make(map[string]*Conversation),
		queryCache:    make(map[string]*QueryResponse),
		queryHistory:  make([]*QueryResponse, 0),
		stopChan:      make(chan struct{}),
	}
}

// generateID 生成唯一 ID
func generateID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// ProcessQuery 处理自然语言查询
func (m *Manager) ProcessQuery(ctx context.Context, req *QueryRequest) (*QueryResponse, error) {
	if !m.config.Enabled {
		return nil, fmt.Errorf("AI assistant is disabled")
	}

	start := time.Now()
	queryID := generateID()

	// 检查缓存
	if m.config.CacheEnabled {
		cacheKey := m.getCacheKey(req)
		m.mu.RLock()
		if cached, ok := m.queryCache[cacheKey]; ok {
			if time.Since(cached.CreatedAt) < time.Duration(m.config.CacheTTLMinutes)*time.Minute {
				m.mu.RUnlock()
				return cached, nil
			}
		}
		m.mu.RUnlock()
	}

	// 确定查询类型
	queryType := req.QueryType
	if queryType == "" {
		queryType = m.detectQueryType(req.Query)
	}

	response := &QueryResponse{
		ID:        queryID,
		Query:     req.Query,
		QueryType: queryType,
		Status:    QueryStatusProcessing,
		CreatedAt: start,
	}

	// 根据查询类型处理
	var err error
	switch queryType {
	case QueryTypeSystem, QueryTypeDisk, QueryTypeMemory, QueryTypeCPU:
		err = m.handleSystemQuery(ctx, req.Query, response)
	case QueryTypeFile:
		err = m.handleFileQuery(ctx, req.Query, response)
	case QueryTypeDiag:
		err = m.handleDiagnosisQuery(ctx, req.Query, response)
	default:
		err = m.handleGeneralQuery(ctx, req.Query, response)
	}

	response.Duration = time.Since(start)

	if err != nil {
		response.Status = QueryStatusFailed
		response.Answer = fmt.Sprintf("处理查询时出错: %v", err)
		m.logger.Error("query failed", zap.String("id", queryID), zap.Error(err))
	} else {
		response.Status = QueryStatusCompleted
	}

	// 缓存结果
	if m.config.CacheEnabled && err == nil {
		m.mu.Lock()
		m.queryCache[m.getCacheKey(req)] = response
		m.mu.Unlock()
	}

	// 记录历史
	m.mu.Lock()
	m.queryHistory = append(m.queryHistory, response)
	// 限制历史大小
	if len(m.queryHistory) > 1000 {
		m.queryHistory = m.queryHistory[len(m.queryHistory)-1000:]
	}
	m.mu.Unlock()

	return response, nil
}

// detectQueryType 自动检测查询类型
func (m *Manager) detectQueryType(query string) QueryType {
	query = strings.ToLower(query)

	// 系统状态关键词
	systemKeywords := []string{"系统状态", "系统信息", "运行状态", "system status", "uptime"}
	for _, kw := range systemKeywords {
		if strings.Contains(query, kw) {
			return QueryTypeSystem
		}
	}

	// 磁盘关键词
	diskKeywords := []string{"磁盘", "硬盘", "存储空间", "disk", "storage", "容量"}
	for _, kw := range diskKeywords {
		if strings.Contains(query, kw) {
			return QueryTypeDisk
		}
	}

	// 内存关键词
	memKeywords := []string{"内存", "ram", "memory", "swap"}
	for _, kw := range memKeywords {
		if strings.Contains(query, kw) {
			return QueryTypeMemory
		}
	}

	// CPU 关键词
	cpuKeywords := []string{"cpu", "处理器", "负载", "load", "使用率"}
	for _, kw := range cpuKeywords {
		if strings.Contains(query, kw) {
			return QueryTypeCPU
		}
	}

	// 文件搜索关键词
	fileKeywords := []string{"文件", "搜索", "查找", "file", "search", "find", "哪里"}
	for _, kw := range fileKeywords {
		if strings.Contains(query, kw) {
			return QueryTypeFile
		}
	}

	// 诊断关键词
	diagKeywords := []string{"故障", "问题", "错误", "异常", "诊断", "error", "problem", "issue", "diagnose"}
	for _, kw := range diagKeywords {
		if strings.Contains(query, kw) {
			return QueryTypeDiag
		}
	}

	return QueryTypeGeneral
}

// handleSystemQuery 处理系统状态查询
func (m *Manager) handleSystemQuery(ctx context.Context, query string, resp *QueryResponse) error {
	// 获取系统状态（简化实现，实际应从系统获取）
	status := m.getSystemStatus()

	// 根据具体查询生成回答
	query = strings.ToLower(query)

	switch resp.QueryType {
	case QueryTypeCPU:
		resp.Answer = fmt.Sprintf("CPU 状态:\n- 型号: %s\n- 核心数: %d\n- 使用率: %.1f%%\n- 温度: %.1f°C",
			status.CPU.Model, status.CPU.Cores, status.CPU.Usage, status.CPU.Temperature)
		resp.Data = status.CPU
		if status.CPU.Usage > 80 {
			resp.Suggestions = append(resp.Suggestions, "CPU 使用率较高，建议检查占用资源的进程")
		}

	case QueryTypeMemory:
		resp.Answer = fmt.Sprintf("内存状态:\n- 总内存: %.2f GB\n- 已使用: %.2f GB (%.1f%%)\n- 可用: %.2f GB\n- Swap: %.2f GB / %.2f GB",
			float64(status.Memory.Total)/1073741824,
			float64(status.Memory.Used)/1073741824,
			status.Memory.Usage,
			float64(status.Memory.Available)/1073741824,
			float64(status.Memory.SwapUsed)/1073741824,
			float64(status.Memory.SwapTotal)/1073741824)
		resp.Data = status.Memory
		if status.Memory.Usage > 90 {
			resp.Suggestions = append(resp.Suggestions, "内存使用率过高，建议关闭不必要的服务或增加内存")
		}

	case QueryTypeDisk:
		var diskInfo strings.Builder
		diskInfo.WriteString("磁盘状态:\n")
		for _, disk := range status.Disks {
			diskInfo.WriteString(fmt.Sprintf("- %s (%s): %.1f%% 已用, 剩余 %.2f GB\n",
				disk.Device, disk.MountPoint, disk.Usage, float64(disk.Available)/1073741824))
		}
		resp.Answer = diskInfo.String()
		resp.Data = status.Disks

	default:
		resp.Answer = fmt.Sprintf("系统概览:\n- 主机名: %s\n- 运行时间: %s\n- CPU: %s (%.1f%%)\n- 内存: %.1f%% 已用\n- 负载: %.2f, %.2f, %.2f",
			status.Hostname,
			formatDuration(status.Uptime),
			status.CPU.Model,
			status.CPU.Usage,
			status.Memory.Usage,
			status.LoadAverage[0], status.LoadAverage[1], status.LoadAverage[2])
		resp.Data = status
	}

	return nil
}

// handleFileQuery 处理文件搜索查询
func (m *Manager) handleFileQuery(ctx context.Context, query string, resp *QueryResponse) error {
	// 从查询中提取搜索关键词
	searchTerm := m.extractSearchTerm(query)
	if searchTerm == "" {
		resp.Answer = "请提供要搜索的文件名或关键词"
		return nil
	}

	// 执行文件搜索
	results, err := m.searchFiles(searchTerm)
	if err != nil {
		return fmt.Errorf("search files: %w", err)
	}

	resp.Data = results
	if results.TotalFound == 0 {
		resp.Answer = fmt.Sprintf("未找到与 '%s' 相关的文件", searchTerm)
	} else {
		resp.Answer = fmt.Sprintf("找到 %d 个与 '%s' 相关的文件", results.TotalFound, searchTerm)
		for i, f := range results.Files {
			if i >= 5 { // 最多显示 5 个
				break
			}
			resp.Answer += fmt.Sprintf("\n- %s (%s)", f.Path, formatSize(f.Size))
		}
		if results.TotalFound > 5 {
			resp.Answer += fmt.Sprintf("\n... 还有 %d 个结果", results.TotalFound-5)
		}
	}

	return nil
}

// handleDiagnosisQuery 处理故障诊断查询
func (m *Manager) handleDiagnosisQuery(ctx context.Context, query string, resp *QueryResponse) error {
	// 分析问题并生成诊断结果
	diagnosis := m.analyzeProblem(query)

	resp.Answer = fmt.Sprintf("诊断结果:\n问题: %s\n严重程度: %s\n\n可能原因:\n",
		diagnosis.Problem, diagnosis.Severity)

	for _, cause := range diagnosis.Causes {
		resp.Answer += fmt.Sprintf("- %s\n", cause)
	}

	resp.Answer += "\n建议解决方案:\n"
	for i, sol := range diagnosis.Solutions {
		resp.Answer += fmt.Sprintf("%d. %s\n", i+1, sol.Title)
		if sol.Automated {
			resp.Answer += "   [可自动执行]\n"
		}
	}

	resp.Data = diagnosis
	resp.Suggestions = []string{
		"查看详细日志",
		"运行系统自检",
		"联系技术支持",
	}

	return nil
}

// handleGeneralQuery 处理通用查询
func (m *Manager) handleGeneralQuery(ctx context.Context, query string, resp *QueryResponse) error {
	// 通用回复
	resp.Answer = fmt.Sprintf("我可以帮助您:\n- 查询系统状态（CPU、内存、磁盘）\n- 搜索文件\n- 诊断系统问题\n\n您的问题: %s\n\n请尝试更具体的查询，例如：\n- '查看磁盘使用情况'\n- '搜索文件 report.pdf'\n- '系统运行缓慢怎么办'", query)
	resp.Suggestions = []string{
		"查看系统状态",
		"搜索文件",
		"检查磁盘空间",
		"查看内存使用",
	}
	return nil
}

// getSystemStatus 获取系统状态（简化实现）
func (m *Manager) getSystemStatus() *SystemStatus {
	hostname, _ := os.Hostname()
	return &SystemStatus{
		Hostname: hostname,
		Uptime:   24 * time.Hour, // 简化
		OS:       "Linux",
		Kernel:   "5.15.0",
		Arch:     "x86_64",
		CPU: CPUInfo{
			Model:       "Intel Core i7",
			Cores:       8,
			Threads:     16,
			Usage:       25.5,
			Temperature: 45.0,
			Frequency:   3200.0,
		},
		Memory: MemoryInfo{
			Total:     17179869184, // 16GB
			Used:      8589934592,  // 8GB
			Available: 8589934592,
			SwapTotal: 4294967296, // 4GB
			SwapUsed:  0,
			Usage:     50.0,
		},
		Disks: []DiskInfo{
			{
				Device:     "/dev/sda1",
				MountPoint: "/",
				FSType:     "ext4",
				Total:      107374182400, // 100GB
				Used:       53687091200,  // 50GB
				Available:  53687091200,
				Usage:      50.0,
				Health:     "good",
			},
		},
		Network: []NetworkInfo{
			{
				Interface: "eth0",
				IPAddress: "192.168.1.100",
				Speed:     "1Gbps",
				Status:    "up",
			},
		},
		LoadAverage: [3]float64{1.2, 1.5, 1.8},
		Timestamp:   time.Now(),
	}
}

// searchFiles 搜索文件
func (m *Manager) searchFiles(query string) (*FileSearchResult, error) {
	start := time.Now()
	result := &FileSearchResult{
		Query: query,
		Files: make([]FileInfo, 0),
	}

	// 搜索常见目录
	searchPaths := []string{"/home", "/data", "/mnt"}
	if home, err := os.UserHomeDir(); err == nil {
		searchPaths = append([]string{home}, searchPaths...)
	}

	queryLower := strings.ToLower(query)
	maxResults := 50

	for _, root := range searchPaths {
		if _, err := os.Stat(root); os.IsNotExist(err) {
			continue
		}

		filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil
			}

			if result.TotalFound >= maxResults {
				return filepath.SkipDir
			}

			name := strings.ToLower(info.Name())
			if strings.Contains(name, queryLower) || strings.Contains(strings.ToLower(path), queryLower) {
				result.Files = append(result.Files, FileInfo{
					Path:       path,
					Name:       info.Name(),
					Size:       info.Size(),
					IsDir:      info.IsDir(),
					ModifiedAt: info.ModTime(),
					Relevance:  calculateRelevance(name, queryLower),
				})
				result.TotalFound++
			}

			return nil
		})
	}

	result.Duration = time.Since(start)
	return result, nil
}

// analyzeProblem 分析问题并生成诊断结果
func (m *Manager) analyzeProblem(problem string) *DiagnosisResult {
	problem = strings.ToLower(problem)

	diagnosis := &DiagnosisResult{
		ID:        generateID(),
		Problem:   problem,
		Symptoms:  make([]string, 0),
		Causes:    make([]string, 0),
		Solutions: make([]Solution, 0),
		CreatedAt: time.Now(),
	}

	// 简单的问题匹配规则
	switch {
	case strings.Contains(problem, "慢") || strings.Contains(problem, "slow") || strings.Contains(problem, "卡"):
		diagnosis.Severity = SeverityMedium
		diagnosis.Category = "performance"
		diagnosis.Symptoms = []string{"系统响应缓慢", "操作延迟"}
		diagnosis.Causes = []string{
			"CPU 或内存使用率过高",
			"磁盘 I/O 瓶颈",
			"后台进程占用资源",
		}
		diagnosis.Solutions = []Solution{
			{
				Title:       "检查系统资源使用",
				Description: "查看 CPU、内存和磁盘使用情况",
				Steps:       []string{"运行 top 或 htop 查看进程", "检查磁盘 I/O 使用 iostat"},
				Risk:        SeverityLow,
				Automated:   true,
			},
			{
				Title:       "清理临时文件",
				Description: "释放磁盘空间和缓存",
				Steps:       []string{"清理 /tmp 目录", "清除系统缓存"},
				Commands:    []string{"sudo apt clean", "sync; echo 3 > /proc/sys/vm/drop_caches"},
				Risk:        SeverityLow,
				Automated:   false,
			},
		}

	case strings.Contains(problem, "磁盘") || strings.Contains(problem, "disk") || strings.Contains(problem, "空间"):
		diagnosis.Severity = SeverityHigh
		diagnosis.Category = "storage"
		diagnosis.Symptoms = []string{"磁盘空间不足", "无法写入文件"}
		diagnosis.Causes = []string{
			"日志文件过大",
			"临时文件堆积",
			"用户数据增长",
		}
		diagnosis.Solutions = []Solution{
			{
				Title:       "清理大文件",
				Description: "查找并删除不必要的大文件",
				Steps:       []string{"使用 du -sh /* 查找大目录", "清理日志和临时文件"},
				Risk:        SeverityMedium,
				Automated:   true,
			},
			{
				Title:       "扩展存储",
				Description: "添加新硬盘或扩展现有分区",
				Steps:       []string{"备份数据", "添加新硬盘", "扩展文件系统"},
				Risk:        SeverityHigh,
				Automated:   false,
			},
		}

	case strings.Contains(problem, "网络") || strings.Contains(problem, "network") || strings.Contains(problem, "连接"):
		diagnosis.Severity = SeverityMedium
		diagnosis.Category = "network"
		diagnosis.Symptoms = []string{"网络连接问题", "无法访问服务"}
		diagnosis.Causes = []string{
			"网络配置错误",
			"防火墙规则阻止",
			"DNS 解析问题",
		}
		diagnosis.Solutions = []Solution{
			{
				Title:       "检查网络配置",
				Description: "验证网络接口和路由配置",
				Steps:       []string{"运行 ip addr 查看接口", "检查 /etc/resolv.conf", "测试 ping 连通性"},
				Risk:        SeverityLow,
				Automated:   true,
			},
		}

	default:
		diagnosis.Severity = SeverityLow
		diagnosis.Category = "general"
		diagnosis.Symptoms = []string{problem}
		diagnosis.Causes = []string{"需要进一步诊断"}
		diagnosis.Solutions = []Solution{
			{
				Title:       "运行系统自检",
				Description: "执行全面的系统健康检查",
				Steps:       []string{"检查系统日志", "运行硬件诊断", "检查服务状态"},
				Risk:        SeverityLow,
				Automated:   true,
			},
		}
	}

	return diagnosis
}

// extractSearchTerm 从查询中提取搜索关键词
func (m *Manager) extractSearchTerm(query string) string {
	// 尝试提取引号中的内容
	re := regexp.MustCompile(`[""「」](.+?)[""「」]`)
	matches := re.FindStringSubmatch(query)
	if len(matches) > 1 {
		return matches[1]
	}

	// 移除常见动词和助词
	stopWords := []string{"搜索", "查找", "找", "文件", "search", "find", "file", "哪里", "where"}
	terms := strings.Fields(query)
	var filtered []string
	for _, t := range terms {
		isStop := false
		for _, sw := range stopWords {
			if strings.EqualFold(t, sw) {
				isStop = true
				break
			}
		}
		if !isStop {
			filtered = append(filtered, t)
		}
	}

	if len(filtered) > 0 {
		return strings.Join(filtered, " ")
	}
	return query
}

// calculateRelevance 计算相关度
func calculateRelevance(filename, query string) float64 {
	if filename == query {
		return 1.0
	}
	if strings.HasPrefix(filename, query) {
		return 0.9
	}
	if strings.HasSuffix(filename, query) {
		return 0.7
	}
	return 0.5
}

// getCacheKey 生成缓存键
func (m *Manager) getCacheKey(req *QueryRequest) string {
	return fmt.Sprintf("%s:%s", req.QueryType, req.Query)
}

// GetQueryHistory 获取查询历史
func (m *Manager) GetQueryHistory(limit int) []*QueryResponse {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if limit <= 0 || limit > len(m.queryHistory) {
		limit = len(m.queryHistory)
	}

	// 返回最新的记录
	start := len(m.queryHistory) - limit
	if start < 0 {
		start = 0
	}

	result := make([]*QueryResponse, limit)
	copy(result, m.queryHistory[start:])
	return result
}

// GetConversation 获取对话
func (m *Manager) GetConversation(id string) (*Conversation, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	conv, ok := m.conversations[id]
	if !ok {
		return nil, fmt.Errorf("conversation %s not found", id)
	}
	return conv, nil
}

// CreateConversation 创建新对话
func (m *Manager) CreateConversation() *Conversation {
	m.mu.Lock()
	defer m.mu.Unlock()

	conv := &Conversation{
		ID:        generateID(),
		Messages:  make([]ConversationMessage, 0),
		Context:   make(map[string]interface{}),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	m.conversations[conv.ID] = conv
	return conv
}

// AddMessage 添加消息到对话
func (m *Manager) AddMessage(convID, role, content string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	conv, ok := m.conversations[convID]
	if !ok {
		return fmt.Errorf("conversation %s not found", convID)
	}

	conv.Messages = append(conv.Messages, ConversationMessage{
		Role:      role,
		Content:   content,
		Timestamp: time.Now(),
	})
	conv.UpdatedAt = time.Now()

	// 限制历史大小
	if len(conv.Messages) > m.config.MaxHistory*2 {
		conv.Messages = conv.Messages[len(conv.Messages)-m.config.MaxHistory*2:]
	}

	return nil
}

// GetConfig 获取配置
func (m *Manager) GetConfig() *AIConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()
	// 返回副本
	cfg := *m.config
	return &cfg
}

// UpdateConfig 更新配置
func (m *Manager) UpdateConfig(cfg *AIConfig) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if cfg != nil {
		m.config = cfg
	}
}

// ClearCache 清除缓存
func (m *Manager) ClearCache() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.queryCache = make(map[string]*QueryResponse)
}

// formatDuration 格式化时间
func formatDuration(d time.Duration) string {
	days := int(d.Hours()) / 24
	hours := int(d.Hours()) % 24
	minutes := int(d.Minutes()) % 60

	if days > 0 {
		return fmt.Sprintf("%d天%d小时%d分钟", days, hours, minutes)
	}
	if hours > 0 {
		return fmt.Sprintf("%d小时%d分钟", hours, minutes)
	}
	return fmt.Sprintf("%d分钟", minutes)
}

// formatSize 格式化文件大小
func formatSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}
