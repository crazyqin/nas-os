package activedirectory

import (
	"testing"
	"time"
)

func TestNewManager(t *testing.T) {
	cfg := ADConfig{
		Servers:      []string{"dc.example.com"},
		BaseDN:       "DC=example,DC=com",
		SyncInterval: 30 * time.Minute,
	}
	m := NewManager(cfg)
	if m == nil {
		t.Fatal("NewManager 返回 nil")
	}
	if len(m.config.Servers) != 1 {
		t.Errorf("期望 1 个 server, 实际 %d", len(m.config.Servers))
	}
}

func TestManager_StartStop(t *testing.T) {
	cfg := ADConfig{Servers: []string{"dc.test.com"}}
	m := NewManager(cfg)
	if err := m.Start(); err != nil {
		t.Fatalf("Start 失败: %v", err)
	}
	if !m.running {
		t.Error("期望 running=true")
	}
	m.Stop()
	if m.running {
		t.Error("期望 running=false")
	}
}

func TestManager_DomainLifecycle(t *testing.T) {
	cfg := ADConfig{Servers: []string{"dc.test.com"}}
	m := NewManager(cfg)

	// 添加域
	domain := &Domain{
		Name:   "example.com",
		Server: "dc.example.com",
		Port:   389,
		BaseDN: "DC=example,DC=com",
	}
	if err := m.AddDomain(domain); err != nil {
		t.Fatalf("AddDomain 失败: %v", err)
	}

	// 获取域
	got, err := m.GetDomain("example.com")
	if err != nil {
		t.Fatalf("GetDomain 失败: %v", err)
	}
	if got.Server != "dc.example.com" {
		t.Errorf("期望 server=dc.example.com, 实际 %s", got.Server)
	}

	// 列表
	domains := m.ListDomains()
	if len(domains) != 1 {
		t.Errorf("期望 1 个域, 实际 %d", len(domains))
	}

	// 重复添加
	if err := m.AddDomain(domain); err == nil {
		t.Error("重复添加应报错")
	}

	// 移除域
	if err := m.RemoveDomain("example.com"); err != nil {
		t.Fatalf("RemoveDomain 失败: %v", err)
	}
}

func TestManager_SyncUsers(t *testing.T) {
	cfg := ADConfig{Servers: []string{"dc.test.com"}}
	m := NewManager(cfg)
	m.AddDomain(&Domain{Name: "test.com", Server: "dc.test.com"})

	result, err := m.SyncUsers("test.com")
	if err != nil {
		t.Fatalf("SyncUsers 失败: %v", err)
	}
	if result.RecordsSynced != 10 {
		t.Errorf("期望同步 10 条记录, 实际 %d", result.RecordsSynced)
	}

	// 检查用户
	users := m.ListADUsers("test.com")
	if len(users) != 10 {
		t.Errorf("期望 10 个用户, 实际 %d", len(users))
	}
}

func TestManager_SyncGroups(t *testing.T) {
	cfg := ADConfig{Servers: []string{"dc.test.com"}}
	m := NewManager(cfg)
	m.AddDomain(&Domain{Name: "test.com", Server: "dc.test.com"})

	result, err := m.SyncGroups("test.com")
	if err != nil {
		t.Fatalf("SyncGroups 失败: %v", err)
	}
	if result.RecordsSynced != 4 {
		t.Errorf("期望同步 4 条记录, 实际 %d", result.RecordsSynced)
	}
}

func TestManager_GetStats(t *testing.T) {
	cfg := ADConfig{Servers: []string{"dc.test.com"}}
	m := NewManager(cfg)
	m.AddDomain(&Domain{Name: "test.com", Server: "dc.test.com"})
	m.SyncUsers("test.com")
	m.SyncGroups("test.com")

	stats := m.GetStats()
	if stats.TotalDomains != 1 {
		t.Errorf("期望 1 个域, 实际 %d", stats.TotalDomains)
	}
	if stats.TotalUsers != 10 {
		t.Errorf("期望 10 个用户, 实际 %d", stats.TotalUsers)
	}
	if stats.TotalGroups != 4 {
		t.Errorf("期望 4 个组, 实际 %d", stats.TotalGroups)
	}
}
