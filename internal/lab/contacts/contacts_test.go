package contacts

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func newTestManager() *Manager {
	logger, _ := zap.NewDevelopment()
	return NewManager(logger, nil)
}

// ========== 联系人 CRUD 测试 ==========

func TestCreateContact(t *testing.T) {
	manager := newTestManager()

	req := &ContactCreateRequest{
		FirstName: "张三",
		LastName:  "李",
		Company:   "测试公司",
		Phones: []Phone{
			{Type: "mobile", Number: "13800138000"},
			{Type: "work", Number: "010-12345678"},
		},
		Emails: []Email{
			{Type: "work", Email: "zhangsan@example.com"},
		},
	}

	contact, err := manager.CreateContact(req)
	require.NoError(t, err)
	assert.NotEmpty(t, contact.ID)
	assert.Equal(t, "张三", contact.FirstName)
	assert.Equal(t, "李", contact.LastName)
	assert.Equal(t, "测试公司", contact.Company)
	assert.Len(t, contact.Phones, 2)
	assert.Len(t, contact.Emails, 1)
}

func TestGetContact(t *testing.T) {
	manager := newTestManager()

	// 创建联系人
	req := &ContactCreateRequest{
		FirstName: "王五",
		LastName:  "赵",
	}
	created, err := manager.CreateContact(req)
	require.NoError(t, err)

	// 获取联系人
	contact, err := manager.GetContact(created.ID)
	require.NoError(t, err)
	assert.Equal(t, created.ID, contact.ID)
	assert.Equal(t, "王五", contact.FirstName)

	// 获取不存在的联系人
	_, err = manager.GetContact("nonexistent")
	assert.Error(t, err)
}

func TestUpdateContact(t *testing.T) {
	manager := newTestManager()

	// 创建联系人
	req := &ContactCreateRequest{
		FirstName: "张三",
	}
	created, err := manager.CreateContact(req)
	require.NoError(t, err)

	// 更新联系人
	updateReq := &ContactUpdateRequest{
		FirstName: "张三丰",
		LastName:  "武当",
		Company:   "武当派",
	}
	updated, err := manager.UpdateContact(created.ID, updateReq)
	require.NoError(t, err)
	assert.Equal(t, "张三丰", updated.FirstName)
	assert.Equal(t, "武当", updated.LastName)
	assert.Equal(t, "武当派", updated.Company)
}

func TestDeleteContact(t *testing.T) {
	manager := newTestManager()

	// 创建联系人
	req := &ContactCreateRequest{
		FirstName: "待删除",
	}
	created, err := manager.CreateContact(req)
	require.NoError(t, err)

	// 删除联系人
	err = manager.DeleteContact(created.ID)
	require.NoError(t, err)

	// 验证已删除
	_, err = manager.GetContact(created.ID)
	assert.Error(t, err)

	// 删除不存在的联系人
	err = manager.DeleteContact("nonexistent")
	assert.Error(t, err)
}

func TestListContacts(t *testing.T) {
	manager := newTestManager()

	// 创建多个联系人
	for i := 0; i < 5; i++ {
		_, err := manager.CreateContact(&ContactCreateRequest{
			FirstName: "联系人",
			LastName:  string(rune('A' + i)),
		})
		require.NoError(t, err)
	}

	// 列出联系人
	contacts := manager.ListContacts(10, 0)
	assert.Len(t, contacts, 5)

	// 分页
	contacts = manager.ListContacts(2, 0)
	assert.Len(t, contacts, 2)

	contacts = manager.ListContacts(2, 2)
	assert.Len(t, contacts, 2)
}

// ========== 分组管理测试 ==========

func TestCreateGroup(t *testing.T) {
	manager := newTestManager()

	req := &ContactGroupCreateRequest{
		Name:        "同事",
		Description: "公司同事",
		Color:       "#FF5733",
	}

	group, err := manager.CreateGroup(req)
	require.NoError(t, err)
	assert.NotEmpty(t, group.ID)
	assert.Equal(t, "同事", group.Name)
	assert.Equal(t, "公司同事", group.Description)
}

