// Package smb3unixext 提供 SMB3 Unix 扩展支持。
// 单元测试覆盖服务层与 HTTP handler 层。
package smb3unixext

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

// ===== 服务层测试 =====

func TestSetExtension(t *testing.T) {
	s := NewService()

	t.Run("启用扩展", func(t *testing.T) {
		req := &SetExtensionRequest{ShareName: "share1", Enabled: true}
		cfg, err := s.SetExtension(req)
		if err != nil {
			t.Fatalf("SetExtension 失败: %v", err)
		}
		if cfg.ShareName != "share1" {
			t.Errorf("期望 share_name=share1, 实际=%q", cfg.ShareName)
		}
		if !cfg.Enabled {
			t.Error("期望 enabled=true")
		}
		if !cfg.IsMultiProtocol {
			t.Error("启用后应标记为多协议模式")
		}
		if cfg.Protocol != ProtocolMulti {
			t.Errorf("期望 protocol=multi, 实际=%q", cfg.Protocol)
		}
		if len(cfg.Capabilities) == 0 {
			t.Error("能力列表不应为空")
		}
		if cfg.UpdatedAt.IsZero() {
			t.Error("UpdatedAt 不应为零值")
		}
		if cfg.CreatedAt.IsZero() {
			t.Error("CreatedAt 不应为零值")
		}
	})

	t.Run("禁用扩展", func(t *testing.T) {
		req := &SetExtensionRequest{ShareName: "share2", Enabled: false}
		cfg, err := s.SetExtension(req)
		if err != nil {
			t.Fatalf("SetExtension 失败: %v", err)
		}
		if cfg.Enabled {
			t.Error("期望 enabled=false")
		}
		if cfg.IsMultiProtocol {
			t.Error("禁用后不应为多协议模式")
		}
	})

	t.Run("空请求", func(t *testing.T) {
		_, err := s.SetExtension(nil)
		if err == nil {
			t.Error("nil 请求应返回错误")
		}
	})

	t.Run("空共享名", func(t *testing.T) {
		req := &SetExtensionRequest{Enabled: true}
		_, err := s.SetExtension(req)
		if err == nil {
			t.Error("空共享名应返回错误")
		}
	})

	t.Run("更新保留创建时间", func(t *testing.T) {
		req1 := &SetExtensionRequest{ShareName: "share3", Enabled: true}
		cfg1, _ := s.SetExtension(req1)
		originalCreated := cfg1.CreatedAt

		time.Sleep(10 * time.Millisecond)

		req2 := &SetExtensionRequest{ShareName: "share3", Enabled: false}
		cfg2, _ := s.SetExtension(req2)

		if !cfg2.CreatedAt.Equal(originalCreated) {
			t.Error("更新后 CreatedAt 应保持不变")
		}
		if !cfg2.UpdatedAt.After(cfg1.UpdatedAt) {
			t.Error("更新后 UpdatedAt 应更新")
		}
	})
}

func TestGetExtension(t *testing.T) {
	s := NewService()
	s.SetExtension(&SetExtensionRequest{ShareName: "share1", Enabled: true})

	t.Run("获取存在的配置", func(t *testing.T) {
		cfg, err := s.GetExtension("share1")
		if err != nil {
			t.Fatalf("GetExtension 失败: %v", err)
		}
		if cfg.ShareName != "share1" {
			t.Errorf("期望 share_name=share1, 实际=%q", cfg.ShareName)
		}
	})

	t.Run("获取不存在的配置", func(t *testing.T) {
		_, err := s.GetExtension("nonexistent")
		if err == nil {
			t.Error("不存在的配置应返回错误")
		}
	})
}

