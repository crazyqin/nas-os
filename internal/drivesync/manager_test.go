// Package drivesync 单元测试
package drivesync

import (
	"testing"
	"time"
)

func newTestManager() *Manager {
	return NewManager("", VersionConfig{
		Enabled:       true,
		MaxVersions:   10,
		RetentionDays: 30,
	})
}

// ========== 同步任务测试 ==========

func TestCreateTask(t *testing.T) {
	m := newTestManager()

	input := SyncTaskInput{
		Name:       "测试同步任务",
		LocalPath:  "/data/local",
		RemotePath: "/data/remote",
		Direction:  SyncBidirectional,
		Enabled:    true,
		Interval:   5 * time.Minute,
	}

	task, err := m.CreateTask(input)
	if err != nil {
		t.Fatalf("创建任务失败: %v", err)
	}

	if task.ID == "" {
		t.Error("任务ID不应为空")
	}
	if task.Name != "测试同步任务" {
		t.Errorf("任务名称不匹配，期望: 测试同步任务，实际: %s", task.Name)
	}
	if task.Direction != SyncBidirectional {
		t.Errorf("同步方向不匹配，期望: bidirectional，实际: %s", task.Direction)
	}
	if task.Status != TaskStatusIdle {
		t.Errorf("任务状态不匹配，期望: idle，实际: %s", task.Status)
	}
	if !task.Enabled {
		t.Error("任务应为启用状态")
	}
}

func TestGetTask(t *testing.T) {
	m := newTestManager()

	input := SyncTaskInput{
		Name:       "获取任务测试",
		LocalPath:  "/data/local",
		RemotePath: "/data/remote",
	}

	task, err := m.CreateTask(input)
	if err != nil {
		t.Fatalf("创建任务失败: %v", err)
	}

	// 获取任务
	got, err := m.GetTask(task.ID)
	if err != nil {
		t.Fatalf("获取任务失败: %v", err)
	}
	if got.ID != task.ID {
		t.Errorf("任务ID不匹配")
	}

	// 获取不存在的任务
	_, err = m.GetTask("non-existent")
	if err != ErrSyncTaskNotFound {
		t.Errorf("期望 ErrSyncTaskNotFound，实际: %v", err)
	}
}

func TestListTasks(t *testing.T) {
	m := newTestManager()

	// 创建多个任务
	for i := 0; i < 3; i++ {
		_, err := m.CreateTask(SyncTaskInput{
			Name:       "任务" + string(rune('A'+i)),
			LocalPath:  "/data/local",
			RemotePath: "/data/remote",
		})
		if err != nil {
			t.Fatalf("创建任务失败: %v", err)
		}
	}

	tasks := m.ListTasks()
	if len(tasks) != 3 {
		t.Errorf("期望3个任务，实际: %d", len(tasks))
	}
}

func TestUpdateTask(t *testing.T) {
	m := newTestManager()

	task, _ := m.CreateTask(SyncTaskInput{
		Name:       "原始名称",
		LocalPath:  "/data/local",
		RemotePath: "/data/remote",
	})

	updated, err := m.UpdateTask(task.ID, SyncTaskInput{
		Name:       "更新后名称",
		LocalPath:  "/data/local2",
		RemotePath: "/data/remote2",
		Direction:  SyncUploadOnly,
	})
	if err != nil {
		t.Fatalf("更新任务失败: %v", err)
	}

	if updated.Name != "更新后名称" {
		t.Errorf("名称未更新，实际: %s", updated.Name)
	}
	if updated.Direction != SyncUploadOnly {
		t.Errorf("方向未更新，实际: %s", updated.Direction)
	}
}

func TestDeleteTask(t *testing.T) {
	m := newTestManager()

	task, _ := m.CreateTask(SyncTaskInput{
		Name:       "待删除任务",
		LocalPath:  "/data/local",
		RemotePath: "/data/remote",
	})

	if err := m.DeleteTask(task.ID); err != nil {
		t.Fatalf("删除任务失败: %v", err)
	}

	// 确认已删除
	_, err := m.GetTask(task.ID)
	if err != ErrSyncTaskNotFound {
		t.Errorf("任务应已被删除")
	}
}

