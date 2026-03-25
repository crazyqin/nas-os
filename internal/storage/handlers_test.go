package storage

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

// setupTestRouter 创建测试路由.
func setupTestRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	return router
}

// setupTestHandlers 创建测试处理器.
func setupTestHandlers() (*Manager, *Handlers, *gin.Engine) {
	router := setupTestRouter()
	mgr := &Manager{
		volumes:   make(map[string]*Volume),
		mountBase: "/tmp/test-mnt",
	}
	handlers := NewHandlers(mgr, nil, nil, nil)
	return mgr, handlers, router
}

// ========== 处理器创建测试 ==========

func TestNewHandlers(t *testing.T) {
	mgr := &Manager{
		volumes:   make(map[string]*Volume),
		mountBase: "/tmp/test",
	}

	h := NewHandlers(mgr, nil, nil, nil)
	if h == nil {
		t.Fatal("NewHandlers returned nil")
	}
	if h.manager != mgr {
		t.Error("Handler manager not set correctly")
	}
}

func TestRegisterRoutes(t *testing.T) {
	router := setupTestRouter()
	mgr := &Manager{
		volumes:   make(map[string]*Volume),
		mountBase: "/tmp/test",
	}
	handlers := NewHandlers(mgr, nil, nil, nil)

	// 注册路由
	api := router.Group("/api/storage")
	handlers.RegisterRoutes(api)

	// 验证路由是否正确注册
	routes := router.Routes()
	routeMap := make(map[string]bool)
	for _, route := range routes {
		routeMap[route.Path] = true
	}

	// 检查关键路由
	expectedRoutes := []string{
		"/api/storage/volumes",
		"/api/storage/subvolumes",
		"/api/storage/snapshots",
		"/api/storage/raid-configs",
	}

	for _, expected := range expectedRoutes {
		if !routeMap[expected] {
			t.Errorf("Expected route %s not found", expected)
		}
	}
}

// ========== 卷列表测试 ==========

