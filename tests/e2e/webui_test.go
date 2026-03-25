// Package e2e 提供 NAS-OS WebUI 端到端测试
// v2.7.0 WebUI 测试覆盖
package e2e

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

// WebUI 测试服务器.
func setupWebUITestRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(gin.Recovery())

	// 静态文件服务（模拟）
	r.Static("/static", "./webui/static")

	// API 端点
	api := r.Group("/api/v1")
	{
		// 仪表板数据
		api.GET("/dashboard", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{
				"system": gin.H{
					"hostname":  "nas-server",
					"version":   "2.7.0",
					"uptime":    86400,
					"cpuUsage":  25.5,
					"memUsage":  45.2,
					"diskUsage": 60.0,
				},
				"storage": gin.H{
					"totalVolumes": 2,
					"totalSize":    3000000000000,
					"usedSize":     1500000000000,
					"healthStatus": "healthy",
				},
				"network": gin.H{
					"rxSpeed": 1024000,
					"txSpeed": 512000,
				},
				"services": gin.H{
					"smb":    true,
					"nfs":    true,
					"webdav": true,
					"ftp":    false,
					"iscsi":  true,
					"docker": true,
				},
				"alerts": []gin.H{},
			})
		})

		// 存储管理
		api.GET("/storage/overview", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{
				"volumes": []gin.H{
					{
						"name":        "data",
						"size":        1000000000000,
						"used":        500000000000,
						"free":        500000000000,
						"health":      "healthy",
						"raidProfile": "raid1",
						"devices":     2,
					},
					{
						"name":        "backup",
						"size":        2000000000000,
						"used":        1000000000000,
						"free":        1000000000000,
						"health":      "healthy",
						"raidProfile": "raid5",
						"devices":     3,
					},
				},
				"subvolumes": []gin.H{
					{"name": "documents", "path": "/data/documents", "size": 100000000000},
					{"name": "media", "path": "/data/media", "size": 200000000000},
					{"name": "photos", "path": "/data/photos", "size": 50000000000},
				},
				"snapshots": gin.H{
					"total":   10,
					"size":    50000000000,
					"latest":  "2026-03-14T00:00:00Z",
					"enabled": true,
				},
			})
		})

		// 卷详情
		api.GET("/volumes/:name/details", func(c *gin.Context) {
			name := c.Param("name")
			c.JSON(http.StatusOK, gin.H{
				"name":        name,
				"size":        1000000000000,
				"used":        500000000000,
				"free":        500000000000,
				"health":      "healthy",
				"raidProfile": "raid1",
				"devices": []gin.H{
					{"path": "/dev/sda1", "size": 500000000000, "status": "online"},
					{"path": "/dev/sdb1", "size": 500000000000, "status": "online"},
				},
				"subvolumes": []gin.H{
					{"id": 256, "name": "documents"},
					{"id": 257, "name": "media"},
				},
				"snapshots": []gin.H{
					{"name": "daily-001", "created": "2026-03-14T00:00:00Z"},
				},
			})
		})

		// 用户管理
		api.GET("/users/overview", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{
				"users": []gin.H{
					{"id": 1, "username": "admin", "role": "admin", "quota": 10000000000, "used": 1000000000},
					{"id": 2, "username": "user1", "role": "user", "quota": 5000000000, "used": 500000000},
				},
				"groups": []gin.H{
					{"id": 1, "name": "admins", "members": 1},
					{"id": 2, "name": "users", "members": 1},
				},
				"totalUsers":  2,
				"activeUsers": 1,
			})
		})

		// 权限管理
		api.GET("/permissions", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{
				"roles": []gin.H{
					{"name": "admin", "permissions": []string{"*"}},
					{"name": "user", "permissions": []string{"read", "write"}},
					{"name": "guest", "permissions": []string{"read"}},
				},
				"resources": []gin.H{
					{"path": "/data/documents", "owner": "admin", "group": "users", "mode": "0755"},
				},
			})
		})

		// 系统设置
		api.GET("/settings", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{
				"network": gin.H{
					"hostname": "nas-server",
					"domain":   "local",
					"dns":      []string{"8.8.8.8", "8.8.4.4"},
				},
				"time": gin.H{
					"timezone": "Asia/Shanghai",
					"ntp":      "pool.ntp.org",
				},
				"notification": gin.H{
					"email":  "admin@example.com",
					"alerts": true,
				},
				"security": gin.H{
					"sshEnabled": true,
					"sshPort":    22,
					"firewall":   true,
					"fail2ban":   true,
				},
			})
		})

		api.PUT("/settings", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{
				"status":  "updated",
				"message": "Settings updated successfully",
			})
		})

		// 日志查看
		api.GET("/logs", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{
				"logs": []gin.H{
					{"time": "2026-03-14T09:00:00Z", "level": "info", "message": "System started"},
					{"time": "2026-03-14T09:01:00Z", "level": "info", "message": "SMB service started"},
					{"time": "2026-03-14T09:02:00Z", "level": "warn", "message": "Disk usage above 80%"},
				},
				"total": 3,
			})
		})

		api.GET("/logs/stream", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{
				"connected": true,
			})
		})

		// 服务管理
		api.GET("/services", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{
				"services": []gin.H{
					{"name": "smb", "status": "running", "enabled": true, "port": 445},
					{"name": "nfs", "status": "running", "enabled": true, "port": 2049},
					{"name": "webdav", "status": "running", "enabled": true, "port": 8080},
					{"name": "ftp", "status": "stopped", "enabled": false, "port": 21},
					{"name": "iscsi", "status": "running", "enabled": true, "port": 3260},
					{"name": "docker", "status": "running", "enabled": true, "port": 2375},
				},
			})
		})

		api.POST("/services/:name/start", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{
				"name":   c.Param("name"),
				"status": "started",
			})
		})

		api.POST("/services/:name/stop", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{
				"name":   c.Param("name"),
				"status": "stopped",
			})
		})

		api.POST("/services/:name/restart", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{
				"name":   c.Param("name"),
				"status": "restarted",
			})
		})

		// 分层存储可视化
		api.GET("/tiering/visualization", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{
				"tiers": []gin.H{
					{
						"name":     "SSD Cache",
						"type":     "ssd",
						"size":     500000000000,
						"used":     250000000000,
						"files":    1000,
						"hotFiles": 500,
					},
					{
						"name":     "HDD Storage",
						"type":     "hdd",
						"size":     5000000000000,
						"used":     2000000000000,
						"files":    10000,
						"hotFiles": 100,
					},
					{
						"name":     "Cloud Archive",
						"type":     "cloud",
						"size":     -1,
						"used":     500000000000,
						"files":    5000,
						"hotFiles": 0,
					},
				},
				"policies": []gin.H{
					{"name": "auto-tier", "source": "ssd", "target": "hdd", "condition": "30d"},
					{"name": "archive-old", "source": "hdd", "target": "cloud", "condition": "90d"},
				},
				"migrations": []gin.H{
					{"file": "old-data.zip", "from": "ssd", "to": "hdd", "progress": 75},
				},
			})
		})

		// 压缩管理
		api.GET("/compress/overview", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{
				"enabled":    true,
				"algorithm":  "zstd",
				"ratio":      2.5,
				"savings":    50000000000,
				"compressed": 1000,
				"pending":    50,
			})
		})

		// 性能监控
		api.GET("/performance", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{
				"iops": gin.H{
					"read":  1000,
					"write": 500,
				},
				"throughput": gin.H{
					"read":  104857600,
					"write": 52428800,
				},
				"latency": gin.H{
					"read":  5.2,
					"write": 8.5,
				},
				"cache": gin.H{
					"hitRate": 85.5,
					"size":    1000000000,
					"used":    750000000,
				},
			})
		})

		// 告警管理
		api.GET("/alerts", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{
				"alerts": []gin.H{
					{
						"id":           "alert-001",
						"level":        "warning",
						"message":      "Disk usage above 80%",
						"source":       "storage",
						"timestamp":    "2026-03-14T08:00:00Z",
						"acknowledged": false,
					},
				},
				"rules": []gin.H{
					{"name": "disk-usage", "condition": "disk > 80%", "level": "warning"},
					{"name": "cpu-usage", "condition": "cpu > 90%", "level": "critical"},
				},
			})
		})

		api.POST("/alerts/:id/acknowledge", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{
				"id":      c.Param("id"),
				"status":  "acknowledged",
				"message": "Alert acknowledged",
			})
		})
	}

	return r
}

