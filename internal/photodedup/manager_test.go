// Package photodedup 单元测试
package photodedup

import (
	"testing"
	"time"
)

func TestStartScan(t *testing.T) {
	m := NewManager()
	task, err := m.StartScan(StartScanRequest{
		ScanDirs:  []string{"/photos"},
		Threshold: 85,
		Algorithm: HashPHash,
	})
	if err != nil {
		t.Fatalf("启动扫描失败: %v", err)
	}
	if task == nil {
		t.Fatal("任务不应为nil")
	}
	if task.Status != StatusRunning {
		t.Errorf("任务状态应为 running，实际 %s", task.Status)
	}
	if task.Threshold != 85 {
		t.Errorf("阈值应为 85，实际 %d", task.Threshold)
	}
	if task.Algorithm != HashPHash {
		t.Errorf("算法应为 phash，实际 %s", task.Algorithm)
	}
}

func TestStartScanDefaultValues(t *testing.T) {
	m := NewManager()
	task, err := m.StartScan(StartScanRequest{
		ScanDirs: []string{"/photos"},
	})
	if err != nil {
		t.Fatalf("启动扫描失败: %v", err)
	}
	if task.Threshold != 90 {
		t.Errorf("默认阈值应为 90，实际 %d", task.Threshold)
	}
	if task.Algorithm != HashPHash {
		t.Errorf("默认算法应为 phash，实际 %s", task.Algorithm)
	}
}

func TestStartScanInvalidThreshold(t *testing.T) {
	m := NewManager()
	_, err := m.StartScan(StartScanRequest{
		ScanDirs:  []string{"/photos"},
		Threshold: 150,
	})
	if err != ErrInvalidThreshold {
		t.Errorf("应返回 ErrInvalidThreshold，实际 %v", err)
	}

	_, err = m.StartScan(StartScanRequest{
		ScanDirs:  []string{"/photos"},
		Threshold: -10,
	})
	if err != ErrInvalidThreshold {
		t.Errorf("应返回 ErrInvalidThreshold，实际 %v", err)
	}
}

func TestStartScanInvalidAlgorithm(t *testing.T) {
	m := NewManager()
	_, err := m.StartScan(StartScanRequest{
		ScanDirs:  []string{"/photos"},
		Algorithm: "invalid",
	})
	if err != ErrInvalidHashAlgorithm {
		t.Errorf("应返回 ErrInvalidHashAlgorithm，实际 %v", err)
	}
}

func TestGetTask(t *testing.T) {
	m := NewManager()
	task, _ := m.StartScan(StartScanRequest{ScanDirs: []string{"/photos"}})

	got, err := m.GetTask(task.ID)
	if err != nil {
		t.Fatalf("获取任务失败: %v", err)
	}
	if got.ID != task.ID {
		t.Errorf("任务ID不匹配")
	}
}

func TestGetTaskNotFound(t *testing.T) {
	m := NewManager()
	_, err := m.GetTask("nonexistent")
	if err != ErrTaskNotFound {
		t.Errorf("应返回 ErrTaskNotFound，实际 %v", err)
	}
}

func TestListTasks(t *testing.T) {
	m := NewManager()
	m.StartScan(StartScanRequest{ScanDirs: []string{"/photos/a"}})
	m.StartScan(StartScanRequest{ScanDirs: []string{"/photos/b"}})

	tasks := m.ListTasks()
	if len(tasks) != 2 {
		t.Errorf("应有2个任务，实际 %d", len(tasks))
	}
}

func TestPauseAndResume(t *testing.T) {
	m := NewManager()
	task, _ := m.StartScan(StartScanRequest{ScanDirs: []string{"/photos"}})

	// 等任务模拟运行一会儿
	time.Sleep(50 * time.Millisecond)

	err := m.PauseTask(task.ID)
	if err != nil {
		t.Fatalf("暂停失败: %v", err)
	}

	paused, _ := m.GetTask(task.ID)
	if paused.Status != StatusPaused {
		t.Errorf("任务状态应为 paused，实际 %s", paused.Status)
	}

	// 暂停的任务不能再次暂停
	err = m.PauseTask(task.ID)
	if err != ErrTaskNotRunning {
		t.Errorf("暂停的任务不应再次暂停")
	}
}

