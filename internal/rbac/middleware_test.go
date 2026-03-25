package rbac

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

// ========== 中间件配置测试 ==========

func TestDefaultMiddlewareConfig(t *testing.T) {
	config := DefaultMiddlewareConfig()

	if len(config.SkipPaths) == 0 {
		t.Error("SkipPaths should not be empty")
	}

	if len(config.PublicPaths) == 0 {
		t.Error("PublicPaths should not be empty")
	}

	if config.TokenExtractor == nil {
		t.Error("TokenExtractor should not be nil")
	}

	if config.OnDenied == nil {
		t.Error("OnDenied should not be nil")
	}

	if config.OnError == nil {
		t.Error("OnError should not be nil")
	}
}

func TestNewMiddleware(t *testing.T) {
	m, cleanup := newTestManager(t)
	defer cleanup()

	config := DefaultMiddlewareConfig()
	middleware := NewMiddleware(m, config)

	if middleware == nil {
		t.Fatal("middleware is nil")
	}
}

// ========== 令牌提取器测试 ==========

func TestExtractBearerToken(t *testing.T) {
	tests := []struct {
		name     string
		header   string
		expected string
	}{
		{"valid bearer", "Bearer token123", "token123"},
		{"no bearer prefix", "token123", ""},
		{"empty header", "", ""},
		{"bearer with extra space", "Bearer  token123", " token123"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequestWithContext(context.Background(), "GET", "/test", nil)
			if tt.header != "" {
				req.Header.Set("Authorization", tt.header)
			}

			result := ExtractBearerToken(req)
			if result != tt.expected {
				t.Errorf("ExtractBearerToken() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestExtractCookieToken(t *testing.T) {
	tests := []struct {
		name       string
		cookieName string
		cookieVal  string
		expected   string
	}{
		{"valid cookie", "session", "token123", "token123"},
		{"missing cookie", "session", "", ""},
		{"different cookie name", "auth", "token123", "token123"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequestWithContext(context.Background(), "GET", "/test", nil)
			if tt.cookieVal != "" {
				req.AddCookie(&http.Cookie{
					Name:  tt.cookieName,
					Value: tt.cookieVal,
				})
			}

			extractor := ExtractCookieToken(tt.cookieName)
			result := extractor(req)
			if result != tt.expected {
				t.Errorf("ExtractCookieToken() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestExtractQueryToken(t *testing.T) {
	tests := []struct {
		name      string
		paramName string
		paramVal  string
		expected  string
	}{
		{"valid query param", "token", "token123", "token123"},
		{"missing param", "token", "", ""},
		{"different param name", "auth", "token123", "token123"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequestWithContext(context.Background(), "GET", "/test", nil)
			if tt.paramVal != "" {
				q := req.URL.Query()
				q.Set(tt.paramName, tt.paramVal)
				req.URL.RawQuery = q.Encode()
			}

			extractor := ExtractQueryToken(tt.paramName)
			result := extractor(req)
			if result != tt.expected {
				t.Errorf("ExtractQueryToken() = %q, want %q", result, tt.expected)
			}
		})
	}
}

// ========== 默认处理器测试 ==========

func TestDefaultDeniedHandler(t *testing.T) {
	w := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), "GET", "/test", nil)

	result := &CheckResult{
		Allowed:      false,
		Reason:       "权限不足",
		DeniedBy:     "policy-123",
		MissingPerms: []string{"storage:write"},
	}

	DefaultDeniedHandler(w, req, result)

	if w.Code != http.StatusForbidden {
		t.Errorf("Status = %d, want %d", w.Code, http.StatusForbidden)
	}
}

func TestDefaultErrorHandler_PermissionDenied(t *testing.T) {
	w := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), "GET", "/test", nil)

	DefaultErrorHandler(w, req, ErrPermissionDenied)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Status = %d, want %d", w.Code, http.StatusUnauthorized)
	}
}

func TestDefaultErrorHandler_OtherError(t *testing.T) {
	w := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), "GET", "/test", nil)

	DefaultErrorHandler(w, req, ErrUserNotFound)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("Status = %d, want %d", w.Code, http.StatusInternalServerError)
	}
}

// ========== 上下文辅助函数测试 ==========

