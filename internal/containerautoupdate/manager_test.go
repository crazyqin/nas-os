// Package containerautoupdate 测试
package containerautoupdate

import (
	"testing"
	"time"
)

func TestNewManager(t *testing.T) {
	m := NewManager()
	if m == nil {
		t.Fatal("manager should not be nil")
	}
}

func TestAddContainer(t *testing.T) {
	m := NewManager()

	container := m.AddContainer(AddContainerRequest{
		Name:  "test-container",
		Image: "nginx",
		Tag:   "1.21",
		Policy: UpdatePolicy{
			Schedule:   "0 3 * * *",
			AutoUpdate: true,
			MaxRetries: 3,
		},
		Rollback: RollbackConfig{
			Enabled:      true,
			AutoRollback: true,
			MaxHistory:   5,
		},
		HealthCheck: HealthCheckConfig{
			Enabled:        true,
			URL:            "http://localhost:8080/health",
			Interval:       10 * time.Second,
			Timeout:        5 * time.Second,
			Retries:        3,
			ExpectedStatus: 200,
		},
	})

	if container == nil {
		t.Fatal("container should not be nil")
	}
	if container.ID == "" {
		t.Error("container should have an ID")
	}
	if container.Name != "test-container" {
		t.Errorf("expected name test-container, got %s", container.Name)
	}
	if container.Image != "nginx" {
		t.Errorf("expected image nginx, got %s", container.Image)
	}
	if container.Tag != "1.21" {
		t.Errorf("expected tag 1.21, got %s", container.Tag)
	}
	if !container.Enabled {
		t.Error("container should be enabled")
	}
}

func TestAddContainerDefaults(t *testing.T) {
	m := NewManager()

	container := m.AddContainer(AddContainerRequest{
		Name:  "test-container",
		Image: "nginx",
	})

	if container.Tag != "latest" {
		t.Errorf("expected default tag latest, got %s", container.Tag)
	}
	if container.Policy.MaxRetries != 3 {
		t.Errorf("expected default max retries 3, got %d", container.Policy.MaxRetries)
	}
	if container.Rollback.MaxHistory != 5 {
		t.Errorf("expected default max history 5, got %d", container.Rollback.MaxHistory)
	}
	if container.Rollback.RollbackTimeout != 5*time.Minute {
		t.Errorf("expected default rollback timeout 5m, got %v", container.Rollback.RollbackTimeout)
	}
	if container.HealthCheck.Interval != 10*time.Second {
		t.Errorf("expected default health check interval 10s, got %v", container.HealthCheck.Interval)
	}
	if container.HealthCheck.ExpectedStatus != 200 {
		t.Errorf("expected default expected status 200, got %d", container.HealthCheck.ExpectedStatus)
	}
}

func TestRemoveContainer(t *testing.T) {
	m := NewManager()

	container := m.AddContainer(AddContainerRequest{
		Name:  "to-remove",
		Image: "nginx",
	})

	err := m.RemoveContainer(container.ID)
	if err != nil {
		t.Fatalf("remove container failed: %v", err)
	}

	// 确认已删除
	_, err = m.GetContainer(container.ID)
	if err == nil {
		t.Error("expected error for removed container")
	}
}

func TestRemoveContainerNotFound(t *testing.T) {
	m := NewManager()

	err := m.RemoveContainer("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent container")
	}
}

func TestGetContainer(t *testing.T) {
	m := NewManager()

	original := m.AddContainer(AddContainerRequest{
		Name:  "test-container",
		Image: "nginx",
		Tag:   "1.21",
	})

	container, err := m.GetContainer(original.ID)
	if err != nil {
		t.Fatalf("get container failed: %v", err)
	}
	if container.ID != original.ID {
		t.Errorf("expected ID %s, got %s", original.ID, container.ID)
	}
	if container.Name != original.Name {
		t.Errorf("expected name %s, got %s", original.Name, container.Name)
	}
}

func TestGetContainerNotFound(t *testing.T) {
	m := NewManager()

	_, err := m.GetContainer("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent container")
	}
}

func TestListContainers(t *testing.T) {
	m := NewManager()

	m.AddContainer(AddContainerRequest{Name: "c1", Image: "nginx"})
	m.AddContainer(AddContainerRequest{Name: "c2", Image: "redis"})
	m.AddContainer(AddContainerRequest{Name: "c3", Image: "postgres"})

	containers := m.ListContainers()
	if len(containers) != 3 {
		t.Errorf("expected 3 containers, got %d", len(containers))
	}
}

