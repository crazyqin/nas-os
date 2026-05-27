package migration

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ========== Planner 测试 ==========

func TestPlanner_DetectSource(t *testing.T) {
	planner := NewPlanner()

	t.Run("成功探测", func(t *testing.T) {
		req := &CreateMigrationRequest{
			SourceHost: "192.168.1.100",
			SourcePort: 22,
			SourceUser: "admin",
		}

		info, err := planner.DetectSource(context.Background(), req)
		require.NoError(t, err)
		assert.NotNil(t, info)
		assert.Equal(t, SourceGenericNAS, info.Type)
		assert.Equal(t, "192.168.1.100", info.Hostname)
	})

	t.Run("空主机地址", func(t *testing.T) {
		req := &CreateMigrationRequest{
			SourceHost: "",
			SourceUser: "admin",
		}

		_, err := planner.DetectSource(context.Background(), req)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "源主机地址不能为空")
	})
}

func TestPlanner_GeneratePlan(t *testing.T) {
	planner := NewPlanner()

	task := &MigrationTask{
		ID:         "test-task-1",
		SourceType: SourceSynology,
		SourceHost: "192.168.1.100",
		TargetPath: "/data/migration",
	}

	sourceInfo := &SourceSystemInfo{
		Type:         SourceSynology,
		Version:      "7.0",
		Hostname:     "synology-nas",
		TotalStorage: 1024 * 1024 * 1024 * 500, // 500GB
		UsedStorage:  1024 * 1024 * 1024 * 200,  // 200GB
		TotalUsers:   10,
		TotalShares:  20,
		TotalApps:    5,
	}

	t.Run("生成计划", func(t *testing.T) {
		plan, err := planner.GeneratePlan(context.Background(), task, sourceInfo)
		require.NoError(t, err)
		assert.NotNil(t, plan)
		assert.NotEmpty(t, plan.ID)
		assert.Equal(t, task.ID, plan.TaskID)
		assert.Equal(t, SourceSynology, plan.SourceType)
		assert.NotEmpty(t, plan.Mappings)
		assert.True(t, plan.Compatible)
	})

	t.Run("映射包含用户数据", func(t *testing.T) {
		plan, err := planner.GeneratePlan(context.Background(), task, sourceInfo)
		require.NoError(t, err)

		var hasUsers bool
		for _, m := range plan.Mappings {
			if m.Category == CategoryUsers {
				hasUsers = true
				assert.Equal(t, 10, m.ItemCount)
				assert.True(t, m.Selected)
			}
		}
		assert.True(t, hasUsers, "应包含用户数据映射")
	})
}

func TestPlanner_UpdateMappingSelection(t *testing.T) {
	planner := NewPlanner()

	plan := &MigrationPlan{
		Mappings: []DataMapping{
			{Category: CategoryUsers, Selected: true},
			{Category: CategoryShares, Selected: true},
		},
	}

	t.Run("取消选择", func(t *testing.T) {
		ok := planner.UpdateMappingSelection(plan, CategoryUsers, false)
		assert.True(t, ok)
		assert.False(t, plan.Mappings[0].Selected)
	})

	t.Run("重新选择", func(t *testing.T) {
		ok := planner.UpdateMappingSelection(plan, CategoryUsers, true)
		assert.True(t, ok)
		assert.True(t, plan.Mappings[0].Selected)
	})

	t.Run("不存在的类别", func(t *testing.T) {
		ok := planner.UpdateMappingSelection(plan, CategoryVMs, true)
		assert.False(t, ok)
	})
}

// ========== Executor 测试 ==========

type mockTransferFunc struct {
	called    bool
	callCount int
}

func (m *mockTransferFunc) Transfer(ctx context.Context, mapping DataMapping, progress func(int64)) error {
	m.called = true
	m.callCount++
	progress(mapping.TotalSize / int64(mapping.ItemCount))
	return nil
}

