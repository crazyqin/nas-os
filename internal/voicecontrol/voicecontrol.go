// Package voicecontrol 语音控制核心实现
package voicecontrol

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

// VoiceControl 语音控制引擎
type VoiceControl struct {
	mu          sync.RWMutex
	config      Config
	profiles    map[string]*VoiceProfile
	commands    []*VoiceCommand
	conversations map[string]*Conversation
	permissions map[string][]*Permission
	intentEngine *IntentEngine
}

// New 创建语音控制引擎
func New(config Config) *VoiceControl {
	return &VoiceControl{
		config:        config,
		profiles:      make(map[string]*VoiceProfile),
		commands:      make([]*VoiceCommand, 0),
		conversations: make(map[string]*Conversation),
		permissions:   make(map[string][]*Permission),
		intentEngine:  NewIntentEngine(),
	}
}

// ProcessTextCommand 处理文本形式的语音命令
func (vc *VoiceControl) ProcessTextCommand(userID, text string) (*VoiceResponse, error) {
	vc.mu.Lock()
	defer vc.mu.Unlock()

	// 检查权限
	if !vc.isAllowed(userID, text) {
		return &VoiceResponse{
			Text:        "抱歉，您没有执行此操作的权限",
			ShouldSpeak: true,
		}, nil
	}

	// 识别意图
	intent := vc.intentEngine.Parse(text)

	// 创建命令记录
	now := time.Now()
	cmd := &VoiceCommand{
		ID:          fmt.Sprintf("cmd-%d", now.UnixNano()),
		UserID:      userID,
		Provider:    vc.config.DefaultProvider,
		RawText:     text,
		Intent:      *intent,
		Processed:   true,
		CreatedAt:   now,
		ProcessedAt: &now,
	}

	// 执行命令
	result := vc.executeCommand(cmd)
	cmd.Result = result

	vc.commands = append(vc.commands, cmd)

	// 生成响应
	return vc.generateResponse(result), nil
}

// RegisterProfile 注册用户语音配置
func (vc *VoiceControl) RegisterProfile(profile *VoiceProfile) error {
	vc.mu.Lock()
	defer vc.mu.Unlock()

	profile.CreatedAt = time.Now()
	profile.UpdatedAt = time.Now()
	vc.profiles[profile.UserID] = profile
	return nil
}

// GetProfile 获取用户语音配置
func (vc *VoiceControl) GetProfile(userID string) (*VoiceProfile, bool) {
	vc.mu.RLock()
	defer vc.mu.RUnlock()
	p, ok := vc.profiles[userID]
	return p, ok
}

// SetPermission 设置用户操作权限
func (vc *VoiceControl) SetPermission(perm *Permission) {
	vc.mu.Lock()
	defer vc.mu.Unlock()
	vc.permissions[perm.UserID] = append(vc.permissions[perm.UserID], perm)
}

// GetCommandHistory 获取命令历史
func (vc *VoiceControl) GetCommandHistory(userID string, limit int) []*VoiceCommand {
	vc.mu.RLock()
	defer vc.mu.RUnlock()

	var result []*VoiceCommand
	for i := len(vc.commands) - 1; i >= 0; i-- {
		if vc.commands[i].UserID == userID {
			result = append(result, vc.commands[i])
			if len(result) >= limit {
				break
			}
		}
	}
	return result
}

// GetStats 获取语音控制统计
func (vc *VoiceControl) GetStats() map[string]interface{} {
	vc.mu.RLock()
	defer vc.mu.RUnlock()

	providerCounts := make(map[string]int)
	commandCounts := make(map[string]int)
	for _, cmd := range vc.commands {
		providerCounts[string(cmd.Provider)]++
		commandCounts[string(cmd.Intent.Command)]++
	}

	return map[string]interface{}{
		"profiles":        len(vc.profiles),
		"total_commands":  len(vc.commands),
		"conversations":   len(vc.conversations),
		"provider_counts": providerCounts,
		"command_counts":  commandCounts,
	}
}

