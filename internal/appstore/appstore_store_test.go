// Package appstore 应用商店 Store/Manager/Handler 测试
package appstore

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ========== Store Tests ==========

func TestStore_NewStore(t *testing.T) {
	store := NewStore()
	assert.NotNil(t, store)
	assert.NotEmpty(t, store.ListTemplates(""), "应有默认模板")
}

func TestStore_ListTemplates_All(t *testing.T) {
	store := NewStore()
	all := store.ListTemplates("")
	assert.NotEmpty(t, all)
}

func TestStore_ListTemplates_Filter(t *testing.T) {
	store := NewStore()
	media := store.ListTemplates("media")
	assert.NotEmpty(t, media)
	for _, tmpl := range media {
		assert.Equal(t, "media", tmpl.Category)
	}
}

func TestStore_ListTemplates_Empty(t *testing.T) {
	store := NewStore()
	empty := store.ListTemplates("nonexistent")
	assert.Empty(t, empty)
}

func TestStore_GetTemplate(t *testing.T) {
	store := NewStore()
	tmpl, ok := store.GetTemplate("jellyfin")
	assert.True(t, ok)
	assert.Equal(t, "Jellyfin", tmpl.Name)
	assert.Equal(t, "media", tmpl.Category)
	assert.NotEmpty(t, tmpl.DockerImage)
}

func TestStore_GetTemplate_NotFound(t *testing.T) {
	store := NewStore()
	_, ok := store.GetTemplate("nonexistent")
	assert.False(t, ok)
}

func TestStore_Install(t *testing.T) {
	store := NewStore()
	ctx := context.Background()

	err := store.Install(ctx, "jellyfin", nil)
	require.NoError(t, err)

	installed := store.ListInstalled()
	assert.Len(t, installed, 1)
	assert.Equal(t, "jellyfin", installed[0].AppID)
	assert.Equal(t, "running", installed[0].Status)
	assert.True(t, installed[0].AutoStart)
}

func TestStore_Install_AlreadyInstalled(t *testing.T) {
	store := NewStore()
	ctx := context.Background()

	store.Install(ctx, "jellyfin", nil)
	err := store.Install(ctx, "jellyfin", nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "已安装")
}

func TestStore_Install_NotFound(t *testing.T) {
	store := NewStore()
	ctx := context.Background()

	err := store.Install(ctx, "nonexistent", nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "不存在")
}

func TestStore_Uninstall(t *testing.T) {
	store := NewStore()
	ctx := context.Background()

	store.Install(ctx, "jellyfin", nil)
	err := store.Uninstall("jellyfin")
	require.NoError(t, err)
	assert.Empty(t, store.ListInstalled())
}

