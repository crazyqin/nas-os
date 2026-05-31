// Package appmarket 应用市场模块测试
package appmarket

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

func setupTestManager(t *testing.T) *Manager {
	t.Helper()
	configPath := filepath.Join(t.TempDir(), "config.json")
	return NewManager(configPath)
}

func setupTestRouter(mgr *Manager) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	api := r.Group("/appmarket")
	RegisterRoutes(api, mgr)
	return r
}

// ========== 类型测试 ==========

func TestAppStatus_Constants(t *testing.T) {
	assert.Equal(t, AppStatus("draft"), StatusDraft)
	assert.Equal(t, AppStatus("pending_review"), StatusPendingReview)
	assert.Equal(t, AppStatus("approved"), StatusApproved)
	assert.Equal(t, AppStatus("rejected"), StatusRejected)
	assert.Equal(t, AppStatus("revision"), StatusRevision)
	assert.Equal(t, AppStatus("published"), StatusPublished)
	assert.Equal(t, AppStatus("suspended"), StatusSuspended)
}

func TestAppCategory_Constants(t *testing.T) {
	assert.Equal(t, AppCategory("productivity"), CategoryProductivity)
	assert.Equal(t, AppCategory("media"), CategoryMedia)
	assert.Equal(t, AppCategory("network"), CategoryNetwork)
	assert.Equal(t, AppCategory("storage"), CategoryStorage)
	assert.Equal(t, AppCategory("security"), CategorySecurity)
	assert.Equal(t, AppCategory("devops"), CategoryDevOps)
	assert.Equal(t, AppCategory("database"), CategoryDatabase)
	assert.Equal(t, AppCategory("ai"), CategoryAI)
	assert.Equal(t, AppCategory("gaming"), CategoryGaming)
	assert.Equal(t, AppCategory("utility"), CategoryUtility)
	assert.Equal(t, AppCategory("other"), CategoryOther)
}

// ========== Manager 测试 ==========

func TestManager_PublishApp(t *testing.T) {
	mgr := setupTestManager(t)

	req := &PublishRequest{
		Name:        "Test App",
		Description: "A test application",
		Version:     "1.0.0",
		Category:    CategoryUtility,
		Tags:        []string{"test", "demo"},
		Size:        1024,
	}

	app, err := mgr.PublishApp(req, "developer1")
	require.NoError(t, err)
	assert.NotEmpty(t, app.ID)
	assert.Equal(t, "Test App", app.Name)
	assert.Equal(t, "1.0.0", app.Version)
	assert.Equal(t, StatusPendingReview, app.Status)
	assert.Equal(t, "developer1", app.DeveloperID)
	assert.Equal(t, int64(0), app.Downloads)
	assert.Equal(t, float64(0), app.Rating)
}

func TestManager_PublishApp_Validation(t *testing.T) {
	mgr := setupTestManager(t)

	// 缺少名称
	_, err := mgr.PublishApp(&PublishRequest{
		Version:  "1.0.0",
		Category: CategoryUtility,
	}, "dev1")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "名称不能为空")

	// 缺少版本
	_, err = mgr.PublishApp(&PublishRequest{
		Name:     "Test",
		Category: CategoryUtility,
	}, "dev1")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "版本不能为空")

	// 缺少分类
	_, err = mgr.PublishApp(&PublishRequest{
		Name:    "Test",
		Version: "1.0.0",
	}, "dev1")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "分类不能为空")
}

func TestManager_UpdateApp(t *testing.T) {
	mgr := setupTestManager(t)

	// 先发布
	app, _ := mgr.PublishApp(&PublishRequest{
		Name:     "Test App",
		Version:  "1.0.0",
		Category: CategoryUtility,
	}, "developer1")

	// 更新
	updated, err := mgr.UpdateApp(app.ID, &PublishRequest{
		Name:        "Updated App",
		Version:     "1.1.0",
		Description: "Updated description",
	}, "developer1")
	require.NoError(t, err)
	assert.Equal(t, "Updated App", updated.Name)
	assert.Equal(t, "1.1.0", updated.Version)
	assert.Equal(t, StatusPendingReview, updated.Status)
}

