package datasovereignty

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ========== 标签管理测试 ==========

func TestCreateTag(t *testing.T) {
	svc := NewService()

	req := TagRequest{
		ResourcePath:   "/data/eu-users/personal.csv",
		ResourceType:   ResourceFile,
		Frameworks:     []ComplianceFramework{FrameworkGDPR},
		AllowedRegions: []DataRegion{RegionEU},
		CreatedBy:      "admin",
	}

	tag, err := svc.CreateTag(req)
	require.NoError(t, err)
	assert.NotEmpty(t, tag.ID)
	assert.Equal(t, "/data/eu-users/personal.csv", tag.ResourcePath)
	assert.Equal(t, ResourceFile, tag.ResourceType)
	assert.Contains(t, tag.Frameworks, FrameworkGDPR)
	assert.Contains(t, tag.AllowedRegions, RegionEU)
	assert.Equal(t, "admin", tag.CreatedBy)
	assert.False(t, tag.CreatedAt.IsZero())
}

func TestCreateTagAutoRegion(t *testing.T) {
	svc := NewService()

	// 创建 PIPL 标签但未指定 CN 区域，应自动补充
	req := TagRequest{
		ResourcePath:   "/data/cn-userdata/",
		ResourceType:   ResourceFolder,
		Frameworks:     []ComplianceFramework{FrameworkPIPL},
		AllowedRegions: []DataRegion{RegionGlobal},
		CreatedBy:      "admin",
	}

	tag, err := svc.CreateTag(req)
	require.NoError(t, err)
	// PIPL 强制 CN 区域应被自动加入
	assert.Contains(t, tag.AllowedRegions, RegionCN)
}

func TestCreateTagDuplicate(t *testing.T) {
	svc := NewService()

	req := TagRequest{
		ResourcePath:   "/data/duplicate",
		ResourceType:   ResourceFile,
		Frameworks:     []ComplianceFramework{FrameworkGDPR},
		AllowedRegions: []DataRegion{RegionEU},
		CreatedBy:      "admin",
	}

	_, err := svc.CreateTag(req)
	require.NoError(t, err)

	// 同一路径再创建应报错
	_, err = svc.CreateTag(req)
	assert.ErrorIs(t, err, ErrTagAlreadyExists)
}

func TestCreateTagInvalidFramework(t *testing.T) {
	svc := NewService()

	req := TagRequest{
		ResourcePath:   "/data/test",
		ResourceType:   ResourceFile,
		Frameworks:     []ComplianceFramework{"INVALID"},
		AllowedRegions: []DataRegion{RegionEU},
		CreatedBy:      "admin",
	}

	_, err := svc.CreateTag(req)
	assert.ErrorIs(t, err, ErrInvalidFramework)
}

func TestCreateTagInvalidRegion(t *testing.T) {
	svc := NewService()

	req := TagRequest{
		ResourcePath:   "/data/test",
		ResourceType:   ResourceFile,
		Frameworks:     []ComplianceFramework{FrameworkGDPR},
		AllowedRegions: []DataRegion{"MARS"},
		CreatedBy:      "admin",
	}

	_, err := svc.CreateTag(req)
	assert.ErrorIs(t, err, ErrInvalidRegion)
}

func TestDeleteTag(t *testing.T) {
	svc := NewService()

	req := TagRequest{
		ResourcePath:   "/data/to-delete",
		ResourceType:   ResourceFile,
		Frameworks:     []ComplianceFramework{FrameworkGDPR},
		AllowedRegions: []DataRegion{RegionEU},
		CreatedBy:      "admin",
	}
	tag, _ := svc.CreateTag(req)

	err := svc.DeleteTag(tag.ID)
	assert.NoError(t, err)

	// 验证已删除
	_, err = svc.GetTag(tag.ID)
	assert.ErrorIs(t, err, ErrTagNotFound)
}

func TestDeleteTagNotFound(t *testing.T) {
	svc := NewService()

	err := svc.DeleteTag("nonexistent")
	assert.ErrorIs(t, err, ErrTagNotFound)
}

func TestGetTagByPath(t *testing.T) {
	svc := NewService()

	req := TagRequest{
		ResourcePath:   "/data/by-path",
		ResourceType:   ResourceFolder,
		Frameworks:     []ComplianceFramework{FrameworkCCPA},
		AllowedRegions: []DataRegion{RegionUS},
		CreatedBy:      "admin",
	}
	tag, _ := svc.CreateTag(req)

	found, err := svc.GetTagByPath("/data/by-path")
	require.NoError(t, err)
	assert.Equal(t, tag.ID, found.ID)
}

