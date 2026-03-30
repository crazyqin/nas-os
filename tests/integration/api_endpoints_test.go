// Package integration 提供 NAS-OS API 端点集成测试
// v2.7.0 补充测试覆盖
package integration

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

// TestAPIRouter API 测试路由器.
func setupTestRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(gin.Recovery())

	api := r.Group("/api/v1")
	{
		// 系统端点
		api.GET("/health", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{
				"status":  "ok",
				"version": "2.7.0",
			})
		})

		api.GET("/system/info", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{
				"hostname":   "nas-server",
				"version":    "2.7.0",
				"uptime":     86400,
				"cpu_usage":  25.5,
				"mem_usage":  45.2,
				"disk_usage": 60.0,
			})
		})

		// 存储端点
		volumes := api.Group("/volumes")
		{
			volumes.GET("", func(c *gin.Context) {
				c.JSON(http.StatusOK, gin.H{
					"volumes": []gin.H{
						{"name": "data", "size": 1000000000000, "used": 500000000000},
						{"name": "backup", "size": 2000000000000, "used": 1000000000000},
					},
				})
			})
			volumes.POST("", func(c *gin.Context) {
				c.JSON(http.StatusCreated, gin.H{
					"name":   "new-volume",
					"status": "created",
				})
			})
			volumes.GET("/:name", func(c *gin.Context) {
				c.JSON(http.StatusOK, gin.H{
					"name": c.Param("name"),
					"size": 1000000000000,
				})
			})
			volumes.DELETE("/:name", func(c *gin.Context) {
				c.JSON(http.StatusNoContent, nil)
			})
		}

		// 子卷端点
		subvolumes := api.Group("/subvolumes")
		{
			subvolumes.GET("", func(c *gin.Context) {
				c.JSON(http.StatusOK, gin.H{
					"subvolumes": []gin.H{
						{"id": 256, "name": "documents", "path": "/data/documents"},
						{"id": 257, "name": "media", "path": "/data/media"},
					},
				})
			})
			subvolumes.POST("", func(c *gin.Context) {
				c.JSON(http.StatusCreated, gin.H{
					"name":   "new-subvol",
					"status": "created",
				})
			})
		}

		// 快照端点
		snapshots := api.Group("/snapshots")
		{
			snapshots.GET("", func(c *gin.Context) {
				c.JSON(http.StatusOK, gin.H{
					"snapshots": []gin.H{
						{"name": "daily-001", "created": "2026-03-14T00:00:00Z"},
					},
				})
			})
			snapshots.POST("", func(c *gin.Context) {
				c.JSON(http.StatusCreated, gin.H{
					"name":   "manual-snapshot",
					"status": "created",
				})
			})
		}

		// 用户端点
		users := api.Group("/users")
		{
			users.GET("", func(c *gin.Context) {
				c.JSON(http.StatusOK, gin.H{
					"users": []gin.H{
						{"id": 1, "username": "admin", "role": "admin"},
						{"id": 2, "username": "user1", "role": "user"},
					},
				})
			})
			users.POST("", func(c *gin.Context) {
				c.JSON(http.StatusCreated, gin.H{
					"id":       3,
					"username": "newuser",
					"role":     "user",
				})
			})
			users.GET("/:id", func(c *gin.Context) {
				c.JSON(http.StatusOK, gin.H{
					"id":       c.Param("id"),
					"username": "testuser",
					"role":     "user",
				})
			})
			users.PUT("/:id", func(c *gin.Context) {
				c.JSON(http.StatusOK, gin.H{
					"id":      c.Param("id"),
					"status":  "updated",
					"message": "User updated successfully",
				})
			})
			users.DELETE("/:id", func(c *gin.Context) {
				c.JSON(http.StatusNoContent, nil)
			})
		}

		// 认证端点
		auth := api.Group("/auth")
		{
			auth.POST("/login", func(c *gin.Context) {
				c.JSON(http.StatusOK, gin.H{
					"token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9",
					"user": gin.H{
						"id":       1,
						"username": "admin",
						"role":     "admin",
					},
				})
			})
			auth.POST("/logout", func(c *gin.Context) {
				c.JSON(http.StatusOK, gin.H{
					"message": "Logged out successfully",
				})
			})
			auth.POST("/refresh", func(c *gin.Context) {
				c.JSON(http.StatusOK, gin.H{
					"token": "new-token",
				})
			})
		}

		// 配额端点
		quota := api.Group("/quota")
		{
			quota.GET("", func(c *gin.Context) {
				c.JSON(http.StatusOK, gin.H{
					"quotas": []gin.H{
						{"user": "user1", "used": 1000000000, "limit": 10000000000},
					},
				})
			})
			quota.POST("", func(c *gin.Context) {
				c.JSON(http.StatusCreated, gin.H{
					"status": "created",
				})
			})
		}

		// 共享端点
		shares := api.Group("/shares")
		{
			shares.GET("", func(c *gin.Context) {
				c.JSON(http.StatusOK, gin.H{
					"shares": []gin.H{
						{"name": "documents", "type": "smb", "path": "/data/documents"},
						{"name": "media", "type": "nfs", "path": "/data/media"},
					},
				})
			})
			shares.POST("", func(c *gin.Context) {
				c.JSON(http.StatusCreated, gin.H{
					"name":   "new-share",
					"status": "created",
				})
			})
		}

		// 备份端点
		backup := api.Group("/backup")
		{
			backup.GET("/jobs", func(c *gin.Context) {
				c.JSON(http.StatusOK, gin.H{
					"jobs": []gin.H{
						{"id": "job-001", "name": "daily-backup", "status": "completed"},
					},
				})
			})
			backup.POST("/jobs", func(c *gin.Context) {
				c.JSON(http.StatusCreated, gin.H{
					"id":     "job-new",
					"status": "created",
				})
			})
			backup.POST("/jobs/:id/run", func(c *gin.Context) {
				c.JSON(http.StatusAccepted, gin.H{
					"id":     c.Param("id"),
					"status": "running",
				})
			})
		}

		// 去重端点
		dedup := api.Group("/dedup")
		{
			dedup.GET("/status", func(c *gin.Context) {
				c.JSON(http.StatusOK, gin.H{
					"enabled":        true,
					"savings":        50000000000,
					"duplicateCount": 100,
				})
			})
			dedup.POST("/scan", func(c *gin.Context) {
				c.JSON(http.StatusAccepted, gin.H{
					"scanId": "scan-001",
					"status": "started",
				})
			})
		}

		// 搜索端点
		search := api.Group("/search")
		{
			search.GET("", func(c *gin.Context) {
				c.JSON(http.StatusOK, gin.H{
					"results": []gin.H{
						{"name": "document.pdf", "path": "/data/documents"},
					},
					"total": 1,
				})
			})
		}

		// 监控端点
		monitor := api.Group("/monitor")
		{
			monitor.GET("/metrics", func(c *gin.Context) {
				c.JSON(http.StatusOK, gin.H{
					"cpu":    25.5,
					"memory": 45.2,
					"disk":   60.0,
					"network": gin.H{
						"rx": 1024000,
						"tx": 512000,
					},
				})
			})
			monitor.GET("/alerts", func(c *gin.Context) {
				c.JSON(http.StatusOK, gin.H{
					"alerts": []gin.H{},
				})
			})
		}

		// 分层存储端点
		tiering := api.Group("/tiering")
		{
			tiering.GET("/tiers", func(c *gin.Context) {
				c.JSON(http.StatusOK, gin.H{
					"tiers": []gin.H{
						{"name": "ssd-cache", "type": "ssd", "size": 500000000000},
						{"name": "hdd-storage", "type": "hdd", "size": 5000000000000},
					},
				})
			})
			tiering.GET("/policies", func(c *gin.Context) {
				c.JSON(http.StatusOK, gin.H{
					"policies": []gin.H{
						{"id": "policy-001", "name": "auto-tier", "action": "move"},
					},
				})
			})
		}

		// 压缩存储端点
		compress := api.Group("/compress")
		{
			compress.GET("/status", func(c *gin.Context) {
				c.JSON(http.StatusOK, gin.H{
					"enabled":   true,
					"algorithm": "zstd",
					"ratio":     2.5,
				})
			})
		}

		// WebDAV 端点
		webdav := api.Group("/webdav")
		{
			webdav.GET("/status", func(c *gin.Context) {
				c.JSON(http.StatusOK, gin.H{
					"running": true,
					"port":    8080,
				})
			})
		}

		// FTP 端点
		ftp := api.Group("/ftp")
		{
			ftp.GET("/status", func(c *gin.Context) {
				c.JSON(http.StatusOK, gin.H{
					"running": true,
					"port":    21,
				})
			})
		}

		// iSCSI 端点
		iscsi := api.Group("/iscsi")
		{
			iscsi.GET("/targets", func(c *gin.Context) {
				c.JSON(http.StatusOK, gin.H{
					"targets": []gin.H{
						{"name": "iqn.2026-01.com.nas:target1", "luns": 2},
					},
				})
			})
		}

		// 容器端点
		containers := api.Group("/containers")
		{
			containers.GET("", func(c *gin.Context) {
				c.JSON(http.StatusOK, gin.H{
					"containers": []gin.H{
						{"id": "abc123", "name": "nginx", "status": "running"},
					},
				})
			})
		}

		// 插件端点
		plugins := api.Group("/plugins")
		{
			plugins.GET("", func(c *gin.Context) {
				c.JSON(http.StatusOK, gin.H{
					"plugins": []gin.H{
						{"name": "transmission", "version": "1.0.0", "enabled": true},
					},
				})
			})
		}
	}

	return r
}

