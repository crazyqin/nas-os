package snapshottimeline

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestManager(t *testing.T) *Manager {
	t.Helper()
	return NewManager()
}

func setupTestRouter(t *testing.T, m *Manager) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	rg := r.Group("/api/v1")
	h := NewHandlers(m)
	h.RegisterRoutes(rg)
	return r
}

func TestCreateSnapshot(t *testing.T) {
	m := setupTestManager(t)

	// 创建快照
	entry, err := m.CreateSnapshot("pool1", "tank/data", "backup-2024", "daily backup", []string{"daily", "important"})
	require.NoError(t, err)
	assert.NotEmpty(t, entry.ID)
	assert.Equal(t, "pool1", entry.PoolID)
	assert.Equal(t, "tank/data", entry.Dataset)
	assert.Equal(t, "backup-2024", entry.Name)
	assert.Equal(t, "daily backup", entry.Description)
	assert.Equal(t, SnapshotStateActive, entry.State)
	assert.Equal(t, []string{"daily", "important"}, entry.Tags)

	// 测试缺少必填字段
	_, err = m.CreateSnapshot("", "tank/data", "test", "", nil)
	assert.Error(t, err)

	_, err = m.CreateSnapshot("pool1", "", "test", "", nil)
	assert.Error(t, err)

	_, err = m.CreateSnapshot("pool1", "tank/data", "", "", nil)
	assert.Error(t, err)
}

func TestGetSnapshot(t *testing.T) {
	m := setupTestManager(t)

	// 创建快照
	entry, err := m.CreateSnapshot("pool1", "tank/data", "snap1", "test", nil)
	require.NoError(t, err)

	// 获取快照
	fetched, err := m.GetSnapshot(entry.ID)
	require.NoError(t, err)
	assert.Equal(t, entry.ID, fetched.ID)
	assert.Equal(t, entry.Name, fetched.Name)

	// 获取不存在的快照
	_, err = m.GetSnapshot("non-existent-id")
	assert.Error(t, err)
}

func TestListSnapshots(t *testing.T) {
	m := setupTestManager(t)

	// 创建多个快照
	_, err := m.CreateSnapshot("pool1", "tank/data", "snap1", "test1", []string{"daily"})
	require.NoError(t, err)

	time.Sleep(10 * time.Millisecond) // 确保时间不同

	_, err = m.CreateSnapshot("pool1", "tank/data", "snap2", "test2", []string{"weekly"})
	require.NoError(t, err)

	time.Sleep(10 * time.Millisecond)

	_, err = m.CreateSnapshot("pool1", "tank/media", "snap3", "test3", []string{"daily"})
	require.NoError(t, err)

	// 列出所有快照
	all, err := m.ListSnapshots(TimelineFilter{})
	require.NoError(t, err)
	assert.Equal(t, 3, len(all))

	// 按 dataset 过滤
	dataSnaps, err := m.ListSnapshots(TimelineFilter{Dataset: "tank/data"})
	require.NoError(t, err)
	assert.Equal(t, 2, len(dataSnaps))

	// 按标签过滤
	dailySnaps, err := m.ListSnapshots(TimelineFilter{Tags: []string{"daily"}})
	require.NoError(t, err)
	assert.Equal(t, 2, len(dailySnaps))

	// 测试分页
	limited, err := m.ListSnapshots(TimelineFilter{Limit: 1})
	require.NoError(t, err)
	assert.Equal(t, 1, len(limited))
}

func TestDeleteSnapshot(t *testing.T) {
	m := setupTestManager(t)

	// 创建快照
	entry, err := m.CreateSnapshot("pool1", "tank/data", "snap1", "test", nil)
	require.NoError(t, err)

	// 删除快照
	err = m.DeleteSnapshot(entry.ID)
	assert.NoError(t, err)

	// 验证已删除
	_, err = m.GetSnapshot(entry.ID)
	assert.Error(t, err)

	// 删除不存在的快照
	err = m.DeleteSnapshot("non-existent-id")
	assert.Error(t, err)
}

