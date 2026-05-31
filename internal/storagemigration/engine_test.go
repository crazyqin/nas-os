package storagemigration

import (
	"testing"
	"time"
)

func TestNew(t *testing.T) {
	e := New()
	if e == nil {
		t.Fatal("New() 返回 nil")
	}
}

func TestValidateConfig(t *testing.T) {
	e := New()

	tests := []struct {
		name    string
		cfg     MigrationConfig
		wantErr bool
	}{
		{
			name: "有效配置",
			cfg: MigrationConfig{
				Source:     SourceSynology,
				SourceHost: "192.168.1.100",
				SourcePath: "/volume1/data",
				DestPath:   "/data/synology",
			},
			wantErr: false,
		},
		{
			name: "无效源系统",
			cfg: MigrationConfig{
				Source:     "invalid",
				SourceHost: "192.168.1.100",
				SourcePath: "/data",
				DestPath:   "/dest",
			},
			wantErr: true,
		},
		{
			name: "缺少源主机",
			cfg: MigrationConfig{
				Source:     SourceTrueNAS,
				SourcePath: "/data",
				DestPath:   "/dest",
			},
			wantErr: true,
		},
		{
			name: "缺少源路径",
			cfg: MigrationConfig{
				Source:     SourceTrueNAS,
				SourceHost: "192.168.1.100",
				DestPath:   "/dest",
			},
			wantErr: true,
		},
		{
			name: "缺少目标路径",
			cfg: MigrationConfig{
				Source:     SourceTrueNAS,
				SourceHost: "192.168.1.100",
				SourcePath: "/data",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := e.ValidateConfig(tt.cfg)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateConfig() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestStart(t *testing.T) {
	e := New()
	cfg := MigrationConfig{
		Source:     SourceSynology,
		SourceHost: "192.168.1.100",
		SourcePath: "/volume1/data",
		DestPath:   "/data/synology",
		Options:    DefaultOptions(),
	}

	task, err := e.Start(cfg)
	if err != nil {
		t.Fatalf("Start() 失败: %v", err)
	}
	if task.ID == "" {
		t.Error("任务 ID 为空")
	}
	if task.Status != StatusPending && task.Status != StatusScanning {
		t.Errorf("初始状态 = %s, 期望 pending 或 scanning", task.Status)
	}
}

func TestGetTask(t *testing.T) {
	e := New()
	cfg := MigrationConfig{
		Source:     SourceSynology,
		SourceHost: "192.168.1.100",
		SourcePath: "/volume1/data",
		DestPath:   "/data/synology",
	}

	task, _ := e.Start(cfg)

	// 等待迁移完成
	time.Sleep(500 * time.Millisecond)

	got, err := e.GetTask(task.ID)
	if err != nil {
		t.Fatalf("GetTask() 失败: %v", err)
	}
	if got.ID != task.ID {
		t.Errorf("任务 ID = %s, 期望 %s", got.ID, task.ID)
	}
}

func TestGetTask_NotFound(t *testing.T) {
	e := New()
	_, err := e.GetTask("nonexistent")
	if err == nil {
		t.Error("期望返回错误")
	}
}

func TestListTasks(t *testing.T) {
	e := New()
	cfg := MigrationConfig{
		Source:     SourceSynology,
		SourceHost: "192.168.1.100",
		SourcePath: "/volume1/data",
		DestPath:   "/data/synology",
	}

	e.Start(cfg)
	e.Start(cfg)

	tasks := e.ListTasks()
	if len(tasks) != 2 {
		t.Errorf("任务数 = %d, 期望 2", len(tasks))
	}
}

func TestCancel(t *testing.T) {
	e := New()
	cfg := MigrationConfig{
		Source:     SourceSynology,
		SourceHost: "192.168.1.100",
		SourcePath: "/volume1/data",
		DestPath:   "/data/synology",
	}

	task, _ := e.Start(cfg)

	// 等待迁移完成后再取消会失败
	time.Sleep(600 * time.Millisecond)

	err := e.Cancel(task.ID)
	// 如果已完成，取消应该失败
	if err == nil {
		// 如果成功取消了，检查状态
		got, _ := e.GetTask(task.ID)
		if got.Status != StatusCancelled {
			t.Errorf("状态 = %s, 期望 cancelled", got.Status)
		}
	}
}

func TestDefaultOptions(t *testing.T) {
	opts := DefaultOptions()
	if !opts.PreserveACL {
		t.Error("默认应保留 ACL")
	}
	if !opts.PreserveTimestamps {
		t.Error("默认应保留时间戳")
	}
	if !opts.VerifyChecksum {
		t.Error("默认应校验 checksum")
	}
	if opts.SyncMode != "copy" {
		t.Errorf("同步模式 = %s, 期望 copy", opts.SyncMode)
	}
}

func TestAllSources(t *testing.T) {
	sources := AllSources()
	if len(sources) < 3 {
		t.Errorf("支持的源系统数 = %d, 期望 >= 3", len(sources))
	}
}
