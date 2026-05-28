// Package aiadvisor 提供 AI 智能顾问功能，集成 Ollama 本地 LLM
package aiadvisor

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net"
	"net/http"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/host"
	"github.com/shirou/gopsutil/v3/load"
	"github.com/shirou/gopsutil/v3/mem"
	psnet "github.com/shirou/gopsutil/v3/net"
	"go.uber.org/zap"
)

// ========== 错误定义 ==========

var (
	ErrEmptyMessage     = errors.New("消息不能为空")
	ErrOllamaUnavail    = errors.New("Ollama 服务不可用")
	ErrEmptyDiagnosis   = errors.New("诊断日志内容不能为空")
	ErrSessionNotFound  = errors.New("会话不存在")
)

// ========== 配置 ==========

// Config AI 顾问配置.
type Config struct {
	// OllamaEndpoint Ollama API 地址.
	OllamaEndpoint string `json:"ollama_endpoint"`
	// Model 使用的模型名称.
	Model string `json:"model"`
	// Temperature 生成温度，越低越确定.
	Temperature float64 `json:"temperature"`
	// MaxHistoryMessages 每个会话最大历史消息数.
	MaxHistoryMessages int `json:"max_history_messages"`
	// RequestTimeout 请求超时时间.
	RequestTimeout time.Duration `json:"request_timeout"`
	// MaxTokens 最大生成 token 数.
	MaxTokens int `json:"max_tokens"`
}

// DefaultConfig 返回默认配置.
func DefaultConfig() *Config {
	return &Config{
		OllamaEndpoint:     "http://localhost:11434",
		Model:              "qwen2.5:7b",
		Temperature:        0.7,
		MaxHistoryMessages: 20,
		RequestTimeout:     60 * time.Second,
		MaxTokens:          2048,
	}
}

// ========== 消息类型 ==========

// Message 对话消息.
type Message struct {
	Role    string `json:"role"`    // system, user, assistant
	Content string `json:"content"`
}

// ChatRequest 聊天请求.
type ChatRequest struct {
	Message   string `json:"message"`
	SessionID string `json:"session_id,omitempty"`
}

// ChatResponse 聊天响应.
type ChatResponse struct {
	Answer    string    `json:"answer"`
	SessionID string    `json:"session_id"`
	Timestamp time.Time `json:"timestamp"`
}

// Suggestion 功能推荐.
type Suggestion struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Category    string `json:"category"` // performance, security, storage, backup, network
	Priority    int    `json:"priority"` // 1=高, 2=中, 3=低
	Action      string `json:"action"`
}

// DiagnoseRequest 故障诊断请求.
type DiagnoseRequest struct {
	LogContent string `json:"log_content"`
	Service    string `json:"service,omitempty"`
}

// DiagnoseResponse 故障诊断响应.
type DiagnoseResponse struct {
	Problem    string   `json:"problem"`
	Cause      string   `json:"cause"`
	Solutions  []string `json:"solutions"`
	Severity   string   `json:"severity"` // critical, warning, info
	Timestamp  time.Time `json:"timestamp"`
}

// SystemContext 系统上下文信息.
type SystemContext struct {
	Hostname    string    `json:"hostname"`
	OS          string    `json:"os"`
	Arch        string    `json:"arch"`
	CPUUsage    float64   `json:"cpu_usage"`
	CPUCores    int       `json:"cpu_cores"`
	MemTotal    uint64    `json:"mem_total"`
	MemUsed     uint64    `json:"mem_used"`
	MemUsage    float64   `json:"mem_usage"`
	SwapTotal   uint64    `json:"swap_total"`
	SwapUsed    uint64    `json:"swap_used"`
	LoadAvg1    float64   `json:"load_avg_1"`
	LoadAvg5    float64   `json:"load_avg_5"`
	LoadAvg15   float64   `json:"load_avg_15"`
	Uptime      uint64    `json:"uptime_seconds"`
	Disks       []DiskInfo `json:"disks"`
	Networks    []NetInfo  `json:"networks"`
	Timestamp   time.Time `json:"timestamp"`
}