func TestManager_UpdateApp_Permission(t *testing.T) {
	mgr := setupTestManager(t)

	app, _ := mgr.PublishApp(&PublishRequest{
		Name:     "Test App",
		Version:  "1.0.0",
		Category: CategoryUtility,
	}, "developer1")

	// 其他开发者无法更新
	_, err := mgr.UpdateApp(app.ID, &PublishRequest{
		Name: "Hacked",
	}, "developer2")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "无权更新")
}

func TestManager_ReviewApp(t *testing.T) {
	mgr := setupTestManager(t)

	app, _ := mgr.PublishApp(&PublishRequest{
		Name:     "Test App",
		Version:  "1.0.0",
		Category: CategoryUtility,
	}, "developer1")

	// 通过审核
	record, err := mgr.ReviewApp(app.ID, &ReviewRequest{
		Action: ReviewApprove,
		Note:   "审核通过",
	}, "admin")
	require.NoError(t, err)
	assert.Equal(t, ReviewApprove, record.Action)

	// 验证状态
	updatedApp, _ := mgr.GetApp(app.ID)
	assert.Equal(t, StatusApproved, updatedApp.Status)
	assert.Equal(t, "admin", updatedApp.ReviewedBy)
}

func TestManager_ReviewApp_Reject(t *testing.T) {
	mgr := setupTestManager(t)

	app, _ := mgr.PublishApp(&PublishRequest{
		Name:     "Test App",
		Version:  "1.0.0",
		Category: CategoryUtility,
	}, "developer1")

	_, err := mgr.ReviewApp(app.ID, &ReviewRequest{
		Action: ReviewReject,
		Note:   "不符合规范",
	}, "admin")
	require.NoError(t, err)

	updatedApp, _ := mgr.GetApp(app.ID)
	assert.Equal(t, StatusRejected, updatedApp.Status)
	assert.Equal(t, "不符合规范", updatedApp.ReviewNote)
}

func TestManager_ReviewApp_Revision(t *testing.T) {
	mgr := setupTestManager(t)

	app, _ := mgr.PublishApp(&PublishRequest{
		Name:     "Test App",
		Version:  "1.0.0",
		Category: CategoryUtility,
	}, "developer1")

	_, err := mgr.ReviewApp(app.ID, &ReviewRequest{
		Action: ReviewRevision,
		Note:   "请补充文档",
	}, "admin")
	require.NoError(t, err)

	updatedApp, _ := mgr.GetApp(app.ID)
	assert.Equal(t, StatusRevision, updatedApp.Status)
	assert.Equal(t, "请补充文档", updatedApp.ReviewNote)
}

func TestManager_ReviewApp_InvalidStatus(t *testing.T) {
	mgr := setupTestManager(t)

	app, _ := mgr.PublishApp(&PublishRequest{
		Name:     "Test App",
		Version:  "1.0.0",
		Category: CategoryUtility,
	}, "developer1")

	// 先通过审核
	mgr.ReviewApp(app.ID, &ReviewRequest{Action: ReviewApprove}, "admin")

	// 再次审核应该失败
	_, err := mgr.ReviewApp(app.ID, &ReviewRequest{Action: ReviewApprove}, "admin")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "不可审核")
}

func TestManager_GetReviewHistory(t *testing.T) {
	mgr := setupTestManager(t)

	app, _ := mgr.PublishApp(&PublishRequest{
		Name:     "Test App",
		Version:  "1.0.0",
		Category: CategoryUtility,
	}, "developer1")

	// 审核打回
	mgr.ReviewApp(app.ID, &ReviewRequest{Action: ReviewRevision, Note: "补充信息"}, "admin")
	// 开发者重新提交
	mgr.UpdateApp(app.ID, &PublishRequest{Name: "Test App", Version: "1.0.1", Category: CategoryUtility}, "developer1")
	// 审核通过
	mgr.ReviewApp(app.ID, &ReviewRequest{Action: ReviewApprove}, "admin")

	reviews := mgr.GetReviewHistory(app.ID)
	// UpdateApp 会重置状态为 PendingReview，所以第二次 ReviewApp 才能执行
	// 总共 2 条审核记录（revision + approve）
	assert.Len(t, reviews, 2)
}