func TestUpdateContainer(t *testing.T) {
	m := NewManager()

	container := m.AddContainer(AddContainerRequest{
		Name:  "test-container",
		Image: "nginx",
		Tag:   "1.21",
	})

	updated, err := m.UpdateContainer(container.ID, AddContainerRequest{
		Name:  "updated-container",
		Image: "nginx",
		Tag:   "1.22",
		Policy: UpdatePolicy{
			Schedule:   "0 4 * * *",
			AutoUpdate: false,
		},
	})

	if err != nil {
		t.Fatalf("update container failed: %v", err)
	}
	if updated.Name != "updated-container" {
		t.Errorf("expected name updated-container, got %s", updated.Name)
	}
	if updated.Tag != "1.22" {
		t.Errorf("expected tag 1.22, got %s", updated.Tag)
	}
	if updated.Policy.AutoUpdate {
		t.Error("auto update should be false")
	}
}

func TestUpdateContainerNotFound(t *testing.T) {
	m := NewManager()

	_, err := m.UpdateContainer("nonexistent", AddContainerRequest{
		Name:  "test",
		Image: "nginx",
	})
	if err == nil {
		t.Error("expected error for nonexistent container")
	}
}

func TestCheckUpdates(t *testing.T) {
	m := NewManager()

	m.AddContainer(AddContainerRequest{
		Name:  "c1",
		Image: "nginx",
		Tag:   "1.21",
	})
	m.AddContainer(AddContainerRequest{
		Name:  "c2",
		Image: "redis",
		Tag:   "6.0",
	})

	updates, err := m.CheckUpdates()
	if err != nil {
		t.Fatalf("check updates failed: %v", err)
	}
	if len(updates) != 2 {
		t.Errorf("expected 2 updates, got %d", len(updates))
	}

	for _, update := range updates {
		if update.ID == "" {
			t.Error("update should have an ID")
		}
		if update.Status != StatusPending {
			t.Errorf("expected status pending, got %s", update.Status)
		}
		if update.StartedAt.IsZero() {
			t.Error("started at should be set")
		}
	}
}

func TestCheckContainerUpdate(t *testing.T) {
	m := NewManager()

	container := m.AddContainer(AddContainerRequest{
		Name:  "test-container",
		Image: "nginx",
		Tag:   "1.21",
	})

	update, err := m.CheckContainerUpdate(container.ID)
	if err != nil {
		t.Fatalf("check container update failed: %v", err)
	}
	if update == nil {
		t.Fatal("update should not be nil")
	}
	if update.ContainerID != container.ID {
		t.Errorf("expected container ID %s, got %s", container.ID, update.ContainerID)
	}
	if update.OldTag != "1.21" {
		t.Errorf("expected old tag 1.21, got %s", update.OldTag)
	}
}

func TestCheckContainerUpdateNotFound(t *testing.T) {
	m := NewManager()

	_, err := m.CheckContainerUpdate("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent container")
	}
}

func TestApplyUpdate(t *testing.T) {
	m := NewManager()

	container := m.AddContainer(AddContainerRequest{
		Name:  "test-container",
		Image: "nginx",
		Tag:   "1.21",
		Policy: UpdatePolicy{
			MaxRetries: 3,
		},
		Rollback: RollbackConfig{
			Enabled:      true,
			AutoRollback: false,
			MaxHistory:   5,
		},
		HealthCheck: HealthCheckConfig{
			Enabled: false,
		},
	})

	update, err := m.ApplyUpdate(container.ID, "1.22")
	if err != nil {
		t.Fatalf("apply update failed: %v", err)
	}
	if update == nil {
		t.Fatal("update should not be nil")
	}
	if update.Status != StatusCompleted {
		t.Errorf("expected status completed, got %s", update.Status)
	}
	if update.NewTag != "1.22" {
		t.Errorf("expected new tag 1.22, got %s", update.NewTag)
	}
	if update.CompletedAt == nil {
		t.Error("completed at should be set")
	}
	if update.Duration <= 0 {
		t.Error("duration should be positive")
	}

	// 验证容器配置已更新
	updatedContainer, _ := m.GetContainer(container.ID)
	if updatedContainer.Tag != "1.22" {
		t.Errorf("expected container tag 1.22, got %s", updatedContainer.Tag)
	}
}

func TestApplyUpdateDefaultTag(t *testing.T) {
	m := NewManager()

	container := m.AddContainer(AddContainerRequest{
		Name:  "test-container",
		Image: "nginx",
		Tag:   "1.21",
		Rollback: RollbackConfig{
			Enabled: false,
		},
		HealthCheck: HealthCheckConfig{
			Enabled: false,
		},
	})

	update, err := m.ApplyUpdate(container.ID, "")
	if err != nil {
		t.Fatalf("apply update failed: %v", err)
	}
	if update.NewTag != "latest" {
		t.Errorf("expected new tag latest, got %s", update.NewTag)
	}
}

