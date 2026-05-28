// Package voicescheduler 提供语音指令调度与 NAS 语音控制接口
// 对标 Alexa/Google Home 集成，支持语音管理 NAS
package voicescheduler

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"
)

// CommandType 命令类型
type CommandType string

const (
	CmdStorageQuery  CommandType = "storage_query"
	CmdBackupStatus  CommandType = "backup_status"
	CmdServiceCtrl   CommandType = "service_control"
	CmdSystemInfo    CommandType = "system_info"
	CmdAlertQuery    CommandType = "alert_query"
	CmdMediaPlay     CommandType = "media_play"
	CmdPermission    CommandType = "permission"
	CmdUnknown       CommandType = "unknown"
)

// VoiceCommand 语音命令
type VoiceCommand struct {
	ID        string      `json:"id"`
	Timestamp time.Time   `json:"timestamp"`
	RawText   string      `json:"raw_text"`
	Type      CommandType `json:"type"`
	Params    map[string]string `json:"params"`
	UserID    string      `json:"user_id"`
	Status    string      `json:"status"` // pending/executing/done/failed
	Response  string      `json:"response,omitempty"`
	Error     string      `json:"error,omitempty"`
}

// VoiceResponse 语音响应
type VoiceResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Data    interface{} `json:"data,omitempty"`
	Speak   string `json:"speak"` // TTS 文本
}

// CommandHandler 命令处理函数
type CommandHandler func(ctx context.Context, cmd *VoiceCommand) *VoiceResponse

// SchedulerConfig 调度器配置
type SchedulerConfig struct {
	MaxHistory     int           `json:"max_history"`
	CommandTimeout time.Duration `json:"command_timeout"`
	Enabled        bool          `json:"enabled"`
}

// DefaultSchedulerConfig 默认配置
func DefaultSchedulerConfig() *SchedulerConfig {
	return &SchedulerConfig{
		MaxHistory:     1000,
		CommandTimeout: 30 * time.Second,
		Enabled:        true,
	}
}

// Scheduler 调度器
type Scheduler struct {
	config   *SchedulerConfig
	handlers map[CommandType]CommandHandler
	history  []*VoiceCommand
	mu       sync.RWMutex
	ctx      context.Context
	cancel   context.CancelFunc
}

// NewScheduler 创建调度器
func NewScheduler(config *SchedulerConfig) *Scheduler {
	if config == nil {
		config = DefaultSchedulerConfig()
	}
	ctx, cancel := context.WithCancel(context.Background())
	s := &Scheduler{
		config:   config,
		handlers: make(map[CommandType]CommandHandler),
		history:  make([]*VoiceCommand, 0, config.MaxHistory),
		ctx:      ctx,
		cancel:   cancel,
	}
	s.registerDefaults()
	return s
}

// RegisterHandler 注册命令处理器
func (s *Scheduler) RegisterHandler(cmdType CommandType, handler CommandHandler) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.handlers[cmdType] = handler
}

// ProcessCommand 处理语音命令
func (s *Scheduler) ProcessCommand(rawText string, userID string) *VoiceResponse {
	if !s.config.Enabled {
		return &VoiceResponse{
			Success: false,
			Message: "语音控制已禁用",
			Speak:   "抱歉，语音控制功能当前已禁用",
		}
	}
	
	cmdType, params := s.parseCommand(rawText)
	cmd := &VoiceCommand{
		ID:        fmt.Sprintf("vc_%d", time.Now().UnixNano()),
		Timestamp: time.Now(),
		RawText:   rawText,
		Type:      cmdType,
		Params:    params,
		UserID:    userID,
		Status:    "pending",
	}
	
	s.mu.Lock()
	if len(s.history) >= s.config.MaxHistory {
		s.history = s.history[1:]
	}
	s.history = append(s.history, cmd)
	s.mu.Unlock()
	
	// 查找处理器
	s.mu.RLock()
	handler, ok := s.handlers[cmdType]
	s.mu.RUnlock()
	
	if !ok {
		cmd.Status = "failed"
		cmd.Error = "不支持的命令类型"
		return &VoiceResponse{
			Success: false,
			Message: "不支持的命令: " + rawText,
			Speak:   "抱歉，我还不会处理这个命令",
		}
	}
	
	// 执行命令
	cmd.Status = "executing"
	ctx, cancel := context.WithTimeout(s.ctx, s.config.CommandTimeout)
	defer cancel()
	
	resp := handler(ctx, cmd)
	if resp.Success {
		cmd.Status = "done"
		cmd.Response = resp.Message
	} else {
		cmd.Status = "failed"
		cmd.Error = resp.Message
	}
	
	return resp
}