func TestGetExtensionStatus(t *testing.T) {
	s := NewService()
	s.SetExtension(&SetExtensionRequest{ShareName: "share1", Enabled: true})

	t.Run("获取已启用的状态", func(t *testing.T) {
		status, err := s.GetExtensionStatus("share1")
		if err != nil {
			t.Fatalf("GetExtensionStatus 失败: %v", err)
		}
		if status.Status != ExtensionStatusEnabled {
			t.Errorf("期望 status=enabled, 实际=%q", status.Status)
		}
		if !status.IsMultiProtocol {
			t.Error("期望 is_multi_protocol=true")
		}
		if len(status.Capabilities) == 0 {
			t.Error("能力列表不应为空")
		}
	})

	t.Run("获取未启用的状态", func(t *testing.T) {
		s.SetExtension(&SetExtensionRequest{ShareName: "share2", Enabled: false})
		status, err := s.GetExtensionStatus("share2")
		if err != nil {
			t.Fatalf("GetExtensionStatus 失败: %v", err)
		}
		if status.Status != ExtensionStatusDisabled {
			t.Errorf("期望 status=disabled, 实际=%q", status.Status)
		}
	})

	t.Run("不存在的共享", func(t *testing.T) {
		_, err := s.GetExtensionStatus("nonexistent")
		if err == nil {
			t.Error("不存在的共享应返回错误")
		}
	})
}

func TestListExtensions(t *testing.T) {
	s := NewService()

	t.Run("空列表", func(t *testing.T) {
		configs := s.ListExtensions()
		if len(configs) != 0 {
			t.Errorf("期望 0 条配置, 实际=%d", len(configs))
		}
	})

	t.Run("多条配置", func(t *testing.T) {
		s.SetExtension(&SetExtensionRequest{ShareName: "s1", Enabled: true})
		s.SetExtension(&SetExtensionRequest{ShareName: "s2", Enabled: false})
		configs := s.ListExtensions()
		if len(configs) != 2 {
			t.Errorf("期望 2 条配置, 实际=%d", len(configs))
		}
	})
}

func TestRemoveExtension(t *testing.T) {
	s := NewService()
	s.SetExtension(&SetExtensionRequest{ShareName: "s1", Enabled: true})

	t.Run("移除存在的配置", func(t *testing.T) {
		err := s.RemoveExtension("s1")
		if err != nil {
			t.Fatalf("RemoveExtension 失败: %v", err)
		}
		if _, err := s.GetExtension("s1"); err == nil {
			t.Error("移除后应返回错误")
		}
	})

	t.Run("移除不存在的配置", func(t *testing.T) {
		err := s.RemoveExtension("nonexistent")
		if err == nil {
			t.Error("移除不存在的配置应返回错误")
		}
	})
}

func TestIsMultiProtocol(t *testing.T) {
	s := NewService()
	s.SetExtension(&SetExtensionRequest{ShareName: "enabled_share", Enabled: true})
	s.SetExtension(&SetExtensionRequest{ShareName: "disabled_share", Enabled: false})

	if !s.IsMultiProtocol("enabled_share") {
		t.Error("已启用的共享应为多协议模式")
	}
	if s.IsMultiProtocol("disabled_share") {
		t.Error("未启用的共享不应为多协议模式")
	}
	if s.IsMultiProtocol("nonexistent") {
		t.Error("不存在的共享应返回 false")
	}
}

func TestCanEnableUnixExtensions(t *testing.T) {
	s := NewService()
	s.SetExtension(&SetExtensionRequest{ShareName: "s1", Enabled: true})

	can, err := s.CanEnableUnixExtensions("s1")
	if err != nil {
		t.Fatalf("CanEnableUnixExtensions 失败: %v", err)
	}
	if !can {
		t.Error("已启用的共享应返回 true")
	}

	s.SetExtension(&SetExtensionRequest{ShareName: "s2", Enabled: false})
	can, err = s.CanEnableUnixExtensions("s2")
	if err != nil {
		t.Fatalf("CanEnableUnixExtensions 失败: %v", err)
	}
	if can {
		t.Error("未启用的共享应返回 false")
	}

	_, err = s.CanEnableUnixExtensions("nonexistent")
	if err == nil {
		t.Error("不存在的共享应返回错误")
	}
}