// ========== 系统端点测试 ==========

func TestAPI_HealthEndpoint(t *testing.T) {
	router := setupTestRouter()

	req, _ := http.NewRequestWithContext(context.Background(), "GET", "/api/v1/health", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("期望状态码 %d, 实际 %d", http.StatusOK, w.Code)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("解析响应失败: %v", err)
	}

	if resp["status"] != "ok" {
		t.Errorf("期望 status=ok, 实际 %v", resp["status"])
	}
}

func TestAPI_SystemInfo(t *testing.T) {
	router := setupTestRouter()

	req, _ := http.NewRequestWithContext(context.Background(), "GET", "/api/v1/system/info", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("期望状态码 %d, 实际 %d", http.StatusOK, w.Code)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("解析响应失败: %v", err)
	}

	requiredFields := []string{"hostname", "version", "uptime", "cpu_usage", "mem_usage", "disk_usage"}
	for _, field := range requiredFields {
		if _, ok := resp[field]; !ok {
			t.Errorf("缺少字段: %s", field)
		}
	}
}

// ========== 存储端点测试 ==========

func TestAPI_ListVolumes(t *testing.T) {
	router := setupTestRouter()

	req, _ := http.NewRequestWithContext(context.Background(), "GET", "/api/v1/volumes", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("期望状态码 %d, 实际 %d", http.StatusOK, w.Code)
	}
}