// DiskInfo 磁盘信息.
type DiskInfo struct {
	Mountpoint  string  `json:"mountpoint"`
	Device      string  `json:"device"`
	FSType      string  `json:"fstype"`
	Total       uint64  `json:"total"`
	Used        uint64  `json:"used"`
	Free        uint64  `json:"free"`
	UsagePct    float64 `json:"usage_pct"`
}

// NetInfo 网络接口信息.
type NetInfo struct {
	Name      string `json:"name"`
	IP        string `json:"ip"`
	BytesSent uint64 `json:"bytes_sent"`
	BytesRecv uint64 `json:"bytes_recv"`
	Up        bool   `json:"up"`
}

// ========== Ollama API 类型 ==========

// ollamaRequest Ollama chat API 请求.
type ollamaRequest struct {
	Model    string    `json:"model"`
	Messages []Message `json:"messages"`
	Stream   bool      `json:"stream"`
	Options  ollamaOptions `json:"options,omitempty"`
}

// ollamaOptions Ollama 生成选项.
type ollamaOptions struct {
	Temperature float64 `json:"temperature,omitempty"`
	NumPredict  int     `json:"num_predict,omitempty"`
}

// ollamaResponse Ollama chat API 响应.
type ollamaResponse struct {
	Model     string  `json:"model"`
	Message   Message `json:"message"`
	Done      bool    `json:"done"`
	TotalDur  int64   `json:"total_duration"`
	EvalCount int     `json:"eval_count"`
}

// ollamaTagsResponse Ollama tags API 响应.
type ollamaTagsResponse struct {
	Models []ollamaModel `json:"models"`
}

// ollamaModel Ollama 模型信息.
type ollamaModel struct {
	Name string `json:"name"`
}

// ========== 核心服务 ==========

// Advisor AI 智能顾问核心服务.
type Advisor struct {
	mu       sync.RWMutex
	config   *Config
	logger   *zap.Logger
	client   *http.Client
	sessions map[string][]Message // sessionID -> history
}

// NewAdvisor 创建 AI 智能顾问.
func NewAdvisor(cfg *Config, logger *zap.Logger) *Advisor {
	if cfg == nil {
		cfg = DefaultConfig()
	}
	if logger == nil {
		logger = zap.NewNop()
	}
	if cfg.OllamaEndpoint == "" {
		cfg.OllamaEndpoint = "http://localhost:11434"
	}
	if cfg.Model == "" {
		cfg.Model = "qwen2.5:7b"
	}
	if cfg.Temperature == 0 {
		cfg.Temperature = 0.7
	}
	if cfg.MaxHistoryMessages == 0 {
		cfg.MaxHistoryMessages = 20
	}
	if cfg.RequestTimeout == 0 {
		cfg.RequestTimeout = 60 * time.Second
	}
	if cfg.MaxTokens == 0 {
		cfg.MaxTokens = 2048
	}

	return &Advisor{
		config:   cfg,
		logger:   logger,
		client:   &http.Client{Timeout: cfg.RequestTimeout},
		sessions: make(map[string][]Message),
	}
}

// Chat 聊天接口.
func (a *Advisor) Chat(ctx context.Context, req *ChatRequest) (*ChatResponse, error) {
	if req.Message == "" {
		return nil, ErrEmptyMessage
	}

	sessionID := req.SessionID
	if sessionID == "" {
		sessionID = fmt.Sprintf("session-%d", time.Now().UnixNano())
	}

	// 收集系统上下文
	sysCtx := a.collectSystemContext()

	// 构建消息列表
	messages := a.buildChatMessages(sessionID, req.Message, sysCtx)

	// 调用 Ollama
	answer, err := a.callOllama(ctx, messages)
	if err != nil {
		return nil, err
	}

	// 保存历史
	a.addToHistory(sessionID, Message{Role: "user", Content: req.Message})
	a.addToHistory(sessionID, Message{Role: "assistant", Content: answer})

	return &ChatResponse{
		Answer:    answer,
		SessionID: sessionID,
		Timestamp: time.Now(),
	}, nil
}