func TestGetTagByPathNotFound(t *testing.T) {
	svc := NewService()

	_, err := svc.GetTagByPath("/nonexistent")
	assert.ErrorIs(t, err, ErrTagNotFound)
}

func TestListTags(t *testing.T) {
	svc := NewService()

	svc.CreateTag(TagRequest{
		ResourcePath:   "/data/1",
		ResourceType:   ResourceFile,
		Frameworks:     []ComplianceFramework{FrameworkGDPR},
		AllowedRegions: []DataRegion{RegionEU},
		CreatedBy:      "admin",
	})
	svc.CreateTag(TagRequest{
		ResourcePath:   "/data/2",
		ResourceType:   ResourceFile,
		Frameworks:     []ComplianceFramework{FrameworkPIPL},
		AllowedRegions: []DataRegion{RegionCN},
		CreatedBy:      "admin",
	})

	tags := svc.ListTags()
	assert.Len(t, tags, 2)
}

// ========== 合规检查测试 ==========

func TestCheckTransferAllowed(t *testing.T) {
	svc := NewService()

	// 创建 GDPR 标签，只允许 EU 区域
	svc.CreateTag(TagRequest{
		ResourcePath:   "/data/eu-docs/report.pdf",
		ResourceType:   ResourceFile,
		Frameworks:     []ComplianceFramework{FrameworkGDPR},
		AllowedRegions: []DataRegion{RegionEU},
		CreatedBy:      "admin",
	})

	// 传输到 EU 应该允许
	resp, err := svc.CheckTransfer(CheckRequest{
		ResourcePath: "/data/eu-docs/report.pdf",
		Action:       ActionCopy,
		TargetRegion: RegionEU,
		User:         "user1",
	})
	require.NoError(t, err)
	assert.True(t, resp.Allowed)
	assert.Equal(t, TransferAllowed, resp.Status)
	assert.NotEmpty(t, resp.EntryID)
}

func TestCheckTransferBlocked(t *testing.T) {
	svc := NewService()

	// 创建 GDPR 标签
	svc.CreateTag(TagRequest{
		ResourcePath:   "/data/eu-docs/secret.pdf",
		ResourceType:   ResourceFile,
		Frameworks:     []ComplianceFramework{FrameworkGDPR},
		AllowedRegions: []DataRegion{RegionEU},
		CreatedBy:      "admin",
	})

	// 传输到 US 应该被阻止
	resp, err := svc.CheckTransfer(CheckRequest{
		ResourcePath: "/data/eu-docs/secret.pdf",
		Action:       ActionCopy,
		TargetRegion: RegionUS,
		User:         "user1",
	})
	require.NoError(t, err)
	assert.False(t, resp.Allowed)
	assert.Equal(t, TransferBlocked, resp.Status)
	assert.Contains(t, resp.Reason, "GDPR")
}

func TestCheckTransferPIPLBlocked(t *testing.T) {
	svc := NewService()

	// 创建 PIPL 标签
	svc.CreateTag(TagRequest{
		ResourcePath:   "/data/cn-userdata/info.csv",
		ResourceType:   ResourceFile,
		Frameworks:     []ComplianceFramework{FrameworkPIPL},
		AllowedRegions: []DataRegion{RegionCN},
		CreatedBy:      "admin",
	})

	// 传输到 EU 应该被阻止
	resp, err := svc.CheckTransfer(CheckRequest{
		ResourcePath: "/data/cn-userdata/info.csv",
		Action:       ActionMove,
		TargetRegion: RegionEU,
		User:         "user2",
	})
	require.NoError(t, err)
	assert.False(t, resp.Allowed)
	assert.Equal(t, TransferBlocked, resp.Status)
	assert.Contains(t, resp.Reason, "PIPL")
}

func TestCheckTransferNoTag(t *testing.T) {
	svc := NewService()

	// 无标签资源应该允许传输
	resp, err := svc.CheckTransfer(CheckRequest{
		ResourcePath: "/data/no-tag/file.txt",
		Action:       ActionCopy,
		TargetRegion: RegionUS,
		User:         "user1",
	})
	require.NoError(t, err)
	assert.True(t, resp.Allowed)
	assert.Equal(t, TransferAllowed, resp.Status)
	assert.Contains(t, resp.Reason, "无主权标签")
}

