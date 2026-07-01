package sharedlabels

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ========== 标签创建测试 ==========

func TestCreateLabel(t *testing.T) {
	svc := NewService()
	ctx := context.Background()

	req := CreateLabelRequest{
		Name:      "重要文件",
		Type:      LabelTypeUser,
		Color:     ColorRed,
		CreatedBy: "admin",
		IsPublic:  true,
		TenantID:  "tenant-1",
	}

	label, err := svc.CreateLabel(ctx, req)
	require.NoError(t, err)
	assert.NotEmpty(t, label.ID)
	assert.Equal(t, "重要文件", label.Name)
	assert.Equal(t, LabelTypeUser, label.Type)
	assert.Equal(t, ColorRed, label.Color)
	assert.True(t, label.IsPublic)
	assert.Equal(t, "tenant-1", label.TenantID)
	assert.False(t, label.CreatedAt.IsZero())
}

func TestCreateLabel_DuplicateName(t *testing.T) {
	svc := NewService()
	ctx := context.Background()

	req := CreateLabelRequest{
		Name:      "测试标签",
		Type:      LabelTypeUser,
		CreatedBy: "admin",
		TenantID:  "tenant-1",
	}

	_, err := svc.CreateLabel(ctx, req)
	require.NoError(t, err)

	// 同名标签应失败
	_, err = svc.CreateLabel(ctx, req)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "已存在")
}

func TestCreateLabel_DifferentTenant(t *testing.T) {
	svc := NewService()
	ctx := context.Background()

	req1 := CreateLabelRequest{
		Name: "标签A", Type: LabelTypeUser, CreatedBy: "u1", TenantID: "t1",
	}
	req2 := CreateLabelRequest{
		Name: "标签A", Type: LabelTypeUser, CreatedBy: "u2", TenantID: "t2",
	}

	_, err := svc.CreateLabel(ctx, req1)
	require.NoError(t, err)

	// 不同租户可以同名
	_, err = svc.CreateLabel(ctx, req2)
	assert.NoError(t, err)
}

// ========== 标签列表测试 ==========

func TestListLabels(t *testing.T) {
	svc := NewService()
	ctx := context.Background()

	svc.CreateLabel(ctx, CreateLabelRequest{Name: "L1", Type: LabelTypeUser, CreatedBy: "u1", TenantID: "t1"})
	svc.CreateLabel(ctx, CreateLabelRequest{Name: "L2", Type: LabelTypeTeam, CreatedBy: "u1", TenantID: "t1"})
	svc.CreateLabel(ctx, CreateLabelRequest{Name: "L3", Type: LabelTypeUser, CreatedBy: "u2", TenantID: "t2"})

	// 全部
	all, err := svc.ListLabels(ctx, "", "")
	require.NoError(t, err)
	assert.Len(t, all, 3)

	// 按类型过滤
	teamLabels, _ := svc.ListLabels(ctx, LabelTypeTeam, "")
	assert.Len(t, teamLabels, 1)
	assert.Equal(t, "L2", teamLabels[0].Name)

	// 按租户过滤
	t1Labels, _ := svc.ListLabels(ctx, "", "t1")
	assert.Len(t, t1Labels, 2)

	// 组合过滤
	combined, _ := svc.ListLabels(ctx, LabelTypeUser, "t1")
	assert.Len(t, combined, 1)
	assert.Equal(t, "L1", combined[0].Name)
}

// ========== 标签分配测试 ==========

func TestAssignLabels(t *testing.T) {
	svc := NewService()
	ctx := context.Background()

	label, _ := svc.CreateLabel(ctx, CreateLabelRequest{
		Name: "项目A", Type: LabelTypeTeam, CreatedBy: "admin",
	})
	label2, _ := svc.CreateLabel(ctx, CreateLabelRequest{
		Name: "紧急", Type: LabelTypeUser, CreatedBy: "admin",
	})

	req := AssignLabelRequest{
		FileID:   "file-001",
		FilePath: "/shared/project-a/report.pdf",
		LabelIDs: []string{label.ID, label2.ID},
		AppliedBy: "user1",
	}

	result, err := svc.AssignLabels(ctx, req)
	require.NoError(t, err)
	assert.Len(t, result, 2)

	// 验证 FileCount 增加
	updated, _ := svc.GetLabel(ctx, label.ID)
	assert.Equal(t, 1, updated.FileCount)
}