func TestDeleteSnapshotWithChildren(t *testing.T) {
	m := setupTestManager(t)

	// 创建父快照
	parent, err := m.CreateSnapshot("pool1", "tank/data", "parent", "parent snap", nil)
	require.NoError(t, err)

	// 创建子快照
	child, err := m.CreateSnapshot("pool1", "tank/data", "child", "child snap", nil)
	require.NoError(t, err)

	// 设置父子关系
	m.mu.Lock()
	child.ParentID = parent.ID
	parent.Children = []string{child.ID}
	m.mu.Unlock()

	// 尝试删除有子快照的父快照（应该失败）
	err = m.DeleteSnapshot(parent.ID)
	assert.Error(t, err)

	// 先删除子快照
	err = m.DeleteSnapshot(child.ID)
	assert.NoError(t, err)

	// 现在可以删除父快照
	err = m.DeleteSnapshot(parent.ID)
	assert.NoError(t, err)
}

func TestRestoreSnapshot_Clone(t *testing.T) {
	m := setupTestManager(t)

	// 创建快照
	entry, err := m.CreateSnapshot("pool1", "tank/data", "snap1", "test", []string{"backup"})
	require.NoError(t, err)

	// 克隆恢复
	result, err := m.RestoreSnapshot(context.Background(), RestoreRequest{
		SnapshotID:  entry.ID,
		CreateClone: true,
	})
	require.NoError(t, err)
	assert.True(t, result.Success)
	assert.Equal(t, "clone", result.RestoreType)
	assert.NotEmpty(t, result.RestoredPath)

	// 验证原快照状态变为 cloned
	fetched, err := m.GetSnapshot(entry.ID)
	require.NoError(t, err)
	assert.Equal(t, SnapshotStateCloned, fetched.State)
	assert.Equal(t, 1, len(fetched.Children))
}

func TestRestoreSnapshot_Rollback(t *testing.T) {
	m := setupTestManager(t)

	// 创建快照
	entry, err := m.CreateSnapshot("pool1", "tank/data", "snap1", "test", nil)
	require.NoError(t, err)

	// 回滚恢复
	result, err := m.RestoreSnapshot(context.Background(), RestoreRequest{
		SnapshotID: entry.ID,
		Force:      true,
	})
	require.NoError(t, err)
	assert.True(t, result.Success)
	assert.Equal(t, "rollback", result.RestoreType)
	assert.Equal(t, "/tank/data", result.RestoredPath)

	// 验证快照状态变为 rollback
	fetched, err := m.GetSnapshot(entry.ID)
	require.NoError(t, err)
	assert.Equal(t, SnapshotStateRollback, fetched.State)
}

func TestGetTimeline(t *testing.T) {
	m := setupTestManager(t)

	// 创建快照
	_, err := m.CreateSnapshot("pool1", "tank/data", "snap1", "test1", nil)
	require.NoError(t, err)

	time.Sleep(10 * time.Millisecond)

	_, err = m.CreateSnapshot("pool1", "tank/data", "snap2", "test2", nil)
	require.NoError(t, err)

	time.Sleep(10 * time.Millisecond)

	_, err = m.CreateSnapshot("pool1", "tank/media", "snap3", "test3", nil)
	require.NoError(t, err)

	// 获取时间线
	timeline, err := m.GetTimeline("tank/data", time.Time{}, time.Time{})
	require.NoError(t, err)
	assert.Equal(t, 2, len(timeline))

	// 验证按时间正序排列
	assert.True(t, timeline[0].CreatedAt.Before(timeline[1].CreatedAt) || timeline[0].CreatedAt.Equal(timeline[1].CreatedAt))

	// 获取不存在的 dataset
	empty, err := m.GetTimeline("tank/nonexistent", time.Time{}, time.Time{})
	require.NoError(t, err)
	assert.Equal(t, 0, len(empty))
}

func TestGetStats(t *testing.T) {
	m := setupTestManager(t)

	// 空统计
	stats := m.GetStats()
	assert.Equal(t, 0, stats.TotalSnapshots)

	// 创建快照
	_, err := m.CreateSnapshot("pool1", "tank/data", "snap1", "test1", nil)
	require.NoError(t, err)

	_, err = m.CreateSnapshot("pool1", "tank/data", "snap2", "test2", nil)
	require.NoError(t, err)

	_, err = m.CreateSnapshot("pool1", "tank/media", "snap3", "test3", nil)
	require.NoError(t, err)

	// 获取统计
	stats = m.GetStats()
	assert.Equal(t, 3, stats.TotalSnapshots)
	assert.Equal(t, 2, stats.ByDataset["tank/data"])
	assert.Equal(t, 1, stats.ByDataset["tank/media"])
}

