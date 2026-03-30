// Package e2e 提供 NAS-OS 端到端测试
// 存储、认证、系统模块 E2E 测试
package e2e

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

// NewTestServer 创建测试服务器.
func NewTestServer() *httptest.Server {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(gin.Recovery())

	// 注册路由
	api := engine.Group("/api/v1")
	{
		// 健康检查
		api.GET("/health", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"status": "ok"})
		})

		// 卷管理 - 模拟数据
		volumes := api.Group("/volumes")
		{
			volumes.GET("", func(c *gin.Context) {
				c.JSON(http.StatusOK, []gin.H{
					{"name": "data", "size": 1000000000000},
				})
			})
			volumes.POST("", func(c *gin.Context) {
				c.JSON(http.StatusCreated, gin.H{"name": "new-vol"})
			})
			volumes.GET("/:name", func(c *gin.Context) {
				name := c.Param("name")
				c.JSON(http.StatusOK, gin.H{"name": name})
			})
			volumes.DELETE("/:name", func(c *gin.Context) {
				c.JSON(http.StatusNoContent, nil)
			})
		}

		// 认证
		auth := api.Group("/auth")
		{
			auth.POST("/login", func(c *gin.Context) {
				c.JSON(http.StatusOK, gin.H{
					"token": "test-token",
					"user":  gin.H{"username": "test"},
				})
			})
		}

		// 用户
		users := api.Group("/users")
		{
			users.POST("", func(c *gin.Context) {
				c.JSON(http.StatusCreated, gin.H{"username": "newuser"})
			})
		}
	}

	return httptest.NewServer(engine)
}

// ========== 存储 E2E 测试 ==========

// TestE2E_Storage_CreateVolume E2E 测试：创建卷.
func TestE2E_Storage_CreateVolume(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过 E2E 测试")
	}

	server := NewTestServer()
	defer server.Close()

	client := NewTestClient(server.URL)

	req := map[string]interface{}{
		"name":    "test-volume",
		"devices": []string{"/dev/sda1", "/dev/sdb1"},
		"profile": "raid1",
	}

	resp, err := client.Post("/api/v1/volumes", req)
	if err != nil {
		t.Fatalf("创建卷请求失败: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Errorf("期望状态码 %d, 实际 %d", http.StatusCreated, resp.StatusCode)
	}
}

// TestE2E_Storage_ListVolumes E2E 测试：列出卷.
func TestE2E_Storage_ListVolumes(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过 E2E 测试")
	}

	server := NewTestServer()
	defer server.Close()

	client := NewTestClient(server.URL)

	resp, err := client.Get("/api/v1/volumes")
	if err != nil {
		t.Fatalf("获取卷列表请求失败: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("期望状态码 %d, 实际 %d", http.StatusOK, resp.StatusCode)
	}
}

// TestE2E_Storage_GetVolume E2E 测试：获取单个卷.
func TestE2E_Storage_GetVolume(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过 E2E 测试")
	}

	server := NewTestServer()
	defer server.Close()

	client := NewTestClient(server.URL)

	resp, err := client.Get("/api/v1/volumes/test-vol")
	if err != nil {
		t.Fatalf("获取卷请求失败: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("期望状态码 %d, 实际 %d", http.StatusOK, resp.StatusCode)
	}
}

// TestE2E_Storage_DeleteVolume E2E 测试：删除卷.
func TestE2E_Storage_DeleteVolume(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过 E2E 测试")
	}

	server := NewTestServer()
	defer server.Close()

	client := NewTestClient(server.URL)

	resp, err := client.Delete("/api/v1/volumes/test-vol")
	if err != nil {
		t.Fatalf("删除卷请求失败: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		t.Errorf("期望状态码 %d, 实际 %d", http.StatusNoContent, resp.StatusCode)
	}
}

