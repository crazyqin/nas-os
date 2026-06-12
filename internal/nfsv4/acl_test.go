package nfsv4

import (
	"testing"
)

func TestSetACL(t *testing.T) {
	m := NewACLManager()

	aces := []*NFSv4ACE{
		{
			Type:        ACLTypeAllow,
			Principal:   "user1",
			Permissions: ACLPermReadData | ACLPermExecute,
		},
		{
			Type:        ACLTypeDeny,
			Principal:   "user2",
			Permissions: ACLPermWriteData,
		},
	}

	err := m.SetACL("/data/shared", "root", "users", aces)
	if err != nil {
		t.Fatalf("设置 ACL 失败: %v", err)
	}

	acl, err := m.GetACL("/data/shared")
	if err != nil {
		t.Fatalf("获取 ACL 失败: %v", err)
	}

	if acl.Owner != "root" {
		t.Errorf("期望所有者 root，实际 %s", acl.Owner)
	}

	if len(acl.ACEs) != 2 {
		t.Errorf("期望 2 个 ACE，实际 %d", len(acl.ACEs))
	}
}

func TestAddACE(t *testing.T) {
	m := NewACLManager()

	m.SetACL("/data/shared", "root", "users", nil)

	ace := &NFSv4ACE{
		Type:        ACLTypeAllow,
		Principal:   "user3",
		Permissions: ACLPermReadData,
	}

	err := m.AddACE("/data/shared", ace)
	if err != nil {
		t.Fatalf("添加 ACE 失败: %v", err)
	}

	acl, _ := m.GetACL("/data/shared")
	if len(acl.ACEs) != 1 {
		t.Errorf("期望 1 个 ACE，实际 %d", len(acl.ACEs))
	}
}

func TestRemoveACE(t *testing.T) {
	m := NewACLManager()

	ace := &NFSv4ACE{
		Type:        ACLTypeAllow,
		Principal:   "user1",
		Permissions: ACLPermReadData,
	}

	m.AddACE("/data/shared", ace)

	err := m.RemoveACE(ace.ID)
	if err != nil {
		t.Fatalf("移除 ACE 失败: %v", err)
	}

	acl, _ := m.GetACL("/data/shared")
	if len(acl.ACEs) != 0 {
		t.Errorf("期望 0 个 ACE，实际 %d", len(acl.ACEs))
	}
}

func TestCheckPermission(t *testing.T) {
	m := NewACLManager()

	aces := []*NFSv4ACE{
		{
			Type:        ACLTypeAllow,
			Principal:   "user1",
			Permissions: ACLPermReadData | ACLPermExecute,
		},
		{
			Type:        ACLTypeDeny,
			Principal:   "user2",
			Permissions: ACLPermWriteData,
		},
	}

	m.SetACL("/data/shared", "root", "users", aces)

	// 测试允许的权限
	allowed, err := m.CheckPermission("/data/shared", "user1", ACLPermReadData)
	if err != nil {
		t.Fatalf("检查权限失败: %v", err)
	}
	if !allowed {
		t.Error("user1 应该有读取权限")
	}

	// 测试拒绝的权限
	allowed, err = m.CheckPermission("/data/shared", "user2", ACLPermWriteData)
	if err != nil {
		t.Fatalf("检查权限失败: %v", err)
	}
	if allowed {
		t.Error("user2 应该被拒绝写入权限")
	}

	// 测试未授权用户
	allowed, err = m.CheckPermission("/data/shared", "user3", ACLPermReadData)
	if err != nil {
		t.Fatalf("检查权限失败: %v", err)
	}
	if allowed {
		t.Error("user3 不应该有任何权限")
	}
}

func TestDeleteACL(t *testing.T) {
	m := NewACLManager()

	aces := []*NFSv4ACE{
		{Type: ACLTypeAllow, Principal: "user1", Permissions: ACLPermReadData},
	}

	m.SetACL("/data/shared", "root", "users", aces)

	err := m.DeleteACL("/data/shared")
	if err != nil {
		t.Fatalf("删除 ACL 失败: %v", err)
	}

	_, err = m.GetACL("/data/shared")
	if err == nil {
		t.Fatal("期望获取已删除 ACL 失败，但成功了")
	}
}

func TestListACLs(t *testing.T) {
	m := NewACLManager()

	m.SetACL("/data/shared1", "root", "users", nil)
	m.SetACL("/data/shared2", "root", "users", nil)

	acls := m.ListACLs()
	if len(acls) != 2 {
		t.Errorf("期望 2 个 ACL，实际 %d", len(acls))
	}
}

func TestACLGetStats(t *testing.T) {
	m := NewACLManager()

	aces := []*NFSv4ACE{
		{Type: ACLTypeAllow, Principal: "user1", Permissions: ACLPermReadData},
		{Type: ACLTypeDeny, Principal: "user2", Permissions: ACLPermWriteData},
	}

	m.SetACL("/data/shared", "root", "users", aces)

	stats := m.GetStats()
	if stats["total_acls"] != 1 {
		t.Errorf("期望 1 个 ACL，实际 %v", stats["total_acls"])
	}
	if stats["total_aces"] != 2 {
		t.Errorf("期望 2 个 ACE，实际 %v", stats["total_aces"])
	}
}

func TestUpdateACE(t *testing.T) {
	m := NewACLManager()

	ace := &NFSv4ACE{
		Type:        ACLTypeAllow,
		Principal:   "user1",
		Permissions: ACLPermReadData,
	}

	m.AddACE("/data/shared", ace)

	err := m.UpdateACE(ace.ID, func(a *NFSv4ACE) {
		a.Permissions = ACLPermReadData | ACLPermWriteData
		a.Principal = "user1-updated"
	})
	if err != nil {
		t.Fatalf("更新 ACE 失败: %v", err)
	}

	if ace.Permissions != ACLPermReadData|ACLPermWriteData {
		t.Errorf("期望权限 %d，实际 %d", ACLPermReadData|ACLPermWriteData, ace.Permissions)
	}

	if ace.Principal != "user1-updated" {
		t.Errorf("期望主体 user1-updated，实际 %s", ace.Principal)
	}
}

func TestWildcardPermission(t *testing.T) {
	m := NewACLManager()

	aces := []*NFSv4ACE{
		{
			Type:        ACLTypeAllow,
			Principal:   "*",
			Permissions: ACLPermReadData,
		},
	}

	m.SetACL("/data/public", "root", "users", aces)

	// 任何用户都应该有读取权限
	allowed, _ := m.CheckPermission("/data/public", "anyone", ACLPermReadData)
	if !allowed {
		t.Error("通配符应允许任何人读取")
	}
}
