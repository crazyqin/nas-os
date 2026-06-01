// Package appmarket 应用市场增强测试
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

func setupTestManager2(t *testing.T) *Manager {
	t.Helper()
	configPath := filepath.Join(t.TempDir(), "config.json")
	return NewManager(configPath)
}

func setupTestRouter2(mgr *Manager) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	api := r.Group("/appmarket")
	RegisterRoutes(api, mgr)
	return r
}

// ========== Handler: ListPendingApps ==========

func TestHandler_ListPendingApps(t *testing.T) {
	mgr := setupTestManager2(t)
	router := setupTestRouter2(mgr)

	// 发布应用（状态为 pending_review）
	app, _ := mgr.PublishApp(&PublishRequest{
		Name: "Pending App", Version: "1.0", Category: CategoryUtility,
	}, "dev1")

	req, _ := http.NewRequest("GET", "/appmarket/apps/pending", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp APIResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	data := resp.Data.([]interface{})
	assert.Len(t, data, 1)
	_ = app
}

func TestHandler_ListPendingApps_Empty(t *testing.T) {
	mgr := setupTestManager2(t)
	router := setupTestRouter2(mgr)

	req, _ := http.NewRequest("GET", "/appmarket/apps/pending", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp APIResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	// 无待审核应用时 Data 可能为 nil 或空数组
	if resp.Data != nil {
		data := resp.Data.([]interface{})
		assert.Len(t, data, 0)
	}
}

// ========== Handler: UpdateApp ==========

func TestHandler_UpdateApp(t *testing.T) {
	mgr := setupTestManager2(t)
	// 使用 middleware 设置 user_id
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("user_id", "dev1")
		c.Next()
	})
	api := r.Group("/appmarket")
	RegisterRoutes(api, mgr)

	app, _ := mgr.PublishApp(&PublishRequest{
		Name: "Test App", Version: "1.0.0", Category: CategoryUtility,
	}, "dev1")

	body := PublishRequest{Name: "Updated App", Version: "1.1.0", Category: CategoryUtility}
	jsonBody, _ := json.Marshal(body)

	req, _ := http.NewRequest("PUT", "/appmarket/apps/"+app.ID, bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestHandler_UpdateApp_InvalidJSON(t *testing.T) {
	mgr := setupTestManager2(t)
	router := setupTestRouter2(mgr)

	app, _ := mgr.PublishApp(&PublishRequest{
		Name: "Test App", Version: "1.0.0", Category: CategoryUtility,
	}, "dev1")

	req, _ := http.NewRequest("PUT", "/appmarket/apps/"+app.ID, bytes.NewBufferString("bad"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandler_UpdateApp_NotFound(t *testing.T) {
	mgr := setupTestManager2(t)
	router := setupTestRouter2(mgr)

	body := PublishRequest{Name: "X", Version: "1.0", Category: CategoryUtility}
	jsonBody, _ := json.Marshal(body)

	req, _ := http.NewRequest("PUT", "/appmarket/apps/nonexistent", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// ========== Handler: GetReviewHistory ==========

func TestHandler_GetReviewHistory(t *testing.T) {
	mgr := setupTestManager2(t)
	router := setupTestRouter2(mgr)

	app, _ := mgr.PublishApp(&PublishRequest{
		Name: "Test App", Version: "1.0.0", Category: CategoryUtility,
	}, "dev1")
	mgr.ReviewApp(app.ID, &ReviewRequest{Action: ReviewApprove, Note: "OK"}, "admin")

	req, _ := http.NewRequest("GET", "/appmarket/apps/"+app.ID+"/reviews", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp APIResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	data := resp.Data.([]interface{})
	assert.Len(t, data, 1)
}

func TestHandler_GetReviewHistory_Empty(t *testing.T) {
	mgr := setupTestManager2(t)
	router := setupTestRouter2(mgr)

	app, _ := mgr.PublishApp(&PublishRequest{
		Name: "Test App", Version: "1.0.0", Category: CategoryUtility,
	}, "dev1")

	req, _ := http.NewRequest("GET", "/appmarket/apps/"+app.ID+"/reviews", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

// ========== Handler: ListAppsByCategory ==========

func TestHandler_ListAppsByCategory(t *testing.T) {
	mgr := setupTestManager2(t)
	router := setupTestRouter2(mgr)

	app, _ := mgr.PublishApp(&PublishRequest{
		Name: "Media App", Version: "1.0", Category: CategoryMedia,
	}, "dev1")
	mgr.ReviewApp(app.ID, &ReviewRequest{Action: ReviewApprove}, "admin")
	mgr.InstallApp(&InstallRequest{AppID: app.ID}, "user1")

	req, _ := http.NewRequest("GET", "/appmarket/apps/category/media", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp APIResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	data := resp.Data.([]interface{})
	assert.Len(t, data, 1)
}

func TestHandler_ListAppsByCategory_Empty(t *testing.T) {
	mgr := setupTestManager2(t)
	router := setupTestRouter2(mgr)

	req, _ := http.NewRequest("GET", "/appmarket/apps/category/ai", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp APIResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Data != nil {
		data := resp.Data.([]interface{})
		assert.Len(t, data, 0)
	}
}

// ========== Handler: GetAppRatings ==========

func TestHandler_GetAppRatings(t *testing.T) {
	mgr := setupTestManager2(t)
	router := setupTestRouter2(mgr)

	app, _ := mgr.PublishApp(&PublishRequest{
		Name: "Test App", Version: "1.0.0", Category: CategoryUtility,
	}, "dev1")
	mgr.ReviewApp(app.ID, &ReviewRequest{Action: ReviewApprove}, "admin")
	mgr.RateApp(app.ID, &RatingRequest{Score: 5, Comment: "Great!"}, "user1")

	req, _ := http.NewRequest("GET", "/appmarket/apps/"+app.ID+"/ratings", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp APIResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	data := resp.Data.([]interface{})
	assert.Len(t, data, 1)
}

func TestHandler_GetAppRatings_Empty(t *testing.T) {
	mgr := setupTestManager2(t)
	router := setupTestRouter2(mgr)

	app, _ := mgr.PublishApp(&PublishRequest{
		Name: "Test App", Version: "1.0.0", Category: CategoryUtility,
	}, "dev1")

	req, _ := http.NewRequest("GET", "/appmarket/apps/"+app.ID+"/ratings", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

// ========== Handler: GetInstalledApp ==========

func TestHandler_GetInstalledApp(t *testing.T) {
	mgr := setupTestManager2(t)
	router := setupTestRouter2(mgr)

	app, _ := mgr.PublishApp(&PublishRequest{
		Name: "Test App", Version: "1.0.0", Category: CategoryUtility,
	}, "dev1")
	mgr.ReviewApp(app.ID, &ReviewRequest{Action: ReviewApprove}, "admin")
	mgr.InstallApp(&InstallRequest{AppID: app.ID}, "user1")

	req, _ := http.NewRequest("GET", "/appmarket/installed/"+app.ID, nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestHandler_GetInstalledApp_NotFound(t *testing.T) {
	mgr := setupTestManager2(t)
	router := setupTestRouter2(mgr)

	req, _ := http.NewRequest("GET", "/appmarket/installed/nonexistent", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

// ========== Handler: UpdateInstalledApp ==========

func TestHandler_UpdateInstalledApp(t *testing.T) {
	mgr := setupTestManager2(t)
	router := setupTestRouter2(mgr)

	app, _ := mgr.PublishApp(&PublishRequest{
		Name: "Test App", Version: "1.0.0", Category: CategoryUtility,
	}, "dev1")
	mgr.ReviewApp(app.ID, &ReviewRequest{Action: ReviewApprove}, "admin")
	mgr.InstallApp(&InstallRequest{AppID: app.ID}, "user1")

	// 发布新版本
	mgr.UpdateApp(app.ID, &PublishRequest{
		Name: "Test App", Version: "2.0.0", Category: CategoryUtility,
	}, "dev1")
	mgr.ReviewApp(app.ID, &ReviewRequest{Action: ReviewApprove}, "admin")

	body := UpdateRequest{AppID: app.ID}
	jsonBody, _ := json.Marshal(body)

	req, _ := http.NewRequest("PUT", "/appmarket/install/"+app.ID, bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestHandler_UpdateInstalledApp_InvalidBody(t *testing.T) {
	mgr := setupTestManager2(t)
	router := setupTestRouter2(mgr)

	req, _ := http.NewRequest("PUT", "/appmarket/install/test", bytes.NewBufferString("bad"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// ========== Handler: PublishApp_InvalidJSON ==========

func TestHandler_PublishApp_InvalidJSONBody(t *testing.T) {
	mgr := setupTestManager2(t)
	router := setupTestRouter2(mgr)

	req, _ := http.NewRequest("POST", "/appmarket/apps", bytes.NewBufferString("not json"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// ========== Handler: RateApp_InvalidJSON ==========

func TestHandler_RateApp_InvalidJSON(t *testing.T) {
	mgr := setupTestManager2(t)
	router := setupTestRouter2(mgr)

	app, _ := mgr.PublishApp(&PublishRequest{
		Name: "Test", Version: "1.0", Category: CategoryUtility,
	}, "dev1")
	mgr.ReviewApp(app.ID, &ReviewRequest{Action: ReviewApprove}, "admin")

	req, _ := http.NewRequest("POST", "/appmarket/apps/"+app.ID+"/rate", bytes.NewBufferString("bad"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// ========== Handler: InstallApp_InvalidJSON ==========

func TestHandler_InstallApp_InvalidJSON(t *testing.T) {
	mgr := setupTestManager2(t)
	router := setupTestRouter2(mgr)

	req, _ := http.NewRequest("POST", "/appmarket/install", bytes.NewBufferString("bad"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// ========== Handler: ReviewApp_InvalidJSON ==========

func TestHandler_ReviewApp_InvalidJSON(t *testing.T) {
	mgr := setupTestManager2(t)
	router := setupTestRouter2(mgr)

	app, _ := mgr.PublishApp(&PublishRequest{
		Name: "Test", Version: "1.0", Category: CategoryUtility,
	}, "dev1")

	req, _ := http.NewRequest("POST", "/appmarket/apps/"+app.ID+"/review", bytes.NewBufferString("bad"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// ========== Manager: ListApps ==========

func TestManager_ListApps_All(t *testing.T) {
	mgr := setupTestManager2(t)

	mgr.PublishApp(&PublishRequest{Name: "A", Version: "1.0", Category: CategoryUtility}, "dev1")
	mgr.PublishApp(&PublishRequest{Name: "B", Version: "1.0", Category: CategoryMedia}, "dev1")

	apps := mgr.ListApps("")
	assert.Len(t, apps, 2)
}

func TestManager_ListApps_FilterStatus(t *testing.T) {
	mgr := setupTestManager2(t)

	app1, _ := mgr.PublishApp(&PublishRequest{Name: "A", Version: "1.0", Category: CategoryUtility}, "dev1")
	mgr.PublishApp(&PublishRequest{Name: "B", Version: "1.0", Category: CategoryMedia}, "dev1")
	mgr.ReviewApp(app1.ID, &ReviewRequest{Action: ReviewApprove}, "admin")

	pending := mgr.ListApps(StatusPendingReview)
	assert.Len(t, pending, 1)

	approved := mgr.ListApps(StatusApproved)
	assert.Len(t, approved, 1)
}

// ========== Manager: GetInstalledApp ==========

func TestManager_GetInstalledApp_Success(t *testing.T) {
	mgr := setupTestManager2(t)

	app, _ := mgr.PublishApp(&PublishRequest{Name: "Test", Version: "1.0", Category: CategoryUtility}, "dev1")
	mgr.ReviewApp(app.ID, &ReviewRequest{Action: ReviewApprove}, "admin")
	mgr.InstallApp(&InstallRequest{AppID: app.ID}, "user1")

	installed, err := mgr.GetInstalledApp(app.ID)
	require.NoError(t, err)
	assert.Equal(t, app.ID, installed.AppID)
	assert.Equal(t, "running", installed.Status)
}

func TestManager_GetInstalledApp_NotFound(t *testing.T) {
	mgr := setupTestManager2(t)

	_, err := mgr.GetInstalledApp("nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "未安装")
}

// ========== Manager: GetAppRatings ==========

func TestManager_GetAppRatings_WithData(t *testing.T) {
	mgr := setupTestManager2(t)

	app, _ := mgr.PublishApp(&PublishRequest{Name: "Test", Version: "1.0", Category: CategoryUtility}, "dev1")
	mgr.ReviewApp(app.ID, &ReviewRequest{Action: ReviewApprove}, "admin")

	mgr.RateApp(app.ID, &RatingRequest{Score: 5}, "user1")
	mgr.RateApp(app.ID, &RatingRequest{Score: 3}, "user2")

	ratings := mgr.GetAppRatings(app.ID)
	assert.Len(t, ratings, 2)
}

func TestManager_GetAppRatings_Empty(t *testing.T) {
	mgr := setupTestManager2(t)

	ratings := mgr.GetAppRatings("nonexistent")
	assert.Nil(t, ratings)
}

// ========== Manager: UpdateInstalledApp ==========

func TestManager_UpdateInstalledApp_AlreadyLatest(t *testing.T) {
	mgr := setupTestManager2(t)

	app, _ := mgr.PublishApp(&PublishRequest{Name: "Test", Version: "1.0", Category: CategoryUtility}, "dev1")
	mgr.ReviewApp(app.ID, &ReviewRequest{Action: ReviewApprove}, "admin")
	mgr.InstallApp(&InstallRequest{AppID: app.ID}, "user1")

	_, err := mgr.UpdateInstalledApp(&UpdateRequest{AppID: app.ID})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "最新版本")
}

func TestManager_UpdateInstalledApp_NotInstalled(t *testing.T) {
	mgr := setupTestManager2(t)

	_, err := mgr.UpdateInstalledApp(&UpdateRequest{AppID: "nonexistent"})
	assert.Error(t, err)
}

// ========== Manager: SearchApps_Sort ==========

func TestManager_SearchApps_SortRating(t *testing.T) {
	mgr := setupTestManager2(t)

	app1, _ := mgr.PublishApp(&PublishRequest{Name: "App A", Version: "1.0", Category: CategoryUtility}, "dev1")
	mgr.ReviewApp(app1.ID, &ReviewRequest{Action: ReviewApprove}, "admin")
	mgr.InstallApp(&InstallRequest{AppID: app1.ID}, "user1")

	app2, _ := mgr.PublishApp(&PublishRequest{Name: "App B", Version: "1.0", Category: CategoryUtility}, "dev1")
	mgr.ReviewApp(app2.ID, &ReviewRequest{Action: ReviewApprove}, "admin")
	mgr.InstallApp(&InstallRequest{AppID: app2.ID}, "user1")

	// 设置不同评分
	mgr.RateApp(app1.ID, &RatingRequest{Score: 3}, "u1")
	mgr.RateApp(app2.ID, &RatingRequest{Score: 5}, "u1")

	result := mgr.SearchApps(&SearchRequest{Sort: SortRating})
	require.Len(t, result.Apps, 2)
	assert.Equal(t, "App B", result.Apps[0].Name) // 评分高的排前面
}

func TestManager_SearchApps_SortDownloads(t *testing.T) {
	mgr := setupTestManager2(t)

	app1, _ := mgr.PublishApp(&PublishRequest{Name: "App A", Version: "1.0", Category: CategoryUtility}, "dev1")
	mgr.ReviewApp(app1.ID, &ReviewRequest{Action: ReviewApprove}, "admin")
	mgr.InstallApp(&InstallRequest{AppID: app1.ID}, "user1")

	app2, _ := mgr.PublishApp(&PublishRequest{Name: "App B", Version: "1.0", Category: CategoryUtility}, "dev1")
	mgr.ReviewApp(app2.ID, &ReviewRequest{Action: ReviewApprove}, "admin")
	mgr.InstallApp(&InstallRequest{AppID: app2.ID}, "user1")
	mgr.UninstallApp(app2.ID)
	mgr.InstallApp(&InstallRequest{AppID: app2.ID}, "user2") // 第二次下载

	result := mgr.SearchApps(&SearchRequest{Sort: SortDownloads})
	require.Len(t, result.Apps, 2)
	assert.Equal(t, "App B", result.Apps[0].Name) // 下载量高的排前面
}