func TestApplyUpdateNotFound(t *testing.T) {
	m := NewManager()

	_, err := m.ApplyUpdate("nonexistent", "1.22")
	if err == nil {
		t.Error("expected error for nonexistent container")
	}
}

func TestRollback(t *testing.T) {
	m := NewManager()

	container := m.AddContainer(AddContainerRequest{
		Name:  "test-container",
		Image: "nginx",
		Tag:   "1.21",
		Rollback: RollbackConfig{
			Enabled:      true,
			AutoRollback: false,
			MaxHistory:   5,
		},
		HealthCheck: HealthCheckConfig{
			Enabled: false,
		},
	})

	// 先应用更新
	_, err := m.ApplyUpdate(container.ID, "1.22")
	if err != nil {
		t.Fatalf("apply update failed: %v", err)
	}

	// 回滚
	rollbackUpdate, err := m.Rollback(container.ID, "")
	if err != nil {
		t.Fatalf("rollback failed: %v", err)
	}
	if rollbackUpdate == nil {
		t.Fatal("rollback update should not be nil")
	}
	if rollbackUpdate.Status != StatusRolledBack {
		t.Errorf("expected status rolled_back, got %s", rollbackUpdate.Status)
	}
	if rollbackUpdate.NewTag != "1.21" {
		t.Errorf("expected rollback tag 1.21, got %s", rollbackUpdate.NewTag)
	}

	// 验证容器配置已回滚
	rolledBackContainer, _ := m.GetContainer(container.ID)
	if rolledBackContainer.Tag != "1.21" {
		t.Errorf("expected container tag 1.21 after rollback, got %s", rolledBackContainer.Tag)
	}
}

func TestRollbackWithUpdateID(t *testing.T) {
	m := NewManager()

	container := m.AddContainer(AddContainerRequest{
		Name:  "test-container",
		Image: "nginx",
		Tag:   "1.21",
		Rollback: RollbackConfig{
			Enabled: true,
		},
		HealthCheck: HealthCheckConfig{
			Enabled: false,
		},
	})

	// 应用更新
	update, _ := m.ApplyUpdate(container.ID, "1.22")

	// 使用更新 ID 回滚
	rollbackUpdate, err := m.Rollback(container.ID, update.ID)
	if err != nil {
		t.Fatalf("rollback failed: %v", err)
	}
	if rollbackUpdate.Status != StatusRolledBack {
		t.Errorf("expected status rolled_back, got %s", rollbackUpdate.Status)
	}
}

func TestRollbackContainerNotFound(t *testing.T) {
	m := NewManager()

	_, err := m.Rollback("nonexistent", "")
	if err == nil {
		t.Error("expected error for nonexistent container")
	}
}

func TestRollbackNoCompletedUpdate(t *testing.T) {
	m := NewManager()

	container := m.AddContainer(AddContainerRequest{
		Name:  "test-container",
		Image: "nginx",
		Tag:   "1.21",
		Rollback: RollbackConfig{
			Enabled: true,
		},
	})

	// 没有应用过更新，直接回滚
	_, err := m.Rollback(container.ID, "")
	if err == nil {
		t.Error("expected error when no completed update exists")
	}
}

func TestHealthCheck(t *testing.T) {
	m := NewManager()

	container := m.AddContainer(AddContainerRequest{
		Name:  "test-container",
		Image: "nginx",
		HealthCheck: HealthCheckConfig{
			Enabled:        true,
			URL:            "http://localhost:8080/health",
			Interval:       10 * time.Millisecond,
			Timeout:        5 * time.Second,
			Retries:        3,
			ExpectedStatus: 200,
		},
	})

	healthy, err := m.HealthCheck(container.ID)
	if err != nil {
		t.Fatalf("health check failed: %v", err)
	}
	if !healthy {
		t.Error("container should be healthy")
	}
}

func TestHealthCheckDisabled(t *testing.T) {
	m := NewManager()

	container := m.AddContainer(AddContainerRequest{
		Name:  "test-container",
		Image: "nginx",
		HealthCheck: HealthCheckConfig{
			Enabled: false,
		},
	})

	healthy, err := m.HealthCheck(container.ID)
	if err != nil {
		t.Fatalf("health check failed: %v", err)
	}
	if !healthy {
		t.Error("container should be healthy when health check is disabled")
	}
}

func TestHealthCheckNotFound(t *testing.T) {
	m := NewManager()

	_, err := m.HealthCheck("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent container")
	}
}

