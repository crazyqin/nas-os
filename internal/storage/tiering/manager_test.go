package tiering

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNewManager(t *testing.T) {
	config := DefaultManagerConfig()
	m := NewManager(config)

	if m == nil {
		t.Fatal("创建管理器失败")
	}

	if m.config.CheckInterval != time.Hour {
		t.Errorf("CheckInterval期望1小时，实际%v", m.config.CheckInterval)
	}

	if m.config.HotThreshold != 100 {
		t.Errorf("HotThreshold期望100，实际%d", m.config.HotThreshold)
	}
}

func TestInitialize(t *testing.T) {
	// 创建临时配置目录
	tmpDir, err := os.MkdirTemp("", "tiering-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	config := DefaultManagerConfig()
	config.ConfigPath = filepath.Join(tmpDir, "tiering.json")

	m := NewManager(config)
	if err := m.Initialize(); err != nil {
		t.Fatalf("初始化失败: %v", err)
	}

	// 验证默认存储层
	tiers := m.ListTiers()
	if len(tiers) != 3 {
		t.Errorf("期望3个存储层，实际%d", len(tiers))
	}

	// 验证默认策略
	policies := m.ListPolicies()
	if len(policies) != 2 {
		t.Errorf("期望2个策略，实际%d", len(policies))
	}
}

func TestCreateTier(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "tiering-test-*")
	defer os.RemoveAll(tmpDir)

	config := DefaultManagerConfig()
	config.ConfigPath = filepath.Join(tmpDir, "tiering.json")

	m := NewManager(config)
	_ = m.Initialize()

	// 创建新存储层
	tier := TierConfig{
		Type:     "nvme",
		Name:     "NVMe缓存层",
		Path:     "/mnt/nvme",
		Priority: 150,
		Enabled:  true,
	}

	if err := m.CreateTier(tier); err != nil {
		t.Fatalf("创建存储层失败: %v", err)
	}

	// 验证
	tiers := m.ListTiers()
	found := false
	for _, t := range tiers {
		if t.Type == "nvme" {
			found = true
			break
		}
	}

	if !found {
		t.Error("未找到新创建的存储层")
	}
}

func TestRecordAccess(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "tiering-test-*")
	defer os.RemoveAll(tmpDir)

	// 创建测试文件
	testFile := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("test content"), 0644); err != nil {
		t.Fatal(err)
	}

	config := DefaultManagerConfig()
	config.ConfigPath = filepath.Join(tmpDir, "tiering.json")

	m := NewManager(config)
	_ = m.Initialize()

	// 记录访问
	if err := m.RecordAccess(testFile, TierTypeSSD, 100, 50); err != nil {
		t.Fatalf("记录访问失败: %v", err)
	}

	// 验证记录
	record, err := m.GetRecord(testFile)
	if err != nil {
		t.Fatalf("获取记录失败: %v", err)
	}

	if record.AccessCount != 1 {
		t.Errorf("AccessCount期望1，实际%d", record.AccessCount)
	}

	if record.ReadBytes != 100 {
		t.Errorf("ReadBytes期望100，实际%d", record.ReadBytes)
	}
}

func TestCalculateFrequency(t *testing.T) {
	config := DefaultManagerConfig()
	m := NewManager(config)
	_ = m.Initialize()

	tests := []struct {
		name      string
		record    *FileAccessRecord
		expected  AccessFrequency
	}{
		{
			name: "热数据",
			record: &FileAccessRecord{
				AccessCount: 100,
				AccessTime:  time.Now(),
			},
			expected: AccessFrequencyHot,
		},
		{
			name: "冷数据",
			record: &FileAccessRecord{
				AccessCount: 5,
				AccessTime:  time.Now().Add(-1000 * time.Hour),
			},
			expected: AccessFrequencyCold,
		},
		{
			name: "温数据",
			record: &FileAccessRecord{
				AccessCount: 50,
				AccessTime:  time.Now().Add(-2 * time.Hour),
			},
			expected: AccessFrequencyWarm,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := m.calculateFrequency(tt.record)
			if result != tt.expected {
				t.Errorf("期望%s，实际%s", tt.expected, result)
			}
		})
	}
}

