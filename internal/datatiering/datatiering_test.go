package datatiering

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestNewManager(t *testing.T) {
	m := NewManager()

	if m == nil {
		t.Fatal("NewManager() 返回 nil")
	}
	if m.policies == nil {
		t.Error("policies map 未初始化")
	}
	if m.files == nil {
		t.Error("files map 未初始化")
	}
	if m.jobs == nil {
		t.Error("jobs map 未初始化")
	}
	if m.tierCap == nil {
		t.Error("tierCap map 未初始化")
	}
	if m.stopCh == nil {
		t.Error("stopCh 未初始化")
	}

	// 验证默认容量
	if m.tierCap[TierHot] != 500*1024*1024*1024 {
		t.Errorf("TierHot 容量应为 500GB，实际 %d", m.tierCap[TierHot])
	}
	if m.tierCap[TierWarm] != 2000*1024*1024*1024 {
		t.Errorf("TierWarm 容量应为 2TB，实际 %d", m.tierCap[TierWarm])
	}
	if m.tierCap[TierCold] != 8000*1024*1024*1024 {
		t.Errorf("TierCold 容量应为 8TB，实际 %d", m.tierCap[TierCold])
	}
}

func TestAddPolicy(t *testing.T) {
	m := NewManager()

	// 测试正常添加
	policy := &TierPolicy{
		ID:             "test-policy",
		Name:           "测试策略",
		Enabled:        true,
		HotToWarmDays:  30,
		WarmToColdDays: 90,
	}
	if err := m.AddPolicy(policy); err != nil {
		t.Fatalf("AddPolicy 失败: %v", err)
	}

	// 验证策略已添加
	policies := m.ListPolicies()
	if len(policies) != 1 {
		t.Fatalf("策略数量应为 1，实际 %d", len(policies))
	}
	if policies[0].ID != "test-policy" {
		t.Errorf("策略ID 应为 test-policy，实际 %s", policies[0].ID)
	}

	// 测试空 ID
	if err := m.AddPolicy(&TierPolicy{}); err == nil {
		t.Error("空 ID 应返回错误")
	}

	// 测试默认值
	policy2 := &TierPolicy{
		ID: "default-test",
	}
	if err := m.AddPolicy(policy2); err != nil {
		t.Fatalf("AddPolicy 失败: %v", err)
	}
	if policy2.HotToWarmDays != 30 {
		t.Errorf("默认 HotToWarmDays 应为 30，实际 %d", policy2.HotToWarmDays)
	}
	if policy2.WarmToColdDays != 90 {
		t.Errorf("默认 WarmToColdDays 应为 90，实际 %d", policy2.WarmToColdDays)
	}
}

func TestUpdatePolicy(t *testing.T) {
	m := NewManager()

	// 先添加策略
	policy := &TierPolicy{
		ID:             "update-test",
		Name:           "原始名称",
		HotToWarmDays:  30,
		WarmToColdDays: 90,
	}
	if err := m.AddPolicy(policy); err != nil {
		t.Fatalf("AddPolicy 失败: %v", err)
	}

	// 更新策略
	policy.Name = "更新后名称"
	policy.HotToWarmDays = 60
	if err := m.UpdatePolicy(policy); err != nil {
		t.Fatalf("UpdatePolicy 失败: %v", err)
	}

	// 验证更新
	policies := m.ListPolicies()
	if policies[0].Name != "更新后名称" {
		t.Errorf("策略名称应为 更新后名称，实际 %s", policies[0].Name)
	}
	if policies[0].HotToWarmDays != 60 {
		t.Errorf("HotToWarmDays 应为 60，实际 %d", policies[0].HotToWarmDays)
	}

	// 测试不存在的策略
	if err := m.UpdatePolicy(&TierPolicy{ID: "non-existent"}); err == nil {
		t.Error("更新不存在的策略应返回错误")
	}
}

func TestDeletePolicy(t *testing.T) {
	m := NewManager()

	// 添加策略
	policy := &TierPolicy{
		ID:             "delete-test",
		Name:           "待删除",
		HotToWarmDays:  30,
		WarmToColdDays: 90,
	}
	if err := m.AddPolicy(policy); err != nil {
		t.Fatalf("AddPolicy 失败: %v", err)
	}

	// 删除策略
	if !m.DeletePolicy("delete-test") {
		t.Error("删除存在的策略应返回 true")
	}

	// 验证已删除
	policies := m.ListPolicies()
	if len(policies) != 0 {
		t.Errorf("删除后策略数量应为 0，实际 %d", len(policies))
	}

	// 测试删除不存在的策略
	if m.DeletePolicy("non-existent") {
		t.Error("删除不存在的策略应返回 false")
	}
}