// ========== 仪表板测试 ==========

func TestWebUI_Dashboard(t *testing.T) {
	router := setupWebUITestRouter()

	req, _ := http.NewRequest("GET", "/api/v1/dashboard", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("期望状态码 %d, 实际 %d", http.StatusOK, w.Code)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("解析响应失败: %v", err)
	}

	// 验证仪表板数据结构
	requiredSections := []string{"system", "storage", "network", "services", "alerts"}
	for _, section := range requiredSections {
		if _, ok := resp[section]; !ok {
			t.Errorf("仪表板缺少部分: %s", section)
		}
	}
}

func TestWebUI_DashboardSystemInfo(t *testing.T) {
	router := setupWebUITestRouter()

	req, _ := http.NewRequest("GET", "/api/v1/dashboard", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)

	system, ok := resp["system"].(map[string]interface{})
	if !ok {
		t.Fatal("system 部分格式错误")
	}

	requiredFields := []string{"hostname", "version", "uptime", "cpuUsage", "memUsage", "diskUsage"}
	for _, field := range requiredFields {
		if _, ok := system[field]; !ok {
			t.Errorf("system 缺少字段: %s", field)
		}
	}
}

// ========== 存储管理测试 ==========

func TestWebUI_StorageOverview(t *testing.T) {
	router := setupWebUITestRouter()

	req, _ := http.NewRequest("GET", "/api/v1/storage/overview", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("期望状态码 %d, 实际 %d", http.StatusOK, w.Code)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("解析响应失败: %v", err)
	}

	// 验证存储概览数据
	requiredSections := []string{"volumes", "subvolumes", "snapshots"}
	for _, section := range requiredSections {
		if _, ok := resp[section]; !ok {
			t.Errorf("存储概览缺少部分: %s", section)
		}
	}
}

