package datapipeline

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

func testPipeline(id string) *Pipeline {
	return &Pipeline{
		ID:          id,
		Name:        "Test Pipeline",
		Description: "测试管道",
		Source: DataSource{
			ID:   "src-1",
			Type: SourceFile,
			Name: "test-source",
			Config: map[string]string{
				"path": "/tmp/test.csv",
			},
		},
		Transforms: []TransformStep{
			{
				ID:      "t1",
				Type:    TransformFilter,
				Name:    "过滤器",
				Config:  map[string]string{"field": "status", "op": "eq", "value": "active"},
				Enabled: true,
			},
			{
				ID:      "t2",
				Type:    TransformMap,
				Name:    "映射器",
				Config:  map[string]string{"mapping": "name->full_name"},
				Enabled: true,
			},
		},
		Sink: DataSink{
			ID:   "sink-1",
			Type: SourceFile,
			Name: "test-output",
			Config: map[string]string{
				"path": "/tmp/output.csv",
			},
		},
		Schedule: Schedule{
			Type:    ScheduleManual,
			Enabled: true,
		},
		Tags: []string{"test", "etl"},
	}
}

func TestCreateAndListPipelines(t *testing.T) {
	_, r := setupTest(t)

	p := testPipeline("pipe-1")
	body, _ := json.Marshal(p)
	req := httptest.NewRequest(http.MethodPost, "/datapipeline/pipelines", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	req2 := httptest.NewRequest(http.MethodGet, "/datapipeline/pipelines", nil)
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)
	assert.Equal(t, http.StatusOK, w2.Code)
	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w2.Body.Bytes(), &resp))
	assert.Equal(t, float64(1), resp["total"])
}

func TestDuplicatePipeline(t *testing.T) {
	mgr, _ := setupTest(t)
	require.NoError(t, mgr.CreatePipeline(testPipeline("dup-1")))
	err := mgr.CreatePipeline(testPipeline("dup-1"))
	assert.ErrorIs(t, err, ErrPipelineExists)
}

func TestPipelineNotFound(t *testing.T) {
	mgr, _ := setupTest(t)
	_, err := mgr.GetPipeline("nonexistent")
	assert.ErrorIs(t, err, ErrPipelineNotFound)
}

func TestPipelineLifecycle(t *testing.T) {
	mgr, r := setupTest(t)
	require.NoError(t, mgr.CreatePipeline(testPipeline("pipe-lc")))

	// 启动
	req := httptest.NewRequest(http.MethodPost, "/datapipeline/pipelines/pipe-lc/start", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	p, _ := mgr.GetPipeline("pipe-lc")
	assert.Equal(t, PipelineRunning, p.Status)

	// 不能重复启动
	err := mgr.StartPipeline("pipe-lc")
	assert.ErrorIs(t, err, ErrPipelineRunning)

	// 停止
	req2 := httptest.NewRequest(http.MethodPost, "/datapipeline/pipelines/pipe-lc/stop", nil)
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)
	assert.Equal(t, http.StatusOK, w2.Code)

	p, _ = mgr.GetPipeline("pipe-lc")
	assert.Equal(t, PipelinePaused, p.Status)

	// 不能停止已停止的
	err = mgr.StopPipeline("pipe-lc")
	assert.ErrorIs(t, err, ErrPipelineNotRunning)
}

func TestDeleteRunningPipeline(t *testing.T) {
	mgr, _ := setupTest(t)
	require.NoError(t, mgr.CreatePipeline(testPipeline("pipe-del")))
	require.NoError(t, mgr.StartPipeline("pipe-del"))

	err := mgr.DeletePipeline("pipe-del")
	assert.ErrorIs(t, err, ErrPipelineRunning)

	require.NoError(t, mgr.StopPipeline("pipe-del"))
	require.NoError(t, mgr.DeletePipeline("pipe-del"))

	_, err = mgr.GetPipeline("pipe-del")
	assert.ErrorIs(t, err, ErrPipelineNotFound)
}