// GetSuggestions 获取功能推荐.
func (a *Advisor) GetSuggestions(ctx context.Context) ([]Suggestion, error) {
	sysCtx := a.collectSystemContext()
	return a.generateSuggestions(sysCtx), nil
}

// Diagnose 故障诊断.
func (a *Advisor) Diagnose(ctx context.Context, req *DiagnoseRequest) (*DiagnoseResponse, error) {
	if req.LogContent == "" {
		return nil, ErrEmptyDiagnosis
	}

	// 收集系统上下文
	sysCtx := a.collectSystemContext()

	// 构建诊断消息
	messages := a.buildDiagnoseMessages(req, sysCtx)

	// 调用 Ollama
	answer, err := a.callOllama(ctx, messages)
	if err != nil {
		return nil, err
	}

	// 解析诊断结果
	resp := a.parseDiagnosis(answer, req.LogContent)
	return resp, nil
}

// GetSystemContext 获取当前系统上下文.
func (a *Advisor) GetSystemContext() *SystemContext {
	return a.collectSystemContext()
}

// ClearSession 清除会话历史.
func (a *Advisor) ClearSession(sessionID string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	delete(a.sessions, sessionID)
}

// HealthCheck 检查 Ollama 服务健康状态.
func (a *Advisor) HealthCheck(ctx context.Context) error {
	url := fmt.Sprintf("%s/api/tags", a.config.OllamaEndpoint)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}

	resp, err := a.client.Do(httpReq)
	if err != nil {
		return ErrOllamaUnavail
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return ErrOllamaUnavail
	}
	return nil
}

// ========== 内部方法 ==========

// collectSystemContext 收集系统上下文信息.
func (a *Advisor) collectSystemContext() *SystemContext {
	ctx := &SystemContext{
		Timestamp: time.Now(),
		Arch:      runtime.GOARCH,
		OS:        runtime.GOOS,
	}

	// 主机信息
	if h, err := host.Info(); err == nil {
		ctx.Hostname = h.Hostname
		ctx.Uptime = h.Uptime
	}

	// CPU 使用率
	if percents, err := cpu.Percent(500*time.Millisecond, false); err == nil && len(percents) > 0 {
		ctx.CPUUsage = math.Round(percents[0]*100) / 100
	}
	ctx.CPUCores = runtime.NumCPU()

	// 负载
	if avg, err := load.Avg(); err == nil {
		ctx.LoadAvg1 = math.Round(avg.Load1*100) / 100
		ctx.LoadAvg5 = math.Round(avg.Load5*100) / 100
		ctx.LoadAvg15 = math.Round(avg.Load15*100) / 100
	}

	// 内存
	if vmem, err := mem.VirtualMemory(); err == nil {
		ctx.MemTotal = vmem.Total
		ctx.MemUsed = vmem.Used
		ctx.MemUsage = math.Round(vmem.UsedPercent*100) / 100
	}
	if swap, err := mem.SwapMemory(); err == nil {
		ctx.SwapTotal = swap.Total
		ctx.SwapUsed = swap.Used
	}

	// 磁盘
	if parts, err := disk.Partitions(false); err == nil {
		seen := make(map[string]bool)
		for _, p := range parts {
			if seen[p.Mountpoint] {
				continue
			}
			seen[p.Mountpoint] = true
			usage, err := disk.Usage(p.Mountpoint)
			if err != nil {
				continue
			}
			if usage.Total == 0 {
				continue
			}
			ctx.Disks = append(ctx.Disks, DiskInfo{
				Mountpoint: p.Mountpoint,
				Device:     p.Device,
				FSType:     p.Fstype,
				Total:      usage.Total,
				Used:       usage.Used,
				Free:       usage.Free,
				UsagePct:   math.Round(usage.UsedPercent*100) / 100,
			})
		}
	}

	// 网络
	if interfaces, err := net.Interfaces(); err == nil {
		for _, iface := range interfaces {
			if iface.Name == "lo" {
				continue
			}
			addrs, err := iface.Addrs()
			if err != nil || len(addrs) == 0 {
				continue
			}
			ip := ""
			for _, addr := range addrs {
				addrStr := addr.String()
				if strings.Contains(addrStr, ".") && !strings.HasPrefix(addrStr, "127.") {
					ip = addrStr
					break
				}
			}
			if ip == "" {
				continue
			}
			up := iface.Flags&net.FlagUp != 0
			var bytesSent, bytesRecv uint64
			if counters, err := psnet.IOCounters(true); err == nil {
				for _, c := range counters {
					if c.Name == iface.Name {
						bytesSent = c.BytesSent
						bytesRecv = c.BytesRecv
						break
					}
				}
			}
			ctx.Networks = append(ctx.Networks, NetInfo{
				Name:      iface.Name,
				IP:        ip,
				BytesSent: bytesSent,
				BytesRecv: bytesRecv,
				Up:        up,
			})
		}
	}

	return ctx
}

