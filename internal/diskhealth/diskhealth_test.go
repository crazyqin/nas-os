// Package diskhealth 测试文件
package diskhealth

import (
	"context"
	"testing"
)

func TestManager_ListPools(t *testing.T) {
	m := NewManager()
	ctx := context.Background()

	pools, err := m.ListPools(ctx)
	if err != nil {
		t.Fatalf("ListPools failed: %v", err)
	}

	if len(pools) != 0 {
		t.Errorf("Expected 0 pools, got %d", len(pools))
	}
}

func TestManager_ListAlerts(t *testing.T) {
	m := NewManager()
	ctx := context.Background()

	alerts, err := m.ListAlerts(ctx, "", false)
	if err != nil {
		t.Fatalf("ListAlerts failed: %v", err)
	}

	// 默认有告警规则，但没有实际告警
	if len(alerts) != 0 {
		t.Errorf("Expected 0 alerts, got %d", len(alerts))
	}
}

func TestManager_GetStats(t *testing.T) {
	m := NewManager()
	ctx := context.Background()

	stats, err := m.GetStats(ctx)
	if err != nil {
		t.Fatalf("GetStats failed: %v", err)
	}

	if stats["total_disks"] != 0 {
		t.Errorf("Expected 0 disks, got %v", stats["total_disks"])
	}
}