func TestNegotiateClientCapabilities(t *testing.T) {
	s := NewService()
	s.SetExtension(&SetExtensionRequest{ShareName: "share1", Enabled: true})

	t.Run("成功协商", func(t *testing.T) {
		req := &ClientCapabilityRequest{
			ShareName: "share1",
			ClientCapabilities: []ClientCapability{
				CapabilityPosixPath,
				CapabilityPosixSymlink,
				CapabilityPosixFileLock,
				"unknown_capability", // 不在服务端能力列表中
			},
		}
		result, err := s.NegotiateClientCapabilities(req)
		if err != nil {
			t.Fatalf("协商失败: %v", err)
		}
		if !result.ClientNegotiated {
			t.Error("协商后 ClientNegotiated 应为 true")
		}
		// 应只包含服务端也支持的能力（交集）
		if len(result.NegotiatedCapabilities) != 3 {
			t.Errorf("期望 3 个协商能力, 实际=%d", len(result.NegotiatedCapabilities))
		}
	})

	t.Run("空请求", func(t *testing.T) {
		_, err := s.NegotiateClientCapabilities(nil)
		if err == nil {
			t.Error("nil 请求应返回错误")
		}
	})

	t.Run("空共享名", func(t *testing.T) {
		req := &ClientCapabilityRequest{
			ClientCapabilities: []ClientCapability{CapabilityPosixPath},
		}
		_, err := s.NegotiateClientCapabilities(req)
		if err == nil {
			t.Error("空共享名应返回错误")
		}
	})

	t.Run("空能力列表", func(t *testing.T) {
		req := &ClientCapabilityRequest{
			ShareName:         "share1",
			ClientCapabilities: []ClientCapability{},
		}
		_, err := s.NegotiateClientCapabilities(req)
		if err == nil {
			t.Error("空能力列表应返回错误")
		}
	})

	t.Run("未启用的共享", func(t *testing.T) {
		s.SetExtension(&SetExtensionRequest{ShareName: "disabled", Enabled: false})
		req := &ClientCapabilityRequest{
			ShareName:         "disabled",
			ClientCapabilities: []ClientCapability{CapabilityPosixPath},
		}
		_, err := s.NegotiateClientCapabilities(req)
		if err == nil {
			t.Error("未启用的共享协商应返回错误")
		}
	})

	t.Run("不存在的共享", func(t *testing.T) {
		req := &ClientCapabilityRequest{
			ShareName:         "nonexistent",
			ClientCapabilities: []ClientCapability{CapabilityPosixPath},
		}
		_, err := s.NegotiateClientCapabilities(req)
		if err == nil {
			t.Error("不存在的共享应返回错误")
		}
	})
}

func TestGetSupportStatus(t *testing.T) {
	s := NewService()

	// 空状态
	status := s.GetSupportStatus()
	if !status.Supported {
		t.Error("应支持 SMB3 Unix 扩展")
	}
	if status.MinSMBVersion != "3.1.1" {
		t.Errorf("期望 min_smb_version=3.1.1, 实际=%q", status.MinSMBVersion)
	}
	if status.TotalShares != 0 {
		t.Errorf("期望 total_shares=0, 实际=%d", status.TotalShares)
	}
	if status.EnabledShares != 0 {
		t.Errorf("期望 enabled_shares=0, 实际=%d", status.EnabledShares)
	}

	// 添加配置
	s.SetExtension(&SetExtensionRequest{ShareName: "s1", Enabled: true})
	s.SetExtension(&SetExtensionRequest{ShareName: "s2", Enabled: false})

	status = s.GetSupportStatus()
	if status.TotalShares != 2 {
		t.Errorf("期望 total_shares=2, 实际=%d", status.TotalShares)
	}
	if status.EnabledShares != 1 {
		t.Errorf("期望 enabled_shares=1, 实际=%d", status.EnabledShares)
	}
	if len(status.DefaultCapabilities) == 0 {
		t.Error("默认能力列表不应为空")
	}
}

