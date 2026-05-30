// Package containerpro 提供容器管理功能
package containerpro

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func setupTestRouter() (*gin.Engine, *Manager) {
	gin.SetMode(gin.TestMode)
	
	config := &ContainerProConfig{
		DockerHost:      "unix:///var/run/docker.sock",
		ComposePath:     "/opt/compose",
		RegistryMirrors: []string{"https://mirror.example.com"},
		DefaultRegistry: "docker.io",
		AutoRestart:     true,
		LogMaxSize:      "100m",
	}
	
	manager := NewManager(config)
	handlers := NewHandlers(manager)
	
	r := gin.New()
	api := r.Group("/api")
	handlers.RegisterRoutes(api)
	
	return r, manager
}

func addTestContainer(manager *Manager, id, name, state string) {
	now := time.Now()
	container := &Container{
		ID:          id,
		Name:        name,
		Image:       "nginx:latest",
		Status:      "Up 2 hours",
		State:       state,
		CreatedAt:   now,
		Ports:       []PortMapping{{HostPort: "8080", ContainerPort: "80", Protocol: "tcp"}},
		MemoryUsage: 1024 * 1024 * 50,
		MemoryLimit: 1024 * 1024 * 512,
	}
	manager.containers[id] = container
}

func TestListContainers(t *testing.T) {
	r, manager := setupTestRouter()
	
	addTestContainer(manager, "c1", "web", "running")
	addTestContainer(manager, "c2", "db", "exited")
	
	// 测试只返回运行中的容器
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/containers", nil)
	r.ServeHTTP(w, req)
	
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
	
	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	data := resp["data"].([]interface{})
	if len(data) != 1 {
		t.Errorf("Expected 1 running container, got %d", len(data))
	}
	
	// 测试返回所有容器
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/api/v1/containers?all=true", nil)
	r.ServeHTTP(w, req)
	
	json.Unmarshal(w.Body.Bytes(), &resp)
	data = resp["data"].([]interface{})
	if len(data) != 2 {
		t.Errorf("Expected 2 containers, got %d", len(data))
	}
}

func TestGetContainer(t *testing.T) {
	r, manager := setupTestRouter()
	
	addTestContainer(manager, "c1", "web", "running")
	
	// 测试获取存在的容器
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/containers/c1", nil)
	r.ServeHTTP(w, req)
	
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
	
	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	container := resp["data"].(map[string]interface{})
	if container["name"] != "web" {
		t.Errorf("Expected name 'web', got %v", container["name"])
	}
	
	// 测试获取不存在的容器
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/api/v1/containers/notexist", nil)
	r.ServeHTTP(w, req)
	
	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", w.Code)
	}
}

func TestStartStopContainer(t *testing.T) {
	r, manager := setupTestRouter()
	
	addTestContainer(manager, "c1", "web", "exited")
	
	// 测试启动容器
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/containers/c1/start", nil)
	r.ServeHTTP(w, req)
	
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
	
	// 验证容器已启动
	container, _ := manager.GetContainer("c1")
	if container.State != "running" {
		t.Errorf("Expected state 'running', got %s", container.State)
	}
	
	// 测试停止容器
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("POST", "/api/v1/containers/c1/stop?timeout=5", nil)
	r.ServeHTTP(w, req)
	
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
	
	container, _ = manager.GetContainer("c1")
	if container.State != "exited" {
		t.Errorf("Expected state 'exited', got %s", container.State)
	}
}

func TestRestartContainer(t *testing.T) {
	r, manager := setupTestRouter()
	
	addTestContainer(manager, "c1", "web", "running")
	
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/containers/c1/restart", nil)
	r.ServeHTTP(w, req)
	
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
	
	container, _ := manager.GetContainer("c1")
	if container.RestartCount != 1 {
		t.Errorf("Expected restart count 1, got %d", container.RestartCount)
	}
}