func TestPauseResumeTask(t *testing.T) {
	m := newTestManager()

	task, _ := m.CreateTask(SyncTaskInput{
		Name:       "暂停恢复测试",
		LocalPath:  "/data/local",
		RemotePath: "/data/remote",
	})

	// 暂停
	if err := m.PauseTask(task.ID); err != nil {
		t.Fatalf("暂停任务失败: %v", err)
	}

	got, _ := m.GetTask(task.ID)
	if got.Status != TaskStatusPaused {
		t.Errorf("任务状态应为 paused，实际: %s", got.Status)
	}

	// 恢复
	if err := m.ResumeTask(task.ID); err != nil {
		t.Fatalf("恢复任务失败: %v", err)
	}

	got, _ = m.GetTask(task.ID)
	if got.Status != TaskStatusIdle {
		t.Errorf("任务状态应为 idle，实际: %s", got.Status)
	}
}

// ========== 版本控制测试 ==========

func TestCreateFileVersion(t *testing.T) {
	m := newTestManager()

	// 创建任务
	task, _ := m.CreateTask(SyncTaskInput{
		Name:       "版本测试",
		LocalPath:  "/data/local",
		RemotePath: "/data/remote",
	})
	_ = task

	// 创建版本
	version, err := m.CreateFileVersion("/data/test.txt", 1024, "abc123", "user1")
	if err != nil {
		t.Fatalf("创建版本失败: %v", err)
	}

	if version.VersionNum != 1 {
		t.Errorf("版本号应为1，实际: %d", version.VersionNum)
	}
	if version.Size != 1024 {
		t.Errorf("文件大小不匹配")
	}

	// 创建第二个版本
	version2, err := m.CreateFileVersion("/data/test.txt", 2048, "def456", "user1")
	if err != nil {
		t.Fatalf("创建第二个版本失败: %v", err)
	}
	if version2.VersionNum != 2 {
		t.Errorf("版本号应为2，实际: %d", version2.VersionNum)
	}

	// 获取版本历史
	versions := m.GetFileVersions("/data/test.txt")
	if len(versions) != 2 {
		t.Errorf("应有2个版本，实际: %d", len(versions))
	}
}

func TestRestoreVersion(t *testing.T) {
	m := newTestManager()

	// 创建版本
	v1, _ := m.CreateFileVersion("/data/restore.txt", 100, "hash1", "user1")
	v2, _ := m.CreateFileVersion("/data/restore.txt", 200, "hash2", "user1")

	// 恢复到 v1
	restored, err := m.RestoreVersion("/data/restore.txt", v1.ID)
	if err != nil {
		t.Fatalf("恢复版本失败: %v", err)
	}
	if restored.ID != v1.ID {
		t.Error("恢复的版本ID不匹配")
	}

	// 恢复到 v2
	restored, err = m.RestoreVersion("/data/restore.txt", v2.ID)
	if err != nil {
		t.Fatalf("恢复版本失败: %v", err)
	}
	if restored.ID != v2.ID {
		t.Error("恢复的版本ID不匹配")
	}

	// 恢复不存在的版本
	_, err = m.RestoreVersion("/data/restore.txt", "non-existent")
	if err != ErrFileVersionNotFound {
		t.Errorf("期望 ErrFileVersionNotFound，实际: %v", err)
	}
}

func TestDiffVersions(t *testing.T) {
	m := newTestManager()

	v1, _ := m.CreateFileVersion("/data/diff.txt", 100, "aaa", "user1")
	v2, _ := m.CreateFileVersion("/data/diff.txt", 200, "bbb", "user1")

	diff, err := m.DiffVersions("/data/diff.txt", v1.ID, v2.ID)
	if err != nil {
		t.Fatalf("版本对比失败: %v", err)
	}

	if diff.FromVersion != v1.ID {
		t.Error("源版本ID不匹配")
	}
	if diff.ToVersion != v2.ID {
		t.Error("目标版本ID不匹配")
	}
	if diff.FromChecksum != "aaa" || diff.ToChecksum != "bbb" {
		t.Error("校验和不匹配")
	}
}

// ========== 冲突测试 ==========