func TestManager_InstallApp(t *testing.T) {
	mgr := setupTestManager(t)

	// 发布并审核通过
	app, _ := mgr.PublishApp(&PublishRequest{
		Name:     "Test App",
		Version:  "1.0.0",
		Category: CategoryUtility,
	}, "developer1")
	mgr.ReviewApp(app.ID, &ReviewRequest{Action: ReviewApprove}, "admin")

	// 安装
	installed, err := mgr.InstallApp(&InstallRequest{
		AppID: app.ID,
	}, "user1")
	require.NoError(t, err)
	assert.Equal(t, app.ID, installed.AppID)
	assert.Equal(t, "1.0.0", installed.Version)
	assert.Equal(t, "running", installed.Status)

	// 验证下载数增加
	updatedApp, _ := mgr.GetApp(app.ID)
	assert.Equal(t, int64(1), updatedApp.Downloads)
	assert.Equal(t, StatusPublished, updatedApp.Status)
}

func TestManager_InstallApp_Dependencies(t *testing.T) {
	mgr := setupTestManager(t)

	// 发布依赖应用
	depApp, _ := mgr.PublishApp(&PublishRequest{
		Name:     "Dependency App",
		Version:  "1.0.0",
		Category: CategoryUtility,
	}, "dev1")
	mgr.ReviewApp(depApp.ID, &ReviewRequest{Action: ReviewApprove}, "admin")

	// 发布主应用
	mainApp, _ := mgr.PublishApp(&PublishRequest{
		Name:         "Main App",
		Version:      "1.0.0",
		Category:     CategoryUtility,
		Dependencies: []string{depApp.ID},
	}, "dev1")
	mgr.ReviewApp(mainApp.ID, &ReviewRequest{Action: ReviewApprove}, "admin")

	// 先安装依赖
	_, err := mgr.InstallApp(&InstallRequest{AppID: depApp.ID}, "user1")
	require.NoError(t, err)

	// 安装主应用
	_, err = mgr.InstallApp(&InstallRequest{AppID: mainApp.ID}, "user1")
	require.NoError(t, err)
}

func TestManager_InstallApp_MissingDependency(t *testing.T) {
	mgr := setupTestManager(t)

	depApp, _ := mgr.PublishApp(&PublishRequest{
		Name:     "Dependency App",
		Version:  "1.0.0",
		Category: CategoryUtility,
	}, "dev1")
	mgr.ReviewApp(depApp.ID, &ReviewRequest{Action: ReviewApprove}, "admin")

	mainApp, _ := mgr.PublishApp(&PublishRequest{
		Name:         "Main App",
		Version:      "1.0.0",
		Category:     CategoryUtility,
		Dependencies: []string{depApp.ID},
	}, "dev1")
	mgr.ReviewApp(mainApp.ID, &ReviewRequest{Action: ReviewApprove}, "admin")

	// 不安装依赖，直接安装主应用
	_, err := mgr.InstallApp(&InstallRequest{AppID: mainApp.ID}, "user1")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "依赖应用")
}

func TestManager_UninstallApp(t *testing.T) {
	mgr := setupTestManager(t)

	app, _ := mgr.PublishApp(&PublishRequest{
		Name:     "Test App",
		Version:  "1.0.0",
		Category: CategoryUtility,
	}, "dev1")
	mgr.ReviewApp(app.ID, &ReviewRequest{Action: ReviewApprove}, "admin")

	mgr.InstallApp(&InstallRequest{AppID: app.ID}, "user1")

	err := mgr.UninstallApp(app.ID)
	assert.NoError(t, err)

	_, err = mgr.GetInstalledApp(app.ID)
	assert.Error(t, err)
}

