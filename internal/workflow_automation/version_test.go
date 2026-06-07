// Package workflow_automation 提供工作流自动化引擎
package workflow_automation

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestNewVersionManager(t *testing.T) {
	vm := NewVersionManager(nil, nil)

	assert.NotNil(t, vm)
	assert.NotNil(t, vm.versions)
}

func TestVersionManagerSaveVersion(t *testing.T) {
	vm := NewVersionManager(nil, nil)

	wf := &Workflow{
		ID:      "wf-1",
		Name:    "Test",
		Version: 1,
	}

	err := vm.SaveVersion("wf-1", wf, "initial version", "tester")
	require.NoError(t, err)

	versions, err := vm.GetVersions("wf-1")
	require.NoError(t, err)
	require.Len(t, versions, 1)

	assert.Equal(t, "wf-1", versions[0].WorkflowID)
	assert.Equal(t, 1, versions[0].Version)
	assert.Equal(t, "initial version", versions[0].Comment)
	assert.Equal(t, "tester", versions[0].CreatedBy)
	assert.NotNil(t, versions[0].Snapshot)
	assert.False(t, versions[0].CreatedAt.IsZero())
}

func TestVersionManagerSaveMultipleVersions(t *testing.T) {
	vm := NewVersionManager(nil, nil)

	for i := 1; i <= 5; i++ {
		wf := &Workflow{
			ID:      "wf-1",
			Name:    "Test",
			Version: i,
		}
		vm.SaveVersion("wf-1", wf, "version", "tester")
	}

	versions, err := vm.GetVersions("wf-1")
	require.NoError(t, err)
	assert.Len(t, versions, 5)
}

func TestVersionManagerGetVersion(t *testing.T) {
	vm := NewVersionManager(nil, nil)

	wf1 := &Workflow{ID: "wf-1", Name: "v1", Version: 1}
	wf2 := &Workflow{ID: "wf-1", Name: "v2", Version: 2}

	vm.SaveVersion("wf-1", wf1, "v1", "")
	vm.SaveVersion("wf-1", wf2, "v2", "")

	v, err := vm.GetVersion("wf-1", 1)
	require.NoError(t, err)
	assert.Equal(t, 1, v.Version)
	assert.Equal(t, "v1", v.Snapshot.Name)

	v, err = vm.GetVersion("wf-1", 2)
	require.NoError(t, err)
	assert.Equal(t, 2, v.Version)
	assert.Equal(t, "v2", v.Snapshot.Name)

	// 不存在的版本
	_, err = vm.GetVersion("wf-1", 99)
	assert.Error(t, err)
}

func TestVersionManagerGetLatestVersion(t *testing.T) {
	vm := NewVersionManager(nil, nil)

	for i := 1; i <= 3; i++ {
		wf := &Workflow{ID: "wf-1", Name: "Test", Version: i}
		vm.SaveVersion("wf-1", wf, "", "")
	}

	latest, err := vm.GetLatestVersion("wf-1")
	require.NoError(t, err)
	assert.Equal(t, 3, latest.Version)
}

func TestVersionManagerGetLatestVersionEmpty(t *testing.T) {
	vm := NewVersionManager(nil, nil)

	_, err := vm.GetLatestVersion("nonexistent")
	assert.Error(t, err)
}

func TestVersionManagerRollback(t *testing.T) {
	engine := NewEngine(nil, nil)
	vm := NewVersionManager(nil, nil)

	// 创建工作流
	wf := &Workflow{
		Name:    "Original",
		Version: 1,
		Status:  StatusActive,
	}
	engine.CreateWorkflow(wf)
	originalVersion := wf.Version

	// 保存版本
	vm.SaveVersion(wf.ID, wf, "original", "")

	// 更新工作流
	wf.Name = "Modified"
	engine.UpdateWorkflow(wf)

	// 回滚到版本 1
	err := vm.Rollback(engine, wf.ID, originalVersion)
	require.NoError(t, err)

	// 验证回滚
	rolled, _ := engine.GetWorkflow(wf.ID)
	assert.Equal(t, "Original", rolled.Name)
	assert.True(t, rolled.Version > originalVersion)
}

func TestVersionManagerRollbackNotFound(t *testing.T) {
	engine := NewEngine(nil, nil)
	vm := NewVersionManager(nil, nil)

	err := vm.Rollback(engine, "nonexistent", 1)
	assert.Error(t, err)
}