func TestConflictResolution(t *testing.T) {
	m := newTestManager()

	// 创建任务
	task, _ := m.CreateTask(SyncTaskInput{
		Name:       "冲突测试",
		LocalPath:  "/data/local",
		RemotePath: "/data/remote",
	})

	// 创建冲突
	conflict := m.CreateConflict(
		task.ID,
		"/data/test.txt",
		"local_hash",
		"remote_hash",
		time.Now().Add(-1*time.Hour),
		time.Now(),
		100,
		200,
		"device1",
		"device2",
	)

	if conflict.Status != ConflictStatusPending {
		t.Errorf("冲突状态应为 pending，实际: %s", conflict.Status)
	}

	// 解决冲突
	if err := m.ResolveConflict(conflict.ID, ConflictKeepBoth, "user1"); err != nil {
		t.Fatalf("解决冲突失败: %v", err)
	}

	// 验证解决状态
	conflicts := m.ListConflicts()
	found := false
	for _, c := range conflicts {
		if c.ID == conflict.ID {
			if c.Status != ConflictStatusResolved {
				t.Errorf("冲突状态应为 resolved，实际: %s", c.Status)
			}
			if c.Resolution != ConflictKeepBoth {
				t.Errorf("解决策略应为 keep_both，实际: %s", c.Resolution)
			}
			found = true
			break
		}
	}
	if !found {
		t.Error("未找到已解决的冲突")
	}
}

// ========== 文件锁测试 ==========

func TestFileLocking(t *testing.T) {
	m := newTestManager()

	// 锁定文件
	lock, err := m.LockFile("/data/locked.txt", FileLockInput{
		LockedBy: "user1",
		LockType: "exclusive",
		Reason:   "编辑中",
		Duration: 30,
	})
	if err != nil {
		t.Fatalf("锁定文件失败: %v", err)
	}

	if lock.LockedBy != "user1" {
		t.Errorf("锁定者不匹配")
	}
	if lock.LockType != "exclusive" {
		t.Errorf("锁类型不匹配")
	}

	// 重复锁定应失败
	_, err = m.LockFile("/data/locked.txt", FileLockInput{
		LockedBy: "user2",
	})
	if err != ErrFileLocked {
		t.Errorf("期望 ErrFileLocked，实际: %v", err)
	}

	// 解锁
	if err := m.UnlockFile("/data/locked.txt", "user1"); err != nil {
		t.Fatalf("解锁失败: %v", err)
	}

	// 非锁定者解锁应失败
	lock2, _ := m.LockFile("/data/locked2.txt", FileLockInput{LockedBy: "user1"})
	if err := m.UnlockFile("/data/locked2.txt", "user2"); err == nil {
		t.Error("非锁定者不应能解锁")
	}

	// 清理
	_ = lock2
}

func TestListLocks(t *testing.T) {
	m := newTestManager()

	// 锁定多个文件
	for i := 0; i < 3; i++ {
		_, _ = m.LockFile("/data/file"+string(rune('A'+i))+".txt", FileLockInput{
			LockedBy: "user1",
			Duration: 30,
		})
	}

	locks := m.ListLocks()
	if len(locks) != 3 {
		t.Errorf("期望3个锁，实际: %d", len(locks))
	}
}

// ========== 协作测试 ==========

func TestComments(t *testing.T) {
	m := newTestManager()

	// 添加评论
	comment := m.AddComment("/data/shared.txt", CommentInput{
		UserID:   "user1",
		UserName: "张三",
		Content:  "这个文件需要修改 @user2",
		Mentions: []string{"user2"},
	})

	if comment.ID == "" {
		t.Error("评论ID不应为空")
	}
	if comment.Content != "这个文件需要修改 @user2" {
		t.Error("评论内容不匹配")
	}

	// 获取评论
	comments := m.GetComments("/data/shared.txt")
	if len(comments) != 1 {
		t.Errorf("期望1条评论，实际: %d", len(comments))
	}
}

func TestActivities(t *testing.T) {
	m := newTestManager()

	// 创建一些操作来产生活动记录
	m.CreateTask(SyncTaskInput{
		Name:       "活动测试",
		LocalPath:  "/data/local",
		RemotePath: "/data/remote",
	})

	m.CreateFileVersion("/data/activity.txt", 100, "hash", "user1")

	m.AddComment("/data/activity.txt", CommentInput{
		UserID:   "user1",
		UserName: "测试用户",
		Content:  "测试评论",
	})

	// 获取活动流
	activities := m.GetActivities(10)
	if len(activities) == 0 {
		t.Error("活动记录不应为空")
	}
}

// ========== 统计测试 ==========

func TestGetStats(t *testing.T) {
	m := newTestManager()

	// 创建任务
	m.CreateTask(SyncTaskInput{
		Name:       "统计测试1",
		LocalPath:  "/data/local1",
		RemotePath: "/data/remote1",
	})
	m.CreateTask(SyncTaskInput{
		Name:       "统计测试2",
		LocalPath:  "/data/local2",
		RemotePath: "/data/remote2",
	})

	stats := m.GetStats()
	if stats.TotalTasks != 2 {
		t.Errorf("期望2个任务，实际: %d", stats.TotalTasks)
	}
	if stats.Uptime <= 0 {
		t.Error("运行时长应大于0")
	}
}