func TestGetGroup(t *testing.T) {
	manager := newTestManager()

	// 创建分组
	req := &ContactGroupCreateRequest{
		Name: "家人",
	}
	created, err := manager.CreateGroup(req)
	require.NoError(t, err)

	// 获取分组
	group, err := manager.GetGroup(created.ID)
	require.NoError(t, err)
	assert.Equal(t, created.ID, group.ID)
	assert.Equal(t, "家人", group.Name)

	// 获取不存在的分组
	_, err = manager.GetGroup("nonexistent")
	assert.Error(t, err)
}

func TestUpdateGroup(t *testing.T) {
	manager := newTestManager()

	// 创建分组
	req := &ContactGroupCreateRequest{
		Name: "原名称",
	}
	created, err := manager.CreateGroup(req)
	require.NoError(t, err)

	// 更新分组
	updateReq := &ContactGroupUpdateRequest{
		Name:        "新名称",
		Description: "新描述",
	}
	updated, err := manager.UpdateGroup(created.ID, updateReq)
	require.NoError(t, err)
	assert.Equal(t, "新名称", updated.Name)
	assert.Equal(t, "新描述", updated.Description)
}

func TestDeleteGroup(t *testing.T) {
	manager := newTestManager()

	// 创建分组
	req := &ContactGroupCreateRequest{
		Name: "待删除",
	}
	created, err := manager.CreateGroup(req)
	require.NoError(t, err)

	// 删除分组
	err = manager.DeleteGroup(created.ID)
	require.NoError(t, err)

	// 验证已删除
	_, err = manager.GetGroup(created.ID)
	assert.Error(t, err)
}

func TestListGroups(t *testing.T) {
	manager := newTestManager()

	// 创建多个分组
	groups := []string{"家人", "同事", "朋友"}
	for _, name := range groups {
		_, err := manager.CreateGroup(&ContactGroupCreateRequest{
			Name: name,
		})
		require.NoError(t, err)
	}

	// 列出分组
	result := manager.ListGroups()
	assert.Len(t, result, 3)
}

func TestAddContactsToGroup(t *testing.T) {
	manager := newTestManager()

	// 创建分组
	group, err := manager.CreateGroup(&ContactGroupCreateRequest{
		Name: "测试组",
	})
	require.NoError(t, err)

	// 创建联系人
	contact1, err := manager.CreateContact(&ContactCreateRequest{
		FirstName: "联系人1",
	})
	require.NoError(t, err)

	contact2, err := manager.CreateContact(&ContactCreateRequest{
		FirstName: "联系人2",
	})
	require.NoError(t, err)

	// 添加联系人到分组
	err = manager.AddContactsToGroup(group.ID, []string{contact1.ID, contact2.ID})
	require.NoError(t, err)

	// 验证
	updatedGroup, _ := manager.GetGroup(group.ID)
	assert.Len(t, updatedGroup.ContactIDs, 2)

	updatedContact1, _ := manager.GetContact(contact1.ID)
	assert.Contains(t, updatedContact1.Groups, group.ID)
}

func TestRemoveContactsFromGroup(t *testing.T) {
	manager := newTestManager()

	// 创建分组
	group, err := manager.CreateGroup(&ContactGroupCreateRequest{
		Name: "测试组",
	})
	require.NoError(t, err)

	// 创建联系人
	contact1, err := manager.CreateContact(&ContactCreateRequest{
		FirstName: "联系人1",
		Groups:    []string{group.ID},
	})
	require.NoError(t, err)

	// 从分组移除
	err = manager.RemoveContactsFromGroup(group.ID, []string{contact1.ID})
	require.NoError(t, err)

	// 验证
	updatedGroup, _ := manager.GetGroup(group.ID)
	assert.Len(t, updatedGroup.ContactIDs, 0)
}

// ========== 搜索功能测试 ==========

