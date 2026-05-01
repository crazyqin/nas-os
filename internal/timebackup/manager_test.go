// Package timebackup 测试
package timebackup

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	"go.uber.org/zap"

	_ "modernc.org/sqlite"
)

func newTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("打开内存数据库失败: %v", err)
	}
	// 创建必要的表
	schema := `
	CREATE TABLE IF NOT EXISTS backup_tasks (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		source_path TEXT NOT NULL,
		strategy TEXT DEFAULT 'full',
		schedule TEXT,
		retention_mode TEXT,
		retention_max_count INTEGER,
		retention_max_age_days INTEGER,
		retention_max_size_gb REAL,
		status TEXT DEFAULT 'idle',
		enabled INTEGER DEFAULT 1,
		last_run TEXT,
		last_error TEXT,
		snapshot_count INTEGER DEFAULT 0,
		created_at TEXT,
		updated_at TEXT
	);
	CREATE TABLE IF NOT EXISTS snapshots (
		id TEXT PRIMARY KEY,
		task_id TEXT NOT NULL,
		source_path TEXT,
		snapshot_dir TEXT,
		size INTEGER DEFAULT 0,
		file_count INTEGER DEFAULT 0,
		strategy TEXT DEFAULT 'full',
		created_at TEXT
	);
	CREATE TABLE IF NOT EXISTS snapshot_files (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		snapshot_id TEXT NOT NULL,
		relative_path TEXT NOT NULL,
		size INTEGER DEFAULT 0,
		modified_at TEXT,
		checksum TEXT
	);
	`
	if _, err := db.Exec(schema); err != nil {
		t.Fatalf("创建表失败: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func newTestManager(t *testing.T) *Manager {
	t.Helper()
	tmpDir := t.TempDir()
	snapshotDir := filepath.Join(tmpDir, "snapshots")
	os.MkdirAll(snapshotDir, 0755)

	logger := zap.NewNop()
	db := newTestDB(t)
	store := NewStore(db, logger)
	return NewManager(snapshotDir, store, logger)
}

func TestNewManager(t *testing.T) {
	mgr := newTestManager(t)
	if mgr == nil {
		t.Fatal("管理器不应为nil")
	}
}

func TestCreateTask(t *testing.T) {
	mgr := newTestManager(t)
	tmpDir := t.TempDir()

	retention := &RetentionPolicy{
		Mode:     RetentionByCount,
		MaxCount: 10,
	}
	task, err := mgr.CreateTask(&CreateTaskRequest{
		Name:       "每日备份",
		SourcePath: tmpDir,
		Schedule:   "0 2 * * *",
		Retention:  retention,
	})
	if err != nil {
		t.Fatalf("创建任务失败: %v", err)
	}
	if task.Name != "每日备份" {
		t.Errorf("名称不匹配: %s", task.Name)
	}
}

func TestListTasks(t *testing.T) {
	mgr := newTestManager(t)
	tmpDir := t.TempDir()

	mgr.CreateTask(&CreateTaskRequest{Name: "task1", SourcePath: tmpDir})
	mgr.CreateTask(&CreateTaskRequest{Name: "task2", SourcePath: tmpDir})

	tasks := mgr.ListTasks()
	if len(tasks) != 2 {
		t.Errorf("期望2个任务，实际 %d", len(tasks))
	}
}

func TestGetTask(t *testing.T) {
	mgr := newTestManager(t)
	tmpDir := t.TempDir()

	task, _ := mgr.CreateTask(&CreateTaskRequest{Name: "test", SourcePath: tmpDir})

	got, err := mgr.GetTask(task.ID)
	if err != nil {
		t.Fatalf("获取任务失败: %v", err)
	}
	if got.ID != task.ID {
		t.Errorf("ID不匹配")
	}
}

func TestDeleteTask(t *testing.T) {
	mgr := newTestManager(t)
	tmpDir := t.TempDir()

	task, _ := mgr.CreateTask(&CreateTaskRequest{Name: "to-delete", SourcePath: tmpDir})
	err := mgr.DeleteTask(task.ID)
	if err != nil {
		t.Fatalf("删除任务失败: %v", err)
	}

	_, err = mgr.GetTask(task.ID)
	if err == nil {
		t.Error("已删除任务不应存在")
	}
}

func TestCreateSnapshot(t *testing.T) {
	mgr := newTestManager(t)
	tmpDir := t.TempDir()

	// 创建测试文件
	os.WriteFile(filepath.Join(tmpDir, "test.txt"), []byte("hello world"), 0644)

	task, _ := mgr.CreateTask(&CreateTaskRequest{Name: "snap-test", SourcePath: tmpDir})

	snap, err := mgr.CreateSnapshot(task.ID)
	if err != nil {
		t.Fatalf("创建快照失败: %v", err)
	}
	if snap.TaskID != task.ID {
		t.Errorf("任务ID不匹配")
	}
}

func TestGetTaskNotFound(t *testing.T) {
	mgr := newTestManager(t)

	_, err := mgr.GetTask("nonexistent")
	if err == nil {
		t.Error("不存在的任务应返回错误")
	}
}

func TestGetSnapshotsByTask(t *testing.T) {
	mgr := newTestManager(t)
	tmpDir := t.TempDir()

	os.WriteFile(filepath.Join(tmpDir, "a.txt"), []byte("a"), 0644)

	task, _ := mgr.CreateTask(&CreateTaskRequest{Name: "multi-snap", SourcePath: tmpDir})
	mgr.CreateSnapshot(task.ID)

	snaps := mgr.GetSnapshotsByTask(task.ID)
	if len(snaps) < 1 {
		t.Errorf("应有至少1个快照")
	}
}

func TestDeleteSnapshot(t *testing.T) {
	mgr := newTestManager(t)
	tmpDir := t.TempDir()

	os.WriteFile(filepath.Join(tmpDir, "test.txt"), []byte("data"), 0644)

	task, _ := mgr.CreateTask(&CreateTaskRequest{Name: "del-snap", SourcePath: tmpDir})
	snap, _ := mgr.CreateSnapshot(task.ID)

	err := mgr.DeleteSnapshot(snap.ID)
	if err != nil {
		t.Fatalf("删除快照失败: %v", err)
	}
}

func TestRetentionModes(t *testing.T) {
	modes := []RetentionMode{RetentionByCount, RetentionByTime, RetentionBySpace}
	for _, m := range modes {
		if m == "" {
			t.Error("保留模式常量不应为空")
		}
	}
}
