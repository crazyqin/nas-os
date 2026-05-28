// Package voicecontrol 单元测试
package voicecontrol

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestVoiceProvider_Constants(t *testing.T) {
	assert.Equal(t, VoiceProvider("siri"), ProviderSiri)
	assert.Equal(t, VoiceProvider("google"), ProviderGoogle)
	assert.Equal(t, VoiceProvider("alexa"), ProviderAlexa)
	assert.Equal(t, VoiceProvider("xiaoai"), ProviderXiaoAI)
}

func TestCommandType_Constants(t *testing.T) {
	assert.Equal(t, CommandType("file_search"), CmdFileSearch)
	assert.Equal(t, CommandType("storage_status"), CmdStorageStatus)
	assert.Equal(t, CommandType("unknown"), CmdUnknown)
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	assert.True(t, cfg.Enabled)
	assert.Equal(t, VoiceProvider("custom"), cfg.DefaultProvider)
	assert.Equal(t, "zh-CN", cfg.DefaultLanguage)
	assert.Equal(t, 30, cfg.MaxAudioDurationSec)
}

func TestVoiceControl_ProcessTextCommand(t *testing.T) {
	cfg := DefaultConfig()
	vc := New(cfg)

	resp, err := vc.ProcessTextCommand("user1", "查看存储空间")
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.True(t, resp.ShouldSpeak)
	assert.Contains(t, resp.Text, "存储")
}

func TestVoiceControl_ProcessSystemStatus(t *testing.T) {
	vc := New(DefaultConfig())
	resp, err := vc.ProcessTextCommand("user1", "系统状态怎么样")
	assert.NoError(t, err)
	assert.Contains(t, resp.Text, "正常")
}

func TestVoiceControl_ProcessFileSearch(t *testing.T) {
	vc := New(DefaultConfig())
	resp, err := vc.ProcessTextCommand("user1", "搜索照片")
	assert.NoError(t, err)
	assert.Contains(t, resp.Text, "找到")
}

func TestVoiceControl_ProcessUnknown(t *testing.T) {
	vc := New(DefaultConfig())
	resp, err := vc.ProcessTextCommand("user1", "asdfghjkl")
	assert.NoError(t, err)
	assert.Contains(t, resp.Text, "未能识别")
}

func TestVoiceControl_RegisterAndGetProfile(t *testing.T) {
	vc := New(DefaultConfig())
	profile := &VoiceProfile{
		UserID:   "user1",
		Language: "zh-CN",
		Provider: ProviderCustom,
	}
	err := vc.RegisterProfile(profile)
	assert.NoError(t, err)

	got, ok := vc.GetProfile("user1")
	assert.True(t, ok)
	assert.Equal(t, "zh-CN", got.Language)
}

func TestVoiceControl_PermissionDenied(t *testing.T) {
	vc := New(DefaultConfig())
	vc.SetPermission(&Permission{
		UserID:  "user1",
		Command: CmdPermission,
		Allowed: false,
	})

	resp, err := vc.ProcessTextCommand("user1", "删除所有文件")
	assert.NoError(t, err)
	assert.Contains(t, resp.Text, "权限")
}

func TestVoiceControl_CommandHistory(t *testing.T) {
	vc := New(DefaultConfig())
	vc.ProcessTextCommand("user1", "查看存储空间")
	vc.ProcessTextCommand("user1", "系统状态")

	history := vc.GetCommandHistory("user1", 10)
	assert.Len(t, history, 2)
}

func TestVoiceControl_GetStats(t *testing.T) {
	vc := New(DefaultConfig())
	vc.ProcessTextCommand("user1", "查看存储空间")
	vc.ProcessTextCommand("user2", "系统状态")

	stats := vc.GetStats()
	assert.Equal(t, 2, stats["total_commands"])
}

func TestIntentEngine_Parse(t *testing.T) {
	engine := NewIntentEngine()

	intent := engine.Parse("搜索文档")
	assert.Equal(t, CmdFileSearch, intent.Command)
	assert.Equal(t, "文档", intent.Parameters["query"])

	intent = engine.Parse("查看存储空间")
	assert.Equal(t, CmdStorageStatus, intent.Command)

	intent = engine.Parse("播放音乐")
	assert.Equal(t, CmdMediaPlay, intent.Command)
}

func TestIntentConfidence(t *testing.T) {
	engine := NewIntentEngine()
	intent := engine.Parse("帮我搜索所有的照片文件")
	assert.True(t, intent.Confidence > 0)
	assert.Equal(t, CmdFileSearch, intent.Command)
}
