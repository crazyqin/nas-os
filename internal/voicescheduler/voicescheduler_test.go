package voicescheduler

import (
	"context"
	"testing"
)

func TestParseCommand(t *testing.T) {
	scheduler := NewScheduler(nil)
	
	tests := []struct {
		input    string
		wantType CommandType
	}{
		{"查看存储空间", CmdStorageQuery},
		{"硬盘还有多少空间", CmdStorageQuery},
		{"备份状态怎么样", CmdBackupStatus},
		{"重启服务", CmdServiceCtrl},
		{"系统运行状态", CmdSystemInfo},
		{"有什么告警吗", CmdAlertQuery},
		{"播放音乐", CmdMediaPlay},
		{"随便说点什么", CmdUnknown},
	}
	
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			cmdType, _ := scheduler.parseCommand(tt.input)
			if cmdType != tt.wantType {
				t.Errorf("parseCommand(%q) = %s, want %s", tt.input, cmdType, tt.wantType)
			}
		})
	}
}

func TestProcessCommand(t *testing.T) {
	scheduler := NewScheduler(nil)
	
	resp := scheduler.ProcessCommand("查看存储空间", "user1")
	if !resp.Success {
		t.Error("expected success response")
	}
	if resp.Speak == "" {
		t.Error("expected speak text")
	}
}

func TestProcessCommandDisabled(t *testing.T) {
	config := DefaultSchedulerConfig()
	config.Enabled = false
	scheduler := NewScheduler(config)
	
	resp := scheduler.ProcessCommand("查看存储空间", "user1")
	if resp.Success {
		t.Error("expected failure when disabled")
	}
}

func TestCommandHistory(t *testing.T) {
	scheduler := NewScheduler(nil)
	
	scheduler.ProcessCommand("查看存储空间", "user1")
	scheduler.ProcessCommand("系统状态", "user1")
	scheduler.ProcessCommand("有什么告警", "user1")
	
	history := scheduler.GetHistory(10)
	if len(history) != 3 {
		t.Errorf("expected 3 history items, got %d", len(history))
	}
}

func TestCommandTypeStats(t *testing.T) {
	scheduler := NewScheduler(nil)
	
	scheduler.ProcessCommand("查看存储空间", "user1")
	scheduler.ProcessCommand("硬盘容量", "user1")
	scheduler.ProcessCommand("系统状态", "user1")
	
	stats := scheduler.GetCommandTypeStats()
	if stats[CmdStorageQuery] != 2 {
		t.Errorf("expected 2 storage queries, got %d", stats[CmdStorageQuery])
	}
	if stats[CmdSystemInfo] != 1 {
		t.Errorf("expected 1 system info, got %d", stats[CmdSystemInfo])
	}
}

func TestServiceControlParse(t *testing.T) {
	scheduler := NewScheduler(nil)
	
	_, params := scheduler.parseCommand("启动 SMB 服务")
	if params["action"] != "start" {
		t.Errorf("expected action=start, got %s", params["action"])
	}
	
	_, params = scheduler.parseCommand("停止 Docker")
	if params["action"] != "stop" {
		t.Errorf("expected action=stop, got %s", params["action"])
	}
}

func TestMaxHistoryLimit(t *testing.T) {
	config := DefaultSchedulerConfig()
	config.MaxHistory = 3
	scheduler := NewScheduler(config)
	
	for i := 0; i < 5; i++ {
		scheduler.ProcessCommand("系统状态", "user1")
	}
	
	history := scheduler.GetHistory(10)
	if len(history) != 3 {
		t.Errorf("expected 3 history items (max), got %d", len(history))
	}
}

func TestCustomHandler(t *testing.T) {
	scheduler := NewScheduler(nil)
	
	customCalled := false
	scheduler.RegisterHandler(CmdStorageQuery, func(ctx context.Context, cmd *VoiceCommand) *VoiceResponse {
		customCalled = true
		return &VoiceResponse{
			Success: true,
			Message: "自定义响应",
			Speak:   "这是自定义处理器",
		}
	})
	
	scheduler.ProcessCommand("存储空间", "user1")
	if !customCalled {
		t.Error("expected custom handler to be called")
	}
}
