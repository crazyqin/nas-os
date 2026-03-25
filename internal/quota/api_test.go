// Package quota 提供存储配额管理 API 测试
package quota

import (
	"context"
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

// setupTestAPI 设置测试 API.
func setupTestAPI(t *testing.T) (*API, *Manager, *gin.Engine) {
	gin.SetMode(gin.TestMode)

	storage := NewMockStorageProvider()
	storage.volumes["test-volume"] = &VolumeInfo{
		Name:       "test-volume",
		MountPoint: "/mnt/test",
		Size:       1000 * 1024 * 1024 * 1024, // 1TB
		Used:       500 * 1024 * 1024 * 1024,  // 500GB
		Free:       500 * 1024 * 1024 * 1024,  // 500GB
	}

	userProvider := NewMockUserProvider()
	userProvider.AddUser("testuser", "/home/testuser")
	userProvider.AddUser("anotheruser", "/home/anotheruser")
	userProvider.AddGroup("testgroup")

	mgr, err := NewManager("", storage, userProvider)
	if err != nil {
		t.Fatalf("创建 Manager 失败: %v", err)
	}

	api := NewAPI(mgr)

	router := gin.New()
	r := router.Group("/api")
	api.RegisterRoutes(r)

	return api, mgr, router
}

// TestAPI_SetQuota 测试设置配额 API.
func TestAPI_SetQuota(t *testing.T) {
	_, _, router := setupTestAPI(t)

	tests := []struct {
		name       string
		request    SetQuotaRequest
		wantStatus int
	}{
		{
			name: "设置用户配额",
			request: SetQuotaRequest{
				Type:       QuotaTypeUser,
				TargetID:   "testuser",
				VolumeName: "test-volume",
				HardLimit:  100 * 1024 * 1024 * 1024, // 100GB
				SoftLimit:  80 * 1024 * 1024 * 1024,  // 80GB
			},
			wantStatus: http.StatusOK,
		},
		{
			name: "设置组配额",
			request: SetQuotaRequest{
				Type:       QuotaTypeGroup,
				TargetID:   "testgroup",
				VolumeName: "test-volume",
				HardLimit:  500 * 1024 * 1024 * 1024, // 500GB
			},
			wantStatus: http.StatusOK,
		},
		{
			name: "用户不存在",
			request: SetQuotaRequest{
				Type:       QuotaTypeUser,
				TargetID:   "nonexistent",
				VolumeName: "test-volume",
				HardLimit:  100 * 1024 * 1024 * 1024,
			},
			wantStatus: http.StatusNotFound,
		},
		{
			name: "无效的硬限制",
			request: SetQuotaRequest{
				Type:       QuotaTypeUser,
				TargetID:   "testuser",
				VolumeName: "test-volume",
				HardLimit:  0,
			},
			wantStatus: http.StatusBadRequest,
		},
		{
			name: "软限制超过硬限制",
			request: SetQuotaRequest{
				Type:       QuotaTypeUser,
				TargetID:   "testuser",
				VolumeName: "test-volume",
				HardLimit:  100,
				SoftLimit:  200,
			},
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, _ := json.Marshal(tt.request)
			req, _ := http.NewRequestWithContext(context.Background(), http.MethodPost, "/api/quota/v2/set", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("状态码错误: got %d, want %d, body: %s", w.Code, tt.wantStatus, w.Body.String())
			}
		})
	}
}

// TestAPI_GetQuota 测试获取配额 API.
func TestAPI_GetQuota(t *testing.T) {
	_, mgr, router := setupTestAPI(t)

	// 先创建一个配额
	quota, err := mgr.CreateQuota(QuotaInput{
		Type:       QuotaTypeUser,
		TargetID:   "testuser",
		VolumeName: "test-volume",
		HardLimit:  100 * 1024 * 1024 * 1024,
		SoftLimit:  80 * 1024 * 1024 * 1024,
	})
	if err != nil {
		t.Fatalf("创建配额失败: %v", err)
	}

	tests := []struct {
		name       string
		quotaID    string
		wantStatus int
	}{
		{
			name:       "获取存在的配额",
			quotaID:    quota.ID,
			wantStatus: http.StatusOK,
		},
		{
			name:       "获取不存在的配额",
			quotaID:    "nonexistent",
			wantStatus: http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, "/api/quota/v2/get/"+tt.quotaID, nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("状态码错误: got %d, want %d", w.Code, tt.wantStatus)
			}
		})
	}
}