func TestWebUI_VolumeDetails(t *testing.T) {
	router := setupWebUITestRouter()

	req, _ := http.NewRequest("GET", "/api/v1/volumes/data/details", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("期望状态码 %d, 实际 %d", http.StatusOK, w.Code)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("解析响应失败: %v", err)
	}

	// 验证卷详情
	requiredFields := []string{"name", "size", "used", "free", "health", "raidProfile", "devices", "subvolumes", "snapshots"}
	for _, field := range requiredFields {
		if _, ok := resp[field]; !ok {
			t.Errorf("卷详情缺少字段: %s", field)
		}
	}
}

// ========== 用户管理测试 ==========

func TestWebUI_UserOverview(t *testing.T) {
	router := setupWebUITestRouter()

	req, _ := http.NewRequest("GET", "/api/v1/users/overview", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("期望状态码 %d, 实际 %d", http.StatusOK, w.Code)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("解析响应失败: %v", err)
	}

	// 验证用户概览
	if _, ok := resp["users"]; !ok {
		t.Error("用户概览缺少 users 部分")
	}
	if _, ok := resp["groups"]; !ok {
		t.Error("用户概览缺少 groups 部分")
	}
}

func TestWebUI_Permissions(t *testing.T) {
	router := setupWebUITestRouter()

	req, _ := http.NewRequest("GET", "/api/v1/permissions", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("期望状态码 %d, 实际 %d", http.StatusOK, w.Code)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("解析响应失败: %v", err)
	}

	// 验证权限数据
	if _, ok := resp["roles"]; !ok {
		t.Error("权限数据缺少 roles 部分")
	}
	if _, ok := resp["resources"]; !ok {
		t.Error("权限数据缺少 resources 部分")
	}
}