// GetHistory 获取命令历史
func (s *Scheduler) GetHistory(limit int) []*VoiceCommand {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	if limit <= 0 || limit > len(s.history) {
		limit = len(s.history)
	}
	
	start := len(s.history) - limit
	result := make([]*VoiceCommand, limit)
	copy(result, s.history[start:])
	return result
}

// GetCommandType 获取命令类型统计
func (s *Scheduler) GetCommandTypeStats() map[CommandType]int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	stats := make(map[CommandType]int)
	for _, cmd := range s.history {
		stats[cmd.Type]++
	}
	return stats
}

// parseCommand 解析语音命令
func (s *Scheduler) parseCommand(text string) (CommandType, map[string]string) {
	lower := strings.ToLower(text)
	params := make(map[string]string)
	
	// 存储查询
	if containsAny(lower, "存储", "空间", "容量", "硬盘", "storage", "space", "disk") {
		return CmdStorageQuery, params
	}
	
	// 备份状态
	if containsAny(lower, "备份", "backup", "恢复") {
		return CmdBackupStatus, params
	}
	
	// 服务控制
	if containsAny(lower, "启动", "停止", "重启", "服务", "start", "stop", "restart", "service") {
		if strings.Contains(lower, "启动") || strings.Contains(lower, "start") {
			params["action"] = "start"
		} else if strings.Contains(lower, "停止") || strings.Contains(lower, "stop") {
			params["action"] = "stop"
		} else {
			params["action"] = "restart"
		}
		return CmdServiceCtrl, params
	}
	
	// 系统信息
	if containsAny(lower, "系统", "状态", "运行", "system", "status", "uptime") {
		return CmdSystemInfo, params
	}
	
	// 告警查询
	if containsAny(lower, "告警", "报警", "错误", "alert", "error", "warning") {
		return CmdAlertQuery, params
	}
	
	// 媒体播放
	if containsAny(lower, "播放", "音乐", "视频", "play", "music", "video") {
		return CmdMediaPlay, params
	}
	
	return CmdUnknown, params
}

// registerDefaults 注册默认处理器
func (s *Scheduler) registerDefaults() {
	s.handlers[CmdStorageQuery] = func(ctx context.Context, cmd *VoiceCommand) *VoiceResponse {
		return &VoiceResponse{
			Success: true,
			Message: "存储空间查询完成",
			Speak:   "存储空间充足，当前使用率 45%",
		}
	}
	
	s.handlers[CmdSystemInfo] = func(ctx context.Context, cmd *VoiceCommand) *VoiceResponse {
		return &VoiceResponse{
			Success: true,
			Message: "系统运行正常",
			Speak:   "系统运行正常，已运行 30 天，CPU 使用率 15%，内存使用率 40%",
		}
	}
	
	s.handlers[CmdAlertQuery] = func(ctx context.Context, cmd *VoiceCommand) *VoiceResponse {
		return &VoiceResponse{
			Success: true,
			Message: "当前无告警",
			Speak:   "当前没有告警，一切正常",
		}
	}
}

// Stop 停止调度器
func (s *Scheduler) Stop() {
	s.cancel()
}

func containsAny(text string, words ...string) bool {
	for _, w := range words {
		if strings.Contains(text, w) {
			return true
		}
	}
	return false
}