func TestAssignLabels_NonExistentLabel(t *testing.T) {
	svc := NewService()
	ctx := context.Background()

	req := AssignLabelRequest{
		FileID:   "file-002",
		FilePath: "/test/file.txt",
		LabelIDs: []string{"nonexistent"},
		AppliedBy: "user1",
	}

	_, err := svc.AssignLabels(ctx, req)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "标签不存在")
}

func TestAssignLabels_DuplicateAssignment(t *testing.T) {
	svc := NewService()
	ctx := context.Background()

	label, _ := svc.CreateLabel(ctx, CreateLabelRequest{
		Name: "标签X", Type: LabelTypeUser, CreatedBy: "admin",
	})

	req := AssignLabelRequest{
		FileID: "file-003", FilePath: "/test.txt",
		LabelIDs: []string{label.ID}, AppliedBy: "u1",
	}

	// 第一次分配
	_, err := svc.AssignLabels(ctx, req)
	require.NoError(t, err)

	// 第二次分配同一标签不应增加 FileCount
	_, err = svc.AssignLabels(ctx, req)
	require.NoError(t, err)

	updated, _ := svc.GetLabel(ctx, label.ID)
	// FileCount 应该保持为 1（跳过已存在的）
	// 注意：当前实现中 continue 跳过已存在但 FileCount 仍会+1
	// 这里验证不重复添加文件到 labelFiles
	assert.Equal(t, 1, updated.FileCount) // 修正：第二次不应增加
}

// ========== 标签移除测试 ==========

func TestRemoveLabels(t *testing.T) {
	svc := NewService()
	ctx := context.Background()

	label, _ := svc.CreateLabel(ctx, CreateLabelRequest{
		Name: "待删除", Type: LabelTypeUser, CreatedBy: "admin",
	})

	svc.AssignLabels(ctx, AssignLabelRequest{
		FileID: "file-010", FilePath: "/dir/file.txt",
		LabelIDs: []string{label.ID}, AppliedBy: "u1",
	})

	// 验证已分配
	fls, _ := svc.GetFileLabels(ctx, "file-010")
	assert.Len(t, fls, 1)

	// 移除
	err := svc.RemoveLabels(ctx, RemoveLabelRequest{
		FileID: "file-010", LabelIDs: []string{label.ID},
	})
	require.NoError(t, err)

	// 验证已移除
	fls, _ = svc.GetFileLabels(ctx, "file-010")
	assert.Empty(t, fls)

	// FileCount 应减少
	updated, _ := svc.GetLabel(ctx, label.ID)
	assert.Equal(t, 0, updated.FileCount)
}

// ========== 搜索测试 ==========

func TestSearchByLabels(t *testing.T) {
	svc := NewService()
	ctx := context.Background()

	label1, _ := svc.CreateLabel(ctx, CreateLabelRequest{Name: "A", Type: LabelTypeUser, CreatedBy: "u"})
	label2, _ := svc.CreateLabel(ctx, CreateLabelRequest{Name: "B", Type: LabelTypeUser, CreatedBy: "u"})

	svc.AssignLabels(ctx, AssignLabelRequest{
		FileID: "f1", FilePath: "/a.txt", LabelIDs: []string{label1.ID}, AppliedBy: "u",
	})
	svc.AssignLabels(ctx, AssignLabelRequest{
		FileID: "f2", FilePath: "/b.txt", LabelIDs: []string{label2.ID}, AppliedBy: "u",
	})
	svc.AssignLabels(ctx, AssignLabelRequest{
		FileID: "f3", FilePath: "/c.txt", LabelIDs: []string{label1.ID, label2.ID}, AppliedBy: "u",
	})

	// 按标签1搜索，应返回 f1 和 f3
	results, err := svc.SearchByLabels(ctx, []string{label1.ID})
	require.NoError(t, err)
	assert.Len(t, results, 2)

	// 按标签1和2搜索（OR语义），应返回 f1, f2, f3
	results, err = svc.SearchByLabels(ctx, []string{label1.ID, label2.ID})
	require.NoError(t, err)
	assert.Len(t, results, 3)

	// 无标签过滤，返回所有
	results, err = svc.SearchByLabels(ctx, nil)
	require.NoError(t, err)
	assert.Len(t, results, 3)
}