func TestSearchContacts(t *testing.T) {
	manager := newTestManager()

	// 创建测试数据
	manager.CreateContact(&ContactCreateRequest{
		FirstName: "张三",
		LastName:  "李",
		Company:   "阿里巴巴",
		Phones:    []Phone{{Type: "mobile", Number: "13800138000"}},
		Emails:    []Email{{Type: "work", Email: "zhangsan@alibaba.com"}},
	})

	manager.CreateContact(&ContactCreateRequest{
		FirstName: "王五",
		LastName:  "赵",
		Company:   "腾讯",
		Phones:    []Phone{{Type: "mobile", Number: "13900139000"}},
	})

	// 按姓名搜索
	results := manager.SearchContacts(&SearchRequest{
		Query: "张三",
	})
	assert.Len(t, results, 1)
	assert.Equal(t, "张三", results[0].FirstName)

	// 按公司搜索
	results = manager.SearchContacts(&SearchRequest{
		Company: "腾讯",
	})
	assert.Len(t, results, 1)
	assert.Equal(t, "王五", results[0].FirstName)

	// 按电话搜索
	results = manager.SearchContacts(&SearchRequest{
		Phone: "13800138000",
	})
	assert.Len(t, results, 1)

	// 按邮箱搜索
	results = manager.SearchContacts(&SearchRequest{
		Email: "zhangsan@alibaba.com",
	})
	assert.Len(t, results, 1)
}

// ========== vCard 测试 ==========

func TestExportVCard(t *testing.T) {
	manager := newTestManager()

	// 创建联系人
	contact, err := manager.CreateContact(&ContactCreateRequest{
		FirstName: "张三",
		LastName:  "李",
		Company:   "测试公司",
		Phones:    []Phone{{Type: "mobile", Number: "13800138000"}},
		Emails:    []Email{{Type: "work", Email: "test@example.com"}},
	})
	require.NoError(t, err)

	// 导出 vCard
	vcard, err := manager.ExportVCard(contact.ID)
	require.NoError(t, err)

	assert.Contains(t, vcard, "BEGIN:VCARD")
	assert.Contains(t, vcard, "END:VCARD")
	assert.Contains(t, vcard, "张三")
	assert.Contains(t, vcard, "李")
	assert.Contains(t, vcard, "测试公司")
}

func TestImportVCard(t *testing.T) {
	manager := newTestManager()

	vcardContent := `BEGIN:VCARD
VERSION:3.0
N:王;五;;;
FN:五 王
ORG:新公司
TEL;TYPE=mobile:13900139000
EMAIL;TYPE=work:wangwu@example.com
END:VCARD`

	result, err := manager.ImportVCard(vcardContent, "")
	require.NoError(t, err)
	assert.Equal(t, 1, result.Total)
	assert.Equal(t, 1, result.Imported)

	// 验证导入的联系人
	contacts := manager.ListContacts(10, 0)
	assert.Len(t, contacts, 1)
	assert.Equal(t, "五", contacts[0].FirstName)
	assert.Equal(t, "王", contacts[0].LastName)
}

func TestImportVCardMultiple(t *testing.T) {
	manager := newTestManager()

	vcardContent := `BEGIN:VCARD
VERSION:3.0
N:张;三;;;
FN:三 张
TEL;TYPE=mobile:13800138000
END:VCARD

BEGIN:VCARD
VERSION:3.0
N:李;四;;;
FN:四 李
TEL;TYPE=mobile:13900139000
END:VCARD`

	result, err := manager.ImportVCard(vcardContent, "")
	require.NoError(t, err)
	assert.Equal(t, 2, result.Total)
	assert.Equal(t, 2, result.Imported)
}

func TestExportVCardBatch(t *testing.T) {
	manager := newTestManager()

	// 创建多个联系人
	c1, _ := manager.CreateContact(&ContactCreateRequest{FirstName: "张三"})
	c2, _ := manager.CreateContact(&ContactCreateRequest{FirstName: "李四"})

	// 批量导出
	vcard, err := manager.ExportVCardBatch([]string{c1.ID, c2.ID})
	require.NoError(t, err)

	assert.Contains(t, vcard, "张三")
	assert.Contains(t, vcard, "李四")
}

// ========== CSV 导入测试 ==========

