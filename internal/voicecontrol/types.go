// Package voicecontrol 提供语音控制NAS系统的功能
// 支持多种语音助手: Siri (HomeKit), Google Assistant, Alexa, 小爱同学
package voicecontrol

import (
	"time"
)

// VoiceProvider 语音助手提供商
type VoiceProvider string

const (
	ProviderSiri     VoiceProvider = "siri"
	ProviderGoogle   VoiceProvider = "google"
	ProviderAlexa    VoiceProvider = "alexa"
	ProviderXiaoAI   VoiceProvider = "xiaoai"
	ProviderCustom   VoiceProvider = "custom"
)

// CommandType 语音命令类型
type CommandType string

const (
	CmdFileSearch    CommandType = "file_search"
	CmdFileOpen      CommandType = "file_open"
	CmdStorageStatus CommandType = "storage_status"
	CmdBackupStart   CommandType = "backup_start"
	CmdMediaPlay     CommandType = "media_play"
	CmdMediaPause    CommandType = "media_pause"
	CmdSystemStatus  CommandType = "system_status"
	CmdDockerManage  CommandType = "docker_manage"
	CmdPermission    CommandType = "permission"
	CmdUnknown       CommandType = "unknown"
)

// Intent 识别后的意图
type Intent struct {
	Command    CommandType          `json:"command"`
	Parameters map[string]string    `json:"parameters"`
	Confidence float64              `json:"confidence"`
	RawText    string               `json:"raw_text"`
	Language   string               `json:"language"`
}

// VoiceCommand 语音命令
type VoiceCommand struct {
	ID         string        `json:"id"`
	UserID     string        `json:"user_id"`
	Provider   VoiceProvider `json:"provider"`
	RawAudio   string        `json:"raw_audio,omitempty"`
	RawText    string        `json:"raw_text"`
	Intent     Intent        `json:"intent"`
	Processed  bool          `json:"processed"`
	Result     *CommandResult `json:"result,omitempty"`
	CreatedAt  time.Time     `json:"created_at"`
	ProcessedAt *time.Time   `json:"processed_at,omitempty"`
}

// CommandResult 命令执行结果
type CommandResult struct {
	Success bool        `json:"success"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
}

// VoiceResponse 语音响应
type VoiceResponse struct {
	Text       string `json:"text"`
	AudioURL   string `json:"audio_url,omitempty"`
	SSML       string `json:"ssml,omitempty"`
	ShouldSpeak bool  `json:"should_speak"`
}

// VoiceProfile 用户语音配置
type VoiceProfile struct {
	UserID       string        `json:"user_id"`
	Language     string        `json:"language"`
	VoicePrint   string        `json:"voice_print,omitempty"`
	Provider     VoiceProvider `json:"provider"`
	WakeWordEnabled bool       `json:"wake_word_enabled"`
	WakeWord     string        `json:"wake_word,omitempty"`
	CreatedAt    time.Time     `json:"created_at"`
	UpdatedAt    time.Time     `json:"updated_at"`
}

// ConversationTurn 对话轮次
type ConversationTurn struct {
	ID         string         `json:"id"`
	UserID     string         `json:"user_id"`
	Command    *VoiceCommand  `json:"command"`
	Response   *VoiceResponse `json:"response"`
	TurnIndex  int            `json:"turn_index"`
	CreatedAt  time.Time      `json:"created_at"`
}

// Conversation 一次完整对话
type Conversation struct {
	ID        string              `json:"id"`
	UserID    string              `json:"user_id"`
	Turns     []*ConversationTurn `json:"turns"`
	Context   map[string]string   `json:"context"`
	StartedAt time.Time           `json:"started_at"`
	EndedAt   *time.Time          `json:"ended_at,omitempty"`
}

// Permission 语音操作权限
type Permission struct {
	UserID    string      `json:"user_id"`
	Command   CommandType `json:"command"`
	Allowed   bool        `json:"allowed"`
	Resources []string    `json:"resources,omitempty"`
}

// Config 语音控制配置
type Config struct {
	Enabled           bool            `json:"enabled"`
	DefaultProvider   VoiceProvider   `json:"default_provider"`
	SupportedProviders []VoiceProvider `json:"supported_providers"`
	WakeWordEnabled   bool            `json:"wake_word_enabled"`
	DefaultLanguage   string          `json:"default_language"`
	MaxAudioDurationSec int           `json:"max_audio_duration_sec"`
}

// DefaultConfig 返回默认配置
func DefaultConfig() Config {
	return Config{
		Enabled:             true,
		DefaultProvider:     ProviderCustom,
		SupportedProviders:  []VoiceProvider{ProviderSiri, ProviderGoogle, ProviderAlexa, ProviderXiaoAI, ProviderCustom},
		WakeWordEnabled:     false,
		DefaultLanguage:     "zh-CN",
		MaxAudioDurationSec: 30,
	}
}