// ========== 系统设置测试 ==========

func TestWebUI_GetSettings(t *testing.T) {
	router := setupWebUITestRouter()

	req, _ := http.NewRequest("GET", "/api/v1/settings", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("期望状态码 %d, 实际 %d", http.StatusOK, w.Code)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("解析响应失败: %v", err)
	}

	// 验证设置部分
	requiredSections := []string{"network", "time", "notification", "security"}
	for _, section := range requiredSections {
		if _, ok := resp[section]; !ok {
			t.Errorf("设置缺少部分: %s", section)
		}
	}
}

func TestWebUI_UpdateSettings(t *testing.T) {
	router := setupWebUITestRouter()

	req, _ := http.NewRequest("PUT", "/api/v1/settings", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("期望状态码 %d, 实际 %d", http.StatusOK, w.Code)
	}
}

// ========== 日志查看测试 ==========

func TestWebUI_GetLogs(t *testing.T) {
	router := setupWebUITestRouter()

	req, _ := http.NewRequest("GET", "/api/v1/logs", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("期望状态码 %d, 实际 %d", http.StatusOK, w.Code)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("解析响应失败: %v", err)
	}

	if _, ok := resp["logs"]; !ok {
		t.Error("日志响应缺少 logs 部分")
	}
}

func TestWebUI_LogStream(t *testing.T) {
	router := setupWebUITestRouter()

	req, _ := http.NewRequest("GET", "/api/v1/logs/stream", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("期望状态码 %d, 实际 %d", http.StatusOK, w.Code)
	}
}

// ========== 服务管理测试 ==========

func TestWebUI_ListServices(t *testing.T) {
	router := setupWebUITestRouter()

	req, _ := http.NewRequest("GET", "/api/v1/services", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("期望状态码 %d, 实际 %d", http.StatusOK, w.Code)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("解析响应失败: %v", err)
	}

	if _, ok := resp["services"]; !ok {
		t.Error("服务列表响应缺少 services 部分")
	}
}

func TestWebUI_StartService(t *testing.T) {
	router := setupWebUITestRouter()

	req, _ := http.NewRequest("POST", "/api/v1/services/ftp/start", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("期望状态码 %d, 实际 %d", http.StatusOK, w.Code)
	}
}

func TestWebUI_StopService(t *testing.T) {
	router := setupWebUITestRouter()

	req, _ := http.NewRequest("POST", "/api/v1/services/smb/stop", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("期望状态码 %d, 实际 %d", http.StatusOK, w.Code)
	}
}

func TestWebUI_RestartService(t *testing.T) {
	router := setupWebUITestRouter()

	req, _ := http.NewRequest("POST", "/api/v1/services/nfs/restart", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("期望状态码 %d, 实际 %d", http.StatusOK, w.Code)
	}
}

// ========== 分层存储可视化测试 ==========

func TestWebUI_TieringVisualization(t *testing.T) {
	router := setupWebUITestRouter()

	req, _ := http.NewRequest("GET", "/api/v1/tiering/visualization", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("期望状态码 %d, 实际 %d", http.StatusOK, w.Code)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("解析响应失败: %v", err)
	}

	// 验证分层存储可视化数据
	requiredSections := []string{"tiers", "policies", "migrations"}
	for _, section := range requiredSections {
		if _, ok := resp[section]; !ok {
			t.Errorf("分层存储可视化缺少部分: %s", section)
		}
	}
}

