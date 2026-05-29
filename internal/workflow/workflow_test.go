package workflow

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTest(t *testing.T) (*Manager, *gin.Engine) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	tmpDir := t.TempDir()
	mgr := NewManager(filepath.Join(tmpDir, "data.json"))
	require.NoError(t, mgr.Initialize())
	r := gin.New()
	grp := r.Group("")
	NewHandlers(mgr).RegisterRoutes(grp)
	return mgr, r
}

func TestCreateAndGetWorkflow(t *testing.T) {
	_, r := setupTest(t)

	wf := Workflow{
		ID:   "wf-test-1",
		Name: "测试工作流",
		Nodes: []Node{
			{ID: "start", Name: "开始", Type: NodeStart},
			{ID: "task", Name: "任务", Type: NodeScript, Config: map[string]string{"script": "echo hello"}},
			{ID: "end", Name: "结束", Type: NodeEnd},
		},
		Edges: []Edge{
			{From: "start", To: "task"},
			{From: "task", To: "end"},
		},
	}
	body, _ := json.Marshal(wf)
	req := httptest.NewRequest(http.MethodPost, "/workflow", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	req2 := httptest.NewRequest(http.MethodGet, "/workflow/wf-test-1", nil)
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)
	assert.Equal(t, http.StatusOK, w2.Code)
	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w2.Body.Bytes(), &resp))
	data := resp["data"].(map[string]interface{})
	assert.Equal(t, "测试工作流", data["name"])
}

func TestListWorkflows(t *testing.T) {
	mgr, r := setupTest(t)

	wf := &Workflow{
		ID:   "wf-list-1",
		Name: "列表测试",
		Nodes: []Node{
			{ID: "start", Name: "开始", Type: NodeStart},
			{ID: "end", Name: "结束", Type: NodeEnd},
		},
		Edges: []Edge{
			{From: "start", To: "end"},
		},
	}
	require.NoError(t, mgr.CreateWorkflow(wf))

	req := httptest.NewRequest(http.MethodGet, "/workflow", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, float64(1), resp["total"])
}

func TestDuplicateWorkflow(t *testing.T) {
	mgr, _ := setupTest(t)

	wf := &Workflow{
		ID:   "wf-dup",
		Name: "重复测试",
		Nodes: []Node{
			{ID: "start", Name: "开始", Type: NodeStart},
			{ID: "end", Name: "结束", Type: NodeEnd},
		},
		Edges: []Edge{
			{From: "start", To: "end"},
		},
	}
	require.NoError(t, mgr.CreateWorkflow(wf))
	err := mgr.CreateWorkflow(wf)
	assert.ErrorIs(t, err, ErrWorkflowExists)
}

func TestWorkflowLifecycle(t *testing.T) {
	mgr, r := setupTest(t)

	wf := &Workflow{
		ID:   "wf-lifecycle",
		Name: "生命周期测试",
		Nodes: []Node{
			{ID: "start", Name: "开始", Type: NodeStart},
			{ID: "end", Name: "结束", Type: NodeEnd},
		},
		Edges: []Edge{
			{From: "start", To: "end"},
		},
	}
	require.NoError(t, mgr.CreateWorkflow(wf))
	assert.Equal(t, WfDraft, wf.Status)

	req := httptest.NewRequest(http.MethodPost, "/workflow/wf-lifecycle/enable", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	updated, _ := mgr.GetWorkflow("wf-lifecycle")
	assert.Equal(t, WfActive, updated.Status)

	req2 := httptest.NewRequest(http.MethodPost, "/workflow/wf-lifecycle/disable", nil)
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)
	assert.Equal(t, http.StatusOK, w2.Code)

	updated, _ = mgr.GetWorkflow("wf-lifecycle")
	assert.Equal(t, WfDisabled, updated.Status)
}

func TestExecuteWorkflow(t *testing.T) {
	mgr, r := setupTest(t)

	wf := &Workflow{
		ID:   "wf-exec",
		Name: "执行测试",
		Nodes: []Node{
			{ID: "start", Name: "开始", Type: NodeStart},
			{ID: "task", Name: "任务", Type: NodeScript, Config: map[string]string{"script": "echo hello"}},
			{ID: "end", Name: "结束", Type: NodeEnd},
		},
		Edges: []Edge{
			{From: "start", To: "task"},
			{From: "task", To: "end"},
		},
	}
	require.NoError(t, mgr.CreateWorkflow(wf))
	require.NoError(t, mgr.EnableWorkflow("wf-exec"))

	reqBody := map[string]string{"trigger_type": "manual", "input": "test input"}
	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/workflow/wf-exec/execute", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	data := resp["data"].(map[string]interface{})
	assert.Equal(t, "success", data["status"])
}

func TestExecuteDisabledWorkflow(t *testing.T) {
	mgr, _ := setupTest(t)

	wf := &Workflow{
		ID:   "wf-disabled",
		Name: "禁用测试",
		Nodes: []Node{
			{ID: "start", Name: "开始", Type: NodeStart},
			{ID: "end", Name: "结束", Type: NodeEnd},
		},
		Edges: []Edge{
			{From: "start", To: "end"},
		},
	}
	require.NoError(t, mgr.CreateWorkflow(wf))
	require.NoError(t, mgr.DisableWorkflow("wf-disabled"))

	_, err := mgr.ExecuteWorkflow("wf-disabled", TriggerManual, "")
	assert.ErrorIs(t, err, ErrWorkflowDisabled)
}