func TestGetHotFiles(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "tiering-test-*")
	defer os.RemoveAll(tmpDir)

	config := DefaultManagerConfig()
	config.ConfigPath = filepath.Join(tmpDir, "tiering.json")

	m := NewManager(config)
	_ = m.Initialize()

	// 添加热数据记录
	for i := 0; i < 5; i++ {
		testFile := filepath.Join(tmpDir, "hot_"+string(rune('a'+i))+".txt")
		os.WriteFile(testFile, []byte("test"), 0644)
		m.RecordAccess(testFile, TierTypeHDD, 100, 0)
		// 增加访问次数到热数据阈值
		for j := 0; j < 100; j++ {
			m.RecordAccess(testFile, TierTypeHDD, 1, 0)
		}
	}

	// 添加冷数据记录
	for i := 0; i < 3; i++ {
		testFile := filepath.Join(tmpDir, "cold_"+string(rune('a'+i))+".txt")
		os.WriteFile(testFile, []byte("test"), 0644)
		record := &FileAccessRecord{
			Path:        testFile,
			CurrentTier: TierTypeSSD,
			AccessTime:  time.Now().Add(-1000 * time.Hour),
			Frequency:   AccessFrequencyCold,
		}
		m.records[testFile] = record
		// 更新存储层索引
		if m.recordsByTier[TierTypeSSD] == nil {
			m.recordsByTier[TierTypeSSD] = make(map[string]*FileAccessRecord)
		}
		m.recordsByTier[TierTypeSSD][testFile] = record
	}

	// 获取热数据
	hotFiles := m.GetHotFiles(TierTypeHDD, 10)
	if len(hotFiles) != 5 {
		t.Errorf("期望5个热数据文件，实际%d", len(hotFiles))
	}

	// 获取冷数据
	coldFiles := m.GetColdFiles(TierTypeSSD, 10)
	if len(coldFiles) != 3 {
		t.Errorf("期望3个冷数据文件，实际%d", len(coldFiles))
	}
}

func TestMigrateHotToSSD(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "tiering-test-*")
	defer os.RemoveAll(tmpDir)

	config := DefaultManagerConfig()
	config.ConfigPath = filepath.Join(tmpDir, "tiering.json")

	m := NewManager(config)
	_ = m.Initialize()

	// 设置测试存储层
	ssdPath := filepath.Join(tmpDir, "ssd")
	hddPath := filepath.Join(tmpDir, "hdd")
	os.MkdirAll(ssdPath, 0755)
	os.MkdirAll(hddPath, 0755)

	m.UpdateTier(TierConfig{
		Type:     TierTypeSSD,
		Name:     "SSD",
		Path:     ssdPath,
		Capacity: 1 << 30, // 1GB
		Enabled:  true,
	})

	m.UpdateTier(TierConfig{
		Type:     TierTypeHDD,
		Name:     "HDD",
		Path:     hddPath,
		Capacity: 10 << 30, // 10GB
		Enabled:  true,
	})

	// 创建热数据文件
	testFile := filepath.Join(hddPath, "hot_file.txt")
	os.WriteFile(testFile, []byte("hot data content"), 0644)

	// 记录访问（达到热数据阈值）
	for i := 0; i < 101; i++ {
		m.RecordAccess(testFile, TierTypeHDD, 100, 0)
	}

	// 执行迁移
	task, err := m.MigrateHotToSSD(context.Background())
	if err != nil {
		t.Fatalf("迁移失败: %v", err)
	}

	if task.Status != "pending" && task.Status != "running" {
		t.Errorf("任务状态异常: %s", task.Status)
	}

	// 等待迁移完成
	time.Sleep(100 * time.Millisecond)

	// 获取任务状态
	updatedTask, err := m.GetTask(task.ID)
	if err != nil {
		t.Fatalf("获取任务失败: %v", err)
	}

	if updatedTask.Status != "completed" && updatedTask.Status != "running" {
		t.Errorf("迁移未完成，状态: %s", updatedTask.Status)
	}
}

