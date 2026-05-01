// Package contacts 测试
package contacts

import (
	"testing"
)

func TestCreateContact(t *testing.T) {
	m := NewManager()
	c, err := m.CreateContact(CreateContactRequest{
		FirstName: "张",
		LastName:  "三",
		Emails:    []string{"zhangsan@example.com"},
		Phones:    []string{"13800138000"},
		Company:   "测试公司",
	})
	if err != nil {
		t.Fatalf("创建联系人失败: %v", err)
	}
	if c.FirstName != "张" {
		t.Errorf("姓名不匹配")
	}
}

func TestGetContact(t *testing.T) {
	m := NewManager()
	c, _ := m.CreateContact(CreateContactRequest{FirstName: "test", LastName: "user"})

	got, err := m.GetContact(c.ID)
	if err != nil {
		t.Fatalf("获取联系人失败: %v", err)
	}
	if got.FirstName != "test" {
		t.Errorf("姓名不匹配")
	}
}

func TestUpdateContact(t *testing.T) {
	m := NewManager()
	c, _ := m.CreateContact(CreateContactRequest{FirstName: "old", LastName: "name"})

	newFirst := "new"
	updated, _ := m.UpdateContact(c.ID, UpdateContactRequest{FirstName: &newFirst})
	if updated.FirstName != "new" {
		t.Errorf("姓名未更新")
	}
}

func TestDeleteContact(t *testing.T) {
	m := NewManager()
	c, _ := m.CreateContact(CreateContactRequest{FirstName: "to", LastName: "delete"})
	err := m.DeleteContact(c.ID)
	if err != nil {
		t.Fatalf("删除失败: %v", err)
	}
	_, err = m.GetContact(c.ID)
	if err == nil {
		t.Error("已删除联系人不应存在")
	}
}

func TestSearch(t *testing.T) {
	m := NewManager()
	m.CreateContact(CreateContactRequest{FirstName: "张三", Company: "阿里"})
	m.CreateContact(CreateContactRequest{FirstName: "李四", Company: "腾讯"})

	results := m.Search("阿里")
	if len(results) == 0 {
		t.Error("搜索应有结果")
	}
}

func TestGroupOperations(t *testing.T) {
	m := NewManager()
	g, _ := m.CreateGroup(CreateGroupRequest{Name: "同事", Description: "工作同事"})
	
	c1, _ := m.CreateContact(CreateContactRequest{FirstName: "A"})
	c2, _ := m.CreateContact(CreateContactRequest{FirstName: "B"})

	m.AddToGroup(c1.ID, g.ID)
	m.AddToGroup(c2.ID, g.ID)

	contacts := m.ListContacts(g.ID)
	if len(contacts) != 2 {
		t.Errorf("分组应有2个联系人，实际 %d", len(contacts))
	}

	m.RemoveFromGroup(c1.ID, g.ID)
	contacts = m.ListContacts(g.ID)
	if len(contacts) != 1 {
		t.Errorf("移除后应有1个联系人，实际 %d", len(contacts))
	}
}

func TestDetectDuplicates(t *testing.T) {
	m := NewManager()
	m.CreateContact(CreateContactRequest{FirstName: "张", LastName: "三"})
	m.CreateContact(CreateContactRequest{FirstName: "张", LastName: "三"})

	dups := m.DetectDuplicates()
	if len(dups) == 0 {
		t.Error("应检测到重复联系人")
	}
}

func TestBatchDelete(t *testing.T) {
	m := NewManager()
	c1, _ := m.CreateContact(CreateContactRequest{FirstName: "a"})
	c2, _ := m.CreateContact(CreateContactRequest{FirstName: "b"})
	c3, _ := m.CreateContact(CreateContactRequest{FirstName: "c"})

	count := m.BatchDelete([]string{c1.ID, c2.ID, c3.ID})
	if count != 3 {
		t.Errorf("应删除3个，实际 %d", count)
	}
}

func TestVCardExport(t *testing.T) {
	m := NewManager()
	c, _ := m.CreateContact(CreateContactRequest{
		FirstName: "Test",
		LastName:  "User",
		Emails:    []string{"test@example.com"},
	})

	vcard, err := m.ExportVCard(c.ID)
	if err != nil {
		t.Fatalf("导出失败: %v", err)
	}
	if vcard == "" {
		t.Error("vCard 不应为空")
	}
}
