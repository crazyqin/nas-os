// DRAID 模块单元测试
// 覆盖 DRAID 阵列管理、分布式热备、数据重分布、性能监控等核心功能

package draid

import (
	"testing"
)

func TestNewManager(t *testing.T) {
	m := NewManager()
	if m == nil {
		t.Fatal("NewManager 返回 nil")
	}
	if m.arrays == nil {
		t.Fatal("arrays map 未初始化")
	}
}

func TestCreateArray(t *testing.T) {
	m := NewManager()

	tests := []struct {
		name      string
		level     string
		devices   []string
		spares    []string
		groupSize int
		dataDisks int
		chunkSize string
		wantErr   bool
		errMsg    string
	}{
		{
			name:      "valid-draid1",
			level:     DRAID1,
			devices:   []string{"/dev/sda", "/dev/sdb", "/dev/sdc", "/dev/sdd", "/dev/sde", "/dev/sdf"},
			spares:    []string{"/dev/sdg"},
			groupSize: 6,
			dataDisks: 5,
			chunkSize: "128K",
			wantErr:   false,
		},
		{
			name:      "valid-draid2",
			level:     DRAID2,
			devices:   []string{"/dev/sda", "/dev/sdb", "/dev/sdc", "/dev/sdd", "/dev/sde", "/dev/sdf", "/dev/sdg", "/dev/sdh"},
			spares:    []string{},
			groupSize: 8,
			dataDisks: 6,
			chunkSize: "256K",
			wantErr:   false,
		},
		{
			name:      "empty-name",
			level:     DRAID1,
			devices:   []string{"/dev/sda", "/dev/sdb"},
			groupSize: 2,
			dataDisks: 1,
			wantErr:   true,
			errMsg:    "阵列名称不能为空",
		},
		{
			name:      "invalid-level",
			level:     "RAID5",
			devices:   []string{"/dev/sda", "/dev/sdb"},
			groupSize: 2,
			dataDisks: 1,
			wantErr:   true,
			errMsg:    "无效的 DRAID 级别",
		},
		{
			name:      "insufficient-devices",
			level:     DRAID1,
			devices:   []string{"/dev/sda"},
			groupSize: 2,
			dataDisks: 1,
			wantErr:   true,
			errMsg:    "设备数量",
		},
		{
			name:      "devices-not-multiple-of-group",
			level:     DRAID1,
			devices:   []string{"/dev/sda", "/dev/sdb", "/dev/sdc"},
			groupSize: 2,
			dataDisks: 1,
			wantErr:   true,
			errMsg:    "必须是组大小",
		},
		{
			name:      "group-size-too-small",
			level:     DRAID1,
			devices:   []string{"/dev/sda", "/dev/sdb"},
			groupSize: 1,
			dataDisks: 1,
			wantErr:   true,
			errMsg:    "组大小必须 >= 数据盘",
		},
		{
			name:      "duplicate-device",
			level:     DRAID1,
			devices:   []string{"/dev/sda", "/dev/sda"},
			groupSize: 2,
			dataDisks: 1,
			wantErr:   true,
			errMsg:    "设备重复",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			arrName := tt.name
			if tt.name == "empty-name" {
				arrName = ""
			}
			err := m.CreateArray(arrName, tt.level, tt.devices, tt.spares, tt.groupSize, tt.dataDisks, tt.chunkSize)
			if (err != nil) != tt.wantErr {
				t.Errorf("CreateArray() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr && err != nil && tt.errMsg != "" {
				if !contains(err.Error(), tt.errMsg) {
					t.Errorf("错误消息不匹配，期望包含 %q, 实际 %q", tt.errMsg, err.Error())
				}
			}
		})
	}

	// 验证创建成功
	arr, err := m.GetArray("valid-draid1")
	if err != nil {
		t.Fatalf("GetArray() error = %v", err)
	}
	if arr.Name != "valid-draid1" {
		t.Errorf("名称 = %v, 期望 valid-draid1", arr.Name)
	}
	if arr.Level != DRAID1 {
		t.Errorf("级别 = %v, 期望 %v", arr.Level, DRAID1)
	}
	if arr.Status != StatusActive {
		t.Errorf("状态 = %v, 期望 %v", arr.Status, StatusActive)
	}
	if arr.ParityDisks != 1 {
		t.Errorf("校验盘数 = %v, 期望 1", arr.ParityDisks)
	}
}