func TestListPolicies(t *testing.T) {
	m := NewManager()

	// 空列表
	policies := m.ListPolicies()
	if len(policies) != 0 {
		t.Errorf("初始策略数量应为 0，实际 %d", len(policies))
	}

	// 添加多个策略
	for i := 0; i < 3; i++ {
		m.AddPolicy(&TierPolicy{
			ID:             fmt.Sprintf("policy-%d", i),
			Name:           fmt.Sprintf("策略 %d", i),
			HotToWarmDays:  30,
			WarmToColdDays: 90,
		})
	}

	policies = m.ListPolicies()
	if len(policies) != 3 {
		t.Errorf("策略数量应为 3，实际 %d", len(policies))
	}
}

func TestRegisterFile(t *testing.T) {
	m := NewManager()

	file := &TieredFile{
		Path:         "/data/test.txt",
		Size:         1024,
		CurrentTier:  TierHot,
		LastAccessed: time.Now().Add(-24 * time.Hour),
		LastModified: time.Now().Add(-24 * time.Hour),
		AccessCount:  10,
	}

	m.RegisterFile(file)

	// 通过 GetTierStats 间接验证文件已注册
	stats := m.GetTierStats()
	found := false
	for _, s := range stats {
		if s.Tier == TierHot && s.FileCount == 1 {
			found = true
			break
		}
	}
	if !found {
		t.Error("注册的文件未在统计中体现")
	}
}

func TestAnalyzeAndMigrate(t *testing.T) {
	m := NewManager()

	// 添加策略
	policy := &TierPolicy{
		ID:             "migrate-test",
		Name:           "迁移测试",
		HotToWarmDays:  30,
		WarmToColdDays: 90,
		MinFileSize:    100,
		MaxFileSize:    10000,
	}
	if err := m.AddPolicy(policy); err != nil {
		t.Fatalf("AddPolicy 失败: %v", err)
	}

	// 注册热文件（超过30天未访问）
	m.RegisterFile(&TieredFile{
		Path:         "/data/hot-file.txt",
		Size:         1024,
		CurrentTier:  TierHot,
		LastAccessed: time.Now().Add(-31 * 24 * time.Hour),
		AccessCount:  5,
	})

	// 注册温文件（超过90天未访问）
	m.RegisterFile(&TieredFile{
		Path:         "/data/warm-file.txt",
		Size:         2048,
		CurrentTier:  TierWarm,
		LastAccessed: time.Now().Add(-91 * 24 * time.Hour),
		AccessCount:  3,
	})

	// 执行迁移
	job, err := m.AnalyzeAndMigrate("migrate-test")
	if err != nil {
		t.Fatalf("AnalyzeAndMigrate 失败: %v", err)
	}

	if job == nil {
		t.Fatal("返回的 job 不应为 nil")
	}
	if job.PolicyID != "migrate-test" {
		t.Errorf("job.PolicyID 应为 migrate-test，实际 %s", job.PolicyID)
	}
	if job.TotalFiles == 0 {
		t.Error("应有文件需要迁移")
	}

	// 等待异步迁移完成
	time.Sleep(100 * time.Millisecond)

	// 验证任务已完成
	updatedJob, err := m.GetJob(job.ID)
	if err != nil {
		t.Fatalf("GetJob 失败: %v", err)
	}
	if updatedJob.Status != JobCompleted {
		t.Errorf("任务状态应为 completed，实际 %s", updatedJob.Status)
	}
	if updatedJob.Migrated != updatedJob.TotalFiles {
		t.Errorf("迁移文件数应等于总文件数，实际 %d/%d", updatedJob.Migrated, updatedJob.TotalFiles)
	}

	// 测试不存在的策略
	_, err = m.AnalyzeAndMigrate("non-existent")
	if err == nil {
		t.Error("不存在的策略应返回错误")
	}
}

func TestGetJob(t *testing.T) {
	m := NewManager()

	// 添加策略和文件
	m.AddPolicy(&TierPolicy{
		ID:             "job-test",
		HotToWarmDays:  30,
		WarmToColdDays: 90,
	})
	m.RegisterFile(&TieredFile{
		Path:         "/data/test.txt",
		Size:         1024,
		CurrentTier:  TierHot,
		LastAccessed: time.Now().Add(-31 * 24 * time.Hour),
	})

	job, _ := m.AnalyzeAndMigrate("job-test")

	// 获取存在的任务
	got, err := m.GetJob(job.ID)
	if err != nil {
		t.Fatalf("GetJob 失败: %v", err)
	}
	if got.ID != job.ID {
		t.Errorf("任务ID 不匹配，期望 %s，实际 %s", job.ID, got.ID)
	}

	// 获取不存在的任务
	_, err = m.GetJob("non-existent-job")
	if err == nil {
		t.Error("不存在的任务应返回错误")
	}
}