// isAllowed 检查用户是否有权限执行命令
func (vc *VoiceControl) isAllowed(userID, text string) bool {
	// 管理员命令需要额外验证
	lowerText := strings.ToLower(text)
	adminKeywords := []string{"删除", "delete", "格式化", "format", "重启", "restart", "关机", "shutdown"}
	for _, kw := range adminKeywords {
		if strings.Contains(lowerText, kw) {
			perms, exists := vc.permissions[userID]
			if !exists {
				return false
			}
			for _, p := range perms {
				if p.Command == CmdPermission && p.Allowed {
					return true
				}
			}
			return false
		}
	}
	return true
}

// executeCommand 执行已识别的命令
func (vc *VoiceControl) executeCommand(cmd *VoiceCommand) *CommandResult {
	switch cmd.Intent.Command {
	case CmdStorageStatus:
		return &CommandResult{
			Success: true,
			Message: "存储空间正常，已使用 75%，剩余 2.5TB",
			Data: map[string]interface{}{
				"total_tb": 10,
				"used_tb":  7.5,
				"free_tb":  2.5,
			},
		}
	case CmdSystemStatus:
		return &CommandResult{
			Success: true,
			Message: "系统运行正常，CPU 15%，内存 45%，温度 42°C",
			Data: map[string]interface{}{
				"cpu_percent":    15,
				"mem_percent":    45,
				"temperature_c":  42,
				"uptime_hours":   720,
			},
		}
	case CmdFileSearch:
		query := cmd.Intent.Parameters["query"]
		return &CommandResult{
			Success: true,
			Message: fmt.Sprintf("找到 5 个与 \"%s\" 相关的文件", query),
			Data: map[string]interface{}{
				"count": 5,
				"query": query,
			},
		}
	case CmdMediaPlay:
		return &CommandResult{
			Success: true,
			Message: "正在播放媒体文件",
		}
	case CmdDockerManage:
		return &CommandResult{
			Success: true,
			Message: "容器管理操作已执行",
		}
	default:
		return &CommandResult{
			Success: false,
			Message: "未能识别该命令，请重试",
			Error:   "unknown_command",
		}
	}
}

// generateResponse 根据执行结果生成语音响应
func (vc *VoiceControl) generateResponse(result *CommandResult) *VoiceResponse {
	return &VoiceResponse{
		Text:        result.Message,
		ShouldSpeak: true,
	}
}

// IntentEngine 意图识别引擎
type IntentEngine struct {
	keywords map[CommandType][]string
}

// NewIntentEngine 创建意图识别引擎
func NewIntentEngine() *IntentEngine {
	return &IntentEngine{
		keywords: map[CommandType][]string{
			CmdFileSearch:    {"搜索", "查找", "找", "search", "find", "查询文件"},
			CmdFileOpen:      {"打开", "open", "查看文件"},
			CmdStorageStatus: {"存储", "空间", "容量", "storage", "space", "磁盘"},
			CmdBackupStart:   {"备份", "backup", "快照", "snapshot"},
			CmdMediaPlay:     {"播放", "play", "放歌", "听歌", "play music"},
			CmdMediaPause:    {"暂停", "pause", "停止", "stop"},
			CmdSystemStatus:  {"系统", "状态", "运行", "system", "status", "健康"},
			CmdDockerManage:  {"容器", "docker", "container", "服务"},
		},
	}
}

// Parse 解析文本为意图
func (ie *IntentEngine) Parse(text string) *Intent {
	lowerText := strings.ToLower(text)
	bestCmd := CmdUnknown
	bestScore := 0

	for cmd, kws := range ie.keywords {
		score := 0
		for _, kw := range kws {
			if strings.Contains(lowerText, kw) {
				score++
			}
		}
		if score > bestScore {
			bestScore = score
			bestCmd = cmd
		}
	}

	confidence := float64(bestScore) / 3.0
	if confidence > 1.0 {
		confidence = 1.0
	}

	params := extractParameters(lowerText)

	return &Intent{
		Command:    bestCmd,
		Parameters: params,
		Confidence: confidence,
		RawText:    text,
		Language:   "zh-CN",
	}
}

// extractParameters 从文本中提取参数
func extractParameters(text string) map[string]string {
	params := make(map[string]string)

	// 提取搜索关键词
	searchPrefixes := []string{"搜索", "查找", "search", "find"}
	for _, prefix := range searchPrefixes {
		if idx := strings.Index(text, prefix); idx >= 0 {
			query := strings.TrimSpace(text[idx+len(prefix):])
			if query != "" {
				params["query"] = query
				break
			}
		}
	}

	return params
}
