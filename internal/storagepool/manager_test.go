// Package storagepool 提供存储池管理功能
package storagepool

import (
	"os"
	"testing"
	"time"
)

func TestNewManager(t *testing.T) {
	// 创建临时目录
	tmpDir, err := os.MkdirTemp("", "storagepool-test-*")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	mgr, err := NewManager(tmpDir, tmpDir+"/mnt")
	if err != nil {
		t.Fatalf("创建管理器失败: %v", err)
	}

	if mgr == nil {
		t.Fatal("管理器不应为 nil")
	}

	if len(mgr.pools) != 0 {
		t.Errorf("新管理器应该没有存储池，实际有 %d 个", len(mgr.pools))
	}
}

func TestCreatePool(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "storagepool-test-*")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	mgr, err := NewManager(tmpDir, tmpDir+"/mnt")
	if err != nil {
		t.Fatalf("创建管理器失败: %v", err)
	}

	// 添加模拟设备
	mgr.devices["/dev/sda"] = &Device{
		ID:     "dev-sda",
		Path:   "/dev/sda",
		Name:   "sda",
		Size:   1_000_000_000_000, // 1TB
		Status: DeviceStatusOnline,
	}
	mgr.devices["/dev/sdb"] = &Device{
		ID:     "dev-sdb",
		Path:   "/dev/sdb",
		Name:   "sdb",
		Size:   1_000_000_000_000,
		Status: DeviceStatusOnline,
	}

	// 测试创建 RAID1 存储池
	req := &CreatePoolRequest{
		Name:        "test-pool",
		Description: "测试存储池",
		RAIDLevel:   RAIDLevelRAID1,
		DevicePaths: []string{"/dev/sda", "/dev/sdb"},
		FileSystem:  "btrfs",
	}

	pool, err := mgr.CreatePool(req)
	if err != nil {
		t.Fatalf("创建存储池失败: %v", err)
	}

	if pool.Name != "test-pool" {
		t.Errorf("存储池名称应为 'test-pool'，实际为 '%s'", pool.Name)
	}

	if pool.RAIDLevel != RAIDLevelRAID1 {
		t.Errorf("RAID 级别应为 'raid1'，实际为 '%s'", pool.RAIDLevel)
	}

	if len(pool.Devices) != 2 {
		t.Errorf("设备数量应为 2，实际为 %d", len(pool.Devices))
	}

	// 验证容量计算 (RAID1 可用容量为 50%)
	expectedSize := uint64(1_000_000_000_000) // 1TB
	if pool.Size != expectedSize {
		t.Errorf("存储池大小应为 %d，实际为 %d", expectedSize, pool.Size)
	}
}

func TestCreatePoolWithInvalidRAID(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "storagepool-test-*")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	mgr, _ := NewManager(tmpDir, tmpDir+"/mnt")

	// 添加设备
	mgr.devices["/dev/sda"] = &Device{
		ID:     "dev-sda",
		Path:   "/dev/sda",
		Name:   "sda",
		Size:   1_000_000_000_000,
		Status: DeviceStatusOnline,
	}

	// 测试设备数量不足
	req := &CreatePoolRequest{
		Name:        "test-pool",
		RAIDLevel:   RAIDLevelRAID1, // RAID1 需要至少 2 个设备
		DevicePaths: []string{"/dev/sda"},
	}

	_, err = mgr.CreatePool(req)
	if err == nil {
		t.Error("设备数量不足应该返回错误")
	}
}