// ========== 统计测试 ==========

func TestGetStats(t *testing.T) {
	svc := NewService()
	ctx := context.Background()

	svc.CreateLabel(ctx, CreateLabelRequest{Name: "S1", Type: LabelTypeUser, CreatedBy: "u", TenantID: "t1"})
	svc.CreateLabel(ctx, CreateLabelRequest{Name: "S2", Type: LabelTypeTeam, CreatedBy: "u", TenantID: "t1"})
	svc.CreateLabel(ctx, CreateLabelRequest{Name: "S3", Type: LabelTypeSystem, CreatedBy: "u", TenantID: "t2"})

	label, _ := svc.CreateLabel(ctx, CreateLabelRequest{Name: "S4", Type: LabelTypeUser, CreatedBy: "u", TenantID: "t1"})

	svc.AssignLabels(ctx, AssignLabelRequest{
		FileID: "f1", FilePath: "/a.txt", LabelIDs: []string{label.ID}, AppliedBy: "u",
	})

	stats, err := svc.GetStats(ctx, "t1")
	require.NoError(t, err)
	assert.Equal(t, 3, stats.TotalLabels)
	assert.Equal(t, 2, stats.LabelsByType["user"])
	assert.Equal(t, 1, stats.LabelsByType["team"])
	assert.Equal(t, 1, stats.TotalFiles)
}

// ========== 删除标签测试 ==========

func TestDeleteLabel(t *testing.T) {
	svc := NewService()
	ctx := context.Background()

	label, _ := svc.CreateLabel(ctx, CreateLabelRequest{
		Name: "待删", Type: LabelTypeUser, CreatedBy: "admin",
	})

	svc.AssignLabels(ctx, AssignLabelRequest{
		FileID: "f1", FilePath: "/a.txt", LabelIDs: []string{label.ID}, AppliedBy: "u",
	})

	err := svc.DeleteLabel(ctx, label.ID)
	require.NoError(t, err)

	// 标签已删除
	_, err = svc.GetLabel(ctx, label.ID)
	assert.Error(t, err)

	// 文件的标签关联已清理
	fls, _ := svc.GetFileLabels(ctx, "f1")
	assert.Empty(t, fls)
}

func TestDeleteLabel_NotFound(t *testing.T) {
	svc := NewService()
	ctx := context.Background()

	err := svc.DeleteLabel(ctx, "nonexistent")
	assert.Error(t, err)
}

// ========== 更新标签测试 ==========

func TestUpdateLabel(t *testing.T) {
	svc := NewService()
	ctx := context.Background()

	label, _ := svc.CreateLabel(ctx, CreateLabelRequest{
		Name: "原名", Type: LabelTypeUser, CreatedBy: "admin", Color: ColorBlue,
	})

	updated, err := svc.UpdateLabel(ctx, label.ID, "新名", "新描述", ColorGreen)
	require.NoError(t, err)
	assert.Equal(t, "新名", updated.Name)
	assert.Equal(t, "新描述", updated.Description)
	assert.Equal(t, ColorGreen, updated.Color)
}

// ========== GetLabelFiles 测试 ==========

func TestGetLabelFiles(t *testing.T) {
	svc := NewService()
	ctx := context.Background()

	label, _ := svc.CreateLabel(ctx, CreateLabelRequest{
		Name: "L", Type: LabelTypeTeam, CreatedBy: "admin",
	})

	svc.AssignLabels(ctx, AssignLabelRequest{
		FileID: "f1", FilePath: "/a.txt", LabelIDs: []string{label.ID}, AppliedBy: "u",
	})
	svc.AssignLabels(ctx, AssignLabelRequest{
		FileID: "f2", FilePath: "/b.txt", LabelIDs: []string{label.ID}, AppliedBy: "u",
	})

	files, err := svc.GetLabelFiles(ctx, label.ID)
	require.NoError(t, err)
	assert.Len(t, files, 2)
}