func TestImportCSV(t *testing.T) {
	manager := newTestManager()

	csvContent := `firstName,lastName,company,phone,email
张三,李,阿里巴巴,13800138000,zhangsan@alibaba.com
王五,赵,腾讯,13900139000,wangwu@tencent.com`

	result, err := manager.ImportCSV(csvContent, "")
	require.NoError(t, err)
	assert.Equal(t, 2, result.Total)
	assert.Equal(t, 2, result.Imported)
	assert.Equal(t, 0, result.Failed)
}

func TestImportCSVWithGroup(t *testing.T) {
	manager := newTestManager()

	// 创建分组
	group, _ := manager.CreateGroup(&ContactGroupCreateRequest{
		Name: "测试组",
	})

	csvContent := `firstName,lastName
张三,李
王五,赵`

	result, err := manager.ImportCSV(csvContent, group.ID)
	require.NoError(t, err)
	assert.Equal(t, 2, result.Imported)

	// 验证分组
	updatedGroup, _ := manager.GetGroup(group.ID)
	assert.Len(t, updatedGroup.ContactIDs, 2)
}

// ========== 去重功能测试 ==========

func TestFindDuplicates(t *testing.T) {
	manager := newTestManager()

	// 创建重复联系人（同名、同电话、同公司，确保高相似度）
	manager.CreateContact(&ContactCreateRequest{
		FirstName: "张三",
		LastName:  "李",
		Company:   "阿里巴巴",
		Phones:    []Phone{{Type: "mobile", Number: "13800138000"}},
		Emails:    []Email{{Type: "work", Email: "zhangsan@example.com"}},
	})

	manager.CreateContact(&ContactCreateRequest{
		FirstName: "张三",
		LastName:  "李",
		Company:   "阿里巴巴",
		Phones:    []Phone{{Type: "mobile", Number: "13800138000"}},
		Emails:    []Email{{Type: "work", Email: "zhangsan@example.com"}},
	})

	// 查找重复
	duplicates := manager.FindDuplicates()
	assert.Len(t, duplicates, 1)
	assert.Greater(t, duplicates[0].Score, 0.8)
}

func TestMergeContacts(t *testing.T) {
	manager := newTestManager()

	// 创建联系人
	c1, _ := manager.CreateContact(&ContactCreateRequest{
		FirstName: "张三",
		Phones:    []Phone{{Type: "mobile", Number: "13800138000"}},
	})

	c2, _ := manager.CreateContact(&ContactCreateRequest{
		FirstName: "张三",
		Emails:    []Email{{Type: "work", Email: "zhangsan@example.com"}},
	})

	// 合并
	result, err := manager.MergeContacts(c1.ID, []string{c2.ID})
	require.NoError(t, err)

	assert.Equal(t, c1.ID, result.Kept.ID)
	assert.Len(t, result.Merged, 1)
	assert.Len(t, result.Kept.Phones, 1)
	assert.Len(t, result.Kept.Emails, 1)

	// 验证被合并的联系人已删除
	_, err = manager.GetContact(c2.ID)
	assert.Error(t, err)
}

// ========== 分享功能测试 ==========

func TestShareGroup(t *testing.T) {
	manager := newTestManager()

	// 创建分组
	group, _ := manager.CreateGroup(&ContactGroupCreateRequest{
		Name: "分享测试组",
	})

	// 分享
	req := &ShareRequest{
		GroupID:    group.ID,
		TargetUser: []string{"user1", "user2"},
		Permission: "read",
	}

	share, err := manager.ShareGroup(req)
	require.NoError(t, err)
	assert.NotEmpty(t, share.ID)
	assert.Equal(t, group.ID, share.GroupID)
	assert.Len(t, share.SharedWith, 2)
}

func TestGetShares(t *testing.T) {
	manager := newTestManager()

	// 创建分组
	group, _ := manager.CreateGroup(&ContactGroupCreateRequest{
		Name: "分享测试组",
	})

	// 分享
	manager.ShareGroup(&ShareRequest{
		GroupID:    group.ID,
		TargetUser: []string{"user1"},
	})

	// 获取分享信息
	shares := manager.GetShares(group.ID)
	assert.Len(t, shares, 1)
}