func TestGetPool(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "storagepool-test-*")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	mgr, _ := NewManager(tmpDir, tmpDir+"/mnt")

	// 添加设备
	mgr.devices["/dev/sda"] = &Device{
		ID:     "dev-sda",
		Path:   "/dev/sda",
		Name:   "sda",
		Size:   1_000_000_000_000,
		Status: DeviceStatusOnline,
	}

	// 创建存储池
	req := &CreatePoolRequest{
		Name:        "test-pool",
		RAIDLevel:   RAIDLevelSingle,
		DevicePaths: []string{"/dev/sda"},
	}

	pool, _ := mgr.CreatePool(req)

	// 测试获取存储池
	got, err := mgr.GetPool(pool.ID)
	if err != nil {
		t.Fatalf("获取存储池失败: %v", err)
	}

	if got.ID != pool.ID {
		t.Errorf("存储池 ID 不匹配")
	}

	// 测试获取不存在的存储池
	_, err = mgr.GetPool("non-existent")
	if err == nil {
		t.Error("获取不存在的存储池应该返回错误")
	}
}

func TestListPools(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "storagepool-test-*")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	mgr, _ := NewManager(tmpDir, tmpDir+"/mnt")

	// 初始应该为空
	pools := mgr.ListPools()
	if len(pools) != 0 {
		t.Errorf("初始存储池列表应为空，实际有 %d 个", len(pools))
	}

	// 添加设备
	mgr.devices["/dev/sda"] = &Device{
		ID:     "dev-sda",
		Path:   "/dev/sda",
		Name:   "sda",
		Size:   1_000_000_000_000,
		Status: DeviceStatusOnline,
	}

	// 创建存储池
	mgr.CreatePool(&CreatePoolRequest{
		Name:        "pool1",
		RAIDLevel:   RAIDLevelSingle,
		DevicePaths: []string{"/dev/sda"},
	})

	pools = mgr.ListPools()
	if len(pools) != 1 {
		t.Errorf("存储池列表应有 1 个，实际有 %d 个", len(pools))
	}
}

func TestDeletePool(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "storagepool-test-*")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	mgr, _ := NewManager(tmpDir, tmpDir+"/mnt")

	// 添加设备
	mgr.devices["/dev/sda"] = &Device{
		ID:     "dev-sda",
		Path:   "/dev/sda",
		Name:   "sda",
		Size:   1_000_000_000_000,
		Status: DeviceStatusOnline,
	}

	// 创建存储池
	pool, _ := mgr.CreatePool(&CreatePoolRequest{
		Name:        "test-pool",
		RAIDLevel:   RAIDLevelSingle,
		DevicePaths: []string{"/dev/sda"},
	})

	// 删除存储池
	err = mgr.DeletePool(pool.ID, false)
	if err != nil {
		t.Fatalf("删除存储池失败: %v", err)
	}

	// 验证已删除
	pools := mgr.ListPools()
	if len(pools) != 0 {
		t.Errorf("删除后存储池列表应为空，实际有 %d 个", len(pools))
	}

	// 验证设备已释放（恢复为 Online 状态，可以再次使用）
	if mgr.devices["/dev/sda"].Status != DeviceStatusOnline {
		t.Errorf("删除存储池后设备状态应恢复为 Online，实际为 %s", mgr.devices["/dev/sda"].Status)
	}
}

func TestAddDevice(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "storagepool-test-*")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	mgr, _ := NewManager(tmpDir, tmpDir+"/mnt")

	// 添加设备
	mgr.devices["/dev/sda"] = &Device{
		ID:     "dev-sda",
		Path:   "/dev/sda",
		Name:   "sda",
		Size:   1_000_000_000_000,
		Status: DeviceStatusOnline,
	}
	mgr.devices["/dev/sdb"] = &Device{
		ID:     "dev-sdb",
		Path:   "/dev/sdb",
		Name:   "sdb",
		Size:   1_000_000_000_000,
		Status: DeviceStatusOnline,
	}
	mgr.devices["/dev/sdc"] = &Device{
		ID:     "dev-sdc",
		Path:   "/dev/sdc",
		Name:   "sdc",
		Size:   1_000_000_000_000,
		Status: DeviceStatusOnline,
	}

	// 创建 RAID5 存储池 (需要 3 个设备)
	pool, _ := mgr.CreatePool(&CreatePoolRequest{
		Name:        "test-pool",
		RAIDLevel:   RAIDLevelRAID5,
		DevicePaths: []string{"/dev/sda", "/dev/sdb", "/dev/sdc"},
	})

	// 添加热备盘
	mgr.devices["/dev/sdd"] = &Device{
		ID:     "dev-sdd",
		Path:   "/dev/sdd",
		Name:   "sdd",
		Size:   1_000_000_000_000,
		Status: DeviceStatusOnline,
	}

	updatedPool, err := mgr.AddDevice(pool.ID, &AddDeviceRequest{
		DevicePaths: []string{"/dev/sdd"},
		IsSpare:     true,
	})

	if err != nil {
		t.Fatalf("添加设备失败: %v", err)
	}

	if len(updatedPool.SpareDevices) != 1 {
		t.Errorf("热备设备数量应为 1，实际为 %d", len(updatedPool.SpareDevices))
	}
}