func TestGetUserIDFromContext(t *testing.T) {
	tests := []struct {
		name     string
		ctx      context.Context
		expected string
	}{
		{"with user id", context.WithValue(context.Background(), UserIDKey, "user123"), "user123"},
		{"without user id", context.Background(), ""},
		{"wrong type", context.WithValue(context.Background(), UserIDKey, 123), ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetUserIDFromContext(tt.ctx)
			if result != tt.expected {
				t.Errorf("GetUserIDFromContext() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestGetUsernameFromContext(t *testing.T) {
	tests := []struct {
		name     string
		ctx      context.Context
		expected string
	}{
		{"with username", context.WithValue(context.Background(), UsernameKey, "testuser"), "testuser"},
		{"without username", context.Background(), ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetUsernameFromContext(tt.ctx)
			if result != tt.expected {
				t.Errorf("GetUsernameFromContext() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestGetRoleFromContext(t *testing.T) {
	tests := []struct {
		name     string
		ctx      context.Context
		expected Role
	}{
		{"with role", context.WithValue(context.Background(), UserRoleKey, RoleAdmin), RoleAdmin},
		{"without role", context.Background(), ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetRoleFromContext(tt.ctx)
			if result != tt.expected {
				t.Errorf("GetRoleFromContext() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestGetPermissionResultFromContext(t *testing.T) {
	result := &CheckResult{Allowed: true, Reason: "test"}

	tests := []struct {
		name     string
		ctx      context.Context
		expected *CheckResult
	}{
		{"with result", context.WithValue(context.Background(), PermissionKey, result), result},
		{"without result", context.Background(), nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetPermissionResultFromContext(tt.ctx)
			if got != tt.expected {
				t.Errorf("GetPermissionResultFromContext() = %v, want %v", got, tt.expected)
			}
		})
	}
}

// ========== 中间件 Handler 测试 ==========

func TestMiddleware_Handler_SkipPaths(t *testing.T) {
	m, cleanup := newTestManager(t)
	defer cleanup()

	config := DefaultMiddlewareConfig()
	config.SkipPaths = []string{"/health"}

	middleware := NewMiddleware(m, config)

	handlerCalled := false
	handler := middleware.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequestWithContext(context.Background(), "GET", "/health", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if !handlerCalled {
		t.Error("handler should be called for skip paths")
	}
}

func TestMiddleware_Handler_PublicPaths(t *testing.T) {
	m, cleanup := newTestManager(t)
	defer cleanup()

	config := DefaultMiddlewareConfig()
	config.PublicPaths = []string{"/api/auth/login"}

	middleware := NewMiddleware(m, config)

	handlerCalled := false
	handler := middleware.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequestWithContext(context.Background(), "POST", "/api/auth/login", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if !handlerCalled {
		t.Error("handler should be called for public paths")
	}
}

func TestMiddleware_Handler_MissingToken(t *testing.T) {
	m, cleanup := newTestManager(t)
	defer cleanup()

	config := DefaultMiddlewareConfig()
	config.OnError = func(w http.ResponseWriter, r *http.Request, err error) {
		w.WriteHeader(http.StatusUnauthorized)
	}

	middleware := NewMiddleware(m, config)

	handler := middleware.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequestWithContext(context.Background(), "GET", "/api/protected", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Status = %d, want %d", w.Code, http.StatusUnauthorized)
	}
}

// ========== RequirePermission 测试 ==========

func TestMiddleware_RequirePermission(t *testing.T) {
	m, cleanup := newTestManager(t)
	defer cleanup()

	_ = m.SetUserRole("user1", "testuser", RoleOperator)

	config := DefaultMiddlewareConfig()
	middleware := NewMiddleware(m, config)

	handlerCalled := false
	handler := middleware.RequirePermission("system", "read")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
		w.WriteHeader(http.StatusOK)
	}))

	// 创建带有用户信息的请求
	ctx := context.WithValue(context.Background(), UserIDKey, "user1")
	req := httptest.NewRequestWithContext(context.Background(), "GET", "/api/test", nil).WithContext(ctx)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if !handlerCalled {
		t.Error("handler should be called for user with permission")
	}
}

func TestMiddleware_RequirePermission_Denied(t *testing.T) {
	m, cleanup := newTestManager(t)
	defer cleanup()

	_ = m.SetUserRole("user1", "testuser", RoleGuest)

	config := DefaultMiddlewareConfig()
	middleware := NewMiddleware(m, config)

	handlerCalled := false
	handler := middleware.RequirePermission("system", "write")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
		w.WriteHeader(http.StatusOK)
	}))

	ctx := context.WithValue(context.Background(), UserIDKey, "user1")
	req := httptest.NewRequestWithContext(context.Background(), "GET", "/api/test", nil).WithContext(ctx)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if handlerCalled {
		t.Error("handler should not be called for user without permission")
	}

	if w.Code != http.StatusForbidden {
		t.Errorf("Status = %d, want %d", w.Code, http.StatusForbidden)
	}
}

// ========== RequireRole 测试 ==========

func TestMiddleware_RequireRole(t *testing.T) {
	m, cleanup := newTestManager(t)
	defer cleanup()

	config := DefaultMiddlewareConfig()
	middleware := NewMiddleware(m, config)

	handlerCalled := false
	handler := middleware.RequireRole(RoleAdmin, RoleOperator)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
		w.WriteHeader(http.StatusOK)
	}))

	ctx := context.WithValue(context.Background(), UserRoleKey, RoleOperator)
	req := httptest.NewRequestWithContext(context.Background(), "GET", "/api/test", nil).WithContext(ctx)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if !handlerCalled {
		t.Error("handler should be called for user with required role")
	}
}

func TestMiddleware_RequireRole_Denied(t *testing.T) {
	m, cleanup := newTestManager(t)
	defer cleanup()

	config := DefaultMiddlewareConfig()
	middleware := NewMiddleware(m, config)

	handlerCalled := false
	handler := middleware.RequireRole(RoleAdmin)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
		w.WriteHeader(http.StatusOK)
	}))

	ctx := context.WithValue(context.Background(), UserRoleKey, RoleGuest)
	req := httptest.NewRequestWithContext(context.Background(), "GET", "/api/test", nil).WithContext(ctx)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if handlerCalled {
		t.Error("handler should not be called for user without required role")
	}

	if w.Code != http.StatusForbidden {
		t.Errorf("Status = %d, want %d", w.Code, http.StatusForbidden)
	}
}

func TestMiddleware_RequireAdmin(t *testing.T) {
	m, cleanup := newTestManager(t)
	defer cleanup()

	config := DefaultMiddlewareConfig()
	middleware := NewMiddleware(m, config)

	handlerCalled := false
	handler := middleware.RequireAdmin()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
		w.WriteHeader(http.StatusOK)
	}))

	ctx := context.WithValue(context.Background(), UserRoleKey, RoleAdmin)
	req := httptest.NewRequestWithContext(context.Background(), "GET", "/api/test", nil).WithContext(ctx)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if !handlerCalled {
		t.Error("handler should be called for admin user")
	}
}