func TestStore_Uninstall_NotInstalled(t *testing.T) {
	store := NewStore()
	err := store.Uninstall("nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "未安装")
}

func TestStore_ListInstalled(t *testing.T) {
	store := NewStore()
	ctx := context.Background()

	assert.Empty(t, store.ListInstalled())

	store.Install(ctx, "jellyfin", nil)
	store.Install(ctx, "nextcloud", nil)

	installed := store.ListInstalled()
	assert.Len(t, installed, 2)
}

func TestStore_Export(t *testing.T) {
	store := NewStore()
	ctx := context.Background()
	store.Install(ctx, "jellyfin", nil)

	data, err := store.Export()
	require.NoError(t, err)
	assert.NotEmpty(t, data)

	var result map[string]interface{}
	err = json.Unmarshal(data, &result)
	require.NoError(t, err)
	assert.Contains(t, result, "templates")
	assert.Contains(t, result, "installed")
}

func TestStore_DefaultTemplates_Contents(t *testing.T) {
	store := NewStore()

	// 验证默认模板的内容
	plex, ok := store.GetTemplate("plex")
	assert.True(t, ok)
	assert.Equal(t, "Plex Media Server", plex.Name)
	assert.Equal(t, "plexinc/pms-docker:latest", plex.DockerImage)
	assert.NotEmpty(t, plex.Ports)
	assert.NotEmpty(t, plex.Volumes)

	ha, ok := store.GetTemplate("homeassistant")
	assert.True(t, ok)
	assert.Equal(t, "Home Assistant", ha.Name)
	assert.Equal(t, "host", ha.Network)
}

func TestStore_MultipleInstalls(t *testing.T) {
	store := NewStore()
	ctx := context.Background()

	apps := []string{"jellyfin", "nextcloud", "plex", "homeassistant"}
	for _, id := range apps {
		err := store.Install(ctx, id, nil)
		require.NoError(t, err, "安装 %s 失败", id)
	}

	installed := store.ListInstalled()
	assert.Len(t, installed, 4)
}

// ========== Manager Tests ==========

func TestManager_New(t *testing.T) {
	mgr := NewManager()
	assert.NotNil(t, mgr)
	assert.NotEmpty(t, mgr.GetCategories())
}

func TestManager_SearchApps_Empty(t *testing.T) {
	mgr := NewManager()
	result := mgr.SearchApps(&AppSearchRequest{Page: 1, PageSize: 20})
	assert.NotNil(t, result)
	assert.Equal(t, 0, result.Total)
	assert.Empty(t, result.Apps)
}

func TestManager_SearchApps_Query(t *testing.T) {
	mgr := NewManager()
	mgr.apps["test-1"] = &App{
		ID: "test-1", Name: "Test App", Description: "A test app", Category: "utilities",
	}
	mgr.apps["test-2"] = &App{
		ID: "test-2", Name: "Media Player", Description: "Play media", Category: "media",
	}

	// contains 函数是 starts-with + case-sensitive，用前缀匹配
	result := mgr.SearchApps(&AppSearchRequest{Query: "Test", Page: 1, PageSize: 20})
	assert.Equal(t, 1, result.Total)
	assert.Equal(t, "Test App", result.Apps[0].Name)
}

func TestManager_SearchApps_Category(t *testing.T) {
	mgr := NewManager()
	mgr.apps["a1"] = &App{ID: "a1", Name: "App1", Category: "media"}
	mgr.apps["a2"] = &App{ID: "a2", Name: "App2", Category: "utilities"}

	result := mgr.SearchApps(&AppSearchRequest{Category: "media", Page: 1, PageSize: 20})
	assert.Equal(t, 1, result.Total)
	assert.Equal(t, "App1", result.Apps[0].Name)
}

func TestManager_SearchApps_Pagination(t *testing.T) {
	mgr := NewManager()
	for i := 0; i < 5; i++ {
		id := string(rune('a' + i))
		mgr.apps[id] = &App{ID: id, Name: "App " + id, Category: "utilities"}
	}

	result := mgr.SearchApps(&AppSearchRequest{Page: 1, PageSize: 2})
	assert.Equal(t, 5, result.Total)
	assert.Len(t, result.Apps, 2)
	assert.True(t, result.HasMore)

	result = mgr.SearchApps(&AppSearchRequest{Page: 3, PageSize: 2})
	assert.Len(t, result.Apps, 1)
	assert.False(t, result.HasMore)
}

func TestManager_SearchApps_DefaultPage(t *testing.T) {
	mgr := NewManager()
	// Page=0 时内部使用默认值1，但结果返回原始请求值
	result := mgr.SearchApps(&AppSearchRequest{})
	assert.NotNil(t, result)
}

func TestManager_GetApp(t *testing.T) {
	mgr := NewManager()
	mgr.apps["test"] = &App{ID: "test", Name: "Test"}

	app, ok := mgr.GetApp("test")
	assert.True(t, ok)
	assert.Equal(t, "Test", app.Name)
}

func TestManager_GetApp_NotFound(t *testing.T) {
	mgr := NewManager()
	_, ok := mgr.GetApp("nonexistent")
	assert.False(t, ok)
}

func TestManager_InstallApp(t *testing.T) {
	mgr := NewManager()
	mgr.apps["test"] = &App{ID: "test", Name: "Test", Version: "1.0"}

	status, err := mgr.InstallApp(&InstallRequest{AppID: "test", AutoStart: true})
	require.NoError(t, err)
	assert.Equal(t, "test", status.AppID)
	assert.Equal(t, "installing", status.Status)
	assert.NotEmpty(t, status.ID)
}

func TestManager_InstallApp_NotFound(t *testing.T) {
	mgr := NewManager()
	_, err := mgr.InstallApp(&InstallRequest{AppID: "nonexistent"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestManager_InstallApp_AlreadyInstalled(t *testing.T) {
	mgr := NewManager()
	mgr.apps["test"] = &App{ID: "test", Name: "Test", Version: "1.0"}
	mgr.installed["test"] = &InstalledApp{AppID: "test", Status: "running"}

	_, err := mgr.InstallApp(&InstallRequest{AppID: "test"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already installed")
}

func TestManager_UninstallApp(t *testing.T) {
	mgr := NewManager()
	mgr.installed["test"] = &InstalledApp{AppID: "test", Status: "running"}

	err := mgr.UninstallApp("test")
	assert.NoError(t, err)
	assert.Empty(t, mgr.GetInstalledApps())
}

func TestManager_UninstallApp_NotInstalled(t *testing.T) {
	mgr := NewManager()
	err := mgr.UninstallApp("nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not installed")
}

func TestManager_GetInstalledApps(t *testing.T) {
	mgr := NewManager()
	assert.Empty(t, mgr.GetInstalledApps())

	mgr.installed["a"] = &InstalledApp{AppID: "a"}
	mgr.installed["b"] = &InstalledApp{AppID: "b"}

	apps := mgr.GetInstalledApps()
	assert.Len(t, apps, 2)
}

func TestManager_GetInstallStatus(t *testing.T) {
	mgr := NewManager()
	mgr.installs["inst-1"] = &InstallStatus{ID: "inst-1", AppID: "test", Status: "completed"}

	status, ok := mgr.GetInstallStatus("inst-1")
	assert.True(t, ok)
	assert.Equal(t, "completed", status.Status)
}

func TestManager_GetInstallStatus_NotFound(t *testing.T) {
	mgr := NewManager()
	_, ok := mgr.GetInstallStatus("nonexistent")
	assert.False(t, ok)
}

func TestManager_GetCategories(t *testing.T) {
	mgr := NewManager()
	cats := mgr.GetCategories()
	assert.NotEmpty(t, cats)
	assert.Contains(t, cats, "productivity")
	assert.Contains(t, cats, "media")
	assert.Contains(t, cats, "security")
}

func TestManager_GetStats(t *testing.T) {
	mgr := NewManager()
	mgr.apps["a"] = &App{ID: "a", Downloads: 10}
	mgr.apps["b"] = &App{ID: "b", Downloads: 20}
	mgr.installed["a"] = &InstalledApp{AppID: "a"}

	stats := mgr.GetStats()
	assert.Equal(t, 2, stats.TotalApps)
	assert.Equal(t, 1, stats.InstalledApps)
	assert.Equal(t, 30, stats.TotalDownloads)
}

func TestManager_GetStats_Empty(t *testing.T) {
	mgr := NewManager()
	stats := mgr.GetStats()
	assert.Equal(t, 0, stats.TotalApps)
	assert.Equal(t, 0, stats.InstalledApps)
	assert.Equal(t, 0, stats.TotalDownloads)
}

// ========== Handler Tests ==========

func TestHandler_HandleApps(t *testing.T) {
	mgr := NewHandler(NewManager())
	mux := http.NewServeMux()
	mgr.RegisterRoutes(mux)

	req, _ := http.NewRequest("GET", "/api/v1/apps", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestHandler_HandleApps_MethodNotAllowed(t *testing.T) {
	mgr := NewHandler(NewManager())
	mux := http.NewServeMux()
	mgr.RegisterRoutes(mux)

	req, _ := http.NewRequest("POST", "/api/v1/apps", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
}

func TestHandler_HandleAppByID(t *testing.T) {
	m := NewManager()
	m.apps["test-app"] = &App{ID: "test-app", Name: "Test App"}
	h := NewHandler(m)
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	req, _ := http.NewRequest("GET", "/api/v1/apps/test-app", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var app App
	json.Unmarshal(w.Body.Bytes(), &app)
	assert.Equal(t, "Test App", app.Name)
}

func TestHandler_HandleAppByID_NotFound(t *testing.T) {
	h := NewHandler(NewManager())
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	req, _ := http.NewRequest("GET", "/api/v1/apps/nonexistent", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestHandler_HandleSearch(t *testing.T) {
	m := NewManager()
	m.apps["test"] = &App{ID: "test", Name: "Test App", Description: "Test desc", Category: "utilities"}
	h := NewHandler(m)
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	body, _ := json.Marshal(AppSearchRequest{Query: "Test", Page: 1, PageSize: 20})
	req, _ := http.NewRequest("POST", "/api/v1/apps/search", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var result AppSearchResult
	json.Unmarshal(w.Body.Bytes(), &result)
	assert.Equal(t, 1, result.Total)
}

func TestHandler_HandleSearch_InvalidBody(t *testing.T) {
	h := NewHandler(NewManager())
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	req, _ := http.NewRequest("POST", "/api/v1/apps/search", bytes.NewReader([]byte("invalid")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandler_HandleCategories(t *testing.T) {
	h := NewHandler(NewManager())
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	req, _ := http.NewRequest("GET", "/api/v1/apps/categories", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var cats []string
	json.Unmarshal(w.Body.Bytes(), &cats)
	assert.NotEmpty(t, cats)
}

func TestHandler_HandleStats(t *testing.T) {
	h := NewHandler(NewManager())
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	req, _ := http.NewRequest("GET", "/api/v1/apps/stats", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestHandler_HandleInstalled(t *testing.T) {
	h := NewHandler(NewManager())
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	req, _ := http.NewRequest("GET", "/api/v1/installed", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestHandler_HandleInstalledByID_Delete(t *testing.T) {
	m := NewManager()
	m.installed["test-app"] = &InstalledApp{AppID: "test-app", Status: "running"}
	h := NewHandler(m)
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	req, _ := http.NewRequest("DELETE", "/api/v1/installed/test-app", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestHandler_HandleInstalledByID_NotFound(t *testing.T) {
	h := NewHandler(NewManager())
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	req, _ := http.NewRequest("DELETE", "/api/v1/installed/nonexistent", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestHandler_HandleInstall(t *testing.T) {
	m := NewManager()
	m.apps["test-app"] = &App{ID: "test-app", Name: "Test", Version: "1.0"}
	h := NewHandler(m)
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	body, _ := json.Marshal(InstallRequest{AppID: "test-app", AutoStart: true})
	req, _ := http.NewRequest("POST", "/api/v1/install", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	assert.Equal(t, http.StatusAccepted, w.Code)
}

func TestHandler_HandleInstall_InvalidBody(t *testing.T) {
	h := NewHandler(NewManager())
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	req, _ := http.NewRequest("POST", "/api/v1/install", bytes.NewReader([]byte("bad")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandler_HandleInstallStatus(t *testing.T) {
	m := NewManager()
	m.installs["inst-1"] = &InstallStatus{ID: "inst-1", AppID: "test", Status: "completed"}
	h := NewHandler(m)
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	req, _ := http.NewRequest("GET", "/api/v1/install/inst-1", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestHandler_HandleInstallStatus_NotFound(t *testing.T) {
	h := NewHandler(NewManager())
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	req, _ := http.NewRequest("GET", "/api/v1/install/nonexistent", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}