func TestExecutor_Execute(t *testing.T) {
	planner := NewPlanner()
	executor := NewExecutor(planner)

	// 使用 mock 传输函数
	mock := &mockTransferFunc{}
	executor.SetTransferFunc(mock.Transfer)

	task := &MigrationTask{
		ID:         "test-exec-1",
		SourceType: SourceSynology,
		SourceHost: "192.168.1.100",
		TargetPath: "/data/migration",
		Progress:   &ProgressInfo{CategoryProgress: make(map[string]int)},
	}

	plan := &MigrationPlan{
		ID:         "plan-1",
		TaskID:     task.ID,
		SourceType: SourceSynology,
		Mappings: []DataMapping{
			{
				ID:         "mapping-1",
				Category:   CategoryUsers,
				SourcePath: "/var/users",
				TargetPath: "/home",
				ItemCount:  2,
				TotalSize:  1024 * 1024 * 100,
				Selected:   true,
				Order:      1,
			},
		},
	}

	t.Run("执行迁移", func(t *testing.T) {
		result, err := executor.Execute(context.Background(), task, plan)
		require.NoError(t, err)
		assert.NotNil(t, result)

		// 等待完成
		time.Sleep(500 * time.Millisecond)

		assert.True(t, mock.called)
		assert.Equal(t, MigrationStatusCompleted, task.Status)
	})

	t.Run("重复执行失败", func(t *testing.T) {
		// 任务已在运行
		_, err := executor.Execute(context.Background(), task, plan)
		assert.Error(t, err)
	})
}

func TestExecutor_Pause(t *testing.T) {
	planner := NewPlanner()
	executor := NewExecutor(planner)

	t.Run("暂停不存在的任务", func(t *testing.T) {
		err := executor.Pause("non-existent")
		assert.Error(t, err)
	})
}

func TestExecutor_GetProgress(t *testing.T) {
	planner := NewPlanner()
	executor := NewExecutor(planner)

	t.Run("获取不存在任务的进度", func(t *testing.T) {
		_, err := executor.GetProgress("non-existent")
		assert.Error(t, err)
	})
}

// ========== Manager 测试 ==========

func TestManager_CreateTask(t *testing.T) {
	manager := NewManager()

	t.Run("创建任务成功", func(t *testing.T) {
		req := &CreateRequest{
			Name:         "测试迁移",
			SourceDevice: "192.168.1.100",
			TargetDevice: "192.168.1.200",
			SourcePath:   "/data",
			TargetPath:   "/backup",
		}

		task, err := manager.CreateTask(req)
		require.NoError(t, err)
		assert.NotEmpty(t, task.ID)
		assert.Equal(t, "测试迁移", task.Name)
		assert.Equal(t, StatusPending, task.Status)
		assert.Equal(t, ModeFull, task.Mode)
	})

	t.Run("缺少必要字段", func(t *testing.T) {
		req := &CreateRequest{
			Name: "测试",
		}

		_, err := manager.CreateTask(req)
		assert.Error(t, err)
	})

	t.Run("默认模式", func(t *testing.T) {
		req := &CreateRequest{
			Name:         "默认模式测试",
			SourceDevice: "src",
			TargetDevice: "dst",
			SourcePath:   "/src",
			TargetPath:   "/dst",
		}

		task, err := manager.CreateTask(req)
		require.NoError(t, err)
		assert.Equal(t, ModeFull, task.Mode)
	})
}

func TestManager_GetTask(t *testing.T) {
	manager := NewManager()

	t.Run("获取不存在的任务", func(t *testing.T) {
		_, err := manager.GetTask("non-existent")
		assert.Error(t, err)
	})

	t.Run("获取存在的任务", func(t *testing.T) {
		req := &CreateRequest{
			Name:         "测试",
			SourceDevice: "src",
			TargetDevice: "dst",
			SourcePath:   "/src",
			TargetPath:   "/dst",
		}

		created, _ := manager.CreateTask(req)
		task, err := manager.GetTask(created.ID)
		require.NoError(t, err)
		assert.Equal(t, created.ID, task.ID)
	})
}

func TestManager_ListTasks(t *testing.T) {
	manager := NewManager()

	t.Run("空列表", func(t *testing.T) {
		tasks := manager.ListTasks()
		assert.Empty(t, tasks)
	})

	t.Run("添加后列出", func(t *testing.T) {
		for i := 0; i < 3; i++ {
			manager.CreateTask(&CreateRequest{
				Name:         "任务",
				SourceDevice: "src",
				TargetDevice: "dst",
				SourcePath:   "/src",
				TargetPath:   "/dst",
			})
		}

		tasks := manager.ListTasks()
		assert.Len(t, tasks, 3)
	})
}

