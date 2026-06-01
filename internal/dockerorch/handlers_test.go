package dockerorch

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func setupTestRouter(handler *Handler) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	api := router.Group("/api/v1")
	handler.RegisterRoutes(api)
	return router
}

func TestNewHandler(t *testing.T) {
	orch := NewOrchestrator()
	handler := NewHandler(orch)
	if handler == nil {
		t.Fatal("NewHandler 返回 nil")
	}
	if handler.orchestrator != orch {
		t.Fatal("orchestrator 指针不匹配")
	}
}

func TestRegisterRoutes(t *testing.T) {
	orch := NewOrchestrator()
	handler := NewHandler(orch)
	router := setupTestRouter(handler)

	routes := router.Routes()
	expectedRoutes := []string{
		"/api/v1/docker-orch/containers",
		"/api/v1/docker-orch/containers/:id",
		"/api/v1/docker-orch/containers/:id/start",
		"/api/v1/docker-orch/containers/:id/stop",
		"/api/v1/docker-orch/containers/:id",
		"/api/v1/docker-orch/services",
		"/api/v1/docker-orch/stacks",
	}

	routeMap := make(map[string]bool)
	for _, r := range routes {
		routeMap[r.Path] = true
	}

	for _, expected := range expectedRoutes {
		if !routeMap[expected] {
			t.Errorf("路由 %s 未注册", expected)
		}
	}
}

func TestListContainers(t *testing.T) {
	orch := NewOrchestrator()
	handler := NewHandler(orch)
	router := setupTestRouter(handler)

	// 添加测试容器
	orch.CreateContainer(Container{
		ID:     "c1",
		Name:   "test1",
		Image:  "nginx",
		Status: ContainerStatusRunning,
	})
	orch.CreateContainer(Container{
		ID:     "c2",
		Name:   "test2",
		Image:  "redis",
		Status: ContainerStatusExited,
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/docker-orch/containers", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("期望状态码 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("解析响应失败: %v", err)
	}

	if resp["total"].(float64) != 2 {
		t.Fatalf("期望 2 个容器, got %v", resp["total"])
	}
}

func TestListContainersWithStatus(t *testing.T) {
	orch := NewOrchestrator()
	handler := NewHandler(orch)
	router := setupTestRouter(handler)

	orch.CreateContainer(Container{
		ID:     "c1",
		Name:   "test1",
		Image:  "nginx",
		Status: ContainerStatusRunning,
	})
	orch.CreateContainer(Container{
		ID:     "c2",
		Name:   "test2",
		Image:  "redis",
		Status: ContainerStatusExited,
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/docker-orch/containers?status=running", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("期望状态码 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("解析响应失败: %v", err)
	}

	if resp["total"].(float64) != 1 {
		t.Fatalf("期望 1 个运行中容器, got %v", resp["total"])
	}
}

func TestCreateContainer(t *testing.T) {
	orch := NewOrchestrator()
	handler := NewHandler(orch)
	router := setupTestRouter(handler)

	body := CreateContainerRequest{
		ID:    "c1",
		Name:  "test",
		Image: "nginx",
	}
	jsonBody, _ := json.Marshal(body)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/docker-orch/containers", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("期望状态码 201, got %d", w.Code)
	}

	var resp Container
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("解析响应失败: %v", err)
	}

	if resp.ID != "c1" {
		t.Fatalf("容器ID不匹配: %s", resp.ID)
	}
	if resp.Status != ContainerStatusCreated {
		t.Fatalf("期望状态 created, got %s", resp.Status)
	}
	if resp.Tag != "latest" {
		t.Fatalf("期望默认 tag latest, got %s", resp.Tag)
	}
}

func TestCreateContainerInvalidRequest(t *testing.T) {
	orch := NewOrchestrator()
	handler := NewHandler(orch)
	router := setupTestRouter(handler)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/docker-orch/containers", bytes.NewBufferString("{}"))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("期望状态码 400, got %d", w.Code)
	}
}