func TestCompareSnapshots(t *testing.T) {
	m := setupTestManager(t)

	// 创建两个快照
	entry1, err := m.CreateSnapshot("pool1", "tank/data", "snap1", "test1", []string{"tag1", "tag2"})
	require.NoError(t, err)

	time.Sleep(10 * time.Millisecond)

	entry2, err := m.CreateSnapshot("pool1", "tank/data", "snap2", "test2", []string{"tag2", "tag3"})
	require.NoError(t, err)

	// 对比快照
	diff, err := m.CompareSnapshots(entry1.ID, entry2.ID)
	require.NoError(t, err)
	assert.NotNil(t, diff.Snapshot1)
	assert.NotNil(t, diff.Snapshot2)
	assert.Equal(t, entry1.ID, diff.Snapshot1.ID)
	assert.Equal(t, entry2.ID, diff.Snapshot2.ID)

	// 验证标签差异
	assert.Contains(t, diff.TagsAdded, "tag3")
	assert.Contains(t, diff.TagsRemoved, "tag1")
	assert.NotContains(t, diff.TagsAdded, "tag2")   // tag2 两边都有
	assert.NotContains(t, diff.TagsRemoved, "tag2") // tag2 两边都有

	// 对比不存在的快照
	_, err = m.CompareSnapshots("non-existent", entry2.ID)
	assert.Error(t, err)

	_, err = m.CompareSnapshots(entry1.ID, "non-existent")
	assert.Error(t, err)
}

func TestHTTPCreateSnapshot(t *testing.T) {
	m := setupTestManager(t)
	router := setupTestRouter(t, m)

	body := map[string]interface{}{
		"pool_id":     "pool1",
		"dataset":     "tank/data",
		"name":        "test-snap",
		"description": "test snapshot",
		"tags":        []string{"test"},
	}

	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/snapshots", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)

	var resp response
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, 0, resp.Code)
}

func TestHTTPGetSnapshot(t *testing.T) {
	m := setupTestManager(t)
	router := setupTestRouter(t, m)

	// 先创建快照
	entry, err := m.CreateSnapshot("pool1", "tank/data", "snap1", "test", nil)
	require.NoError(t, err)

	// 获取快照
	req := httptest.NewRequest(http.MethodGet, "/api/v1/snapshots/"+entry.ID, nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp response
	err = json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, 0, resp.Code)

	// 获取不存在的快照
	req = httptest.NewRequest(http.MethodGet, "/api/v1/snapshots/non-existent", nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestHTTPDeleteSnapshot(t *testing.T) {
	m := setupTestManager(t)
	router := setupTestRouter(t, m)

	// 先创建快照
	entry, err := m.CreateSnapshot("pool1", "tank/data", "snap1", "test", nil)
	require.NoError(t, err)

	// 删除快照
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/snapshots/"+entry.ID, nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	// 验证已删除
	_, err = m.GetSnapshot(entry.ID)
	assert.Error(t, err)
}

func TestHTTPGetTimeline(t *testing.T) {
	m := setupTestManager(t)
	router := setupTestRouter(t, m)

	// 创建快照
	_, err := m.CreateSnapshot("pool1", "tank/data", "snap1", "test", nil)
	require.NoError(t, err)

	// 获取时间线
	req := httptest.NewRequest(http.MethodGet, "/api/v1/snapshots/timeline?dataset=tank/data", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp response
	err = json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, 0, resp.Code)
}

func TestHTTPGetStats(t *testing.T) {
	m := setupTestManager(t)
	router := setupTestRouter(t, m)

	// 创建快照
	_, err := m.CreateSnapshot("pool1", "tank/data", "snap1", "test", nil)
	require.NoError(t, err)

	// 获取统计
	req := httptest.NewRequest(http.MethodGet, "/api/v1/snapshots/stats", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp response
	err = json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, 0, resp.Code)
}