func TestManager_DeleteTask(t *testing.T) {
	manager := NewManager()

	t.Run("删除不存在的任务", func(t *testing.T) {
		err := manager.DeleteTask("non-existent")
		assert.Error(t, err)
	})

	t.Run("删除存在的任务", func(t *testing.T) {
		req := &CreateRequest{
			Name:         "待删除",
			SourceDevice: "src",
			TargetDevice: "dst",
			SourcePath:   "/src",
			TargetPath:   "/dst",
		}

		task, _ := manager.CreateTask(req)
		err := manager.DeleteTask(task.ID)
		assert.NoError(t, err)

		_, err = manager.GetTask(task.ID)
		assert.Error(t, err)
	})
}

func TestManager_Scan(t *testing.T) {
	manager := NewManager()

	t.Run("扫描不存在的任务", func(t *testing.T) {
		_, err := manager.Scan(context.Background(), "non-existent")
		assert.Error(t, err)
	})

	t.Run("扫描存在的任务", func(t *testing.T) {
		req := &CreateRequest{
			Name:         "扫描测试",
			SourceDevice: "src",
			TargetDevice: "dst",
			SourcePath:   "/src",
			TargetPath:   "/dst",
		}

		task, _ := manager.CreateTask(req)
		result, err := manager.Scan(context.Background(), task.ID)
		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.Greater(t, result.TotalSize, int64(0))
		assert.Greater(t, result.TotalFiles, int64(0))
	})
}

func TestManager_Start(t *testing.T) {
	manager := NewManager()

	t.Run("启动不存在的任务", func(t *testing.T) {
		err := manager.Start("non-existent")
		assert.Error(t, err)
	})

	t.Run("启动任务", func(t *testing.T) {
		req := &CreateRequest{
			Name:         "启动测试",
			SourceDevice: "src",
			TargetDevice: "dst",
			SourcePath:   "/src",
			TargetPath:   "/dst",
		}

		task, _ := manager.CreateTask(req)
		err := manager.Start(task.ID)
		assert.NoError(t, err)

		// 等待完成
		time.Sleep(1 * time.Second)

		updated, _ := manager.GetTask(task.ID)
		assert.Equal(t, StatusCompleted, updated.Status)
	})

	t.Run("重复启动", func(t *testing.T) {
		req := &CreateRequest{
			Name:         "重复启动测试",
			SourceDevice: "src",
			TargetDevice: "dst",
			SourcePath:   "/src",
			TargetPath:   "/dst",
		}

		task, _ := manager.CreateTask(req)
		_ = manager.Start(task.ID)

		err := manager.Start(task.ID)
		assert.Error(t, err)
	})
}

func TestManager_Cancel(t *testing.T) {
	manager := NewManager()

	t.Run("取消未运行的任务", func(t *testing.T) {
		err := manager.Cancel("non-existent")
		assert.Error(t, err)
	})
}

func TestManager_Rollback(t *testing.T) {
	manager := NewManager()

	t.Run("回滚不存在的任务", func(t *testing.T) {
		err := manager.Rollback("non-existent")
		assert.Error(t, err)
	})

	t.Run("回滚无快照的任务", func(t *testing.T) {
		req := &CreateRequest{
			Name:         "回滚测试",
			SourceDevice: "src",
			TargetDevice: "dst",
			SourcePath:   "/src",
			TargetPath:   "/dst",
		}

		task, _ := manager.CreateTask(req)
		err := manager.Rollback(task.ID)
		assert.Error(t, err)
	})

	t.Run("回滚已完成的任务", func(t *testing.T) {
		req := &CreateRequest{
			Name:         "回滚完成任务",
			SourceDevice: "src",
			TargetDevice: "dst",
			SourcePath:   "/src",
			TargetPath:   "/dst",
		}

		task, _ := manager.CreateTask(req)
		_ = manager.Start(task.ID)
		time.Sleep(1 * time.Second)

		err := manager.Rollback(task.ID)
		assert.NoError(t, err)

		updated, _ := manager.GetTask(task.ID)
		assert.Equal(t, StatusRolledBack, updated.Status)
	})
}

