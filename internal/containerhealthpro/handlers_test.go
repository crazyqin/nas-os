package containerhealthpro

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func setupTestRouter() (*gin.Engine, *Handlers) {
	gin.SetMode(gin.TestMode)
	manager := NewManager()
	handlers := NewHandlers(manager)
	router := gin.New()
	handlers.RegisterRoutes(router.Group("/api"))
	return router, handlers
}

func TestRegisterContainer(t *testing.T) {
	router, _ := setupTestRouter()

	container := ContainerHealthPro{
		ContainerID: "test-container-1",
		Name:        "web-server",
		Image:       "nginx:latest",
		HealthCheck: HealthCheckConfig{
			Type:     HealthCheckHTTP,
			Endpoint: "http://localhost:8080/health",
			Interval: 30,
			Timeout:  5,
		},
		AutoRestart:    true,
		RecoveryPolicy: RecoveryRestart,
	}

	body, _ := json.Marshal(container)
	req := httptest.NewRequest("POST", "/api/container-health-pro/containers", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("注册容器失败，状态码: %d, 响应: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["code"] != float64(0) {
		t.Fatalf("注册容器失败，响应: %v", resp)
	}
}

func TestListContainers(t *testing.T) {
	router, handlers := setupTestRouter()

	// 先注册一个容器
	manager := handlers.manager
	container := &ContainerHealthPro{
		ContainerID: "test-container-1",
		Name:        "web-server",
		Image:       "nginx:latest",
		HealthCheck: HealthCheckConfig{
			Type: HealthCheckTCP,
			Port: 8080,
		},
	}
	manager.RegisterContainer(container)

	req := httptest.NewRequest("GET", "/api/container-health-pro/containers", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("获取容器列表失败，状态码: %d", w.Code)
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	data := resp["data"].([]interface{})
	if len(data) != 1 {
		t.Fatalf("期望1个容器，实际 %d", len(data))
	}
}

func TestGetContainer(t *testing.T) {
	router, handlers := setupTestRouter()

	// 先注册一个容器
	manager := handlers.manager
	container := &ContainerHealthPro{
		ContainerID: "test-container-1",
		Name:        "web-server",
		Image:       "nginx:latest",
		HealthCheck: HealthCheckConfig{
			Type: HealthCheckTCP,
			Port: 8080,
		},
	}
	manager.RegisterContainer(container)

	req := httptest.NewRequest("GET", "/api/container-health-pro/containers/test-container-1", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("获取容器详情失败，状态码: %d", w.Code)
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	data := resp["data"].(map[string]interface{})
	if data["container_id"] != "test-container-1" {
		t.Fatalf("容器ID不匹配，期望 test-container-1，实际 %v", data["container_id"])
	}
}

func TestCheckHealth(t *testing.T) {
	router, handlers := setupTestRouter()

	// 先注册一个容器
	manager := handlers.manager
	container := &ContainerHealthPro{
		ContainerID: "test-container-1",
		Name:        "web-server",
		Image:       "nginx:latest",
		HealthCheck: HealthCheckConfig{
			Type:     HealthCheckTCP,
			Endpoint: "localhost",
			Port:     1, // 使用一个通常不通的端口
			Timeout:  1,
		},
	}
	manager.RegisterContainer(container)

	req := httptest.NewRequest("POST", "/api/container-health-pro/containers/test-container-1/check", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("检查健康状态失败，状态码: %d", w.Code)
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	data := resp["data"].(map[string]interface{})
	// 由于端口1不通，应该检查失败
	if data["fail_count"].(float64) < 1 {
		t.Fatal("健康检查应失败，fail_count 应大于 0")
	}
}

func TestUnregisterContainer(t *testing.T) {
	router, handlers := setupTestRouter()

	// 先注册一个容器
	manager := handlers.manager
	container := &ContainerHealthPro{
		ContainerID: "test-container-1",
		Name:        "web-server",
		Image:       "nginx:latest",
		HealthCheck: HealthCheckConfig{
			Type: HealthCheckTCP,
			Port: 8080,
		},
	}
	manager.RegisterContainer(container)

	req := httptest.NewRequest("DELETE", "/api/container-health-pro/containers/test-container-1", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("注销容器失败，状态码: %d", w.Code)
	}

	// 验证容器已被注销
	_, err := manager.GetContainer("test-container-1")
	if err == nil {
		t.Fatal("注销后容器仍存在")
	}
}

func TestSetAutoRestart(t *testing.T) {
	router, handlers := setupTestRouter()

	// 先注册一个容器
	manager := handlers.manager
	container := &ContainerHealthPro{
		ContainerID: "test-container-1",
		Name:        "web-server",
		Image:       "nginx:latest",
		HealthCheck: HealthCheckConfig{
			Type: HealthCheckTCP,
			Port: 8080,
		},
		AutoRestart: false,
	}
	manager.RegisterContainer(container)

	reqBody := map[string]interface{}{
		"enable": true,
	}
	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest("PUT", "/api/container-health-pro/containers/test-container-1/auto-restart", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("设置自动重启失败，状态码: %d", w.Code)
	}

	// 验证设置已更新
	c, _ := manager.GetContainer("test-container-1")
	if !c.AutoRestart {
		t.Fatal("自动重启应为 true")
	}
}

func TestSetRecoveryPolicy(t *testing.T) {
	router, handlers := setupTestRouter()

	// 先注册一个容器
	manager := handlers.manager
	container := &ContainerHealthPro{
		ContainerID: "test-container-1",
		Name:        "web-server",
		Image:       "nginx:latest",
		HealthCheck: HealthCheckConfig{
			Type: HealthCheckTCP,
			Port: 8080,
		},
		RecoveryPolicy: RecoveryRestart,
	}
	manager.RegisterContainer(container)

	reqBody := map[string]interface{}{
		"policy": "redeploy",
	}
	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest("PUT", "/api/container-health-pro/containers/test-container-1/recovery-policy", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("设置恢复策略失败，状态码: %d", w.Code)
	}

	// 验证设置已更新
	c, _ := manager.GetContainer("test-container-1")
	if c.RecoveryPolicy != RecoveryRedeploy {
		t.Fatalf("恢复策略应为 redeploy，实际 %s", c.RecoveryPolicy)
	}
}

func TestSetDependency(t *testing.T) {
	router, handlers := setupTestRouter()

	// 先注册两个容器
	manager := handlers.manager
	container1 := &ContainerHealthPro{
		ContainerID: "db",
		Name:        "database",
		Image:       "postgres:14",
		HealthCheck: HealthCheckConfig{Type: HealthCheckTCP, Port: 5432},
	}
	container2 := &ContainerHealthPro{
		ContainerID: "web",
		Name:        "web-server",
		Image:       "nginx:latest",
		HealthCheck: HealthCheckConfig{Type: HealthCheckTCP, Port: 8080},
	}
	manager.RegisterContainer(container1)
	manager.RegisterContainer(container2)

	reqBody := map[string]interface{}{
		"depends_on": []string{"db"},
	}
	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest("POST", "/api/container-health-pro/containers/web/dependency", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("设置依赖关系失败，状态码: %d", w.Code)
	}

	// 验证依赖关系
	graph := manager.GetDependencyGraph()
	webDep, exists := graph["web"]
	if !exists {
		t.Fatal("web 容器的依赖关系不存在")
	}
	if len(webDep.DependsOn) != 1 || webDep.DependsOn[0] != "db" {
		t.Fatalf("web 容器应依赖 db，实际 %v", webDep.DependsOn)
	}
}

func TestUpdateResourceUsage(t *testing.T) {
	router, handlers := setupTestRouter()

	// 先注册一个容器
	manager := handlers.manager
	container := &ContainerHealthPro{
		ContainerID: "test-container-1",
		Name:        "web-server",
		Image:       "nginx:latest",
		HealthCheck: HealthCheckConfig{Type: HealthCheckTCP, Port: 8080},
		ResourceLimits: ResourceLimits{
			CPUPercent:    80.0,
			MemoryPercent: 90.0,
		},
	}
	manager.RegisterContainer(container)

	reqBody := ResourceUsage{
		CPUPercent:    95.0,
		MemoryUsed:    1024 * 1024 * 512,  // 512MB
		MemoryTotal:   1024 * 1024 * 1024, // 1GB
		MemoryPercent: 50.0,
	}
	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest("PUT", "/api/container-health-pro/containers/test-container-1/resource-usage", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("更新资源使用失败，状态码: %d", w.Code)
	}

	// 验证资源使用已更新
	c, _ := manager.GetContainer("test-container-1")
	if c.ResourceUsage.CPUPercent != 95.0 {
		t.Fatalf("CPU使用率应为 95.0，实际 %f", c.ResourceUsage.CPUPercent)
	}
	// CPU超过阈值80%，应该有偏差告警
	if len(c.Deviations) == 0 {
		t.Fatal("CPU超过阈值应有性能偏差")
	}
}

func TestGetHealthReport(t *testing.T) {
	router, handlers := setupTestRouter()

	// 先注册多个容器
	manager := handlers.manager
	container1 := &ContainerHealthPro{
		ContainerID: "c1",
		Name:        "web",
		Image:       "nginx:latest",
		HealthCheck: HealthCheckConfig{Type: HealthCheckTCP, Port: 8080},
	}
	container2 := &ContainerHealthPro{
		ContainerID: "c2",
		Name:        "db",
		Image:       "postgres:14",
		HealthCheck: HealthCheckConfig{Type: HealthCheckTCP, Port: 5432},
	}
	manager.RegisterContainer(container1)
	manager.RegisterContainer(container2)

	req := httptest.NewRequest("GET", "/api/container-health-pro/report", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("获取报告失败，状态码: %d", w.Code)
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	data := resp["data"].(map[string]interface{})
	if data["total_containers"].(float64) != 2 {
		t.Fatalf("期望总容器数2，实际 %v", data["total_containers"])
	}
}

func TestAddLogPattern(t *testing.T) {
	router, _ := setupTestRouter()

	pattern := LogPattern{
		Pattern:     "error",
		Severity:    AlertWarning,
		Description: "错误日志模式",
	}

	body, _ := json.Marshal(pattern)
	req := httptest.NewRequest("POST", "/api/container-health-pro/log-patterns", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("添加日志模式失败，状态码: %d", w.Code)
	}
}