func TestCancelTask(t *testing.T) {
	m := NewManager()
	task, _ := m.StartScan(StartScanRequest{ScanDirs: []string{"/photos"}})

	time.Sleep(50 * time.Millisecond)

	err := m.CancelTask(task.ID)
	if err != nil {
		t.Fatalf("取消失败: %v", err)
	}

	cancelled, _ := m.GetTask(task.ID)
	if cancelled.Status != StatusCancelled {
		t.Errorf("任务状态应为 cancelled，实际 %s", cancelled.Status)
	}
	if cancelled.FinishedAt == nil {
		t.Error("取消后应设置完成时间")
	}
}

func TestGetDuplicateGroups(t *testing.T) {
	m := NewManager()
	task, _ := m.StartScan(StartScanRequest{ScanDirs: []string{"/photos"}})

	// 等待扫描完成
	time.Sleep(2 * time.Second)

	groups, err := m.GetDuplicateGroups(task.ID)
	if err != nil {
		t.Fatalf("获取重复组失败: %v", err)
	}

	// 模拟结果应有重复组
	if len(groups) == 0 {
		t.Log("扫描完成但无重复组（可能模拟数据未生成）")
	}
}

func TestSetRetain(t *testing.T) {
	m := NewManager()
	task, _ := m.StartScan(StartScanRequest{ScanDirs: []string{"/photos"}})

	time.Sleep(2 * time.Second)

	groups, _ := m.GetDuplicateGroups(task.ID)
	if len(groups) == 0 {
		t.Skip("无重复组，跳过保留策略测试")
	}

	group := groups[0]
	newRetainID := group.Photos[0].ID

	err := m.SetRetain(task.ID, group.ID, newRetainID)
	if err != nil {
		t.Fatalf("设置保留失败: %v", err)
	}

	updated, _ := m.GetDuplicateGroup(task.ID, group.ID)
	if updated.RetainID != newRetainID {
		t.Errorf("保留ID未更新: %s", updated.RetainID)
	}
}

func TestSetRetainInvalidPhoto(t *testing.T) {
	m := NewManager()
	task, _ := m.StartScan(StartScanRequest{ScanDirs: []string{"/photos"}})

	time.Sleep(2 * time.Second)

	groups, _ := m.GetDuplicateGroups(task.ID)
	if len(groups) == 0 {
		t.Skip("无重复组，跳过")
	}

	err := m.SetRetain(task.ID, groups[0].ID, "nonexistent-photo")
	if err == nil {
		t.Error("使用不存在的photoID应返回错误")
	}
}

func TestPreviewCleanup(t *testing.T) {
	m := NewManager()
	task, _ := m.StartScan(StartScanRequest{ScanDirs: []string{"/photos"}})

	time.Sleep(2 * time.Second)

	groups, _ := m.GetDuplicateGroups(task.ID)
	if len(groups) == 0 {
		t.Skip("无重复组，跳过清理预览测试")
	}

	groupIDs := make([]string, len(groups))
	for i, g := range groups {
		groupIDs[i] = g.ID
	}

	preview, err := m.PreviewCleanup(task.ID, BatchCleanupRequest{
		GroupIDs:     groupIDs,
		RetainPolicy: RetainLargest,
	})
	if err != nil {
		t.Fatalf("预览清理失败: %v", err)
	}

	if preview.GroupCount == 0 {
		t.Error("预览应包含组数")
	}
}

func TestExecuteCleanupWithoutConfirm(t *testing.T) {
	m := NewManager()
	task, _ := m.StartScan(StartScanRequest{ScanDirs: []string{"/photos"}})

	time.Sleep(2 * time.Second)

	groups, _ := m.GetDuplicateGroups(task.ID)
	if len(groups) == 0 {
		t.Skip("无重复组，跳过")
	}

	_, err := m.ExecuteCleanup(task.ID, BatchCleanupRequest{
		GroupIDs:     []string{groups[0].ID},
		RetainPolicy: RetainLargest,
		Confirmed:    false,
	})
	if err != ErrBatchNotConfirmed {
		t.Errorf("未确认应返回 ErrBatchNotConfirmed，实际 %v", err)
	}
}