func TestMiddleware_RequireOperator(t *testing.T) {
	m, cleanup := newTestManager(t)
	defer cleanup()

	config := DefaultMiddlewareConfig()
	middleware := NewMiddleware(m, config)

	tests := []struct {
		name     string
		role     Role
		expected bool
	}{
		{"admin", RoleAdmin, true},
		{"operator", RoleOperator, true},
		{"readonly", RoleReadOnly, false},
		{"guest", RoleGuest, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handlerCalled := false
			handler := middleware.RequireOperator()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				handlerCalled = true
				w.WriteHeader(http.StatusOK)
			}))

			ctx := context.WithValue(context.Background(), UserRoleKey, tt.role)
			req := httptest.NewRequestWithContext(context.Background(), "GET", "/api/test", nil).WithContext(ctx)
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)

			if handlerCalled != tt.expected {
				t.Errorf("handler called = %v, want %v", handlerCalled, tt.expected)
			}
		})
	}
}

// ========== 资源权限检查器测试 ==========

func TestNewResourcePermission(t *testing.T) {
	m, cleanup := newTestManager(t)
	defer cleanup()

	config := DefaultMiddlewareConfig()
	middleware := NewMiddleware(m, config)

	rp := NewResourcePermission(m, middleware)
	if rp == nil {
		t.Fatal("ResourcePermission is nil")
	}
}

func TestResourcePermission_CanRead(t *testing.T) {
	m, cleanup := newTestManager(t)
	defer cleanup()
	_ = m.SetUserRole("user1", "testuser", RoleReadOnly)

	config := DefaultMiddlewareConfig()
	middleware := NewMiddleware(m, config)
	rp := NewResourcePermission(m, middleware)

	handlerCalled := false
	handler := rp.CanRead("storage")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
		w.WriteHeader(http.StatusOK)
	}))

	ctx := context.WithValue(context.Background(), UserIDKey, "user1")
	req := httptest.NewRequestWithContext(context.Background(), "GET", "/api/test", nil).WithContext(ctx)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if !handlerCalled {
		t.Error("handler should be called for user with read permission")
	}
}