func TestEnableAll(t *testing.T) {
	s := NewService()
	s.SetExtension(&SetExtensionRequest{ShareName: "s1", Enabled: false})
	s.SetExtension(&SetExtensionRequest{ShareName: "s2", Enabled: false})

	count := s.EnableAll()
	if count != 2 {
		t.Errorf("期望启用 2 个, 实际=%d", count)
	}

	// 再次调用不应启用任何
	count = s.EnableAll()
	if count != 0 {
		t.Errorf("期望启用 0 个, 实际=%d", count)
	}

	// 验证都已启用
	for _, name := range []string{"s1", "s2"} {
		cfg, _ := s.GetExtension(name)
		if !cfg.Enabled {
			t.Errorf("共享 %s 应已启用", name)
		}
	}
}

func TestDisableAll(t *testing.T) {
	s := NewService()
	s.SetExtension(&SetExtensionRequest{ShareName: "s1", Enabled: true})
	s.SetExtension(&SetExtensionRequest{ShareName: "s2", Enabled: true})

	count := s.DisableAll()
	if count != 2 {
		t.Errorf("期望禁用 2 个, 实际=%d", count)
	}

	// 再次调用不应禁用任何
	count = s.DisableAll()
	if count != 0 {
		t.Errorf("期望禁用 0 个, 实际=%d", count)
	}

	// 验证都已禁用
	for _, name := range []string{"s1", "s2"} {
		cfg, _ := s.GetExtension(name)
		if cfg.Enabled {
			t.Errorf("共享 %s 应已禁用", name)
		}
		if cfg.ClientNegotiated {
			t.Errorf("共享 %s 禁用后 ClientNegotiated 应为 false", name)
		}
	}
}

// ===== HTTP Handler 测试 =====

func setupTestRouter(t *testing.T, s *Service) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := NewHandler(s)
	h.RegisterRoutes(r.Group("/api/v1"))
	return r
}

func TestHandler_GetSupportStatus(t *testing.T) {
	s := NewService()
	r := setupTestRouter(t, s)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/smb3-unix-ext/support-status", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("期望状态码 200, 实际=%d: %s", w.Code, w.Body.String())
	}

	var resp response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("解析响应失败: %v", err)
	}
	if resp.Code != 0 {
		t.Errorf("期望 code=0, 实际=%d", resp.Code)
	}
}

func TestHandler_SetExtension(t *testing.T) {
	s := NewService()
	r := setupTestRouter(t, s)

	body := `{"share_name":"share1","enabled":true}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/smb3-unix-ext/extensions", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("期望状态码 201, 实际=%d: %s", w.Code, w.Body.String())
	}

	var resp response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("解析响应失败: %v", err)
	}
	if resp.Code != 0 {
		t.Errorf("期望 code=0, 实际=%d", resp.Code)
	}
}

func TestHandler_SetExtension_InvalidBody(t *testing.T) {
	s := NewService()
	r := setupTestRouter(t, s)

	body := `{"enabled":true}` // 缺少 share_name
	req := httptest.NewRequest(http.MethodPost, "/api/v1/smb3-unix-ext/extensions", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("期望状态码 400, 实际=%d", w.Code)
	}
}

func TestHandler_GetExtension(t *testing.T) {
	s := NewService()
	s.SetExtension(&SetExtensionRequest{ShareName: "share1", Enabled: true})
	r := setupTestRouter(t, s)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/smb3-unix-ext/extensions/share1", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("期望状态码 200, 实际=%d: %s", w.Code, w.Body.String())
	}
}

func TestHandler_GetExtension_NotFound(t *testing.T) {
	s := NewService()
	r := setupTestRouter(t, s)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/smb3-unix-ext/extensions/nonexistent", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("期望状态码 404, 实际=%d", w.Code)
	}
}

func TestHandler_ListExtensions(t *testing.T) {
	s := NewService()
	s.SetExtension(&SetExtensionRequest{ShareName: "s1", Enabled: true})
	s.SetExtension(&SetExtensionRequest{ShareName: "s2", Enabled: false})
	r := setupTestRouter(t, s)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/smb3-unix-ext/extensions", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("期望状态码 200, 实际=%d", w.Code)
	}

	var resp response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("解析响应失败: %v", err)
	}
	if resp.Code != 0 {
		t.Errorf("期望 code=0, 实际=%d", resp.Code)
	}
}

func TestHandler_RemoveExtension(t *testing.T) {
	s := NewService()
	s.SetExtension(&SetExtensionRequest{ShareName: "share1", Enabled: true})
	r := setupTestRouter(t, s)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/smb3-unix-ext/extensions/share1", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("期望状态码 200, 实际=%d: %s", w.Code, w.Body.String())
	}

	// 验证已移除
	if _, err := s.GetExtension("share1"); err == nil {
		t.Error("移除后应返回错误")
	}
}

func TestHandler_RemoveExtension_NotFound(t *testing.T) {
	s := NewService()
	r := setupTestRouter(t, s)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/smb3-unix-ext/extensions/nonexistent", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("期望状态码 404, 实际=%d", w.Code)
	}
}

func TestHandler_NegotiateCapabilities(t *testing.T) {
	s := NewService()
	s.SetExtension(&SetExtensionRequest{ShareName: "share1", Enabled: true})
	r := setupTestRouter(t, s)

	body := `{"share_name":"share1","client_capabilities":["posix_path_operations","posix_symlink_operations"]}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/smb3-unix-ext/negotiate", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("期望状态码 200, 实际=%d: %s", w.Code, w.Body.String())
	}

	var resp response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("解析响应失败: %v", err)
	}
	if resp.Code != 0 {
		t.Errorf("期望 code=0, 实际=%d", resp.Code)
	}
}