// buildChatMessages 构建聊天消息列表.
func (a *Advisor) buildChatMessages(sessionID, userMsg string, sysCtx *SystemContext) []Message {
	msgs := []Message{
		{
			Role: "system",
			Content: `你是 NAS-OS 系统的智能顾问助手。你可以帮助用户：
1. 查询系统状态（CPU、内存、磁盘、网络）
2. 推荐 NAS 功能和优化建议
3. 诊断故障并提供修复方案

回答规则：
- 简洁准确，直接回答问题
- 基于实际系统数据回答，不要臆测
- 使用中文回答
- 对复杂问题给出分步骤建议`,
		},
	}

	// 系统上下文注入
	ctxJSON, _ := json.Marshal(sysCtx)
	msgs = append(msgs, Message{
		Role:    "system",
		Content: fmt.Sprintf("当前系统状态（JSON）：%s", string(ctxJSON)),
	})

	// 历史消息
	a.mu.RLock()
	hist := a.sessions[sessionID]
	a.mu.RUnlock()

	start := 0
	if len(hist) > a.config.MaxHistoryMessages {
		start = len(hist) - a.config.MaxHistoryMessages
	}
	msgs = append(msgs, hist[start:]...)

	// 当前用户消息
	msgs = append(msgs, Message{Role: "user", Content: userMsg})

	return msgs
}

// buildDiagnoseMessages 构建诊断消息列表.
func (a *Advisor) buildDiagnoseMessages(req *DiagnoseRequest, sysCtx *SystemContext) []Message {
	serviceHint := ""
	if req.Service != "" {
		serviceHint = fmt.Sprintf("（相关服务: %s）", req.Service)
	}

	ctxJSON, _ := json.Marshal(sysCtx)

	return []Message{
		{
			Role: "system",
			Content: `你是 NAS 系统故障诊断专家。分析用户提供的日志内容，给出：
1. 问题描述（problem）
2. 原因分析（cause）  
3. 解决方案列表（solutions），每个方案一个步骤
4. 严重程度（severity）：critical / warning / info

请用 JSON 格式回复，格式如下：
{"problem": "...", "cause": "...", "solutions": ["步骤1", "步骤2"], "severity": "warning"}`,
		},
		{
			Role:    "system",
			Content: fmt.Sprintf("当前系统状态：%s", string(ctxJSON)),
		},
		{
			Role: "user",
			Content: fmt.Sprintf("请分析以下日志%s并给出诊断：\n\n%s", serviceHint, req.LogContent),
		},
	}
}