func TestListVolumes_Empty(t *testing.T) {
	_, handlers, router := setupTestHandlers()

	api := router.Group("/api/storage")
	handlers.RegisterRoutes(api)

	req, _ := http.NewRequestWithContext(context.Background(), "GET", "/api/storage/volumes", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	data, ok := response["data"].([]interface{})
	if !ok {
		t.Fatal("Expected data to be an array")
	}
	if len(data) != 0 {
		t.Errorf("Expected empty array, got %d items", len(data))
	}
}

func TestListVolumes_WithVolumes(t *testing.T) {
	mgr, handlers, router := setupTestHandlers()

	// 添加测试卷
	mgr.volumes["vol1"] = &Volume{
		Name:        "vol1",
		UUID:        "uuid-1",
		Devices:     []string{"/dev/sda1"},
		Size:        1000000000000,
		Used:        500000000000,
		Free:        500000000000,
		DataProfile: "raid1",
		MountPoint:  "/mnt/vol1",
		Status:      VolumeStatus{Healthy: true},
		Subvolumes:  []*SubVolume{{Name: "docs"}},
	}

	api := router.Group("/api/storage")
	handlers.RegisterRoutes(api)

	req, _ := http.NewRequestWithContext(context.Background(), "GET", "/api/storage/volumes", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	data, ok := response["data"].([]interface{})
	if !ok {
		t.Fatal("Expected data to be an array")
	}
	if len(data) != 1 {
		t.Errorf("Expected 1 volume, got %d", len(data))
	}
}

// ========== 卷详情测试 ==========

func TestGetVolume_Found(t *testing.T) {
	mgr, handlers, router := setupTestHandlers()

	mgr.volumes["data"] = &Volume{
		Name:        "data",
		UUID:        "test-uuid",
		Devices:     []string{"/dev/sda1"},
		DataProfile: "raid1",
	}

	api := router.Group("/api/storage")
	handlers.RegisterRoutes(api)

	req, _ := http.NewRequestWithContext(context.Background(), "GET", "/api/storage/volumes/data", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestGetVolume_NotFound(t *testing.T) {
	_, handlers, router := setupTestHandlers()

	api := router.Group("/api/storage")
	handlers.RegisterRoutes(api)

	req, _ := http.NewRequestWithContext(context.Background(), "GET", "/api/storage/volumes/nonexistent", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", w.Code)
	}
}

// ========== 创建卷测试 ==========

func TestCreateVolume_ValidRequest(t *testing.T) {
	// 注意：创建卷需要实际的 btrfs 操作
	// 这个测试主要验证请求解析和路由
	// 跳过实际创建测试以避免 panic
	t.Skip("Skipping test that requires actual btrfs operations")
}

func TestCreateVolume_MissingName(t *testing.T) {
	_, handlers, router := setupTestHandlers()

	api := router.Group("/api/storage")
	handlers.RegisterRoutes(api)

	body := CreateVolumeRequest{
		Devices: []string{"/dev/sda1"},
	}
	jsonBody, _ := json.Marshal(body)

	req, _ := http.NewRequestWithContext(context.Background(), "POST", "/api/storage/volumes", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400 for missing name, got %d", w.Code)
	}
}

func TestCreateVolume_MissingDevices(t *testing.T) {
	_, handlers, router := setupTestHandlers()

	api := router.Group("/api/storage")
	handlers.RegisterRoutes(api)

	body := CreateVolumeRequest{
		Name: "test-vol",
	}
	jsonBody, _ := json.Marshal(body)

	req, _ := http.NewRequestWithContext(context.Background(), "POST", "/api/storage/volumes", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400 for missing devices, got %d", w.Code)
	}
}

// ========== 删除卷测试 ==========

func TestDeleteVolume_NotFound(t *testing.T) {
	_, handlers, router := setupTestHandlers()

	api := router.Group("/api/storage")
	handlers.RegisterRoutes(api)

	req, _ := http.NewRequestWithContext(context.Background(), "DELETE", "/api/storage/volumes/nonexistent", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400 for nonexistent volume, got %d", w.Code)
	}
}

// ========== 挂载卷测试 ==========

func TestMountVolume_NotFound(t *testing.T) {
	_, handlers, router := setupTestHandlers()

	api := router.Group("/api/storage")
	handlers.RegisterRoutes(api)

	req, _ := http.NewRequestWithContext(context.Background(), "POST", "/api/storage/volumes/nonexistent/mount", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400 for nonexistent volume, got %d", w.Code)
	}
}

func TestUnmountVolume_NotFound(t *testing.T) {
	_, handlers, router := setupTestHandlers()

	api := router.Group("/api/storage")
	handlers.RegisterRoutes(api)

	req, _ := http.NewRequestWithContext(context.Background(), "POST", "/api/storage/volumes/nonexistent/unmount", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400 for nonexistent volume, got %d", w.Code)
	}
}

// ========== Scrub 测试 ==========

func TestStartScrub_NotFound(t *testing.T) {
	_, handlers, router := setupTestHandlers()

	api := router.Group("/api/storage")
	handlers.RegisterRoutes(api)

	req, _ := http.NewRequestWithContext(context.Background(), "POST", "/api/storage/volumes/nonexistent/scrub", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestGetScrubStatus_NotFound(t *testing.T) {
	_, handlers, router := setupTestHandlers()

	api := router.Group("/api/storage")
	handlers.RegisterRoutes(api)

	req, _ := http.NewRequestWithContext(context.Background(), "GET", "/api/storage/volumes/nonexistent/scrub/status", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

// ========== Balance 测试 ==========

func TestStartBalance_NotFound(t *testing.T) {
	_, handlers, router := setupTestHandlers()

	api := router.Group("/api/storage")
	handlers.RegisterRoutes(api)

	req, _ := http.NewRequestWithContext(context.Background(), "POST", "/api/storage/volumes/nonexistent/balance", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestGetBalanceStatus_NotFound(t *testing.T) {
	_, handlers, router := setupTestHandlers()

	api := router.Group("/api/storage")
	handlers.RegisterRoutes(api)

	req, _ := http.NewRequestWithContext(context.Background(), "GET", "/api/storage/volumes/nonexistent/balance/status", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

// ========== 子卷列表测试 ==========

func TestListSubvolumes_VolumeNotFound(t *testing.T) {
	_, handlers, router := setupTestHandlers()

	api := router.Group("/api/storage")
	handlers.RegisterRoutes(api)

	req, _ := http.NewRequestWithContext(context.Background(), "GET", "/api/storage/volumes/nonexistent/subvolumes", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestListSubvolumes_WithSubvolumes(t *testing.T) {
	// 跳过测试，因为需要实际的 btrfs 客户端
	t.Skip("Skipping test that requires actual btrfs client")
}

// ========== 全局子卷列表测试 ==========

func TestListAllSubvolumes_Empty(t *testing.T) {
	_, handlers, router := setupTestHandlers()

	api := router.Group("/api/storage")
	handlers.RegisterRoutes(api)

	req, _ := http.NewRequestWithContext(context.Background(), "GET", "/api/storage/subvolumes", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	data, ok := response["data"].([]interface{})
	if !ok {
		t.Fatal("Expected data to be an array")
	}
	if len(data) != 0 {
		t.Errorf("Expected empty array, got %d items", len(data))
	}
}

func TestListAllSubvolumes_WithFilter(t *testing.T) {
	mgr, handlers, router := setupTestHandlers()

	mgr.volumes["data"] = &Volume{
		Name:       "data",
		MountPoint: "/mnt/data",
		Subvolumes: []*SubVolume{
			{Name: "documents", ID: 256, Path: "/mnt/data/documents"},
		},
	}
	mgr.volumes["backup"] = &Volume{
		Name:       "backup",
		MountPoint: "/mnt/backup",
		Subvolumes: []*SubVolume{
			{Name: "archives", ID: 258, Path: "/mnt/backup/archives"},
		},
	}

	api := router.Group("/api/storage")
	handlers.RegisterRoutes(api)

	// 只获取 data 卷的子卷
	req, _ := http.NewRequestWithContext(context.Background(), "GET", "/api/storage/subvolumes?volume=data", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	data, ok := response["data"].([]interface{})
	if !ok {
		t.Fatal("Expected data to be an array")
	}
	if len(data) != 1 {
		t.Errorf("Expected 1 subvolume with filter, got %d", len(data))
	}
}

// ========== 子卷详情测试 ==========

func TestGetSubvolume_Found(t *testing.T) {
	mgr, handlers, router := setupTestHandlers()

	mgr.volumes["data"] = &Volume{
		Name:       "data",
		MountPoint: "/mnt/data",
		Subvolumes: []*SubVolume{
			{Name: "documents", ID: 256, Path: "/mnt/data/documents"},
		},
	}

	api := router.Group("/api/storage")
	handlers.RegisterRoutes(api)

	req, _ := http.NewRequestWithContext(context.Background(), "GET", "/api/storage/volumes/data/subvolumes/documents", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestGetSubvolume_NotFound(t *testing.T) {
	mgr, handlers, router := setupTestHandlers()

	mgr.volumes["data"] = &Volume{
		Name:       "data",
		MountPoint: "/mnt/data",
		Subvolumes: []*SubVolume{},
	}

	api := router.Group("/api/storage")
	handlers.RegisterRoutes(api)

	req, _ := http.NewRequestWithContext(context.Background(), "GET", "/api/storage/volumes/data/subvolumes/nonexistent", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", w.Code)
	}
}

// ========== 创建子卷测试 ==========

func TestCreateSubvolume_ValidRequest(t *testing.T) {
	// 跳过测试，因为需要实际的 btrfs 客户端
	t.Skip("Skipping test that requires actual btrfs client")
}

func TestCreateSubvolume_MissingName(t *testing.T) {
	_, handlers, router := setupTestHandlers()

	api := router.Group("/api/storage")
	handlers.RegisterRoutes(api)

	body := CreateSubvolumeRequest{}
	jsonBody, _ := json.Marshal(body)

	req, _ := http.NewRequestWithContext(context.Background(), "POST", "/api/storage/volumes/data/subvolumes", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400 for missing name, got %d", w.Code)
	}
}

// ========== 删除子卷测试 ==========

func TestDeleteSubvolume_NotFound(t *testing.T) {
	mgr, handlers, router := setupTestHandlers()

	mgr.volumes["data"] = &Volume{
		Name:       "data",
		MountPoint: "/mnt/data",
		Subvolumes: []*SubVolume{},
	}

	api := router.Group("/api/storage")
	handlers.RegisterRoutes(api)

	req, _ := http.NewRequestWithContext(context.Background(), "DELETE", "/api/storage/volumes/data/subvolumes/nonexistent", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

// ========== 挂载子卷测试 ==========

func TestMountSubvolume_ValidRequest(t *testing.T) {
	// 跳过测试，因为需要实际的 btrfs 客户端
	t.Skip("Skipping test that requires actual btrfs client")
}

func TestMountSubvolume_SubvolNotFound(t *testing.T) {
	mgr, handlers, router := setupTestHandlers()

	mgr.volumes["data"] = &Volume{
		Name:       "data",
		MountPoint: "/mnt/data",
		Subvolumes: []*SubVolume{},
	}

	api := router.Group("/api/storage")
	handlers.RegisterRoutes(api)

	body := MountSubvolumeRequest{MountPath: "/mnt/documents"}
	jsonBody, _ := json.Marshal(body)

	req, _ := http.NewRequestWithContext(context.Background(), "POST", "/api/storage/volumes/data/subvolumes/nonexistent/mount", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

// ========== 设置子卷只读测试 ==========

func TestSetSubvolumeReadOnly(t *testing.T) {
	// 跳过测试，因为需要实际的 btrfs 客户端
	t.Skip("Skipping test that requires actual btrfs client")
}

// ========== 快照列表测试 ==========

func TestListSnapshots_VolumeNotFound(t *testing.T) {
	_, handlers, router := setupTestHandlers()

	api := router.Group("/api/storage")
	handlers.RegisterRoutes(api)

	req, _ := http.NewRequestWithContext(context.Background(), "GET", "/api/storage/volumes/nonexistent/snapshots", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestListSnapshots_WithSnapshots(t *testing.T) {
	mgr, handlers, router := setupTestHandlers()

	mgr.volumes["data"] = &Volume{
		Name:       "data",
		MountPoint: "/mnt/data",
		Subvolumes: []*SubVolume{
			{
				Name: "documents",
				Snapshots: []*Snapshot{
					{Name: "snap1", ReadOnly: true, Source: "documents"},
					{Name: "manual-backup", ReadOnly: true, Source: "documents"},
				},
			},
		},
	}

	api := router.Group("/api/storage")
	handlers.RegisterRoutes(api)

	req, _ := http.NewRequestWithContext(context.Background(), "GET", "/api/storage/volumes/data/snapshots", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

// ========== 全局快照列表测试 ==========

func TestListAllSnapshots(t *testing.T) {
	mgr, handlers, router := setupTestHandlers()

	mgr.volumes["data"] = &Volume{
		Name:       "data",
		MountPoint: "/mnt/data",
		Subvolumes: []*SubVolume{
			{
				Name: "documents",
				Snapshots: []*Snapshot{
					{Name: "snap1", ReadOnly: true},
				},
			},
		},
	}

	api := router.Group("/api/storage")
	handlers.RegisterRoutes(api)

	req, _ := http.NewRequestWithContext(context.Background(), "GET", "/api/storage/snapshots", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

// ========== 快照详情测试 ==========

func TestGetSnapshot_Found(t *testing.T) {
	// 跳过测试，因为需要实际的 btrfs 客户端
	t.Skip("Skipping test that requires actual btrfs client")
}

// ========== 创建快照测试 ==========

func TestCreateSnapshot_ValidRequest(t *testing.T) {
	// 跳过测试，因为需要实际的 btrfs 客户端
	t.Skip("Skipping test that requires actual btrfs client")
}

// ========== 删除快照测试 ==========

func TestDeleteSnapshot_NotFound(t *testing.T) {
	_, handlers, router := setupTestHandlers()

	api := router.Group("/api/storage")
	handlers.RegisterRoutes(api)

	req, _ := http.NewRequestWithContext(context.Background(), "DELETE", "/api/storage/volumes/nonexistent/snapshots/snap1", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

// ========== 恢复快照测试 ==========

func TestRestoreSnapshot_ValidRequest(t *testing.T) {
	// 跳过测试，因为需要实际的 btrfs 客户端
	t.Skip("Skipping test that requires actual btrfs client")
}

// ========== 回滚快照测试 ==========

func TestRollbackSnapshot_ValidRequest(t *testing.T) {
	// 跳过测试，因为需要实际的 btrfs 客户端
	t.Skip("Skipping test that requires actual btrfs client")
}

// ========== 设备统计测试 ==========

func TestGetDeviceStats_NotFound(t *testing.T) {
	_, handlers, router := setupTestHandlers()

	api := router.Group("/api/storage")
	handlers.RegisterRoutes(api)

	req, _ := http.NewRequestWithContext(context.Background(), "GET", "/api/storage/volumes/nonexistent/devices", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

// ========== 添加设备测试 ==========

func TestAddDevice_ValidRequest(t *testing.T) {
	// 跳过测试，因为需要实际的 btrfs 客户端
	t.Skip("Skipping test that requires actual btrfs client")
}

// ========== 移除设备测试 ==========

func TestRemoveDevice_NotFound(t *testing.T) {
	_, handlers, router := setupTestHandlers()

	api := router.Group("/api/storage")
	handlers.RegisterRoutes(api)

	req, _ := http.NewRequestWithContext(context.Background(), "DELETE", "/api/storage/volumes/nonexistent/devices/sda1", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

// ========== RAID 转换测试 ==========

func TestConvertRAID_ValidRequest(t *testing.T) {
	// 跳过测试，因为需要实际的 btrfs 客户端
	t.Skip("Skipping test that requires actual btrfs client")
}

// ========== RAID 配置测试 ==========

func TestGetRAIDConfigs(t *testing.T) {
	_, handlers, router := setupTestHandlers()

	api := router.Group("/api/storage")
	handlers.RegisterRoutes(api)

	req, _ := http.NewRequestWithContext(context.Background(), "GET", "/api/storage/raid-configs", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	data, ok := response["data"].(map[string]interface{})
	if !ok {
		t.Fatal("Expected data to be a map")
	}

	// 验证所有 RAID 配置
	expectedConfigs := []string{"single", "raid0", "raid1", "raid5", "raid6", "raid10"}
	for _, config := range expectedConfigs {
		if _, exists := data[config]; !exists {
			t.Errorf("Expected RAID config %s not found", config)
		}
	}
}

// ========== 并发请求测试 ==========

func TestConcurrentRequests(t *testing.T) {
	mgr, handlers, router := setupTestHandlers()

	mgr.volumes["data"] = &Volume{
		Name:       "data",
		MountPoint: "/mnt/data",
		Subvolumes: []*SubVolume{
			{Name: "documents"},
		},
	}

	api := router.Group("/api/storage")
	handlers.RegisterRoutes(api)

	// 并发发送请求
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func() {
			req, _ := http.NewRequestWithContext(context.Background(), "GET", "/api/storage/volumes", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)
			done <- true
		}()
	}

	// 等待所有请求完成
	for i := 0; i < 10; i++ {
		<-done
	}
}