func TestRAIDConfigs(t *testing.T) {
	// 测试 RAID 配置
	tests := []struct {
		level          RAIDLevel
		expectedMin    int
		expectedToler  int
		expectedUsable float64
	}{
		{RAIDLevelSingle, 1, 0, 1.0},
		{RAIDLevelRAID0, 2, 0, 1.0},
		{RAIDLevelRAID1, 2, 1, 0.5},
		{RAIDLevelRAID5, 3, 1, 0.67},
		{RAIDLevelRAID6, 4, 2, 0.67},
		{RAIDLevelRAID10, 4, 1, 0.5},
	}

	for _, tt := range tests {
		config, ok := RAIDConfigs[tt.level]
		if !ok {
			t.Errorf("未找到 RAID 配置: %s", tt.level)
			continue
		}

		if config.MinDevices != tt.expectedMin {
			t.Errorf("%s: 最小设备数应为 %d，实际为 %d",
				tt.level, tt.expectedMin, config.MinDevices)
		}

		if config.FaultTolerance != tt.expectedToler {
			t.Errorf("%s: 容错数应为 %d，实际为 %d",
				tt.level, tt.expectedToler, config.FaultTolerance)
		}
	}
}

func TestPoolStats(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "storagepool-test-*")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	mgr, _ := NewManager(tmpDir, tmpDir+"/mnt")

	mgr.devices["/dev/sda"] = &Device{
		ID:     "dev-sda",
		Path:   "/dev/sda",
		Name:   "sda",
		Size:   1_000_000_000_000,
		Status: DeviceStatusOnline,
	}

	pool, _ := mgr.CreatePool(&CreatePoolRequest{
		Name:        "test-pool",
		RAIDLevel:   RAIDLevelSingle,
		DevicePaths: []string{"/dev/sda"},
	})

	stats, err := mgr.GetPoolStats(pool.ID)
	if err != nil {
		t.Fatalf("获取统计信息失败: %v", err)
	}

	if stats["name"] != "test-pool" {
		t.Errorf("统计信息中的名称应为 'test-pool'")
	}

	if stats["status"] != PoolStatusCreating {
		t.Errorf("新创建的存储池状态应为 'creating'")
	}
}

func TestMonitor(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "storagepool-test-*")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	mgr, _ := NewManager(tmpDir, tmpDir+"/mnt")
	monitor := NewMonitor(mgr)

	monitor.SetInterval(1 * time.Second)

	// 启动监控
	monitor.Start()

	// 验证正在运行
	if !monitor.running {
		t.Error("监控器应该正在运行")
	}

	// 停止监控
	monitor.Stop()

	if monitor.running {
		t.Error("监控器应该已停止")
	}
}

func TestService(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "storagepool-test-*")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	svc, err := NewService(tmpDir, tmpDir+"/mnt")
	if err != nil {
		t.Fatalf("创建服务失败: %v", err)
	}

	if svc.Manager == nil {
		t.Error("服务管理器不应为 nil")
	}

	if svc.Handlers == nil {
		t.Error("服务处理器不应为 nil")
	}

	// 初始化
	if err := svc.Initialize(); err != nil {
		t.Fatalf("初始化服务失败: %v", err)
	}

	// 关闭
	svc.Close()
}