func TestVersionAndRollback(t *testing.T) {
	mgr, r := setupTest(t)

	wf := &Workflow{
		ID:   "wf-version",
		Name: "版本测试",
		Nodes: []Node{
			{ID: "start", Name: "开始", Type: NodeStart},
			{ID: "end", Name: "结束", Type: NodeEnd},
		},
		Edges: []Edge{
			{From: "start", To: "end"},
		},
	}
	require.NoError(t, mgr.CreateWorkflow(wf))
	assert.Equal(t, 1, wf.Version)

	// 更新工作流
	updatedWf := &Workflow{
		ID:   "wf-version",
		Name: "版本测试 v2",
		Nodes: []Node{
			{ID: "start", Name: "开始", Type: NodeStart},
			{ID: "new", Name: "新节点", Type: NodeNotify, Config: map[string]string{"title": "test"}},
			{ID: "end", Name: "结束", Type: NodeEnd},
		},
		Edges: []Edge{
			{From: "start", To: "new"},
			{From: "new", To: "end"},
		},
	}
	require.NoError(t, mgr.UpdateWorkflow(updatedWf))
	assert.Equal(t, 2, updatedWf.Version)

	// 获取版本历史
	req := httptest.NewRequest(http.MethodGet, "/workflow/wf-version/versions", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	// 回滚到版本1
	req2 := httptest.NewRequest(http.MethodPost, "/workflow/wf-version/rollback/1", nil)
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)
	assert.Equal(t, http.StatusOK, w2.Code)

	rolledBack, _ := mgr.GetWorkflow("wf-version")
	assert.Equal(t, 3, rolledBack.Version)
	assert.Equal(t, 2, len(rolledBack.Nodes)) // 回滚后只有start和end
}

func TestDAGValidation(t *testing.T) {
	mgr, _ := setupTest(t)

	// 测试无开始节点
	wf1 := &Workflow{
		ID:   "wf-no-start",
		Name: "无开始节点",
		Nodes: []Node{
			{ID: "end", Name: "结束", Type: NodeEnd},
		},
	}
	err := mgr.CreateWorkflow(wf1)
	assert.ErrorIs(t, err, ErrNoStartNode)

	// 测试无结束节点
	wf2 := &Workflow{
		ID:   "wf-no-end",
		Name: "无结束节点",
		Nodes: []Node{
			{ID: "start", Name: "开始", Type: NodeStart},
		},
	}
	err = mgr.CreateWorkflow(wf2)
	assert.ErrorIs(t, err, ErrNoEndNode)

	// 测试环
	wf3 := &Workflow{
		ID:   "wf-cycle",
		Name: "环测试",
		Nodes: []Node{
			{ID: "start", Name: "开始", Type: NodeStart},
			{ID: "a", Name: "A", Type: NodeScript},
			{ID: "b", Name: "B", Type: NodeScript},
			{ID: "end", Name: "结束", Type: NodeEnd},
		},
		Edges: []Edge{
			{From: "start", To: "a"},
			{From: "a", To: "b"},
			{From: "b", To: "a"}, // 形成环
			{From: "b", To: "end"},
		},
	}
	err = mgr.CreateWorkflow(wf3)
	assert.ErrorIs(t, err, ErrInvalidDAG)
}

func TestTemplates(t *testing.T) {
	_, r := setupTest(t)

	req := httptest.NewRequest(http.MethodGet, "/workflow/templates", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.True(t, resp["total"].(float64) > 0)
}

func TestCreateFromTemplate(t *testing.T) {
	_, r := setupTest(t)

	reqBody := map[string]string{"name": "我的备份工作流"}
	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/workflow/templates/backup-to-cloud/create", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	data := resp["data"].(map[string]interface{})
	assert.Equal(t, "我的备份工作流", data["name"])
}

func TestStats(t *testing.T) {
	mgr, r := setupTest(t)

	wf := &Workflow{
		ID:   "wf-stats",
		Name: "统计测试",
		Nodes: []Node{
			{ID: "start", Name: "开始", Type: NodeStart},
			{ID: "end", Name: "结束", Type: NodeEnd},
		},
		Edges: []Edge{
			{From: "start", To: "end"},
		},
	}
	require.NoError(t, mgr.CreateWorkflow(wf))
	require.NoError(t, mgr.EnableWorkflow("wf-stats"))

	req := httptest.NewRequest(http.MethodGet, "/workflow/stats", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	data := resp["data"].(map[string]interface{})
	assert.Equal(t, float64(1), data["total_workflows"])
	assert.Equal(t, float64(1), data["active_workflows"])
	assert.True(t, data["total_templates"].(float64) > 0)
}

func TestDeleteWorkflow(t *testing.T) {
	mgr, _ := setupTest(t)

	wf := &Workflow{
		ID:   "wf-delete",
		Name: "删除测试",
		Nodes: []Node{
			{ID: "start", Name: "开始", Type: NodeStart},
			{ID: "end", Name: "结束", Type: NodeEnd},
		},
		Edges: []Edge{
			{From: "start", To: "end"},
		},
	}
	require.NoError(t, mgr.CreateWorkflow(wf))
	require.NoError(t, mgr.DeleteWorkflow("wf-delete"))

	_, err := mgr.GetWorkflow("wf-delete")
	assert.ErrorIs(t, err, ErrWorkflowNotFound)
}

func TestTopologicalSort(t *testing.T) {
	nodes := []Node{
		{ID: "start", Type: NodeStart},
		{ID: "a", Type: NodeScript},
		{ID: "b", Type: NodeScript},
		{ID: "end", Type: NodeEnd},
	}
	edges := []Edge{
		{From: "start", To: "a"},
		{From: "start", To: "b"},
		{From: "a", To: "end"},
		{From: "b", To: "end"},
	}

	order, err := topologicalSort(nodes, edges)
	require.NoError(t, err)
	assert.Equal(t, 4, len(order))
	assert.Equal(t, "start", order[0])
	assert.Equal(t, "end", order[3])
}
