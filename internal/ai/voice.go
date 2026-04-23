// Package ai - Voice Control Interface
// 参考铁威马AI NAS的语音控制功能，实现语音命令识别和执行
package ai

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// VoiceCommand 语音命令
type VoiceCommand struct {
	ID        string            `json:"id"`
	Text      string            `json:"text"`      // 识别后的文本
	Intent    string            `json:"intent"`    // 识别的意图
	Params    map[string]string `json:"params"`    // 提取的参数
	Confidence float64          `json:"confidence"` // 识别置信度
	Language  string            `json:"language"`  // 语言 (zh-CN, en-US等)
	Status    string            `json:"status"`    // pending, processing, completed, failed
	Result    *CommandResult    `json:"result,omitempty"`
	Error     string            `json:"error,omitempty"`
	CreatedAt time.Time         `json:"createdAt"`
}

// CommandResult 命令执行结果
type CommandResult struct {
	Action    string                 `json:"action"`    // 执行的动作
	Message   string                 `json:"message"`   // 返回的消息
	Data      map[string]interface{} `json:"data"`      // 返回的数据
	Success   bool                   `json:"success"`   // 是否成功
	Timestamp time.Time              `json:"timestamp"` // 执行时间
}

// VoiceIntent 语音意图定义
type VoiceIntent struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`        // 意图名称
	Description string            `json:"description"` // 描述
	Patterns    []string          `json:"patterns"`    // 匹配模式 (正则)
	Params      []IntentParam      `json:"params"`      // 参数定义
	Handler     string            `json:"handler"`     // 处理器名称
	Enabled     bool              `json:"enabled"`     // 是否启用
	Priority    int               `json:"priority"`    // 优先级
}

// IntentParam 意图参数
type IntentParam struct {
	Name        string `json:"name"`        // 参数名
	Type        string `json:"type"`        // 类型: string, number, path, date
	Required    bool   `json:"required"`    // 是否必需
	Default     string `json:"default"`     // 默认值
	Description string `json:"description"` // 描述
}

// VoiceController 语音控制器
type VoiceController struct {
	mu          sync.RWMutex
	intents     map[string]*VoiceIntent
	handlers    map[string]CommandHandler
	history     []*VoiceCommand
	maxHistory  int
	speechToText SpeechToTextEngine
	enabled     bool
}

// CommandHandler 命令处理器接口
type CommandHandler interface {
	Handle(ctx context.Context, cmd *VoiceCommand) (*CommandResult, error)
}

// SpeechToTextEngine 语音转文本引擎接口
type SpeechToTextEngine interface {
	Transcribe(ctx context.Context, audioData []byte, language string) (string, float64, error)
	IsAvailable() bool
}

// VoiceConfig 语音控制配置
type VoiceConfig struct {
	Enabled      bool     `json:"enabled"`
	Languages    []string `json:"languages"`    // 支持的语言
	MaxHistory   int      `json:"maxHistory"`   // 历史记录最大数量
	ConfidenceThreshold float64 `json:"confidenceThreshold"` // 置信度阈值
}

// DefaultVoiceConfig 默认配置
func DefaultVoiceConfig() VoiceConfig {
	return VoiceConfig{
		Enabled:      true,
		Languages:    []string{"zh-CN", "en-US", "ja-JP"},
		MaxHistory:   100,
		ConfidenceThreshold: 0.7,
	}
}

// NewVoiceController 创建语音控制器
func NewVoiceController(config VoiceConfig) *VoiceController {
	vc := &VoiceController{
		intents:    make(map[string]*VoiceIntent),
		handlers:   make(map[string]CommandHandler),
		history:    make([]*VoiceCommand, 0),
		maxHistory: config.MaxHistory,
		enabled:    config.Enabled,
	}

	// 注册默认意图
	vc.registerDefaultIntents()
	
	return vc
}

// registerDefaultIntents 注册默认意图
func (vc *VoiceController) registerDefaultIntents() {
	defaultIntents := []VoiceIntent{
		// 文件操作
		{
			ID:          "file_list",
			Name:        "列出文件",
			Description: "列出指定目录的文件",
			Patterns: []string{
				`(?i)列出(.+)的文件`,
				`(?i)显示(.+)目录`,
				`(?i)查看(.+)文件夹`,
				`(?i)list files in (.+)`,
				`(?i)show (.+) folder`,
			},
			Params: []IntentParam{
				{Name: "path", Type: "path", Required: true, Description: "目录路径"},
			},
			Handler:  "file_list",
			Enabled:  true,
			Priority: 10,
		},
		{
			ID:          "file_search",
			Name:        "搜索文件",
			Description: "搜索指定名称的文件",
			Patterns: []string{
				`(?i)搜索(.+)文件`,
				`(?i)查找(.+)`,
				`(?i)找(.+)文件`,
				`(?i)search for (.+)`,
				`(?i)find (.+)`,
			},
			Params: []IntentParam{
				{Name: "query", Type: "string", Required: true, Description: "搜索关键词"},
			},
			Handler:  "file_search",
			Enabled:  true,
			Priority: 10,
		},
		{
			ID:          "file_copy",
			Name:        "复制文件",
			Description: "复制文件到指定位置",
			Patterns: []string{
				`(?i)复制(.+)到(.+)`,
				`(?i)把(.+)复制到(.+)`,
				`(?i)copy (.+) to (.+)`,
			},
			Params: []IntentParam{
				{Name: "source", Type: "path", Required: true, Description: "源文件"},
				{Name: "target", Type: "path", Required: true, Description: "目标位置"},
			},
			Handler:  "file_copy",
			Enabled:  true,
			Priority: 9,
		},
		{
			ID:          "file_move",
			Name:        "移动文件",
			Description: "移动文件到指定位置",
			Patterns: []string{
				`(?i)移动(.+)到(.+)`,
				`(?i)把(.+)移到(.+)`,
				`(?i)move (.+) to (.+)`,
			},
			Params: []IntentParam{
				{Name: "source", Type: "path", Required: true, Description: "源文件"},
				{Name: "target", Type: "path", Required: true, Description: "目标位置"},
			},
			Handler:  "file_move",
			Enabled:  true,
			Priority: 9,
		},
		{
			ID:          "file_delete",
			Name:        "删除文件",
			Description: "删除指定文件",
			Patterns: []string{
				`(?i)删除(.+)`,
				`(?i)移除(.+)`,
				`(?i)delete (.+)`,
				`(?i)remove (.+)`,
			},
			Params: []IntentParam{
				{Name: "path", Type: "path", Required: true, Description: "文件路径"},
			},
			Handler:  "file_delete",
			Enabled:  true,
			Priority: 8,
		},

		// 系统操作
		{
			ID:          "system_status",
			Name:        "系统状态",
			Description: "查看系统运行状态",
			Patterns: []string{
				`(?i)系统状态`,
				`(?i)查看状态`,
				`(?i)nas状态`,
				`(?i)system status`,
				`(?i)check status`,
			},
			Params: []IntentParam{},
			Handler:  "system_status",
			Enabled:  true,
			Priority: 10,
		},
		{
			ID:          "storage_info",
			Name:        "存储信息",
			Description: "查看存储空间使用情况",
			Patterns: []string{
				`(?i)存储空间`,
				`(?i)磁盘空间`,
				`(?i)硬盘容量`,
				`(?i)storage info`,
				`(?i)disk space`,
			},
			Params: []IntentParam{},
			Handler:  "storage_info",
			Enabled:  true,
			Priority: 10,
		},

		// 照片操作
		{
			ID:          "photo_search",
			Name:        "搜索照片",
			Description: "搜索照片",
			Patterns: []string{
				`(?i)搜索照片(.+)`,
				`(?i)找照片(.+)`,
				`(?i)查看(.+)的照片`,
				`(?i)search photos (.+)`,
				`(?i)find photos (.+)`,
			},
			Params: []IntentParam{
				{Name: "query", Type: "string", Required: true, Description: "搜索关键词"},
			},
			Handler:  "photo_search",
			Enabled:  true,
			Priority: 10,
		},
		{
			ID:          "photo_album",
			Name:        "查看相册",
			Description: "查看或创建相册",
			Patterns: []string{
				`(?i)查看相册`,
				`(?i)显示相册`,
				`(?i)我的相册`,
				`(?i)show albums`,
				`(?i)my albums`,
			},
			Params: []IntentParam{},
			Handler:  "photo_album",
			Enabled:  true,
			Priority: 10,
		},

		// 备份操作
		{
			ID:          "backup_start",
			Name:        "开始备份",
			Description: "启动备份任务",
			Patterns: []string{
				`(?i)开始备份`,
				`(?i)执行备份`,
				`(?i)备份(.+)`,
				`(?i)start backup`,
				`(?i)run backup`,
			},
			Params: []IntentParam{
				{Name: "target", Type: "string", Required: false, Default: "default", Description: "备份目标"},
			},
			Handler:  "backup_start",
			Enabled:  true,
			Priority: 9,
		},
		{
			ID:          "backup_status",
			Name:        "备份状态",
			Description: "查看备份状态",
			Patterns: []string{
				`(?i)备份状态`,
				`(?i)查看备份`,
				`(?i)backup status`,
			},
			Params: []IntentParam{},
			Handler:  "backup_status",
			Enabled:  true,
			Priority: 10,
		},

		// 共享操作
		{
			ID:          "share_create",
			Name:        "创建共享",
			Description: "创建文件共享",
			Patterns: []string{
				`(?i)共享(.+)`,
				`(?i)分享(.+)`,
				`(?i)创建共享(.+)`,
				`(?i)share (.+)`,
				`(?i)create share (.+)`,
			},
			Params: []IntentParam{
				{Name: "path", Type: "path", Required: true, Description: "共享路径"},
			},
			Handler:  "share_create",
			Enabled:  true,
			Priority: 9,
		},

		// 帮助
		{
			ID:          "help",
			Name:        "帮助",
			Description: "显示可用命令帮助",
			Patterns: []string{
				`(?i)帮助`,
				`(?i)help`,
				`(?i)我能做什么`,
				`(?i)what can you do`,
			},
			Params: []IntentParam{},
			Handler:  "help",
			Enabled:  true,
			Priority: 10,
		},
	}

	for _, intent := range defaultIntents {
		vc.intents[intent.ID] = &intent
	}
}

// RegisterHandler 注册命令处理器
func (vc *VoiceController) RegisterHandler(name string, handler CommandHandler) {
	vc.mu.Lock()
	defer vc.mu.Unlock()
	vc.handlers[name] = handler
}

// SetSpeechEngine 设置语音转文本引擎
func (vc *VoiceController) SetSpeechEngine(engine SpeechToTextEngine) {
	vc.mu.Lock()
	defer vc.mu.Unlock()
	vc.speechToText = engine
}

// ProcessVoice 处理语音输入
func (vc *VoiceController) ProcessVoice(ctx context.Context, audioData []byte, language string) (*VoiceCommand, error) {
	if !vc.enabled {
		return nil, fmt.Errorf("语音控制未启用")
	}

	// 1. 语音转文本
	var text string
	var confidence float64
	var err error

	if vc.speechToText != nil && vc.speechToText.IsAvailable() {
		text, confidence, err = vc.speechToText.Transcribe(ctx, audioData, language)
		if err != nil {
			return nil, fmt.Errorf("语音识别失败: %w", err)
		}
	} else {
		// 如果没有语音引擎，返回提示
		return nil, fmt.Errorf("语音识别引擎未配置")
	}

	// 2. 创建命令对象
	cmd := &VoiceCommand{
		ID:         uuid.New().String(),
		Text:       text,
		Confidence: confidence,
		Language:   language,
		Status:     "pending",
		CreatedAt:  time.Now(),
	}

	// 3. 解析意图
	if err := vc.parseIntent(cmd); err != nil {
		cmd.Status = "failed"
		cmd.Error = err.Error()
		vc.addToHistory(cmd)
		return cmd, nil
	}

	// 4. 执行命令
	result, err := vc.executeCommand(ctx, cmd)
	if err != nil {
		cmd.Status = "failed"
		cmd.Error = err.Error()
	} else {
		cmd.Status = "completed"
		cmd.Result = result
	}

	vc.addToHistory(cmd)
	return cmd, nil
}

// ProcessText 处理文本命令（直接输入）
func (vc *VoiceController) ProcessText(ctx context.Context, text string, language string) (*VoiceCommand, error) {
	if !vc.enabled {
		return nil, fmt.Errorf("语音控制未启用")
	}

	cmd := &VoiceCommand{
		ID:         uuid.New().String(),
		Text:       text,
		Confidence: 1.0, // 文本输入置信度为1
		Language:   language,
		Status:     "pending",
		CreatedAt:  time.Now(),
	}

	// 解析意图
	if err := vc.parseIntent(cmd); err != nil {
		cmd.Status = "failed"
		cmd.Error = err.Error()
		vc.addToHistory(cmd)
		return cmd, nil
	}

	// 执行命令
	result, err := vc.executeCommand(ctx, cmd)
	if err != nil {
		cmd.Status = "failed"
		cmd.Error = err.Error()
	} else {
		cmd.Status = "completed"
		cmd.Result = result
	}

	vc.addToHistory(cmd)
	return cmd, nil
}

// parseIntent 解析命令意图
func (vc *VoiceController) parseIntent(cmd *VoiceCommand) error {
	vc.mu.RLock()
	defer vc.mu.RUnlock()

	text := strings.ToLower(cmd.Text)

	// 按优先级排序意图
	sortedIntents := make([]*VoiceIntent, 0, len(vc.intents))
	for _, intent := range vc.intents {
		if intent.Enabled {
			sortedIntents = append(sortedIntents, intent)
		}
	}

	// 按优先级降序排序
	sortIntentsByPriority(sortedIntents)

	// 尝试匹配每个意图
	for _, intent := range sortedIntents {
		for _, pattern := range intent.Patterns {
			re, err := regexp.Compile(pattern)
			if err != nil {
				log.Printf("⚠️ 正则表达式错误: %s: %v", pattern, err)
				continue
			}

			matches := re.FindStringSubmatch(text)
			if matches == nil {
				continue
			}

			// 匹配成功，设置意图和参数
			cmd.Intent = intent.ID

			// 提取参数
			cmd.Params = make(map[string]string)
			for i, param := range intent.Params {
				if i+1 < len(matches) && matches[i+1] != "" {
					cmd.Params[param.Name] = matches[i+1]
				} else if param.Default != "" {
					cmd.Params[param.Name] = param.Default
				} else if param.Required {
					return fmt.Errorf("缺少必需参数: %s", param.Name)
				}
			}

			return nil
		}
	}

	return fmt.Errorf("无法识别命令意图: %s", cmd.Text)
}

// sortIntentsByPriority 按优先级排序意图
func sortIntentsByPriority(intents []*VoiceIntent) {
	for i := 0; i < len(intents)-1; i++ {
		for j := i + 1; j < len(intents); j++ {
			if intents[j].Priority > intents[i].Priority {
				intents[i], intents[j] = intents[j], intents[i]
			}
		}
	}
}

// executeCommand 执行命令
func (vc *VoiceController) executeCommand(ctx context.Context, cmd *VoiceCommand) (*CommandResult, error) {
	vc.mu.RLock()
	defer vc.mu.RUnlock()

	intent, exists := vc.intents[cmd.Intent]
	if !exists {
		return nil, fmt.Errorf("未知的意图: %s", cmd.Intent)
	}

	handler, exists := vc.handlers[intent.Handler]
	if !exists {
		// 返回默认处理结果
		return &CommandResult{
			Action:    intent.Handler,
			Message:   fmt.Sprintf("已识别命令: %s，但处理器未注册", intent.Name),
			Success:   false,
			Timestamp: time.Now(),
		}, nil
	}

	cmd.Status = "processing"
	return handler.Handle(ctx, cmd)
}

// addToHistory 添加到历史记录
func (vc *VoiceController) addToHistory(cmd *VoiceCommand) {
	vc.mu.Lock()
	defer vc.mu.Unlock()

	vc.history = append(vc.history, cmd)

	// 限制历史记录数量
	if len(vc.history) > vc.maxHistory {
		vc.history = vc.history[len(vc.history)-vc.maxHistory:]
	}
}

// GetHistory 获取历史记录
func (vc *VoiceController) GetHistory(limit int) []*VoiceCommand {
	vc.mu.RLock()
	defer vc.mu.RUnlock()

	if limit <= 0 || limit > len(vc.history) {
		limit = len(vc.history)
	}

	result := make([]*VoiceCommand, limit)
	copy(result, vc.history[len(vc.history)-limit:])
	return result
}

// GetIntents 获取所有意图
func (vc *VoiceController) GetIntents() []VoiceIntent {
	vc.mu.RLock()
	defer vc.mu.RUnlock()

	result := make([]VoiceIntent, 0, len(vc.intents))
	for _, intent := range vc.intents {
		result = append(result, *intent)
	}
	return result
}

// AddIntent 添加自定义意图
func (vc *VoiceController) AddIntent(intent VoiceIntent) error {
	vc.mu.Lock()
	defer vc.mu.Unlock()

	intent.ID = fmt.Sprintf("custom_%s_%d", intent.Name, time.Now().UnixNano())
	vc.intents[intent.ID] = &intent
	return nil
}

// Enable 启用/禁用语音控制
func (vc *VoiceController) Enable(enabled bool) {
	vc.mu.Lock()
	defer vc.mu.Unlock()
	vc.enabled = enabled
}

// IsEnabled 检查是否启用
func (vc *VoiceController) IsEnabled() bool {
	vc.mu.RLock()
	defer vc.mu.RUnlock()
	return vc.enabled
}

// VoiceAPIHandler 语音API处理器
type VoiceAPIHandler struct {
	controller *VoiceController
}

// NewVoiceAPIHandler 创建API处理器
func NewVoiceAPIHandler(controller *VoiceController) *VoiceAPIHandler {
	return &VoiceAPIHandler{controller: controller}
}

// HandleProcess 处理语音输入API
func (h *VoiceAPIHandler) HandleProcess(c *gin.Context) {
	var req struct {
		Text     string `json:"text"`     // 文本命令（可选）
		Language string `json:"language"` // 语言
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "无效的请求参数",
		})
		return
	}

	// 默认语言
	if req.Language == "" {
		req.Language = "zh-CN"
	}

	ctx := c.Request.Context()
	
	// 处理文本命令
	cmd, err := h.controller.ProcessText(ctx, req.Text, req.Language)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    cmd,
	})
}

// HandleIntents 获取意图列表API
func (h *VoiceAPIHandler) HandleIntents(c *gin.Context) {
	intents := h.controller.GetIntents()
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    intents,
	})
}

// HandleHistory 获取历史记录API
func (h *VoiceAPIHandler) HandleHistory(c *gin.Context) {
	limit := 20
	if l := c.Query("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 {
			limit = parsed
		}
	}

	history := h.controller.GetHistory(limit)
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    history,
	})
}

// RegisterRoutes 注册API路由
func (h *VoiceAPIHandler) RegisterRoutes(r *gin.RouterGroup) {
	voice := r.Group("/voice")
	{
		voice.POST("/process", h.HandleProcess)
		voice.GET("/intents", h.HandleIntents)
		voice.GET("/history", h.HandleHistory)
	}
}

// MockSpeechEngine 模拟语音引擎（用于测试）
type MockSpeechEngine struct{}

func (e *MockSpeechEngine) Transcribe(ctx context.Context, audioData []byte, language string) (string, float64, error) {
	return "系统状态", 0.95, nil
}

func (e *MockSpeechEngine) IsAvailable() bool {
	return true
}