func TestManager_UninstallApp_WithDependents(t *testing.T) {
	mgr := setupTestManager(t)

	depApp, _ := mgr.PublishApp(&PublishRequest{
		Name:     "Dependency App",
		Version:  "1.0.0",
		Category: CategoryUtility,
	}, "dev1")
	mgr.ReviewApp(depApp.ID, &ReviewRequest{Action: ReviewApprove}, "admin")

	mainApp, _ := mgr.PublishApp(&PublishRequest{
		Name:         "Main App",
		Version:      "1.0.0",
		Category:     CategoryUtility,
		Dependencies: []string{depApp.ID},
	}, "dev1")
	mgr.ReviewApp(mainApp.ID, &ReviewRequest{Action: ReviewApprove}, "admin")

	mgr.InstallApp(&InstallRequest{AppID: depApp.ID}, "user1")
	mgr.InstallApp(&InstallRequest{AppID: mainApp.ID}, "user1")

	// 无法卸载被依赖的应用
	err := mgr.UninstallApp(depApp.ID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "无法卸载")
}

func TestManager_UpdateInstalledApp(t *testing.T) {
	mgr := setupTestManager(t)

	app, _ := mgr.PublishApp(&PublishRequest{
		Name:     "Test App",
		Version:  "1.0.0",
		Category: CategoryUtility,
	}, "dev1")
	mgr.ReviewApp(app.ID, &ReviewRequest{Action: ReviewApprove}, "admin")

	mgr.InstallApp(&InstallRequest{AppID: app.ID}, "user1")

	// 更新应用版本
	updatedApp, _ := mgr.GetApp(app.ID)
	_ = updatedApp

	// 模拟新版本发布
	mgr.UpdateApp(app.ID, &PublishRequest{
		Name:     "Test App",
		Version:  "2.0.0",
		Category: CategoryUtility,
	}, "dev1")
	mgr.ReviewApp(app.ID, &ReviewRequest{Action: ReviewApprove}, "admin")

	// 更新已安装应用
	installed, err := mgr.UpdateInstalledApp(&UpdateRequest{
		AppID: app.ID,
	})
	require.NoError(t, err)
	assert.Equal(t, "2.0.0", installed.Version)
}

func TestManager_SearchApps(t *testing.T) {
	mgr := setupTestManager(t)

	// 发布多个应用
	apps := []PublishRequest{
		{Name: "Media Player", Version: "1.0", Category: CategoryMedia, Tags: []string{"video", "audio"}},
		{Name: "File Manager", Version: "1.0", Category: CategoryStorage, Tags: []string{"files"}},
		{Name: "Network Tool", Version: "1.0", Category: CategoryNetwork, Tags: []string{"network"}},
		{Name: "Media Converter", Version: "1.0", Category: CategoryMedia, Tags: []string{"video", "convert"}},
	}

	for _, req := range apps {
		app, _ := mgr.PublishApp(&req, "dev1")
		mgr.ReviewApp(app.ID, &ReviewRequest{Action: ReviewApprove}, "admin")
		// 安装以触发状态变为 published
		mgr.InstallApp(&InstallRequest{AppID: app.ID}, "user1")
		mgr.UninstallApp(app.ID)
	}

	// 搜索关键词
	result := mgr.SearchApps(&SearchRequest{Query: "media"})
	assert.Len(t, result.Apps, 2)

	// 按分类过滤
	result = mgr.SearchApps(&SearchRequest{Category: CategoryMedia})
	assert.Len(t, result.Apps, 2)

	// 按标签过滤
	result = mgr.SearchApps(&SearchRequest{Tags: []string{"video"}})
	assert.Len(t, result.Apps, 2)

	// 分页
	result = mgr.SearchApps(&SearchRequest{PageSize: 2, Page: 1})
	assert.Len(t, result.Apps, 2)
	assert.Equal(t, 4, result.Total)
	assert.Equal(t, 2, result.TotalPages)
}