func TestManager_Verify(t *testing.T) {
	manager := NewManager()

	t.Run("验证不存在的任务", func(t *testing.T) {
		_, err := manager.Verify(context.Background(), "non-existent")
		assert.Error(t, err)
	})

	t.Run("验证未完成的任务", func(t *testing.T) {
		req := &CreateRequest{
			Name:         "验证测试",
			SourceDevice: "src",
			TargetDevice: "dst",
			SourcePath:   "/src",
			TargetPath:   "/dst",
		}

		task, _ := manager.CreateTask(req)
		_, err := manager.Verify(context.Background(), task.ID)
		assert.Error(t, err)
	})
}

// ========== ComputeChecksum 测试 ==========

func TestComputeChecksum(t *testing.T) {
	t.Run("计算校验和", func(t *testing.T) {
		data := "Hello, World!"
		reader := strings.NewReader(data)

		checksum, err := ComputeChecksum(reader)
		require.NoError(t, err)
		assert.NotEmpty(t, checksum)
		assert.Len(t, checksum, 64) // SHA256 hex string length
	})

	t.Run("相同内容相同校验和", func(t *testing.T) {
		data := "test data"
		r1 := strings.NewReader(data)
		r2 := strings.NewReader(data)

		h1, _ := ComputeChecksum(r1)
		h2, _ := ComputeChecksum(r2)
		assert.Equal(t, h1, h2)
	})
}

// ========== 类型测试 ==========

func TestMigrationStatus_Constants(t *testing.T) {
	assert.Equal(t, MigrationStatus("pending"), MigrationStatusPending)
	assert.Equal(t, MigrationStatus("running"), MigrationStatusRunning)
	assert.Equal(t, MigrationStatus("completed"), MigrationStatusCompleted)
	assert.Equal(t, MigrationStatus("failed"), MigrationStatusFailed)
	assert.Equal(t, MigrationStatus("cancelled"), MigrationStatusCancelled)
}

func TestDataCategory_Constants(t *testing.T) {
	assert.Equal(t, DataCategory("system"), CategorySystem)
	assert.Equal(t, DataCategory("users"), CategoryUsers)
	assert.Equal(t, DataCategory("shares"), CategoryShares)
	assert.Equal(t, DataCategory("apps"), CategoryApps)
	assert.Equal(t, DataCategory("docker"), CategoryDocker)
}

func TestMigrationSourceType_Constants(t *testing.T) {
	assert.Equal(t, MigrationSourceType("synology"), SourceSynology)
	assert.Equal(t, MigrationSourceType("qnap"), SourceQNAP)
	assert.Equal(t, MigrationSourceType("truenas"), SourceTrueNAS)
	assert.Equal(t, MigrationSourceType("unraid"), SourceUnraid)
}

// ========== Handler 测试 ==========

func TestNewHandler(t *testing.T) {
	planner := NewPlanner()
	executor := NewExecutor(planner)

	handler := NewHandler(planner, executor)
	assert.NotNil(t, handler)
	assert.NotNil(t, handler.planner)
	assert.NotNil(t, handler.executor)
}

func TestNewHandlerWithManager(t *testing.T) {
	manager := NewManager()

	handler := NewHandlerWithManager(manager)
	assert.NotNil(t, handler)
	assert.NotNil(t, handler.manager)
}

// ========== ProgressInfo 测试 ==========

func TestProgressInfo_Fields(t *testing.T) {
	progress := &ProgressInfo{
		OverallPercent:   50,
		CurrentCategory:  CategoryUsers,
		CategoryPercent:  75,
		TransferredBytes: 1024 * 1024 * 100,
		TotalBytes:       1024 * 1024 * 200,
		Speed:            1024 * 1024 * 10, // 10MB/s
		RemainingSec:     10,
		Phase:            "正在迁移用户数据",
		CategoryProgress: map[string]int{
			"users":  75,
			"shares": 100,
		},
	}

	assert.Equal(t, 50, progress.OverallPercent)
	assert.Equal(t, CategoryUsers, progress.CurrentCategory)
	assert.Equal(t, 75, progress.CategoryPercent)
	assert.Equal(t, int64(10), progress.RemainingSec)
}

// ========== MigrationResult 测试 ==========