func TestGetUpdateHistory(t *testing.T) {
	m := NewManager()

	container := m.AddContainer(AddContainerRequest{
		Name:  "test-container",
		Image: "nginx",
		Tag:   "1.21",
		Rollback: RollbackConfig{
			Enabled: false,
		},
		HealthCheck: HealthCheckConfig{
			Enabled: false,
		},
	})

	// 应用多次更新
	m.ApplyUpdate(container.ID, "1.22")
	m.ApplyUpdate(container.ID, "1.23")

	history := m.GetUpdateHistory(10)
	if len(history) != 2 {
		t.Errorf("expected 2 history items, got %d", len(history))
	}

	// 限制数量
	limited := m.GetUpdateHistory(1)
	if len(limited) != 1 {
		t.Errorf("expected 1 history item, got %d", len(limited))
	}
}

func TestGetContainerHistory(t *testing.T) {
	m := NewManager()

	c1 := m.AddContainer(AddContainerRequest{
		Name:  "c1",
		Image: "nginx",
		Tag:   "1.21",
		Rollback: RollbackConfig{
			Enabled: false,
		},
		HealthCheck: HealthCheckConfig{
			Enabled: false,
		},
	})
	c2 := m.AddContainer(AddContainerRequest{
		Name:  "c2",
		Image: "redis",
		Tag:   "6.0",
		Rollback: RollbackConfig{
			Enabled: false,
		},
		HealthCheck: HealthCheckConfig{
			Enabled: false,
		},
	})

	m.ApplyUpdate(c1.ID, "1.22")
	m.ApplyUpdate(c2.ID, "6.1")
	m.ApplyUpdate(c1.ID, "1.23")

	c1History := m.GetContainerHistory(c1.ID, 10)
	if len(c1History) != 2 {
		t.Errorf("expected 2 history items for c1, got %d", len(c1History))
	}

	c2History := m.GetContainerHistory(c2.ID, 10)
	if len(c2History) != 1 {
		t.Errorf("expected 1 history item for c2, got %d", len(c2History))
	}
}

func TestGetStats(t *testing.T) {
	m := NewManager()

	container := m.AddContainer(AddContainerRequest{
		Name:  "test-container",
		Image: "nginx",
		Tag:   "1.21",
		Rollback: RollbackConfig{
			Enabled: false,
		},
		HealthCheck: HealthCheckConfig{
			Enabled: false,
		},
	})

	// 成功的更新
	m.ApplyUpdate(container.ID, "1.22")
	m.ApplyUpdate(container.ID, "1.23")

	stats := m.GetStats()
	if stats == nil {
		t.Fatal("stats should not be nil")
	}
	if stats.TotalUpdates != 2 {
		t.Errorf("expected total updates 2, got %d", stats.TotalUpdates)
	}
	if stats.SuccessCount != 2 {
		t.Errorf("expected success count 2, got %d", stats.SuccessCount)
	}
	if stats.FailedCount != 0 {
		t.Errorf("expected failed count 0, got %d", stats.FailedCount)
	}
	if stats.RollbackCount != 0 {
		t.Errorf("expected rollback count 0, got %d", stats.RollbackCount)
	}
	if stats.LastUpdateTime.IsZero() {
		t.Error("last update time should be set")
	}
}

func TestGetStatsEmpty(t *testing.T) {
	m := NewManager()

	stats := m.GetStats()
	if stats.TotalUpdates != 0 {
		t.Errorf("expected total updates 0, got %d", stats.TotalUpdates)
	}
}

func TestClearHistory(t *testing.T) {
	m := NewManager()

	container := m.AddContainer(AddContainerRequest{
		Name:  "test-container",
		Image: "nginx",
		Tag:   "1.21",
		Rollback: RollbackConfig{
			Enabled: false,
		},
		HealthCheck: HealthCheckConfig{
			Enabled: false,
		},
	})

	m.ApplyUpdate(container.ID, "1.22")
	m.ApplyUpdate(container.ID, "1.23")

	if len(m.GetUpdateHistory(0)) != 2 {
		t.Error("should have 2 history items before clear")
	}

	m.ClearHistory()

	if len(m.GetUpdateHistory(0)) != 0 {
		t.Error("should have 0 history items after clear")
	}

	stats := m.GetStats()
	if stats.TotalUpdates != 0 {
		t.Error("total updates should be 0 after clear")
	}
}

func TestUpdateStatusConstants(t *testing.T) {
	// 验证状态常量
	if StatusPending != "pending" {
		t.Errorf("expected StatusPending to be 'pending', got %s", StatusPending)
	}
	if StatusCompleted != "completed" {
		t.Errorf("expected StatusCompleted to be 'completed', got %s", StatusCompleted)
	}
	if StatusFailed != "failed" {
		t.Errorf("expected StatusFailed to be 'failed', got %s", StatusFailed)
	}
	if StatusRolledBack != "rolled_back" {
		t.Errorf("expected StatusRolledBack to be 'rolled_back', got %s", StatusRolledBack)
	}
}
