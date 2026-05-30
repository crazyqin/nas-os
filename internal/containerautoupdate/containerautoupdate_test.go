// Package containerautoupdate 测试
package containerautoupdate

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNewManager(t *testing.T) {
	tmpDir := t.TempDir()
	storagePath := filepath.Join(tmpDir, "test.json")

	m := NewManager(storagePath)
	if m == nil {
		t.Fatal("manager should not be nil")
	}
	if m.storagePath != storagePath {
		t.Errorf("expected storage path %s, got %s", storagePath, m.storagePath)
	}
}

func TestNewManagerEmptyPath(t *testing.T) {
	m := NewManager("")
	if m == nil {
		t.Fatal("manager should not be nil")
	}
}

func TestSetPolicy(t *testing.T) {
	tmpDir := t.TempDir()
	storagePath := filepath.Join(tmpDir, "test.json")
	m := NewManager(storagePath)
	ctx := context.Background()

	policy := UpdatePolicy{
		ContainerID:       "test-container-1",
		ContainerName:     "nginx",
		Enabled:           true,
		Schedule:          "0 3 * * *",
		MaxRetries:        3,
		RollbackOnFailure: true,
		HealthCheckURL:    "http://localhost:8080/health",
		HealthCheckTimeout: 30,
		NotifyOnUpdate:    true,
		NotifyOnFailure:   true,
	}

	result, err := m.SetPolicy(ctx, policy)
	if err != nil {
		t.Fatalf("set policy failed: %v", err)
	}

	if result.ID == "" {
		t.Error("policy should have an ID")
	}
	if result.ContainerID != "test-container-1" {
		t.Errorf("expected container ID test-container-1, got %s", result.ContainerID)
	}
	if !result.Enabled {
		t.Error("policy should be enabled")
	}
	if result.MaxRetries != 3 {
		t.Errorf("expected max retries 3, got %d", result.MaxRetries)
	}
	if result.CreatedAt.IsZero() {
		t.Error("created at should be set")
	}
}

func TestSetPolicyDefaults(t *testing.T) {
	tmpDir := t.TempDir()
	storagePath := filepath.Join(tmpDir, "test.json")
	m := NewManager(storagePath)
	ctx := context.Background()

	policy := UpdatePolicy{
		ContainerID: "test-container-1",
	}

	result, err := m.SetPolicy(ctx, policy)
	if err != nil {
		t.Fatalf("set policy failed: %v", err)
	}

	if result.MaxRetries != 3 {
		t.Errorf("expected default max retries 3, got %d", result.MaxRetries)
	}
	if result.HealthCheckTimeout != 30 {
		t.Errorf("expected default health check timeout 30, got %d", result.HealthCheckTimeout)
	}
}

func TestGetPolicy(t *testing.T) {
	tmpDir := t.TempDir()
	storagePath := filepath.Join(tmpDir, "test.json")
	m := NewManager(storagePath)
	ctx := context.Background()

	// 设置策略
	m.SetPolicy(ctx, UpdatePolicy{
		ContainerID:   "test-container-1",
		ContainerName: "nginx",
		Enabled:       true,
	})

	// 获取策略
	policy, err := m.GetPolicy("test-container-1")
	if err != nil {
		t.Fatalf("get policy failed: %v", err)
	}

	if policy.ContainerID != "test-container-1" {
		t.Errorf("expected container ID test-container-1, got %s", policy.ContainerID)
	}
	if policy.ContainerName != "nginx" {
		t.Errorf("expected container name nginx, got %s", policy.ContainerName)
	}
}

func TestGetPolicyNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	storagePath := filepath.Join(tmpDir, "test.json")
	m := NewManager(storagePath)

	_, err := m.GetPolicy("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent policy")
	}
}

func TestCheckUpdates(t *testing.T) {
	tmpDir := t.TempDir()
	storagePath := filepath.Join(tmpDir, "test.json")
	m := NewManager(storagePath)
	ctx := context.Background()

	// 设置策略
	m.SetPolicy(ctx, UpdatePolicy{
		ContainerID: "test-container-1",
		Enabled:     true,
	})

	// 检查更新
	check, err := m.CheckUpdates(ctx, "test-container-1")
	if err != nil {
		t.Fatalf("check updates failed: %v", err)
	}

	if check == nil {
		t.Fatal("check should not be nil")
	}
	if check.ContainerID != "test-container-1" {
		t.Errorf("expected container ID test-container-1, got %s", check.ContainerID)
	}
	if check.CheckedAt.IsZero() {
		t.Error("checked at should be set")
	}
}

func TestCheckUpdatesNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	storagePath := filepath.Join(tmpDir, "test.json")
	m := NewManager(storagePath)
	ctx := context.Background()

	_, err := m.CheckUpdates(ctx, "nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent container")
	}
}