func TestGetContainer(t *testing.T) {
	orch := NewOrchestrator()
	handler := NewHandler(orch)
	router := setupTestRouter(handler)

	orch.CreateContainer(Container{
		ID:     "c1",
		Name:   "test",
		Image:  "nginx",
		Status: ContainerStatusRunning,
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/docker-orch/containers/c1", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("期望状态码 200, got %d", w.Code)
	}

	var resp Container
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("解析响应失败: %v", err)
	}

	if resp.ID != "c1" {
		t.Fatalf("容器ID不匹配: %s", resp.ID)
	}
}

func TestGetContainerNotFound(t *testing.T) {
	orch := NewOrchestrator()
	handler := NewHandler(orch)
	router := setupTestRouter(handler)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/docker-orch/containers/nonexistent", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("期望状态码 404, got %d", w.Code)
	}
}

func TestStartContainer(t *testing.T) {
	orch := NewOrchestrator()
	handler := NewHandler(orch)
	router := setupTestRouter(handler)

	orch.CreateContainer(Container{
		ID:     "c1",
		Name:   "test",
		Image:  "nginx",
		Status: ContainerStatusCreated,
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/docker-orch/containers/c1/start", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("期望状态码 200, got %d", w.Code)
	}

	// 验证状态已更新
	container, _ := orch.GetContainer("c1")
	if container.Status != ContainerStatusRunning {
		t.Fatalf("期望状态 running, got %s", container.Status)
	}
}

func TestStopContainer(t *testing.T) {
	orch := NewOrchestrator()
	handler := NewHandler(orch)
	router := setupTestRouter(handler)

	orch.CreateContainer(Container{
		ID:     "c1",
		Name:   "test",
		Image:  "nginx",
		Status: ContainerStatusRunning,
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/docker-orch/containers/c1/stop", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("期望状态码 200, got %d", w.Code)
	}

	// 验证状态已更新
	container, _ := orch.GetContainer("c1")
	if container.Status != ContainerStatusExited {
		t.Fatalf("期望状态 exited, got %s", container.Status)
	}
}

func TestDeleteContainer(t *testing.T) {
	orch := NewOrchestrator()
	handler := NewHandler(orch)
	router := setupTestRouter(handler)

	orch.CreateContainer(Container{
		ID:     "c1",
		Name:   "test",
		Image:  "nginx",
		Status: ContainerStatusExited,
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("DELETE", "/api/v1/docker-orch/containers/c1", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("期望状态码 200, got %d", w.Code)
	}

	// 验证容器已删除
	_, err := orch.GetContainer("c1")
	if err == nil {
		t.Fatal("容器应该已被删除")
	}
}

func TestListServices(t *testing.T) {
	orch := NewOrchestrator()
	handler := NewHandler(orch)
	router := setupTestRouter(handler)

	// 添加测试服务
	orch.CreateService(Service{
		ID:     "s1",
		Name:   "web",
		Image:  "nginx",
		Status: ServiceStatusRunning,
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/docker-orch/services", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("期望状态码 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("解析响应失败: %v", err)
	}

	if resp["total"].(float64) != 1 {
		t.Fatalf("期望 1 个服务, got %v", resp["total"])
	}
}

func TestListStacks(t *testing.T) {
	orch := NewOrchestrator()
	handler := NewHandler(orch)
	router := setupTestRouter(handler)

	// 添加测试栈
	orch.CreateStack(Stack{
		ID:   "stack1",
		Name: "webapp",
		Services: []Service{
			{ID: "s1", Name: "web", Image: "nginx"},
		},
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/docker-orch/stacks", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("期望状态码 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("解析响应失败: %v", err)
	}

	if resp["total"].(float64) != 1 {
		t.Fatalf("期望 1 个栈, got %v", resp["total"])
	}
}