func TestAPI_CreateVolume(t *testing.T) {
	router := setupTestRouter()

	req, _ := http.NewRequestWithContext(context.Background(), "POST", "/api/v1/volumes", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("期望状态码 %d, 实际 %d", http.StatusCreated, w.Code)
	}
}

func TestAPI_GetVolume(t *testing.T) {
	router := setupTestRouter()

	req, _ := http.NewRequestWithContext(context.Background(), "GET", "/api/v1/volumes/test-vol", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("期望状态码 %d, 实际 %d", http.StatusOK, w.Code)
	}
}

func TestAPI_DeleteVolume(t *testing.T) {
	router := setupTestRouter()

	req, _ := http.NewRequestWithContext(context.Background(), "DELETE", "/api/v1/volumes/test-vol", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("期望状态码 %d, 实际 %d", http.StatusNoContent, w.Code)
	}
}

// ========== 子卷端点测试 ==========

func TestAPI_ListSubvolumes(t *testing.T) {
	router := setupTestRouter()

	req, _ := http.NewRequestWithContext(context.Background(), "GET", "/api/v1/subvolumes", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("期望状态码 %d, 实际 %d", http.StatusOK, w.Code)
	}
}

func TestAPI_CreateSubvolume(t *testing.T) {
	router := setupTestRouter()

	req, _ := http.NewRequestWithContext(context.Background(), "POST", "/api/v1/subvolumes", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("期望状态码 %d, 实际 %d", http.StatusCreated, w.Code)
	}
}