func TestCheckAllUpdates(t *testing.T) {
	tmpDir := t.TempDir()
	storagePath := filepath.Join(tmpDir, "test.json")
	m := NewManager(storagePath)
	ctx := context.Background()

	// 设置多个策略
	m.SetPolicy(ctx, UpdatePolicy{
		ContainerID: "container-1",
		Enabled:     true,
	})
	m.SetPolicy(ctx, UpdatePolicy{
		ContainerID: "container-2",
		Enabled:     true,
	})
	m.SetPolicy(ctx, UpdatePolicy{
		ContainerID: "container-3",
		Enabled:     false, // 禁用的容器不应被检查
	})

	checks := m.CheckAllUpdates(ctx)
	if len(checks) != 2 {
		t.Errorf("expected 2 checks, got %d", len(checks))
	}
}

func TestApplyUpdate(t *testing.T) {
	tmpDir := t.TempDir()
	storagePath := filepath.Join(tmpDir, "test.json")
	m := NewManager(storagePath)
	ctx := context.Background()

	// 设置策略（禁用健康检查以简化测试）
	m.SetPolicy(ctx, UpdatePolicy{
		ContainerID:   "test-container-1",
		ContainerName: "nginx",
		Enabled:       true,
	})

	// 执行更新
	record, err := m.ApplyUpdate(ctx, "test-container-1")
	if err != nil {
		t.Fatalf("apply update failed: %v", err)
	}

	if record == nil {
		t.Fatal("record should not be nil")
	}
	if record.ID == "" {
		t.Error("record should have an ID")
	}
	if record.ContainerID != "test-container-1" {
		t.Errorf("expected container ID test-container-1, got %s", record.ContainerID)
	}
	if record.Status != StatusPending {
		t.Errorf("expected status pending, got %s", record.Status)
	}
	if record.StartedAt.IsZero() {
		t.Error("started at should be set")
	}

	// 等待更新完成
	time.Sleep(500 * time.Millisecond)

	// 检查更新状态
	history := m.GetUpdateHistory(ctx, "test-container-1")
	if len(history) == 0 {
		t.Fatal("history should not be empty")
	}

	lastRecord := history[len(history)-1]
	if lastRecord.Status != StatusSuccess && lastRecord.Status != StatusFailed && lastRecord.Status != StatusRolledBack {
		t.Errorf("expected final status, got %s", lastRecord.Status)
	}
}

func TestApplyUpdateNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	storagePath := filepath.Join(tmpDir, "test.json")
	m := NewManager(storagePath)
	ctx := context.Background()

	_, err := m.ApplyUpdate(ctx, "nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent container")
	}
}

func TestRollback(t *testing.T) {
	tmpDir := t.TempDir()
	storagePath := filepath.Join(tmpDir, "test.json")
	m := NewManager(storagePath)
	ctx := context.Background()

	// 设置策略
	m.SetPolicy(ctx, UpdatePolicy{
		ContainerID:   "test-container-1",
		ContainerName: "nginx",
		Enabled:       true,
	})

	// 执行更新
	record, err := m.ApplyUpdate(ctx, "test-container-1")
	if err != nil {
		t.Fatalf("apply update failed: %v", err)
	}

	// 等待更新完成
	time.Sleep(500 * time.Millisecond)

	// 执行回滚
	err = m.Rollback(ctx, record.ID)
	if err != nil {
		t.Fatalf("rollback failed: %v", err)
	}

	// 验证回滚状态
	history := m.GetUpdateHistory(ctx, "test-container-1")
	found := false
	for _, r := range history {
		if r.ID == record.ID && r.Status == StatusRolledBack {
			found = true
			break
		}
	}
	if !found {
		t.Error("rollback record not found in history")
	}
}

func TestRollbackNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	storagePath := filepath.Join(tmpDir, "test.json")
	m := NewManager(storagePath)
	ctx := context.Background()

	err := m.Rollback(ctx, "nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent record")
	}
}

func TestGetHealth(t *testing.T) {
	tmpDir := t.TempDir()
	storagePath := filepath.Join(tmpDir, "test.json")
	m := NewManager(storagePath)
	ctx := context.Background()

	// 获取未知容器的健康状态
	health, err := m.GetHealth(ctx, "test-container-1")
	if err != nil {
		t.Fatalf("get health failed: %v", err)
	}

	if health.ContainerID != "test-container-1" {
		t.Errorf("expected container ID test-container-1, got %s", health.ContainerID)
	}
	if health.Status != HealthUnknown {
		t.Errorf("expected status unknown, got %s", health.Status)
	}
}

func TestGetUpdateHistory(t *testing.T) {
	tmpDir := t.TempDir()
	storagePath := filepath.Join(tmpDir, "test.json")
	m := NewManager(storagePath)
	ctx := context.Background()

	// 设置策略
	m.SetPolicy(ctx, UpdatePolicy{
		ContainerID:   "test-container-1",
		ContainerName: "nginx",
		Enabled:       true,
	})

	// 执行多次更新
	m.ApplyUpdate(ctx, "test-container-1")
	time.Sleep(500 * time.Millisecond)
	m.ApplyUpdate(ctx, "test-container-1")
	time.Sleep(500 * time.Millisecond)

	history := m.GetUpdateHistory(ctx, "test-container-1")
	if len(history) < 2 {
		t.Errorf("expected at least 2 history items, got %d", len(history))
	}
}