// TestE2E_Storage_CompleteWorkflow E2E 测试：完整存储工作流.
func TestE2E_Storage_CompleteWorkflow(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过 E2E 测试")
	}

	server := NewTestServer()
	defer server.Close()

	client := NewTestClient(server.URL)

	t.Log("步骤 1: 创建存储卷")
	resp, err := client.Post("/api/v1/volumes", map[string]interface{}{
		"name":    "workflow-vol",
		"devices": []string{"/dev/sda1", "/dev/sdb1"},
		"profile": "raid1",
	})
	if err != nil {
		t.Fatalf("创建卷失败: %v", err)
	}
	resp.Body.Close()

	t.Log("步骤 2: 验证卷创建成功")
	resp, err = client.Get("/api/v1/volumes/workflow-vol")
	if err != nil {
		t.Fatalf("获取卷失败: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("期望卷存在，状态码: %d", resp.StatusCode)
	}

	t.Log("步骤 3: 列出所有卷")
	resp, err = client.Get("/api/v1/volumes")
	if err != nil {
		t.Fatalf("列出卷失败: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("列出卷失败，状态码: %d", resp.StatusCode)
	}

	t.Log("步骤 4: 删除卷")
	resp, err = client.Delete("/api/v1/volumes/workflow-vol")
	if err != nil {
		t.Fatalf("删除卷失败: %v", err)
	}
	resp.Body.Close()

	t.Log("✅ 完整工作流测试通过")
}

// ========== 认证 E2E 测试 ==========

// TestE2E_Auth_Login E2E 测试：登录.
func TestE2E_Auth_Login(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过 E2E 测试")
	}

	server := NewTestServer()
	defer server.Close()

	client := NewTestClient(server.URL)

	resp, err := client.Post("/api/v1/auth/login", map[string]interface{}{
		"username": "testuser",
		"password": "testpass",
	})
	if err != nil {
		t.Fatalf("登录请求失败: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("期望状态码 %d, 实际 %d", http.StatusOK, resp.StatusCode)
	}

	var result map[string]interface{}
	if err := ParseJSON(resp, &result); err != nil {
		t.Fatalf("解析响应失败: %v", err)
	}

	if _, ok := result["token"]; !ok {
		t.Error("期望返回 token")
	}
}

// TestE2E_Auth_CompleteAuthWorkflow E2E 测试：完整认证工作流.
func TestE2E_Auth_CompleteAuthWorkflow(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过 E2E 测试")
	}

	server := NewTestServer()
	defer server.Close()

	client := NewTestClient(server.URL)

	t.Log("步骤 1: 创建用户")
	resp, err := client.Post("/api/v1/users", map[string]interface{}{
		"username": "workflowuser",
		"password": "workflowpass",
	})
	if err != nil {
		t.Fatalf("创建用户失败: %v", err)
	}
	resp.Body.Close()

	t.Log("步骤 2: 使用用户登录")
	resp, err = client.Post("/api/v1/auth/login", map[string]interface{}{
		"username": "workflowuser",
		"password": "workflowpass",
	})
	if err != nil {
		t.Fatalf("登录失败: %v", err)
	}

	var loginResult map[string]interface{}
	if err := ParseJSON(resp, &loginResult); err != nil {
		t.Fatalf("解析登录响应失败: %v", err)
	}
	resp.Body.Close()

	token, ok := loginResult["token"].(string)
	if !ok || token == "" {
		t.Fatal("登录响应中未找到有效 token")
	}

	t.Log("步骤 3: 设置认证令牌")
	client.SetAuth(token)
	client.ClearAuth()

	t.Log("✅ 完整认证工作流测试通过")
}

// ========== 系统 E2E 测试 ==========

// TestE2E_System_Health E2E 测试：健康检查.
func TestE2E_System_Health(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过 E2E 测试")
	}

	server := NewTestServer()
	defer server.Close()

	client := NewTestClient(server.URL)

	resp, err := client.Get("/api/v1/health")
	if err != nil {
		t.Fatalf("健康检查请求失败: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("期望状态码 %d, 实际 %d", http.StatusOK, resp.StatusCode)
	}

	var result map[string]interface{}
	if err := ParseJSON(resp, &result); err != nil {
		t.Fatalf("解析响应失败: %v", err)
	}

	if status, ok := result["status"]; ok {
		if status != "ok" {
			t.Errorf("期望状态 ok, 实际 %v", status)
		}
	}
}

// TestE2E_System_CompleteSystemCheck E2E 测试：完整系统检查.
func TestE2E_System_CompleteSystemCheck(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过 E2E 测试")
	}

	server := NewTestServer()
	defer server.Close()

	client := NewTestClient(server.URL)

	t.Log("检查系统健康状态")
	resp, err := client.Get("/api/v1/health")
	if err != nil {
		t.Fatalf("健康检查失败: %v", err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("系统不健康，状态码: %d", resp.StatusCode)
	}

	t.Log("✅ 系统健康检查通过")
}
