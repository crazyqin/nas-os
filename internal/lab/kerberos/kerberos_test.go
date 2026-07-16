package kerberos

import (
	"testing"
	"time"
)

func TestNewManager(t *testing.T) {
	cfg := KerberosConfig{
		Realm:          "EXAMPLE.COM",
		KDCHost:        "kdc.example.com",
		KDCPort:        88,
		TicketLifetime: 10 * time.Hour,
		RenewLifetime:  7 * 24 * time.Hour,
	}
	m := NewManager(cfg)
	if m == nil {
		t.Fatal("NewManager 返回 nil")
	}
	if m.config.Realm != "EXAMPLE.COM" {
		t.Errorf("期望 realm EXAMPLE.COM, 实际 %s", m.config.Realm)
	}
}

func TestManager_StartStop(t *testing.T) {
	cfg := KerberosConfig{Realm: "TEST.COM", KDCHost: "kdc.test.com"}
	m := NewManager(cfg)
	if err := m.Start(); err != nil {
		t.Fatalf("Start 失败: %v", err)
	}
	if !m.running {
		t.Error("期望 running=true")
	}
	// 重复启动不应报错
	if err := m.Start(); err != nil {
		t.Errorf("重复 Start 报错: %v", err)
	}
	m.Stop()
	if m.running {
		t.Error("期望 running=false")
	}
}

func TestManager_ConfigureRealm(t *testing.T) {
	cfg := KerberosConfig{Realm: "TEST.COM"}
	m := NewManager(cfg)
	realm := &Realm{
		Name:    "TEST.COM",
		KDCHost: "kdc.test.com",
		KDCPort: 88,
		Domain:  "test.com",
	}
	if err := m.ConfigureRealm(realm); err != nil {
		t.Fatalf("ConfigureRealm 失败: %v", err)
	}
	got := m.GetRealm()
	if got == nil || got.Name != "TEST.COM" {
		t.Error("Realm 配置不正确")
	}
}

func TestManager_PrincipalLifecycle(t *testing.T) {
	cfg := KerberosConfig{Realm: "TEST.COM"}
	m := NewManager(cfg)

	// 创建
	p := &Principal{
		ID:   "p1",
		Name: "admin",
		Type: PrincipalTypeAdmin,
	}
	if err := m.CreatePrincipal(p); err != nil {
		t.Fatalf("CreatePrincipal 失败: %v", err)
	}

	// 获取
	got, err := m.GetPrincipal("p1")
	if err != nil {
		t.Fatalf("GetPrincipal 失败: %v", err)
	}
	if got.Name != "admin" {
		t.Errorf("期望 name=admin, 实际 %s", got.Name)
	}

	// 列表
	list := m.ListPrincipals()
	if len(list) != 1 {
		t.Errorf("期望 1 个 principal, 实际 %d", len(list))
	}

	// 重复创建
	if err := m.CreatePrincipal(p); err == nil {
		t.Error("重复创建应报错")
	}

	// 删除
	if err := m.DeletePrincipal("p1"); err != nil {
		t.Fatalf("DeletePrincipal 失败: %v", err)
	}
	if _, err := m.GetPrincipal("p1"); err == nil {
		t.Error("删除后获取应报错")
	}
}

func TestManager_TicketLifecycle(t *testing.T) {
	cfg := KerberosConfig{
		Realm:          "TEST.COM",
		TicketLifetime: time.Hour,
		RenewLifetime:  24 * time.Hour,
	}
	m := NewManager(cfg)

	// 创建 principal
	p := &Principal{
		ID:        "p1",
		Name:      "user1",
		Type:      PrincipalTypeUser,
		ExpiresAt: time.Now().Add(24 * time.Hour),
	}
	m.CreatePrincipal(p)

	// 申请票据
	ticket, err := m.RequestTicket("p1")
	if err != nil {
		t.Fatalf("RequestTicket 失败: %v", err)
	}
	if ticket.PrincipalID != "p1" {
		t.Errorf("期望 principalID=p1, 实际 %s", ticket.PrincipalID)
	}

	// 验证票据
	valid, err := m.ValidateTicket(ticket.ID)
	if err != nil {
		t.Fatalf("ValidateTicket 失败: %v", err)
	}
	if !valid {
		t.Error("票据应有效")
	}

	// 吊销票据
	if err := m.RevokeTicket(ticket.ID); err != nil {
		t.Fatalf("RevokeTicket 失败: %v", err)
	}
	if _, err := m.ValidateTicket(ticket.ID); err == nil {
		t.Error("吊销后验证应报错")
	}
}

func TestManager_GetStats(t *testing.T) {
	cfg := KerberosConfig{Realm: "TEST.COM"}
	m := NewManager(cfg)
	m.CreatePrincipal(&Principal{ID: "p1", Name: "user1", Type: PrincipalTypeUser})
	m.CreatePrincipal(&Principal{ID: "p2", Name: "user2", Type: PrincipalTypeUser})
	stats := m.GetStats()
	if stats.TotalPrincipals != 2 {
		t.Errorf("期望 2 个 principal, 实际 %d", stats.TotalPrincipals)
	}
}