func TestGetStats(t *testing.T) {
	tmpDir := t.TempDir()
	storagePath := filepath.Join(tmpDir, "test.json")
	m := NewManager(storagePath)
	ctx := context.Background()

	// 设置策略
	m.SetPolicy(ctx, UpdatePolicy{
		ContainerID:   "test-container-1",
		ContainerName: "nginx",
		Enabled:       true,
	})

	// 执行更新
	m.ApplyUpdate(ctx, "test-container-1")
	time.Sleep(500 * time.Millisecond)

	stats := m.GetStats(ctx)
	if stats.TotalUpdates == 0 {
		t.Error("expected total updates > 0")
	}
	if stats.LastUpdateTime.IsZero() {
		t.Error("last update time should be set")
	}
}

func TestGetStatsEmpty(t *testing.T) {
	tmpDir := t.TempDir()
	storagePath := filepath.Join(tmpDir, "test.json")
	m := NewManager(storagePath)
	ctx := context.Background()

	stats := m.GetStats(ctx)
	if stats.TotalUpdates != 0 {
		t.Errorf("expected total updates 0, got %d", stats.TotalUpdates)
	}
}

func TestRunScheduledChecks(t *testing.T) {
	tmpDir := t.TempDir()
	storagePath := filepath.Join(tmpDir, "test.json")
	m := NewManager(storagePath)
	ctx := context.Background()

	// 设置策略
	m.SetPolicy(ctx, UpdatePolicy{
		ContainerID:   "test-container-1",
		ContainerName: "nginx",
		Enabled:       true,
	})

	// 执行定时检查
	err := m.RunScheduledChecks(ctx)
	if err != nil {
		t.Fatalf("run scheduled checks failed: %v", err)
	}

	// 等待更新完成
	time.Sleep(1 * time.Second)
}

func TestNotifyUpdate(t *testing.T) {
	tmpDir := t.TempDir()
	storagePath := filepath.Join(tmpDir, "test.json")
	m := NewManager(storagePath)
	ctx := context.Background()

	err := m.NotifyUpdate(ctx, "test-container-1", "sha256:newdigest")
	if err != nil {
		t.Fatalf("notify update failed: %v", err)
	}
}

func TestPersistence(t *testing.T) {
	tmpDir := t.TempDir()
	storagePath := filepath.Join(tmpDir, "test.json")

	// 创建管理器并添加数据
	m1 := NewManager(storagePath)
	ctx := context.Background()

	m1.SetPolicy(ctx, UpdatePolicy{
		ContainerID:   "test-container-1",
		ContainerName: "nginx",
		Enabled:       true,
		Schedule:      "0 3 * * *",
	})

	// 创建新管理器，验证数据已持久化
	m2 := NewManager(storagePath)

	policy, err := m2.GetPolicy("test-container-1")
	if err != nil {
		t.Fatalf("get policy failed: %v", err)
	}

	if policy.ContainerName != "nginx" {
		t.Errorf("expected container name nginx, got %s", policy.ContainerName)
	}
	if policy.Schedule != "0 3 * * *" {
		t.Errorf("expected schedule 0 3 * * *, got %s", policy.Schedule)
	}
}

func TestStatusConstants(t *testing.T) {
	// 验证状态常量
	if StatusPending != "pending" {
		t.Errorf("expected StatusPending to be 'pending', got %s", StatusPending)
	}
	if StatusDownloading != "downloading" {
		t.Errorf("expected StatusDownloading to be 'downloading', got %s", StatusDownloading)
	}
	if StatusSuccess != "success" {
		t.Errorf("expected StatusSuccess to be 'success', got %s", StatusSuccess)
	}
	if StatusFailed != "failed" {
		t.Errorf("expected StatusFailed to be 'failed', got %s", StatusFailed)
	}
	if StatusRolledBack != "rolled_back" {
		t.Errorf("expected StatusRolledBack to be 'rolled_back', got %s", StatusRolledBack)
	}
}

func TestHealthStatusConstants(t *testing.T) {
	if HealthHealthy != "healthy" {
		t.Errorf("expected HealthHealthy to be 'healthy', got %s", HealthHealthy)
	}
	if HealthUnhealthy != "unhealthy" {
		t.Errorf("expected HealthUnhealthy to be 'unhealthy', got %s", HealthUnhealthy)
	}
	if HealthStarting != "starting" {
		t.Errorf("expected HealthStarting to be 'starting', got %s", HealthStarting)
	}
	if HealthUnknown != "unknown" {
		t.Errorf("expected HealthUnknown to be 'unknown', got %s", HealthUnknown)
	}
}

func TestMain(m *testing.M) {
	os.Exit(m.Run())
}