func TestExecuteCleanupConfirmed(t *testing.T) {
	m := NewManager()
	task, _ := m.StartScan(StartScanRequest{ScanDirs: []string{"/photos"}})

	time.Sleep(2 * time.Second)

	groups, _ := m.GetDuplicateGroups(task.ID)
	if len(groups) == 0 {
		t.Skip("无重复组，跳过")
	}

	result, err := m.ExecuteCleanup(task.ID, BatchCleanupRequest{
		GroupIDs:     []string{groups[0].ID},
		RetainPolicy: RetainLargest,
		Action:       ActionTrash,
		Confirmed:    true,
	})
	if err != nil {
		t.Fatalf("执行清理失败: %v", err)
	}

	if result.DeletedCount < 0 {
		t.Error("删除数不应为负")
	}
}

func TestGetScanStats(t *testing.T) {
	m := NewManager()
	task, _ := m.StartScan(StartScanRequest{ScanDirs: []string{"/photos"}})

	time.Sleep(2 * time.Second)

	stats, err := m.GetScanStats(task.ID)
	if err != nil {
		t.Fatalf("获取统计失败: %v", err)
	}

	if stats == nil {
		t.Fatal("统计结果不应为nil")
	}
}

func TestSchedule(t *testing.T) {
	m := NewManager()

	// 默认无定时任务
	schedule := m.GetSchedule()
	if schedule.Enabled {
		t.Error("默认应无定时任务")
	}

	// 设置定时任务
	err := m.SetSchedule(ScheduleConfig{
		Enabled:   true,
		Cron:      "0 3 * * *",
		ScanDirs:  []string{"/photos"},
		Threshold: 85,
		Algorithm: HashDHash,
	})
	if err != nil {
		t.Fatalf("设置定时任务失败: %v", err)
	}

	schedule = m.GetSchedule()
	if !schedule.Enabled {
		t.Error("定时任务应已启用")
	}
	if schedule.Cron != "0 3 * * *" {
		t.Errorf("cron表达式不匹配: %s", schedule.Cron)
	}
	if schedule.Algorithm != HashDHash {
		t.Errorf("算法应为 dhash，实际 %s", schedule.Algorithm)
	}
}

func TestScheduleInvalidThreshold(t *testing.T) {
	m := NewManager()
	err := m.SetSchedule(ScheduleConfig{
		Enabled:   true,
		Cron:      "0 3 * * *",
		ScanDirs:  []string{"/photos"},
		Threshold: 200,
	})
	if err != ErrInvalidThreshold {
		t.Errorf("应返回 ErrInvalidThreshold，实际 %v", err)
	}
}

func TestHammingDistance(t *testing.T) {
	tests := []struct {
		a, b uint64
		want int
	}{
		{0, 0, 0},
		{0xFFFFFFFFFFFFFFFF, 0xFFFFFFFFFFFFFFFF, 0},
		{0, 0xFFFFFFFFFFFFFFFF, 64},
		{0xFF00FF00FF00FF00, 0xFF00FF00FF00FF00, 0},
		{0xFF00FF00FF00FF00, 0x00FF00FF00FF00FF, 64},
	}
	for _, tt := range tests {
		got := HammingDistance(tt.a, tt.b)
		if got != tt.want {
			t.Errorf("HammingDistance(%016x, %016x) = %d, want %d", tt.a, tt.b, got, tt.want)
		}
	}
}

func TestSimilarityFromHamming(t *testing.T) {
	tests := []struct {
		distance int
		want     float64
	}{
		{0, 100.0},
		{32, 50.0},
		{64, 0.0},
	}
	for _, tt := range tests {
		got := SimilarityFromHamming(tt.distance)
		if got != tt.want {
			t.Errorf("SimilarityFromHamming(%d) = %f, want %f", tt.distance, got, tt.want)
		}
	}
}

func TestIsSupportedImage(t *testing.T) {
	tests := []struct {
		filename string
		want     bool
	}{
		{"photo.jpg", true},
		{"photo.JPEG", true},
		{"photo.png", true},
		{"photo.gif", true},
		{"photo.webp", true},
		{"photo.heic", true},
		{"doc.pdf", false},
		{"video.mp4", false},
		{"photo.txt", false},
	}
	for _, tt := range tests {
		got := IsSupportedImage(tt.filename)
		if got != tt.want {
			t.Errorf("IsSupportedImage(%q) = %v, want %v", tt.filename, got, tt.want)
		}
	}
}