func TestVersionManagerDiffVersions(t *testing.T) {
	vm := NewVersionManager(nil, nil)

	// 版本 1
	wf1 := &Workflow{
		ID:      "wf-1",
		Name:    "Test",
		Version: 1,
		Nodes: map[string]*Node{
			"start": {ID: "start", Type: NodeTypeStart, Name: "Start"},
			"end":   {ID: "end", Type: NodeTypeEnd, Name: "End"},
			"n1":    {ID: "n1", Type: NodeTypeAction, Name: "Action 1"},
		},
		Edges: []*Edge{
			{From: "start", To: "n1"},
			{From: "n1", To: "end"},
		},
		Status: StatusDraft,
	}
	vm.SaveVersion("wf-1", wf1, "v1", "")

	// 版本 2（添加了节点和边）
	wf2 := &Workflow{
		ID:      "wf-1",
		Name:    "Test",
		Version: 2,
		Nodes: map[string]*Node{
			"start": {ID: "start", Type: NodeTypeStart, Name: "Start"},
			"end":   {ID: "end", Type: NodeTypeEnd, Name: "End"},
			"n1":    {ID: "n1", Type: NodeTypeAction, Name: "Action 1"},
			"n2":    {ID: "n2", Type: NodeTypeAction, Name: "Action 2"},
		},
		Edges: []*Edge{
			{From: "start", To: "n1"},
			{From: "n1", To: "n2"},
			{From: "n2", To: "end"},
		},
		Status: StatusActive,
	}
	vm.SaveVersion("wf-1", wf2, "v2", "")

	diff, err := vm.DiffVersions("wf-1", 1, 2)
	require.NoError(t, err)

	assert.Equal(t, "wf-1", diff.WorkflowID)
	assert.Equal(t, 1, diff.Version1)
	assert.Equal(t, 2, diff.Version2)
	assert.Contains(t, diff.AddedNodes, "n2")
	assert.Equal(t, "draft -> active", diff.StatusChange)
}

func TestVersionManagerAutoSave(t *testing.T) {
	engine := NewEngine(nil, nil)
	vm := NewVersionManager(nil, nil)

	wf := &Workflow{Name: "Test", Status: StatusDraft}
	engine.CreateWorkflow(wf)

	err := vm.AutoSave(engine, wf.ID)
	require.NoError(t, err)

	versions, _ := vm.GetVersions(wf.ID)
	assert.Len(t, versions, 1)
	assert.Equal(t, "auto-save", versions[0].Comment)
}

func TestVersionManagerCleanupOldVersions(t *testing.T) {
	vm := NewVersionManager(nil, nil)

	for i := 1; i <= 10; i++ {
		wf := &Workflow{ID: "wf-1", Name: "Test", Version: i}
		vm.SaveVersion("wf-1", wf, "", "")
	}

	removed := vm.CleanupOldVersions("wf-1", 3)
	assert.Equal(t, 7, removed)

	versions, _ := vm.GetVersions("wf-1")
	assert.Len(t, versions, 3)
}

func TestVersionManagerCleanupNoRemovalNeeded(t *testing.T) {
	vm := NewVersionManager(nil, nil)

	for i := 1; i <= 3; i++ {
		wf := &Workflow{ID: "wf-1", Name: "Test", Version: i}
		vm.SaveVersion("wf-1", wf, "", "")
	}

	removed := vm.CleanupOldVersions("wf-1", 5)
	assert.Equal(t, 0, removed)
}

func TestCloneWorkflow(t *testing.T) {
	original := &Workflow{
		ID:          "wf-1",
		Name:        "Original",
		Description: "Test",
		Version:     1,
		Status:      StatusActive,
		Nodes: map[string]*Node{
			"start": {
				ID:       "start",
				Type:     NodeTypeStart,
				Name:     "Start",
				Config:   map[string]string{"key": "value"},
				Position: &Position{X: 10, Y: 20},
			},
		},
		Edges: []*Edge{
			{ID: "e1", From: "start", To: "end"},
		},
		Variables: map[string]string{"env": "test"},
		Labels:    map[string]string{"team": "devops"},
	}

	clone := cloneWorkflow(original)

	// 验证深拷贝
	assert.Equal(t, original.ID, clone.ID)
	assert.Equal(t, original.Name, clone.Name)
	assert.Equal(t, original.Version, clone.Version)

	// 修改克隆不应影响原始
	clone.Name = "Modified"
	assert.Equal(t, "Original", original.Name)

	// 修改节点配置不应影响原始
	clone.Nodes["start"].Config["key"] = "modified"
	assert.Equal(t, "value", original.Nodes["start"].Config["key"])

	// 修改变量不应影响原始
	clone.Variables["env"] = "modified"
	assert.Equal(t, "test", original.Variables["env"])
}

func TestCreateAutoSaveVersion(t *testing.T) {
	wf := &Workflow{
		ID:      "wf-1",
		Name:    "Test",
		Version: 2,
	}

	version := CreateAutoSaveVersion(wf)

	assert.Equal(t, "wf-1", version.WorkflowID)
	assert.Equal(t, 2, version.Version)
	assert.Equal(t, "auto-save", version.Comment)
	assert.NotNil(t, version.Snapshot)
	assert.False(t, version.CreatedAt.IsZero())

	// 验证是深拷贝
	version.Snapshot.Name = "Modified"
	assert.Equal(t, "Test", wf.Name)
}

func TestNewWorkflowVersionID(t *testing.T) {
	id1 := NewWorkflowVersionID()
	id2 := NewWorkflowVersionID()

	assert.NotEmpty(t, id1)
	assert.NotEmpty(t, id2)
	assert.NotEqual(t, id1, id2)
}

func TestVersionManagerWithLogger(t *testing.T) {
	logger := zap.NewNop()
	vm := NewVersionManager(nil, logger)

	assert.Equal(t, logger, vm.logger)
}

func TestVersionManagerDiffVersionsNotFound(t *testing.T) {
	vm := NewVersionManager(nil, nil)

	_, err := vm.DiffVersions("nonexistent", 1, 2)
	assert.Error(t, err)
}