// ========== 增量同步引擎测试 ==========

func TestSyncEngineBlockSize(t *testing.T) {
	engine := NewSyncEngine(0) // 使用默认块大小
	if engine.BlockSize() != defaultBlockSize {
		t.Errorf("默认块大小应为 %d，实际: %d", defaultBlockSize, engine.BlockSize())
	}

	engine2 := NewSyncEngine(128 * 1024)
	if engine2.BlockSize() != 128*1024 {
		t.Errorf("自定义块大小应为 %d，实际: %d", 128*1024, engine2.BlockSize())
	}
}

func TestShouldFullSync(t *testing.T) {
	engine := NewSyncEngine(0)

	// 相同文件不需要同步
	if engine.ShouldFullSync(100, "abc", 100, "abc") {
		t.Error("相同文件不应需要全量同步")
	}

	// 大小差异过大应全量同步
	if !engine.ShouldFullSync(100, "abc", 10000, "def") {
		t.Error("大小差异过大应全量同步")
	}

	// 零大小文件应全量同步
	if !engine.ShouldFullSync(0, "", 100, "abc") {
		t.Error("零大小文件应全量同步")
	}
}

func TestComputeDelta(t *testing.T) {
	engine := NewSyncEngine(4)

	localBlocks := []BlockInfo{
		{Index: 0, Offset: 0, Size: 4, Checksum: "aaa"},
		{Index: 1, Offset: 4, Size: 4, Checksum: "bbb"},
		{Index: 2, Offset: 8, Size: 4, Checksum: "ccc"},
		{Index: 3, Offset: 12, Size: 4, Checksum: "ddd"},
	}

	remoteBlocks := []BlockInfo{
		{Index: 0, Offset: 0, Size: 4, Checksum: "aaa"},  // 匹配
		{Index: 1, Offset: 4, Size: 4, Checksum: "xxx"},  // 不匹配
		{Index: 2, Offset: 8, Size: 4, Checksum: "ccc"},  // 匹配
		{Index: 3, Offset: 12, Size: 4, Checksum: "yyy"}, // 不匹配
	}

	delta := engine.ComputeDelta(localBlocks, remoteBlocks)

	if len(delta.MatchedBlocks) != 2 {
		t.Errorf("应有2个匹配块，实际: %d", len(delta.MatchedBlocks))
	}
	if len(delta.NewBlocks) != 2 {
		t.Errorf("应有2个新块，实际: %d", len(delta.NewBlocks))
	}
	if delta.SavedBytes != 8 {
		t.Errorf("应节省8字节，实际: %d", delta.SavedBytes)
	}
}

// ========== 冲突检测器测试 ==========

func TestConflictDetector(t *testing.T) {
	detector := NewConflictDetector(ConflictNewerWins)

	if detector.GetStrategy() != ConflictNewerWins {
		t.Errorf("策略不匹配")
	}

	detector.SetStrategy(ConflictKeepLocal)
	if detector.GetStrategy() != ConflictKeepLocal {
		t.Errorf("策略更新失败")
	}
}

func TestRenameConflictFile(t *testing.T) {
	result := RenameConflictFile("/data/test.txt")
	if result == "/data/test.txt" {
		t.Error("重命名后的路径不应与原路径相同")
	}
	// 验证包含冲突标记
	if len(result) <= len("/data/test.txt") {
		t.Error("重命名后的路径应更长")
	}
}

// ========== 工具函数测试 ==========

func TestExtractMentions(t *testing.T) {
	tests := []struct {
		content  string
		expected []string
	}{
		{"你好 @user1", []string{"user1"}},
		{"@alice 和 @bob 看一下", []string{"alice", "bob"}},
		{"没有提及", nil},
		{"@a,@b,@c", []string{"a", "b", "c"}},
	}

	for _, tt := range tests {
		result := ExtractMentions(tt.content)
		if len(result) != len(tt.expected) {
			t.Errorf("内容 %q: 期望 %d 个提及，实际 %d", tt.content, len(tt.expected), len(result))
			continue
		}
		for i, m := range result {
			if m != tt.expected[i] {
				t.Errorf("内容 %q: 期望提及 %q，实际 %q", tt.content, tt.expected[i], m)
			}
		}
	}
}