func TestRevokeShare(t *testing.T) {
	manager := newTestManager()

	// 创建分组
	group, _ := manager.CreateGroup(&ContactGroupCreateRequest{
		Name: "分享测试组",
	})

	// 分享
	share, _ := manager.ShareGroup(&ShareRequest{
		GroupID:    group.ID,
		TargetUser: []string{"user1"},
	})

	// 撤销分享
	err := manager.RevokeShare(share.ID)
	require.NoError(t, err)

	// 验证已撤销
	shares := manager.GetShares(group.ID)
	assert.Len(t, shares, 0)
}

// ========== 统计测试 ==========

func TestGetStats(t *testing.T) {
	manager := newTestManager()

	// 创建数据
	manager.CreateContact(&ContactCreateRequest{FirstName: "张三"})
	manager.CreateContact(&ContactCreateRequest{FirstName: "李四"})
	manager.CreateGroup(&ContactGroupCreateRequest{Name: "测试组"})

	stats := manager.GetStats()
	assert.Equal(t, 2, stats["total_contacts"])
	assert.Equal(t, 1, stats["total_groups"])
	assert.Equal(t, 0, stats["total_shares"])
}

// ========== 边界情况测试 ==========

func TestMaxContactsLimit(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	config := &ContactsConfig{
		MaxContacts:    2,
		MaxGroups:      10,
		DefaultCountry: "CN",
		DedupThreshold: 0.8,
	}
	manager := NewManager(logger, config)

	// 创建达到上限
	_, err := manager.CreateContact(&ContactCreateRequest{FirstName: "1"})
	require.NoError(t, err)
	_, err = manager.CreateContact(&ContactCreateRequest{FirstName: "2"})
	require.NoError(t, err)

	// 超出上限
	_, err = manager.CreateContact(&ContactCreateRequest{FirstName: "3"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "maximum contacts limit")
}

func TestMaxGroupsLimit(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	config := &ContactsConfig{
		MaxContacts:    100,
		MaxGroups:      2,
		DefaultCountry: "CN",
		DedupThreshold: 0.8,
	}
	manager := NewManager(logger, config)

	// 创建达到上限
	_, err := manager.CreateGroup(&ContactGroupCreateRequest{Name: "1"})
	require.NoError(t, err)
	_, err = manager.CreateGroup(&ContactGroupCreateRequest{Name: "2"})
	require.NoError(t, err)

	// 超出上限
	_, err = manager.CreateGroup(&ContactGroupCreateRequest{Name: "3"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "maximum groups limit")
}

// ========== 电话标准化测试 ==========

func TestNormalizePhone(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"13800138000", "13800138000"},
		{"+8613800138000", "13800138000"},
		{"8613800138000", "13800138000"},
		{"138-0013-8000", "13800138000"},
		{"(138) 0013 8000", "13800138000"},
	}

	for _, tt := range tests {
		result := normalizePhone(tt.input)
		assert.Equal(t, tt.expected, result, "normalizePhone(%s)", tt.input)
	}
}

// ========== 相似度计算测试 ==========

func TestCalculateSimilarity(t *testing.T) {
	manager := newTestManager()

	c1 := &Contact{
		FirstName: "张三",
		LastName:  "李",
		Phones:    []Phone{{Type: "mobile", Number: "13800138000"}},
		Emails:    []Email{{Type: "work", Email: "test@example.com"}},
		Company:   "阿里巴巴",
	}

	c2 := &Contact{
		FirstName: "张三",
		LastName:  "李",
		Phones:    []Phone{{Type: "mobile", Number: "13800138000"}},
		Emails:    []Email{{Type: "work", Email: "test@example.com"}},
		Company:   "阿里巴巴",
	}

	score, reasons := manager.calculateSimilarity(c1, c2)
	assert.Equal(t, 1.0, score)
	assert.NotEmpty(t, reasons)

	// 不同联系人
	c3 := &Contact{
		FirstName: "王五",
		Phones:    []Phone{{Type: "mobile", Number: "13900139000"}},
	}

	score, _ = manager.calculateSimilarity(c1, c3)
	assert.Less(t, score, 0.5)
}