func TestHandler_NegotiateCapabilities_InvalidBody(t *testing.T) {
	s := NewService()
	s.SetExtension(&SetExtensionRequest{ShareName: "share1", Enabled: true})
	r := setupTestRouter(t, s)

	body := `{"share_name":"share1"}` // 缺少 client_capabilities
	req := httptest.NewRequest(http.MethodPost, "/api/v1/smb3-unix-ext/negotiate", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("期望状态码 400, 实际=%d", w.Code)
	}
}

func TestHandler_EnableAll(t *testing.T) {
	s := NewService()
	s.SetExtension(&SetExtensionRequest{ShareName: "s1", Enabled: false})
	s.SetExtension(&SetExtensionRequest{ShareName: "s2", Enabled: false})
	r := setupTestRouter(t, s)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/smb3-unix-ext/enable-all", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("期望状态码 200, 实际=%d: %s", w.Code, w.Body.String())
	}
}

func TestHandler_DisableAll(t *testing.T) {
	s := NewService()
	s.SetExtension(&SetExtensionRequest{ShareName: "s1", Enabled: true})
	s.SetExtension(&SetExtensionRequest{ShareName: "s2", Enabled: true})
	r := setupTestRouter(t, s)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/smb3-unix-ext/disable-all", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("期望状态码 200, 实际=%d: %s", w.Code, w.Body.String())
	}
}

// ===== 类型常量测试 =====

func TestExtensionStatusValues(t *testing.T) {
	if ExtensionStatusEnabled != "enabled" {
		t.Errorf("期望 'enabled', 实际=%q", ExtensionStatusEnabled)
	}
	if ExtensionStatusDisabled != "disabled" {
		t.Errorf("期望 'disabled', 实际=%q", ExtensionStatusDisabled)
	}
}

func TestDefaultCapabilities(t *testing.T) {
	if len(DefaultCapabilities) != 6 {
		t.Errorf("期望 6 个默认能力, 实际=%d", len(DefaultCapabilities))
	}

	// 确保所有默认能力非空
	for _, cap := range DefaultCapabilities {
		if cap == "" {
			t.Error("默认能力不应为空字符串")
		}
	}
}

func TestShareProtocolValues(t *testing.T) {
	if ProtocolSMB != "smb" {
		t.Errorf("期望 'smb', 实际=%q", ProtocolSMB)
	}
	if ProtocolNFS != "nfs" {
		t.Errorf("期望 'nfs', 实际=%q", ProtocolNFS)
	}
	if ProtocolAFP != "afp" {
		t.Errorf("期望 'afp', 实际=%q", ProtocolAFP)
	}
	if ProtocolMulti != "multi" {
		t.Errorf("期望 'multi', 实际=%q", ProtocolMulti)
	}
}