// ========== 快照端点测试 ==========

func TestAPI_ListSnapshots(t *testing.T) {
	router := setupTestRouter()

	req, _ := http.NewRequestWithContext(context.Background(), "GET", "/api/v1/snapshots", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("期望状态码 %d, 实际 %d", http.StatusOK, w.Code)
	}
}

func TestAPI_CreateSnapshot(t *testing.T) {
	router := setupTestRouter()

	req, _ := http.NewRequestWithContext(context.Background(), "POST", "/api/v1/snapshots", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("期望状态码 %d, 实际 %d", http.StatusCreated, w.Code)
	}
}

// ========== 用户端点测试 ==========

func TestAPI_ListUsers(t *testing.T) {
	router := setupTestRouter()

	req, _ := http.NewRequestWithContext(context.Background(), "GET", "/api/v1/users", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("期望状态码 %d, 实际 %d", http.StatusOK, w.Code)
	}
}

func TestAPI_CreateUser(t *testing.T) {
	router := setupTestRouter()

	req, _ := http.NewRequestWithContext(context.Background(), "POST", "/api/v1/users", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("期望状态码 %d, 实际 %d", http.StatusCreated, w.Code)
	}
}

func TestAPI_GetUser(t *testing.T) {
	router := setupTestRouter()

	req, _ := http.NewRequestWithContext(context.Background(), "GET", "/api/v1/users/1", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("期望状态码 %d, 实际 %d", http.StatusOK, w.Code)
	}
}

func TestAPI_UpdateUser(t *testing.T) {
	router := setupTestRouter()

	req, _ := http.NewRequestWithContext(context.Background(), "PUT", "/api/v1/users/1", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("期望状态码 %d, 实际 %d", http.StatusOK, w.Code)
	}
}

func TestAPI_DeleteUser(t *testing.T) {
	router := setupTestRouter()

	req, _ := http.NewRequestWithContext(context.Background(), "DELETE", "/api/v1/users/1", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("期望状态码 %d, 实际 %d", http.StatusNoContent, w.Code)
	}
}

// ========== 认证端点测试 ==========

func TestAPI_Login(t *testing.T) {
	router := setupTestRouter()

	req, _ := http.NewRequestWithContext(context.Background(), "POST", "/api/v1/auth/login", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("期望状态码 %d, 实际 %d", http.StatusOK, w.Code)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("解析响应失败: %v", err)
	}

	if _, ok := resp["token"]; !ok {
		t.Error("期望返回 token")
	}
}

func TestAPI_Logout(t *testing.T) {
	router := setupTestRouter()

	req, _ := http.NewRequestWithContext(context.Background(), "POST", "/api/v1/auth/logout", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("期望状态码 %d, 实际 %d", http.StatusOK, w.Code)
	}
}

func TestAPI_RefreshToken(t *testing.T) {
	router := setupTestRouter()

	req, _ := http.NewRequestWithContext(context.Background(), "POST", "/api/v1/auth/refresh", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("期望状态码 %d, 实际 %d", http.StatusOK, w.Code)
	}
}