func TestCheckTransferInvalidRegion(t *testing.T) {
	svc := NewService()

	_, err := svc.CheckTransfer(CheckRequest{
		ResourcePath: "/data/test",
		Action:       ActionCopy,
		TargetRegion: "MARS",
		User:         "user1",
	})
	assert.ErrorIs(t, err, ErrInvalidRegion)
}

func TestCheckTransferRestrictedRegion(t *testing.T) {
	svc := NewService()

	// 创建标签，允许 US 但禁止 CN
	svc.CreateTag(TagRequest{
		ResourcePath:   "/data/us-only/data.dat",
		ResourceType:   ResourceFile,
		Frameworks:     []ComplianceFramework{FrameworkCCPA},
		AllowedRegions: []DataRegion{RegionUS},
		RestrictedRegions: []DataRegion{RegionCN},
		CreatedBy:      "admin",
	})

	// 传输到 CN 应该被阻止
	resp, err := svc.CheckTransfer(CheckRequest{
		ResourcePath: "/data/us-only/data.dat",
		Action:       ActionReplicate,
		TargetRegion: RegionCN,
		User:         "user1",
	})
	require.NoError(t, err)
	assert.False(t, resp.Allowed)
}

// ========== 审计日志测试 ==========

func TestQueryAuditAll(t *testing.T) {
	svc := NewService()

	// 产生一些审计日志
	svc.CheckTransfer(CheckRequest{
		ResourcePath: "/data/1", Action: ActionCopy, TargetRegion: RegionEU, User: "u1",
	})
	svc.CheckTransfer(CheckRequest{
		ResourcePath: "/data/2", Action: ActionMove, TargetRegion: RegionUS, User: "u2",
	})

	entries := svc.QueryAudit(AuditQuery{})
	assert.Len(t, entries, 2)
}

func TestQueryAuditByUser(t *testing.T) {
	svc := NewService()

	svc.CheckTransfer(CheckRequest{
		ResourcePath: "/data/1", Action: ActionCopy, TargetRegion: RegionEU, User: "alice",
	})
	svc.CheckTransfer(CheckRequest{
		ResourcePath: "/data/2", Action: ActionMove, TargetRegion: RegionUS, User: "bob",
	})
	svc.CheckTransfer(CheckRequest{
		ResourcePath: "/data/3", Action: ActionSync, TargetRegion: RegionCN, User: "alice",
	})

	entries := svc.QueryAudit(AuditQuery{User: "alice"})
	assert.Len(t, entries, 2)
	for _, e := range entries {
		assert.Equal(t, "alice", e.User)
	}
}

func TestQueryAuditByAction(t *testing.T) {
	svc := NewService()

	svc.CheckTransfer(CheckRequest{
		ResourcePath: "/data/1", Action: ActionCopy, TargetRegion: RegionEU, User: "u1",
	})
	svc.CheckTransfer(CheckRequest{
		ResourcePath: "/data/2", Action: ActionMove, TargetRegion: RegionUS, User: "u2",
	})
	svc.CheckTransfer(CheckRequest{
		ResourcePath: "/data/3", Action: ActionCopy, TargetRegion: RegionCN, User: "u3",
	})

	entries := svc.QueryAudit(AuditQuery{Action: ActionCopy})
	assert.Len(t, entries, 2)
	for _, e := range entries {
		assert.Equal(t, ActionCopy, e.Action)
	}
}

func TestQueryAuditByResourcePath(t *testing.T) {
	svc := NewService()

	svc.CheckTransfer(CheckRequest{
		ResourcePath: "/data/specific", Action: ActionCopy, TargetRegion: RegionEU, User: "u1",
	})
	svc.CheckTransfer(CheckRequest{
		ResourcePath: "/data/other", Action: ActionMove, TargetRegion: RegionUS, User: "u2",
	})

	entries := svc.QueryAudit(AuditQuery{ResourcePath: "/data/specific"})
	assert.Len(t, entries, 1)
	assert.Equal(t, "/data/specific", entries[0].ResourcePath)
}