// TestAPI_ListQuotas 测试列出配额 API.
func TestAPI_ListQuotas(t *testing.T) {
	_, mgr, router := setupTestAPI(t)

	// 创建多个配额
	mgr.CreateQuota(QuotaInput{
		Type:       QuotaTypeUser,
		TargetID:   "testuser",
		VolumeName: "test-volume",
		HardLimit:  100 * 1024 * 1024 * 1024,
	})
	mgr.CreateQuota(QuotaInput{
		Type:       QuotaTypeUser,
		TargetID:   "anotheruser",
		VolumeName: "test-volume",
		HardLimit:  50 * 1024 * 1024 * 1024,
	})
	mgr.CreateQuota(QuotaInput{
		Type:       QuotaTypeGroup,
		TargetID:   "testgroup",
		VolumeName: "test-volume",
		HardLimit:  500 * 1024 * 1024 * 1024,
	})

	tests := []struct {
		name       string
		query      string
		wantCount  int
		wantStatus int
	}{
		{
			name:       "列出所有配额",
			query:      "",
			wantCount:  3,
			wantStatus: http.StatusOK,
		},
		{
			name:       "列出用户配额",
			query:      "?type=user",
			wantCount:  2,
			wantStatus: http.StatusOK,
		},
		{
			name:       "列出组配额",
			query:      "?type=group",
			wantCount:  1,
			wantStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, "/api/quota/v2/list"+tt.query, nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("状态码错误: got %d, want %d", w.Code, tt.wantStatus)
				return
			}

			var resp Response
			if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
				t.Errorf("解析响应失败: %v", err)
				return
			}

			data, ok := resp.Data.(map[string]interface{})
			if !ok {
				t.Errorf("响应数据格式错误")
				return
			}

			quotas, ok := data["quotas"].([]interface{})
			if !ok {
				t.Errorf("配额列表格式错误")
				return
			}

			if len(quotas) != tt.wantCount {
				t.Errorf("配额数量错误: got %d, want %d", len(quotas), tt.wantCount)
			}
		})
	}
}

// TestAPI_AdjustQuota 测试调整配额 API.
func TestAPI_AdjustQuota(t *testing.T) {
	_, mgr, router := setupTestAPI(t)

	// 创建配额
	quota, err := mgr.CreateQuota(QuotaInput{
		Type:       QuotaTypeUser,
		TargetID:   "testuser",
		VolumeName: "test-volume",
		HardLimit:  100 * 1024 * 1024 * 1024, // 100GB
		SoftLimit:  80 * 1024 * 1024 * 1024,  // 80GB
	})
	if err != nil {
		t.Fatalf("创建配额失败: %v", err)
	}

	tests := []struct {
		name       string
		quotaID    string
		request    AdjustQuotaRequest
		wantStatus int
	}{
		{
			name:    "增加配额",
			quotaID: quota.ID,
			request: AdjustQuotaRequest{
				HardLimitDelta: 50 * 1024 * 1024 * 1024, // 增加 50GB
				SoftLimitDelta: 40 * 1024 * 1024 * 1024, // 增加 40GB
			},
			wantStatus: http.StatusOK,
		},
		{
			name:    "减少配额",
			quotaID: quota.ID,
			request: AdjustQuotaRequest{
				HardLimitDelta: -10 * 1024 * 1024 * 1024, // 减少 10GB
			},
			wantStatus: http.StatusOK,
		},
		{
			name:    "无效调整 - 硬限制为负",
			quotaID: quota.ID,
			request: AdjustQuotaRequest{
				HardLimitDelta: -200 * 1024 * 1024 * 1024, // 减少 200GB (超过当前值)
			},
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, _ := json.Marshal(tt.request)
			req, _ := http.NewRequestWithContext(context.Background(), http.MethodPut, "/api/quota/v2/adjust/"+tt.quotaID, bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("状态码错误: got %d, want %d, body: %s", w.Code, tt.wantStatus, w.Body.String())
			}
		})
	}
}

// TestAPI_DeleteQuota 测试删除配额 API.
func TestAPI_DeleteQuota(t *testing.T) {
	_, mgr, router := setupTestAPI(t)

	// 创建配额
	quota, err := mgr.CreateQuota(QuotaInput{
		Type:       QuotaTypeUser,
		TargetID:   "testuser",
		VolumeName: "test-volume",
		HardLimit:  100 * 1024 * 1024 * 1024,
	})
	if err != nil {
		t.Fatalf("创建配额失败: %v", err)
	}

	tests := []struct {
		name       string
		quotaID    string
		wantStatus int
	}{
		{
			name:       "删除存在的配额",
			quotaID:    quota.ID,
			wantStatus: http.StatusOK,
		},
		{
			name:       "删除不存在的配额",
			quotaID:    "nonexistent",
			wantStatus: http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, _ := http.NewRequestWithContext(context.Background(), http.MethodDelete, "/api/quota/v2/delete/"+tt.quotaID, nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("状态码错误: got %d, want %d", w.Code, tt.wantStatus)
			}
		})
	}
}