// callOllama 调用 Ollama API.
func (a *Advisor) callOllama(ctx context.Context, messages []Message) (string, error) {
	url := fmt.Sprintf("%s/api/chat", a.config.OllamaEndpoint)

	reqBody := ollamaRequest{
		Model:    a.config.Model,
		Messages: messages,
		Stream:   false,
		Options: ollamaOptions{
			Temperature: a.config.Temperature,
			NumPredict:  a.config.MaxTokens,
		},
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("序列化请求失败: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(bodyBytes))
	if err != nil {
		return "", fmt.Errorf("创建请求失败: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := a.client.Do(httpReq)
	if err != nil {
		if ctx.Err() != nil {
			return "", fmt.Errorf("请求超时: %w", err)
		}
		return "", fmt.Errorf("Ollama 调用失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("Ollama 返回错误 (status=%d): %s", resp.StatusCode, string(body))
	}

	var ollamaResp ollamaResponse
	if err := json.NewDecoder(resp.Body).Decode(&ollamaResp); err != nil {
		return "", fmt.Errorf("解析 Ollama 响应失败: %w", err)
	}

	return ollamaResp.Message.Content, nil
}

// addToHistory 添加消息到会话历史.
func (a *Advisor) addToHistory(sessionID string, msg Message) {
	a.mu.Lock()
	defer a.mu.Unlock()

	hist := a.sessions[sessionID]
	hist = append(hist, msg)

	// 限制历史长度
	maxLen := a.config.MaxHistoryMessages * 2
	if len(hist) > maxLen {
		hist = hist[len(hist)-a.config.MaxHistoryMessages:]
	}

	a.sessions[sessionID] = hist
}

// generateSuggestions 基于系统状态生成推荐.
func (a *Advisor) generateSuggestions(sysCtx *SystemContext) []Suggestion {
	var suggestions []Suggestion
	id := 0
	nextID := func() string {
		id++
		return fmt.Sprintf("sug-%04d", id)
	}

	// CPU 相关建议
	if sysCtx.CPUUsage > 80 {
		suggestions = append(suggestions, Suggestion{
			ID:          nextID(),
			Title:       "CPU 使用率过高",
			Description: fmt.Sprintf("当前 CPU 使用率 %.1f%%，建议检查高占用进程", sysCtx.CPUUsage),
			Category:    "performance",
			Priority:    1,
			Action:      "查看进程列表并优化",
		})
	}

	// 内存相关建议
	if sysCtx.MemUsage > 85 {
		suggestions = append(suggestions, Suggestion{
			ID:          nextID(),
			Title:       "内存使用率过高",
			Description: fmt.Sprintf("当前内存使用率 %.1f%%，可能导致系统变慢或 OOM", sysCtx.MemUsage),
			Category:    "performance",
			Priority:    1,
			Action:      "考虑增加内存或优化服务配置",
		})
	} else if sysCtx.MemUsage > 70 {
		suggestions = append(suggestions, Suggestion{
			ID:          nextID(),
			Title:       "内存使用率偏高",
			Description: fmt.Sprintf("当前内存使用率 %.1f%%，建议关注内存占用情况", sysCtx.MemUsage),
			Category:    "performance",
			Priority:    2,
			Action:      "监控内存使用趋势",
		})
	}

	// 磁盘相关建议
	for _, d := range sysCtx.Disks {
		if d.UsagePct > 90 {
			suggestions = append(suggestions, Suggestion{
				ID:          nextID(),
				Title:       fmt.Sprintf("磁盘 %s 空间不足", d.Mountpoint),
				Description: fmt.Sprintf("磁盘使用率 %.1f%%，剩余 %.2f GB", d.UsagePct, float64(d.Free)/(1024*1024*1024)),
				Category:    "storage",
				Priority:    1,
				Action:      "清理无用文件或扩容存储",
			})
		} else if d.UsagePct > 80 {
			suggestions = append(suggestions, Suggestion{
				ID:          nextID(),
				Title:       fmt.Sprintf("磁盘 %s 空间预警", d.Mountpoint),
				Description: fmt.Sprintf("磁盘使用率 %.1f%%，建议提前规划存储空间", d.UsagePct),
				Category:    "storage",
				Priority:    2,
				Action:      "检查并清理大文件、旧快照",
			})
		}
	}

	// 负载相关建议
	if sysCtx.LoadAvg1 > float64(sysCtx.CPUCores)*2 {
		suggestions = append(suggestions, Suggestion{
			ID:          nextID(),
			Title:       "系统负载过高",
			Description: fmt.Sprintf("1分钟负载 %.2f，超过 CPU 核心数 %d 的 2 倍", sysCtx.LoadAvg1, sysCtx.CPUCores),
			Category:    "performance",
			Priority:    1,
			Action:      "检查是否有异常进程或计划任务",
		})
	}

	// 备份建议（通用）
	suggestions = append(suggestions, Suggestion{
		ID:          nextID(),
		Title:       "定期备份建议",
		Description: "建议启用自动快照和异地备份策略，保护数据安全",
		Category:    "backup",
		Priority:    2,
		Action:      "配置定时快照和备份计划",
	})

	// 安全建议（通用）
	suggestions = append(suggestions, Suggestion{
		ID:          nextID(),
		Title:       "安全加固建议",
		Description: "建议启用防火墙、SSH 密钥认证、定期更新系统补丁",
		Category:    "security",
		Priority:    2,
		Action:      "检查安全配置并加固",
	})

	return suggestions
}

// parseDiagnosis 解析 AI 诊断结果.
func (a *Advisor) parseDiagnosis(answer, logContent string) *DiagnoseResponse {
	resp := &DiagnoseResponse{
		Timestamp: time.Now(),
		Severity:  "info",
	}

	// 尝试解析 JSON 根式响应
	trimmed := strings.TrimSpace(answer)
	// 查找 JSON 块
	jsonStart := strings.Index(trimmed, "{")
	jsonEnd := strings.LastIndex(trimmed, "}")
	if jsonStart >= 0 && jsonEnd > jsonStart {
		jsonStr := trimmed[jsonStart : jsonEnd+1]
		var parsed struct {
			Problem   string   `json:"problem"`
			Cause     string   `json:"cause"`
			Solutions []string `json:"solutions"`
			Severity  string   `json:"severity"`
		}
		if err := json.Unmarshal([]byte(jsonStr), &parsed); err == nil {
			resp.Problem = parsed.Problem
			resp.Cause = parsed.Cause
			resp.Solutions = parsed.Solutions
			if parsed.Severity != "" {
				resp.Severity = parsed.Severity
			}
			return resp
		}
	}

	// JSON 解析失败时，基于日志关键词做基础诊断
	resp.Problem = "系统日志异常"
	resp.Cause = answer
	resp.Solutions = []string{"请查看完整日志了解详细信息", "根据 AI 分析的建议进行排查"}
	resp.Severity = a.detectSeverity(logContent)

	return resp
}

// detectSeverity 基于关键词检测严重程度.
func (a *Advisor) detectSeverity(log string) string {
	lower := strings.ToLower(log)

	criticalKeywords := []string{"panic", "fatal", "critical", "oom", "kernel panic", "segfault", "硬件故障"}
	for _, kw := range criticalKeywords {
		if strings.Contains(lower, kw) {
			return "critical"
		}
	}

	warningKeywords := []string{"error", "fail", "warning", "warn", "timeout", "refused", "denied"}
	for _, kw := range warningKeywords {
		if strings.Contains(lower, kw) {
			return "warning"
		}
	}

	return "info"
}

// formatBytes 格式化字节数.
func formatBytes(b uint64) string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
		TB = GB * 1024
	)
	switch {
	case b >= TB:
		return fmt.Sprintf("%.2f TB", float64(b)/float64(TB))
	case b >= GB:
		return fmt.Sprintf("%.2f GB", float64(b)/float64(GB))
	case b >= MB:
		return fmt.Sprintf("%.2f MB", float64(b)/float64(MB))
	case b >= KB:
		return fmt.Sprintf("%.2f KB", float64(b)/float64(KB))
	default:
		return fmt.Sprintf("%d B", b)
	}
}

// round2 保留两位小数.
func round2(v float64) float64 {
	return math.Round(v*100) / 100
}