func TestListJobs(t *testing.T) {
	m := NewManager()

	// 初始无任务
	jobs := m.ListJobs()
	if len(jobs) != 0 {
		t.Errorf("初始任务数量应为 0，实际 %d", len(jobs))
	}

	// 添加策略和文件
	m.AddPolicy(&TierPolicy{
		ID:             "list-job-test",
		HotToWarmDays:  30,
		WarmToColdDays: 90,
	})
	m.RegisterFile(&TieredFile{
		Path:         "/data/test.txt",
		Size:         1024,
		CurrentTier:  TierHot,
		LastAccessed: time.Now().Add(-31 * 24 * time.Hour),
	})

	// 执行迁移
	m.AnalyzeAndMigrate("list-job-test")

	jobs = m.ListJobs()
	if len(jobs) == 0 {
		t.Error("应有迁移任务")
	}
}

func TestGetTierStats(t *testing.T) {
	m := NewManager()

	// 注册不同层级的文件
	m.RegisterFile(&TieredFile{
		Path:        "/data/hot1.txt",
		Size:        1024,
		CurrentTier: TierHot,
	})
	m.RegisterFile(&TieredFile{
		Path:        "/data/hot2.txt",
		Size:        2048,
		CurrentTier: TierHot,
	})
	m.RegisterFile(&TieredFile{
		Path:        "/data/warm1.txt",
		Size:        4096,
		CurrentTier: TierWarm,
	})
	m.RegisterFile(&TieredFile{
		Path:        "/data/cold1.txt",
		Size:        8192,
		CurrentTier: TierCold,
	})

	stats := m.GetTierStats()

	if len(stats) != 3 {
		t.Fatalf("应有 3 个层级统计，实际 %d", len(stats))
	}

	statsMap := map[StorageTier]*TierStats{}
	for i := range stats {
		statsMap[stats[i].Tier] = &stats[i]
	}

	// 验证 Hot 层
	if statsMap[TierHot].FileCount != 2 {
		t.Errorf("Hot 层文件数应为 2，实际 %d", statsMap[TierHot].FileCount)
	}
	if statsMap[TierHot].TotalSize != 3072 {
		t.Errorf("Hot 层总大小应为 3072，实际 %d", statsMap[TierHot].TotalSize)
	}

	// 验证 Warm 层
	if statsMap[TierWarm].FileCount != 1 {
		t.Errorf("Warm 层文件数应为 1，实际 %d", statsMap[TierWarm].FileCount)
	}

	// 验证 Cold 层
	if statsMap[TierCold].FileCount != 1 {
		t.Errorf("Cold 层文件数应为 1，实际 %d", statsMap[TierCold].FileCount)
	}

	// 验证百分比计算
	if statsMap[TierHot].UsedPercent <= 0 {
		t.Error("UsedPercent 应大于 0")
	}
	if statsMap[TierHot].AvailableGB <= 0 {
		t.Error("AvailableGB 应大于 0")
	}
}

func TestGetReport(t *testing.T) {
	m := NewManager()

	// 注册文件
	m.RegisterFile(&TieredFile{
		Path:        "/data/test.txt",
		Size:        1024,
		CurrentTier: TierHot,
	})

	report := m.GetReport()

	if report == nil {
		t.Fatal("GetReport() 返回 nil")
	}
	if report.TotalFiles != 1 {
		t.Errorf("TotalFiles 应为 1，实际 %d", report.TotalFiles)
	}
	if report.TotalSize != 1024 {
		t.Errorf("TotalSize 应为 1024，实际 %d", report.TotalSize)
	}
	if len(report.Tiers) != 3 {
		t.Errorf("Tiers 数量应为 3，实际 %d", len(report.Tiers))
	}
	if report.GeneratedAt.IsZero() {
		t.Error("GeneratedAt 不应为零值")
	}
	if report.Suggestions == nil {
		t.Error("Suggestions 不应为 nil")
	}
	if report.RecentJobs == nil {
		t.Error("RecentJobs 不应为 nil")
	}
}