// TestAPI_SetLimits 测试设置限制 API.
func TestAPI_SetLimits(t *testing.T) {
	_, mgr, router := setupTestAPI(t)

	quota, err := mgr.CreateQuota(QuotaInput{
		Type:       QuotaTypeUser,
		TargetID:   "testuser",
		VolumeName: "test-volume",
		HardLimit:  100 * 1024 * 1024 * 1024,
	})
	if err != nil {
		t.Fatalf("创建配额失败: %v", err)
	}

	tests := []struct {
		name       string
		quotaID    string
		request    SetLimitsRequest
		wantStatus int
	}{
		{
			name:    "设置有效限制",
			quotaID: quota.ID,
			request: SetLimitsRequest{
				HardLimit:   200 * 1024 * 1024 * 1024,
				SoftLimit:   150 * 1024 * 1024 * 1024,
				GracePeriod: 24,
			},
			wantStatus: http.StatusOK,
		},
		{
			name:    "软限制超过硬限制",
			quotaID: quota.ID,
			request: SetLimitsRequest{
				HardLimit: 100,
				SoftLimit: 200,
			},
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, _ := json.Marshal(tt.request)
			req, _ := http.NewRequestWithContext(context.Background(), http.MethodPut, "/api/quota/v2/limits/"+tt.quotaID, bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("状态码错误: got %d, want %d, body: %s", w.Code, tt.wantStatus, w.Body.String())
			}
		})
	}
}

// TestAPI_GetQuotaStatus 测试获取配额状态 API.
func TestAPI_GetQuotaStatus(t *testing.T) {
	_, mgr, router := setupTestAPI(t)

	quota, err := mgr.CreateQuota(QuotaInput{
		Type:       QuotaTypeUser,
		TargetID:   "testuser",
		VolumeName: "test-volume",
		HardLimit:  100 * 1024 * 1024 * 1024,
		SoftLimit:  80 * 1024 * 1024 * 1024,
	})
	if err != nil {
		t.Fatalf("创建配额失败: %v", err)
	}

	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, "/api/quota/v2/status/"+quota.ID, nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("状态码错误: got %d, want %d", w.Code, http.StatusOK)
		return
	}

	var resp Response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Errorf("解析响应失败: %v", err)
		return
	}

	data, ok := resp.Data.(map[string]interface{})
	if !ok {
		t.Errorf("响应数据格式错误")
		return
	}

	if _, ok := data["status"]; !ok {
		t.Errorf("缺少 status 字段")
	}
}

// TestGracePeriodManager 测试宽限期管理器.
func TestGracePeriodManager(t *testing.T) {
	gpm := NewGracePeriodManager()

	// 测试设置宽限期
	gpm.SetGracePeriod("quota-1", 24*time.Hour)

	duration, expiry := gpm.GetGracePeriodInfo("quota-1")
	if duration != 24*time.Hour {
		t.Errorf("宽限期设置错误: got %v, want %v", duration, 24*time.Hour)
	}
	if expiry != nil {
		t.Errorf("新设置的宽限期不应有过期时间")
	}

	// 测试延长宽限期
	newExpiry := time.Now().Add(48 * time.Hour)
	gpm.ExtendGracePeriod("quota-1", newExpiry)

	_, expiry = gpm.GetGracePeriodInfo("quota-1")
	if expiry == nil {
		t.Errorf("延长后应有过期时间")
	}

	// 测试不存在的配额
	duration, expiry = gpm.GetGracePeriodInfo("nonexistent")
	if duration != 0 || expiry != nil {
		t.Errorf("不存在的配额应返回零值")
	}
}

// TestAPI_BatchSetQuota 测试批量设置配额 API.
func TestAPI_BatchSetQuota(t *testing.T) {
	_, _, router := setupTestAPI(t)

	request := BatchSetQuotaRequest{
		Quotas: []SetQuotaRequest{
			{
				Type:       QuotaTypeUser,
				TargetID:   "testuser",
				VolumeName: "test-volume",
				HardLimit:  100 * 1024 * 1024 * 1024,
			},
			{
				Type:       QuotaTypeUser,
				TargetID:   "anotheruser",
				VolumeName: "test-volume",
				HardLimit:  50 * 1024 * 1024 * 1024,
			},
			{
				Type:       QuotaTypeUser,
				TargetID:   "nonexistent",
				VolumeName: "test-volume",
				HardLimit:  50 * 1024 * 1024 * 1024,
			},
		},
	}

	body, _ := json.Marshal(request)
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodPost, "/api/quota/v2/batch-set", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("状态码错误: got %d, want %d", w.Code, http.StatusOK)
		return
	}

	var resp Response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Errorf("解析响应失败: %v", err)
		return
	}

	data, ok := resp.Data.(map[string]interface{})
	if !ok {
		t.Errorf("响应数据格式错误")
		return
	}

	success := int(data["success"].(float64))
	failed := int(data["failed"].(float64))

	if success != 2 {
		t.Errorf("成功数量错误: got %d, want 2", success)
	}
	if failed != 1 {
		t.Errorf("失败数量错误: got %d, want 1", failed)
	}
}

// TestAPI_GetViolations 测试获取违规列表 API.
func TestAPI_GetViolations(t *testing.T) {
	_, mgr, router := setupTestAPI(t)

	// 创建配额（设置较小的限制以便测试违规）
	mgr.CreateQuota(QuotaInput{
		Type:       QuotaTypeUser,
		TargetID:   "testuser",
		VolumeName: "test-volume",
		HardLimit:  100, // 100 字节（非常小，会触发违规）
		SoftLimit:  50,  // 50 字节
	})

	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, "/api/quota/v2/violations", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("状态码错误: got %d, want %d", w.Code, http.StatusOK)
	}
}
