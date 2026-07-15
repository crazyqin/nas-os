// Package voicehub 提供多平台语音助手集成功能，支持 Alexa、Google Home、Siri、小爱同学等。
// 提供语音命令解析与路由、语音场景管理、设备控制、TTS 文本转语音等功能。
package voicehub

import "time"

// VoicePlatform 语音助手平台.
type VoicePlatform string

const (
	PlatformAlexa  VoicePlatform = "alexa"
	PlatformGoogle VoicePlatform = "google_home"
	PlatformSiri   VoicePlatform = "siri"
	PlatformXiaoAi VoicePlatform = "xiaoai"
	PlatformLocal  VoicePlatform = "local"
)

// VoiceLanguage 语音语言.
type VoiceLanguage string

const (
	LangChinese  VoiceLanguage = "zh-CN"
	LangEnglish  VoiceLanguage = "en-US"
	LangJapanese VoiceLanguage = "ja-JP"
)

// CommandType 设备控制命令类型.
type CommandType string

const (
	CmdPowerOn    CommandType = "power_on"
	CmdPowerOff   CommandType = "power_off"
	CmdVolumeUp   CommandType = "volume_up"
	CmdVolumeDown CommandType = "volume_down"
	CmdSetVolume  CommandType = "set_volume"
	CmdPlay       CommandType = "play"
	CmdPause      CommandType = "pause"
	CmdStop       CommandType = "stop"
	CmdNext       CommandType = "next"
	CmdPrevious   CommandType = "previous"
	CmdMute       CommandType = "mute"
	CmdUnmute     CommandType = "unmute"
)

// SceneType 语音场景类型.
type SceneType string

const (
	SceneMovieMode    SceneType = "movie_mode"
	SceneSecurityMode SceneType = "security_mode"
	SceneEnergySaving SceneType = "energy_saving"
	SceneSleepMode    SceneType = "sleep_mode"
	SceneWakeUpMode   SceneType = "wake_up_mode"
	ScenePartyMode    SceneType = "party_mode"
	SceneWorkMode     SceneType = "work_mode"
	SceneCustom       SceneType = "custom"
)

// CommandStatus 命令执行状态.
type CommandStatus string

const (
	CommandStatusPending    CommandStatus = "pending"
	CommandStatusProcessing CommandStatus = "processing"
	CommandStatusSuccess    CommandStatus = "success"
	CommandStatusFailed     CommandStatus = "failed"
	CommandStatusTimeout    CommandStatus = "timeout"
)

// VoiceCommand 语音命令请求.
type VoiceCommand struct {
	ID         string        `json:"id"`
	Platform   VoicePlatform `json:"platform"`
	Language   VoiceLanguage `json:"language"`
	Query      string        `json:"query" binding:"required"`
	SessionID  string        `json:"session_id,omitempty"`
	DeviceInfo *DeviceInfo   `json:"device_info,omitempty"`
	Timestamp  time.Time     `json:"timestamp"`
}

// DeviceInfo 设备信息.
type DeviceInfo struct {
	DeviceID   string `json:"device_id"`
	DeviceName string `json:"device_name"`
	DeviceType string `json:"device_type"`
	Room       string `json:"room,omitempty"`
	Zone       string `json:"zone,omitempty"`
}

// VoiceResponse 语音命令响应.
type VoiceResponse struct {
	ID          string         `json:"id"`
	CommandID   string         `json:"command_id"`
	Status      CommandStatus  `json:"status"`
	TextReply   string         `json:"text_reply"`
	SsmlReply   string         `json:"ssml_reply,omitempty"`
	AudioURL    string         `json:"audio_url,omitempty"`
	Actions     []DeviceAction `json:"actions,omitempty"`
	Scene       *SceneInfo     `json:"scene,omitempty"`
	Suggestions []string       `json:"suggestions,omitempty"`
	Duration    time.Duration  `json:"duration"`
	CreatedAt   time.Time      `json:"created_at"`
}

// DeviceAction 设备动作.
type DeviceAction struct {
	DeviceID    string                 `json:"device_id"`
	DeviceName  string                 `json:"device_name,omitempty"`
	CommandType CommandType            `json:"command_type"`
	Parameters  map[string]interface{} `json:"parameters,omitempty"`
	Status      CommandStatus          `json:"status"`
	Error       string                 `json:"error,omitempty"`
}

// SceneInfo 场景信息.
type SceneInfo struct {
	ID          string         `json:"id"`
	Name        string         `json:"name"`
	Type        SceneType      `json:"type"`
	Description string         `json:"description,omitempty"`
	Devices     []DeviceAction `json:"devices"`
	IsActive    bool           `json:"is_active"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
}

// SceneRequest 场景请求.
type SceneRequest struct {
	Name        string         `json:"name" binding:"required"`
	Type        SceneType      `json:"type" binding:"required"`
	Description string         `json:"description,omitempty"`
	Devices     []DeviceAction `json:"devices" binding:"required,min=1"`
}

// TTSRequest 文本转语音请求.
type TTSRequest struct {
	Text     string        `json:"text" binding:"required"`
	Language VoiceLanguage `json:"language,omitempty"`
	Voice    string        `json:"voice,omitempty"`
	Speed    float64       `json:"speed,omitempty"`
	Pitch    float64       `json:"pitch,omitempty"`
	Volume   float64       `json:"volume,omitempty"`
	Format   string        `json:"format,omitempty"`
}

// TTSResponse 文本转语音响应.
type TTSResponse struct {
	ID        string        `json:"id"`
	AudioURL  string        `json:"audio_url"`
	AudioData []byte        `json:"audio_data,omitempty"`
	Duration  time.Duration `json:"duration"`
	Format    string        `json:"format"`
	CreatedAt time.Time     `json:"created_at"`
}

// WakeWordConfig 唤醒词配置.
type WakeWordConfig struct {
	ID          string        `json:"id"`
	WakeWord    string        `json:"wake_word" binding:"required"`
	Platform    VoicePlatform `json:"platform"`
	Language    VoiceLanguage `json:"language"`
	Sensitivity float64       `json:"sensitivity"`
	IsActive    bool          `json:"is_active"`
	CreatedAt   time.Time     `json:"created_at"`
}

// CustomCommand 自定义语音命令.
type CustomCommand struct {
	ID        string         `json:"id"`
	Name      string         `json:"name" binding:"required"`
	Pattern   string         `json:"pattern" binding:"required"`
	Language  VoiceLanguage  `json:"language"`
	Response  string         `json:"response" binding:"required"`
	Actions   []DeviceAction `json:"actions,omitempty"`
	IsActive  bool           `json:"is_active"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
}