func TestTriggerExecution(t *testing.T) {
	mgr, _ := setupTest(t)
	require.NoError(t, mgr.CreatePipeline(testPipeline("pipe-trig")))

	exec, err := mgr.TriggerExecution("pipe-trig")
	require.NoError(t, err)
	assert.NotEmpty(t, exec.ID)
	assert.Equal(t, "manual", exec.Trigger)

	execs := mgr.GetExecutions("pipe-trig", 10)
	assert.Len(t, execs, 1)
}

func TestTriggerExecutionNotFound(t *testing.T) {
	mgr, _ := setupTest(t)
	_, err := mgr.TriggerExecution("nonexistent")
	assert.ErrorIs(t, err, ErrPipelineNotFound)
}

func TestUpdatePipeline(t *testing.T) {
	mgr, r := setupTest(t)
	require.NoError(t, mgr.CreatePipeline(testPipeline("pipe-upd")))

	update := Pipeline{Name: "Updated Name", Description: "新描述"}
	body, _ := json.Marshal(update)
	req := httptest.NewRequest(http.MethodPut, "/datapipeline/pipelines/pipe-upd", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	p, _ := mgr.GetPipeline("pipe-upd")
	assert.Equal(t, "Updated Name", p.Name)
	assert.Equal(t, "新描述", p.Description)
}

func TestUpdateRunningPipeline(t *testing.T) {
	mgr, _ := setupTest(t)
	require.NoError(t, mgr.CreatePipeline(testPipeline("pipe-updr")))
	require.NoError(t, mgr.StartPipeline("pipe-updr"))

	err := mgr.UpdatePipeline("pipe-updr", &Pipeline{Name: "Nope"})
	assert.ErrorIs(t, err, ErrPipelineRunning)
}

func TestListPipelinesWithFilter(t *testing.T) {
	mgr, _ := setupTest(t)
	require.NoError(t, mgr.CreatePipeline(testPipeline("pipe-f1")))
	require.NoError(t, mgr.CreatePipeline(testPipeline("pipe-f2")))
	require.NoError(t, mgr.StartPipeline("pipe-f1"))

	// 按状态过滤
	running := mgr.ListPipelines(PipelineRunning, "")
	assert.Len(t, running, 1)
	assert.Equal(t, "pipe-f1", running[0].ID)

	// 按标签过滤
	tagged := mgr.ListPipelines("", "etl")
	assert.Len(t, tagged, 2)
}

func TestInvalidSourceType(t *testing.T) {
	mgr, _ := setupTest(t)
	p := testPipeline("pipe-inv")
	p.Source.Type = "invalid"
	err := mgr.CreatePipeline(p)
	assert.ErrorIs(t, err, ErrInvalidSource)
}

func TestInvalidTransformType(t *testing.T) {
	mgr, _ := setupTest(t)
	p := testPipeline("pipe-invt")
	p.Transforms = []TransformStep{
		{ID: "t1", Type: "bad_transform", Name: "bad"},
	}
	err := mgr.CreatePipeline(p)
	assert.ErrorIs(t, err, ErrInvalidTransform)
}

func TestInvalidScheduleType(t *testing.T) {
	mgr, _ := setupTest(t)
	p := testPipeline("pipe-invs")
	p.Schedule.Type = "bad_schedule"
	err := mgr.CreatePipeline(p)
	assert.ErrorIs(t, err, ErrInvalidSchedule)
}

func TestDLQOperations(t *testing.T) {
	mgr, _ := setupTest(t)

	// 手动添加DLQ条目
	mgr.addToDLQ("pipe-dlq", "exec-1", "test-data", "test error", 3)

	entries := mgr.GetDLQ("", 10)
	assert.Len(t, entries, 1)
	assert.Equal(t, "test error", entries[0].Error)

	// 按管道过滤
	entries = mgr.GetDLQ("pipe-dlq", 10)
	assert.Len(t, entries, 1)

	entries = mgr.GetDLQ("other-pipe", 10)
	assert.Len(t, entries, 0)

	// 清空
	removed := mgr.ClearDLQ("pipe-dlq")
	assert.Equal(t, 1, removed)

	entries = mgr.GetDLQ("", 10)
	assert.Len(t, entries, 0)
}

func TestStats(t *testing.T) {
	mgr, r := setupTest(t)
	require.NoError(t, mgr.CreatePipeline(testPipeline("pipe-stat")))
	require.NoError(t, mgr.StartPipeline("pipe-stat"))

	req := httptest.NewRequest(http.MethodGet, "/datapipeline/stats", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	data := resp["data"].(map[string]interface{})
	assert.Equal(t, float64(1), data["total_pipelines"])
	assert.Equal(t, float64(1), data["running_pipelines"])
}

func TestPipelineHealth(t *testing.T) {
	mgr, _ := setupTest(t)
	require.NoError(t, mgr.CreatePipeline(testPipeline("pipe-health")))

	health, err := mgr.GetPipelineHealth("pipe-health")
	require.NoError(t, err)
	assert.Equal(t, "pipe-health", health["pipeline_id"])
	assert.Equal(t, PipelineIdle, health["status"])
	assert.Equal(t, 0.0, health["success_rate"])
}

func TestGetExecutionNotFound(t *testing.T) {
	mgr, _ := setupTest(t)
	_, err := mgr.GetExecution("pipe-x", "exec-x")
	assert.ErrorIs(t, err, ErrExecutionNotFound)
}

func TestRetryDLQEntryNotFound(t *testing.T) {
	mgr, _ := setupTest(t)
	err := mgr.RetryDLQEntry("nonexistent")
	assert.ErrorIs(t, err, ErrExecutionNotFound)
}

func TestDLQEndpoints(t *testing.T) {
	mgr, r := setupTest(t)
	mgr.addToDLQ("pipe-dlq2", "exec-dlq", "data", "error", 3)

	// 获取DLQ
	req := httptest.NewRequest(http.MethodGet, "/datapipeline/dlq", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, float64(1), resp["total"])

	// 清空DLQ
	req2 := httptest.NewRequest(http.MethodDelete, "/datapipeline/dlq?pipeline_id=pipe-dlq2", nil)
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)
	assert.Equal(t, http.StatusOK, w2.Code)
}

func TestTriggerEndpoint(t *testing.T) {
	_, r := setupTest(t)

	// 先创建管道
	p := testPipeline("pipe-trigep")
	body, _ := json.Marshal(p)
	req := httptest.NewRequest(http.MethodPost, "/datapipeline/pipelines", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	// 触发执行
	req2 := httptest.NewRequest(http.MethodPost, "/datapipeline/pipelines/pipe-trigep/trigger", nil)
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)
	assert.Equal(t, http.StatusOK, w2.Code)
}

func TestGetExecutionsEndpoint(t *testing.T) {
	mgr, r := setupTest(t)
	require.NoError(t, mgr.CreatePipeline(testPipeline("pipe-exec-ep")))

	// 先执行一次
	_, err := mgr.TriggerExecution("pipe-exec-ep")
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "/datapipeline/pipelines/pipe-exec-ep/executions?limit=5", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, float64(1), resp["total"])
}

func TestDefaultRetryPolicy(t *testing.T) {
	mgr, _ := setupTest(t)
	p := testPipeline("pipe-retry")
	p.Retry = RetryPolicy{} // 零值，应该被填充默认值
	require.NoError(t, mgr.CreatePipeline(p))

	saved, _ := mgr.GetPipeline("pipe-retry")
	assert.Equal(t, 3, saved.Retry.MaxRetries)
	assert.True(t, saved.DeadLetter.Enabled)
}