func TestEdgeCases(t *testing.T) {
	t.Run("空策略ID", func(t *testing.T) {
		m := NewManager()
		if err := m.AddPolicy(&TierPolicy{}); err == nil {
			t.Error("空 ID 应返回错误")
		}
	})

	t.Run("更新不存在的策略", func(t *testing.T) {
		m := NewManager()
		if err := m.UpdatePolicy(&TierPolicy{ID: "ghost"}); err == nil {
			t.Error("应返回错误")
		}
	})

	t.Run("删除不存在的策略", func(t *testing.T) {
		m := NewManager()
		if m.DeletePolicy("ghost") {
			t.Error("应返回 false")
		}
	})

	t.Run("获取不存在的任务", func(t *testing.T) {
		m := NewManager()
		_, err := m.GetJob("ghost")
		if err == nil {
			t.Error("应返回错误")
		}
	})

	t.Run("不存在策略的迁移", func(t *testing.T) {
		m := NewManager()
		_, err := m.AnalyzeAndMigrate("ghost")
		if err == nil {
			t.Error("应返回错误")
		}
	})

	t.Run("无文件时迁移", func(t *testing.T) {
		m := NewManager()
		m.AddPolicy(&TierPolicy{
			ID:             "empty",
			HotToWarmDays:  30,
			WarmToColdDays: 90,
		})
		job, err := m.AnalyzeAndMigrate("empty")
		if err != nil {
			t.Fatalf("不应报错: %v", err)
		}
		if job.Status != JobCompleted {
			t.Errorf("无文件迁移应直接完成，实际 %s", job.Status)
		}
		if job.TotalFiles != 0 {
			t.Errorf("TotalFiles 应为 0，实际 %d", job.TotalFiles)
		}
	})

	t.Run("排除路径", func(t *testing.T) {
		m := NewManager()
		m.AddPolicy(&TierPolicy{
			ID:             "exclude-test",
			HotToWarmDays:  30,
			WarmToColdDays: 90,
			ExcludePaths:   []string{"/data/protected/"},
		})
		m.RegisterFile(&TieredFile{
			Path:         "/data/protected/secret.txt",
			Size:         1024,
			CurrentTier:  TierHot,
			LastAccessed: time.Now().Add(-60 * 24 * time.Hour),
		})
		job, _ := m.AnalyzeAndMigrate("exclude-test")
		if job.TotalFiles != 0 {
			t.Error("被排除路径的文件不应迁移")
		}
	})

	t.Run("文件大小过滤", func(t *testing.T) {
		m := NewManager()
		m.AddPolicy(&TierPolicy{
			ID:             "size-test",
			HotToWarmDays:  30,
			WarmToColdDays: 90,
			MinFileSize:    1000,
			MaxFileSize:    5000,
		})
		m.RegisterFile(&TieredFile{
			Path:         "/data/small.txt",
			Size:         500,
			CurrentTier:  TierHot,
			LastAccessed: time.Now().Add(-60 * 24 * time.Hour),
		})
		m.RegisterFile(&TieredFile{
			Path:         "/data/large.txt",
			Size:         10000,
			CurrentTier:  TierHot,
			LastAccessed: time.Now().Add(-60 * 24 * time.Hour),
		})
		m.RegisterFile(&TieredFile{
			Path:         "/data/medium.txt",
			Size:         3000,
			CurrentTier:  TierHot,
			LastAccessed: time.Now().Add(-60 * 24 * time.Hour),
		})
		job, _ := m.AnalyzeAndMigrate("size-test")
		if job.TotalFiles != 1 {
			t.Errorf("只有 medium.txt 应被迁移，实际 %d 个文件", job.TotalFiles)
		}
	})

	t.Run("冷数据提升", func(t *testing.T) {
		m := NewManager()
		m.AddPolicy(&TierPolicy{
			ID:             "cold-promote",
			HotToWarmDays:  30,
			WarmToColdDays: 90,
		})
		m.RegisterFile(&TieredFile{
			Path:         "/data/active-cold.txt",
			Size:         1024,
			CurrentTier:  TierCold,
			LastAccessed: time.Now().Add(-3 * 24 * time.Hour),
			AccessCount:  150,
		})
		job, _ := m.AnalyzeAndMigrate("cold-promote")
		if job.TotalFiles != 1 {
			t.Error("高访问频率的冷数据应被提升")
		}
	})

	t.Run("并发策略操作", func(t *testing.T) {
		m := NewManager()
		var wg sync.WaitGroup

		// 并发添加策略
		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				m.AddPolicy(&TierPolicy{
					ID:             fmt.Sprintf("concurrent-%d", id),
					HotToWarmDays:  30,
					WarmToColdDays: 90,
				})
			}(i)
		}
		wg.Wait()

		policies := m.ListPolicies()
		if len(policies) != 10 {
			t.Errorf("并发添加后策略数量应为 10，实际 %d", len(policies))
		}

		// 并发读取
		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				m.ListPolicies()
				m.GetTierStats()
			}()
		}
		wg.Wait()
	})

	t.Run("并发文件操作", func(t *testing.T) {
		m := NewManager()
		var wg sync.WaitGroup

		// 并发注册文件
		for i := 0; i < 100; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				m.RegisterFile(&TieredFile{
					Path:        fmt.Sprintf("/data/file-%d.txt", id),
					Size:        int64(id * 100),
					CurrentTier: TierHot,
				})
			}(i)
		}
		wg.Wait()

		stats := m.GetTierStats()
		for _, s := range stats {
			if s.Tier == TierHot && s.FileCount != 100 {
				t.Errorf("Hot 层应有 100 个文件，实际 %d", s.FileCount)
			}
		}
	})
}