func TestManager_SearchApps_Sort(t *testing.T) {
	mgr := setupTestManager(t)

	// 发布应用并设置不同的下载量
	app1, _ := mgr.PublishApp(&PublishRequest{Name: "App A", Version: "1.0", Category: CategoryUtility}, "dev1")
	mgr.ReviewApp(app1.ID, &ReviewRequest{Action: ReviewApprove}, "admin")
	mgr.InstallApp(&InstallRequest{AppID: app1.ID}, "user1")
	mgr.UninstallApp(app1.ID)

	app2, _ := mgr.PublishApp(&PublishRequest{Name: "App B", Version: "1.0", Category: CategoryUtility}, "dev1")
	mgr.ReviewApp(app2.ID, &ReviewRequest{Action: ReviewApprove}, "admin")
	mgr.InstallApp(&InstallRequest{AppID: app2.ID}, "user1")
	mgr.UninstallApp(app2.ID)
	mgr.InstallApp(&InstallRequest{AppID: app2.ID}, "user2")

	// 按下载量排序
	result := mgr.SearchApps(&SearchRequest{Sort: SortDownloads})
	assert.Equal(t, "App B", result.Apps[0].Name)

	// 按名称排序
	result = mgr.SearchApps(&SearchRequest{Sort: SortName})
	assert.Equal(t, "App A", result.Apps[0].Name)
}

func TestManager_RateApp(t *testing.T) {
	mgr := setupTestManager(t)

	app, _ := mgr.PublishApp(&PublishRequest{
		Name:     "Test App",
		Version:  "1.0.0",
		Category: CategoryUtility,
	}, "dev1")
	mgr.ReviewApp(app.ID, &ReviewRequest{Action: ReviewApprove}, "admin")

	// 评分
	rating, err := mgr.RateApp(app.ID, &RatingRequest{
		Score:   5,
		Comment: "非常好用",
	}, "user1")
	require.NoError(t, err)
	assert.Equal(t, 5, rating.Score)

	// 验证平均评分
	updatedApp, _ := mgr.GetApp(app.ID)
	assert.Equal(t, float64(5), updatedApp.Rating)
	assert.Equal(t, 1, updatedApp.RatingCount)
}

func TestManager_RateApp_MultipleUsers(t *testing.T) {
	mgr := setupTestManager(t)

	app, _ := mgr.PublishApp(&PublishRequest{
		Name:     "Test App",
		Version:  "1.0.0",
		Category: CategoryUtility,
	}, "dev1")
	mgr.ReviewApp(app.ID, &ReviewRequest{Action: ReviewApprove}, "admin")

	// 多用户评分
	mgr.RateApp(app.ID, &RatingRequest{Score: 5}, "user1")
	mgr.RateApp(app.ID, &RatingRequest{Score: 3}, "user2")
	mgr.RateApp(app.ID, &RatingRequest{Score: 4}, "user3")

	updatedApp, _ := mgr.GetApp(app.ID)
	assert.InDelta(t, 4.0, updatedApp.Rating, 0.01)
	assert.Equal(t, 3, updatedApp.RatingCount)
}

func TestManager_RateApp_UpdateRating(t *testing.T) {
	mgr := setupTestManager(t)

	app, _ := mgr.PublishApp(&PublishRequest{
		Name:     "Test App",
		Version:  "1.0.0",
		Category: CategoryUtility,
	}, "dev1")
	mgr.ReviewApp(app.ID, &ReviewRequest{Action: ReviewApprove}, "admin")

	// 第一次评分
	mgr.RateApp(app.ID, &RatingRequest{Score: 3}, "user1")

	// 更新评分
	rating, err := mgr.RateApp(app.ID, &RatingRequest{
		Score:   5,
		Comment: "修改评分",
	}, "user1")
	require.NoError(t, err)
	assert.Equal(t, 5, rating.Score)

	updatedApp, _ := mgr.GetApp(app.ID)
	assert.Equal(t, float64(5), updatedApp.Rating)
	assert.Equal(t, 1, updatedApp.RatingCount) // 仍然是1条评分
}