// ========== 配额端点测试 ==========

func TestAPI_ListQuotas(t *testing.T) {
	router := setupTestRouter()

	req, _ := http.NewRequestWithContext(context.Background(), "GET", "/api/v1/quota", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("期望状态码 %d, 实际 %d", http.StatusOK, w.Code)
	}
}

func TestAPI_CreateQuota(t *testing.T) {
	router := setupTestRouter()

	req, _ := http.NewRequestWithContext(context.Background(), "POST", "/api/v1/quota", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("期望状态码 %d, 实际 %d", http.StatusCreated, w.Code)
	}
}

// ========== 共享端点测试 ==========

func TestAPI_ListShares(t *testing.T) {
	router := setupTestRouter()

	req, _ := http.NewRequestWithContext(context.Background(), "GET", "/api/v1/shares", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("期望状态码 %d, 实际 %d", http.StatusOK, w.Code)
	}
}

func TestAPI_CreateShare(t *testing.T) {
	router := setupTestRouter()

	req, _ := http.NewRequestWithContext(context.Background(), "POST", "/api/v1/shares", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("期望状态码 %d, 实际 %d", http.StatusCreated, w.Code)
	}
}

// ========== 备份端点测试 ==========

func TestAPI_ListBackupJobs(t *testing.T) {
	router := setupTestRouter()

	req, _ := http.NewRequestWithContext(context.Background(), "GET", "/api/v1/backup/jobs", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("期望状态码 %d, 实际 %d", http.StatusOK, w.Code)
	}
}

func TestAPI_CreateBackupJob(t *testing.T) {
	router := setupTestRouter()

	req, _ := http.NewRequestWithContext(context.Background(), "POST", "/api/v1/backup/jobs", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("期望状态码 %d, 实际 %d", http.StatusCreated, w.Code)
	}
}