func TestMigrationResult_Fields(t *testing.T) {
	result := &MigrationResult{
		TaskID:          "task-1",
		Status:          MigrationStatusCompleted,
		TotalMigrated:   100,
		TotalFailed:     5,
		TotalSkipped:    2,
		BytesMigrated:   1024 * 1024 * 500,
		Duration:        time.Minute * 10,
		CategoryResults: make([]CategoryResult, 0),
		Errors:          make([]MigrationError, 0),
		Warnings:        make([]string, 0),
		RollbackID:      "rollback-1",
		CompletedAt:     time.Now(),
	}

	assert.Equal(t, "task-1", result.TaskID)
	assert.Equal(t, MigrationStatusCompleted, result.Status)
	assert.Equal(t, 100, result.TotalMigrated)
	assert.Equal(t, 5, result.TotalFailed)
	assert.NotEmpty(t, result.RollbackID)
}

// ========== DataMapping 测试 ==========

func TestDataMapping_Fields(t *testing.T) {
	mapping := DataMapping{
		ID:          "mapping-1",
		Category:    CategoryShares,
		SourcePath:  "/volume1/data",
		TargetPath:  "/data/shares",
		ItemCount:   100,
		TotalSize:   1024 * 1024 * 1024,
		Selected:    true,
		Convertible: false,
		Order:       1,
	}

	assert.Equal(t, "mapping-1", mapping.ID)
	assert.Equal(t, CategoryShares, mapping.Category)
	assert.True(t, mapping.Selected)
	assert.False(t, mapping.Convertible)
}

// ========== Checkpoint 测试 ==========

func TestCheckpoint_Fields(t *testing.T) {
	now := time.Now()
	cp := &Checkpoint{
		TaskID:           "task-1",
		CategoryIndex:    2,
		ItemIndex:        50,
		BytesTransferred: 1024 * 1024 * 100,
		Timestamp:        now,
	}

	assert.Equal(t, "task-1", cp.TaskID)
	assert.Equal(t, 2, cp.CategoryIndex)
	assert.Equal(t, 50, cp.ItemIndex)
	assert.Equal(t, now, cp.Timestamp)
}

// ========== SourceSystemInfo 测试 ==========

func TestSourceSystemInfo_Fields(t *testing.T) {
	info := &SourceSystemInfo{
		Type:         SourceSynology,
		Version:      "7.0",
		Hostname:     "my-nas",
		TotalStorage: 1024 * 1024 * 1024 * 1000,
		UsedStorage:  1024 * 1024 * 1024 * 500,
		TotalUsers:   20,
		TotalShares:  30,
		TotalApps:    15,
		IPAddresses:  []string{"192.168.1.100"},
		SystemModel:  "DS920+",
	}

	assert.Equal(t, SourceSynology, info.Type)
	assert.Equal(t, "7.0", info.Version)
	assert.Equal(t, "my-nas", info.Hostname)
	assert.Equal(t, 20, info.TotalUsers)
	assert.Equal(t, "DS920+", info.SystemModel)
}

// ========== PlanWarning 测试 ==========

func TestPlanWarning_Fields(t *testing.T) {
	warning := PlanWarning{
		Level:    "warning",
		Category: "apps",
		Message:  "部分应用配置可能需要手动调整",
	}

	assert.Equal(t, "warning", warning.Level)
	assert.Equal(t, "apps", warning.Category)
	assert.NotEmpty(t, warning.Message)
}

// ========== CategoryResult 测试 ==========

func TestCategoryResult_Fields(t *testing.T) {
	result := CategoryResult{
		Category:    CategoryUsers,
		Status:      "success",
		Migrated:    10,
		Failed:      0,
		Skipped:     0,
		SizeBytes:   1024 * 1024 * 50,
		Duration:    time.Second * 30,
		ErrorDetail: "",
	}

	assert.Equal(t, CategoryUsers, result.Category)
	assert.Equal(t, "success", result.Status)
	assert.Equal(t, 10, result.Migrated)
	assert.Equal(t, 0, result.Failed)
}

// ========== MigrationError 测试 ==========

func TestMigrationError_Fields(t *testing.T) {
	migErr := MigrationError{
		Category: CategoryDocker,
		Item:     "container-1",
		Error:    "连接超时",
		Code:     "TIMEOUT",
	}

	assert.Equal(t, CategoryDocker, migErr.Category)
	assert.Equal(t, "container-1", migErr.Item)
	assert.Equal(t, "连接超时", migErr.Error)
	assert.Equal(t, "TIMEOUT", migErr.Code)
}