// CustomCommandRequest 自定义命令请求.
type CustomCommandRequest struct {
	Name     string         `json:"name" binding:"required"`
	Pattern  string         `json:"pattern" binding:"required"`
	Language VoiceLanguage  `json:"language,omitempty"`
	Response string         `json:"response" binding:"required"`
	Actions  []DeviceAction `json:"actions,omitempty"`
}

// ReplyTemplate 语音回复模板.
type ReplyTemplate struct {
	ID        string        `json:"id"`
	Name      string        `json:"name" binding:"required"`
	Language  VoiceLanguage `json:"language"`
	Template  string        `json:"template" binding:"required"`
	Category  string        `json:"category,omitempty"`
	Variables []string      `json:"variables,omitempty"`
	CreatedAt time.Time     `json:"created_at"`
	UpdatedAt time.Time     `json:"updated_at"`
}

// ReplyTemplateRequest 回复模板请求.
type ReplyTemplateRequest struct {
	Name     string        `json:"name" binding:"required"`
	Language VoiceLanguage `json:"language,omitempty"`
	Template string        `json:"template" binding:"required"`
	Category string        `json:"category,omitempty"`
}

// CommandHistory 命令历史记录.
type CommandHistory struct {
	ID        string         `json:"id"`
	Command   VoiceCommand   `json:"command"`
	Response  *VoiceResponse `json:"response,omitempty"`
	Success   bool           `json:"success"`
	Error     string         `json:"error,omitempty"`
	CreatedAt time.Time      `json:"created_at"`
}

// VoiceHubConfig 语音助手配置.
type VoiceHubConfig struct {
	Enabled            bool             `json:"enabled"`
	DefaultPlatform    VoicePlatform    `json:"default_platform"`
	DefaultLanguage    VoiceLanguage    `json:"default_language"`
	WakeWords          []WakeWordConfig `json:"wake_words,omitempty"`
	TTSEnabled         bool             `json:"tts_enabled"`
	TTSVoice           string           `json:"tts_voice,omitempty"`
	TTSSpeed           float64          `json:"tts_speed"`
	TTSPitch           float64          `json:"tts_pitch"`
	MaxHistory         int              `json:"max_history"`
	CacheEnabled       bool             `json:"cache_enabled"`
	CacheTTLMinutes    int              `json:"cache_ttl_minutes"`
	SupportedPlatforms []VoicePlatform  `json:"supported_platforms,omitempty"`
}

// DefaultVoiceHubConfig 默认配置.
func DefaultVoiceHubConfig() *VoiceHubConfig {
	return &VoiceHubConfig{
		Enabled:         true,
		DefaultPlatform: PlatformLocal,
		DefaultLanguage: LangChinese,
		TTSEnabled:      true,
		TTSVoice:        "default",
		TTSSpeed:        1.0,
		TTSPitch:        1.0,
		MaxHistory:      1000,
		CacheEnabled:    true,
		CacheTTLMinutes: 30,
		SupportedPlatforms: []VoicePlatform{
			PlatformAlexa,
			PlatformGoogle,
			PlatformSiri,
			PlatformXiaoAi,
			PlatformLocal,
		},
		WakeWords: []WakeWordConfig{
			{
				ID:          "ww-001",
				WakeWord:    "小助手",
				Platform:    PlatformLocal,
				Language:    LangChinese,
				Sensitivity: 0.8,
				IsActive:    true,
			},
		},
	}
}

// PlatformConfig 平台配置.
type PlatformConfig struct {
	Platform VoicePlatform     `json:"platform"`
	Enabled  bool              `json:"enabled"`
	Endpoint string            `json:"endpoint,omitempty"`
	APIKey   string            `json:"api_key,omitempty"`
	DeviceID string            `json:"device_id,omitempty"`
	Extra    map[string]string `json:"extra,omitempty"`
}

// SupportedLanguages 获取支持的语言列表.
func SupportedLanguages() []VoiceLanguage {
	return []VoiceLanguage{LangChinese, LangEnglish, LangJapanese}
}

// SupportedPlatforms 获取支持的平台列表.
func SupportedPlatforms() []VoicePlatform {
	return []VoicePlatform{PlatformAlexa, PlatformGoogle, PlatformSiri, PlatformXiaoAi, PlatformLocal}
}

// IsValidLanguage 检查语言是否有效.
func IsValidLanguage(lang VoiceLanguage) bool {
	for _, l := range SupportedLanguages() {
		if l == lang {
			return true
		}
	}
	return false
}

// IsValidPlatform 检查平台是否有效.
func IsValidPlatform(p VoicePlatform) bool {
	for _, platform := range SupportedPlatforms() {
		if platform == p {
			return true
		}
	}
	return false
}