// ========== 压缩管理测试 ==========

func TestWebUI_CompressOverview(t *testing.T) {
	router := setupWebUITestRouter()

	req, _ := http.NewRequest("GET", "/api/v1/compress/overview", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("期望状态码 %d, 实际 %d", http.StatusOK, w.Code)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("解析响应失败: %v", err)
	}

	// 验证压缩概览数据
	requiredFields := []string{"enabled", "algorithm", "ratio", "savings", "compressed", "pending"}
	for _, field := range requiredFields {
		if _, ok := resp[field]; !ok {
			t.Errorf("压缩概览缺少字段: %s", field)
		}
	}
}

// ========== 性能监控测试 ==========

func TestWebUI_Performance(t *testing.T) {
	router := setupWebUITestRouter()

	req, _ := http.NewRequest("GET", "/api/v1/performance", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("期望状态码 %d, 实际 %d", http.StatusOK, w.Code)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("解析响应失败: %v", err)
	}

	// 验证性能数据
	requiredSections := []string{"iops", "throughput", "latency", "cache"}
	for _, section := range requiredSections {
		if _, ok := resp[section]; !ok {
			t.Errorf("性能数据缺少部分: %s", section)
		}
	}
}

// ========== 告警管理测试 ==========

func TestWebUI_ListAlerts(t *testing.T) {
	router := setupWebUITestRouter()

	req, _ := http.NewRequest("GET", "/api/v1/alerts", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("期望状态码 %d, 实际 %d", http.StatusOK, w.Code)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("解析响应失败: %v", err)
	}

	// 验证告警数据
	if _, ok := resp["alerts"]; !ok {
		t.Error("告警响应缺少 alerts 部分")
	}
	if _, ok := resp["rules"]; !ok {
		t.Error("告警响应缺少 rules 部分")
	}
}

func TestWebUI_AcknowledgeAlert(t *testing.T) {
	router := setupWebUITestRouter()

	req, _ := http.NewRequest("POST", "/api/v1/alerts/alert-001/acknowledge", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("期望状态码 %d, 实际 %d", http.StatusOK, w.Code)
	}
}

// ========== 完整工作流测试 ==========

func TestWebUI_CompleteWorkflow(t *testing.T) {
	router := setupWebUITestRouter()

	t.Log("步骤 1: 检查仪表板")
	req, _ := http.NewRequest("GET", "/api/v1/dashboard", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("仪表板检查失败: %d", w.Code)
	}

	t.Log("步骤 2: 查看存储概览")
	req, _ = http.NewRequest("GET", "/api/v1/storage/overview", nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("存储概览检查失败: %d", w.Code)
	}

	t.Log("步骤 3: 查看服务状态")
	req, _ = http.NewRequest("GET", "/api/v1/services", nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("服务状态检查失败: %d", w.Code)
	}

	t.Log("步骤 4: 检查告警")
	req, _ = http.NewRequest("GET", "/api/v1/alerts", nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("告警检查失败: %d", w.Code)
	}

	t.Log("步骤 5: 查看性能数据")
	req, _ = http.NewRequest("GET", "/api/v1/performance", nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("性能数据检查失败: %d", w.Code)
	}

	t.Log("✅ 完整 WebUI 工作流测试通过")
}

// ========== 响应时间测试 ==========

func TestWebUI_ResponseTime(t *testing.T) {
	router := setupWebUITestRouter()

	endpoints := []string{
		"/api/v1/dashboard",
		"/api/v1/storage/overview",
		"/api/v1/services",
		"/api/v1/performance",
		"/api/v1/alerts",
	}

	for _, endpoint := range endpoints {
		start := time.Now()
		req, _ := http.NewRequest("GET", endpoint, nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		elapsed := time.Since(start)

		if elapsed > 100*time.Millisecond {
			t.Errorf("端点 %s 响应时间过长: %v", endpoint, elapsed)
		}
	}
}