func TestManager_RateApp_InvalidScore(t *testing.T) {
	mgr := setupTestManager(t)

	app, _ := mgr.PublishApp(&PublishRequest{
		Name:     "Test App",
		Version:  "1.0.0",
		Category: CategoryUtility,
	}, "dev1")
	mgr.ReviewApp(app.ID, &ReviewRequest{Action: ReviewApprove}, "admin")

	_, err := mgr.RateApp(app.ID, &RatingRequest{Score: 0}, "user1")
	assert.Error(t, err)

	_, err = mgr.RateApp(app.ID, &RatingRequest{Score: 6}, "user1")
	assert.Error(t, err)
}

func TestManager_ListCategories(t *testing.T) {
	mgr := setupTestManager(t)

	categories := mgr.ListCategories()
	assert.Len(t, categories, 11)
	assert.Contains(t, categories, CategoryProductivity)
	assert.Contains(t, categories, CategoryAI)
}

func TestManager_ListAppsByCategory(t *testing.T) {
	mgr := setupTestManager(t)

	app1, _ := mgr.PublishApp(&PublishRequest{Name: "Media App", Version: "1.0", Category: CategoryMedia}, "dev1")
	mgr.ReviewApp(app1.ID, &ReviewRequest{Action: ReviewApprove}, "admin")
	mgr.InstallApp(&InstallRequest{AppID: app1.ID}, "user1")

	app2, _ := mgr.PublishApp(&PublishRequest{Name: "Tool App", Version: "1.0", Category: CategoryUtility}, "dev1")
	mgr.ReviewApp(app2.ID, &ReviewRequest{Action: ReviewApprove}, "admin")
	mgr.InstallApp(&InstallRequest{AppID: app2.ID}, "user1")

	mediaApps := mgr.ListAppsByCategory(CategoryMedia)
	assert.Len(t, mediaApps, 1)
	assert.Equal(t, "Media App", mediaApps[0].Name)
}

func TestManager_Persistence(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.json")

	// 创建并保存数据
	mgr1 := NewManager(configPath)
	app, _ := mgr1.PublishApp(&PublishRequest{
		Name:     "Persistent App",
		Version:  "1.0.0",
		Category: CategoryUtility,
	}, "dev1")
	mgr1.ReviewApp(app.ID, &ReviewRequest{Action: ReviewApprove}, "admin")
	mgr1.InstallApp(&InstallRequest{AppID: app.ID}, "user1")
	mgr1.RateApp(app.ID, &RatingRequest{Score: 5, Comment: "Great!"}, "user1")

	// 重新加载
	mgr2 := NewManager(configPath)
	loadedApp, err := mgr2.GetApp(app.ID)
	require.NoError(t, err)
	assert.Equal(t, "Persistent App", loadedApp.Name)
	assert.Equal(t, StatusPublished, loadedApp.Status)

	installed, err := mgr2.GetInstalledApp(app.ID)
	require.NoError(t, err)
	assert.Equal(t, "running", installed.Status)

	ratings := mgr2.GetAppRatings(app.ID)
	assert.Len(t, ratings, 1)
	assert.Equal(t, 5, ratings[0].Score)
}

// ========== Handler 测试 ==========

func TestHandler_PublishApp(t *testing.T) {
	mgr := setupTestManager(t)
	router := setupTestRouter(mgr)

	body := PublishRequest{
		Name:     "Test App",
		Version:  "1.0.0",
		Category: CategoryUtility,
	}
	jsonBody, _ := json.Marshal(body)

	req, _ := http.NewRequest("POST", "/appmarket/apps", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp APIResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, 0, resp.Code)
}