func TestPolicyManagement(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "tiering-test-*")
	defer os.RemoveAll(tmpDir)

	config := DefaultManagerConfig()
	config.ConfigPath = filepath.Join(tmpDir, "tiering.json")

	m := NewManager(config)
	_ = m.Initialize()

	// 创建新策略
	policy := PolicyConfig{
		ID:             "test-policy",
		Name:           "测试策略",
		Enabled:        true,
		SourceTier:     TierTypeSSD,
		TargetTier:     TierTypeHDD,
		MinAccessCount: 50,
		MaxAccessAge:   48 * time.Hour,
	}

	if err := m.CreatePolicy(policy); err != nil {
		t.Fatalf("创建策略失败: %v", err)
	}

	// 获取策略
	p, err := m.GetPolicy("test-policy")
	if err != nil {
		t.Fatalf("获取策略失败: %v", err)
	}

	if p.Name != "测试策略" {
		t.Errorf("策略名称错误: %s", p.Name)
	}

	// 更新策略
	p.MinAccessCount = 100
	if err := m.UpdatePolicy(*p); err != nil {
		t.Fatalf("更新策略失败: %v", err)
	}

	// 删除策略
	if err := m.DeletePolicy("test-policy"); err != nil {
		t.Fatalf("删除策略失败: %v", err)
	}

	// 验证删除
	policies := m.ListPolicies()
	for _, p := range policies {
		if p.ID == "test-policy" {
			t.Error("策略未被删除")
		}
	}
}

func TestGetStatus(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "tiering-test-*")
	defer os.RemoveAll(tmpDir)

	config := DefaultManagerConfig()
	config.ConfigPath = filepath.Join(tmpDir, "tiering.json")

	m := NewManager(config)
	_ = m.Initialize()

	status := m.GetStatus()

	if _, ok := status["enabled"]; !ok {
		t.Error("状态中缺少enabled字段")
	}

	if _, ok := status["totalPolicies"]; !ok {
		t.Error("状态中缺少totalPolicies字段")
	}
}

func TestGetTierStats(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "tiering-test-*")
	defer os.RemoveAll(tmpDir)

	config := DefaultManagerConfig()
	config.ConfigPath = filepath.Join(tmpDir, "tiering.json")

	m := NewManager(config)
	_ = m.Initialize()

	stats, err := m.GetTierStats(TierTypeSSD)
	if err != nil {
		t.Fatalf("获取存储层统计失败: %v", err)
	}

	if stats["type"] != TierTypeSSD {
		t.Errorf("存储层类型错误: %v", stats["type"])
	}
}

func TestConfigPersistence(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "tiering-test-*")
	defer os.RemoveAll(tmpDir)

	configPath := filepath.Join(tmpDir, "tiering.json")
	config := DefaultManagerConfig()
	config.ConfigPath = configPath

	// 创建并初始化管理器
	m := NewManager(config)
	_ = m.Initialize()

	// 修改配置
	tier := TierConfig{
		Type:     "custom",
		Name:     "自定义存储层",
		Path:     "/mnt/custom",
		Priority: 200,
		Enabled:  true,
	}
	_ = m.CreateTier(tier)

	// 创建新管理器加载配置
	m2 := NewManager(config)
	if err := m2.loadConfig(); err != nil {
		t.Fatalf("加载配置失败: %v", err)
	}

	// 验证配置
	tiers := m2.ListTiers()
	found := false
	for _, t := range tiers {
		if t.Type == "custom" {
			found = true
			break
		}
	}

	if !found {
		t.Error("配置持久化失败，未找到自定义存储层")
	}
}