func TestResourcePermission_CanWrite(t *testing.T) {
	m, cleanup := newTestManager(t)
	defer cleanup()
	_ = m.SetUserRole("user1", "testuser", RoleOperator)

	config := DefaultMiddlewareConfig()
	middleware := NewMiddleware(m, config)
	rp := NewResourcePermission(m, middleware)

	handlerCalled := false
	handler := rp.CanWrite("storage")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
		w.WriteHeader(http.StatusOK)
	}))

	ctx := context.WithValue(context.Background(), UserIDKey, "user1")
	req := httptest.NewRequestWithContext(context.Background(), "GET", "/api/test", nil).WithContext(ctx)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if !handlerCalled {
		t.Error("handler should be called for user with write permission")
	}
}

func TestResourcePermission_CanAdmin(t *testing.T) {
	m, cleanup := newTestManager(t)
	defer cleanup()
	_ = m.SetUserRole("user1", "testuser", RoleAdmin)

	config := DefaultMiddlewareConfig()
	middleware := NewMiddleware(m, config)
	rp := NewResourcePermission(m, middleware)

	handlerCalled := false
	handler := rp.CanAdmin("storage")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
		w.WriteHeader(http.StatusOK)
	}))

	ctx := context.WithValue(context.Background(), UserIDKey, "user1")
	req := httptest.NewRequestWithContext(context.Background(), "GET", "/api/test", nil).WithContext(ctx)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if !handlerCalled {
		t.Error("handler should be called for admin user")
	}
}

// ========== 批量权限检查测试 ==========

func TestManager_CheckMultiple(t *testing.T) {
	m, cleanup := newTestManager(t)
	defer cleanup()
	_ = m.SetUserRole("user1", "testuser", RoleOperator)

	checks := []struct{ Resource, Action string }{
		{"system", "read"},
		{"system", "write"},
		{"user", "admin"},
	}

	results := m.CheckMultiple("user1", checks)

	if len(results) != 3 {
		t.Errorf("CheckMultiple returned %d results, want 3", len(results))
	}

	if !results["system:read"] {
		t.Error("user should have system:read permission")
	}

	if !results["system:write"] {
		t.Error("user should have system:write permission")
	}

	if results["user:admin"] {
		t.Error("user should not have user:admin permission")
	}
}

func TestManager_CheckAll(t *testing.T) {
	m, cleanup := newTestManager(t)
	defer cleanup()
	_ = m.SetUserRole("user1", "testuser", RoleOperator)

	tests := []struct {
		name     string
		checks   []struct{ Resource, Action string }
		expected bool
	}{
		{
			"all granted",
			[]struct{ Resource, Action string }{
				{"system", "read"},
				{"system", "write"},
			},
			true,
		},
		{
			"one denied",
			[]struct{ Resource, Action string }{
				{"system", "read"},
				{"user", "admin"},
			},
			false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := m.CheckAll("user1", tt.checks)
			if result != tt.expected {
				t.Errorf("CheckAll() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestManager_CheckAny(t *testing.T) {
	m, cleanup := newTestManager(t)
	defer cleanup()
	_ = m.SetUserRole("user1", "testuser", RoleOperator)

	tests := []struct {
		name     string
		checks   []struct{ Resource, Action string }
		expected bool
	}{
		{
			"all denied",
			[]struct{ Resource, Action string }{
				{"user", "admin"},
				{"user", "delete"},
			},
			false,
		},
		{
			"one granted",
			[]struct{ Resource, Action string }{
				{"system", "read"},
				{"user", "admin"},
			},
			true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := m.CheckAny("user1", tt.checks)
			if result != tt.expected {
				t.Errorf("CheckAny() = %v, want %v", result, tt.expected)
			}
		})
	}
}

// ========== WithPermission 测试 ==========

func TestMiddleware_WithPermission(t *testing.T) {
	m, cleanup := newTestManager(t)
	defer cleanup()
	_ = m.SetUserRole("user1", "testuser", RoleOperator)

	config := DefaultMiddlewareConfig()
	middleware := NewMiddleware(m, config)

	handlerCalled := false
	handler := middleware.WithPermission("system", "read", func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
		w.WriteHeader(http.StatusOK)
	})

	ctx := context.WithValue(context.Background(), UserIDKey, "user1")
	req := httptest.NewRequestWithContext(context.Background(), "GET", "/api/test", nil).WithContext(ctx)
	w := httptest.NewRecorder()
	handler(w, req)

	if !handlerCalled {
		t.Error("handler should be called for user with permission")
	}
}