func TestQueryAuditByStatus(t *testing.T) {
	svc := NewService()

	// 创建一个会被阻止的传输
	svc.CreateTag(TagRequest{
		ResourcePath:   "/data/blocked",
		ResourceType:   ResourceFile,
		Frameworks:     []ComplianceFramework{FrameworkGDPR},
		AllowedRegions: []DataRegion{RegionEU},
		CreatedBy:      "admin",
	})

	svc.CheckTransfer(CheckRequest{
		ResourcePath: "/data/blocked", Action: ActionCopy, TargetRegion: RegionUS, User: "u1",
	})
	svc.CheckTransfer(CheckRequest{
		ResourcePath: "/data/free", Action: ActionCopy, TargetRegion: RegionUS, User: "u2",
	})

	blocked := svc.QueryAudit(AuditQuery{Status: TransferBlocked})
	assert.Len(t, blocked, 1)
	assert.Equal(t, TransferBlocked, blocked[0].Status)

	allowed := svc.QueryAudit(AuditQuery{Status: TransferAllowed})
	assert.Len(t, allowed, 1)
}

func TestQueryAuditWithLimit(t *testing.T) {
	svc := NewService()

	for i := 0; i < 10; i++ {
		svc.CheckTransfer(CheckRequest{
			ResourcePath: "/data/file", Action: ActionCopy, TargetRegion: RegionEU, User: "u1",
		})
	}

	entries := svc.QueryAudit(AuditQuery{Limit: 5})
	assert.Len(t, entries, 5)
}

func TestQueryAuditByTimeRange(t *testing.T) {
	svc := NewService()

	before := time.Now().Add(-1 * time.Second)
	svc.CheckTransfer(CheckRequest{
		ResourcePath: "/data/1", Action: ActionCopy, TargetRegion: RegionEU, User: "u1",
	})
	after := time.Now().Add(1 * time.Second)

	entries := svc.QueryAudit(AuditQuery{StartTime: &before, EndTime: &after})
	assert.Len(t, entries, 1)
}

func TestGetAuditCount(t *testing.T) {
	svc := NewService()

	assert.Equal(t, 0, svc.GetAuditCount())

	svc.CheckTransfer(CheckRequest{
		ResourcePath: "/data/1", Action: ActionCopy, TargetRegion: RegionEU, User: "u1",
	})
	svc.CheckTransfer(CheckRequest{
		ResourcePath: "/data/2", Action: ActionMove, TargetRegion: RegionUS, User: "u2",
	})

	assert.Equal(t, 2, svc.GetAuditCount())
}

// ========== 集成测试 ==========

func TestFullSovereigntyFlow(t *testing.T) {
	svc := NewService()

	// 1. 创建标签
	tag, err := svc.CreateTag(TagRequest{
		ResourcePath:   "/data/gdpr-sensitive/user-data.csv",
		ResourceType:   ResourceFile,
		Frameworks:     []ComplianceFramework{FrameworkGDPR},
		AllowedRegions: []DataRegion{RegionEU},
		Description:    "GDPR 敏感用户数据",
		CreatedBy:      "compliance-officer",
	})
	require.NoError(t, err)
	assert.NotEmpty(t, tag.ID)

	// 2. 合规检查 - 允许传输到 EU
	resp, err := svc.CheckTransfer(CheckRequest{
		ResourcePath: "/data/gdpr-sensitive/user-data.csv",
		Action:       ActionBackup,
		TargetRegion: RegionEU,
		User:         "backup-svc",
		ClientIP:     "10.0.0.1",
	})
	require.NoError(t, err)
	assert.True(t, resp.Allowed)

	// 3. 合规检查 - 阻止传输到 CN
	resp, err = svc.CheckTransfer(CheckRequest{
		ResourcePath: "/data/gdpr-sensitive/user-data.csv",
		Action:       ActionReplicate,
		TargetRegion: RegionCN,
		User:         "replication-svc",
		ClientIP:     "10.0.0.2",
	})
	require.NoError(t, err)
	assert.False(t, resp.Allowed)
	assert.Equal(t, TransferBlocked, resp.Status)

	// 4. 查询审计日志
	entries := svc.QueryAudit(AuditQuery{})
	assert.Len(t, entries, 2) // 一次允许 + 一次阻止

	// 5. 按状态查询
	blocked := svc.QueryAudit(AuditQuery{Status: TransferBlocked})
	assert.Len(t, blocked, 1)
	assert.Contains(t, blocked[0].Reason, "GDPR")

	// 6. 删除标签
	err = svc.DeleteTag(tag.ID)
	require.NoError(t, err)

	// 7. 删除后传输不再受限
	resp, err = svc.CheckTransfer(CheckRequest{
		ResourcePath: "/data/gdpr-sensitive/user-data.csv",
		Action:       ActionCopy,
		TargetRegion: RegionCN,
		User:         "user1",
	})
	require.NoError(t, err)
	assert.True(t, resp.Allowed)
}