func TestHandler_PublishApp_InvalidRequest(t *testing.T) {
	mgr := setupTestManager(t)
	router := setupTestRouter(mgr)

	// 空请求
	req, _ := http.NewRequest("POST", "/appmarket/apps", bytes.NewBuffer([]byte("{}")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandler_SearchApps(t *testing.T) {
	mgr := setupTestManager(t)
	router := setupTestRouter(mgr)

	// 先发布一个应用
	app, _ := mgr.PublishApp(&PublishRequest{
		Name:     "Search Test",
		Version:  "1.0.0",
		Category: CategoryUtility,
	}, "dev1")
	mgr.ReviewApp(app.ID, &ReviewRequest{Action: ReviewApprove}, "admin")
	mgr.InstallApp(&InstallRequest{AppID: app.ID}, "user1")
	mgr.UninstallApp(app.ID)

	req, _ := http.NewRequest("GET", "/appmarket/apps?q=Search", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp APIResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	data := resp.Data.(map[string]interface{})
	assert.Equal(t, float64(1), data["total"])
}

func TestHandler_InstallApp(t *testing.T) {
	mgr := setupTestManager(t)
	router := setupTestRouter(mgr)

	app, _ := mgr.PublishApp(&PublishRequest{
		Name:     "Install Test",
		Version:  "1.0.0",
		Category: CategoryUtility,
	}, "dev1")
	mgr.ReviewApp(app.ID, &ReviewRequest{Action: ReviewApprove}, "admin")

	body := InstallRequest{AppID: app.ID}
	jsonBody, _ := json.Marshal(body)

	req, _ := http.NewRequest("POST", "/appmarket/install", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestHandler_UninstallApp(t *testing.T) {
	mgr := setupTestManager(t)
	router := setupTestRouter(mgr)

	app, _ := mgr.PublishApp(&PublishRequest{
		Name:     "Uninstall Test",
		Version:  "1.0.0",
		Category: CategoryUtility,
	}, "dev1")
	mgr.ReviewApp(app.ID, &ReviewRequest{Action: ReviewApprove}, "admin")
	mgr.InstallApp(&InstallRequest{AppID: app.ID}, "user1")

	req, _ := http.NewRequest("DELETE", "/appmarket/install/"+app.ID, nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestHandler_RateApp(t *testing.T) {
	mgr := setupTestManager(t)
	router := setupTestRouter(mgr)

	app, _ := mgr.PublishApp(&PublishRequest{
		Name:     "Rate Test",
		Version:  "1.0.0",
		Category: CategoryUtility,
	}, "dev1")
	mgr.ReviewApp(app.ID, &ReviewRequest{Action: ReviewApprove}, "admin")

	body := RatingRequest{Score: 5, Comment: "Good!"}
	jsonBody, _ := json.Marshal(body)

	req, _ := http.NewRequest("POST", "/appmarket/apps/"+app.ID+"/rate", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestHandler_ListCategories(t *testing.T) {
	mgr := setupTestManager(t)
	router := setupTestRouter(mgr)

	req, _ := http.NewRequest("GET", "/appmarket/categories", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	var resp APIResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	data := resp.Data.([]interface{})
	assert.Len(t, data, 11)
}

func TestHandler_GetApp_NotFound(t *testing.T) {
	mgr := setupTestManager(t)
	router := setupTestRouter(mgr)

	req, _ := http.NewRequest("GET", "/appmarket/apps/nonexistent", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestHandler_ReviewApp(t *testing.T) {
	mgr := setupTestManager(t)
	router := setupTestRouter(mgr)

	app, _ := mgr.PublishApp(&PublishRequest{
		Name:     "Review Test",
		Version:  "1.0.0",
		Category: CategoryUtility,
	}, "dev1")

	body := ReviewRequest{Action: ReviewApprove, Note: "OK"}
	jsonBody, _ := json.Marshal(body)

	req, _ := http.NewRequest("POST", "/appmarket/apps/"+app.ID+"/review", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestHandler_ListInstalledApps(t *testing.T) {
	mgr := setupTestManager(t)
	router := setupTestRouter(mgr)

	app, _ := mgr.PublishApp(&PublishRequest{
		Name:     "Installed Test",
		Version:  "1.0.0",
		Category: CategoryUtility,
	}, "dev1")
	mgr.ReviewApp(app.ID, &ReviewRequest{Action: ReviewApprove}, "admin")
	mgr.InstallApp(&InstallRequest{AppID: app.ID}, "user1")

	req, _ := http.NewRequest("GET", "/appmarket/installed", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	var resp APIResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	data := resp.Data.([]interface{})
	assert.Len(t, data, 1)
}