func TestDeleteArray(t *testing.T) {
	m := NewManager()

	// 创建测试阵列
	m.CreateArray("test-draid", DRAID1, []string{"/dev/sda", "/dev/sdb"}, nil, 2, 1, "128K")

	// 删除存在的阵列
	err := m.DeleteArray("test-draid")
	if err != nil {
		t.Errorf("DeleteArray() error = %v", err)
	}

	// 验证已删除
	_, err = m.GetArray("test-draid")
	if err == nil {
		t.Error("GetArray() 应返回错误，但没有")
	}

	// 删除不存在的阵列
	err = m.DeleteArray("nonexistent")
	if err == nil {
		t.Error("删除不存在的阵列应返回错误")
	}
}

func TestGetArray(t *testing.T) {
	m := NewManager()
	m.CreateArray("test-draid", DRAID1, []string{"/dev/sda", "/dev/sdb"}, nil, 2, 1, "128K")

	arr, err := m.GetArray("test-draid")
	if err != nil {
		t.Fatalf("GetArray() error = %v", err)
	}
	if arr == nil {
		t.Fatal("GetArray() 返回 nil")
	}
	if arr.Name != "test-draid" {
		t.Errorf("Name = %v, 期望 test-draid", arr.Name)
	}
}

func TestListArrays(t *testing.T) {
	m := NewManager()

	// 空列表
	arrays := m.ListArrays()
	if len(arrays) != 0 {
		t.Errorf("空管理器应返回 0 个阵列，实际 %d", len(arrays))
	}

	// 添加多个阵列
	m.CreateArray("draid1", DRAID1, []string{"/dev/sda", "/dev/sdb"}, nil, 2, 1, "128K")
	m.CreateArray("draid2", DRAID2, []string{"/dev/sdc", "/dev/sdd", "/dev/sde", "/dev/sdf"}, nil, 4, 2, "256K")

	arrays = m.ListArrays()
	if len(arrays) != 2 {
		t.Errorf("应返回 2 个阵列，实际 %d", len(arrays))
	}
}

func TestDistributedSpareManagement(t *testing.T) {
	m := NewManager()
	m.CreateArray("test-draid", DRAID1, []string{"/dev/sda", "/dev/sdb"}, nil, 2, 1, "128K")

	// 添加分布式热备
	err := m.AddDistributedSpare("test-draid", "/dev/sdc")
	if err != nil {
		t.Fatalf("AddDistributedSpare() error = %v", err)
	}

	// 重复添加
	err = m.AddDistributedSpare("test-draid", "/dev/sdc")
	if err == nil {
		t.Error("重复添加热备应返回错误")
	}

	// 添加与阵列设备重复的热备
	err = m.AddDistributedSpare("test-draid", "/dev/sda")
	if err == nil {
		t.Error("添加与阵列设备重复的热备应返回错误")
	}

	// 列出热备
	spares, err := m.ListDistributedSpares("test-draid")
	if err != nil {
		t.Fatalf("ListDistributedSpares() error = %v", err)
	}
	if len(spares) != 1 {
		t.Errorf("应有 1 个热备，实际 %d", len(spares))
	}

	// 移除热备
	err = m.RemoveDistributedSpare("test-draid", "/dev/sdc")
	if err != nil {
		t.Fatalf("RemoveDistributedSpare() error = %v", err)
	}

	// 移除不存在的热备
	err = m.RemoveDistributedSpare("test-draid", "/dev/sdc")
	if err == nil {
		t.Error("移除不存在的热备应返回错误")
	}
}

func TestDeviceFailure(t *testing.T) {
	m := NewManager()
	m.CreateArray("test-draid", DRAID1, []string{"/dev/sda", "/dev/sdb"}, []string{"/dev/sdc"}, 2, 1, "128K")

	// 报告设备故障，应自动分配热备
	err := m.ReportDeviceFailure("test-draid", "/dev/sda")
	if err != nil {
		t.Fatalf("ReportDeviceFailure() error = %v", err)
	}

	arr, _ := m.GetArray("test-draid")
	if arr.Status != StatusDegraded {
		t.Errorf("状态 = %v, 期望 %v", arr.Status, StatusDegraded)
	}
	if len(arr.FailedDevices) != 1 {
		t.Errorf("故障设备数 = %v, 期望 1", len(arr.FailedDevices))
	}

	// 重复报告故障
	err = m.ReportDeviceFailure("test-draid", "/dev/sda")
	if err == nil {
		t.Error("重复报告故障应返回错误")
	}

	// 报告不存在的设备
	err = m.ReportDeviceFailure("test-draid", "/dev/sdz")
	if err == nil {
		t.Error("报告不存在的设备应返回错误")
	}
}