func TestRemoveContainer(t *testing.T) {
	r, manager := setupTestRouter()
	
	addTestContainer(manager, "c1", "web", "exited")
	addTestContainer(manager, "c2", "web-running", "running")
	
	// 测试删除停止的容器
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("DELETE", "/api/v1/containers/c1", nil)
	r.ServeHTTP(w, req)
	
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
	
	// 测试删除运行中的容器（不带 force）
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("DELETE", "/api/v1/containers/c2", nil)
	r.ServeHTTP(w, req)
	
	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", w.Code)
	}
	
	// 测试强制删除运行中的容器
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("DELETE", "/api/v1/containers/c2?force=true", nil)
	r.ServeHTTP(w, req)
	
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestGetContainerStats(t *testing.T) {
	r, manager := setupTestRouter()
	
	addTestContainer(manager, "c1", "web", "running")
	
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/containers/c1/stats", nil)
	r.ServeHTTP(w, req)
	
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
	
	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	data := resp["data"].(map[string]interface{})
	if data["pids"] == nil {
		t.Error("Expected pids field in stats")
	}
}

func TestComposeOperations(t *testing.T) {
	r, manager := setupTestRouter()
	
	// 测试部署 Compose 项目
	body := `{"path": "/opt/compose/webapp"}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/compose/deploy", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	
	if w.Code != http.StatusCreated {
		t.Errorf("Expected status 201, got %d", w.Code)
	}
	
	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	data := resp["data"].(map[string]interface{})
	projectID := data["id"].(string)
	
	// 测试列出 Compose 项目
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/api/v1/compose/projects", nil)
	r.ServeHTTP(w, req)
	
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
	
	json.Unmarshal(w.Body.Bytes(), &resp)
	projects := resp["data"].([]interface{})
	if len(projects) != 1 {
		t.Errorf("Expected 1 project, got %d", len(projects))
	}
	
	// 测试停止 Compose 项目
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("POST", "/api/v1/compose/projects/"+projectID+"/stop", nil)
	r.ServeHTTP(w, req)
	
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
	
	project, _ := manager.composeProjects[projectID]
	if project.Status != "stopped" {
		t.Errorf("Expected status 'stopped', got %s", project.Status)
	}
}

func TestImageOperations(t *testing.T) {
	r, _ := setupTestRouter()
	
	// 测试拉取镜像
	body := `{"image": "nginx:latest"}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/images/pull", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
	
	// 测试列出镜像
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/api/v1/images", nil)
	r.ServeHTTP(w, req)
	
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
	
	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	images := resp["data"].([]interface{})
	if len(images) != 1 {
		t.Errorf("Expected 1 image, got %d", len(images))
	}
}

func TestRegistryOperations(t *testing.T) {
	r, _ := setupTestRouter()
	
	// 测试添加仓库
	body := `{
		"name": "My Registry",
		"url": "https://registry.example.com",
		"type": "Harbor",
		"username": "admin"
	}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/registries", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	
	if w.Code != http.StatusCreated {
		t.Errorf("Expected status 201, got %d", w.Code)
	}
	
	// 测试列出仓库
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/api/v1/registries", nil)
	r.ServeHTTP(w, req)
	
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
	
	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	registries := resp["data"].([]interface{})
	if len(registries) != 1 {
		t.Errorf("Expected 1 registry, got %d", len(registries))
	}
	
	// 验证仓库内容
	registry := registries[0].(map[string]interface{})
	if registry["name"] != "My Registry" {
		t.Errorf("Expected name 'My Registry', got %v", registry["name"])
	}
}

func TestNewManager(t *testing.T) {
	config := &ContainerProConfig{
		DockerHost:  "unix:///var/run/docker.sock",
		AutoRestart: true,
	}
	
	manager := NewManager(config)
	
	if manager == nil {
		t.Fatal("Expected manager to be created")
	}
	
	if manager.config.DockerHost != config.DockerHost {
		t.Errorf("Expected DockerHost %s, got %s", config.DockerHost, manager.config.DockerHost)
	}
	
	if len(manager.containers) != 0 {
		t.Error("Expected empty containers map")
	}
}