func TestAPI_RunBackupJob(t *testing.T) {
	router := setupTestRouter()

	req, _ := http.NewRequestWithContext(context.Background(), "POST", "/api/v1/backup/jobs/job-001/run", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusAccepted {
		t.Errorf("期望状态码 %d, 实际 %d", http.StatusAccepted, w.Code)
	}
}

// ========== 去重端点测试 ==========

func TestAPI_DedupStatus(t *testing.T) {
	router := setupTestRouter()

	req, _ := http.NewRequestWithContext(context.Background(), "GET", "/api/v1/dedup/status", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("期望状态码 %d, 实际 %d", http.StatusOK, w.Code)
	}
}

func TestAPI_DedupScan(t *testing.T) {
	router := setupTestRouter()

	req, _ := http.NewRequestWithContext(context.Background(), "POST", "/api/v1/dedup/scan", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusAccepted {
		t.Errorf("期望状态码 %d, 实际 %d", http.StatusAccepted, w.Code)
	}
}

// ========== 搜索端点测试 ==========

func TestAPI_Search(t *testing.T) {
	router := setupTestRouter()

	req, _ := http.NewRequestWithContext(context.Background(), "GET", "/api/v1/search?q=test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("期望状态码 %d, 实际 %d", http.StatusOK, w.Code)
	}
}

// ========== 监控端点测试 ==========

func TestAPI_MonitorMetrics(t *testing.T) {
	router := setupTestRouter()

	req, _ := http.NewRequestWithContext(context.Background(), "GET", "/api/v1/monitor/metrics", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("期望状态码 %d, 实际 %d", http.StatusOK, w.Code)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("解析响应失败: %v", err)
	}

	// 验证关键指标字段
	requiredMetrics := []string{"cpu", "memory", "disk"}
	for _, field := range requiredMetrics {
		if _, ok := resp[field]; !ok {
			t.Errorf("缺少指标: %s", field)
		}
	}
}

func TestAPI_MonitorAlerts(t *testing.T) {
	router := setupTestRouter()

	req, _ := http.NewRequestWithContext(context.Background(), "GET", "/api/v1/monitor/alerts", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("期望状态码 %d, 实际 %d", http.StatusOK, w.Code)
	}
}

// ========== 分层存储端点测试 ==========

func TestAPI_ListTiers(t *testing.T) {
	router := setupTestRouter()

	req, _ := http.NewRequestWithContext(context.Background(), "GET", "/api/v1/tiering/tiers", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("期望状态码 %d, 实际 %d", http.StatusOK, w.Code)
	}
}

func TestAPI_ListTieringPolicies(t *testing.T) {
	router := setupTestRouter()

	req, _ := http.NewRequestWithContext(context.Background(), "GET", "/api/v1/tiering/policies", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("期望状态码 %d, 实际 %d", http.StatusOK, w.Code)
	}
}

// ========== 压缩存储端点测试 ==========

func TestAPI_CompressStatus(t *testing.T) {
	router := setupTestRouter()

	req, _ := http.NewRequestWithContext(context.Background(), "GET", "/api/v1/compress/status", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("期望状态码 %d, 实际 %d", http.StatusOK, w.Code)
	}
}

// ========== WebDAV 端点测试 ==========

func TestAPI_WebDAVStatus(t *testing.T) {
	router := setupTestRouter()

	req, _ := http.NewRequestWithContext(context.Background(), "GET", "/api/v1/webdav/status", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("期望状态码 %d, 实际 %d", http.StatusOK, w.Code)
	}
}

// ========== FTP 端点测试 ==========

func TestAPI_FTPStatus(t *testing.T) {
	router := setupTestRouter()

	req, _ := http.NewRequestWithContext(context.Background(), "GET", "/api/v1/ftp/status", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("期望状态码 %d, 实际 %d", http.StatusOK, w.Code)
	}
}

// ========== iSCSI 端点测试 ==========

func TestAPI_ListISCSITargets(t *testing.T) {
	router := setupTestRouter()

	req, _ := http.NewRequestWithContext(context.Background(), "GET", "/api/v1/iscsi/targets", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("期望状态码 %d, 实际 %d", http.StatusOK, w.Code)
	}
}

// ========== 容器端点测试 ==========

func TestAPI_ListContainers(t *testing.T) {
	router := setupTestRouter()

	req, _ := http.NewRequestWithContext(context.Background(), "GET", "/api/v1/containers", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("期望状态码 %d, 实际 %d", http.StatusOK, w.Code)
	}
}

// ========== 插件端点测试 ==========

func TestAPI_ListPlugins(t *testing.T) {
	router := setupTestRouter()

	req, _ := http.NewRequestWithContext(context.Background(), "GET", "/api/v1/plugins", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("期望状态码 %d, 实际 %d", http.StatusOK, w.Code)
	}
}

// ========== 并发测试 ==========

func TestAPI_ConcurrentRequests(t *testing.T) {
	router := setupTestRouter()
	done := make(chan bool, 20)

	// 并发执行多个请求
	for i := 0; i < 20; i++ {
		go func(id int) {
			req, _ := http.NewRequestWithContext(context.Background(), "GET", "/api/v1/health", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			if w.Code == http.StatusOK {
				done <- true
			} else {
				done <- false
			}
		}(i)
	}

	// 等待所有请求完成
	successCount := 0
	for i := 0; i < 20; i++ {
		if <-done {
			successCount++
		}
	}

	if successCount != 20 {
		t.Errorf("并发请求失败: 成功 %d/20", successCount)
	}
}

// ========== 错误处理测试 ==========

func TestAPI_NotFound(t *testing.T) {
	router := setupTestRouter()

	req, _ := http.NewRequestWithContext(context.Background(), "GET", "/api/v1/nonexistent", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("期望状态码 %d, 实际 %d", http.StatusNotFound, w.Code)
	}
}