func TestRebuild(t *testing.T) {
	m := NewManager()
	m.CreateArray("test-draid", DRAID1, []string{"/dev/sda", "/dev/sdb"}, []string{"/dev/sdc"}, 2, 1, "128K")

	// 阵列正常状态不能重建
	err := m.RebuildArray("test-draid")
	if err == nil {
		t.Error("正常状态阵列不能重建")
	}

	// 模拟设备故障
	m.ReportDeviceFailure("test-draid", "/dev/sda")

	// 触发重建
	err = m.RebuildArray("test-draid")
	if err != nil {
		t.Fatalf("RebuildArray() error = %v", err)
	}

	// 更新重建进度
	err = m.UpdateRebuildProgress("test-draid", 50)
	if err != nil {
		t.Fatalf("UpdateRebuildProgress() error = %v", err)
	}

	arr, _ := m.GetArray("test-draid")
	if arr.RebuildProgress != 50 {
		t.Errorf("重建进度 = %v, 期望 50", arr.RebuildProgress)
	}

	// 完成重建
	err = m.UpdateRebuildProgress("test-draid", 100)
	if err != nil {
		t.Fatalf("UpdateRebuildProgress() error = %v", err)
	}

	arr, _ = m.GetArray("test-draid")
	if arr.Status != StatusActive {
		t.Errorf("状态 = %v, 期望 %v", arr.Status, StatusActive)
	}
	if len(arr.FailedDevices) != 0 {
		t.Errorf("故障设备数 = %v, 期望 0", len(arr.FailedDevices))
	}
}

func TestReshare(t *testing.T) {
	m := NewManager()
	m.CreateArray("test-draid", DRAID1, []string{"/dev/sda", "/dev/sdb"}, nil, 2, 1, "128K")

	// 触发数据重分布
	newDevices := []string{"/dev/sdc", "/dev/sdd"}
	err := m.ReshareData("test-draid", newDevices)
	if err != nil {
		t.Fatalf("ReshareData() error = %v", err)
	}

	arr, _ := m.GetArray("test-draid")
	if arr.Status != StatusResharing {
		t.Errorf("状态 = %v, 期望 %v", arr.Status, StatusResharing)
	}

	// 更新重分布进度
	err = m.UpdateReshareProgress("test-draid", 50, newDevices)
	if err != nil {
		t.Fatalf("UpdateReshareProgress() error = %v", err)
	}

	// 完成重分布
	err = m.UpdateReshareProgress("test-draid", 100, newDevices)
	if err != nil {
		t.Fatalf("UpdateReshareProgress() error = %v", err)
	}

	arr, _ = m.GetArray("test-draid")
	if arr.Status != StatusActive {
		t.Errorf("状态 = %v, 期望 %v", arr.Status, StatusActive)
	}
	if len(arr.Devices) != 4 {
		t.Errorf("设备数 = %v, 期望 4", len(arr.Devices))
	}
}

func TestMetrics(t *testing.T) {
	m := NewManager()
	m.CreateArray("test-draid", DRAID1, []string{"/dev/sda", "/dev/sdb"}, nil, 2, 1, "128K")

	// 更新性能指标
	metrics := &PerformanceMetrics{
		IOPSRead:      1000,
		IOPSWrite:     500,
		ThroughputRead:  1024 * 1024 * 100,
		ThroughputWrite: 1024 * 1024 * 50,
		LatencyRead:   0.5,
		LatencyWrite:  1.2,
	}
	err := m.UpdateMetrics("test-draid", metrics)
	if err != nil {
		t.Fatalf("UpdateMetrics() error = %v", err)
	}

	// 获取性能指标
	metricsResult, err := m.GetMetrics("test-draid")
	if err != nil {
		t.Fatalf("GetMetrics() error = %v", err)
	}
	if metricsResult.IOPSRead != 1000 {
		t.Errorf("IOPSRead = %v, 期望 1000", metricsResult.IOPSRead)
	}
	if metricsResult.IOPSWrite != 500 {
		t.Errorf("IOPSWrite = %v, 期望 500", metricsResult.IOPSWrite)
	}
}

func TestGetArrayStatus(t *testing.T) {
	m := NewManager()
	m.CreateArray("test-draid", DRAID1, []string{"/dev/sda", "/dev/sdb"}, []string{"/dev/sdc"}, 2, 1, "128K")

	status, err := m.GetArrayStatus("test-draid")
	if err != nil {
		t.Fatalf("GetArrayStatus() error = %v", err)
	}

	if status["name"] != "test-draid" {
		t.Errorf("name = %v, 期望 test-draid", status["name"])
	}
	if status["level"] != DRAID1 {
		t.Errorf("level = %v, 期望 %v", status["level"], DRAID1)
	}
	if status["device_count"] != 2 {
		t.Errorf("device_count = %v, 期望 2", status["device_count"])
	}
	if status["spare_count"] != 1 {
		t.Errorf("spare_count = %v, 期望 1", status["spare_count"])
	}
}

// contains 检查字符串是否包含子串
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstr(s, substr))
}

func containsSubstr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
